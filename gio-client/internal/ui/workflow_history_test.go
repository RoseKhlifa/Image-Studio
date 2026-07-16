package ui

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

func newWorkflowHistoryTestApp(workspaceIDs ...string) *App {
	graphs := make(map[string]workflowGraphModel, len(workspaceIDs))
	selected := make(map[string]string, len(workspaceIDs))
	workspaces := make([]workspaceState, 0, len(workspaceIDs))
	for _, workspaceID := range workspaceIDs {
		graphs[workspaceID] = defaultWorkflowGraph()
		selected[workspaceID] = "generate"
		workspaces = append(workspaces, workspaceState{ID: workspaceID, Name: workspaceID})
	}
	active := ""
	if len(workspaceIDs) > 0 {
		active = workspaceIDs[0]
	}
	return &App{
		activeWorkspaceID:      active,
		workspaces:             workspaces,
		workflowGraphs:         graphs,
		workflowGraphHistories: map[string]*workflowGraphHistory{},
		workflowSelectedNodes:  selected,
	}
}

func TestWorkflowMoveTransactionCreatesSingleUndoStep(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-one")
	original := app.workflowGraph("ws-one")
	app.beginWorkflowNodeMove("ws-one", "prompt")
	for index := 0; index < 20; index++ {
		app.setWorkflowNodePosition("ws-one", "prompt", image.Pt(100+index*5, 120+index*3))
	}
	app.endWorkflowNodeMove("ws-one", "prompt")
	moved := app.workflowGraph("ws-one")
	finalNode, _ := moved.node("prompt")
	if finalNode.Position != image.Pt(195, 177) {
		t.Fatalf("final position=%v", finalNode.Position)
	}
	history := app.workflowHistory("ws-one")
	if len(history.Undo) != 1 || history.Move != nil {
		t.Fatalf("move history undo=%d active=%+v want one completed transaction", len(history.Undo), history.Move)
	}
	if !app.undoWorkflowGraph("ws-one") {
		t.Fatal("undo move returned false")
	}
	restored := app.workflowGraph("ws-one")
	originalNode, _ := original.node("prompt")
	restoredNode, _ := restored.node("prompt")
	if restoredNode.Position != originalNode.Position {
		t.Fatalf("undo position=%v want %v", restoredNode.Position, originalNode.Position)
	}
	if restored.Revision <= moved.Revision {
		t.Fatalf("undo revision=%d want greater than moved revision %d", restored.Revision, moved.Revision)
	}
	if !app.redoWorkflowGraph("ws-one") {
		t.Fatal("redo move returned false")
	}
	redone := app.workflowGraph("ws-one")
	redoneNode, _ := redone.node("prompt")
	if redoneNode.Position != finalNode.Position || redone.Revision <= restored.Revision {
		t.Fatalf("redo node=%v revision=%d want node=%v revision>%d", redoneNode.Position, redone.Revision, finalNode.Position, restored.Revision)
	}
}

func TestWorkflowHistoryClearsRedoAfterBranchingEdit(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-one")
	edge := workflowEdgeModel{FromNode: "preview", FromPort: "image", ToNode: "export", ToPort: "image"}
	if err := app.rewireWorkflowConnection("ws-one", &edge, nil); err != nil {
		t.Fatalf("disconnect export: %v", err)
	}
	if !app.undoWorkflowGraph("ws-one") || !app.canRedoWorkflowGraph("ws-one") {
		t.Fatal("undo did not make redo available")
	}
	app.setWorkflowNodePosition("ws-one", "prompt", image.Pt(300, 240))
	if app.canRedoWorkflowGraph("ws-one") {
		t.Fatal("branching graph edit retained stale redo history")
	}
}

