package ui

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yuanhua/image-gptcodex/pkg/client"
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

func TestWorkspaceSwitchPreservesPrompt(t *testing.T) {
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

func TestAdvancedSummaryRefreshesAfterRelevantChanges(t *testing.T) {
	app := &App{
		format:     "png",
		background: "transparent",
		moderation: "auto",
	}
	app.negativePromptInput.SetText("no watermark")
	app.partialImagesInput.SetText("0")
	first := app.advancedSummary()
	if !strings.Contains(first, "仅最终图") {
		t.Fatalf("first=%q want partial-preview summary", first)
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
	if len(first) != 1 || first[0].Title != "first prompt" {
		t.Fatalf("first=%v want single first prompt item", first)
	}

	second := app.promptLabelsCached([]string{"second prompt"})
	if len(second) != 1 || second[0].Title != "second prompt" {
		t.Fatalf("second=%v want single second prompt item", second)
	}
	if first[0].Title == second[0].Title {
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
		KernelRuntimeMode: "remote",
		BatchCount:        4,
	})

	if app.size != "1536x1024" || app.quality != "high" || app.format != "webp" {
		t.Fatalf("basic preset fields not applied: size=%q quality=%q format=%q", app.size, app.quality, app.format)
	}
	if app.negativePromptInput.Text() != "no watermark" {
		t.Fatalf("negativePrompt=%q want no watermark", app.negativePromptInput.Text())
	}
	if app.background != "transparent" || app.outputCompressionInput.Text() != "87" || app.inputFidelity != "high" || app.imageStyle != "vivid" || app.moderation != "auto" || app.styleTag != "anime" || app.kernelRuntimeMode != "remote" || app.batchCount != 4 {
		t.Fatalf("extended preset fields not applied: background=%q compression=%q fidelity=%q imageStyle=%q moderation=%q style=%q runtime=%q batch=%d", app.background, app.outputCompressionInput.Text(), app.inputFidelity, app.imageStyle, app.moderation, app.styleTag, app.kernelRuntimeMode, app.batchCount)
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

	app := &App{running: true}
	app.applyPartialPreview(client.PartialImage{
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
	assertImagePixelColor(t, app.result.Image, fill)
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
