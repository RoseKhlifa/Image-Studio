package ui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/yuanhua/image-gptcodex/pkg/client"
	"image-studio/gio-client/internal/kernel"
	sharedCompat "image-studio/shared/compat"
)

func TestRequestedRunConcurrencyUsesLoopSetting(t *testing.T) {
	if got := requestedRunConcurrency(4, true, 2, false, 0); got != 2 {
		t.Fatalf("requestedRunConcurrency(batch)= %d want 2", got)
	}
	if got := requestedRunConcurrency(10, false, 0, true, 3); got != 3 {
		t.Fatalf("requestedRunConcurrency(loop)= %d want 3", got)
	}
	if got := requestedRunConcurrency(2, false, 0, true, 9); got != 2 {
		t.Fatalf("requestedRunConcurrency(loop clamped)= %d want 2", got)
	}
	if got := requestedRunConcurrency(2, true, 9, true, 1); got != 2 {
		t.Fatalf("requestedRunConcurrency(batch overrides loop)= %d want 2", got)
	}
}

func TestBuildRunExecutionPlanKeepsModeSpecificTotalsAndConcurrency(t *testing.T) {
	total, concurrency := buildRunExecutionPlan(17, true, 3, false, 0)
	if total != 17 || concurrency != 3 {
		t.Fatalf("batch plan=(%d,%d) want (17,3)", total, concurrency)
	}

	total, concurrency = buildRunExecutionPlan(42, false, 0, true, 4)
	if total != 42 || concurrency != 4 {
		t.Fatalf("loop plan=(%d,%d) want (42,4)", total, concurrency)
	}

	total, concurrency = buildRunExecutionPlan(17, false, 0, false, 0)
	if total != 9 || concurrency != 9 {
		t.Fatalf("regular plan=(%d,%d) want (9,9)", total, concurrency)
	}
}

func TestRunPreviewSlotPoolOnlyReusesReleasedSlots(t *testing.T) {
	slots := newRunPreviewSlotPool(2)
	first := <-slots
	second := <-slots
	if first != 0 || second != 1 {
		t.Fatalf("initial slots=(%d,%d) want (0,1)", first, second)
	}
	select {
	case slot := <-slots:
		t.Fatalf("received occupied slot %d", slot)
	default:
	}
	slots <- second
	if got := <-slots; got != second {
		t.Fatalf("reused slot=%d want %d", got, second)
	}
}

func TestRunConcurrencyLimitErrorMatchesMode(t *testing.T) {
	if got := runConcurrencyLimitError(client.APIModeResponses, 2, 4, false, false); got != "Responses API 并发限制 2,当前还可提交 2 个,本次需要 4 个。" {
		t.Fatalf("regular error=%q", got)
	}
	if got := runConcurrencyLimitError(client.APIModeImages, 3, 5, true, false); got != "Images API 并发限制 3,当前还可提交 3 个,批处理并发需要 5 个。" {
		t.Fatalf("batch error=%q", got)
	}
	if got := runConcurrencyLimitError(client.APIModeResponses, 1, 2, false, true); got != "Responses API 并发限制 1,当前还可提交 1 个,循环模式并发需要 2 个。" {
		t.Fatalf("loop error=%q", got)
	}
}

func TestValidateKernelRuntimeForRunMatchesRemoteConstraints(t *testing.T) {
	if got := validateKernelRuntimeForRun("local", kernel.Config{
		ProxyMode:          client.ProxyModeCustom,
		APIMode:            client.APIModeResponses,
		ResponsesTransport: client.ResponsesTransportWebSocket,
	}); got != "" {
		t.Fatalf("local mode should not block, got %q", got)
	}
	if got := validateKernelRuntimeForRun("remote", kernel.Config{
		ProxyMode: client.ProxyModeCustom,
	}); got != "当前远程内核不能控制代理,请切回本地内核或使用 Android 原生运行" {
		t.Fatalf("proxy constraint=%q", got)
	}
	if got := validateKernelRuntimeForRun("remote", kernel.Config{
		ProxyMode:          client.ProxyModeSystem,
		APIMode:            client.APIModeResponses,
		ResponsesTransport: client.ResponsesTransportWebSocket,
	}); got != "当前远程内核模式暂不支持 Responses WebSocket mode，请切回本地内核或关闭该开关。" {
		t.Fatalf("websocket constraint=%q", got)
	}
}

