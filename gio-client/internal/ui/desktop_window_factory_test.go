package ui

import (
	"errors"
	"image"
	"testing"
	"time"

	"image-studio/gio-client/internal/windowing"

	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/unit"
)

type fakeDesktopWindowActions struct {
	pending system.Action
	takes   int
}

func (actions *fakeDesktopWindowActions) Take() system.Action {
	actions.takes++
	pending := actions.pending
	actions.pending = 0
	return pending
}

type fakeDesktopWindowActionPerformer struct {
	started chan system.Action
}

func (window *fakeDesktopWindowActionPerformer) Perform(actions system.Action) {
	window.started <- actions
}

func detachedTestRoot() *App {
	return &App{desktopCommands: make(chan desktopCommand, 64)}
}

func detachedTestPublication(withImage bool) desktopPublication {
	workspace := desktopWorkspacePublication{
		ID:           "ws-1",
		Name:         "主视觉工作区",
		Prompt:       "电影感产品摄影",
		Mode:         "generate",
		Size:         "1536x1024",
		Quality:      "high",
		OutputFormat: "png",
		SourceCount:  2,
		SelectedNode: "generate",
		Graph:        defaultWorkflowGraph(),
		Runtime: map[string]workflowNodeRuntime{
			"prompt":   {Phase: workflowNodePhaseSuccess, Detail: "提示词已就绪", Progress: 1},
			"generate": {Phase: workflowNodePhaseRunning, Detail: "正在生成", Progress: 0.5},
		},
		Running:   true,
		Status:    "正在生成第 2 张图像",
		Completed: 1,
		Total:     4,
	}
	if withImage {
		workspace.ResultImage = image.NewNRGBA(image.Rect(0, 0, 64, 48))
		workspace.ResultRevision = 1
	}
	return desktopPublication{
		Revision:     3,
		DesktopStyle: desktopStyleMacOS,
		ColorMode:    desktopColorModeDark,
		ActiveID:     workspace.ID,
		Running:      true,
		Status:       "正在生成第 2 张图像",
		Completed:    1,
		Total:        4,
		Logs:         []string{"任务已开始", "主视觉工作区: 正在生成", "ERROR: 示例错误行"},
		Workspaces: []desktopWorkspacePublication{
			workspace,
			{ID: "ws-2", Name: "备选工作区", Graph: defaultWorkflowGraph()},
		},
	}
}

func TestDetachedProgressPreviewCachesImageOpByRevision(t *testing.T) {
	view := detachedViewForTest(t, detachedTestRoot(), windowing.RoleProgress, "ws-1")
	spec := desktopThemeSpec(desktopStyleMacOS, desktopColorModeDark)
	first := image.NewNRGBA(image.Rect(0, 0, 64, 48))
	second := image.NewNRGBA(image.Rect(0, 0, 96, 72))

	view.imagePreview(detachedTestContext(view, image.Pt(320, 180)), spec, first, 7)
	if got := view.previewOp.Size(); got != first.Bounds().Size() {
		t.Fatalf("initial preview op size=%v want %v", got, first.Bounds().Size())
	}
	view.imagePreview(detachedTestContext(view, image.Pt(320, 180)), spec, second, 7)
	if got := view.previewOp.Size(); got != first.Bounds().Size() {
		t.Fatalf("same revision rebuilt preview op: size=%v", got)
	}
	view.imagePreview(detachedTestContext(view, image.Pt(320, 180)), spec, second, 8)
	if got := view.previewOp.Size(); got != second.Bounds().Size() {
		t.Fatalf("new revision preview op size=%v want %v", got, second.Bounds().Size())
	}
}

func detachedTestContext(view *desktopWindowView, size image.Point) layout.Context {
	view.ops.Reset()
	return layout.Context{
		Ops:         &view.ops,
		Constraints: layout.Exact(size),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Now:         time.Unix(100, 0),
	}
}

func detachedViewForTest(t *testing.T, root *App, role windowing.Role, workspaceID string) *desktopWindowView {
	t.Helper()
	created, err := NewDesktopWindowFactory(root).NewView(windowing.Request{Role: role, WorkspaceID: workspaceID})
	if err != nil {
		t.Fatalf("NewView(%s): %v", role, err)
	}
	view, ok := created.(*desktopWindowView)
	if !ok {
		t.Fatalf("NewView(%s) type=%T want *desktopWindowView", role, created)
	}
	return view
}

func detachedNextCommand(t *testing.T, root *App) desktopCommand {
	t.Helper()
	select {
	case command := <-root.desktopCommands:
		return command
	default:
		t.Fatal("desktop command queue is empty")
		return desktopCommand{}
	}
}

