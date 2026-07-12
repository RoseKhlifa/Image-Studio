# Gio UI Maintainer Guide

This directory owns the immediate-mode Gio presentation layer and the main UI
actor. It renders the simple and workflow experiences, translates UI events into
application mutations, publishes read-only desktop snapshots, and persists Gio
desktop preferences through `internal/desktopstate`.

It does not own native multi-window registration. Process-level window creation,
deduplication, and lifecycle belong to `internal/windowing` and the process
bootstrap that supplies a fresh `windowing.View` for every request.

## Experience routing

The primary window loop starts in `App.Run` and renders through
[`layout_shell.go`](layout_shell.go):

1. Every non-destroy window event lets `desktopSessionActor` drain detached
   commands and publish a fresh immutable snapshot. This also handles inactive
   window wakeups that do not produce a `FrameEvent`.
2. On a `FrameEvent`, the shared header renders `layoutExperienceSwitch`, which persists `simple`
   or `workflow` through [`desktop_preferences.go`](desktop_preferences.go).
3. `layoutBody` routes workflow mode to `layoutWorkflowShell`. Simple mode keeps
   the existing controls, result canvas, and history rail layout.
4. Full-screen mode always routes directly to the result canvas, regardless of
   the selected experience.
5. Common modals are rendered after the selected body.
6. Snapshot publication invalidates registered detached windows after command
   processing and state updates; rendering itself remains frame-local.

The switch changes the presentation and persisted startup mode. It does not
replace the request kernel, compatibility history, profiles, or active job.

## File ownership

The simple experience remains split across `layout_controls.go`,
`layout_canvas.go`, `layout_history.go`, and the focused modal/widget files.

The workflow experience is divided as follows:

| File | Responsibility |
| --- | --- |
| [`workflow_model.go`](workflow_model.go) | Typed node, port, edge, and graph model; default graph; cloning; connection validation; cycle detection; node movement. |
| [`workflow_state.go`](workflow_state.go) | Per-workspace graph selection and runtime projection from the active generation state. |
| [`layout_workflow_shell.go`](layout_workflow_shell.go) | Workflow command bar, three-pane body, center canvas/dock composition, and command button events. |
| [`layout_workflow_library.go`](layout_workflow_library.go) | Workspace list and node library in the left pane. |
| [`layout_workflow_canvas.go`](layout_workflow_canvas.go) | Window-local graph viewport, pan/zoom, node interaction, edges, ports, and runtime badges. |
| [`layout_workflow_inspector.go`](layout_workflow_inspector.go) | Selected-node settings and the bridge to existing generation controls. |
| [`layout_workflow_console.go`](layout_workflow_console.go) | Queue, logs, errors, and artifacts in the bottom dock. |
| [`desktop_publication.go`](desktop_publication.go) | Immutable-by-copy desktop publication and the detached-window command queue. |
| [`desktop_window_contract.go`](desktop_window_contract.go) | Narrow window-controller interface plus role-specific requests, titles, and sizes. |
| [`desktop_preferences.go`](desktop_preferences.go) | `desktop-state.json` load/save, workspace conversion, window restoration, and desktopstate/windowing role mapping. |
| [`layout_desktop_settings.go`](layout_desktop_settings.go) | Desktop style, experience, default layout, restore, reopen, and auto-progress settings. |

Do not add workflow behavior to `app.go` or `layout_shell.go` when one of these
focused owners can contain it.

## Main actor and desktop publication

`App` remains the command actor for mutable UI/application state. Detached
windows must not call workspace, job, graph, editor, or result mutators directly.
The cross-window boundary is:

```text
primary App actor --publishes--> desktopPublication --read by--> detached View
primary App actor <--commands---- desktopCommand <----enqueued by-- detached View
```

`publishDesktopState` builds a cloned publication containing style, color mode,
experience, job status, logs, and per-workspace graph/runtime/result data.
`desktopSnapshot` returns another clone under `desktopPublishMu`, so a detached
view cannot mutate the actor's slices or maps.

Detached views send intent through `enqueueDesktopCommand`. The channel is
bounded to 256 entries and enqueue is non-blocking; callers must respect a false
return instead of waiting on the render thread. Enqueue requests an event-loop
wakeup, and `desktopSessionActor` drains at most 256 commands during the next
non-destroy main-window event before publishing the resulting snapshot. This
keeps commands moving while the main window is inactive or minimized.
Supported commands currently cover workspace activation, run/cancel, log clear,
node selection/movement, detached-window open, and raising the primary window.

Add new cross-window behavior by adding one command kind and handling it on the
primary actor. Do not expose an App pointer as a general detached-view API.

## Per-window ownership

Every top-level `app.Window` requires a fresh View and event loop. Each View must
own its own:

- `op.Ops` and frame context;
- every `widget.Clickable`, `widget.Editor`, `widget.List`, drag gesture, focus
  tag, and pointer tag;
- `material.Theme` and text `Shaper`;
- canvas viewport, hover, selection, and drag state;
- window-local `paint.ImageOp` and other render caches.

Never share the primary `*App`, widget values, a `material.Theme`, or a Shaper
between window event goroutines. Widget event queues and text shaping state are
not a cross-window state bus. Share only immutable publications, durable artifact
references, and explicitly concurrency-safe services.

## Theme tokens

[`theme_tokens.go`](theme_tokens.go) defines semantic colors and metrics shared
by both experiences. [`theme_macos.go`](theme_macos.go) and
[`theme_windows_style.go`](theme_windows_style.go) provide complete light/dark
token values. macOS and Windows are selectable Gio visual languages, not native
AppKit or WinUI control implementations.

