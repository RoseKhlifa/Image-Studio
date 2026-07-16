package windowing

import (
	"sync"
	"time"

	"gioui.org/app"
	"gioui.org/io/system"
)

const pendingActionWakeInterval = 10 * time.Millisecond

// Manager owns the process-local set of independent Gio desktop windows.
// The program must still call app.Main or app.Events once from its main
// goroutine after opening its initial windows.
type Manager struct {
	factory Factory
	onError ErrorHandler
	windows *registry[*managedWindow]
}

// NewManager creates a window manager. A nil onError handler is allowed.
// A missing factory is reported by Open so a Manager can be constructed before
// all application services are wired.
func NewManager(factory Factory, onError ErrorHandler) *Manager {
	return &Manager{
		factory: factory,
		onError: onError,
		windows: newRegistry[*managedWindow](),
	}
}

// Open creates an independent app.Window and window goroutine. If the same
// normalized Role and WorkspaceID are already registered, Open raises that
// window and returns created=false.
func (m *Manager) Open(request Request) (created bool, err error) {
	if m == nil || m.factory == nil {
		return false, ErrFactoryRequired
	}
	normalized, err := request.Normalized()
	if err != nil {
		return false, err
	}

	for {
		candidate := newManagedWindow(normalized)
		_, actual, loaded, err := m.windows.loadOrStore(normalized, candidate)
		if err != nil {
			return false, err
		}
		if loaded {
			if startErr := actual.waitUntilReady(); startErr != nil {
				return false, startErr
			}
			if actual.raise() {
				return false, nil
			}
			// Run has returned but its deferred registry removal has not yet
			// completed. Remove the stale value and retry atomically.
			m.windows.delete(normalized, actual)
			continue
		}

		view, createErr := m.factory.NewView(normalized)
		if createErr != nil {
			candidate.completeStart(createErr)
			m.windows.delete(normalized, candidate)
			return false, createErr
		}
		if view == nil {
			candidate.completeStart(ErrNilView)
			m.windows.delete(normalized, candidate)
			return false, ErrNilView
		}
		candidate.completeStart(nil)
		go m.run(candidate, view)
		return true, nil
	}
}

// Close requests that a registered window close. The registration is retained
// until its View.Run method returns.
func (m *Manager) Close(role Role, workspaceID string) bool {
	if m == nil || m.windows == nil {
		return false
	}
	window, ok := m.windows.load(role, workspaceID)
	if !ok {
		return false
	}
	return window.close()
}

// CloseAll requests closure of every registered window. It returns the number
// of live or starting windows that accepted the request.
func (m *Manager) CloseAll() int {
	if m == nil || m.windows == nil {
		return 0
	}
	closed := 0
	for _, window := range m.windows.values() {
		if window.close() {
			closed++
		}
	}
	return closed
}

// InvalidateAll requests a fresh frame from every live window. It returns the
// number of windows that received the request.
func (m *Manager) InvalidateAll() int {
	if m == nil || m.windows == nil {
		return 0
	}
	invalidated := 0
	for _, window := range m.windows.values() {
		if window.invalidate() {
			invalidated++
		}
	}
	return invalidated
}

// Count returns the number of registered, including starting, windows.
func (m *Manager) Count() int {
	if m == nil || m.windows == nil {
		return 0
	}
	return m.windows.count()
}

// Requests returns normalized request snapshots sorted by role and workspace.
func (m *Manager) Requests() []Request {
	if m == nil || m.windows == nil {
		return nil
	}
	return m.windows.requests()
}

func (m *Manager) run(window *managedWindow, view View) {
	var runErr error
	defer func() {
		window.finish()
		m.windows.delete(window.request, window)
		if runErr != nil && m.onError != nil {
			m.onError(window.request, runErr)
		}
	}()

	nativeWindow := new(app.Window)
	nativeWindow.Option(windowOptions(window.request)...)
	window.bind(nativeWindow)
	runErr = view.Run(nativeWindow, window)
}

func windowOptions(request Request) []app.Option {
	options := []app.Option{app.Title(request.Title)}
	if request.Size.configured() {
		options = append(options, app.Size(request.Size.Width, request.Size.Height))
	}
	if request.MinSize.configured() {
		options = append(options, app.MinSize(request.MinSize.Width, request.MinSize.Height))
	}
	if request.TopMost {
		options = append(options, app.TopMost(true))
	}
	options = append(options, platformWindowOptions()...)
	return options
}

type managedWindow struct {
	request Request
	ready   chan struct{}

	mu          sync.Mutex
	window      *app.Window
	startErr    error
	readyClosed bool
	finished    bool
	closing     bool
	pending     system.Action
	waking      bool
}

func newManagedWindow(request Request) *managedWindow {
	return &managedWindow{request: request, ready: make(chan struct{})}
}

func (w *managedWindow) completeStart(err error) {
	w.mu.Lock()
	w.startErr = err
	if err != nil {
		w.finished = true
	}
	if !w.readyClosed {
		close(w.ready)
		w.readyClosed = true
	}
	w.mu.Unlock()
}

func (w *managedWindow) waitUntilReady() error {
	<-w.ready
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.startErr
}

func (w *managedWindow) bind(window *app.Window) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.finished {
		return
	}
	w.window = window
}

func (w *managedWindow) raise() bool {
	w.mu.Lock()
	if w.finished {
		w.mu.Unlock()
		return false
	}
	if w.closing {
		w.mu.Unlock()
		return true
	}
	w.pending |= system.ActionRaise
	w.ensurePendingActionWakeLocked()
	w.mu.Unlock()
	return true
}

func (w *managedWindow) close() bool {
	w.mu.Lock()
	if w.finished || w.closing {
		w.mu.Unlock()
		return false
	}
	w.closing = true
	// Closing supersedes a raise that has not reached the event goroutine yet.
	w.pending &^= system.ActionRaise
	w.pending |= system.ActionClose
	w.ensurePendingActionWakeLocked()
	w.mu.Unlock()
	return true
}

// Gio coalesces Invalidate calls while a frame is in flight. Keep requesting a
// wake until the view takes the pending action, otherwise a close or raise that
// lands at the frame/Event boundary can remain queued indefinitely.
func (w *managedWindow) ensurePendingActionWakeLocked() {
	if w.finished || w.waking {
		return
	}
	w.waking = true
	go w.wakePendingActions()
}

func (w *managedWindow) wakePendingActions() {
	ticker := time.NewTicker(pendingActionWakeInterval)
	defer ticker.Stop()
	for {
		w.mu.Lock()
		if w.finished || w.pending == 0 {
			w.waking = false
			w.mu.Unlock()
			return
		}
		window := w.window
		w.mu.Unlock()
		if window != nil {
			window.Invalidate()
		}
		<-ticker.C
	}
}

// Take implements WindowActions. The corresponding View.Run event goroutine is
// the sole consumer; it decides how to dispatch the returned native actions.
func (w *managedWindow) Take() system.Action {
	w.mu.Lock()
	actions := w.pending
	w.pending = 0
	w.mu.Unlock()
	return actions
}

func (w *managedWindow) invalidate() bool {
	w.mu.Lock()
	if w.finished || w.closing || w.window == nil {
		w.mu.Unlock()
		return false
	}
	window := w.window
	w.mu.Unlock()
	window.Invalidate()
	return true
}

func (w *managedWindow) finish() {
	w.mu.Lock()
	w.finished = true
	w.window = nil
	w.pending = 0
	w.waking = false
	w.mu.Unlock()
}

var _ WindowActions = (*managedWindow)(nil)