func TestWorkflowHistoryIsWorkspaceScopedAndBounded(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-one", "ws-two")
	app.setWorkflowNodePosition("ws-one", "prompt", image.Pt(220, 180))
	app.setWorkflowNodePosition("ws-two", "prompt", image.Pt(420, 380))
	if !app.undoWorkflowGraph("ws-one") {
		t.Fatal("workspace one undo returned false")
	}
	one, _ := app.workflowGraph("ws-one").node("prompt")
	two, _ := app.workflowGraph("ws-two").node("prompt")
	if one.Position != image.Pt(72, 92) || two.Position != image.Pt(420, 380) {
		t.Fatalf("workspace isolation failed: one=%v two=%v", one.Position, two.Position)
	}

	for index := 0; index < workflowGraphHistoryLimit+12; index++ {
		app.setWorkflowNodePosition("ws-two", "prompt", image.Pt(500+index, 400))
	}
	if got := len(app.workflowHistory("ws-two").Undo); got != workflowGraphHistoryLimit {
		t.Fatalf("bounded undo entries=%d want %d", got, workflowGraphHistoryLimit)
	}
}

func TestWorkflowResetCanBeUndone(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-one")
	app.setWorkflowNodePosition("ws-one", "prompt", image.Pt(640, 480))
	beforeReset := app.workflowGraph("ws-one")
	app.resetWorkflowGraph("ws-one")
	resetNode, _ := app.workflowGraph("ws-one").node("prompt")
	if resetNode.Position != image.Pt(72, 92) {
		t.Fatalf("reset position=%v", resetNode.Position)
	}
	if !app.undoWorkflowGraph("ws-one") {
		t.Fatal("undo reset returned false")
	}
	restoredNode, _ := app.workflowGraph("ws-one").node("prompt")
	wantNode, _ := beforeReset.node("prompt")
	if restoredNode.Position != wantNode.Position {
		t.Fatalf("undo reset position=%v want %v", restoredNode.Position, wantNode.Position)
	}
}

func TestWorkflowHistoryHelperRejectsNoopAndTrimsOldest(t *testing.T) {
	entries := []workflowGraphModel(nil)
	for index := 0; index < workflowGraphHistoryLimit+3; index++ {
		graph := defaultWorkflowGraph()
		graph.Revision = index + 1
		graph.Nodes[0].Title = fmt.Sprintf("prompt-%d", index)
		entries = appendWorkflowHistoryEntry(entries, graph)
	}
	if len(entries) != workflowGraphHistoryLimit || entries[0].Nodes[0].Title != "prompt-3" {
		t.Fatalf("trimmed history len=%d first=%q", len(entries), entries[0].Nodes[0].Title)
	}
}

func TestWorkflowHistoryKeyboardShortcutsRouteToGraph(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-one")
	app.experienceMode = experienceModeWorkflow
	app.setWorkflowNodePosition("ws-one", "prompt", image.Pt(420, 320))
	var (
		ops    op.Ops
		router input.Router
	)
	render := func() {
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(900, 600)),
		}
		app.handleCanvasKeyboardShortcuts(gtx, snapshot{})
		router.Frame(&ops)
	}

	render()
	router.Source().Execute(key.FocusCmd{Tag: &app.keyboardShortcutTag})
	render()
	router.Queue(key.Event{Name: "Z", Modifiers: key.ModCtrl, State: key.Press})
	render()
	undone, _ := app.workflowGraph("ws-one").node("prompt")
	if undone.Position != image.Pt(72, 92) {
		t.Fatalf("Ctrl+Z position=%v", undone.Position)
	}
	router.Queue(key.Event{Name: "Z", Modifiers: key.ModCtrl | key.ModShift, State: key.Press})
	render()
	redone, _ := app.workflowGraph("ws-one").node("prompt")
	if redone.Position != image.Pt(420, 320) {
		t.Fatalf("Ctrl+Shift+Z position=%v", redone.Position)
	}

	app.selectWorkflowNode("ws-one", "prompt")
	router.Queue(key.Event{Name: "M", Modifiers: key.ModCtrl, State: key.Press})
	render()
	prompt, _ := app.workflowGraph("ws-one").node("prompt")
	if prompt.Enabled {
		t.Fatal("Ctrl+M did not disable selected workflow node")
	}
	router.Queue(key.Event{Name: key.NameDeleteForward, State: key.Press})
	render()
	if _, ok := app.workflowGraph("ws-one").node("prompt"); ok {
		t.Fatal("Delete did not remove selected workflow node")
	}
	router.Queue(key.Event{Name: "Z", Modifiers: key.ModCtrl, State: key.Press})
	render()
	prompt, ok := app.workflowGraph("ws-one").node("prompt")
	if !ok || prompt.Enabled {
		t.Fatalf("undo delete prompt=%+v ok=%t want restored disabled node", prompt, ok)
	}
}

