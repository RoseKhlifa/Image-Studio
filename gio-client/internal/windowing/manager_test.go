package windowing

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/unit"
)

type blockingView struct {
	started chan struct{}
	release chan struct{}
	err     error
}

func (v *blockingView) Run(_ *app.Window, _ WindowActions) error {
	close(v.started)
	<-v.release
	return v.err
}

type actionDrainRequest struct {
	result chan system.Action
}

type actionHandoffView struct {
	started chan struct{}
	drain   chan actionDrainRequest
	release chan struct{}
}

func newActionHandoffView() *actionHandoffView {
	return &actionHandoffView{
		started: make(chan struct{}),
		drain:   make(chan actionDrainRequest),
		release: make(chan struct{}),
	}
}

func (v *actionHandoffView) Run(_ *app.Window, actions WindowActions) error {
	close(v.started)
	for {
		select {
		case request := <-v.drain:
			request.result <- actions.Take()
		case <-v.release:
			return nil
		}
	}
}

func (v *actionHandoffView) take(t *testing.T) system.Action {
	t.Helper()
	result := make(chan system.Action, 1)
	select {
	case v.drain <- actionDrainRequest{result: result}:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out requesting pending window actions")
	}
	select {
	case actions := <-result:
		return actions
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pending window actions")
		return 0
	}
}

