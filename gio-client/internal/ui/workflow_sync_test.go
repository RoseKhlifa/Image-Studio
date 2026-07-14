package ui

import (
	"image"
	"path/filepath"
	"testing"

	"image-studio/gio-client/internal/desktopstate"
	"image-studio/gio-client/internal/windowing"

	gioapp "gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func TestWorkflowCanvasOverridesStayWorkspaceScopedAndClearOnConfirmation(t *testing.T) {
	graph := defaultWorkflowGraph()
	view := workflowCanvasViewState{}
	view.syncModel(workflowCanvasData{Workspace: "ws-one", Graph: graph})
	view.setOverride("prompt", image.Pt(280, 190), graph.Revision)

	view.syncModel(workflowCanvasData{Workspace: "ws-two", Graph: graph})
	if len(view.overrides) != 0 || len(view.overrideRevisions) != 0 {
		t.Fatalf("workspace switch retained overrides: positions=%v revisions=%v", view.overrides, view.overrideRevisions)
	}

	view.setOverride("prompt", image.Pt(310, 210), graph.Revision)
	confirmed := moveWorkflowNode(graph, "prompt", image.Pt(310, 210))
	view.syncModel(workflowCanvasData{Workspace: "ws-two", Graph: confirmed})
	if len(view.overrides) != 0 || len(view.overrideRevisions) != 0 {
		t.Fatalf("confirmed model retained overrides: positions=%v revisions=%v", view.overrides, view.overrideRevisions)
	}
}

func TestResetWorkflowGraphClearsOverridesAndAdvancesRevision(t *testing.T) {
	graph := moveWorkflowNode(defaultWorkflowGraph(), "prompt", image.Pt(500, 400))
	app := &App{
		workflowGraphs:        map[string]workflowGraphModel{"ws-one": graph},
		workflowSelectedNodes: map[string]string{"ws-one": "prompt"},
		workflowCanvas: workflowCanvasViewState{
			workspaceID:       "ws-one",
			graphRevision:     graph.Revision,
			overrides:         map[string]image.Point{"prompt": image.Pt(520, 420)},
			overrideRevisions: map[string]int{"prompt": graph.Revision},
		},
	}

	app.resetWorkflowGraph("ws-one")
	reset := app.workflowGraph("ws-one")
	prompt, ok := reset.node("prompt")
	if !ok || prompt.Position != image.Pt(72, 92) {
		t.Fatalf("reset prompt=%+v ok=%t", prompt, ok)
	}
	if reset.Revision <= graph.Revision {
		t.Fatalf("reset revision=%d want greater than %d", reset.Revision, graph.Revision)
	}
	if len(app.workflowCanvas.overrides) != 0 || len(app.workflowCanvas.overrideRevisions) != 0 {
		t.Fatalf("reset retained canvas overrides: %+v", app.workflowCanvas.overrides)
	}

	detached := workflowCanvasViewState{}
	detached.syncModel(workflowCanvasData{Workspace: "ws-one", Graph: graph})
	detached.setOverride("prompt", image.Pt(520, 420), graph.Revision)
	detached.syncModel(workflowCanvasData{Workspace: "ws-one", Graph: reset})
	if len(detached.overrides) != 0 || len(detached.overrideRevisions) != 0 {
		t.Fatalf("published graph reset retained detached overrides: %+v", detached.overrides)
	}
}

func TestDeleteWorkflowWorkspaceStateDropsDraftRevision(t *testing.T) {
	app := &App{
		workflowGraphs:        map[string]workflowGraphModel{"ws-one": defaultWorkflowGraph()},
		workflowSelectedNodes: map[string]string{"ws-one": "prompt"},
		desktopDraftModels: map[string]desktopDraftModel{
			"ws-one": {Revision: 12},
		},
	}
	app.deleteWorkflowWorkspaceState("ws-one")
	if _, exists := app.desktopDraftModels["ws-one"]; exists {
		t.Fatal("deleted workspace retained draft revision")
	}
}

