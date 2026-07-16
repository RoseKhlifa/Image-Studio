package ui

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/yuanhua/image-gptcodex/pkg/client"
	giodCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/kernel"
	sharedCompat "image-studio/shared/compat"
)

func writeSolidTestPNG(t *testing.T, path string, fill color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	writeImagePNG(t, path, img)
}

func isolateGioStableDataRoot(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	originalRoot := giodCompat.StableDataRootForTest()
	giodCompat.SetStableDataRootForTest(func() (string, error) { return root, nil })
	t.Cleanup(func() { giodCompat.SetStableDataRootForTest(originalRoot) })
}

func writeSizedSolidTestPNG(t *testing.T, path string, width int, height int, fill color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	writeImagePNG(t, path, img)
}

func writeImagePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create png %s: %v", path, err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode png %s: %v", path, err)
	}
}

func assertImagePixelColor(t *testing.T, img image.Image, want color.NRGBA) {
	t.Helper()
	got := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
	if got != want {
		t.Fatalf("pixel=%#v want %#v", got, want)
	}
}

func waitForImage(t *testing.T, fn func() image.Image) image.Image {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if img := fn(); img != nil {
			return img
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for image")
	return nil
}

func TestCopyImageFileCopiesToExplicitPath(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.png")
	if err := os.WriteFile(src, []byte("image"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	dst := filepath.Join(dir, "nested", "copy.png")
	saved, err := copyImageFile(src, dst)
	if err != nil {
		t.Fatalf("copyImageFile: %v", err)
	}
	if saved != dst {
		t.Fatalf("saved=%q want %q", saved, dst)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read copied: %v", err)
	}
	if string(data) != "image" {
		t.Fatalf("copied data=%q", data)
	}
}

func TestCopyImageFileDirectoryTargetKeepsSourceName(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.webp")
	if err := os.WriteFile(src, []byte("image"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	targetDir := filepath.Join(dir, "target")
	if err := os.Mkdir(targetDir, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	saved, err := copyImageFile(src, targetDir)
	if err != nil {
		t.Fatalf("copyImageFile: %v", err)
	}
	want := filepath.Join(targetDir, "source.webp")
	if saved != want {
		t.Fatalf("saved=%q want %q", saved, want)
	}
}

func TestMatchHistoryQueryMatchesPromptAndPath(t *testing.T) {
	item := sharedCompat.HistoryItem{
		Prompt:        "生成一张雪山海报",
		RevisedPrompt: "cinematic snow mountain poster",
		SavedPath:     "/tmp/snow.png",
		Size:          "1024x1024",
		Quality:       "high",
	}
	if !matchHistoryQuery(item, "雪山") {
		t.Fatalf("expected prompt match")
	}
	if !matchHistoryQuery(item, "snow.png") {
		t.Fatalf("expected path match")
	}
	if matchHistoryQuery(item, "desert") {
		t.Fatalf("unexpected query match")
	}
}

func TestTodayHistoryCountUsesLocalDayBoundary(t *testing.T) {
	now := time.Date(2026, time.May, 31, 15, 4, 0, 0, time.Local)
	items := []sharedCompat.HistoryItem{
		{ID: "a", CreatedAt: now.Add(-2 * time.Hour).UnixMilli()},
		{ID: "b", CreatedAt: now.Add(-26 * time.Hour).UnixMilli()},
	}
	if got := todayHistoryCount(items, now); got != 1 {
		t.Fatalf("todayHistoryCount=%d want 1", got)
	}
}

func TestFilteredHistoryItemsRespectsQueryModeAndDate(t *testing.T) {
	now := time.Date(2026, time.May, 31, 15, 4, 0, 0, time.Local)
	items := []sharedCompat.HistoryItem{
		{ID: "a", Prompt: "城市夜景", Mode: "generate", CreatedAt: now.Add(-2 * time.Hour).UnixMilli()},
		{ID: "b", Prompt: "城市夜景", Mode: "edit", CreatedAt: now.Add(-48 * time.Hour).UnixMilli()},
		{ID: "c", Prompt: "森林雾气", Mode: "generate", CreatedAt: now.Add(-2 * time.Hour).UnixMilli()},
	}
	got := filteredHistoryItems(items, "城市", "generate", "today", now)
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("filteredHistoryItems=%v want only a", got)
	}
}

func TestAugmentPromptWithStyle(t *testing.T) {
	got := augmentPromptWithStyle("一只猫坐在窗边", "anime")
	want := "一只猫坐在窗边, anime style, cel shading, vibrant colors, detailed illustration"
	if got != want {
		t.Fatalf("augmentPromptWithStyle=%q want %q", got, want)
	}
	if same := augmentPromptWithStyle("一只猫", ""); same != "一只猫" {
		t.Fatalf("augmentPromptWithStyle without style=%q", same)
	}
}

func TestNormalizeBatchCount(t *testing.T) {
	if got := normalizeBatchCount(0); got != 1 {
		t.Fatalf("normalizeBatchCount(0)=%d want 1", got)
	}
	if got := normalizeBatchCount(12); got != 9 {
		t.Fatalf("normalizeBatchCount(12)=%d want 9", got)
	}
	if got := normalizeBatchCount(4); got != 4 {
		t.Fatalf("normalizeBatchCount(4)=%d want 4", got)
	}
}

func TestBuildPromptSuggestionsMergesHistorySources(t *testing.T) {
	promptHistory := []string{"一只猫坐在窗边", "夜色城市海报"}
	history := []sharedCompat.HistoryItem{
		{ID: "a", Prompt: "夜色城市海报"},
		{ID: "b", Prompt: "山谷晨雾风景"},
	}
	got := buildPromptSuggestions(promptHistory, history)
	if len(got) != 3 {
		t.Fatalf("len(buildPromptSuggestions)=%d want 3", len(got))
	}
	if got[0] != "一只猫坐在窗边" || got[2] != "山谷晨雾风景" {
		t.Fatalf("buildPromptSuggestions=%v", got)
	}
}

func TestFindPromptGroupForItemReturnsGroupedItems(t *testing.T) {
	items := []sharedCompat.HistoryItem{
		{ID: "1", Prompt: "cat poster"},
		{ID: "2", Prompt: "cat poster"},
		{ID: "3", Prompt: "dog poster"},
	}
	group, ok := findPromptGroupForItem(items, "2")
	if !ok {
		t.Fatalf("expected prompt group")
	}
	if len(group.Items) != 2 {
		t.Fatalf("group size=%d want 2", len(group.Items))
	}
}

func TestPromptGroupKeyForEntriesReturnsVisibleGroupKey(t *testing.T) {
	items := []sharedCompat.HistoryItem{
		{ID: "1", Prompt: "cat poster"},
		{ID: "2", Prompt: "cat poster"},
		{ID: "3", Prompt: "dog poster"},
	}
	entries := buildHistoryPromptEntriesLimited(items, 2)
	if got := promptGroupKeyForEntries(entries, "2"); got != "prompt:cat poster" {
		t.Fatalf("promptGroupKeyForEntries(grouped)=%q want prompt:cat poster", got)
	}
	if got := promptGroupKeyForEntries(entries, "3"); got != "prompt:dog poster" {
		t.Fatalf("promptGroupKeyForEntries(single)=%q want prompt:dog poster", got)
	}
	if got := promptGroupKeyForEntries(entries, "missing"); got != "" {
		t.Fatalf("promptGroupKeyForEntries(missing)=%q want empty", got)
	}
}

func TestCurrentConfigIncludesResponsesTransportAndReasoning(t *testing.T) {
	app := &App{
		mode:               string(client.ModeGenerate),
		api:                string(client.APIModeResponses),
		policy:             string(client.RequestPolicyOpenAI),
		responsesTransport: string(client.ResponsesTransportWebSocket),
		reasoningEffort:    "high",
	}
	cfg := app.currentConfig()
	if cfg.ResponsesTransport != client.ResponsesTransportWebSocket {
		t.Fatalf("responses transport=%q want websocket", cfg.ResponsesTransport)
	}
	if cfg.ReasoningEffort != "high" {
		t.Fatalf("reasoning effort=%q want high", cfg.ReasoningEffort)
	}
}

func TestCurrentConfigNormalizesUnsupportedSizeForCurrentModel(t *testing.T) {
	app := &App{
		api:    string(client.APIModeResponses),
		policy: string(client.RequestPolicyOpenAI),
		size:   "2048x1152",
	}
	app.imageModelInput.SetText("dall-e-3")

	cfg := app.currentConfig()
	if cfg.Size != "1024x1024" {
		t.Fatalf("cfg.Size=%q want 1024x1024", cfg.Size)
	}

	app.size = "auto"
	cfg = app.currentConfig()
	if cfg.Size != "1024x1024" {
		t.Fatalf("cfg.Size for dalle3 auto=%q want 1024x1024", cfg.Size)
	}

	app.imageModelInput.SetText("gpt-image-1.5")
	cfg = app.currentConfig()
	if cfg.Size != "auto" {
		t.Fatalf("cfg.Size for legacy gpt-image auto=%q want auto", cfg.Size)
	}
}

func TestCurrentConfigUsesManualEditAutoAspectForSourceImage(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source-21x9.png")
	writeSizedSolidTestPNG(t, sourcePath, 2100, 900, color.NRGBA{R: 0x44, G: 0x77, B: 0xaa, A: 0xff})

	app := &App{
		mode:                     string(client.ModeEdit),
		api:                      string(client.APIModeResponses),
		policy:                   string(client.RequestPolicyOpenAI),
		editAutoAspectResolution: "1k",
	}
	app.imageModelInput.SetText("gpt-image-2")
	app.sourcePathsInput.SetText(sourcePath)

	cfg := app.currentConfig()
	if cfg.Size != "1536x656" {
		t.Fatalf("cfg.Size=%q want 1536x656", cfg.Size)
	}

	app.imageModelInput.SetText("dall-e-3")
	cfg = app.currentConfig()
	if cfg.Size != "1792x1024" {
		t.Fatalf("cfg.Size with dalle3=%q want 1792x1024", cfg.Size)
	}
}

func TestCurrentConfigUsesManualEditAutoAspectForImplicitCurrentImage(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "implicit-current.png")
	writeSizedSolidTestPNG(t, sourcePath, 1600, 900, color.NRGBA{R: 0x66, G: 0x99, B: 0xcc, A: 0xff})

	app := &App{
		mode:                     string(client.ModeEdit),
		api:                      string(client.APIModeResponses),
		policy:                   string(client.RequestPolicyOpenAI),
		editAutoAspectResolution: "1k",
		result: resultState{
			SavedPath: sourcePath,
			Item: sharedCompat.HistoryItem{
				ID:        "current-result",
				SavedPath: sourcePath,
			},
			HasItem: true,
		},
	}
	app.imageModelInput.SetText("gpt-image-2")

	cfg := app.currentConfig()
	if cfg.Size != "1536x864" {
		t.Fatalf("cfg.Size=%q want 1536x864", cfg.Size)
	}
}

func TestWorkspaceSnapshotPreservesEditAutoAspectResolution(t *testing.T) {
	app := &App{
		activeWorkspaceID:        "ws-1",
		workspaces:               []workspaceState{{ID: "ws-1", Name: "图片 1"}},
		editAutoAspectResolution: "1k",
	}

	snapshot := app.buildWorkspaceSnapshot()
	if snapshot.EditAutoAspectResolution != "1k" {
		t.Fatalf("snapshot.EditAutoAspectResolution=%q want 1k", snapshot.EditAutoAspectResolution)
	}

	app.editAutoAspectResolution = ""
	app.applyWorkspace(snapshot)
	if app.editAutoAspectResolution != "1k" {
		t.Fatalf("app.editAutoAspectResolution=%q want 1k", app.editAutoAspectResolution)
	}
}

func TestOpenCustomSizeModalUsesCurrentSizeOrDefault(t *testing.T) {
	app := &App{
		api:    string(client.APIModeResponses),
		policy: string(client.RequestPolicyOpenAI),
		size:   "auto",
	}
	app.imageModelInput.SetText("gpt-image-2")

	app.openCustomSizeModal()
	if !app.customSizeModalOpen {
		t.Fatal("custom size modal should be open")
	}
	if got := app.customSizeWidthInput.Text(); got != "1024" {
		t.Fatalf("width input=%q want 1024", got)
	}
	if got := app.customSizeHeightInput.Text(); got != "1024" {
		t.Fatalf("height input=%q want 1024", got)
	}

	app.size = "1536x864"
	app.openCustomSizeModal()
	if got := app.customSizeWidthInput.Text(); got != "1536" {
		t.Fatalf("width input=%q want 1536", got)
	}
	if got := app.customSizeHeightInput.Text(); got != "864" {
		t.Fatalf("height input=%q want 864", got)
	}
}

func TestApplyCustomSizeSetsExactSizeAndClosesModal(t *testing.T) {
	app := &App{
		api:    string(client.APIModeResponses),
		policy: string(client.RequestPolicyOpenAI),
	}
	app.imageModelInput.SetText("gpt-image-2")
	app.customSizeModalOpen = true
	app.customSizeWidthInput.SetText("1536")
	app.customSizeHeightInput.SetText("656")

	app.applyCustomSize()

	if app.customSizeModalOpen {
		t.Fatal("custom size modal should be closed after apply")
	}
	if app.size != "1536x656" {
		t.Fatalf("size=%q want 1536x656", app.size)
	}
}

func TestCurrentConfigEnablesPreviewOnlyResultForRemoteKernel(t *testing.T) {
	app := &App{
		kernelRuntimeMode: "remote",
	}
	cfg := app.currentConfig()
	if !cfg.PreviewOnlyResult {
		t.Fatal("remote kernel should enable preview-only result mode")
	}

	app.kernelRuntimeMode = "local"
	cfg = app.currentConfig()
	if cfg.PreviewOnlyResult {
		t.Fatal("local kernel should not enable preview-only result mode")
	}
}

func TestCurrentConfigUsesPreviewOnlyResultAsDataURLForRemoteEditFallback(t *testing.T) {
	app := &App{
		mode:              string(client.ModeEdit),
		kernelRuntimeMode: "remote",
		result: resultState{
			Item: sharedCompat.HistoryItem{
				OutputFormat: "png",
				ImageB64:     base64.StdEncoding.EncodeToString([]byte("image-bytes")),
			},
		},
	}
	cfg := app.currentConfig()
	if len(cfg.SourcePaths) != 0 {
		t.Fatalf("source paths=%v want empty for remote preview-only fallback", cfg.SourcePaths)
	}
	if len(cfg.SourceImageDataURLs) != 1 || !strings.HasPrefix(cfg.SourceImageDataURLs[0], "data:image/png;base64,") {
		t.Fatalf("source image data urls=%v want one png data URL", cfg.SourceImageDataURLs)
	}
}

func TestCurrentConfigKeepsMixedFileAndVirtualEditSourcesInRemoteMode(t *testing.T) {
	app := &App{
		mode:              string(client.ModeEdit),
		kernelRuntimeMode: "remote",
	}
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString([]byte("image-bytes")), "preview.png", "png")
	app.sourcePathsInput.SetText("/tmp/a.png\n" + virtualPath)

	cfg := app.currentConfig()

	if len(cfg.SourcePaths) != 1 || cfg.SourcePaths[0] != "/tmp/a.png" {
		t.Fatalf("source paths=%v want [/tmp/a.png]", cfg.SourcePaths)
	}
	if len(cfg.SourceImageDataURLs) != 1 || !strings.HasPrefix(cfg.SourceImageDataURLs[0], "data:image/png;base64,") {
		t.Fatalf("source image data urls=%v want one png data URL", cfg.SourceImageDataURLs)
	}
	if cfg.ParentID != "/tmp/a.png" {
		t.Fatalf("parentID=%q want /tmp/a.png", cfg.ParentID)
	}
}

func TestCurrentConfigUsesVirtualSavedPathAsImplicitEditSource(t *testing.T) {
	app := &App{
		mode:              string(client.ModeEdit),
		kernelRuntimeMode: "local",
	}
	virtualPath := registerVirtualImage(testPNGBase64(t, color.NRGBA{R: 0x44, G: 0x77, B: 0xaa, A: 0xff}), "implicit-source.png", "png")
	app.result = resultState{
		SavedPath: virtualPath,
		Item: sharedCompat.HistoryItem{
			ID:           "virtual-current",
			SavedPath:    virtualPath,
			OutputFormat: "png",
		},
		HasItem: true,
	}

	cfg := app.currentConfig()

	if len(cfg.SourcePaths) != 0 {
		t.Fatalf("source paths=%v want empty for implicit virtual current result", cfg.SourcePaths)
	}
	if len(cfg.SourceImageDataURLs) != 1 || !strings.HasPrefix(cfg.SourceImageDataURLs[0], "data:image/png;base64,") {
		t.Fatalf("source image data urls=%v want one png data URL", cfg.SourceImageDataURLs)
	}
	if cfg.ParentID != virtualPath {
		t.Fatalf("parentID=%q want %q", cfg.ParentID, virtualPath)
	}
}

func TestCurrentConfigAugmentsPromptFromCanvasAnnotations(t *testing.T) {
	app := &App{
		mode: "edit",
		result: resultState{
			Image: image.NewNRGBA(image.Rect(0, 0, 100, 100)),
		},
		canvasAnnotations: []canvasAnnotation{
			{ID: "a", Rect: image.Rect(0, 0, 20, 20)},
		},
	}
	app.promptInput.SetText("focus prompt")

	cfg := app.currentConfig()
	if !strings.Contains(cfg.Prompt, "focus prompt") {
		t.Fatalf("prompt=%q want original prompt included", cfg.Prompt)
	}
	if !strings.Contains(cfg.Prompt, "标注区域") || !strings.Contains(cfg.Prompt, "上左部") {
		t.Fatalf("prompt=%q want augmented annotation region", cfg.Prompt)
	}
}

func TestStartPromptOptimizeBlocksWhenRemoteKernelCannotControlProxy(t *testing.T) {
	app := &App{
		kernelRuntimeMode: "remote",
		proxy:             client.ProxyModeCustom,
	}
	app.apiKeyInput.SetText("sk-test")
	app.baseURLInput.SetText("https://example.com")
	app.promptInput.SetText("hello")
	app.textModelInput.SetText(client.TextModel)
	app.proxyURLInput.SetText("http://127.0.0.1:7890")

	app.startPromptOptimize()

	if app.optimizingPrompt {
		t.Fatal("startPromptOptimize should not enter optimizing state when remote kernel cannot control proxy")
	}
	if len(app.logs) == 0 || !strings.Contains(app.logs[len(app.logs)-1], "当前远程内核不能控制代理") {
		t.Fatalf("unexpected logs: %v", app.logs)
	}
}

func TestResolveFallbackProfileConfigRequiresKeyAndBaseURL(t *testing.T) {
	state := sharedCompat.State{
		Profiles: []sharedCompat.UpstreamProfile{
			{
				ID:                 "fallback-1",
				Name:               "备用",
				APIMode:            string(client.APIModeResponses),
				ResponsesTransport: string(client.ResponsesTransportWebSocket),
				RequestPolicy:      string(client.RequestPolicyCompat),
				BaseURL:            "https://fallback.example",
				TextModelID:        "gpt-5.5",
				ImageModelID:       "gpt-image-2",
				ReasoningEffort:    "high",
			},
		},
	}
	got := resolveFallbackProfileConfig(state, "fallback-1", func(profileID string) (string, error) {
		if profileID != "fallback-1" {
			t.Fatalf("unexpected profileID %q", profileID)
		}
		return "sk-fallback", nil
	})
	if got == nil {
		t.Fatal("expected fallback profile config")
	}
	if got.APIKey != "sk-fallback" || got.BaseURL != "https://fallback.example" || got.ResponsesTransport != client.ResponsesTransportWebSocket || got.RequestPolicy != client.RequestPolicyCompat || got.ReasoningEffort != "high" {
		t.Fatalf("unexpected fallback config: %#v", got)
	}

	if got := resolveFallbackProfileConfig(state, "fallback-1", func(string) (string, error) {
		return "", nil
	}); got != nil {
		t.Fatalf("expected nil fallback when key missing, got %#v", got)
	}

	state.Profiles[0].BaseURL = ""
	if got := resolveFallbackProfileConfig(state, "fallback-1", func(string) (string, error) {
		return "sk-fallback", nil
	}); got != nil {
		t.Fatalf("expected nil fallback when baseURL missing, got %#v", got)
	}
}

func TestApplyHistoryThumbBackfillUpdatesInMemoryState(t *testing.T) {
	app := &App{}
	item := sharedCompat.HistoryItem{ID: "hist-1", SavedPath: "/tmp/full.png"}
	app.setHistoryLocked([]sharedCompat.HistoryItem{item})
	app.mu.Lock()
	app.batchResultIDs = []string{"hist-1"}
	app.batchResultsSnapshot = historyItemsByIDs(app.history, app.batchResultIDs)
	app.batchResultsKey = "hist-1"
	app.batchResultsRev = app.historyRev
	app.mu.Unlock()
	_ = app.historyPanelData(app.history)
	_ = app.historyTimelineData(app.history)
	_, _ = app.promptGroupForHistoryItem(app.history, "hist-1")
	app.result = resultState{Item: item, HasItem: true, Rev: 1}
	app.compare = resultState{Item: item, HasItem: true, Rev: 1}
	app.activeResultDetail = item
	groupItem := item
	app.activePromptGroup = historyPromptGroup{
		Key:            "prompt:test",
		Representative: item,
		Items:          []*sharedCompat.HistoryItem{&groupItem},
	}
	beforeRev := app.historyRev

	app.applyHistoryThumbBackfill(map[string]historyMediaBackfillUpdate{
		"hist-1": {ThumbPath: "/tmp/thumb.png", PreviewPath: "/tmp/preview.png"},
	})

	if app.historyRev != beforeRev {
		t.Fatalf("historyRev=%d want unchanged %d", app.historyRev, beforeRev)
	}
	if got := app.history[0].ThumbPath; got != "/tmp/thumb.png" {
		t.Fatalf("history thumb=%q want /tmp/thumb.png", got)
	}
	if got := app.history[0].PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("history preview=%q want /tmp/preview.png", got)
	}
	if got := app.result.Item.ThumbPath; got != "/tmp/thumb.png" {
		t.Fatalf("result thumb=%q want /tmp/thumb.png", got)
	}
	if got := app.result.Item.PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("result preview=%q want /tmp/preview.png", got)
	}
	if got := app.compare.Item.ThumbPath; got != "/tmp/thumb.png" {
		t.Fatalf("compare thumb=%q want /tmp/thumb.png", got)
	}
	if got := app.compare.Item.PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("compare preview=%q want /tmp/preview.png", got)
	}
	if got := app.activeResultDetail.ThumbPath; got != "/tmp/thumb.png" {
		t.Fatalf("detail thumb=%q want /tmp/thumb.png", got)
	}
	if got := app.activeResultDetail.PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("detail preview=%q want /tmp/preview.png", got)
	}
	if got := app.activePromptGroup.Representative.ThumbPath; got != "/tmp/thumb.png" {
		t.Fatalf("group representative thumb=%q want /tmp/thumb.png", got)
	}
	if got := app.activePromptGroup.Representative.PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("group representative preview=%q want /tmp/preview.png", got)
	}
	if got := app.activePromptGroup.Items[0].ThumbPath; got != "/tmp/thumb.png" {
		t.Fatalf("group item thumb=%q want /tmp/thumb.png", got)
	}
	if got := app.activePromptGroup.Items[0].PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("group item preview=%q want /tmp/preview.png", got)
	}
	panel := app.historyPanelData(app.history)
	if got := panel.latest.PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("panel latest preview=%q want /tmp/preview.png", got)
	}
	if got := panel.entries[0].Group.Representative.PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("panel group preview=%q want /tmp/preview.png", got)
	}
	timeline := app.historyTimelineData(app.history)
	if got := timeline.dayGroups[0].Entries[0].Group.Representative.PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("timeline group preview=%q want /tmp/preview.png", got)
	}
	group, ok := app.promptGroupForHistoryItem(app.history, "hist-1")
	if !ok {
		t.Fatal("promptGroupForHistoryItem missing hist-1")
	}
	if got := group.Representative.PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("group lookup preview=%q want /tmp/preview.png", got)
	}
	snap := app.readSnapshot()
	if got := snap.BatchResults[0].PreviewPath; got != "/tmp/preview.png" {
		t.Fatalf("batch result preview=%q want /tmp/preview.png", got)
	}
}