func TestStartRunBlocksWhenConcurrencyLimitTooLow(t *testing.T) {
	app := &App{
		api:        string(client.APIModeResponses),
		batchCount: 4,
	}
	app.apiKeyInput.SetText("sk-test")
	app.baseURLInput.SetText("https://example.com")
	app.promptInput.SetText("hello")
	app.concurrencyLimitInput.SetText("2")

	app.startRun()

	if app.isRunning() {
		t.Fatal("startRun should not enter running state when concurrency limit is exceeded")
	}
	if len(app.logs) == 0 {
		t.Fatal("expected concurrency limit warning log")
	}
	if !strings.Contains(app.logs[len(app.logs)-1], "并发限制 2") || !strings.Contains(app.logs[len(app.logs)-1], "本次需要 4 个") {
		t.Fatalf("unexpected log: %q", app.logs[len(app.logs)-1])
	}
}

func TestStartRunBlocksWhenLoopAutoSaveDirMissing(t *testing.T) {
	app := &App{
		api:             string(client.APIModeResponses),
		loopEnabled:     true,
		loopAutoSave:    true,
		loopTotalCount:  10,
		loopConcurrency: 2,
	}
	app.apiKeyInput.SetText("sk-test")
	app.baseURLInput.SetText("https://example.com")
	app.promptInput.SetText("hello")
	app.loopAutoSaveDirInput.SetText("")

	app.startRun()

	if app.isRunning() {
		t.Fatal("startRun should not enter running state when loop auto save dir is missing")
	}
	if len(app.logs) == 0 {
		t.Fatal("expected missing auto save dir warning log")
	}
	if !strings.Contains(app.logs[len(app.logs)-1], "请先为循环出图配置自动另存为路径") {
		t.Fatalf("unexpected log: %q", app.logs[len(app.logs)-1])
	}
}

func TestStartRunBlocksInvalidWorkflowBeforeSubmitting(t *testing.T) {
	graph := defaultWorkflowGraph()
	var err error
	graph, err = toggleWorkflowConnection(graph, workflowEdgeModel{
		FromNode: "preview", FromPort: "image", ToNode: "export", ToPort: "image",
	})
	if err != nil {
		t.Fatalf("disconnect export: %v", err)
	}
	app := &App{
		experienceMode:        experienceModeWorkflow,
		activeWorkspaceID:     "ws-1",
		workflowGraphs:        map[string]workflowGraphModel{"ws-1": graph},
		workflowSelectedNodes: map[string]string{"ws-1": "export"},
	}

	app.startRun()

	if app.isRunning() {
		t.Fatal("invalid workflow entered running state")
	}
	if len(app.logs) == 0 || !strings.Contains(app.logs[len(app.logs)-1], "工作流无效") {
		t.Fatalf("missing workflow validation log: %#v", app.logs)
	}
}

func TestRetryLastRunDoesNotBypassWorkflowValidation(t *testing.T) {
	graph, err := toggleWorkflowConnection(defaultWorkflowGraph(), workflowEdgeModel{
		FromNode: "generate", FromPort: "job", ToNode: "preview", ToPort: "job",
	})
	if err != nil {
		t.Fatalf("disconnect preview: %v", err)
	}
	app := &App{
		experienceMode:        experienceModeWorkflow,
		activeWorkspaceID:     "ws-1",
		workflowGraphs:        map[string]workflowGraphModel{"ws-1": graph},
		workflowSelectedNodes: map[string]string{"ws-1": "preview"},
		lastRunValid:          true,
		lastRunBatchCount:     1,
		lastRunConfig:         kernel.Config{Mode: client.ModeGenerate},
	}

	app.retryLastRun()

	if app.isRunning() {
		t.Fatal("retry bypassed workflow validation")
	}
	if len(app.logs) == 0 || !strings.Contains(app.logs[len(app.logs)-1], "工作流无效") {
		t.Fatalf("missing retry validation log: %#v", app.logs)
	}
}

func TestHistoryItemByRawPathFindsMatchingItem(t *testing.T) {
	items := []sharedCompat.HistoryItem{
		{ID: "a", RawPath: "/tmp/a.txt"},
		{ID: "b", RawPath: "/tmp/b.txt"},
	}
	got, ok := historyItemByRawPath(items, "/tmp/b.txt")
	if !ok {
		t.Fatal("expected raw path match")
	}
	if got.ID != "b" {
		t.Fatalf("history item=%q want b", got.ID)
	}
}

func TestLoadCanvasImmediatePreviewForStateUsesImageB64WhenPathsMissing(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	fill := color.NRGBA{R: 0x44, G: 0x77, B: 0xaa, A: 0xff}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	app := &App{}
	got := app.loadCanvasImmediatePreviewForState("", resultState{
		Item: sharedCompat.HistoryItem{
			ID:       "hist-1",
			ImageB64: base64.StdEncoding.EncodeToString(buf.Bytes()),
		},
	})
	if got == nil {
		t.Fatal("expected inline preview image")
	}
}
