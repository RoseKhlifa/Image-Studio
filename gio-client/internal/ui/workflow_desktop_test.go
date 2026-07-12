package ui

import (
	"image"
	"path/filepath"
	"testing"

	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/desktopstate"
	"image-studio/gio-client/internal/windowing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type fakeDesktopWindowController struct {
	opened []windowing.Request
}

func (fake *fakeDesktopWindowController) Open(request windowing.Request) (bool, error) {
	fake.opened = append(fake.opened, request)
	return true, nil
}

func (*fakeDesktopWindowController) CloseAll() int      { return 0 }
func (*fakeDesktopWindowController) InvalidateAll() int { return 0 }
func (fake *fakeDesktopWindowController) Count() int    { return len(fake.opened) }
func (fake *fakeDesktopWindowController) Requests() []windowing.Request {
	return append([]windowing.Request(nil), fake.opened...)
}

func TestWorkflowCanvasHeadlessLayoutUsesStableViewport(t *testing.T) {
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(1280, 760)),
	}
	view := workflowCanvasViewState{}
	graph := defaultWorkflowGraph()
	runtime := map[string]workflowNodeRuntime{
		"generate": {Phase: workflowNodePhaseRunning, Detail: "running", Progress: 0.4},
	}
	dims := view.Layout(gtx, material.NewTheme(), desktopThemeSpec(desktopStyleWindows, desktopColorModeDark), workflowCanvasData{
		Graph:    graph,
		Selected: "generate",
		Runtime:  runtime,
	}, workflowCanvasCallbacks{})
	if dims.Size != image.Pt(1280, 760) {
		t.Fatalf("canvas size=%v", dims.Size)
	}
	if len(view.interactions) != len(graph.Nodes) {
		t.Fatalf("node interactions=%d want %d", len(view.interactions), len(graph.Nodes))
	}
	if view.zoom != 1 {
		t.Fatalf("initial zoom=%v", view.zoom)
	}
}

func TestDesktopSnapshotReturnsDeepCopies(t *testing.T) {
	app := &App{desktopPublished: desktopPublication{
		Revision: 7,
		Logs:     []string{"one"},
		Workspaces: []desktopWorkspacePublication{{
			ID:      "ws-1",
			Graph:   defaultWorkflowGraph(),
			Runtime: map[string]workflowNodeRuntime{"generate": {Detail: "ready"}},
		}},
	}}
	first := app.desktopSnapshot()
	first.Logs[0] = "changed"
	first.Workspaces[0].Graph.Nodes[0].Title = "changed"
	first.Workspaces[0].Runtime["generate"] = workflowNodeRuntime{Detail: "changed"}
	second := app.desktopSnapshot()
	if second.Logs[0] != "one" {
		t.Fatalf("logs alias: %#v", second.Logs)
	}
	if second.Workspaces[0].Graph.Nodes[0].Title == "changed" {
		t.Fatal("workflow graph aliases publication state")
	}
	if second.Workspaces[0].Runtime["generate"].Detail != "ready" {
		t.Fatal("runtime map aliases publication state")
	}
}

func TestDesktopCommandRoutesWorkflowSelectionAndMovement(t *testing.T) {
	app := &App{
		activeWorkspaceID:     "ws-1",
		workspaces:            []workspaceState{{ID: "ws-1", Name: "Workspace"}},
		workflowGraphs:        map[string]workflowGraphModel{"ws-1": defaultWorkflowGraph()},
		workflowSelectedNodes: map[string]string{"ws-1": "generate"},
	}
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandSelectNode, WorkspaceID: "ws-1", NodeID: "prompt"})
	if got := app.selectedWorkflowNode("ws-1"); got != "prompt" {
		t.Fatalf("selected=%q", got)
	}
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandMoveNode, WorkspaceID: "ws-1", NodeID: "prompt", Position: image.Pt(333, 444)})
	node, _ := app.workflowGraph("ws-1").node("prompt")
	if node.Position != image.Pt(333, 444) {
		t.Fatalf("position=%v", node.Position)
	}
}

