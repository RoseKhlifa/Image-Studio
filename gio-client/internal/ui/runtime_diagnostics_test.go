package ui

import (
	"encoding/base64"
	"image"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"gioui.org/gpu"
	"gioui.org/widget"
	sharedCompat "image-studio/shared/compat"
)

type fakeContextWithAPI struct {
	api gpu.API
}

func (f fakeContextWithAPI) API() gpu.API {
	return f.api
}

func TestFormatRenderDiagnosticsIncludesBackendAndTiming(t *testing.T) {
	snap := snapshot{
		RenderBackend:   "Vulkan",
		RenderFrameTime: 25 * time.Millisecond,
		RenderFPS:       40,
		RenderActive:    true,
	}
	got := formatRenderDiagnostics(snap)
	want := "GPU Vulkan · 40.0 FPS · 25.0ms"
	if got != want {
		t.Fatalf("formatRenderDiagnostics=%q want %q", got, want)
	}
}

func TestFormatRenderDiagnosticsShowsIdleWhenInactive(t *testing.T) {
	snap := snapshot{RenderBackend: "Metal"}
	got := formatRenderDiagnostics(snap)
	want := "GPU Metal · 空闲"
	if got != want {
		t.Fatalf("formatRenderDiagnostics=%q want %q", got, want)
	}
}

func TestSlowestLayoutSamplePrefersLargestVisibleDuration(t *testing.T) {
	got := slowestLayoutSample([]layoutTimingSample{
		{name: "shell", duration: 3 * time.Millisecond, visible: true},
		{name: "controls", duration: 9 * time.Millisecond, visible: true},
		{name: "timeline", duration: 15 * time.Millisecond, visible: false},
	})
	if got != "controls=9.0ms" {
		t.Fatalf("slowestLayoutSample=%q want controls=9.0ms", got)
	}
}

func TestDescribeRenderAPI(t *testing.T) {
	tests := []struct {
		name string
		api  gpu.API
		want string
	}{
		{name: "d3d11", api: gpu.Direct3D11{}, want: "Direct3D 11"},
		{name: "vulkan", api: gpu.Vulkan{}, want: "Vulkan"},
		{name: "metal", api: gpu.Metal{}, want: "Metal"},
		{name: "opengl", api: gpu.OpenGL{}, want: "OpenGL"},
	}
	for _, tt := range tests {
		if got := describeRenderAPI(tt.api); got != tt.want {
			t.Fatalf("%s: describeRenderAPI=%q want %q", tt.name, got, tt.want)
		}
	}
}

func TestRenderBackendFromContext(t *testing.T) {
	got := renderBackendFromContext(fakeContextWithAPI{api: gpu.Direct3D11{}})
	if got != "Direct3D 11" {
		t.Fatalf("renderBackendFromContext=%q want Direct3D 11", got)
	}
	if got := renderBackendFromContext(struct{}{}); got != "" {
		t.Fatalf("renderBackendFromContext(non-provider)=%q want empty", got)
	}
}