func TestWorkflowHistoryAvailabilityPublishesToDetachedWindows(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-one")
	app.setWorkflowNodePosition("ws-one", "prompt", image.Pt(260, 210))
	app.publishDesktopState(snapshot{})
	workspace, ok := app.desktopSnapshot().workspace("ws-one")
	if !ok || !workspace.CanUndo || workspace.CanRedo {
		t.Fatalf("published history after edit=%+v ok=%t", workspace, ok)
	}
	app.undoWorkflowGraph("ws-one")
	app.publishDesktopState(snapshot{})
	workspace, ok = app.desktopSnapshot().workspace("ws-one")
	if !ok || workspace.CanUndo || !workspace.CanRedo {
		t.Fatalf("published history after undo=%+v ok=%t", workspace, ok)
	}
}

func TestWorkflowNodeCatalogEditsParticipateInHistory(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-one")
	app.selectWorkflowNode("ws-one", "generate")
	if !app.deleteSelectedWorkflowNode("ws-one") {
		t.Fatal("delete selected generate returned false")
	}
	if _, ok := app.workflowGraph("ws-one").node("generate"); ok {
		t.Fatal("deleted generate remained in app graph")
	}
	if app.selectedWorkflowNode("ws-one") != "prompt" {
		t.Fatalf("fallback selection=%q want prompt", app.selectedWorkflowNode("ws-one"))
	}
	if !app.undoWorkflowGraph("ws-one") {
		t.Fatal("undo delete returned false")
	}
	if _, ok := app.workflowGraph("ws-one").node("generate"); !ok {
		t.Fatal("undo did not restore generate")
	}
	if !app.redoWorkflowGraph("ws-one") {
		t.Fatal("redo delete returned false")
	}
	if err := app.addWorkflowNode("ws-one", "generate"); err != nil {
		t.Fatalf("re-add generate: %v", err)
	}
	if app.selectedWorkflowNode("ws-one") != "generate" {
		t.Fatalf("selection after add=%q", app.selectedWorkflowNode("ws-one"))
	}
	if !app.toggleSelectedWorkflowNodeEnabled("ws-one") {
		t.Fatal("toggle generate returned false")
	}
	generate, _ := app.workflowGraph("ws-one").node("generate")
	if generate.Enabled {
		t.Fatal("generate remained enabled")
	}
	if !app.undoWorkflowGraph("ws-one") {
		t.Fatal("undo disable returned false")
	}
	generate, _ = app.workflowGraph("ws-one").node("generate")
	if !generate.Enabled {
		t.Fatal("undo did not re-enable generate")
	}
}

func TestWorkflowNodePropertiesAndTitleParticipateInHistory(t *testing.T) {
	app := newWorkflowHistoryTestApp("ws-one")
	graph := app.workflowGraph("ws-one")
	prompt, _ := graph.node("prompt")
	properties := cloneWorkflowProperties(prompt.Properties)
	properties[workflowPropertyPrompt] = "custom branch prompt"
	if !app.configureWorkflowNode("ws-one", prompt.ID, "主分支提示词", properties, true) {
		t.Fatal("configure prompt returned false")
	}
	configured, _ := app.workflowGraph("ws-one").node(prompt.ID)
	if configured.Title != "主分支提示词" || configured.Properties[workflowPropertyPrompt] != "custom branch prompt" {
		t.Fatalf("configured prompt=%+v", configured)
	}
	app.workflowEditingNodeKey = workflowNodeEditorKey("ws-one", prompt.ID)
	if !app.undoWorkflowGraph("ws-one") {
		t.Fatal("undo properties returned false")
	}
	restored, _ := app.workflowGraph("ws-one").node(prompt.ID)
	if restored.Title != prompt.Title || restored.Properties[workflowPropertyPrompt] != prompt.Properties[workflowPropertyPrompt] {
		t.Fatalf("restored prompt=%+v", restored)
	}
	if app.workflowEditingNodeKey != "" {
		t.Fatalf("undo retained stale editor key %q", app.workflowEditingNodeKey)
	}
}
