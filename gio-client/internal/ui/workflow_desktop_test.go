package ui

import (
	"image"
	"path/filepath"
	"testing"

	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/desktopstate"
	"image-studio/gio-client/internal/windowing"

	"gioui.org/io/input"
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

func TestMacWorkflowPaneWidthsMatchAppleContract(t *testing.T) {
	spec := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	wide := resolveWorkflowPaneWidths(unit.Dp(1440), spec)
	if wide.Left != unit.Dp(280) || wide.Right != unit.Dp(320) || wide.Center != unit.Dp(840) {
		t.Fatalf("wide panes=%+v want 280/840/320", wide)
	}

	minimum := resolveWorkflowPaneWidths(unit.Dp(1040), spec)
	if minimum.Center < unit.Dp(440) {
		t.Fatalf("minimum-window center=%v want >=440", minimum.Center)
	}
	if minimum.Left < unit.Dp(240) || minimum.Right < unit.Dp(280) {
		t.Fatalf("minimum-window side panes=%+v below readable bounds", minimum)
	}
	if minimum.Left+minimum.Center+minimum.Right != unit.Dp(1040) {
		t.Fatalf("minimum-window panes=%+v do not consume available width", minimum)
	}
}

func TestMacWorkflowPaneVisibilityPreservesCanvasPriority(t *testing.T) {
	spec := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	if got := resolveVisibleWorkflowPaneWidths(unit.Dp(1440), spec, true, true); got != (workflowPaneWidths{Left: 280, Center: 840, Right: 320}) {
		t.Fatalf("visible workflow panes=%+v", got)
	}
	if got := resolveVisibleWorkflowPaneWidths(unit.Dp(1440), spec, true, false); got != (workflowPaneWidths{Left: 280, Center: 1160}) {
		t.Fatalf("library-only workflow panes=%+v", got)
	}
	if got := resolveVisibleWorkflowPaneWidths(unit.Dp(1440), spec, false, true); got != (workflowPaneWidths{Center: 1120, Right: 320}) {
		t.Fatalf("inspector-only workflow panes=%+v", got)
	}
	if got := resolveVisibleWorkflowPaneWidths(unit.Dp(1440), spec, false, false); got != (workflowPaneWidths{Center: 1440}) {
		t.Fatalf("canvas-only workflow panes=%+v", got)
	}
}

func TestWindowsWorkflowPaneWidthsRemainStable(t *testing.T) {
	spec := desktopThemeSpec(desktopStyleWindows, desktopColorModeLight)
	wide := resolveWorkflowPaneWidths(unit.Dp(1440), spec)
	if wide.Left != spec.Metrics.LeftPaneWidth || wide.Right != spec.Metrics.RightPaneWidth {
		t.Fatalf("wide Windows panes=%+v want theme metrics %+v", wide, spec.Metrics)
	}
	compact := resolveWorkflowPaneWidths(unit.Dp(1040), spec)
	if compact.Left != unit.Dp(224) || compact.Right != unit.Dp(284) || compact.Center != unit.Dp(532) {
		t.Fatalf("compact Windows panes=%+v want 224/532/284", compact)
	}
}

func TestMacWorkflowPresentationMetrics(t *testing.T) {
	mac := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	prompt := resolveWorkflowPromptEditorMetrics(mac)
	if prompt.Height != unit.Dp(176) || prompt.Radius != unit.Dp(18) {
		t.Fatalf("mac prompt metrics=%+v want 176dp/r18", prompt)
	}
	if got := workflowSectionRadius(mac); got != mac.Metrics.CardRadius {
		t.Fatalf("mac section radius=%v want %v", got, mac.Metrics.CardRadius)
	}

	windows := desktopThemeSpec(desktopStyleWindows, desktopColorModeLight)
	windowsPrompt := resolveWorkflowPromptEditorMetrics(windows)
	if windowsPrompt.Height != unit.Dp(166) || windowsPrompt.Radius != windows.Metrics.InputRadius {
		t.Fatalf("Windows prompt metrics=%+v want legacy metrics", windowsPrompt)
	}
	if got := workflowSectionRadius(windows); got != windows.Metrics.CardRadius {
		t.Fatalf("Windows section radius=%v want %v", got, windows.Metrics.CardRadius)
	}
}

func TestMacWorkflowCommandBarKeepsIconActionsAccessible(t *testing.T) {
	isolateGioStableDataRoot(t)
	previousTheme := installedDesktopTheme
	defer installDesktopThemeSpec(previousTheme.Style, previousTheme.ColorMode)
	spec := installDesktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	app := New()
	app.desktopStyle = desktopStyleMacOS
	app.installResolvedTheme(desktopColorModeLight)
	var (
		ops    op.Ops
		router input.Router
	)
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(1040, int(spec.Metrics.CommandBarHeight))),
	}
	dims := app.layoutMacWorkflowCommandBar(gtx, snapshot{}, spec)
	if dims.Size != image.Pt(1040, int(spec.Metrics.CommandBarHeight)) {
		t.Fatalf("Apple workflow command bar size=%v", dims.Size)
	}
	router.Frame(&ops)
	nodes := router.AppendSemantics(nil)
	consoleLabel := chooseString(app.workflowConsoleOpen, "隐藏底部", "显示底部")
	for _, label := range []string{"撤销", "重做", "保存", "加载", "缩小", "适合画布", "放大", "画布窗口", "控制台窗口", "进度窗口", consoleLabel, "重置布局", "运行"} {
		if _, ok := semanticTreeButtonByLabel(nodes, label); !ok {
			t.Fatalf("Apple workflow command %q is missing an accessible label", label)
		}
	}
}