func TestCollectHistoryThumbBackfillCandidatesSkipsInflightAndDuplicates(t *testing.T) {
	dir := t.TempDir()
	full1 := filepath.Join(dir, "a.png")
	full2 := filepath.Join(dir, "b.png")
	writeSolidTestPNG(t, full1, color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	writeSolidTestPNG(t, full2, color.NRGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xff})

	items := []sharedCompat.HistoryItem{
		{ID: "1", SavedPath: full1},
		{ID: "2", SavedPath: full1},
		{ID: "3", SavedPath: full2},
		{ID: "4", SavedPath: full2, ThumbPath: "/tmp/existing-thumb.png", PreviewPath: "/tmp/existing-preview.png"},
	}
	got := collectHistoryThumbBackfillCandidates(items, map[string]struct{}{full2: {}})
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("candidates=%+v want only first unique non-inflight item", got)
	}
}

func TestCollectHistoryThumbBackfillCandidatesScansPastFirstBatchWindow(t *testing.T) {
	dir := t.TempDir()
	items := make([]sharedCompat.HistoryItem, 0, historyThumbBackfillLimit+1)
	for i := 0; i < historyThumbBackfillLimit; i++ {
		full := filepath.Join(dir, fmt.Sprintf("done-%02d.png", i))
		writeSolidTestPNG(t, full, color.NRGBA{R: 0x22, G: 0x44, B: 0x66, A: 0xff})
		items = append(items, sharedCompat.HistoryItem{
			ID:          fmt.Sprintf("done-%02d", i),
			SavedPath:   full,
			ThumbPath:   "/tmp/thumb.png",
			PreviewPath: "/tmp/preview.png",
		})
	}
	missingPath := filepath.Join(dir, "missing.png")
	writeSolidTestPNG(t, missingPath, color.NRGBA{R: 0x88, G: 0xaa, B: 0xcc, A: 0xff})
	items = append(items, sharedCompat.HistoryItem{
		ID:        "missing",
		SavedPath: missingPath,
	})

	got := collectHistoryThumbBackfillCandidates(items, nil)
	if len(got) != 1 || got[0].ID != "missing" {
		t.Fatalf("candidates=%+v want trailing missing item", got)
	}
}

func TestCollectHistoryThumbBackfillCandidatesPrefersPreviewOnlyItems(t *testing.T) {
	dir := t.TempDir()
	fullHeavy1 := filepath.Join(dir, "heavy-1.png")
	fullHeavy2 := filepath.Join(dir, "heavy-2.png")
	fullPreviewOnly := filepath.Join(dir, "preview-only.png")
	writeSolidTestPNG(t, fullHeavy1, color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	writeSolidTestPNG(t, fullHeavy2, color.NRGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xff})
	writeSolidTestPNG(t, fullPreviewOnly, color.NRGBA{R: 0x77, G: 0x88, B: 0x99, A: 0xff})

	items := []sharedCompat.HistoryItem{
		{ID: "heavy-1", SavedPath: fullHeavy1},
		{ID: "heavy-2", SavedPath: fullHeavy2},
		{ID: "preview-only", SavedPath: fullPreviewOnly, ThumbPath: "/tmp/existing-thumb.png"},
	}
	got := collectHistoryThumbBackfillCandidatesWithLimit(items, nil, 2)
	if len(got) != 2 {
		t.Fatalf("len(candidates)=%d want 2", len(got))
	}
	if got[0].ID != "preview-only" {
		t.Fatalf("first candidate=%q want preview-only", got[0].ID)
	}
	if got[1].ID != "heavy-1" {
		t.Fatalf("second candidate=%q want heavy-1", got[1].ID)
	}
}