func TestDesktopWindowFactoryCreatesFreshViewsForEveryRole(t *testing.T) {
	root := detachedTestRoot()
	roles := []windowing.Role{
		windowing.RoleCanvas,
		windowing.RoleConsole,
		windowing.RoleProgress,
		windowing.RoleWorkspace,
	}
	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			first := detachedViewForTest(t, root, role, " ws-1 ")
			second := detachedViewForTest(t, root, role, "ws-1")
			if first == second {
				t.Fatal("factory reused detached view")
			}
			if first.request.WorkspaceID != "ws-1" {
				t.Fatalf("workspace=%q want normalized ws-1", first.request.WorkspaceID)
			}
			if &first.ops == &second.ops || first.theme == second.theme || first.theme.Shaper == second.theme.Shaper {
				t.Fatal("detached views share ops, theme, or shaper")
			}
			if &first.consoleList == &second.consoleList || &first.workspaceList == &second.workspaceList {
				t.Fatal("detached views share list state")
			}
			first.workspaceButton("private")
			if len(second.workspaceButtons) != 0 {
				t.Fatal("detached views share clickable map")
			}
			first.canvas.overrides["prompt"] = image.Pt(1, 2)
			if len(second.canvas.overrides) != 0 {
				t.Fatal("detached views share canvas state")
			}
		})
	}
}

func TestDesktopWindowFactoryRejectsInvalidInputs(t *testing.T) {
	if _, err := NewDesktopWindowFactory(nil).NewView(windowing.Request{Role: windowing.RoleCanvas}); !errors.Is(err, errDesktopWindowRootRequired) {
		t.Fatalf("nil root error=%v want %v", err, errDesktopWindowRootRequired)
	}
	if _, err := NewDesktopWindowFactory(detachedTestRoot()).NewView(windowing.Request{Role: "unknown"}); err == nil {
		t.Fatal("unsupported role should fail")
	}
	view := detachedViewForTest(t, detachedTestRoot(), windowing.RoleCanvas, "ws-1")
	if err := view.Run(nil, nil); !errors.Is(err, errDesktopWindowRequired) {
		t.Fatalf("Run(nil) error=%v want %v", err, errDesktopWindowRequired)
	}
}