func TestMaybeRecordLowFPSLockedThrottlesLogs(t *testing.T) {
	resetThumbDiagnosticsCounters()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if thumbDecodeQueueLen() == 0 && thumbDecodeBusyCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	resetThumbDiagnosticsCounters()
	app := &App{
		renderBackend:           "Vulkan",
		frameRawFPS:             8.5,
		frameRawIntervalEMA:     120 * time.Millisecond,
		running:                 true,
		layoutShellEMA:          5 * time.Millisecond,
		layoutControlsEMA:       6 * time.Millisecond,
		layoutSubmitDockEMA:     7 * time.Millisecond,
		layoutActionsEMA:        8 * time.Millisecond,
		layoutPromptCardEMA:     9 * time.Millisecond,
		layoutComposeCardEMA:    10 * time.Millisecond,
		layoutAdvancedCardEMA:   11 * time.Millisecond,
		layoutCanvasEMA:         3 * time.Millisecond,
		layoutCanvasToolbarEMA:  12 * time.Millisecond,
		layoutResultSurfaceEMA:  13 * time.Millisecond,
		layoutCanvasStatusEMA:   14 * time.Millisecond,
		layoutHistoryRailEMA:    2 * time.Millisecond,
		layoutUpstreamCardEMA:   4 * time.Millisecond,
		layoutHistorySummaryEMA: 5 * time.Millisecond,
		layoutLatestHistoryEMA:  6 * time.Millisecond,
		layoutHistoryResultsEMA: 7 * time.Millisecond,
		layoutTimelineModalEMA:  4 * time.Millisecond,
		history: []sharedCompat.HistoryItem{
			{ID: "a"},
			{ID: "b"},
		},
		historyTimelineOpen: true,
		resultGridOpen:      true,
	}
	app.result = resultState{
		SavedPath: "/tmp/result.png",
		Item: sharedCompat.HistoryItem{
			ID:          "result",
			SavedPath:   "/tmp/result.png",
			PreviewPath: "/tmp/result-preview.png",
			ThumbPath:   "/tmp/result-thumb.png",
		},
		HasItem: true,
	}
	now := time.Unix(100, 0)
	app.noteRenderActivityLocked(now)

	for i := 0; i < lowFPSMinSamples-1; i++ {
		if app.maybeRecordLowFPSLocked(now.Add(time.Duration(i) * 100 * time.Millisecond)) {
			t.Fatalf("did not expect log before enough samples")
		}
	}
	if !app.maybeRecordLowFPSLocked(now.Add(time.Duration(lowFPSMinSamples-1) * 100 * time.Millisecond)) {
		t.Fatalf("expected low-fps log after enough samples")
	}
	if len(app.logs) != 1 {
		t.Fatalf("logs=%v want single low-fps entry", app.logs)
	}
	if !strings.Contains(app.logs[0], "history=2") || !strings.Contains(app.logs[0], "running=true") || !strings.Contains(app.logs[0], "timeline=true") || !strings.Contains(app.logs[0], "grid=true") || !strings.Contains(app.logs[0], "layout_ms shell=5.0 controls=6.0 submit=7.0 actions=8.0 prompt=9.0 compose=10.0 advanced=11.0 canvas=3.0 canvas_toolbar=12.0 result_surface=13.0 canvas_status=14.0 rail=2.0 upstream=4.0 summary=5.0 latest=6.0 results=7.0 timeline=4.0") || !strings.Contains(app.logs[0], "slowest=canvas_status=14.0ms") || !strings.Contains(app.logs[0], "history_thumb=0/2(0.0%)") || !strings.Contains(app.logs[0], "history_preview=0/2(0.0%)") || !strings.Contains(app.logs[0], "history_backfill=0") || !strings.Contains(app.logs[0], "history_prewarm=0/0(0.0ms,none)") || !strings.Contains(app.logs[0], "thumb_src=0/0/0") || !strings.Contains(app.logs[0], "thumb_queue=") || !strings.Contains(app.logs[0], "thumb_busy=") || !strings.Contains(app.logs[0], "thumb_req=0") || !strings.Contains(app.logs[0], "thumb_hit=0") || !strings.Contains(app.logs[0], "thumb_miss=0") || !strings.Contains(app.logs[0], "thumb_hit_rate=0.0%") || !strings.Contains(app.logs[0], "canvas_src=0/0/0/0") || !strings.Contains(app.logs[0], "current_result=false/false/false target=2048 managed=false") || !strings.Contains(app.logs[0], "goroutines=") || !strings.Contains(app.logs[0], "alloc=") || !strings.Contains(app.logs[0], "建议切换低特效模式") {
		t.Fatalf("log=%q missing expected diagnostics", app.logs[0])
	}
	if app.maybeRecordLowFPSLocked(now.Add(5 * time.Second)) {
		t.Fatalf("expected throttling within 10 seconds")
	}
	if len(app.logs) != 1 {
		t.Fatalf("logs=%v want still single entry", app.logs)
	}
	if !app.maybeRecordLowFPSLocked(now.Add(11 * time.Second)) {
		t.Fatalf("expected logging after throttle window")
	}
	if len(app.logs) != 2 {
		t.Fatalf("logs=%v want second low-fps entry", app.logs)
	}
}