func TestDesktopMoveCommandsCoalesceToFinalPosition(t *testing.T) {
	app := &App{
		activeWorkspaceID:     "ws-1",
		workspaces:            []workspaceState{{ID: "ws-1", Name: "Workspace"}},
		workflowGraphs:        map[string]workflowGraphModel{"ws-1": defaultWorkflowGraph()},
		workflowSelectedNodes: map[string]string{"ws-1": "prompt"},
		desktopCommands:       make(chan desktopCommand, 1),
		desktopPendingMoves:   map[string]desktopCommand{},
	}
	for index := 0; index < 600; index++ {
		if !app.enqueueDesktopCommand(desktopCommand{
			Kind:        desktopCommandMoveNode,
			WorkspaceID: "ws-1",
			NodeID:      "prompt",
			Position:    image.Pt(index, index+1),
		}) {
			t.Fatalf("move %d was rejected", index)
		}
	}
	if len(app.desktopCommands) != 0 {
		t.Fatalf("move commands should not flood main command channel: %d", len(app.desktopCommands))
	}
	app.processDesktopCommands()
	node, _ := app.workflowGraph("ws-1").node("prompt")
	if node.Position != image.Pt(599, 600) {
		t.Fatalf("final position=%v", node.Position)
	}
}

func TestDesktopDraftUpdateKeepsWorkspaceEditsIndependent(t *testing.T) {
	app := &App{
		activeWorkspaceID: "ws-1",
		workspaces: []workspaceState{
			{ID: "ws-1", Name: "One", Prompt: "one", Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png"},
			{ID: "ws-2", Name: "Two", Prompt: "two", Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png"},
		},
		workflowGraphs:        map[string]workflowGraphModel{"ws-1": defaultWorkflowGraph(), "ws-2": defaultWorkflowGraph()},
		workflowSelectedNodes: map[string]string{"ws-1": "generate", "ws-2": "generate"},
	}
	app.applyDesktopDraftUpdate("ws-2", desktopDraftUpdate{
		Prompt:         "updated two",
		NegativePrompt: "noise",
		Mode:           "edit",
		Size:           "1536x1024",
		Quality:        "high",
		OutputFormat:   "webp",
	})
	if app.workspaces[0].Prompt != "one" {
		t.Fatalf("active workspace changed: %#v", app.workspaces[0])
	}
	updated := app.workspaces[1]
	if updated.Prompt != "updated two" || updated.NegativePrompt != "noise" || updated.Mode != "edit" || updated.Size != "1536x1024" || updated.Quality != "high" || updated.OutputFormat != "webp" {
		t.Fatalf("inactive update=%#v", updated)
	}
}

func TestDesktopWindowRequestUsesStableRoleSizes(t *testing.T) {
	request := desktopWindowRequest(windowing.RoleProgress, " ws-1 ", " Demo ")
	if request.WorkspaceID != "ws-1" || request.Title != "任务进度 - Demo" {
		t.Fatalf("request=%#v", request)
	}
	if request.Size.Width != unit.Dp(420) || request.Size.Height != unit.Dp(260) {
		t.Fatalf("progress size=%#v", request.Size)
	}
	canvas := desktopWindowRequest(windowing.RoleCanvas, "ws-1", "Demo")
	if canvas.Size.Width <= canvas.MinSize.Width || canvas.Size.Height <= canvas.MinSize.Height {
		t.Fatalf("canvas size=%#v min=%#v", canvas.Size, canvas.MinSize)
	}
}

func TestNormalizeExperienceModePreservesBeginnerDefault(t *testing.T) {
	if got := normalizeExperienceMode(""); got != experienceModeSimple {
		t.Fatalf("default mode=%q", got)
	}
	if got := normalizeExperienceMode("WORKFLOW"); got != experienceModeWorkflow {
		t.Fatalf("workflow mode=%q", got)
	}
}

func TestExperienceSwitchClicksRouteBetweenSimpleAndWorkflow(t *testing.T) {
	originalTheme := installedDesktopTheme
	defer installDesktopThemeSpec(originalTheme.Style, originalTheme.ColorMode)

	app := &App{
		th:                    material.NewTheme(),
		desktopState:          desktopstate.Default(),
		desktopStyle:          desktopStyleWindows,
		resolvedThemeMode:     desktopColorModeLight,
		experienceMode:        experienceModeSimple,
		experienceModeButtons: make([]widget.Clickable, 2),
		activeWorkspaceID:     "workspace-1",
	}
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(224, 34)),
	}

	app.experienceModeButtons[1].Click()
	app.layoutExperienceSwitch(gtx)
	if app.experienceMode != experienceModeWorkflow {
		t.Fatalf("workflow click left mode=%q", app.experienceMode)
	}
	app.experienceModeButtons[0].Click()
	app.layoutExperienceSwitch(gtx)
	if app.experienceMode != experienceModeSimple {
		t.Fatalf("simple click left mode=%q", app.experienceMode)
	}
}