func TestCollectHistoryPreviewOnlyBackfillCandidatesSkipsHeavyItems(t *testing.T) {
	dir := t.TempDir()
	fullHeavy := filepath.Join(dir, "heavy.png")
	fullPreviewOnly1 := filepath.Join(dir, "preview-only-1.png")
	fullPreviewOnly2 := filepath.Join(dir, "preview-only-2.png")
	writeSolidTestPNG(t, fullHeavy, color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	writeSolidTestPNG(t, fullPreviewOnly1, color.NRGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xff})
	writeSolidTestPNG(t, fullPreviewOnly2, color.NRGBA{R: 0x77, G: 0x88, B: 0x99, A: 0xff})

	items := []sharedCompat.HistoryItem{
		{ID: "heavy", SavedPath: fullHeavy},
		{ID: "preview-only-1", SavedPath: fullPreviewOnly1, ThumbPath: "/tmp/existing-thumb-1.png"},
		{ID: "preview-only-2", SavedPath: fullPreviewOnly2, ThumbPath: "/tmp/existing-thumb-2.png"},
	}
	got := collectHistoryPreviewOnlyBackfillCandidatesWithLimit(items, nil, 8)
	if len(got) != 2 {
		t.Fatalf("len(candidates)=%d want 2", len(got))
	}
	if got[0].ID != "preview-only-1" || got[1].ID != "preview-only-2" {
		t.Fatalf("candidates=%+v want only preview-only items in order", got)
	}
}

func TestBuildHistoryMediaBackfillUpdatesUsesExistingThumbForPreview(t *testing.T) {
	dir := t.TempDir()
	savedPath := filepath.Join(dir, "images", "source.png")
	if err := os.MkdirAll(filepath.Dir(savedPath), 0o700); err != nil {
		t.Fatalf("mkdir images: %v", err)
	}
	writeSizedSolidTestPNG(t, savedPath, 512, 384, color.NRGBA{R: 0xcc, G: 0x33, B: 0x22, A: 0xff})
	thumbPath := filepath.Join(dir, "thumbs", "source.png")
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0o700); err != nil {
		t.Fatalf("mkdir thumbs: %v", err)
	}
	writeSizedSolidTestPNG(t, thumbPath, 96, 72, color.NRGBA{R: 0x22, G: 0x66, B: 0xcc, A: 0xff})

	updates := buildHistoryMediaBackfillUpdates([]sharedCompat.HistoryItem{{
		ID:        "item-1",
		SavedPath: savedPath,
		ThumbPath: thumbPath,
	}})
	update, ok := updates["item-1"]
	if !ok {
		t.Fatalf("updates=%v want item-1 entry", updates)
	}
	if strings.TrimSpace(update.PreviewPath) == "" {
		t.Fatalf("preview path empty: %+v", update)
	}
	if strings.TrimSpace(update.ThumbPath) != "" {
		t.Fatalf("thumb path should stay empty when history already has one: %+v", update)
	}
	preview, err := decodeImageFile(update.PreviewPath)
	if err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	assertImagePixelColor(t, preview, color.NRGBA{R: 0x22, G: 0x66, B: 0xcc, A: 0xff})
}

