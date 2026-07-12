# Gio Window Coordination

`windowing` manages independent Gio desktop windows without owning application
state or a window's immediate-mode layout. It is intended for detached canvas,
console, progress, and workspace windows on Windows, macOS, and Linux.

## Integration

Create one manager for the process and provide a factory that returns a fresh
view for every normalized request:

```go
manager := windowing.NewManager(
	windowing.FactoryFunc(func(req windowing.Request) (windowing.View, error) {
		return newWorkspaceView(core, req.Role, req.WorkspaceID), nil
	}),
	func(req windowing.Request, err error) {
		reportWindowError(req, err)
	},
)

created, err := manager.Open(windowing.Request{
	Role:        windowing.RoleCanvas,
	WorkspaceID: workspaceID,
	Title:       "Image Studio - Canvas",
	Size:        windowing.DpSize{Width: 1100, Height: 800},
	MinSize:     windowing.DpSize{Width: 640, Height: 480},
	TopMost:     false,
})
```

`Open` returns `created=false` and raises the existing window when the same
normalized role and workspace ID are already registered. The original title
and options remain in effect.

Each `View` owns one event loop, consumes manager actions on that same
goroutine, and returns after `app.DestroyEvent`:

```go
func (v *CanvasView) Run(window *app.Window, actions windowing.WindowActions) error {
	var ops op.Ops
	for {
		event := window.Event()
		if _, destroying := event.(app.DestroyEvent); !destroying {
			if pending := actions.Take(); pending != 0 {
				window.Perform(pending)
			}
		}
		switch event := event.(type) {
		case app.DestroyEvent:
			return event.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			v.Layout(gtx)
			event.Frame(gtx.Ops)
		}
	}
}
```

`Manager.Open` duplicate raises and `Manager.Close` calls only merge actions
into `WindowActions` and retry `app.Window.Invalidate`, which Gio documents as
safe for concurrent use, until the view takes the action. They never call
`app.Window.Perform` from the manager caller's goroutine. The view calls `Take`
after receiving a non-destroy event, so the native driver is initialized first.
All actions stay on the `Run` goroutine. The repository's local Gio compatibility
patch snapshots the native driver under its invalidation lock and defers macOS
native close delivery to the next main-loop turn, so `Perform(ActionClose)` can
return before `Run` waits for `DestroyEvent`. Repeated raises are coalesced, and
a queued close supersedes an unconsumed raise.

Open initial windows in goroutines through the manager, then call `app.Main()`
or `app.Events(...)` exactly once from the program's main goroutine. The manager
creates a distinct zero-value `app.Window` and goroutine for every registration.

On macOS, the manager marks only these managed detached windows as fullscreen
auxiliaries. The local Gio fork maps that opt-in to
`NSWindowCollectionBehaviorMoveToActiveSpace | NSWindowCollectionBehaviorFullScreenAuxiliary`,
so a canvas, console, progress, or workspace window moves to the current Space
when shown and can coexist with the primary fullscreen window there. The
primary window is created outside this manager and retains AppKit's normal
fullscreen behavior.

Useful process-level operations are:

```go
manager.Close(windowing.RoleCanvas, workspaceID)
manager.CloseAll()
manager.InvalidateAll()
count := manager.Count()
requests := manager.Requests()
```

`Close` and `CloseAll` queue `system.ActionClose`; registration is removed only
after `View.Run` returns. Non-nil `Run` errors are delivered to the error handler
after removal. The manager never logs fatally or exits the process.

## Platform Boundary

Requests intentionally contain no window coordinates. Gio does not expose a
portable window-position or monitor-placement API, and Wayland applications
cannot select absolute positions. `TopMost` is a platform hint and Gio v0.10.1
only implements it on macOS. `ActionRaise` is best effort and may be ignored by
the window manager. AppKit still owns focus, ordering, Mission Control
placement, and monitor selection. The integration deliberately uses
`NSWindowCollectionBehaviorMoveToActiveSpace` instead of `CanJoinAllSpaces`:
the two Space-placement policies are mutually exclusive, and detached work
surfaces should move when shown rather than remain visible in every Space.
Layout state, widgets, `op.Ops`, themes, and text shapers
must remain local to each `View`; shared workspace state belongs in a separate
concurrency-safe application core.