`desktopThemeSpec` returns tokens by value. A detached View should resolve and
retain its own value copy and use it to construct its own `material.Theme` and
Shaper. Do not share pointers to token state between windows. The package-level
`fluent` and `installedDesktopTheme` values are a transitional compatibility
bridge for the primary layout while older widgets migrate to semantic tokens;
they are not per-window mutable theme storage.

Style is currently a process-wide desktop preference. When it changes, the main
actor installs the new compatibility tokens and invalidates detached windows so
they can resolve the new style from the next publication.

## Detached roles

`internal/windowing` currently accepts four top-level roles. Role plus normalized
workspace ID is the window identity; opening the same identity raises the
registered window instead of creating a duplicate.

| Role | Default request | Purpose |
| --- | --- | --- |
| `canvas` | 1180 x 820 dp | Detached workflow canvas for one workspace. |
| `console` | 880 x 620 dp | Queue, log, error, and artifact monitoring. |
| `progress` | 420 x 260 dp | Compact task status and preview. |
| `workspace` | 1280 x 860 dp | Another full workspace surface. |

`single` opens no detached role. `dual` opens canvas and console. `multi` also
opens progress. When workflow mode starts a job and `AutoShowProgress` is true,
`run_job.go` requests the progress role automatically. Persisted
`desktopstate.WindowRoleWorkflow` restores through the canvas role.

`DesktopWindowController` is the UI-side injection contract. A process-level
`windowing.Manager` implements it, while its Factory must return a new View with
the window-local ownership described above. Closing a detached View must return
from its event loop; it must not cancel the shared job or terminate the process.

## Dependency boundary

```text
internal/ui ----------> internal/desktopstate (Gio-only persisted documents)
     |---------------> internal/windowing    (window coordination contract)

internal/desktopstate -> standard library only
internal/windowing ----> Gio app/window APIs, never internal/ui
```

`desktopstate` owns preferences, window descriptors, typed workspace drafts,
stable result references, and lightweight persisted graphs. It owns no secrets,
image bytes, widgets, jobs, or runtime publications. `windowing` owns registration
and native window loops, not workspace state or layout. Keep these directions
acyclic.

## Extending the UI

### Add a workflow node

1. Add the node kind and compatible ports in `workflow_model.go`, then include
   the node and any default edges in `defaultWorkflowGraph`.
2. Add active runtime projection in `workflowCanvasData` and inactive projection
   in `workflowRuntimeForInactiveWorkspace`.
3. Add the selected-node editor in `layout_workflow_inspector.go`. The library
   and generic canvas render nodes from the graph automatically.
4. Update `desktopWorkflowGraph` and `workflowGraphFromDesktop` if the node needs
   persisted properties beyond identity and position.
5. Extend `workflow_model_test.go` with valid ports, invalid connections, cycle,
   normalization, and movement coverage.

The current graph is a fixed product graph, not a plugin registry.
`workflowGraphFromDesktop` starts from `defaultWorkflowGraph` and restores known
node positions and edges. A persisted unknown node alone does not make a new
runtime node available. Although `desktopstate.WorkflowGraph` has a Viewport
field, the current UI bridge does not round-trip it; pan/zoom remains
window-local `workflowCanvasViewState` until an explicit per-window persistence
mapping is added.

### Add a detached window role

1. Add the role, validation, and default title in `internal/windowing/types.go`.
2. Add the persisted role and default dimensions in
   `internal/desktopstate/model.go`.
3. Extend `desktopWindowRole`, `windowingRole`, `desktopWindowRequest`, and
   `desktopWindowRoleLabel` in the desktop preference/window contract files.
4. Add a fresh View implementation to the process Factory. Keep every widget,
   theme, Shaper, Ops list, and interaction cache local to that View.
5. Add settings/command entry points only where users need them, then cover role
   normalization, deduplication, close, restore, and error handling in tests.

### Add a desktop setting

1. Add the typed JSON field to `desktopstate.Preferences`, its default and any
   normalization, plus store/model tests. Missing fields decode over `Default()`,
   so choose backward-compatible defaults deliberately.
2. Load and save it through `desktop_preferences.go`.
3. Add App-owned clickables in `app.go`, initialize them in `New`, handle events
   and render controls in `layout_desktop_settings.go`.
4. Invalidate affected windows after a change. Add a desktop command only when a
   detached window must mutate the setting.
5. Update the desktopstate format example and bump the schema only for an
   incompatible format change.

## Tests

Run from `gio-client` with writable Go caches:

```bash
go test ./internal/ui ./internal/desktopstate ./internal/windowing
go test -race ./internal/desktopstate ./internal/windowing ./internal/ui
go test ./...
```

Focused coverage lives in `workflow_model_test.go`, `theme_tokens_test.go`,
`internal/desktopstate/*_test.go`, and `internal/windowing/*_test.go`. Windowing
manager tests use fake Views and must not create native windows.

## Pure Gio limits

- `windowing.Request` intentionally has no X/Y coordinates. Gio has no portable
  monitor-placement API, and Wayland clients cannot choose absolute positions.
- Gio exposes no portable owner/child relationship for these windows. Detached
  roles are independent top-level windows, not owned dialogs or native child
  windows.
- Gio v0.10 implements `TopMost` only on macOS. The progress request enables the
  hint only on Darwin; Windows and Linux cannot rely on an always-on-top preview
  without a separate platform adapter.
- `ActionRaise` is best effort and may be ignored by the desktop window manager.
- Window decorations, native materials, and platform behaviors remain controlled
  by Gio and the OS. The macOS/Windows token sets style the content surface only.