func TestManagerUnregistersAfterRunReturnsAndReportsError(t *testing.T) {
	runErr := errors.New("view stopped")
	view := &blockingView{
		started: make(chan struct{}),
		release: make(chan struct{}),
		err:     runErr,
	}
	type errorReport struct {
		request Request
		err     error
		count   int
	}
	reports := make(chan errorReport, 1)
	var manager *Manager
	manager = NewManager(
		FactoryFunc(func(Request) (View, error) { return view, nil }),
		func(request Request, err error) {
			reports <- errorReport{request: request, err: err, count: manager.Count()}
		},
	)

	created, err := manager.Open(Request{Role: RoleCanvas, WorkspaceID: " ws-1 "})
	if err != nil || !created {
		t.Fatalf("Open created=%v err=%v", created, err)
	}
	waitForSignal(t, view.started, "view start")
	if manager.Count() != 1 {
		t.Fatalf("count while Run is active=%d want 1", manager.Count())
	}
	close(view.release)

	select {
	case report := <-reports:
		if !errors.Is(report.err, runErr) {
			t.Fatalf("reported error=%v want %v", report.err, runErr)
		}
		if report.request.Role != RoleCanvas || report.request.WorkspaceID != "ws-1" {
			t.Fatalf("reported request=%+v", report.request)
		}
		if report.count != 0 {
			t.Fatalf("count inside error callback=%d want 0", report.count)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run error callback")
	}
	if manager.Count() != 0 {
		t.Fatalf("count after Run returned=%d want 0", manager.Count())
	}
}

func TestManagerOpenDeduplicatesAndCloseRetainsRegistrationUntilRunReturns(t *testing.T) {
	view := &blockingView{started: make(chan struct{}), release: make(chan struct{})}
	var factoryCalls atomic.Int64
	manager := NewManager(FactoryFunc(func(Request) (View, error) {
		factoryCalls.Add(1)
		return view, nil
	}), nil)

	created, err := manager.Open(Request{Role: RoleConsole, WorkspaceID: "ws-2"})
	if err != nil || !created {
		t.Fatalf("first Open created=%v err=%v", created, err)
	}
	waitForSignal(t, view.started, "view start")
	created, err = manager.Open(Request{Role: Role(" CONSOLE "), WorkspaceID: " ws-2 "})
	if err != nil || created {
		t.Fatalf("duplicate Open created=%v err=%v", created, err)
	}
	if calls := factoryCalls.Load(); calls != 1 {
		t.Fatalf("factory calls=%d want 1", calls)
	}
	if !manager.Close(RoleConsole, " ws-2 ") {
		t.Fatal("Close did not find the registered window")
	}
	if manager.Count() != 1 {
		t.Fatalf("count after Close before Run return=%d want 1", manager.Count())
	}
	if manager.Close(RoleConsole, "ws-2") {
		t.Fatal("second Close should not be accepted")
	}
	close(view.release)
	waitForCount(t, manager, 0)
}

func TestManagerBatchOperationsDoNotCreateNativeWindows(t *testing.T) {
	views := make(chan *blockingView, 2)
	manager := NewManager(FactoryFunc(func(Request) (View, error) {
		view := &blockingView{started: make(chan struct{}), release: make(chan struct{})}
		views <- view
		return view, nil
	}), nil)
	for _, request := range []Request{
		{Role: RoleProgress, WorkspaceID: "ws-1"},
		{Role: RoleProgress, WorkspaceID: "ws-2"},
	} {
		created, err := manager.Open(request)
		if err != nil || !created {
			t.Fatalf("Open(%+v) created=%v err=%v", request, created, err)
		}
	}
	first, second := <-views, <-views
	waitForSignal(t, first.started, "first view start")
	waitForSignal(t, second.started, "second view start")
	if invalidated := manager.InvalidateAll(); invalidated != 2 {
		t.Fatalf("InvalidateAll=%d want 2", invalidated)
	}
	if closed := manager.CloseAll(); closed != 2 {
		t.Fatalf("CloseAll=%d want 2", closed)
	}
	if manager.Count() != 2 {
		t.Fatalf("count before Run returns=%d want 2", manager.Count())
	}
	close(first.release)
	close(second.release)
	waitForCount(t, manager, 0)
}

func TestManagerHandsRaiseAndCloseToViewRun(t *testing.T) {
	view := newActionHandoffView()
	manager := NewManager(FactoryFunc(func(Request) (View, error) {
		return view, nil
	}), nil)
	request := Request{Role: RoleCanvas, WorkspaceID: "ws-actions"}

	created, err := manager.Open(request)
	if err != nil || !created {
		t.Fatalf("first Open created=%v err=%v", created, err)
	}
	waitForSignal(t, view.started, "action view start")
	created, err = manager.Open(request)
	if err != nil || created {
		t.Fatalf("duplicate Open created=%v err=%v", created, err)
	}
	if actions := view.take(t); actions != system.ActionRaise {
		t.Fatalf("actions after duplicate Open=%v want %v", actions, system.ActionRaise)
	}
	if !manager.Close(RoleCanvas, request.WorkspaceID) {
		t.Fatal("Close did not queue an action")
	}
	if actions := view.take(t); actions != system.ActionClose {
		t.Fatalf("actions after Close=%v want %v", actions, system.ActionClose)
	}
	close(view.release)
	waitForCount(t, manager, 0)
}

func TestManagerCloseSupersedesUnconsumedRaise(t *testing.T) {
	view := newActionHandoffView()
	manager := NewManager(FactoryFunc(func(Request) (View, error) {
		return view, nil
	}), nil)
	request := Request{Role: RoleConsole, WorkspaceID: "ws-close"}
	created, err := manager.Open(request)
	if err != nil || !created {
		t.Fatalf("first Open created=%v err=%v", created, err)
	}
	waitForSignal(t, view.started, "close view start")
	created, err = manager.Open(request)
	if err != nil || created {
		t.Fatalf("duplicate Open created=%v err=%v", created, err)
	}
	if !manager.Close(RoleConsole, request.WorkspaceID) {
		t.Fatal("Close did not queue an action")
	}
	if actions := view.take(t); actions != system.ActionClose {
		t.Fatalf("actions=%v want only %v", actions, system.ActionClose)
	}
	close(view.release)
	waitForCount(t, manager, 0)
}

func TestWindowOptionsScopeFullscreenAuxiliaryToDarwin(t *testing.T) {
	var config app.Config
	for _, option := range windowOptions(Request{Role: RoleCanvas, Title: "Canvas"}) {
		option(unit.Metric{}, &config)
	}
	want := runtime.GOOS == "darwin"
	if config.FullscreenAuxiliary != want {
		t.Fatalf("FullscreenAuxiliary=%v want %v on %s", config.FullscreenAuxiliary, want, runtime.GOOS)
	}
}

func TestManagerConcurrentDuplicateOpenMergesRaiseWithoutPerformRace(t *testing.T) {
	view := newActionHandoffView()
	var factoryCalls atomic.Int64
	manager := NewManager(FactoryFunc(func(Request) (View, error) {
		factoryCalls.Add(1)
		return view, nil
	}), nil)
	request := Request{Role: RoleWorkspace, WorkspaceID: "ws-race"}
	created, err := manager.Open(request)
	if err != nil || !created {
		t.Fatalf("first Open created=%v err=%v", created, err)
	}
	waitForSignal(t, view.started, "race view start")

	const goroutines = 24
	const opensPerGoroutine = 40
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range opensPerGoroutine {
				created, err := manager.Open(request)
				if err != nil {
					errs <- err
					return
				}
				if created {
					errs <- errors.New("duplicate Open unexpectedly created a window")
					return
				}
				manager.InvalidateAll()
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if calls := factoryCalls.Load(); calls != 1 {
		t.Fatalf("factory calls=%d want 1", calls)
	}
	if actions := view.take(t); actions != system.ActionRaise {
		t.Fatalf("merged actions=%v want %v", actions, system.ActionRaise)
	}

	if !manager.Close(RoleWorkspace, request.WorkspaceID) {
		t.Fatal("Close did not queue an action")
	}
	if actions := view.take(t); actions != system.ActionClose {
		t.Fatalf("close actions=%v want %v", actions, system.ActionClose)
	}
	close(view.release)
	waitForCount(t, manager, 0)
}

func waitForSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func waitForCount(t *testing.T, manager *Manager, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if manager.Count() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("manager count=%d want %d", manager.Count(), want)
}