func TestWorkflowLibraryRowsExposeSelectedSemantics(t *testing.T) {
	previousTheme := installedDesktopTheme
	defer installDesktopThemeSpec(previousTheme.Style, previousTheme.ColorMode)
	spec := installDesktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	var (
		ops    op.Ops
		router input.Router
	)
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(420, 240)),
	}
	app := &App{
		th:                material.NewTheme(),
		desktopStyle:      desktopStyleMacOS,
		resolvedThemeMode: desktopColorModeLight,
		fontScale:         1,
	}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.workflowWorkspaceRow(gtx, workspaceState{ID: "current", Name: "当前工作区"}, true, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.workflowWorkspaceRow(gtx, workspaceState{ID: "other", Name: "其他工作区"}, false, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.workflowNodeLibraryRow(gtx, workflowNodeModel{ID: "prompt", Title: "提示词节点", Subtitle: "输入提示词", Enabled: true}, workflowNodeRuntime{}, true, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.workflowNodeLibraryRow(gtx, workflowNodeModel{ID: "export", Title: "输出节点", Subtitle: "保存图像", Enabled: true}, workflowNodeRuntime{}, false, spec)
		}),
	)
	router.Frame(&ops)
	nodes := router.AppendSemantics(nil)
	assertSemanticSelected(t, nodes, "当前工作区", true)
	assertSemanticSelected(t, nodes, "其他工作区", false)
	assertSemanticSelected(t, nodes, "提示词节点", true)
	assertSemanticSelected(t, nodes, "输出节点", false)
}