func TestPrewarmHistoryThumbsPopulatesCache(t *testing.T) {
	dir := t.TempDir()
	savedPath := filepath.Join(dir, "images", "source.png")
	if err := os.MkdirAll(filepath.Dir(savedPath), 0o700); err != nil {
		t.Fatalf("mkdir images: %v", err)
	}
	writeSizedSolidTestPNG(t, savedPath, 512, 384, color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})
	previewPath := filepath.Join(dir, "previews", "source.png")
	if err := os.MkdirAll(filepath.Dir(previewPath), 0o700); err != nil {
		t.Fatalf("mkdir previews: %v", err)
	}
	writeSizedSolidTestPNG(t, previewPath, 128, 96, color.NRGBA{R: 0x44, G: 0x55, B: 0x66, A: 0xff})
	thumbPath := filepath.Join(dir, "thumbs", "source.png")
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0o700); err != nil {
		t.Fatalf("mkdir thumbs: %v", err)
	}
	writeSizedSolidTestPNG(t, thumbPath, 256, 192, color.NRGBA{R: 0x77, G: 0x88, B: 0x99, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	loaded, failed := app.prewarmHistoryThumbs([]sharedCompat.HistoryItem{{
		ID:          "item-1",
		SavedPath:   savedPath,
		PreviewPath: previewPath,
		ThumbPath:   thumbPath,
	}})
	if loaded != 1 || failed != 0 {
		t.Fatalf("loaded=%d failed=%d want 1 0", loaded, failed)
	}
	if len(app.imageCache) == 0 {
		t.Fatal("expected prewarm to populate image cache")
	}
}

func TestBuildHistoryDayGroupsKeepsPromptGroupsByDay(t *testing.T) {
	now := time.Date(2026, time.May, 31, 15, 4, 0, 0, time.Local)
	items := []sharedCompat.HistoryItem{
		{ID: "1", Prompt: "cat poster", CreatedAt: now.UnixMilli()},
		{ID: "2", Prompt: "cat poster", CreatedAt: now.Add(-2 * time.Hour).UnixMilli()},
		{ID: "3", Prompt: "dog poster", CreatedAt: now.Add(-26 * time.Hour).UnixMilli()},
	}
	groups := buildHistoryDayGroups(items)
	if len(groups) != 2 {
		t.Fatalf("len(buildHistoryDayGroups)=%d want 2", len(groups))
	}
	if groups[0].Label != "2026-05-31" || len(groups[0].Entries) != 1 || groups[0].Entries[0].Kind != "group" {
		t.Fatalf("unexpected first day group: %#v", groups[0])
	}
	if groups[1].Label != "2026-05-30" || len(groups[1].Entries) != 1 || groups[1].Entries[0].Item.ID != "3" {
		t.Fatalf("unexpected second day group: %#v", groups[1])
	}
}

func TestNextProfileNameFindsSmallestMissingNumber(t *testing.T) {
	profiles := []sharedCompat.UpstreamProfile{
		{Name: "配置1"},
		{Name: "配置3"},
	}
	if got := nextProfileName(profiles); got != "配置2" {
		t.Fatalf("nextProfileName=%q want 配置2", got)
	}
}

func TestNextProfileIDAvoidsSameMillisecondCollision(t *testing.T) {
	profiles := []sharedCompat.UpstreamProfile{
		{ID: "gio-1234"},
		{ID: "gio-1234-2"},
	}
	if got := nextProfileID(profiles, 1234); got != "gio-1234-3" {
		t.Fatalf("nextProfileID=%q want gio-1234-3", got)
	}
}

func TestWorkspaceSwitchPreservesPrompt(t *testing.T) {
	isolateGioStableDataRoot(t)
	app := New()
	app.promptInput.SetText("workspace one")
	app.createWorkspace()
	if len(app.workspaces) != 2 {
		t.Fatalf("workspaces=%d want 2", len(app.workspaces))
	}
	second := app.activeWorkspaceID
	app.promptInput.SetText("workspace two")
	first := app.workspaces[0].ID
	app.switchWorkspace(first)
	if got := strings.TrimSpace(app.promptInput.Text()); got != "workspace one" {
		t.Fatalf("after switch back prompt=%q want workspace one", got)
	}
	app.switchWorkspace(second)
	if got := strings.TrimSpace(app.promptInput.Text()); got != "workspace two" {
		t.Fatalf("after switch second prompt=%q want workspace two", got)
	}
}

func TestWorkspaceRenameUpdatesState(t *testing.T) {
	app := New()
	id := app.activeWorkspaceID
	app.startWorkspaceRename(id)
	app.workspaceNameInput.SetText("封面方案")
	app.commitWorkspaceRename()
	if app.workspaces[0].Name != "封面方案" {
		t.Fatalf("workspace name=%q want 封面方案", app.workspaces[0].Name)
	}
}

func TestWorkspaceSwitchClearsCanvasTransientState(t *testing.T) {
	app := New()
	app.result = resultState{
		Item: sharedCompat.HistoryItem{
			ID:       "hist-1",
			ImageB64: testPNGBase64(t, color.NRGBA{R: 0x55, G: 0x88, B: 0xcc, A: 0xff}),
		},
		HasItem: true,
	}
	app.canvasTool = canvasToolMask
	app.canvasMaskStrokes = []canvasMaskStroke{{
		Points:   []f32.Point{f32.Pt(0.1, 0.1), f32.Pt(0.2, 0.2)},
		SizeNorm: 0.1,
	}}
	app.canvasAnnotations = []canvasAnnotation{{
		ID:    "ann-1",
		Kind:  canvasAnnotationKindRect,
		Rect:  image.Rect(0, 0, 20, 20),
		Color: rgb(0xff4d4d),
	}}
	app.canvasSelectedAnnotationID = "ann-1"
	app.canvasViewScale = 2
	app.canvasViewOffset = image.Pt(12, 18)

	first := app.activeWorkspaceID
	app.createWorkspace()

	if len(app.canvasMaskStrokes) != 0 {
		t.Fatalf("mask strokes should clear on new workspace, got %d", len(app.canvasMaskStrokes))
	}
	if len(app.canvasAnnotations) != 0 {
		t.Fatalf("annotations should clear on new workspace, got %d", len(app.canvasAnnotations))
	}
	if app.canvasSelectedAnnotationID != "" {
		t.Fatalf("selected annotation=%q want cleared", app.canvasSelectedAnnotationID)
	}
	if app.canvasViewScale != 1 || app.canvasViewOffset != (image.Point{}) {
		t.Fatalf("canvas view=(%f,%v) want reset", app.canvasViewScale, app.canvasViewOffset)
	}

	app.switchWorkspace(first)

	if app.result.Item.ID != "hist-1" {
		t.Fatalf("result item=%q want hist-1 restored", app.result.Item.ID)
	}
	if len(app.canvasMaskStrokes) != 0 {
		t.Fatalf("mask strokes should remain cleared after workspace switch, got %d", len(app.canvasMaskStrokes))
	}
	if len(app.canvasAnnotations) != 0 {
		t.Fatalf("annotations should remain cleared after workspace switch, got %d", len(app.canvasAnnotations))
	}
	if app.canvasViewScale != 1 || app.canvasViewOffset != (image.Point{}) {
		t.Fatalf("canvas view after switch=(%f,%v) want reset", app.canvasViewScale, app.canvasViewOffset)
	}
}

func TestDisplayedWorkspaceNameUsesPromptForDefaultActiveWorkspace(t *testing.T) {
	app := New()
	app.promptInput.SetText("夜色城市概念海报")
	app.workspaces[0].Name = "图片 1"
	name := app.displayedWorkspaceName(app.workspaces[0])
	if name != "夜色城市概念海报" {
		t.Fatalf("displayedWorkspaceName=%q want 夜色城市概念海报", name)
	}
}

func TestImageForHistoryThumbPrefersThumbPath(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "full.png")
	thumbPath := filepath.Join(dir, "thumb.png")
	writeSolidTestPNG(t, fullPath, color.NRGBA{R: 0xf0, G: 0x44, B: 0x44, A: 0xff})
	writeSolidTestPNG(t, thumbPath, color.NRGBA{R: 0x44, G: 0x88, B: 0xff, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	img, err := app.imageForHistoryThumb(sharedCompat.HistoryItem{
		ID:        "hist-thumb",
		SavedPath: fullPath,
		ThumbPath: thumbPath,
	})
	if err != nil {
		t.Fatalf("imageForHistoryThumb: %v", err)
	}
	assertImagePixelColor(t, img, color.NRGBA{R: 0x44, G: 0x88, B: 0xff, A: 0xff})
}

func TestImageForHistoryItemPrefersSavedPathAndFallsBackToThumb(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "full.png")
	thumbPath := filepath.Join(dir, "thumb.png")
	writeSolidTestPNG(t, fullPath, color.NRGBA{R: 0xf0, G: 0x44, B: 0x44, A: 0xff})
	writeSolidTestPNG(t, thumbPath, color.NRGBA{R: 0x44, G: 0x88, B: 0xff, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	fullImg, err := app.imageForHistoryItem(sharedCompat.HistoryItem{
		ID:        "hist-full",
		SavedPath: fullPath,
		ThumbPath: thumbPath,
	})
	if err != nil {
		t.Fatalf("imageForHistoryItem full: %v", err)
	}
	assertImagePixelColor(t, fullImg, color.NRGBA{R: 0xf0, G: 0x44, B: 0x44, A: 0xff})

	app = &App{imageCache: map[string]cachedImage{}}
	fallbackImg, err := app.imageForHistoryItem(sharedCompat.HistoryItem{
		ID:        "hist-fallback",
		SavedPath: filepath.Join(dir, "missing.png"),
		ThumbPath: thumbPath,
	})
	if err != nil {
		t.Fatalf("imageForHistoryItem fallback: %v", err)
	}
	assertImagePixelColor(t, fallbackImg, color.NRGBA{R: 0x44, G: 0x88, B: 0xff, A: 0xff})
}

func TestImageForHistoryThumbDownscalesSavedImageWhenThumbMissing(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "full-large.png")
	writeSizedSolidTestPNG(t, fullPath, 2048, 1024, color.NRGBA{R: 0x22, G: 0x77, B: 0xcc, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	img, err := app.imageForHistoryThumb(sharedCompat.HistoryItem{
		ID:        "hist-large",
		SavedPath: fullPath,
	})
	if err != nil {
		t.Fatalf("imageForHistoryThumb large: %v", err)
	}
	if got := img.Bounds().Dx(); got > historyThumbFallbackMaxDimension {
		t.Fatalf("thumb width=%d want <= %d", got, historyThumbFallbackMaxDimension)
	}
	if got := img.Bounds().Dy(); got > historyThumbFallbackMaxDimension {
		t.Fatalf("thumb height=%d want <= %d", got, historyThumbFallbackMaxDimension)
	}

	fullImg, err := app.imageForHistoryItem(sharedCompat.HistoryItem{
		ID:        "hist-large",
		SavedPath: fullPath,
	})
	if err != nil {
		t.Fatalf("imageForHistoryItem large: %v", err)
	}
	if fullImg.Bounds().Dx() != 2048 || fullImg.Bounds().Dy() != 1024 {
		t.Fatalf("full image bounds=%v want 2048x1024", fullImg.Bounds())
	}
}

func TestImageForPathThumbDownscalesWithoutChangingFullImageCache(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidTestPNG(t, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	thumb, err := app.imageForPathThumb(fullPath, 256)
	if err != nil {
		t.Fatalf("imageForPathThumb: %v", err)
	}
	if thumb.Bounds().Dx() > 256 || thumb.Bounds().Dy() > 256 {
		t.Fatalf("thumb bounds=%v want <= 256", thumb.Bounds())
	}

	full, err := app.imageForPath(fullPath)
	if err != nil {
		t.Fatalf("imageForPath: %v", err)
	}
	if full.Bounds().Dx() != 1800 || full.Bounds().Dy() != 1200 {
		t.Fatalf("full bounds=%v want 1800x1200", full.Bounds())
	}
}

func TestImageForPathThumbReusesBaseThumbAcrossSizes(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidTestPNG(t, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	thumb96, err := app.imageForPathThumb(fullPath, 96)
	if err != nil {
		t.Fatalf("imageForPathThumb 96: %v", err)
	}
	if thumb96.Bounds().Dx() > 96 || thumb96.Bounds().Dy() > 96 {
		t.Fatalf("thumb96 bounds=%v want <= 96", thumb96.Bounds())
	}
	baseKey := pathThumbCacheKey(fullPath, pathThumbReuseBaseMinDimension)
	if _, ok := app.imageCache[baseKey]; !ok {
		t.Fatalf("expected base thumb cache %q to be populated", baseKey)
	}
	if err := os.Remove(fullPath); err != nil {
		t.Fatalf("remove source-large.png: %v", err)
	}
	thumb224, err := app.imageForPathThumb(fullPath, 224)
	if err != nil {
		t.Fatalf("imageForPathThumb 224 after removing source: %v", err)
	}
	if thumb224.Bounds().Dx() > 224 || thumb224.Bounds().Dy() > 224 {
		t.Fatalf("thumb224 bounds=%v want <= 224", thumb224.Bounds())
	}
}

func TestLoadDisplayPathThumbUsesManagedPreviewAfterSourceRemoved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidTestPNG(t, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	got, err := app.loadDisplayPathThumb(fullPath, 48)
	if err != nil {
		t.Fatalf("loadDisplayPathThumb first: %v", err)
	}
	if got.Image == nil {
		t.Fatalf("expected first managed preview image")
	}
	app.imageCache = map[string]cachedImage{}
	if err := os.Remove(fullPath); err != nil {
		t.Fatalf("remove source-large.png: %v", err)
	}
	got, err = app.loadDisplayPathThumb(fullPath, 48)
	if err != nil {
		t.Fatalf("loadDisplayPathThumb second: %v", err)
	}
	if got.Image == nil {
		t.Fatalf("expected managed preview image after source removal")
	}
}

func TestLoadHistoryImageScaledUncachedDoesNotPopulatePathThumbCache(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "uncached-large.png")
	writeSizedSolidTestPNG(t, fullPath, 1600, 900, color.NRGBA{R: 0x77, G: 0xbb, B: 0xee, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	item := sharedCompat.HistoryItem{ID: "uncached", SavedPath: fullPath}
	img, err := app.loadHistoryImageScaledUncached(item, 256)
	if err != nil {
		t.Fatalf("loadHistoryImageScaledUncached: %v", err)
	}
	if img.Bounds().Dx() > 256 || img.Bounds().Dy() > 256 {
		t.Fatalf("uncached thumb bounds=%v want <= 256", img.Bounds())
	}
	if _, ok := app.imageCache["path-thumb:256:"+fullPath]; ok {
		t.Fatalf("unexpected path-thumb cache population for uncached load")
	}
}

func TestLoadHistoryPreviewKeepsSavedPathAndLoadsDisplayImage(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "full.png")
	thumbPath := filepath.Join(dir, "thumb.png")
	writeSolidTestPNG(t, fullPath, color.NRGBA{R: 0xf0, G: 0x44, B: 0x44, A: 0xff})
	writeSolidTestPNG(t, thumbPath, color.NRGBA{R: 0x44, G: 0x88, B: 0xff, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	item := sharedCompat.HistoryItem{
		ID:          "hist-preview",
		Prompt:      "赛博海报",
		PreviewPath: thumbPath,
		SavedPath:   fullPath,
		ThumbPath:   thumbPath,
	}
	if err := app.loadHistoryPreview(item, true); err != nil {
		t.Fatalf("loadHistoryPreview: %v", err)
	}
	snap := app.readSnapshot()
	if snap.Result.SavedPath != fullPath {
		t.Fatalf("result.SavedPath=%q want %q", snap.Result.SavedPath, fullPath)
	}
	if snap.SelectedHistoryID != item.ID {
		t.Fatalf("selectedHistoryID=%q want %q", snap.SelectedHistoryID, item.ID)
	}
	if snap.Result.Image == nil {
		t.Fatal("expected immediate preview image")
	}
	assertImagePixelColor(t, snap.Result.Image, color.NRGBA{R: 0x44, G: 0x88, B: 0xff, A: 0xff})
	img := waitForImage(t, func() image.Image {
		current := app.readSnapshot().Result.Image
		if current == nil {
			return nil
		}
		pixel := color.NRGBAModel.Convert(current.At(0, 0)).(color.NRGBA)
		if pixel == (color.NRGBA{R: 0x44, G: 0x88, B: 0xff, A: 0xff}) {
			return nil
		}
		return current
	})
	assertImagePixelColor(t, img, color.NRGBA{R: 0xf0, G: 0x44, B: 0x44, A: 0xff})
}

func TestReadSnapshotCachesUntilInvalidated(t *testing.T) {
	app := &App{}
	app.logs = []string{"first"}
	app.logsRev = 1
	app.logsSnapshotRev = -1
	app.setHistoryLocked([]sharedCompat.HistoryItem{{ID: "item-1"}})

	snap1 := app.readSnapshot()
	snap2 := app.readSnapshot()
	if len(snap1.Logs) != 1 || snap1.Logs[0] != "first" {
		t.Fatalf("snap1 logs=%v want [first]", snap1.Logs)
	}
	if len(snap2.History) != 1 || snap2.History[0].ID != "item-1" {
		t.Fatalf("snap2 history=%v want item-1", snap2.History)
	}

	app.mu.Lock()
	app.logs = []string{"second"}
	app.logsRev = 2
	app.setHistoryLocked([]sharedCompat.HistoryItem{{ID: "item-2"}})
	app.mu.Unlock()

	stale := app.readSnapshot()
	if len(stale.Logs) != 1 || stale.Logs[0] != "first" {
		t.Fatalf("stale logs=%v want cached [first]", stale.Logs)
	}

	app.invalidateNow()
	fresh := app.readSnapshot()
	if len(fresh.Logs) != 1 || fresh.Logs[0] != "second" {
		t.Fatalf("fresh logs=%v want [second]", fresh.Logs)
	}
	if len(fresh.History) != 1 || fresh.History[0].ID != "item-2" {
		t.Fatalf("fresh history=%v want item-2", fresh.History)
	}
}

func TestHistoryPanelDataRefreshesAfterHistoryRevisionChanges(t *testing.T) {
	app := &App{historyRev: 1}
	first := []sharedCompat.HistoryItem{{ID: "item-1", Prompt: "first"}}
	second := []sharedCompat.HistoryItem{{ID: "item-2", Prompt: "second"}}

	data1 := app.historyPanelData(first)
	if len(data1.entries) != 1 || data1.entries[0].Item.ID != "item-1" {
		t.Fatalf("data1 entries=%v want item-1", data1.entries)
	}

	app.mu.Lock()
	app.setHistoryLocked(second)
	app.mu.Unlock()

	data2 := app.historyPanelData(second)
	if len(data2.entries) != 1 || data2.entries[0].Item.ID != "item-2" {
		t.Fatalf("data2 entries=%v want item-2", data2.entries)
	}
}

func TestPromptSuggestionsCacheRefreshesAfterPromptHistoryChanges(t *testing.T) {
	app := &App{promptHistoryRev: 1}
	app.setHistoryLocked([]sharedCompat.HistoryItem{{ID: "hist-1", Prompt: "history prompt"}})
	app.promptHistory = []string{"first prompt"}

	got1 := app.promptSuggestions(app.history)
	if len(got1) == 0 || got1[0] != "first prompt" {
		t.Fatalf("got1=%v want first prompt", got1)
	}

	app.mu.Lock()
	app.promptHistory = []string{"second prompt"}
	app.promptHistoryRev++
	app.mu.Unlock()

	got2 := app.promptSuggestions(app.history)
	if len(got2) == 0 || got2[0] != "second prompt" {
		t.Fatalf("got2=%v want second prompt", got2)
	}
}

func TestHistoryItemDisplayCacheRefreshesAfterHistoryRevisionChanges(t *testing.T) {
	app := &App{}
	first := sharedCompat.HistoryItem{
		ID:        "hist-1",
		Prompt:    "first prompt",
		Size:      "1024x1024",
		Quality:   "high",
		CreatedAt: time.Unix(1710, 0).UnixMilli(),
	}
	app.setHistoryLocked([]sharedCompat.HistoryItem{first})

	got1 := app.historyItemDisplay(first)
	if got1.ShortPrompt != shortPrompt(first.Prompt) {
		t.Fatalf("got1.ShortPrompt=%q want %q", got1.ShortPrompt, shortPrompt(first.Prompt))
	}

	second := first
	second.Prompt = "second prompt"
	app.setHistoryLocked([]sharedCompat.HistoryItem{second})

	got2 := app.historyItemDisplay(second)
	if got2.ShortPrompt != shortPrompt(second.Prompt) {
		t.Fatalf("got2.ShortPrompt=%q want %q", got2.ShortPrompt, shortPrompt(second.Prompt))
	}
	if got2.ShortPrompt == got1.ShortPrompt {
		t.Fatalf("history item display cache did not refresh across history revisions")
	}
}

func TestSourcePathsCacheRefreshesAfterTextChanges(t *testing.T) {
	app := &App{}
	app.sourcePathsInput.SetText("a.png\nb.png")
	first := app.sourcePaths()
	if len(first) != 2 || first[0] != "a.png" || first[1] != "b.png" {
		t.Fatalf("first=%v want [a.png b.png]", first)
	}

	app.sourcePathsInput.SetText("c.png")
	second := app.sourcePaths()
	if len(second) != 1 || second[0] != "c.png" {
		t.Fatalf("second=%v want [c.png]", second)
	}
	if len(app.sourcePathParseCache) < 2 {
		t.Fatalf("sourcePathParseCache=%v want cached entries for both texts", app.sourcePathParseCache)
	}
}

func TestMoveSourcePathReordersPaths(t *testing.T) {
	app := &App{}
	app.sourcePathsInput.SetText("a.png\nb.png\nc.png")

	app.moveSourcePath("b.png", -1)
	first := app.sourcePaths()
	if len(first) != 3 || first[0] != "b.png" || first[1] != "a.png" || first[2] != "c.png" {
		t.Fatalf("after move left=%v want [b.png a.png c.png]", first)
	}

	app.moveSourcePath("b.png", 1)
	second := app.sourcePaths()
	if len(second) != 3 || second[0] != "a.png" || second[1] != "b.png" || second[2] != "c.png" {
		t.Fatalf("after move right=%v want [a.png b.png c.png]", second)
	}

	app.moveSourcePath("a.png", -1)
	third := app.sourcePaths()
	if len(third) != 3 || third[0] != "a.png" || third[1] != "b.png" || third[2] != "c.png" {
		t.Fatalf("out-of-range move should no-op, got %v", third)
	}
}

func TestApplyCanvasZoomClampsAndMovesInExpectedDirection(t *testing.T) {
	if got := applyCanvasZoom(1, -120); got <= 1 {
		t.Fatalf("zoom in should increase scale, got %f", got)
	}
	if got := applyCanvasZoom(1, 120); got >= 1 {
		t.Fatalf("zoom out should decrease scale, got %f", got)
	}
	if got := applyCanvasZoom(0.01, 120); got < 0.05 {
		t.Fatalf("zoom should clamp to min, got %f", got)
	}
	if got := applyCanvasZoom(100, -120); got > 8 {
		t.Fatalf("zoom should clamp to max, got %f", got)
	}
}

func TestComposeSummaryRefreshesAfterRelevantChanges(t *testing.T) {
	app := &App{
		size:       "1024x1024",
		quality:    "high",
		batchCount: 1,
		mode:       "generate",
	}
	app.imageModelInput.SetText("gpt-image-1")
	first := app.composeSummary(snapshot{})
	if !strings.Contains(first, "文生图") {
		t.Fatalf("first=%q want generate summary", first)
	}

	app.mode = "edit"
	app.sourcePathsInput.SetText("a.png\nb.png")
	second := app.composeSummary(snapshot{})
	if !strings.Contains(second, "2 张源图") {
		t.Fatalf("second=%q want source-count summary", second)
	}
	if first == second {
		t.Fatalf("compose summary did not refresh after relevant changes")
	}
}

func TestComposeSummaryTreatsPreviewOnlyCurrentResultAsImplicitEditSource(t *testing.T) {
	app := &App{
		mode:              "edit",
		size:              "1024x1024",
		quality:           "high",
		kernelRuntimeMode: "remote",
	}
	app.result = resultState{
		Item: sharedCompat.HistoryItem{
			ID:       "preview-1",
			ImageB64: base64.StdEncoding.EncodeToString([]byte("image-bytes")),
		},
		HasItem: true,
	}

	summary := app.composeSummary(snapshot{Result: app.result})
	if !strings.Contains(summary, "画板图作源图") {
		t.Fatalf("summary=%q want implicit-current-image source label", summary)
	}
}

func TestCanTransformCurrentResultRequiresSavedPath(t *testing.T) {
	if canTransformCurrentResult(snapshot{}) {
		t.Fatal("empty snapshot should not allow transforms")
	}
	if canTransformCurrentResult(snapshot{
		Result: resultState{
			HasItem: true,
			Item: sharedCompat.HistoryItem{
				ImageB64: base64.StdEncoding.EncodeToString([]byte("image-bytes")),
			},
		},
	}) {
		t.Fatal("preview-only result without savedPath should not allow transforms")
	}
	if !canTransformCurrentResult(snapshot{
		Result: resultState{
			HasItem:   true,
			SavedPath: "/tmp/image.png",
		},
	}) {
		t.Fatal("saved result should allow transforms")
	}
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString([]byte("virtual-image")), "virtual.png", "png")
	if !canTransformCurrentResult(snapshot{
		Result: resultState{
			HasItem:   true,
			SavedPath: virtualPath,
		},
	}) {
		t.Fatal("virtual saved result should allow transforms")
	}
}

func TestReplaceCurrentResultWithPathSupportsVirtualImagePath(t *testing.T) {
	app := &App{
		result: resultState{
			Item: sharedCompat.HistoryItem{
				ID:       "preview-1",
				ImageB64: base64.StdEncoding.EncodeToString([]byte("old-image")),
			},
		},
	}
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString([]byte("new-image")), "rotated.png", "png")

	if err := app.replaceCurrentResultWithPath(virtualPath, "rotate"); err != nil {
		t.Fatalf("replaceCurrentResultWithPath: %v", err)
	}
	snap := app.readSnapshot()
	if snap.Result.SavedPath != virtualPath {
		t.Fatalf("savedPath=%q want %q", snap.Result.SavedPath, virtualPath)
	}
	if snap.Result.Item.ImageB64 != base64.StdEncoding.EncodeToString([]byte("new-image")) {
		t.Fatalf("imageB64=%q want updated virtual image data", snap.Result.Item.ImageB64)
	}
}

func TestReplaceCurrentResultWithPathPromotesTransformResultToFirstEditSource(t *testing.T) {
	app := &App{
		mode:      string(client.ModeGenerate),
		batchMode: true,
		result: resultState{
			Item: sharedCompat.HistoryItem{
				ID:       "preview-1",
				ImageB64: base64.StdEncoding.EncodeToString([]byte("old-image")),
			},
		},
	}
	app.sourcePathsInput.SetText("/tmp/a.png\n/tmp/b.png")
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString([]byte("new-image")), "rotated.png", "png")

	if err := app.replaceCurrentResultWithPath(virtualPath, "rotate"); err != nil {
		t.Fatalf("replaceCurrentResultWithPath: %v", err)
	}
	if app.mode != string(client.ModeEdit) {
		t.Fatalf("mode=%q want edit", app.mode)
	}
	if app.batchMode {
		t.Fatal("batchMode should reset to false after promoting transformed result")
	}
	paths := app.sourcePaths()
	if len(paths) != 2 || paths[0] != virtualPath || paths[1] != "/tmp/b.png" {
		t.Fatalf("source paths=%v want [%s /tmp/b.png]", paths, virtualPath)
	}
}

func TestAdvancedSummaryRefreshesAfterRelevantChanges(t *testing.T) {
	app := &App{
		format:               "png",
		background:           "transparent",
		moderation:           "auto",
		protectStreamPreview: true,
	}
	app.negativePromptInput.SetText("no watermark")
	app.partialImagesInput.SetText("0")
	first := app.advancedSummary()
	if !strings.Contains(first, "仅最终图") {
		t.Fatalf("first=%q want partial-preview summary", first)
	}
	if !strings.Contains(first, "背景 transparent") {
		t.Fatalf("first=%q want raw background value summary", first)
	}
	if strings.Contains(first, "预览保护") {
		t.Fatalf("first=%q should not include protect-stream summary", first)
	}

	app.seedInput.SetText("123")
	second := app.advancedSummary()
	if !strings.Contains(second, "Seed 123") {
		t.Fatalf("second=%q want seed summary", second)
	}
	if first == second {
		t.Fatalf("advanced summary did not refresh after relevant changes")
	}
}

func TestPromptLabelsCachedRefreshesAfterSuggestionChanges(t *testing.T) {
	app := &App{}
	first := app.promptLabelsCached([]string{"first prompt"})
	if len(first) != 1 || first[0].Title != "历史 1" || first[0].Detail != "first prompt" {
		t.Fatalf("first=%v want single first prompt item", first)
	}

	second := app.promptLabelsCached([]string{"second prompt"})
	if len(second) != 1 || second[0].Title != "历史 1" || second[0].Detail != "second prompt" {
		t.Fatalf("second=%v want single second prompt item", second)
	}
	if first[0].Detail == second[0].Detail {
		t.Fatalf("prompt label cache did not refresh after suggestion changes")
	}
}

func TestPresetLabelsCachedRefreshesAfterPresetChanges(t *testing.T) {
	app := &App{}
	first := app.presetLabelsCached([]sharedCompat.Preset{{ID: "a", Name: "A", Size: "1024x1024", Quality: "high", OutputFormat: "png", BatchCount: 1}})
	if len(first) != 1 || first[0].Title != "A" {
		t.Fatalf("first=%v want preset A", first)
	}
	if first[0].Kind != "preset" {
		t.Fatalf("first kind=%q want preset", first[0].Kind)
	}

	second := app.presetLabelsCached([]sharedCompat.Preset{{ID: "b", Name: "B", Size: "1536x1024", Quality: "medium", OutputFormat: "webp", BatchCount: 2}})
	if len(second) != 1 || second[0].Title != "B" {
		t.Fatalf("second=%v want preset B", second)
	}
	if first[0].Title == second[0].Title {
		t.Fatalf("preset label cache did not refresh after preset changes")
	}
}

func TestPromptHelperApplyTextPrefersDetail(t *testing.T) {
	item := promptHelperItem{Title: "short", Detail: "full prompt"}
	if got := promptHelperApplyText(item); got != "full prompt" {
		t.Fatalf("promptHelperApplyText=%q want full prompt", got)
	}
}

func TestPromptHelperApplyTextFallsBackToTitle(t *testing.T) {
	item := promptHelperItem{Title: "title only", Detail: "   "}
	if got := promptHelperApplyText(item); got != "title only" {
		t.Fatalf("promptHelperApplyText=%q want title only", got)
	}
}

func TestPromptLabelsNumberHistoryInDisplayOrder(t *testing.T) {
	items := promptLabels([]string{"newest prompt", "older prompt"})
	if len(items) != 2 {
		t.Fatalf("len(promptLabels)=%d want 2", len(items))
	}
	if items[0].Title != "历史 1" || items[0].Detail != "newest prompt" {
		t.Fatalf("first history label=%#v", items[0])
	}
	if items[1].Title != "历史 2" || items[1].Detail != "older prompt" {
		t.Fatalf("second history label=%#v", items[1])
	}
}

func TestPromptTemplateItemsIncludesSharedAndBuiltInTemplates(t *testing.T) {
	app := &App{
		promptTemplates: []sharedCompat.PromptTemplate{{
			ID:    "custom-1",
			Label: "我的模板",
			Text:  "custom prompt",
		}},
	}
	items := app.promptTemplateItems()
	if len(items) != len(builtInPromptTemplates)+1 {
		t.Fatalf("len(promptTemplateItems)=%d want %d", len(items), len(builtInPromptTemplates)+1)
	}
	if items[0].ID != "custom-1" || items[0].Kind != "template" || items[0].Detail != "custom prompt" {
		t.Fatalf("unexpected custom template item: %#v", items[0])
	}
}

func TestApplyPresetUsesExtendedSharedFields(t *testing.T) {
	app := &App{}
	compression := 87
	app.outputCompressionInput.SetText("100")
	app.batchCount = 1
	app.kernelRuntimeMode = "auto"
	app.editAutoAspectResolution = ""

	app.applyPreset(sharedCompat.Preset{
		ID:                "preset-1",
		Name:              "配置1",
		Size:              "1536x1024",
		Quality:           "high",
		OutputFormat:      "webp",
		NegativePrompt:    "no watermark",
		Background:        "transparent",
		OutputCompression: &compression,
		InputFidelity:     "high",
		ImageStyle:        "vivid",
		Moderation:        "auto",
		StyleTag:          "anime",
		EditAutoAspectRes: "1k",
		KernelRuntimeMode: "remote",
		BatchCount:        4,
	})

	if app.size != "1536x1024" || app.quality != "high" || app.format != "webp" {
		t.Fatalf("basic preset fields not applied: size=%q quality=%q format=%q", app.size, app.quality, app.format)
	}
	if app.negativePromptInput.Text() != "no watermark" {
		t.Fatalf("negativePrompt=%q want no watermark", app.negativePromptInput.Text())
	}
	if app.background != "transparent" || app.outputCompressionInput.Text() != "87" || app.inputFidelity != "high" || app.imageStyle != "vivid" || app.moderation != "auto" || app.styleTag != "anime" || app.editAutoAspectResolution != "1k" || app.kernelRuntimeMode != "remote" || app.batchCount != 4 || app.selectedPresetID != "preset-1" {
		t.Fatalf("extended preset fields not applied: background=%q compression=%q fidelity=%q imageStyle=%q moderation=%q style=%q autoAspect=%q runtime=%q batch=%d selectedPreset=%q", app.background, app.outputCompressionInput.Text(), app.inputFidelity, app.imageStyle, app.moderation, app.styleTag, app.editAutoAspectResolution, app.kernelRuntimeMode, app.batchCount, app.selectedPresetID)
	}
}

func TestWorkspaceSnapshotPersistsSelectedPresetID(t *testing.T) {
	app := &App{
		activeWorkspaceID: "ws-1",
		workspaces:        []workspaceState{{ID: "ws-1", Name: "图片 1"}},
		selectedPresetID:  "preset-2",
	}

	app.saveActiveWorkspaceSnapshot()

	if len(app.workspaces) != 1 || app.workspaces[0].SelectedPresetID != "preset-2" {
		t.Fatalf("workspace snapshot selectedPresetID=%q want preset-2", app.workspaces[0].SelectedPresetID)
	}

	app.selectedPresetID = ""
	app.applyWorkspace(app.workspaces[0])
	if app.selectedPresetID != "preset-2" {
		t.Fatalf("selectedPresetID=%q want preset-2 after applyWorkspace", app.selectedPresetID)
	}
}

func TestCurrentPresetSnapshotIncludesEditAutoAspectResolution(t *testing.T) {
	app := &App{
		size:                     "1536x1024",
		quality:                  "high",
		format:                   "png",
		background:               "transparent",
		inputFidelity:            "high",
		imageStyle:               "vivid",
		moderation:               "auto",
		styleTag:                 "anime",
		editAutoAspectResolution: "1k",
		kernelRuntimeMode:        "remote",
		batchCount:               4,
	}
	app.presetNameInput.SetText("配置1")
	app.outputCompressionInput.SetText("87")

	preset := app.currentPresetSnapshot()
	if preset.EditAutoAspectRes != "1k" {
		t.Fatalf("preset.EditAutoAspectRes=%q want 1k", preset.EditAutoAspectRes)
	}
}

func TestCurrentPresetSummaryStateDetectsMatchedPreset(t *testing.T) {
	app := &App{
		size:                     "1536x1024",
		quality:                  "high",
		format:                   "png",
		background:               "transparent",
		inputFidelity:            "high",
		imageStyle:               "vivid",
		moderation:               "auto",
		styleTag:                 "anime",
		editAutoAspectResolution: "1k",
		kernelRuntimeMode:        "remote",
		batchCount:               4,
	}
	compression := 87
	app.outputCompressionInput.SetText("87")
	app.presets = []sharedCompat.Preset{{
		ID:                "preset-1",
		Name:              "配置1",
		Size:              "1536x1024",
		Quality:           "high",
		OutputFormat:      "png",
		NegativePrompt:    "",
		Background:        "transparent",
		OutputCompression: &compression,
		InputFidelity:     "high",
		ImageStyle:        "vivid",
		Moderation:        "auto",
		StyleTag:          "anime",
		EditAutoAspectRes: "1k",
		KernelRuntimeMode: "remote",
		BatchCount:        4,
	}}

	summary := app.currentPresetSummaryState()
	if summary.MatchedPreset == nil || summary.MatchedPreset.ID != "preset-1" {
		t.Fatalf("matched preset=%#v want preset-1", summary.MatchedPreset)
	}
	if summary.Title == "" || summary.Detail == "" {
		t.Fatalf("summary should not be empty: %#v", summary)
	}
}

func TestLoadPresetDraftLockedLoadsEditableFields(t *testing.T) {
	app := &App{}
	app.presets = []sharedCompat.Preset{{
		ID:                "preset-1",
		Name:              "配置1",
		Size:              "1536x1024",
		Quality:           "high",
		OutputFormat:      "webp",
		StyleTag:          "anime",
		EditAutoAspectRes: "1k",
		BatchCount:        4,
	}}

	if !app.loadPresetDraftLocked("preset-1") {
		t.Fatal("expected preset draft to load")
	}
	if app.selectedPresetID != "preset-1" || app.presetNameInput.Text() != "配置1" {
		t.Fatalf("selected/id draft name mismatch: selected=%q name=%q", app.selectedPresetID, app.presetNameInput.Text())
	}
	if app.presetSizeInput.Text() != "1536x1024" || app.presetQualityInput.Text() != "high" || app.presetOutputFormatInput.Text() != "webp" || app.presetStyleTagInput.Text() != "anime" || app.editAutoAspectResolution != "1k" || app.presetBatchCountInput.Text() != "4" {
		t.Fatalf("editable draft fields not loaded: size=%q quality=%q format=%q style=%q autoAspect=%q batch=%q", app.presetSizeInput.Text(), app.presetQualityInput.Text(), app.presetOutputFormatInput.Text(), app.presetStyleTagInput.Text(), app.editAutoAspectResolution, app.presetBatchCountInput.Text())
	}
}

func TestCurrentPresetDraftValuesPreferDraftInputs(t *testing.T) {
	app := &App{size: "1024x1024", quality: "auto", format: "png", batchCount: 1, styleTag: ""}
	app.presetSizeInput.SetText("1536x1024")
	app.presetQualityInput.SetText("high")
	app.presetOutputFormatInput.SetText("webp")
	app.presetBatchCountInput.SetText("4")
	app.presetStyleTagInput.SetText("anime")

	size, quality, outputFormat, batchCount, styleTag := app.currentPresetDraftValues()
	if size != "1536x1024" || quality != "high" || outputFormat != "webp" || batchCount != 4 || styleTag != "anime" {
		t.Fatalf("draft values mismatch: size=%q quality=%q format=%q batch=%d style=%q", size, quality, outputFormat, batchCount, styleTag)
	}
}

func TestBuildUpdatedPresetFromDraftKeepsUneditedFields(t *testing.T) {
	compression := 87
	current := sharedCompat.Preset{
		ID:                "preset-1",
		Name:              "配置1",
		Size:              "1024x1024",
		Quality:           "auto",
		OutputFormat:      "png",
		NegativePrompt:    "keep me",
		Background:        "transparent",
		OutputCompression: &compression,
		InputFidelity:     "high",
		ImageStyle:        "vivid",
		Moderation:        "auto",
		StyleTag:          "",
		EditAutoAspectRes: "1k",
		KernelRuntimeMode: "remote",
		BatchCount:        1,
	}

	updated := buildUpdatedPresetFromDraft(current, "新名字", "1536x1024", "high", "webp", 4, "anime")

	if updated.Name != "新名字" || updated.Size != "1536x1024" || updated.Quality != "high" || updated.OutputFormat != "webp" || updated.BatchCount != 4 || updated.StyleTag != "anime" {
		t.Fatalf("edited fields mismatch: %#v", updated)
	}
	if updated.NegativePrompt != "keep me" || updated.Background != "transparent" || updated.InputFidelity != "high" || updated.ImageStyle != "vivid" || updated.Moderation != "auto" || updated.EditAutoAspectRes != "1k" || updated.KernelRuntimeMode != "remote" {
		t.Fatalf("unedited fields should stay unchanged: %#v", updated)
	}
	if updated.OutputCompression == nil || *updated.OutputCompression != 87 {
		t.Fatalf("output compression should be preserved: %#v", updated.OutputCompression)
	}
}

func TestOpenPromptHelperPopoverDefaultsToTemplates(t *testing.T) {
	app := &App{}
	app.promptHelperTab = "history"
	app.promptHelperOpen = false
	app.openPromptHelperPopover("", nil, image.Point{})

	if !app.promptHelperOpen {
		t.Fatal("prompt helper should open from preset picker button")
	}
	if app.promptHelperTab != "templates" {
		t.Fatalf("promptHelperTab=%q want templates", app.promptHelperTab)
	}
	if app.promptHelperAnchorRect == (image.Rectangle{}) {
		t.Fatal("prompt helper should fall back to a stable anchor rect")
	}
}

func TestCanvasShortcutModalsOpenIncludesPromptHelper(t *testing.T) {
	app := &App{promptHelperOpen: true}
	if !app.canvasShortcutModalsOpen(snapshot{}) {
		t.Fatal("prompt helper should block canvas shortcuts while open")
	}
}

func TestCanvasShortcutModalsOpenIncludesPresetPicker(t *testing.T) {
	app := &App{presetPickerOpen: true}
	if !app.canvasShortcutModalsOpen(snapshot{}) {
		t.Fatal("preset picker should block canvas shortcuts while open")
	}
}

func TestCanvasShortcutModalsOpenIncludesLoopModal(t *testing.T) {
	app := &App{loopModalOpen: true}
	if !app.canvasShortcutModalsOpen(snapshot{}) {
		t.Fatal("loop modal should block canvas shortcuts while open")
	}
}

func TestSetLoopEnabledOpeningAlsoOpensLoopModal(t *testing.T) {
	app := &App{}
	app.setLoopEnabled(true)
	if !app.loopEnabled {
		t.Fatal("loop should be enabled")
	}
	if !app.loopModalOpen {
		t.Fatal("enabling loop from launcher should also open loop modal")
	}
}

func TestAdvancedPanelStateHelpers(t *testing.T) {
	app := &App{}
	if app.advancedOpen {
		t.Fatal("advanced panel should start closed")
	}
	app.openAdvancedPanel()
	if !app.advancedOpen {
		t.Fatal("advanced panel should open")
	}
	app.closeAdvancedPanel()
	if app.advancedOpen {
		t.Fatal("advanced panel should close")
	}

	app.toggleAdvancedGroup("core")
	if !app.advancedCoreGroupOpen {
		t.Fatal("core group should toggle open")
	}
	app.toggleAdvancedGroup("output")
	if !app.advancedOutputGroupOpen {
		t.Fatal("output group should toggle open")
	}
}

func TestClampAdvancedPanelPosKeepsFloatingPanelInViewport(t *testing.T) {
	app := &App{}
	got := app.clampAdvancedPanelPos(image.Pt(9999, -40), image.Pt(1280, 720), 360, 420)
	if got.X < advancedPanelMargin || got.Y < advancedPanelMargin {
		t.Fatalf("clamped pos=%v should stay inside panel margin", got)
	}
	if got.X > 1280-360-advancedPanelMargin || got.Y > 720-420-advancedPanelMargin {
		t.Fatalf("clamped pos=%v should stay within viewport", got)
	}
}

func TestNewRestoresAdvancedFloatingPanelPrefs(t *testing.T) {
	root := t.TempDir()
	origStable := giodCompat.StableDataRootForTest()
	giodCompat.SetStableDataRootForTest(func() (string, error) { return root, nil })
	defer giodCompat.SetStableDataRootForTest(origStable)

	x := 812
	y := 136
	state := sharedCompat.State{
		Settings: sharedCompat.Settings{
			AdvancedFloatingPanel: &sharedCompat.AdvancedFloatingPanelPrefs{
				X: &x,
				Y: &y,
				Groups: map[string]bool{
					"core":     false,
					"output":   true,
					"strategy": true,
					"stream":   false,
				},
			},
		},
	}
	if err := giodCompat.SaveState(state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	app := New()
	if app.advancedPanelPos != image.Pt(812, 136) {
		t.Fatalf("advancedPanelPos=%v want (812,136)", app.advancedPanelPos)
	}
	if app.advancedCoreGroupOpen || !app.advancedOutputGroupOpen || !app.advancedStrategyGroupOpen || app.advancedStreamGroupOpen {
		t.Fatalf("advanced group prefs not restored: core=%t output=%t strategy=%t stream=%t", app.advancedCoreGroupOpen, app.advancedOutputGroupOpen, app.advancedStrategyGroupOpen, app.advancedStreamGroupOpen)
	}
}

func TestAppendPromptTemplateTextMatchesWebviewSemantics(t *testing.T) {
	if got := appendPromptTemplateText("", "anime style"); got != "anime style" {
		t.Fatalf("appendPromptTemplateText(empty)=%q want anime style", got)
	}
	if got := appendPromptTemplateText("a cat in rain", "anime style"); got != "a cat in rain, anime style" {
		t.Fatalf("appendPromptTemplateText(non-empty)=%q want comma join", got)
	}
}

func TestApplyPromptSuggestionAppendsWithCommaSeparator(t *testing.T) {
	app := &App{}
	app.promptInput.SetText("a cat in rain")

	app.applyPromptSuggestion("anime style")

	if got := app.promptInput.Text(); got != "a cat in rain, anime style" {
		t.Fatalf("prompt=%q want comma-joined prompt", got)
	}
	if app.promptHelperOpen {
		t.Fatal("prompt helper should close after applying suggestion")
	}
}

func TestApplyPartialPreviewReplacesCanvasImageWhileRunning(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	fill := color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0xff}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	tmp := filepath.Join(t.TempDir(), "partial.png")
	writeImagePNG(t, tmp, img)
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("read partial fixture: %v", err)
	}

	app := &App{
		running: true,
		lastRunConfig: kernel.Config{
			Prompt:       "preview prompt",
			Mode:         client.ModeEdit,
			Size:         "1024x1536",
			Quality:      "high",
			OutputFormat: "png",
			ParentID:     "/tmp/source-parent.png",
		},
	}
	app.applyPartialPreview(0, 0, client.PartialImage{
		ImageB64:          base64.StdEncoding.EncodeToString(data),
		RevisedPrompt:     "partial rev",
		PartialImageIndex: 0,
	})

	if app.result.Image == nil {
		t.Fatal("result image not populated")
	}
	if app.result.SourceEvent != "partial" {
		t.Fatalf("sourceEvent=%q want partial", app.result.SourceEvent)
	}
	if app.result.RevisedPrompt != "partial rev" {
		t.Fatalf("revisedPrompt=%q want partial rev", app.result.RevisedPrompt)
	}
	if !app.result.HasItem {
		t.Fatal("partial preview should expose transient item metadata")
	}
	if app.result.Item.Prompt != "preview prompt" || app.result.Item.Mode != "edit" || app.result.Item.ParentID != "/tmp/source-parent.png" {
		t.Fatalf("partial preview item metadata = %#v", app.result.Item)
	}
	if app.result.Item.ImageB64 == "" || !app.result.Item.PreviewOnly {
		t.Fatalf("partial preview item should keep inline image and previewOnly flag: %#v", app.result.Item)
	}
	assertImagePixelColor(t, app.result.Image, fill)
}

func TestApplyPartialPreviewStoresBatchSlotWithoutReplacingCanvas(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	fill := color.NRGBA{R: 0xaa, G: 0xbb, B: 0xcc, A: 0xff}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	tmp := filepath.Join(t.TempDir(), "batch-partial.png")
	writeImagePNG(t, tmp, img)
	data, err := os.ReadFile(tmp)
	if err != nil {
		t.Fatalf("read batch partial fixture: %v", err)
	}

	app := &App{
		running:           true,
		lastRunBatchCount: 4,
		result: resultState{
			SourceEvent: "history",
		},
		lastRunConfig: kernel.Config{
			Prompt:       "preview prompt",
			Mode:         client.ModeGenerate,
			Size:         "1024x1024",
			Quality:      "high",
			OutputFormat: "png",
		},
	}
	app.applyPartialPreview(2, 1, client.PartialImage{
		ImageB64:          base64.StdEncoding.EncodeToString(data),
		RevisedPrompt:     "batch partial",
		PartialImageIndex: 0,
	})

	if app.result.Image != nil {
		t.Fatal("batch partial preview should not replace current canvas image")
	}
	if app.result.SourceEvent != "history" {
		t.Fatalf("sourceEvent=%q want history preserved", app.result.SourceEvent)
	}
	if !app.resultGridOpen {
		t.Fatal("batch partial preview should keep result grid open")
	}
	if len(app.batchPreviewItems) != 1 {
		t.Fatalf("batchPreviewItems=%d want 1", len(app.batchPreviewItems))
	}
	item, ok := app.batchPreviewItems[1]
	if !ok {
		t.Fatalf("missing batch preview slot 1: %#v", app.batchPreviewItems)
	}
	if item.BatchIndex != 2 || item.PreviewSlotIndex != 1 || item.RevisedPrompt != "batch partial" || !item.PreviewOnly {
		t.Fatalf("unexpected batch preview item: %#v", item)
	}
	if item.ImageB64 == "" {
		t.Fatalf("batch preview item should retain inline image data: %#v", item)
	}
}

func TestPromptInputMetricsRefreshesAfterTextChanges(t *testing.T) {
	app := &App{}
	app.promptInput.SetText("  hello world  ")
	trimmed1, len1 := app.promptInputMetrics()
	if trimmed1 != "hello world" || len1 != len([]rune("hello world")) {
		t.Fatalf("first metrics=(%q,%d) want (hello world,%d)", trimmed1, len1, len([]rune("hello world")))
	}

	app.promptInput.SetText("提示词")
	trimmed2, len2 := app.promptInputMetrics()
	if trimmed2 != "提示词" || len2 != len([]rune("提示词")) {
		t.Fatalf("second metrics=(%q,%d) want (提示词,%d)", trimmed2, len2, len([]rune("提示词")))
	}
	if trimmed1 == trimmed2 {
		t.Fatalf("prompt metrics did not refresh after text changes")
	}
}

func TestOpenRawResponseModalReadsVirtualText(t *testing.T) {
	app := &App{}
	path := registerVirtualText("hello raw response", "raw.txt")

	app.openRawResponseModal(path)

	snap := app.readSnapshot()
	if snap.RawResponseModalPath != path {
		t.Fatalf("raw response path=%q want %q", snap.RawResponseModalPath, path)
	}
	if snap.RawResponseModalError != "" {
		t.Fatalf("raw response error=%q want empty", snap.RawResponseModalError)
	}
	if snap.RawResponseModalText != "hello raw response" {
		t.Fatalf("raw response text=%q want %q", snap.RawResponseModalText, "hello raw response")
	}
}

func TestOpenHistoryActionMenuExposesSnapshotState(t *testing.T) {
	app := &App{}
	item := sharedCompat.HistoryItem{ID: "history-1", Prompt: "cat", SavedPath: "/tmp/cat.png"}

	app.openHistoryActionMenu(item, "timeline")

	snap := app.readSnapshot()
	if snap.HistoryActionMenuItem.ID != item.ID {
		t.Fatalf("menu item id=%q want %q", snap.HistoryActionMenuItem.ID, item.ID)
	}
	if snap.HistoryActionMenuContext != "timeline" {
		t.Fatalf("menu context=%q want timeline", snap.HistoryActionMenuContext)
	}
	if app.historyActionMenuPos == (image.Point{}) {
		t.Fatal("history action menu should capture a fallback anchor position")
	}

	app.closeHistoryActionMenu()
	snap = app.readSnapshot()
	if snap.HistoryActionMenuItem.ID != "" || snap.HistoryActionMenuContext != "" {
		t.Fatalf("menu should be cleared, got item=%q context=%q", snap.HistoryActionMenuItem.ID, snap.HistoryActionMenuContext)
	}
}

func TestCanvasShortcutModalsOpenIncludesHistoryActionMenu(t *testing.T) {
	app := &App{}
	snap := snapshot{HistoryActionMenuItem: sharedCompat.HistoryItem{ID: "history-1", SavedPath: "/tmp/cat.png"}}
	if !app.canvasShortcutModalsOpen(snap) {
		t.Fatal("history action menu should block canvas shortcuts while open")
	}
}

func TestHistoryActionMenuDetailKeepsTimelineContextOpen(t *testing.T) {
	app := &App{}
	item := sharedCompat.HistoryItem{ID: "history-2", Prompt: "dog", SavedPath: "/tmp/dog.png"}
	app.historyTimelineOpen = true

	app.triggerHistoryActionMenu(layout.Context{}, "detail", item, "timeline")

	snap := app.readSnapshot()
	if !snap.HistoryTimelineOpen {
		t.Fatal("timeline should stay open after opening detail from timeline context")
	}
	if snap.ActiveResultDetail.ID != item.ID {
		t.Fatalf("active detail id=%q want %q", snap.ActiveResultDetail.ID, item.ID)
	}
}

func TestHistoryActionMenuOpenRawKeepsPromptGroupContextOpen(t *testing.T) {
	app := &App{}
	path := registerVirtualText("raw payload", "raw.txt")
	item := sharedCompat.HistoryItem{ID: "history-3", Prompt: "bird", RawPath: path}
	app.activePromptGroup = historyPromptGroup{Key: "group-1"}

	app.triggerHistoryActionMenu(layout.Context{}, "open-raw", item, "prompt-group")

	snap := app.readSnapshot()
	if snap.ActivePromptGroup.Key != "group-1" {
		t.Fatalf("prompt group should stay open, got key=%q", snap.ActivePromptGroup.Key)
	}
	if snap.RawResponseModalPath != path {
		t.Fatalf("raw path=%q want %q", snap.RawResponseModalPath, path)
	}
}

func TestCloseHistoryTimelineClearsHistoryActionMenuState(t *testing.T) {
	app := &App{}
	app.historyTimelineOpen = true
	app.historyActionMenuItem = sharedCompat.HistoryItem{ID: "history-4", Prompt: "fox"}
	app.historyActionMenuContext = "timeline"

	app.closeHistoryTimeline()

	snap := app.readSnapshot()
	if snap.HistoryTimelineOpen {
		t.Fatal("timeline should be closed")
	}
	if snap.HistoryActionMenuItem.ID != "" || snap.HistoryActionMenuContext != "" {
		t.Fatalf("history action menu should be cleared, got item=%q context=%q", snap.HistoryActionMenuItem.ID, snap.HistoryActionMenuContext)
	}
}

func TestReuseHistoryItemAsSourceAppendsSavedPath(t *testing.T) {
	app := &App{}
	item := sharedCompat.HistoryItem{
		ID:        "history-5",
		Prompt:    "horse",
		SavedPath: "/tmp/horse.png",
	}

	app.reuseHistoryItemAsSource(item)

	paths := app.sourcePaths()
	if len(paths) != 1 || paths[0] != item.SavedPath {
		t.Fatalf("source paths=%v want [%s]", paths, item.SavedPath)
	}
}

func TestHandlePromptGroupItemClickDoubleClickReusesAndKeepsGroupOpen(t *testing.T) {
	app := &App{}
	app.activePromptGroup = historyPromptGroup{Key: "group-2"}
	item := sharedCompat.HistoryItem{
		ID:        "history-6",
		Prompt:    "rabbit",
		SavedPath: "/tmp/rabbit.png",
	}

	app.handlePromptGroupItemClick(widget.Click{NumClicks: 2}, item)

	paths := app.sourcePaths()
	if len(paths) != 1 || paths[0] != item.SavedPath {
		t.Fatalf("source paths=%v want [%s]", paths, item.SavedPath)
	}
	if snap := app.readSnapshot(); snap.ActivePromptGroup.Key != "group-2" {
		t.Fatalf("prompt group should stay open, got key=%q", snap.ActivePromptGroup.Key)
	}
}

func TestHandlePromptGroupItemClickShiftTogglesCompareAndKeepsGroupOpen(t *testing.T) {
	app := &App{}
	app.activePromptGroup = historyPromptGroup{Key: "group-4"}
	item := sharedCompat.HistoryItem{
		ID:        "history-9",
		Prompt:    "whale",
		SavedPath: "/tmp/whale.png",
	}

	app.handlePromptGroupItemClick(widget.Click{Modifiers: key.ModShift}, item)

	snap := app.readSnapshot()
	if snap.Compare.Item.ID != item.ID {
		t.Fatalf("compare id=%q want %q", snap.Compare.Item.ID, item.ID)
	}
	if snap.ActivePromptGroup.Key != "group-4" {
		t.Fatalf("prompt group should stay open, got key=%q", snap.ActivePromptGroup.Key)
	}
}

func TestPromptGroupHeroLatestUsesSingleSelectPreviewBehavior(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "hero-latest-full.png")
	thumbPath := filepath.Join(dir, "hero-latest-thumb.png")
	writeSolidTestPNG(t, fullPath, color.NRGBA{R: 0xaa, G: 0x44, B: 0x66, A: 0xff})
	writeSolidTestPNG(t, thumbPath, color.NRGBA{R: 0x33, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	app.activePromptGroup = historyPromptGroup{Key: "group-hero"}
	item := sharedCompat.HistoryItem{
		ID:          "hero-latest",
		Prompt:      "hero latest",
		PreviewPath: thumbPath,
		SavedPath:   fullPath,
		ThumbPath:   thumbPath,
	}

	if err := app.loadHistoryPreview(item, true); err != nil {
		t.Fatalf("loadHistoryPreview: %v", err)
	}

	snap := app.readSnapshot()
	if snap.SelectedHistoryID != item.ID {
		t.Fatalf("selectedHistoryID=%q want %q", snap.SelectedHistoryID, item.ID)
	}
	if snap.ActivePromptGroup.Key != "group-hero" {
		t.Fatalf("prompt group should stay open after latest preview, got key=%q", snap.ActivePromptGroup.Key)
	}
	if len(app.sourcePaths()) != 0 {
		t.Fatalf("latest preview should not reuse as source, got %v", app.sourcePaths())
	}
	if snap.Compare.Item.ID != "" {
		t.Fatalf("latest preview should not toggle compare, got %q", snap.Compare.Item.ID)
	}
}

func TestHandleHistoryGroupSummaryClickShiftTogglesCompare(t *testing.T) {
	app := &App{}
	group := historyPromptGroup{
		Key: "group-3",
		Representative: sharedCompat.HistoryItem{
			ID:        "history-7",
			Prompt:    "owl",
			SavedPath: "/tmp/owl.png",
		},
	}

	app.handleHistoryGroupSummaryClick(widget.Click{Modifiers: key.ModShift}, group, false)

	snap := app.readSnapshot()
	if snap.Compare.Item.ID != group.Representative.ID {
		t.Fatalf("compare id=%q want %q", snap.Compare.Item.ID, group.Representative.ID)
	}
}

func TestHandleHistoryGroupSummaryClickShiftClearsCompareWhenGroupAlreadyCompared(t *testing.T) {
	app := &App{}
	other := sharedCompat.HistoryItem{
		ID:        "history-8",
		Prompt:    "owl",
		SavedPath: "/tmp/owl-alt.png",
	}
	group := historyPromptGroup{
		Key:            "group-3",
		Representative: sharedCompat.HistoryItem{ID: "history-7", Prompt: "owl", SavedPath: "/tmp/owl.png"},
		Items:          []*sharedCompat.HistoryItem{{ID: "history-7", Prompt: "owl", SavedPath: "/tmp/owl.png"}, &other},
	}
	app.compare = resultState{Item: other, HasItem: true, Rev: 1}

	app.handleHistoryGroupSummaryClick(widget.Click{Modifiers: key.ModShift}, group, false)

	snap := app.readSnapshot()
	if snap.Compare.Item.ID != "" || snap.Compare.HasItem {
		t.Fatalf("compare should be cleared, got id=%q hasItem=%v", snap.Compare.Item.ID, snap.Compare.HasItem)
	}
}

func TestBuildHistoryPromptEntriesUsesEmptyPromptLabelFallback(t *testing.T) {
	items := []sharedCompat.HistoryItem{
		{ID: "a1", Prompt: ""},
		{ID: "a2", Prompt: "   "},
	}

	entries := buildHistoryPromptEntries(items)
	if len(entries) != 1 || entries[0].Kind != "group" || entries[0].Group == nil {
		t.Fatalf("entries=%#v want single prompt group", entries)
	}
	if entries[0].Group.Title != "(无 prompt)" {
		t.Fatalf("group title=%q want (无 prompt)", entries[0].Group.Title)
	}
	if entries[0].Group.PromptPreview != "(无 prompt)" {
		t.Fatalf("group preview=%q want (无 prompt)", entries[0].Group.PromptPreview)
	}
}

func TestToggleExpandedPromptGroupTogglesEntry(t *testing.T) {
	app := &App{}

	app.toggleExpandedPromptGroup("group-1")
	if !app.expandedPromptGroups["group-1"] {
		t.Fatalf("expected group-1 to be expanded")
	}

	app.toggleExpandedPromptGroup("group-1")
	if _, ok := app.expandedPromptGroups["group-1"]; ok {
		t.Fatalf("expected group-1 to be collapsed")
	}

	app.toggleExpandedPromptGroup("")
	if len(app.expandedPromptGroups) != 0 {
		t.Fatalf("expected empty key to be ignored, got %v", app.expandedPromptGroups)
	}
}

func TestHandleHistoryItemClickDoubleClickReusesSource(t *testing.T) {
	app := &App{}
	item := sharedCompat.HistoryItem{
		ID:        "history-8",
		Prompt:    "seal",
		SavedPath: "/tmp/seal.png",
	}

	app.handleHistoryItemClick(widget.Click{NumClicks: 2}, item, false)

	paths := app.sourcePaths()
	if len(paths) != 1 || paths[0] != item.SavedPath {
		t.Fatalf("source paths=%v want [%s]", paths, item.SavedPath)
	}
}

func TestHistoryActionMenuEntriesIncludesDragOutOnDarwinWhenSavable(t *testing.T) {
	app := &App{}
	item := sharedCompat.HistoryItem{ID: "history-10", Prompt: "koala", SavedPath: "/tmp/koala.png"}

	entries := app.historyActionMenuEntries(item, "history", "")
	found := false
	for _, entry := range entries {
		if entry.ID != "drag-out" {
			continue
		}
		found = true
		wantDisabled := runtime.GOOS != "darwin"
		if entry.Disabled != wantDisabled {
			t.Fatalf("drag-out disabled=%v want %v on %s", entry.Disabled, wantDisabled, runtime.GOOS)
		}
	}
	if !found {
		t.Fatal("expected drag-out history action entry")
	}
}

func TestBeginNativeFileDragRejectsVirtualPath(t *testing.T) {
	app := &App{}
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString([]byte("img")), "virtual.png", "png")

	err := app.beginNativeFileDrag(virtualPath)
	if err == nil {
		t.Fatal("expected error for virtual drag path")
	}
	if !strings.Contains(err.Error(), "本地文件") && runtime.GOOS == "darwin" {
		t.Fatalf("unexpected darwin error: %v", err)
	}
}

func TestPrepareHistoryItemForNativeDragMaterializesInlineImage(t *testing.T) {
	app := &App{}
	dir := t.TempDir()
	app.outputDirInput.SetText(dir)
	validPath := filepath.Join(dir, "valid.png")
	writeSolidTestPNG(t, validPath, color.NRGBA{R: 0x44, G: 0x77, B: 0xaa, A: 0xff})
	raw, readErr := os.ReadFile(validPath)
	if readErr != nil {
		t.Fatalf("read valid png: %v", readErr)
	}
	item := sharedCompat.HistoryItem{
		ID:           "history-11",
		Prompt:       "otter",
		OutputFormat: "png",
		ImageB64:     base64.StdEncoding.EncodeToString(raw),
	}
	next, path, err := app.prepareHistoryItemForNativeDrag(item)
	if err != nil {
		t.Fatalf("prepareHistoryItemForNativeDrag: %v", err)
	}
	if path == "" || next.SavedPath == "" {
		t.Fatalf("expected materialized path, got path=%q item=%#v", path, next)
	}
}

func TestPromptGroupForHistoryItemCacheRefreshesAfterHistoryRevisionChanges(t *testing.T) {
	app := &App{}
	first := []sharedCompat.HistoryItem{
		{ID: "1", Prompt: "cat poster"},
		{ID: "2", Prompt: "cat poster"},
	}
	second := []sharedCompat.HistoryItem{
		{ID: "3", Prompt: "dog poster"},
		{ID: "4", Prompt: "dog poster"},
	}
	app.setHistoryLocked(first)

	group1, ok := app.promptGroupForHistoryItem(first, "2")
	if !ok || len(group1.Items) != 2 || group1.Items[0].Prompt != "cat poster" {
		t.Fatalf("group1=%+v ok=%t", group1, ok)
	}

	app.mu.Lock()
	app.setHistoryLocked(second)
	app.mu.Unlock()

	group2, ok := app.promptGroupForHistoryItem(second, "4")
	if !ok || len(group2.Items) != 2 || group2.Items[0].Prompt != "dog poster" {
		t.Fatalf("group2=%+v ok=%t", group2, ok)
	}
}

func TestEffectiveThumbMaxDimensionHonorsReducedEffects(t *testing.T) {
	app := &App{}
	if got := app.effectiveThumbMaxDimension(512); got != 512 {
		t.Fatalf("normal thumb max=%d want 512", got)
	}
	app.reducedEffects = true
	if got := app.effectiveThumbMaxDimension(512); got != reducedEffectsThumbMaxDimension {
		t.Fatalf("reduced thumb max=%d want %d", got, reducedEffectsThumbMaxDimension)
	}
	if got := app.effectiveThumbMaxDimension(128); got != 128 {
		t.Fatalf("small thumb max=%d want 128", got)
	}
}

func TestNormalizeThumbCacheDimensionBucketsSizes(t *testing.T) {
	if got := normalizeThumbCacheDimension(48); got != thumbCacheMinDimension {
		t.Fatalf("48 -> %d want %d", got, thumbCacheMinDimension)
	}
	if got := normalizeThumbCacheDimension(88); got != 96 {
		t.Fatalf("88 -> %d want 96", got)
	}
	if got := normalizeThumbCacheDimension(208); got != 224 {
		t.Fatalf("208 -> %d want 224", got)
	}
}

func TestPrepareCanvasDisplayImageHonorsReducedEffects(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4096, 2048))
	app := &App{}
	normal := app.prepareCanvasDisplayImage(img)
	if normal.Bounds().Dx() > canvasDisplayMaxDimension || normal.Bounds().Dy() > canvasDisplayMaxDimension {
		t.Fatalf("normal canvas bounds=%v exceed %d", normal.Bounds(), canvasDisplayMaxDimension)
	}
	app.reducedEffects = true
	reduced := app.prepareCanvasDisplayImage(img)
	if reduced.Bounds().Dx() > reducedEffectsCanvasDisplayMaxDimension || reduced.Bounds().Dy() > reducedEffectsCanvasDisplayMaxDimension {
		t.Fatalf("reduced canvas bounds=%v exceed %d", reduced.Bounds(), reducedEffectsCanvasDisplayMaxDimension)
	}
}

func TestApplyReducedEffectsRefreshesCanvasDisplayImage(t *testing.T) {
	dir := t.TempDir()
	fullPath := filepath.Join(dir, "canvas-large.png")
	writeSizedSolidTestPNG(t, fullPath, 4096, 2048, color.NRGBA{R: 0x66, G: 0xaa, B: 0xee, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	fullDisplay, err := app.imageForPathThumb(fullPath, canvasDisplayMaxDimension)
	if err != nil {
		t.Fatalf("imageForPathThumb full display: %v", err)
	}
	app.result = resultState{
		Image:     fullDisplay,
		SavedPath: fullPath,
		Rev:       1,
	}

	app.applyReducedEffects(true)
	if got := app.readSnapshot().Result.Image.Bounds().Dx(); got > reducedEffectsCanvasDisplayMaxDimension {
		t.Fatalf("reduced canvas width=%d want <= %d", got, reducedEffectsCanvasDisplayMaxDimension)
	}
	if !app.reducedEffects {
		t.Fatalf("expected reducedEffects to be enabled")
	}
}

func TestPruneImageCacheLockedRemovesOrphanedHistoryEntries(t *testing.T) {
	app := &App{imageCache: map[string]cachedImage{}}
	keep := sharedCompat.HistoryItem{ID: "keep", SavedPath: "/tmp/keep.png", ThumbPath: "/tmp/keep-thumb.png"}
	drop := sharedCompat.HistoryItem{ID: "drop", SavedPath: "/tmp/drop.png", ThumbPath: "/tmp/drop-thumb.png"}

	app.setHistoryLocked([]sharedCompat.HistoryItem{keep, drop})
	app.imageCache[historyImageCacheKey(keep, true)] = cachedImage{}
	app.imageCache[historyImageDisplayCacheKey(keep, 256)] = cachedImage{}
	app.imageCache["path:/tmp/keep.png"] = cachedImage{}
	app.imageCache["path-thumb:256:/tmp/keep.png"] = cachedImage{}
	app.imageCache[historyImageCacheKey(drop, true)] = cachedImage{}
	app.imageCache[historyImageDisplayCacheKey(drop, 256)] = cachedImage{}
	app.imageCache["path:/tmp/drop.png"] = cachedImage{}

	app.setHistoryLocked([]sharedCompat.HistoryItem{keep})

	if _, ok := app.imageCache[historyImageCacheKey(keep, true)]; !ok {
		t.Fatalf("expected keep history thumb cache to remain")
	}
	if _, ok := app.imageCache[historyImageDisplayCacheKey(keep, 256)]; !ok {
		t.Fatalf("expected keep display thumb cache to remain")
	}
	if _, ok := app.imageCache["path:/tmp/keep.png"]; !ok {
		t.Fatalf("expected keep path cache to remain")
	}
	if _, ok := app.imageCache[historyImageCacheKey(drop, true)]; ok {
		t.Fatalf("expected dropped history thumb cache to be pruned")
	}
	if _, ok := app.imageCache[historyImageDisplayCacheKey(drop, 256)]; ok {
		t.Fatalf("expected dropped display thumb cache to be pruned")
	}
	if _, ok := app.imageCache["path:/tmp/drop.png"]; ok {
		t.Fatalf("expected dropped path cache to be pruned")
	}
}

func TestPruneImageCacheLockedKeepsSelectedHistoryOutsideRecentWindow(t *testing.T) {
	app := &App{imageCache: map[string]cachedImage{}}
	items := make([]sharedCompat.HistoryItem, 0, historyCacheRetention+5)
	for i := 0; i < historyCacheRetention+5; i++ {
		items = append(items, sharedCompat.HistoryItem{
			ID:        fmt.Sprintf("hist-%03d", i),
			SavedPath: fmt.Sprintf("/tmp/hist-%03d.png", i),
		})
	}
	selected := items[len(items)-1]
	app.setHistoryLocked(items)
	app.selectedHistoryID = selected.ID
	app.imageCache[historyImageCacheKey(selected, true)] = cachedImage{}
	app.imageCache[historyImageDisplayCacheKey(selected, 256)] = cachedImage{}

	app.pruneImageCacheLocked()

	if _, ok := app.imageCache[historyImageCacheKey(selected, true)]; !ok {
		t.Fatalf("expected selected history thumb cache to be retained")
	}
	if _, ok := app.imageCache[historyImageDisplayCacheKey(selected, 256)]; !ok {
		t.Fatalf("expected selected history display cache to be retained")
	}
}

func TestSetHistoryLockedResetsExpandedPromptGroups(t *testing.T) {
	app := &App{
		expandedPromptGroups: map[string]bool{
			"prompt:old": true,
		},
	}
	app.setHistoryLocked([]sharedCompat.HistoryItem{{ID: "1", Prompt: "new"}})
	if len(app.expandedPromptGroups) != 0 {
		t.Fatalf("expandedPromptGroups=%v want empty after history reset", app.expandedPromptGroups)
	}
}

func TestFinishCachedImageIfLoadingSkipsPrunedEntries(t *testing.T) {
	app := &App{imageCache: map[string]cachedImage{}}
	key := "history-thumb:/tmp/example"
	app.imageCache[key] = cachedImage{Loading: true}
	app.finishCachedImageIfLoading(key, cachedImage{Image: image.NewRGBA(image.Rect(0, 0, 1, 1))})
	if cached, ok := app.imageCache[key]; !ok || cached.Loading || cached.Image == nil {
		t.Fatalf("expected loading cache to be finalized: %#v", cached)
	}

	delete(app.imageCache, key)
	app.finishCachedImageIfLoading(key, cachedImage{Image: image.NewRGBA(image.Rect(0, 0, 1, 1))})
	if _, ok := app.imageCache[key]; ok {
		t.Fatalf("expected pruned cache entry to stay absent")
	}
}

func TestBuildHistoryPromptEntriesLimitedKeepsLaterItemsForVisibleGroups(t *testing.T) {
	items := []sharedCompat.HistoryItem{
		{ID: "a1", Prompt: "A"},
		{ID: "b1", Prompt: "B"},
		{ID: "c1", Prompt: "C"},
		{ID: "a2", Prompt: "A"},
	}
	entries := buildHistoryPromptEntriesLimited(items, 2)
	if len(entries) != 2 {
		t.Fatalf("entries=%d want 2", len(entries))
	}
	if entries[0].Group.Key != "prompt:a" || len(entries[0].Group.Items) != 2 {
		t.Fatalf("first group=%+v want prompt:a with 2 items", entries[0].Group)
	}
	if entries[0].Group.CountText != "2 张" || entries[0].Group.PromptPreview != "A" || entries[0].Group.Title != "A" {
		t.Fatalf("first group display=%+v want count/title/prompt preview populated", entries[0].Group)
	}
	if entries[1].Group.Key != "prompt:b" || len(entries[1].Group.Items) != 1 {
		t.Fatalf("second group=%+v want prompt:b with 1 item", entries[1].Group)
	}
}

func TestContainNoUpscaleSize(t *testing.T) {
	if got := containNoUpscaleSize(512, 512, 1200, 900); got != (image.Pt(512, 512)) {
		t.Fatalf("containNoUpscaleSize upscale=%v want 512x512", got)
	}
	if got := containNoUpscaleSize(2048, 1024, 800, 600); got != (image.Pt(800, 400)) {
		t.Fatalf("containNoUpscaleSize downscale=%v want 800x400", got)
	}
	if got := containNoUpscaleSize(1024, 2048, 800, 600); got != (image.Pt(300, 600)) {
		t.Fatalf("containNoUpscaleSize portrait=%v want 300x600", got)
	}
}

func TestPrefillControlsFromHistoryItemRestoresSourcePaths(t *testing.T) {
	app := New()
	app.prefillControlsFromHistoryItem(sharedCompat.HistoryItem{
		Mode:        "edit",
		SavedPath:   "/tmp/fallback.png",
		SourcePaths: []string{"/tmp/a.png", "/tmp/b.png", "/tmp/a.png"},
	})
	if got := app.sourcePaths(); len(got) != 2 || got[0] != "/tmp/a.png" || got[1] != "/tmp/b.png" {
		t.Fatalf("sourcePaths=%v want [/tmp/a.png /tmp/b.png]", got)
	}
}

func TestPrefillControlsFromHistoryItemFallsBackToParentIDBeforeSavedPath(t *testing.T) {
	app := New()
	app.prefillControlsFromHistoryItem(sharedCompat.HistoryItem{
		Mode:      "edit",
		ParentID:  "/tmp/source-parent.png",
		SavedPath: "/tmp/result.png",
	})
	if got := app.sourcePaths(); len(got) != 1 || got[0] != "/tmp/source-parent.png" {
		t.Fatalf("sourcePaths=%v want [/tmp/source-parent.png]", got)
	}
}

func TestApplyHistoryParamsDoesNotRestoreSourcePaths(t *testing.T) {
	app := New()
	app.sourcePathsInput.SetText("/tmp/current.png")
	app.applyHistoryParams(sharedCompat.HistoryItem{
		Prompt:       "history prompt",
		Mode:         "edit",
		Size:         "1536x1024",
		Quality:      "high",
		SavedPath:    "/tmp/fallback.png",
		SourcePaths:  []string{"/tmp/a.png", "/tmp/b.png"},
		OutputFormat: "png",
	})
	if got := app.promptInput.Text(); got != "history prompt" {
		t.Fatalf("prompt=%q want history prompt", got)
	}
	if app.mode != "edit" {
		t.Fatalf("mode=%q want edit", app.mode)
	}
	if app.size != "1536x1024" {
		t.Fatalf("size=%q want 1536x1024", app.size)
	}
	if app.quality != "high" {
		t.Fatalf("quality=%q want high", app.quality)
	}
	if got := app.sourcePaths(); len(got) != 1 || got[0] != "/tmp/current.png" {
		t.Fatalf("sourcePaths=%v want existing source path unchanged", got)
	}
}

func TestResolveThemeMode(t *testing.T) {
	prev := systemThemeResolver
	systemThemeResolver = func() string { return "dark" }
	defer func() { systemThemeResolver = prev }()
	if got := resolveThemeMode("dark"); got != "dark" {
		t.Fatalf("resolveThemeMode(dark)=%q", got)
	}
	if got := resolveThemeMode("system"); got != "dark" {
		t.Fatalf("resolveThemeMode(system)=%q", got)
	}
	if got := normalizeThemeMode("unknown"); got != "system" {
		t.Fatalf("normalizeThemeMode(unknown)=%q", got)
	}
}

func TestIsDarkThemeUsesCachedResolution(t *testing.T) {
	prev := systemThemeResolver
	resolverCalls := 0
	systemThemeResolver = func() string {
		resolverCalls++
		return "light"
	}
	defer func() { systemThemeResolver = prev }()

	app := &App{themeMode: "system", resolvedThemeMode: "dark"}
	for i := 0; i < 10; i++ {
		if !app.isDarkTheme() {
			t.Fatal("cached dark theme should remain dark")
		}
	}
	if resolverCalls != 0 {
		t.Fatalf("system theme resolver called %d times from render hot path", resolverCalls)
	}
}

func TestRefreshSystemThemeOnlyResolvesSystemMode(t *testing.T) {
	prevResolver := systemThemeResolver
	prevPalette := fluent
	resolverCalls := 0
	systemThemeResolver = func() string {
		resolverCalls++
		return "dark"
	}
	defer func() {
		systemThemeResolver = prevResolver
		fluent = prevPalette
	}()

	app := &App{themeMode: "light", resolvedThemeMode: "light"}
	app.refreshSystemTheme()
	if resolverCalls != 0 {
		t.Fatalf("fixed theme unexpectedly queried system resolver %d times", resolverCalls)
	}
	app.themeMode = "system"
	app.refreshSystemTheme()
	if resolverCalls != 1 || app.resolvedThemeMode != "dark" {
		t.Fatalf("refresh calls=%d resolved=%q want 1,dark", resolverCalls, app.resolvedThemeMode)
	}
}

func TestDesktopFramesDoNotResolveSystemTheme(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	prevResolver := systemThemeResolver
	prevPalette := fluent
	resolverCalls := 0
	systemThemeResolver = func() string {
		resolverCalls++
		return "dark"
	}
	defer func() {
		systemThemeResolver = prevResolver
		fluent = prevPalette
	}()

	app := New()
	if resolverCalls != 1 {
		t.Fatalf("New resolved system theme %d times want 1", resolverCalls)
	}
	resolverCalls = 0
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Constraints: layout.Exact(image.Pt(1920, 1080)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Now:         time.Now(),
	}
	for frame := 0; frame < 3; frame++ {
		ops.Reset()
		gtx.Now = gtx.Now.Add(time.Second / 60)
		app.layout(gtx)
	}
	if resolverCalls != 0 {
		t.Fatalf("desktop frames resolved system theme %d times want 0", resolverCalls)
	}
}

func TestToggleFullscreenWithoutWindow(t *testing.T) {
	app := &App{}
	app.toggleFullscreen()
	if !app.fullscreen {
		t.Fatalf("fullscreen should be true after first toggle")
	}
	app.toggleFullscreen()
	if app.fullscreen {
		t.Fatalf("fullscreen should be false after second toggle")
	}
}

func TestParseDialogPathsDeduplicatesAndTrims(t *testing.T) {
	got := parseDialogPaths(" /tmp/a.png \n\"/tmp/b.jpg\"\n/tmp/a.png\n")
	if len(got) != 2 {
		t.Fatalf("len(parseDialogPaths)=%d want 2", len(got))
	}
	if got[0] != "/tmp/a.png" || got[1] != "/tmp/b.jpg" {
		t.Fatalf("parseDialogPaths=%v", got)
	}
}

func TestParseDialogPathsAcceptsAppleScriptMultiselectOutput(t *testing.T) {
	got := parseDialogPaths("/Users/test/Pictures/first image.png\r/Users/test/Pictures/second image.jpg\r")
	want := []string{
		"/Users/test/Pictures/first image.png",
		"/Users/test/Pictures/second image.jpg",
	}
	if len(got) != len(want) {
		t.Fatalf("len(parseDialogPaths)=%d want %d: %v", len(got), len(want), got)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("parseDialogPaths[%d]=%q want %q", idx, got[idx], want[idx])
		}
	}
}