func TestMaybeRecordLowFPSLockedOmitsLowEffectsHintWhenAlreadyEnabled(t *testing.T) {
	app := &App{
		renderBackend:       "Vulkan",
		frameRawFPS:         8.5,
		frameRawIntervalEMA: 120 * time.Millisecond,
		reducedEffects:      true,
	}
	now := time.Unix(200, 0)
	app.noteRenderActivityLocked(now)
	for i := 0; i < lowFPSMinSamples; i++ {
		app.maybeRecordLowFPSLocked(now.Add(time.Duration(i) * 100 * time.Millisecond))
	}
	if len(app.logs) != 1 {
		t.Fatalf("logs=%v want single low-fps entry", app.logs)
	}
	if strings.Contains(app.logs[0], "建议切换低特效模式") {
		t.Fatalf("log=%q should not contain reduced-effects hint", app.logs[0])
	}
}

func TestMaybeRecordLowFPSLockedSkipsIdleSamplesWithoutRecentActivity(t *testing.T) {
	app := &App{
		renderBackend:       "Vulkan",
		frameRawFPS:         5,
		frameRawIntervalEMA: 200 * time.Millisecond,
	}
	now := time.Unix(300, 0)
	for i := 0; i < lowFPSMinSamples+2; i++ {
		if app.maybeRecordLowFPSLocked(now.Add(time.Duration(i) * 200 * time.Millisecond)) {
			t.Fatalf("unexpected low-fps log for idle samples")
		}
	}
	if len(app.logs) != 0 {
		t.Fatalf("logs=%v want empty", app.logs)
	}
}

func TestRecordRenderFrameZeroTimeDoesNotLeaveMutexLocked(t *testing.T) {
	app := &App{}
	done := make(chan struct{})
	go func() {
		app.recordRenderFrame(time.Time{}, image.Point{})
		app.mu.Lock()
		app.mu.Unlock()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recordRenderFrame left mutex locked on zero time")
	}
}

func TestRecordRenderFrameSkipsIdleGapsForFPSSampling(t *testing.T) {
	app := &App{}
	start := time.Unix(100, 0)
	app.recordRenderFrame(start, image.Pt(100, 100))
	app.recordRenderFrame(start.Add(500*time.Millisecond), image.Pt(100, 100))
	if app.frameFPS != 0 {
		t.Fatalf("frameFPS=%v want 0 after idle gap reset", app.frameFPS)
	}
	if app.lowFPSStreak != 0 {
		t.Fatalf("lowFPSStreak=%d want 0 after idle gap reset", app.lowFPSStreak)
	}
}

func TestRecordRenderFrameTreatsSlowIdleCadenceAsIdle(t *testing.T) {
	app := &App{}
	start := time.Unix(120, 0)
	app.recordRenderFrame(start, image.Pt(100, 100))
	app.recordRenderFrame(start.Add(200*time.Millisecond), image.Pt(100, 100))
	if app.renderActive {
		t.Fatalf("renderActive=%t want false for idle cadence", app.renderActive)
	}
	if app.frameFPS != 0 {
		t.Fatalf("frameFPS=%v want 0 for idle cadence", app.frameFPS)
	}
	if app.frameRawFPS == 0 {
		t.Fatalf("frameRawFPS=%v want raw slow cadence to be recorded", app.frameRawFPS)
	}
}

func TestRecordRenderFrameKeepsSlowCadenceActiveAfterRecentActivity(t *testing.T) {
	app := &App{}
	start := time.Unix(140, 0)
	app.noteRenderActivityLocked(start)
	app.recordRenderFrame(start, image.Pt(100, 100))
	app.recordRenderFrame(start.Add(120*time.Millisecond), image.Pt(100, 100))
	if !app.renderActive {
		t.Fatalf("renderActive=%t want true after recent activity", app.renderActive)
	}
	if app.frameFPS == 0 {
		t.Fatalf("frameFPS=%v want active frame sample", app.frameFPS)
	}
}

