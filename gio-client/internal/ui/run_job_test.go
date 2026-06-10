package ui

import (
	"strings"
	"testing"

	"github.com/yuanhua/image-gptcodex/pkg/client"
	"image-studio/gio-client/internal/kernel"
)

func TestRequestedRunConcurrencyUsesLoopSetting(t *testing.T) {
	if got := requestedRunConcurrency(4, false, 0); got != 4 {
		t.Fatalf("requestedRunConcurrency(batch)= %d want 4", got)
	}
	if got := requestedRunConcurrency(10, true, 3); got != 3 {
		t.Fatalf("requestedRunConcurrency(loop)= %d want 3", got)
	}
	if got := requestedRunConcurrency(2, true, 9); got != 2 {
		t.Fatalf("requestedRunConcurrency(loop clamped)= %d want 2", got)
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