func TestDesktopPreferenceTogglesInvertPersistedValues(t *testing.T) {
	originalTheme := installedDesktopTheme
	defer installDesktopThemeSpec(originalTheme.Style, originalTheme.ColorMode)
	app := &App{
		desktopState:               desktopstate.Default(),
		desktopStyle:               desktopStyleWindows,
		experienceMode:             experienceModeSimple,
		desktopStyleButtons:        make([]widget.Clickable, 2),
		generalStartupModeButtons:  make([]widget.Clickable, 2),
		generalWindowLayoutButtons: make([]widget.Clickable, 3),
	}
	app.desktopStyleButtons[0].Click()
	app.generalStartupModeButtons[1].Click()
	app.generalWindowLayoutButtons[2].Click()
	app.generalAutoProgressToggle.Click()
	app.generalReopenWindowsToggle.Click()
	app.generalRestoreSessionToggle.Click()
	app.handleDesktopPreferenceEvents(layout.Context{Ops: new(op.Ops)})

	preferences := app.desktopState.Preferences
	if preferences.AutoShowProgress || preferences.ReopenDetachedWindows || preferences.RestoreSession {
		t.Fatalf("toggles did not invert defaults: %#v", preferences)
	}
	if app.desktopStyle != desktopStyleMacOS || app.experienceMode != experienceModeWorkflow || preferences.DefaultWindowLayout != desktopstate.WindowLayoutMulti {
		t.Fatalf("desktop preference clicks not applied: style=%q mode=%q preferences=%#v", app.desktopStyle, app.experienceMode, preferences)
	}
}