func TestWorkflowLibraryAndInspectorRouteCatalogNodeActions(t *testing.T) {
	isolateGioStableDataRoot(t)
	previousTheme := installedDesktopTheme
	defer installDesktopThemeSpec(previousTheme.Style, previousTheme.ColorMode)
	app := New()
	app.experienceMode = experienceModeWorkflow
	workspaceID := app.activeWorkspaceID
	if !app.deleteWorkflowNode(workspaceID, "source") {
		t.Fatal("delete source setup returned false")
	}
	spec := installDesktopThemeSpec(desktopStyleWindows, desktopColorModeLight)
	libraryContext := func() layout.Context {
		return layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(320, 720)),
		}
	}
	app.workflowAddNodeButton("source").Click()
	app.layoutWorkflowLibrary(libraryContext(), snapshot{}, spec)
	if _, ok := app.workflowGraph(workspaceID).node("source"); !ok {
		t.Fatal("library add button did not restore source")
	}
	app.workflowAddNodeButton("prompt").Click()
	app.layoutWorkflowLibrary(libraryContext(), snapshot{}, spec)
	if repeated, ok := app.workflowGraph(workspaceID).node("prompt-2"); !ok || repeated.Kind != workflowNodePrompt {
		t.Fatalf("library repeat add prompt=%+v ok=%t", repeated, ok)
	}

	app.selectWorkflowNode(workspaceID, "source")
	app.workflowDuplicateNodeButton.Click()
	app.layoutWorkflowInspector(libraryContext(), snapshot{}, spec)
	if duplicated, ok := app.workflowGraph(workspaceID).node("source-2"); !ok || duplicated.Kind != workflowNodeSource {
		t.Fatalf("inspector duplicate source=%+v ok=%t", duplicated, ok)
	}
	app.selectWorkflowNode(workspaceID, "source")
	app.workflowToggleNodeButton.Click()
	app.layoutWorkflowInspector(libraryContext(), snapshot{}, spec)
	source, _ := app.workflowGraph(workspaceID).node("source")
	if source.Enabled {
		t.Fatal("inspector toggle button did not disable source")
	}
	app.workflowDeleteNodeButton.Click()
	app.layoutWorkflowInspector(libraryContext(), snapshot{}, spec)
	if _, ok := app.workflowGraph(workspaceID).node("source"); ok {
		t.Fatal("inspector delete button did not remove source")
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
	edge := workflowEdgeModel{FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"}
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandRewireConnection, WorkspaceID: "ws-1", PreviousEdge: edge, HasPreviousEdge: true})
	if workflowEdgeConnected(app.workflowGraph("ws-1"), edge) {
		t.Fatal("desktop connection command did not disconnect existing edge")
	}
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandRewireConnection, WorkspaceID: "ws-1", Edge: edge, HasEdge: true})
	if !workflowEdgeConnected(app.workflowGraph("ws-1"), edge) {
		t.Fatal("desktop connection command did not reconnect edge")
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

func TestDesktopMoveTransactionAndHistoryCommandsStayAtomic(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-1")
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandBeginNodeMove, WorkspaceID: "ws-1", NodeID: "prompt"})
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandMoveNode, WorkspaceID: "ws-1", NodeID: "prompt", Position: image.Pt(240, 200)})
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandMoveNode, WorkspaceID: "ws-1", NodeID: "prompt", Position: image.Pt(360, 280)})
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandEndNodeMove, WorkspaceID: "ws-1", NodeID: "prompt"})
	if got := len(app.workflowHistory("ws-1").Undo); got != 1 {
		t.Fatalf("desktop move undo entries=%d want 1", got)
	}
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandUndoWorkflow, WorkspaceID: "ws-1"})
	undone, _ := app.workflowGraph("ws-1").node("prompt")
	if undone.Position != image.Pt(72, 92) {
		t.Fatalf("desktop undo position=%v", undone.Position)
	}
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandRedoWorkflow, WorkspaceID: "ws-1"})
	redone, _ := app.workflowGraph("ws-1").node("prompt")
	if redone.Position != image.Pt(360, 280) {
		t.Fatalf("desktop redo position=%v", redone.Position)
	}
}

func TestQueuedDesktopMoveFlushesBeforeImmediateUndo(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-1")
	app.desktopCommands = make(chan desktopCommand, 8)
	app.desktopPendingMoves = map[string]desktopCommand{}
	app.enqueueDesktopCommand(desktopCommand{Kind: desktopCommandBeginNodeMove, WorkspaceID: "ws-1", NodeID: "prompt"})
	for index := 0; index < 40; index++ {
		app.enqueueDesktopCommand(desktopCommand{
			Kind: desktopCommandMoveNode, WorkspaceID: "ws-1", NodeID: "prompt", Position: image.Pt(300+index, 240+index),
		})
	}
	app.enqueueDesktopCommand(desktopCommand{Kind: desktopCommandEndNodeMove, WorkspaceID: "ws-1", NodeID: "prompt"})
	app.enqueueDesktopCommand(desktopCommand{Kind: desktopCommandUndoWorkflow, WorkspaceID: "ws-1"})
	app.processDesktopCommands()

	node, _ := app.workflowGraph("ws-1").node("prompt")
	if node.Position != image.Pt(72, 92) {
		t.Fatalf("immediate undo position=%v want original", node.Position)
	}
	if !app.canRedoWorkflowGraph("ws-1") {
		t.Fatal("immediate undo did not retain final coalesced move for redo")
	}
	app.redoWorkflowGraph("ws-1")
	node, _ = app.workflowGraph("ws-1").node("prompt")
	if node.Position != image.Pt(339, 279) {
		t.Fatalf("coalesced redo position=%v want final move", node.Position)
	}
}

func TestDesktopCommandsToggleAndDeleteWorkflowNode(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-1")
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandToggleWorkflowNode, WorkspaceID: "ws-1", NodeID: "preview"})
	preview, ok := app.workflowGraph("ws-1").node("preview")
	if !ok || preview.Enabled {
		t.Fatalf("toggle command preview=%+v ok=%t", preview, ok)
	}
	app.applyDesktopCommand(desktopCommand{Kind: desktopCommandDeleteWorkflowNode, WorkspaceID: "ws-1", NodeID: "preview"})
	if _, ok := app.workflowGraph("ws-1").node("preview"); ok {
		t.Fatal("delete command retained preview node")
	}
	if got := len(app.workflowHistory("ws-1").Undo); got != 2 {
		t.Fatalf("desktop graph edit undo entries=%d want 2", got)
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