func TestDesktopWindowViewPerformsMergedActionsOnEventLoop(t *testing.T) {
	view := detachedViewForTest(t, detachedTestRoot(), windowing.RoleCanvas, "ws-1")
	actions := &fakeDesktopWindowActions{pending: system.ActionRaise | system.ActionClose}
	window := &fakeDesktopWindowActionPerformer{
		started: make(chan system.Action, 1),
	}

	view.performPendingWindowActions(window, actions)
	if actions.takes != 1 {
		t.Fatalf("Take calls=%d want 1", actions.takes)
	}
	select {
	case performed := <-window.started:
		if performed != system.ActionRaise|system.ActionClose {
			t.Fatalf("performed=%v want merged raise and close", performed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close action was not dispatched")
	}
	view.performPendingWindowActions(window, actions)
	select {
	case performed := <-window.started:
		t.Fatalf("empty action source performed again: %v", performed)
	default:
	}
}

func TestDesktopWindowLayoutsRunHeadlessly(t *testing.T) {
	publication := detachedTestPublication(true)
	cases := []struct {
		role windowing.Role
		size image.Point
	}{
		{windowing.RoleCanvas, image.Pt(1180, 820)},
		{windowing.RoleConsole, image.Pt(880, 620)},
		{windowing.RoleProgress, image.Pt(420, 260)},
		{windowing.RoleWorkspace, image.Pt(1280, 860)},
	}
	for _, testCase := range cases {
		t.Run(string(testCase.role), func(t *testing.T) {
			view := detachedViewForTest(t, detachedTestRoot(), testCase.role, "ws-1")
			dimensions := view.layout(detachedTestContext(view, testCase.size), publication)
			if dimensions.Size.X <= 0 || dimensions.Size.Y <= 0 {
				t.Fatalf("layout dimensions=%v", dimensions.Size)
			}
		})
	}
}

func TestDesktopWindowLayoutsEnqueueCommands(t *testing.T) {
	publication := detachedTestPublication(true)
	tests := []struct {
		name  string
		role  windowing.Role
		click func(*desktopWindowView)
		want  desktopCommand
	}{
		{
			name:  "run",
			role:  windowing.RoleCanvas,
			click: func(view *desktopWindowView) { view.runButton.Click() },
			want:  desktopCommand{Kind: desktopCommandRun, WorkspaceID: "ws-1"},
		},
		{
			name:  "cancel",
			role:  windowing.RoleProgress,
			click: func(view *desktopWindowView) { view.cancelButton.Click() },
			want:  desktopCommand{Kind: desktopCommandCancel},
		},
		{
			name:  "clear logs",
			role:  windowing.RoleConsole,
			click: func(view *desktopWindowView) { view.clearButton.Click() },
			want:  desktopCommand{Kind: desktopCommandClearLogs},
		},
		{
			name:  "open canvas",
			role:  windowing.RoleProgress,
			click: func(view *desktopWindowView) { view.openCanvasButton.Click() },
			want:  desktopCommand{Kind: desktopCommandOpenWindow, WorkspaceID: "ws-1", WindowRole: windowing.RoleCanvas},
		},
		{
			name:  "activate workspace",
			role:  windowing.RoleWorkspace,
			click: func(view *desktopWindowView) { view.activateButton.Click() },
			want:  desktopCommand{Kind: desktopCommandActivate, WorkspaceID: "ws-1"},
		},
		{
			name:  "raise main",
			role:  windowing.RoleCanvas,
			click: func(view *desktopWindowView) { view.raiseMainButton.Click() },
			want:  desktopCommand{Kind: desktopCommandRaiseMain},
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			root := detachedTestRoot()
			view := detachedViewForTest(t, root, testCase.role, "ws-1")
			testCase.click(view)
			view.layout(detachedTestContext(view, image.Pt(1280, 860)), publication)
			if got := detachedNextCommand(t, root); got != testCase.want {
				t.Fatalf("command=%+v want %+v", got, testCase.want)
			}
		})
	}
}

func TestDetachedConsoleWorkspaceSelectorEnqueuesActivate(t *testing.T) {
	root := detachedTestRoot()
	view := detachedViewForTest(t, root, windowing.RoleConsole, "ws-1")
	view.workspaceButton("console:ws-2").Click()
	view.layout(detachedTestContext(view, image.Pt(880, 620)), detachedTestPublication(false))
	if got := detachedNextCommand(t, root); got != (desktopCommand{Kind: desktopCommandActivate, WorkspaceID: "ws-2"}) {
		t.Fatalf("command=%+v", got)
	}
	if view.consoleFilterID != "ws-2" {
		t.Fatalf("filter=%q want ws-2", view.consoleFilterID)
	}
}

func TestDesktopCanvasCallbacksEnqueueSelectAndMove(t *testing.T) {
	root := detachedTestRoot()
	view := detachedViewForTest(t, root, windowing.RoleCanvas, "ws-1")
	callbacks := view.canvasCallbacks("ws-1")
	callbacks.Select("preview")
	callbacks.Move("preview", image.Pt(320, 240))
	if got := detachedNextCommand(t, root); got != (desktopCommand{Kind: desktopCommandSelectNode, WorkspaceID: "ws-1", NodeID: "preview"}) {
		t.Fatalf("select command=%+v", got)
	}
	root.desktopPendingMoveMu.Lock()
	got := root.desktopPendingMoves["ws-1:preview"]
	root.desktopPendingMoveMu.Unlock()
	if got != (desktopCommand{Kind: desktopCommandMoveNode, WorkspaceID: "ws-1", NodeID: "preview", Position: image.Pt(320, 240)}) {
		t.Fatalf("move command=%+v", got)
	}
}

func TestDetachedWorkspaceDraftFlowsThroughMainActor(t *testing.T) {
	root := &App{
		desktopCommands:   make(chan desktopCommand, 8),
		activeWorkspaceID: "ws-1",
		workspaces: []workspaceState{{
			ID: "ws-1", Name: "Main", Prompt: "before", Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png",
		}},
		workflowGraphs:        map[string]workflowGraphModel{"ws-1": defaultWorkflowGraph()},
		workflowSelectedNodes: map[string]string{"ws-1": "generate"},
	}
	view := detachedViewForTest(t, root, windowing.RoleWorkspace, "ws-1")
	workspace := desktopWorkspacePublication{ID: "ws-1", Prompt: "before", Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png"}
	view.syncDetachedWorkspaceDraft(workspace)
	view.promptEditor.SetText("after")
	view.negativeEditor.SetText("noise")
	view.draftQuality = "high"
	view.draftFormat = "webp"
	if !view.enqueueDetachedDraft("ws-1") {
		t.Fatal("draft command was not enqueued")
	}
	root.processDesktopCommands()
	if root.promptInput.Text() != "after" || root.negativePromptInput.Text() != "noise" || root.quality != "high" || root.format != "webp" {
		t.Fatalf("actor draft prompt=%q negative=%q quality=%q format=%q", root.promptInput.Text(), root.negativePromptInput.Text(), root.quality, root.format)
	}
}

func TestDesktopWindowMissingWorkspaceAndThemeIsolation(t *testing.T) {
	previousFluent := fluent
	previousInstalled := installedDesktopTheme
	view := detachedViewForTest(t, detachedTestRoot(), windowing.RoleWorkspace, "missing")
	dimensions := view.layout(detachedTestContext(view, image.Pt(840, 560)), detachedTestPublication(false))
	if dimensions.Size.X <= 0 || dimensions.Size.Y <= 0 {
		t.Fatalf("missing layout dimensions=%v", dimensions.Size)
	}
	if fluent != previousFluent {
		t.Fatal("detached layout mutated fluent package palette")
	}
	if installedDesktopTheme != previousInstalled {
		t.Fatal("detached layout mutated installed desktop theme")
	}
}