func TestWorkflowRuntimeReflectsDisconnectedPorts(t *testing.T) {
	graph := defaultWorkflowGraph()
	prompt, _ := graph.node("prompt")
	prompt.Properties[workflowPropertyPrompt] = "a complete prompt"
	graph = setWorkflowNodeProperties(graph, prompt.ID, prompt.Properties)
	source, _ := graph.node("source")
	source.Properties[workflowPropertySourcePaths] = "/tmp/source.png"
	graph = setWorkflowNodeProperties(graph, source.ID, source.Properties)
	generate, _ := graph.node("generate")
	generate.Properties[workflowPropertyMode] = "edit"
	graph = setWorkflowNodeProperties(graph, generate.ID, generate.Properties)
	for _, edge := range []workflowEdgeModel{
		{FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"},
		{FromNode: "source", FromPort: "image", ToNode: "generate", ToPort: "source"},
		{FromNode: "generate", FromPort: "job", ToNode: "preview", ToPort: "job"},
		{FromNode: "preview", FromPort: "image", ToNode: "export", ToPort: "image"},
	} {
		var err error
		graph, err = toggleWorkflowConnection(graph, edge)
		if err != nil {
			t.Fatalf("disconnect %s: %v", workflowEdgeID(edge), err)
		}
	}
	app := &App{
		activeWorkspaceID:     "ws-one",
		workflowGraphs:        map[string]workflowGraphModel{"ws-one": graph},
		workflowSelectedNodes: map[string]string{"ws-one": "generate"},
		mode:                  "edit",
	}
	runtime := app.workflowCanvasData(snapshot{}, "ws-one").Runtime
	for _, nodeID := range []string{"prompt", "source", "generate", "preview", "export"} {
		if runtime[nodeID].Phase != workflowNodePhaseWarning {
			t.Fatalf("node %s phase=%s detail=%q want warning", nodeID, runtime[nodeID].Phase, runtime[nodeID].Detail)
		}
	}
}

func TestInactiveWorkflowRuntimeReflectsItsPublishedGraph(t *testing.T) {
	graph := defaultWorkflowGraph()
	for _, edge := range []workflowEdgeModel{
		{FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"},
		{FromNode: "preview", FromPort: "image", ToNode: "export", ToPort: "image"},
	} {
		var err error
		graph, err = toggleWorkflowConnection(graph, edge)
		if err != nil {
			t.Fatalf("disconnect %s: %v", workflowEdgeID(edge), err)
		}
	}
	runtime := workflowRuntimeForInactiveWorkspace(workspaceState{
		Prompt:          "saved prompt",
		Mode:            "generate",
		ResultHasItem:   true,
		ResultSavedPath: "/tmp/result.png",
	}, graph)
	for _, nodeID := range []string{"prompt", "generate", "export"} {
		if runtime[nodeID].Phase != workflowNodePhaseWarning {
			t.Fatalf("inactive node %s phase=%s detail=%q want warning", nodeID, runtime[nodeID].Phase, runtime[nodeID].Detail)
		}
	}
	if runtime["preview"].Phase != workflowNodePhaseIdle {
		t.Fatalf("unselected preview branch phase=%s want idle", runtime["preview"].Phase)
	}
}

func TestDetachedWorkspaceInspectorListItemHeightIsBounded(t *testing.T) {
	view := newDesktopWindowView(&App{desktopCommands: make(chan desktopCommand, 1)}, windowing.Request{
		Role:        windowing.RoleWorkspace,
		WorkspaceID: "ws-one",
	})
	spec := desktopThemeSpec(desktopStyleWindows, desktopColorModeDark)
	view.applyTheme(spec)
	view.syncDetachedWorkspaceDraft(desktopWorkspacePublication{
		ID:           "ws-one",
		Name:         "Workspace",
		Mode:         "generate",
		Size:         "auto",
		Quality:      "auto",
		OutputFormat: "png",
	})
	var ops op.Ops
	gtx := layout.Context{
		Ops:    &ops,
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{
			Max: image.Pt(320, 1_000_000),
		},
	}
	dimensions := view.layoutDetachedWorkspaceInspectorContent(gtx, spec, desktopWorkspacePublication{Name: "Workspace"})
	if dimensions.Size.Y <= 0 || dimensions.Size.Y >= 10_000 {
		t.Fatalf("inspector list item height=%d want finite content height", dimensions.Size.Y)
	}
}

func TestDetachedWorkspaceDraftTracksRevisionWithoutOverwritingDirtyEditors(t *testing.T) {
	view := newDesktopWindowView(&App{}, windowing.Request{Role: windowing.RoleWorkspace, WorkspaceID: "ws-one"})
	view.syncDetachedWorkspaceDraft(desktopWorkspacePublication{
		ID: "ws-one", DraftRevision: 1, Prompt: "first", Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png",
	})
	view.syncDetachedWorkspaceDraft(desktopWorkspacePublication{
		ID: "ws-one", DraftRevision: 2, Prompt: "main update", Mode: "generate", Size: "auto", Quality: "high", OutputFormat: "png",
	})
	if got := view.promptEditor.Text(); got != "main update" || view.draftRevision != 2 || view.draftDirty {
		t.Fatalf("clean draft did not follow publication: prompt=%q revision=%d dirty=%t", got, view.draftRevision, view.draftDirty)
	}

	view.promptEditor.SetText("local edit")
	view.syncDetachedWorkspaceDraft(desktopWorkspacePublication{
		ID: "ws-one", DraftRevision: 3, Prompt: "newer main update", Mode: "generate", Size: "auto", Quality: "low", OutputFormat: "png",
	})
	if got := view.promptEditor.Text(); got != "local edit" || view.draftRevision != 2 || !view.draftDirty {
		t.Fatalf("dirty draft was overwritten: prompt=%q revision=%d dirty=%t", got, view.draftRevision, view.draftDirty)
	}

	view.syncDetachedWorkspaceDraft(desktopWorkspacePublication{
		ID: "ws-one", DraftRevision: 4, Prompt: "local edit", Mode: "generate", Size: "auto", Quality: "high", OutputFormat: "png",
	})
	if view.draftRevision != 4 || view.draftDirty {
		t.Fatalf("confirmed draft was not acknowledged: revision=%d dirty=%t", view.draftRevision, view.draftDirty)
	}
}

func TestDesktopDraftAndRunRejectsStaleRevisionAtomically(t *testing.T) {
	app := &App{
		activeWorkspaceID: "ws-one",
		workspaces: []workspaceState{{
			ID: "ws-one", Name: "Workspace", Prompt: "first", Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png",
		}},
	}
	app.promptInput.SetText("first")
	app.mode, app.size, app.quality, app.format = "generate", "auto", "auto", "png"
	base, _ := app.currentDesktopDraft("ws-one")
	if revision := app.observeDesktopDraft("ws-one", base); revision != 1 {
		t.Fatalf("initial revision=%d want 1", revision)
	}
	app.promptInput.SetText("newer main value")
	current, _ := app.currentDesktopDraft("ws-one")
	if revision := app.observeDesktopDraft("ws-one", current); revision != 2 {
		t.Fatalf("updated revision=%d want 2", revision)
	}

	app.applyDesktopCommand(desktopCommand{
		Kind:          desktopCommandUpdateDraftAndRun,
		WorkspaceID:   "ws-one",
		DraftRevision: 1,
		Draft: desktopDraftUpdate{
			Prompt: "stale detached value", Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png",
		},
	})
	if got := app.promptInput.Text(); got != "newer main value" {
		t.Fatalf("stale draft overwrote main editor: %q", got)
	}
	if app.isRunning() || len(app.desktopQueuedWorkspaceRuns) != 0 {
		t.Fatalf("stale atomic command started or queued run: running=%t queue=%v", app.isRunning(), app.desktopQueuedWorkspaceRuns)
	}
}

func TestDesktopDraftAndRunAppliesDraftBeforeQueueing(t *testing.T) {
	app := &App{
		activeWorkspaceID: "ws-one",
		workspaces: []workspaceState{{
			ID: "ws-one", Name: "Workspace", Prompt: "before", Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png",
		}},
		running: true,
	}
	app.promptInput.SetText("before")
	app.mode, app.size, app.quality, app.format = "generate", "auto", "auto", "png"
	base, _ := app.currentDesktopDraft("ws-one")
	revision := app.observeDesktopDraft("ws-one", base)

	app.applyDesktopCommand(desktopCommand{
		Kind:          desktopCommandUpdateDraftAndRun,
		WorkspaceID:   "ws-one",
		DraftRevision: revision,
		Draft: desktopDraftUpdate{
			Prompt: "after", Mode: "generate", Size: "auto", Quality: "high", OutputFormat: "png",
		},
	})
	if app.promptInput.Text() != "after" || app.quality != "high" {
		t.Fatalf("atomic run queued before applying draft: prompt=%q quality=%q", app.promptInput.Text(), app.quality)
	}
	if len(app.desktopQueuedWorkspaceRuns) != 1 || app.desktopQueuedWorkspaceRuns[0] != "ws-one" {
		t.Fatalf("atomic run queue=%v want ws-one", app.desktopQueuedWorkspaceRuns)
	}
}

func TestDetachedWorkspaceRunEnqueuesSingleAtomicCommand(t *testing.T) {
	root := &App{desktopCommands: make(chan desktopCommand, 4)}
	view := newDesktopWindowView(root, windowing.Request{Role: windowing.RoleWorkspace, WorkspaceID: "ws-one"})
	publication := desktopPublication{
		ActiveID: "ws-one",
		Workspaces: []desktopWorkspacePublication{{
			ID: "ws-one", Name: "Workspace", DraftRevision: 7, Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png", Graph: defaultWorkflowGraph(),
		}},
	}
	view.runButton.Click()
	view.layout(detachedTestContext(view, image.Pt(1280, 860)), publication)
	command := detachedNextCommand(t, root)
	if command.Kind != desktopCommandUpdateDraftAndRun || command.WorkspaceID != "ws-one" || command.DraftRevision != 7 {
		t.Fatalf("run command=%+v want one revisioned draft-and-run", command)
	}
	select {
	case extra := <-root.desktopCommands:
		t.Fatalf("run enqueued non-atomic extra command: %+v", extra)
	default:
	}
}

func TestDetachedCanvasHistoryButtonsRouteWorkspaceCommands(t *testing.T) {
	root := &App{desktopCommands: make(chan desktopCommand, 4)}
	view := newDesktopWindowView(root, windowing.Request{Role: windowing.RoleCanvas, WorkspaceID: "ws-one"})
	publication := desktopPublication{
		ActiveID: "ws-one",
		Workspaces: []desktopWorkspacePublication{{
			ID: "ws-one", Name: "Workspace", Graph: defaultWorkflowGraph(), CanUndo: true, CanRedo: true,
		}},
	}
	view.undoButton.Click()
	view.layout(detachedTestContext(view, image.Pt(1280, 860)), publication)
	undo := detachedNextCommand(t, root)
	if undo.Kind != desktopCommandUndoWorkflow || undo.WorkspaceID != "ws-one" {
		t.Fatalf("undo command=%+v", undo)
	}

	view.redoButton.Click()
	view.layout(detachedTestContext(view, image.Pt(1280, 860)), publication)
	redo := detachedNextCommand(t, root)
	if redo.Kind != desktopCommandRedoWorkflow || redo.WorkspaceID != "ws-one" {
		t.Fatalf("redo command=%+v", redo)
	}
}

func TestDetachedConfigSizeOverridesInitialRequestInSavedSession(t *testing.T) {
	request := windowing.Request{
		Role:        windowing.RoleCanvas,
		WorkspaceID: "ws-one",
		Size:        windowing.DpSize{Width: unit.Dp(1180), Height: unit.Dp(820)},
	}
	controller := &fakeDesktopWindowController{opened: []windowing.Request{request}}
	store := desktopstate.NewStore(filepath.Join(t.TempDir(), desktopstate.FileName))
	root := &App{
		desktopStore:      store,
		desktopState:      desktopstate.Default(),
		desktopWindows:    controller,
		activeWorkspaceID: "ws-one",
		workspaces: []workspaceState{{
			ID: "ws-one", Name: "Workspace", Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png",
		}},
	}
	view := newDesktopWindowView(root, request)
	view.metric = unit.Metric{PxPerDp: 2, PxPerSp: 2}
	view.config = gioapp.Config{Size: image.Pt(2000, 1200)}
	view.reportWindowSize()

	if err := root.saveGioDesktopState(); err != nil {
		t.Fatalf("save desktop state: %v", err)
	}
	saved, err := store.Load()
	if err != nil {
		t.Fatalf("load desktop state: %v", err)
	}
	if len(saved.Windows) != 1 || saved.Windows[0].WidthDp != 1000 || saved.Windows[0].HeightDp != 600 {
		t.Fatalf("saved windows=%+v want actual 1000x600 dp config size", saved.Windows)
	}
}