func TestNewRestoresGioWorkflowWorkspaceSession(t *testing.T) {
	root := t.TempDir()
	originalRoot := gioCompat.StableDataRootForTest()
	gioCompat.SetStableDataRootForTest(func() (string, error) { return root, nil })
	defer gioCompat.SetStableDataRootForTest(originalRoot)
	originalTheme := installedDesktopTheme
	defer installDesktopThemeSpec(originalTheme.Style, originalTheme.ColorMode)

	store := desktopstate.NewStore(filepath.Join(root, "gio", desktopstate.FileName))
	_, err := store.Save(desktopstate.State{
		Preferences: desktopstate.Preferences{
			InterfaceStyle:        desktopstate.InterfaceStyleWindows,
			ExperienceMode:        desktopstate.ExperienceModeWorkflow,
			DefaultWindowLayout:   desktopstate.WindowLayoutDual,
			AutoShowProgress:      true,
			ReopenDetachedWindows: true,
			RestoreSession:        true,
		},
		Workspaces: []desktopstate.Workspace{{
			ID:   "ws-persisted",
			Name: "封面流程",
			Draft: desktopstate.WorkspaceDraft{
				Prompt:          "cinematic cover",
				Mode:            "generate",
				Size:            "1536x1024",
				Quality:         "high",
				OutputFormat:    "png",
				BatchCount:      2,
				LoopTotalCount:  10,
				LoopConcurrency: 2,
			},
			Workflow: desktopstate.WorkflowGraph{Nodes: []desktopstate.WorkflowNode{{ID: "prompt", Kind: "prompt", X: 321, Y: 123}}},
		}},
	})
	if err != nil {
		t.Fatalf("save desktop session: %v", err)
	}

	app := New()
	if app.experienceMode != experienceModeWorkflow || app.desktopStyle != desktopStyleWindows {
		t.Fatalf("experience=%q style=%q", app.experienceMode, app.desktopStyle)
	}
	if len(app.workspaces) != 1 || app.activeWorkspaceID != "ws-persisted" || app.promptInput.Text() != "cinematic cover" {
		t.Fatalf("restored workspace=%#v active=%q prompt=%q", app.workspaces, app.activeWorkspaceID, app.promptInput.Text())
	}
	node, ok := app.workflowGraph("ws-persisted").node("prompt")
	if !ok || node.Position != image.Pt(321, 123) {
		t.Fatalf("restored prompt node=%#v ok=%t", node, ok)
	}
}

func TestRestoreDesktopWindowsReopensSavedRoles(t *testing.T) {
	controller := new(fakeDesktopWindowController)
	app := &App{
		experienceMode:    experienceModeWorkflow,
		activeWorkspaceID: "ws-1",
		workspaces:        []workspaceState{{ID: "ws-1", Name: "Main"}},
		desktopWindows:    controller,
		desktopState: desktopstate.State{
			Preferences: desktopstate.Preferences{
				ReopenDetachedWindows: true,
				DefaultWindowLayout:   desktopstate.WindowLayoutSingle,
			},
			Windows: []desktopstate.Window{
				{Role: desktopstate.WindowRoleCanvas, WorkspaceID: "ws-1", WidthDp: 1000, HeightDp: 700, Visible: true},
				{Role: desktopstate.WindowRoleConsole, WorkspaceID: "missing", WidthDp: 800, HeightDp: 500, Visible: true},
				{Role: desktopstate.WindowRoleProgress, WorkspaceID: "ws-1", Visible: false},
			},
		},
	}
	app.RestoreDesktopWindows()
	if len(controller.opened) != 2 {
		t.Fatalf("opened=%#v", controller.opened)
	}
	if controller.opened[0].Role != windowing.RoleCanvas || controller.opened[0].WorkspaceID != "ws-1" || controller.opened[0].Size.Width != unit.Dp(1000) {
		t.Fatalf("canvas request=%#v", controller.opened[0])
	}
	if controller.opened[1].Role != windowing.RoleConsole || controller.opened[1].WorkspaceID != "ws-1" {
		t.Fatalf("console fallback request=%#v", controller.opened[1])
	}
}

func TestRestoreDesktopWindowsHonorsDisabledReopenPreference(t *testing.T) {
	controller := new(fakeDesktopWindowController)
	app := &App{
		experienceMode:    experienceModeWorkflow,
		activeWorkspaceID: "ws-1",
		workspaces:        []workspaceState{{ID: "ws-1", Name: "Main"}},
		desktopWindows:    controller,
		desktopState: desktopstate.State{
			Preferences: desktopstate.Preferences{
				ReopenDetachedWindows: false,
				DefaultWindowLayout:   desktopstate.WindowLayoutMulti,
			},
			Windows: []desktopstate.Window{{
				Role:        desktopstate.WindowRoleCanvas,
				WorkspaceID: "ws-1",
				Visible:     true,
			}},
		},
	}

	app.RestoreDesktopWindows()
	if len(controller.opened) != 0 {
		t.Fatalf("opened=%#v want no restored or default windows", controller.opened)
	}
}