func TestWriteLowFPSDiagnosticsSnapshotCreatesFile(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", root)
	path, err := writeLowFPSDiagnosticsSnapshot("diagnostics body", time.Date(2026, time.June, 5, 12, 34, 56, 0, time.Local))
	if err != nil {
		t.Fatalf("writeLowFPSDiagnosticsSnapshot: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if string(data) != "diagnostics body" {
		t.Fatalf("snapshot body=%q want diagnostics body", data)
	}
	if !strings.Contains(path, "diagnostics") || !strings.HasSuffix(path, ".txt") {
		t.Fatalf("snapshot path=%q want diagnostics/*.txt", path)
	}
}

func TestBuildPerformanceDiagnosticsReportIncludesKeyFields(t *testing.T) {
	resetThumbDiagnosticsCounters()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if thumbDecodeQueueLen() == 0 && thumbDecodeBusyCount() == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	resetThumbDiagnosticsCounters()
	app := &App{
		renderBackend:             "Vulkan",
		frameFPS:                  12.5,
		frameIntervalEMA:          80 * time.Millisecond,
		renderActive:              true,
		layoutShellEMA:            4 * time.Millisecond,
		layoutControlsEMA:         5 * time.Millisecond,
		layoutSubmitDockEMA:       6 * time.Millisecond,
		layoutActionsEMA:          7 * time.Millisecond,
		layoutPromptCardEMA:       8 * time.Millisecond,
		layoutComposeCardEMA:      9 * time.Millisecond,
		layoutAdvancedCardEMA:     10 * time.Millisecond,
		layoutCanvasEMA:           2 * time.Millisecond,
		layoutCanvasToolbarEMA:    3 * time.Millisecond,
		layoutResultSurfaceEMA:    4 * time.Millisecond,
		layoutCanvasStatusEMA:     5 * time.Millisecond,
		layoutHistoryRailEMA:      1 * time.Millisecond,
		layoutUpstreamCardEMA:     2 * time.Millisecond,
		layoutHistorySummaryEMA:   3 * time.Millisecond,
		layoutLatestHistoryEMA:    4 * time.Millisecond,
		layoutHistoryResultsEMA:   5 * time.Millisecond,
		layoutTimelineModalEMA:    3 * time.Millisecond,
		lastFrameSize:             image.Pt(1600, 900),
		reducedEffects:            true,
		historyTimelineOpen:       true,
		resultGridOpen:            true,
		running:                   true,
		status:                    "1/1 · testing",
		lastProbeSummary:          "models=2",
		lastErrorMessage:          "none",
		lastLowFPSDiagnosticsPath: "/tmp/low-fps.txt",
		logs:                      []string{"10:00:00 first", "10:00:01 second"},
		logsRev:                   1,
		logsSnapshotRev:           -1,
		imageCache:                map[string]cachedImage{"a": {}, "b": {}},
		historyButtons:            map[string]*widget.Clickable{"h": new(widget.Clickable)},
		historyActionButtons:      map[string]*widget.Clickable{"ha": new(widget.Clickable)},
		sourceButtons:             map[string]*widget.Clickable{"s": new(widget.Clickable)},
		promptButtons:             map[string]*widget.Clickable{"p": new(widget.Clickable)},
		workspaceButtons:          map[string]*widget.Clickable{"w": new(widget.Clickable)},
	}
	app.partialImagesInput.SetText("0")
	app.setHistoryLocked([]sharedCompat.HistoryItem{
		{ID: "a", CreatedAt: time.Now().UnixMilli()},
		{ID: "b", CreatedAt: time.Now().UnixMilli()},
	})
	app.imageCache = map[string]cachedImage{"a": {}, "b": {}}
	app.result = resultState{
		SavedPath: "/tmp/result.png",
		Item: sharedCompat.HistoryItem{
			ID:          "result",
			SavedPath:   "/tmp/result.png",
			PreviewPath: "/tmp/result-preview.png",
			ThumbPath:   "/tmp/result-thumb.png",
		},
		HasItem: true,
	}
	resetThumbDiagnosticsCounters()

	report := app.buildPerformanceDiagnosticsReport()
	for _, want := range []string{
		"Image Studio Gio Performance Diagnostics",
		"platform: " + runtime.GOOS + "/" + runtime.GOARCH,
		"backend: Vulkan",
		"render_state: active",
		"fps: 12.5",
		"partial_images: 0",
		"reduced_effects: true",
		"window_px: 1600x900",
		"history_count: 2",
		"history_thumb_paths_present: 0",
		"history_thumb_paths_missing: 2",
		"history_thumb_coverage: 0.0%",
		"history_preview_paths_present: 0",
		"history_preview_paths_missing: 2",
		"history_preview_coverage: 0.0%",
		"history_backfill_inflight: 0",
		"image_cache_entries: 2",
		"thumb_decode_queue: 0",
		"thumb_decode_busy: 0",
		"thumb_decode_queue_peak: 0",
		"thumb_decode_busy_peak: 0",
		"thumb_display_requests: 0",
		"thumb_display_cache_hits: 0",
		"thumb_display_cache_misses: 0",
		"thumb_display_hit_rate: 0.0%",
		"thumb_display_loads_queued: 0",
		"history_thumb_source_preview: 0",
		"history_thumb_source_thumb: 0",
		"history_thumb_source_saved: 0",
		"canvas_display_source_managed_preview: 0",
		"canvas_display_source_path_thumb: 0",
		"canvas_display_source_history_scaled: 0",
		"canvas_display_source_inline: 0",
		"current_result_saved_present: false",
		"current_result_preview_present: false",
		"current_result_thumb_present: false",
		"current_result_canvas_target_px: 1536",
		"current_result_managed_preview_ready: false",
		"history_button_entries: 0",
		"workspace_button_entries: 1",
		"layout_shell_ms: 4.0",
		"layout_controls_ms: 5.0",
		"layout_submit_dock_ms: 6.0",
		"layout_actions_ms: 7.0",
		"layout_prompt_card_ms: 8.0",
		"layout_compose_card_ms: 9.0",
		"layout_advanced_card_ms: 10.0",
		"layout_canvas_ms: 2.0",
		"layout_canvas_toolbar_ms: 3.0",
		"layout_result_surface_ms: 4.0",
		"layout_canvas_status_ms: 5.0",
		"layout_history_rail_ms: 1.0",
		"layout_upstream_card_ms: 2.0",
		"layout_history_summary_ms: 3.0",
		"layout_latest_history_ms: 4.0",
		"layout_history_results_ms: 5.0",
		"layout_history_timeline_ms: 3.0",
		"slowest_layout: advanced_card=10.0ms",
		"slowest_layout_peak: none",
		"last_low_fps_snapshot: /tmp/low-fps.txt",
		"history_timeline_open: true",
		"result_grid_open: true",
		"recent_logs:",
		"10:00:01 second",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q:\n%s", want, report)
		}
	}
}

func TestBuildPerformanceDiagnosticsReportMarksVirtualCurrentResultPresent(t *testing.T) {
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString([]byte("virtual-image")), "remote-result.png", "png")
	app := &App{}
	app.result = resultState{
		SavedPath: virtualPath,
		Item: sharedCompat.HistoryItem{
			ID:        "result-virtual",
			SavedPath: virtualPath,
		},
		HasItem: true,
	}

	report := app.buildPerformanceDiagnosticsReport()
	if !strings.Contains(report, "current_result_saved_present: true") {
		t.Fatalf("report missing virtual saved presence:\n%s", report)
	}
	if !strings.Contains(report, "current_result_managed_preview_ready: false") {
		t.Fatalf("report missing virtual managed preview state:\n%s", report)
	}
}
