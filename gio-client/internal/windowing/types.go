package windowing

import (
	"errors"
	"fmt"
	"strings"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/unit"
)

// Role identifies the purpose of a top-level desktop window.
type Role string

const (
	RoleCanvas    Role = "canvas"
	RoleConsole   Role = "console"
	RoleProgress  Role = "progress"
	RoleWorkspace Role = "workspace"
)

var (
	ErrFactoryRequired = errors.New("windowing: factory is required")
	ErrNilView         = errors.New("windowing: factory returned a nil view")
)

// DpSize is a window size expressed in Gio density-independent pixels.
// The zero value leaves the corresponding platform default unchanged.
type DpSize struct {
	Width  unit.Dp
	Height unit.Dp
}

func (s DpSize) configured() bool {
	return s.Width != 0 || s.Height != 0
}

func (s DpSize) validate(name string) error {
	if !s.configured() {
		return nil
	}
	if s.Width <= 0 || s.Height <= 0 {
		return fmt.Errorf("windowing: %s width and height must both be positive", name)
	}
	return nil
}

// Request describes one independently managed top-level window.
//
// Role and WorkspaceID form its identity. Title and window options do not
// replace an existing window; opening the same identity raises that window.
type Request struct {
	Role        Role
	WorkspaceID string
	Title       string
	Size        DpSize
	MinSize     DpSize
	TopMost     bool
}

// Key is the normalized identity used to deduplicate windows.
type Key struct {
	Role        Role
	WorkspaceID string
}

func (k Key) String() string {
	if k.WorkspaceID == "" {
		return string(k.Role)
	}
	return string(k.Role) + ":" + k.WorkspaceID
}

// Normalized validates a request and returns its canonical representation.
func (r Request) Normalized() (Request, error) {
	role, err := normalizeRole(r.Role)
	if err != nil {
		return Request{}, err
	}
	if err := r.Size.validate("size"); err != nil {
		return Request{}, err
	}
	if err := r.MinSize.validate("minimum size"); err != nil {
		return Request{}, err
	}
	r.Role = role
	r.WorkspaceID = strings.TrimSpace(r.WorkspaceID)
	r.Title = strings.TrimSpace(r.Title)
	if r.Title == "" {
		r.Title = defaultTitle(role)
	}
	return r, nil
}

// Key returns the normalized identity for the request.
func (r Request) Key() (Key, error) {
	normalized, err := r.Normalized()
	if err != nil {
		return Key{}, err
	}
	return normalized.key(), nil
}

func (r Request) key() Key {
	return Key{Role: r.Role, WorkspaceID: r.WorkspaceID}
}

func normalizeRole(role Role) (Role, error) {
	role = Role(strings.ToLower(strings.TrimSpace(string(role))))
	switch role {
	case RoleCanvas, RoleConsole, RoleProgress, RoleWorkspace:
		return role, nil
	default:
		return "", fmt.Errorf("windowing: unsupported role %q", role)
	}
}

func defaultTitle(role Role) string {
	switch role {
	case RoleCanvas:
		return "Image Studio Canvas"
	case RoleConsole:
		return "Image Studio Console"
	case RoleProgress:
		return "Image Studio Progress"
	case RoleWorkspace:
		return "Image Studio Workspace"
	default:
		return "Image Studio"
	}
}

// WindowActions is the concurrency-safe handoff from Manager to a View's
// window event goroutine. Take atomically returns and clears the merged pending
// actions. Only the View.Run goroutine should call Take and Perform them.
type WindowActions interface {
	Take() system.Action
}

// View owns the event and frame loop for one app.Window. Run must perform
// pending actions after native window initialization and return after receiving
// app.DestroyEvent so Manager can unregister the window.
type View interface {
	Run(*app.Window, WindowActions) error
}

// ViewFunc adapts a function to View.
type ViewFunc func(*app.Window, WindowActions) error

func (f ViewFunc) Run(window *app.Window, actions WindowActions) error {
	return f(window, actions)
}

// Factory creates a fresh, window-local View for a normalized Request.
type Factory interface {
	NewView(Request) (View, error)
}

// FactoryFunc adapts a function to Factory.
type FactoryFunc func(Request) (View, error)

func (f FactoryFunc) NewView(request Request) (View, error) {
	return f(request)
}

// ErrorHandler receives non-nil errors returned by View.Run. It is called from
// the corresponding window goroutine after the window has been unregistered.
type ErrorHandler func(Request, error)
