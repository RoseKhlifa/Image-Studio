package ui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unsafe"

	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/kernel"

	"gioui.org/app"
	giogpu "gioui.org/gpu"
	"github.com/yuanhua/image-gptcodex/pkg/client"
)

const (
	renderSampleMaxGap        = 250 * time.Millisecond
	renderActiveMaxDelta      = 100 * time.Millisecond
	renderActivityWindow      = 700 * time.Millisecond
	lowFPSMinSamples          = 5
	renderFrameSmoothingAlpha = 0.18
)

type layoutTimingKind uint8

const (
	layoutTimingShell layoutTimingKind = iota
	layoutTimingControls
	layoutTimingSubmitDock
	layoutTimingActions
	layoutTimingPromptCard
	layoutTimingComposeCard
	layoutTimingAdvancedCard
	layoutTimingCanvas
	layoutTimingCanvasToolbar
	layoutTimingResultSurface
	layoutTimingCanvasStatusBar
	layoutTimingHistoryRail
	layoutTimingUpstreamCard
	layoutTimingHistorySummaryCard
	layoutTimingLatestHistoryCard
	layoutTimingHistoryResultsCard
	layoutTimingTimelineModal
	layoutTimingCount
)

func (a *App) recordRenderFrame(now time.Time, size image.Point) {
	shouldInvalidate := false
	a.mu.Lock()

	if a.renderBackend == "" {
		if backend := currentRenderBackend(a.window); backend != "" {
			a.renderBackend = backend
		}
	}
	if now.IsZero() {
		a.mu.Unlock()
		return
	}
	if !a.frameLastAt.IsZero() {
		delta := now.Sub(a.frameLastAt)
		if delta > 0 {
			if delta > renderSampleMaxGap {
				a.resetFrameSamplingLocked()
				a.frameLastAt = now
				if size.X > 0 && size.Y > 0 {
					a.lastFrameSize = size
				}
				a.mu.Unlock()
				return
			}
			a.frameRawIntervalEMA = blendDurationEMA(a.frameRawIntervalEMA, delta, renderFrameSmoothingAlpha)
			if a.frameRawIntervalEMA > 0 {
				a.frameRawFPS = float64(time.Second) / float64(a.frameRawIntervalEMA)
			}
			if a.isActiveRenderSampleLocked(now, delta) {
				a.frameIntervalEMA = blendDurationEMA(a.frameIntervalEMA, delta, renderFrameSmoothingAlpha)
				if a.frameIntervalEMA > 0 {
					a.frameFPS = float64(time.Second) / float64(a.frameIntervalEMA)
				}
				a.renderActive = a.frameFPS > 0
			} else {
				a.renderActive = false
				a.frameIntervalEMA = 0
				a.frameFPS = 0
			}
		}
	}
	a.frameLastAt = now
	if size.X > 0 && size.Y > 0 {
		a.lastFrameSize = size
	}
	shouldInvalidate = a.maybeRecordLowFPSLocked(now)
	a.mu.Unlock()
	if shouldInvalidate {
		a.captureLowFPSDiagnosticsSnapshot()
		a.invalidateSoon(33 * time.Millisecond)
	}
}

func formatRenderDiagnostics(snap snapshot) string {
	parts := make([]string, 0, 3)
	if backend := strings.TrimSpace(snap.RenderBackend); backend != "" {
		parts = append(parts, "GPU "+backend)
	}
	if snap.RenderActive {
		if snap.RenderFPS > 0 {
			parts = append(parts, fmt.Sprintf("%.1f FPS", snap.RenderFPS))
		}
		if snap.RenderFrameTime > 0 {
			parts = append(parts, fmt.Sprintf("%.1fms", float64(snap.RenderFrameTime)/float64(time.Millisecond)))
		}
	} else if len(parts) > 0 {
		parts = append(parts, "空闲")
	}
	return strings.Join(parts, " · ")
}

func blendDurationEMA(current time.Duration, sample time.Duration, alpha float64) time.Duration {
	if sample <= 0 {
		return current
	}
	if current <= 0 {
		return sample
	}
	return time.Duration((1-alpha)*float64(current) + alpha*float64(sample))
}

func (a *App) recordLayoutTiming(kind layoutTimingKind, started time.Time) {
	if started.IsZero() {
		return
	}
	duration := time.Since(started)
	if duration <= 0 {
		return
	}
	a.mu.Lock()
	switch kind {
	case layoutTimingShell:
		a.layoutShellEMA = blendDurationEMA(a.layoutShellEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingControls:
		a.layoutControlsEMA = blendDurationEMA(a.layoutControlsEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingSubmitDock:
		a.layoutSubmitDockEMA = blendDurationEMA(a.layoutSubmitDockEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingActions:
		a.layoutActionsEMA = blendDurationEMA(a.layoutActionsEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingPromptCard:
		a.layoutPromptCardEMA = blendDurationEMA(a.layoutPromptCardEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingComposeCard:
		a.layoutComposeCardEMA = blendDurationEMA(a.layoutComposeCardEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingAdvancedCard:
		a.layoutAdvancedCardEMA = blendDurationEMA(a.layoutAdvancedCardEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingCanvas:
		a.layoutCanvasEMA = blendDurationEMA(a.layoutCanvasEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingCanvasToolbar:
		a.layoutCanvasToolbarEMA = blendDurationEMA(a.layoutCanvasToolbarEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingResultSurface:
		a.layoutResultSurfaceEMA = blendDurationEMA(a.layoutResultSurfaceEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingCanvasStatusBar:
		a.layoutCanvasStatusEMA = blendDurationEMA(a.layoutCanvasStatusEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingHistoryRail:
		a.layoutHistoryRailEMA = blendDurationEMA(a.layoutHistoryRailEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingUpstreamCard:
		a.layoutUpstreamCardEMA = blendDurationEMA(a.layoutUpstreamCardEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingHistorySummaryCard:
		a.layoutHistorySummaryEMA = blendDurationEMA(a.layoutHistorySummaryEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingLatestHistoryCard:
		a.layoutLatestHistoryEMA = blendDurationEMA(a.layoutLatestHistoryEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingHistoryResultsCard:
		a.layoutHistoryResultsEMA = blendDurationEMA(a.layoutHistoryResultsEMA, duration, renderFrameSmoothingAlpha)
	case layoutTimingTimelineModal:
		a.layoutTimelineModalEMA = blendDurationEMA(a.layoutTimelineModalEMA, duration, renderFrameSmoothingAlpha)
	}
	if kind < layoutTimingCount && duration > a.layoutPeaks[kind] {
		a.layoutPeaks[kind] = duration
	}
	a.mu.Unlock()
}

func formatLayoutTimingValue(duration time.Duration, visible bool) string {
	if !visible {
		return "hidden"
	}
	return fmt.Sprintf("%.1f", float64(duration)/float64(time.Millisecond))
}

func currentResultDiagnosticsState(result resultState, canvasTarget int) (savedPresent bool, previewPresent bool, thumbPresent bool, managedPreviewReady bool, managedPreviewPath string) {
	savedPath := strings.TrimSpace(result.SavedPath)
	if savedPath == "" {
		savedPath = strings.TrimSpace(result.Item.SavedPath)
	}
	previewPath := strings.TrimSpace(result.Item.PreviewPath)
	thumbPath := strings.TrimSpace(result.Item.ThumbPath)
	savedPresent = headlessPathReady(savedPath)
	previewPresent = headlessPathReady(previewPath)
	thumbPresent = headlessPathReady(thumbPath)
	if savedPath == "" || isVirtualImagePath(savedPath) {
		return savedPresent, previewPresent, thumbPresent, false, ""
	}
	path, err := managedSourcePreviewPath(savedPath, canvasTarget)
	if err != nil {
		return savedPresent, previewPresent, thumbPresent, false, ""
	}
	return savedPresent, previewPresent, thumbPresent, headlessPathReady(path), path
}

type layoutTimingSample struct {
	name     string
	duration time.Duration
	visible  bool
}

func slowestLayoutSample(samples []layoutTimingSample) string {
	var best layoutTimingSample
	found := false
	for _, sample := range samples {
		if !sample.visible || sample.duration <= 0 {
			continue
		}
		if !found || sample.duration > best.duration {
			best = sample
			found = true
		}
	}
	if !found {
		return "none"
	}
	return fmt.Sprintf("%s=%.1fms", best.name, float64(best.duration)/float64(time.Millisecond))
}

func (a *App) noteRenderActivity() {
	a.mu.Lock()
	a.noteRenderActivityLocked(time.Now())
	a.mu.Unlock()
}

func (a *App) noteRenderActivityLocked(now time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	if a.lastRenderActivityAt.IsZero() || now.After(a.lastRenderActivityAt) {
		a.lastRenderActivityAt = now
	}
}

func (a *App) hasRecentRenderActivityLocked(now time.Time) bool {
	if a.running || a.processingImageTransform {
		return true
	}
	if a.lastRenderActivityAt.IsZero() {
		return false
	}
	return now.Sub(a.lastRenderActivityAt) < renderActivityWindow
}

func (a *App) isActiveRenderSampleLocked(now time.Time, delta time.Duration) bool {
	if delta > 0 && delta <= renderActiveMaxDelta {
		return true
	}
	if a.frameRawIntervalEMA > 0 && a.frameRawIntervalEMA <= renderActiveMaxDelta {
		return true
	}
	return a.hasRecentRenderActivityLocked(now)
}

func (a *App) resetFrameSamplingLocked() {
	a.lowFPSStreak = 0
	a.frameRawIntervalEMA = 0
	a.frameRawFPS = 0
	a.frameIntervalEMA = 0
	a.frameFPS = 0
	a.renderActive = false
}

func currentRenderBackend(w *app.Window) string {
	if w == nil {
		return ""
	}
	ctxValue, ok := readUnexportedField(w, "ctx")
	if !ok || ctxValue == nil {
		return ""
	}
	return renderBackendFromContext(ctxValue)
}

func renderBackendFromContext(ctxValue any) string {
	apiProvider, ok := ctxValue.(interface{ API() giogpu.API })
	if !ok {
		return ""
	}
	return describeRenderAPI(apiProvider.API())
}

func describeRenderAPI(api giogpu.API) string {
	switch api.(type) {
	case giogpu.Direct3D11:
		return "Direct3D 11"
	case giogpu.Vulkan:
		return "Vulkan"
	case giogpu.Metal:
		return "Metal"
	case giogpu.OpenGL:
		return "OpenGL"
	default:
		return ""
	}
}

func (a *App) buildPerformanceDiagnosticsReport() string {
	snap := a.readSnapshot()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	backend := strings.TrimSpace(snap.RenderBackend)
	if backend == "" {
		backend = "unknown"
	}
	renderState := "idle"
	if snap.RenderActive {
		renderState = "active"
	}
	partialImages := strings.TrimSpace(a.partialImagesInput.Text())
	if partialImages == "" {
		partialImages = strconv.Itoa(kernel.DefaultConfig().PartialImages)
	}
	a.mu.Lock()
	imageCacheEntries := len(a.imageCache)
	historyButtonEntries := len(a.historyButtons)
	historyActionEntries := len(a.historyActionButtons)
	sourceButtonEntries := len(a.sourceButtons)
	promptButtonEntries := len(a.promptButtons)
	workspaceButtonEntries := len(a.workspaceButtons)
	layoutShellEMA := a.layoutShellEMA
	layoutControlsEMA := a.layoutControlsEMA
	layoutSubmitDockEMA := a.layoutSubmitDockEMA
	layoutActionsEMA := a.layoutActionsEMA
	layoutPromptCardEMA := a.layoutPromptCardEMA
	layoutComposeCardEMA := a.layoutComposeCardEMA
	layoutAdvancedCardEMA := a.layoutAdvancedCardEMA
	layoutCanvasEMA := a.layoutCanvasEMA
	layoutCanvasToolbarEMA := a.layoutCanvasToolbarEMA
	layoutResultSurfaceEMA := a.layoutResultSurfaceEMA
	layoutCanvasStatusEMA := a.layoutCanvasStatusEMA
	layoutHistoryRailEMA := a.layoutHistoryRailEMA
	layoutUpstreamCardEMA := a.layoutUpstreamCardEMA
	layoutHistorySummaryEMA := a.layoutHistorySummaryEMA
	layoutLatestHistoryEMA := a.layoutLatestHistoryEMA
	layoutHistoryResultsEMA := a.layoutHistoryResultsEMA
	layoutTimelineModalEMA := a.layoutTimelineModalEMA
	layoutPeaks := a.layoutPeaks
	lastLowFPSDiagnosticsPath := strings.TrimSpace(a.lastLowFPSDiagnosticsPath)
	lastHistoryThumbPrewarmAt := a.lastHistoryThumbPrewarmAt
	lastHistoryThumbPrewarmMs := a.lastHistoryThumbPrewarmMs
	lastHistoryThumbPrewarmLoad := a.lastHistoryThumbPrewarmLoad
	lastHistoryThumbPrewarmFail := a.lastHistoryThumbPrewarmFail
	historyBackfillInFlight := len(a.historyThumbBackfillInFlight)
	controlsVisible := !a.fullscreen
	submitDockVisible := !a.fullscreen
	actionsVisible := !a.fullscreen
	controlCardsVisible := !a.fullscreen
	historyRailVisible := !a.fullscreen
	timelineVisible := a.historyTimelineOpen
	a.mu.Unlock()
	thumbDecodeQueueLen := thumbDecodeQueueLen()
	thumbDecodeBusyCount := thumbDecodeBusyCount()
	thumbDecodeQueuePeak := thumbDecodeQueuePeakCount()
	thumbDecodeBusyPeak := thumbDecodeBusyPeakCount()
	thumbRequests := thumbDisplayRequestCount()
	thumbHits := thumbDisplayCacheHitCount()
	thumbLoadsQueued := thumbDisplayLoadQueuedCount()
	historyThumbPreviewHits := historyThumbSourcePreviewCount()
	historyThumbThumbHits := historyThumbSourceThumbCount()
	historyThumbSavedHits := historyThumbSourceSavedCount()
	canvasManagedPreviewHits := canvasDisplaySourceManagedPreviewCount()
	canvasPathThumbHits := canvasDisplaySourcePathThumbCount()
	canvasHistoryScaledHits := canvasDisplaySourceHistoryScaledCount()
	canvasInlineHits := canvasDisplaySourceInlineCount()
	thumbMisses := max(0, int(thumbRequests-thumbHits))
	thumbHitRate := 0.0
	if thumbRequests > 0 {
		thumbHitRate = float64(thumbHits) * 100 / float64(thumbRequests)
	}
	historyThumbPathsPresent := 0
	historyPreviewPathsPresent := 0
	for _, item := range snap.History {
		if strings.TrimSpace(item.ThumbPath) != "" {
			historyThumbPathsPresent++
		}
		if strings.TrimSpace(item.PreviewPath) != "" {
			historyPreviewPathsPresent++
		}
	}
	historyThumbPathsMissing := max(0, len(snap.History)-historyThumbPathsPresent)
	historyPreviewPathsMissing := max(0, len(snap.History)-historyPreviewPathsPresent)
	historyThumbCoverage := 0.0
	historyPreviewCoverage := 0.0
	if len(snap.History) > 0 {
		historyThumbCoverage = float64(historyThumbPathsPresent) * 100 / float64(len(snap.History))
		historyPreviewCoverage = float64(historyPreviewPathsPresent) * 100 / float64(len(snap.History))
	}
	currentResultSavedPresent := false
	currentResultPreviewPresent := false
	currentResultThumbPresent := false
	currentResultManagedPreviewReady := false
	currentResultCanvasTarget := a.effectiveCanvasMaxDimension()
	currentResultManagedPreviewPath := ""
	currentResultSavedPresent, currentResultPreviewPresent, currentResultThumbPresent, currentResultManagedPreviewReady, currentResultManagedPreviewPath =
		currentResultDiagnosticsState(snap.Result, currentResultCanvasTarget)
	slowestLayout := slowestLayoutSample([]layoutTimingSample{
		{name: "shell", duration: layoutShellEMA, visible: true},
		{name: "controls", duration: layoutControlsEMA, visible: controlsVisible},
		{name: "submit_dock", duration: layoutSubmitDockEMA, visible: submitDockVisible},
		{name: "actions", duration: layoutActionsEMA, visible: actionsVisible},
		{name: "prompt_card", duration: layoutPromptCardEMA, visible: controlCardsVisible},
		{name: "compose_card", duration: layoutComposeCardEMA, visible: controlCardsVisible},
		{name: "advanced_card", duration: layoutAdvancedCardEMA, visible: controlCardsVisible},
		{name: "canvas", duration: layoutCanvasEMA, visible: true},
		{name: "canvas_toolbar", duration: layoutCanvasToolbarEMA, visible: true},
		{name: "result_surface", duration: layoutResultSurfaceEMA, visible: true},
		{name: "canvas_status", duration: layoutCanvasStatusEMA, visible: true},
		{name: "history_rail", duration: layoutHistoryRailEMA, visible: historyRailVisible},
		{name: "upstream_card", duration: layoutUpstreamCardEMA, visible: historyRailVisible},
		{name: "history_summary", duration: layoutHistorySummaryEMA, visible: historyRailVisible},
		{name: "latest_history", duration: layoutLatestHistoryEMA, visible: historyRailVisible},
		{name: "history_results", duration: layoutHistoryResultsEMA, visible: historyRailVisible},
		{name: "history_timeline", duration: layoutTimelineModalEMA, visible: timelineVisible},
	})
	slowestLayoutPeak := slowestLayoutSample([]layoutTimingSample{
		{name: "shell", duration: layoutPeaks[layoutTimingShell], visible: true},
		{name: "controls", duration: layoutPeaks[layoutTimingControls], visible: controlsVisible},
		{name: "submit_dock", duration: layoutPeaks[layoutTimingSubmitDock], visible: submitDockVisible},
		{name: "actions", duration: layoutPeaks[layoutTimingActions], visible: actionsVisible},
		{name: "prompt_card", duration: layoutPeaks[layoutTimingPromptCard], visible: controlCardsVisible},
		{name: "compose_card", duration: layoutPeaks[layoutTimingComposeCard], visible: controlCardsVisible},
		{name: "advanced_card", duration: layoutPeaks[layoutTimingAdvancedCard], visible: controlCardsVisible},
		{name: "canvas", duration: layoutPeaks[layoutTimingCanvas], visible: true},
		{name: "canvas_toolbar", duration: layoutPeaks[layoutTimingCanvasToolbar], visible: true},
		{name: "result_surface", duration: layoutPeaks[layoutTimingResultSurface], visible: true},
		{name: "canvas_status", duration: layoutPeaks[layoutTimingCanvasStatusBar], visible: true},
		{name: "history_rail", duration: layoutPeaks[layoutTimingHistoryRail], visible: historyRailVisible},
		{name: "upstream_card", duration: layoutPeaks[layoutTimingUpstreamCard], visible: historyRailVisible},
		{name: "history_summary", duration: layoutPeaks[layoutTimingHistorySummaryCard], visible: historyRailVisible},
		{name: "latest_history", duration: layoutPeaks[layoutTimingLatestHistoryCard], visible: historyRailVisible},
		{name: "history_results", duration: layoutPeaks[layoutTimingHistoryResultsCard], visible: historyRailVisible},
		{name: "history_timeline", duration: layoutPeaks[layoutTimingTimelineModal], visible: timelineVisible},
	})

	lines := []string{
		"Image Studio Gio Performance Diagnostics",
		"version: " + client.Version,
		"timestamp: " + time.Now().Format(time.RFC3339),
		"platform: " + runtime.GOOS + "/" + runtime.GOARCH,
		"backend: " + backend,
		"render_state: " + renderState,
		fmt.Sprintf("fps: %.1f", snap.RenderFPS),
		fmt.Sprintf("frame_ms: %.1f", float64(snap.RenderFrameTime)/float64(time.Millisecond)),
		fmt.Sprintf("status: %s", strings.TrimSpace(snap.Status)),
		fmt.Sprintf("running: %t", snap.Running),
		fmt.Sprintf("fullscreen: %t", snap.Fullscreen),
		fmt.Sprintf("history_timeline_open: %t", snap.HistoryTimelineOpen),
		fmt.Sprintf("result_grid_open: %t", snap.ResultGridOpen),
		fmt.Sprintf("reduced_effects: %t", a.reducedEffects),
		fmt.Sprintf("history_count: %d", len(snap.History)),
		fmt.Sprintf("today_history_count: %d", snap.TodayHistoryCount),
		fmt.Sprintf("history_thumb_paths_present: %d", historyThumbPathsPresent),
		fmt.Sprintf("history_thumb_paths_missing: %d", historyThumbPathsMissing),
		fmt.Sprintf("history_thumb_coverage: %.1f%%", historyThumbCoverage),
		fmt.Sprintf("history_preview_paths_present: %d", historyPreviewPathsPresent),
		fmt.Sprintf("history_preview_paths_missing: %d", historyPreviewPathsMissing),
		fmt.Sprintf("history_preview_coverage: %.1f%%", historyPreviewCoverage),
		fmt.Sprintf("history_backfill_inflight: %d", historyBackfillInFlight),
		fmt.Sprintf("history_thumb_prewarm_loaded: %d", lastHistoryThumbPrewarmLoad),
		fmt.Sprintf("history_thumb_prewarm_failed: %d", lastHistoryThumbPrewarmFail),
		fmt.Sprintf("history_thumb_prewarm_ms: %.1f", float64(lastHistoryThumbPrewarmMs)/float64(time.Millisecond)),
		fmt.Sprintf("partial_images: %s", partialImages),
		fmt.Sprintf("image_cache_entries: %d", imageCacheEntries),
		fmt.Sprintf("thumb_decode_queue: %d", thumbDecodeQueueLen),
		fmt.Sprintf("thumb_decode_busy: %d", thumbDecodeBusyCount),
		fmt.Sprintf("thumb_decode_queue_peak: %d", thumbDecodeQueuePeak),
		fmt.Sprintf("thumb_decode_busy_peak: %d", thumbDecodeBusyPeak),
		fmt.Sprintf("thumb_display_requests: %d", thumbRequests),
		fmt.Sprintf("thumb_display_cache_hits: %d", thumbHits),
		fmt.Sprintf("thumb_display_cache_misses: %d", thumbMisses),
		fmt.Sprintf("thumb_display_hit_rate: %.1f%%", thumbHitRate),
		fmt.Sprintf("thumb_display_loads_queued: %d", thumbLoadsQueued),
		fmt.Sprintf("history_thumb_source_preview: %d", historyThumbPreviewHits),
		fmt.Sprintf("history_thumb_source_thumb: %d", historyThumbThumbHits),
		fmt.Sprintf("history_thumb_source_saved: %d", historyThumbSavedHits),
		fmt.Sprintf("canvas_display_source_managed_preview: %d", canvasManagedPreviewHits),
		fmt.Sprintf("canvas_display_source_path_thumb: %d", canvasPathThumbHits),
		fmt.Sprintf("canvas_display_source_history_scaled: %d", canvasHistoryScaledHits),
		fmt.Sprintf("canvas_display_source_inline: %d", canvasInlineHits),
		fmt.Sprintf("current_result_saved_present: %t", currentResultSavedPresent),
		fmt.Sprintf("current_result_preview_present: %t", currentResultPreviewPresent),
		fmt.Sprintf("current_result_thumb_present: %t", currentResultThumbPresent),
		fmt.Sprintf("current_result_canvas_target_px: %d", currentResultCanvasTarget),
		fmt.Sprintf("current_result_managed_preview_ready: %t", currentResultManagedPreviewReady),
		fmt.Sprintf("history_button_entries: %d", historyButtonEntries),
		fmt.Sprintf("history_action_entries: %d", historyActionEntries),
		fmt.Sprintf("source_button_entries: %d", sourceButtonEntries),
		fmt.Sprintf("prompt_button_entries: %d", promptButtonEntries),
		fmt.Sprintf("workspace_button_entries: %d", workspaceButtonEntries),
		"layout_shell_ms: " + formatLayoutTimingValue(layoutShellEMA, true),
		"layout_controls_ms: " + formatLayoutTimingValue(layoutControlsEMA, controlsVisible),
		"layout_submit_dock_ms: " + formatLayoutTimingValue(layoutSubmitDockEMA, submitDockVisible),
		"layout_actions_ms: " + formatLayoutTimingValue(layoutActionsEMA, actionsVisible),
		"layout_prompt_card_ms: " + formatLayoutTimingValue(layoutPromptCardEMA, controlCardsVisible),
		"layout_compose_card_ms: " + formatLayoutTimingValue(layoutComposeCardEMA, controlCardsVisible),
		"layout_advanced_card_ms: " + formatLayoutTimingValue(layoutAdvancedCardEMA, controlCardsVisible),
		"layout_canvas_ms: " + formatLayoutTimingValue(layoutCanvasEMA, true),
		"layout_canvas_toolbar_ms: " + formatLayoutTimingValue(layoutCanvasToolbarEMA, true),
		"layout_result_surface_ms: " + formatLayoutTimingValue(layoutResultSurfaceEMA, true),
		"layout_canvas_status_ms: " + formatLayoutTimingValue(layoutCanvasStatusEMA, true),
		"layout_history_rail_ms: " + formatLayoutTimingValue(layoutHistoryRailEMA, historyRailVisible),
		"layout_upstream_card_ms: " + formatLayoutTimingValue(layoutUpstreamCardEMA, historyRailVisible),
		"layout_history_summary_ms: " + formatLayoutTimingValue(layoutHistorySummaryEMA, historyRailVisible),
		"layout_latest_history_ms: " + formatLayoutTimingValue(layoutLatestHistoryEMA, historyRailVisible),
		"layout_history_results_ms: " + formatLayoutTimingValue(layoutHistoryResultsEMA, historyRailVisible),
		"layout_history_timeline_ms: " + formatLayoutTimingValue(layoutTimelineModalEMA, timelineVisible),
		"slowest_layout: " + slowestLayout,
		"slowest_layout_peak: " + slowestLayoutPeak,
		fmt.Sprintf("window_px: %dx%d", a.lastFrameSize.X, a.lastFrameSize.Y),
		fmt.Sprintf("goroutines: %d", runtime.NumGoroutine()),
		fmt.Sprintf("alloc_mb: %.1f", float64(mem.Alloc)/(1024*1024)),
	}
	if msg := strings.TrimSpace(snap.LastErrorMessage); msg != "" {
		lines = append(lines, "last_error: "+msg)
	}
	if summary := strings.TrimSpace(snap.LastProbeSummary); summary != "" {
		lines = append(lines, "last_probe: "+summary)
	}
	if currentResultManagedPreviewPath != "" {
		lines = append(lines, "current_result_managed_preview_path: "+currentResultManagedPreviewPath)
	}
	if !lastHistoryThumbPrewarmAt.IsZero() {
		lines = append(lines, "last_history_thumb_prewarm_at: "+lastHistoryThumbPrewarmAt.Format(time.RFC3339))
	}
	if lastLowFPSDiagnosticsPath != "" {
		lines = append(lines, "last_low_fps_snapshot: "+lastLowFPSDiagnosticsPath)
	}
	if len(snap.Logs) > 0 {
		lines = append(lines, "", "recent_logs:")
		start := max(0, len(snap.Logs)-8)
		for _, line := range snap.Logs[start:] {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func (a *App) maybeRecordLowFPSLocked(now time.Time) bool {
	frameFPS := a.frameRawFPS
	frameInterval := a.frameRawIntervalEMA
	if frameFPS <= 0 || frameInterval <= 0 {
		frameFPS = a.frameFPS
		frameInterval = a.frameIntervalEMA
	}
	if frameFPS <= 0 || frameInterval < 80*time.Millisecond || !a.hasRecentRenderActivityLocked(now) {
		a.lowFPSStreak = 0
		return false
	}
	a.lowFPSStreak++
	if a.lowFPSStreak < lowFPSMinSamples {
		return false
	}
	if !a.lowFPSLastLoggedAt.IsZero() && now.Sub(a.lowFPSLastLoggedAt) < 10*time.Second {
		return false
	}
	backend := strings.TrimSpace(a.renderBackend)
	if backend == "" {
		backend = "unknown"
	}
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	thumbQueueLen := thumbDecodeQueueLen()
	thumbBusyCount := thumbDecodeBusyCount()
	thumbQueuePeak := thumbDecodeQueuePeakCount()
	thumbBusyPeak := thumbDecodeBusyPeakCount()
	thumbRequests := thumbDisplayRequestCount()
	thumbHits := thumbDisplayCacheHitCount()
	thumbLoadsQueued := thumbDisplayLoadQueuedCount()
	historyThumbPreviewHits := historyThumbSourcePreviewCount()
	historyThumbThumbHits := historyThumbSourceThumbCount()
	historyThumbSavedHits := historyThumbSourceSavedCount()
	canvasManagedPreviewHits := canvasDisplaySourceManagedPreviewCount()
	canvasPathThumbHits := canvasDisplaySourcePathThumbCount()
	canvasHistoryScaledHits := canvasDisplaySourceHistoryScaledCount()
	canvasInlineHits := canvasDisplaySourceInlineCount()
	thumbMisses := max(0, int(thumbRequests-thumbHits))
	thumbHitRate := 0.0
	if thumbRequests > 0 {
		thumbHitRate = float64(thumbHits) * 100 / float64(thumbRequests)
	}
	historyThumbPathsPresent := 0
	historyPreviewPathsPresent := 0
	for _, item := range a.history {
		if strings.TrimSpace(item.ThumbPath) != "" {
			historyThumbPathsPresent++
		}
		if strings.TrimSpace(item.PreviewPath) != "" {
			historyPreviewPathsPresent++
		}
	}
	historyThumbPathsMissing := max(0, len(a.history)-historyThumbPathsPresent)
	historyPreviewPathsMissing := max(0, len(a.history)-historyPreviewPathsPresent)
	historyThumbCoverage := 0.0
	historyPreviewCoverage := 0.0
	if len(a.history) > 0 {
		historyThumbCoverage = float64(historyThumbPathsPresent) * 100 / float64(len(a.history))
		historyPreviewCoverage = float64(historyPreviewPathsPresent) * 100 / float64(len(a.history))
	}
	historyBackfillInFlight := len(a.historyThumbBackfillInFlight)
	currentResultSavedPresent := false
	currentResultPreviewPresent := false
	currentResultThumbPresent := false
	currentResultManagedPreviewReady := false
	currentResultCanvasTarget := a.effectiveCanvasMaxDimension()
	currentResultManagedPreviewPath := ""
	currentResultSavedPresent, currentResultPreviewPresent, currentResultThumbPresent, currentResultManagedPreviewReady, currentResultManagedPreviewPath =
		currentResultDiagnosticsState(a.result, currentResultCanvasTarget)
	lastHistoryThumbPrewarmAt := a.lastHistoryThumbPrewarmAt
	lastHistoryThumbPrewarmMs := a.lastHistoryThumbPrewarmMs
	lastHistoryThumbPrewarmLoad := a.lastHistoryThumbPrewarmLoad
	lastHistoryThumbPrewarmFail := a.lastHistoryThumbPrewarmFail
	layoutShell := formatLayoutTimingValue(a.layoutShellEMA, true)
	layoutControls := formatLayoutTimingValue(a.layoutControlsEMA, !a.fullscreen)
	layoutSubmitDock := formatLayoutTimingValue(a.layoutSubmitDockEMA, !a.fullscreen)
	layoutActions := formatLayoutTimingValue(a.layoutActionsEMA, !a.fullscreen)
	layoutPromptCard := formatLayoutTimingValue(a.layoutPromptCardEMA, !a.fullscreen)
	layoutComposeCard := formatLayoutTimingValue(a.layoutComposeCardEMA, !a.fullscreen)
	layoutAdvancedCard := formatLayoutTimingValue(a.layoutAdvancedCardEMA, !a.fullscreen)
	layoutCanvas := formatLayoutTimingValue(a.layoutCanvasEMA, true)
	layoutCanvasToolbar := formatLayoutTimingValue(a.layoutCanvasToolbarEMA, true)
	layoutResultSurface := formatLayoutTimingValue(a.layoutResultSurfaceEMA, true)
	layoutCanvasStatus := formatLayoutTimingValue(a.layoutCanvasStatusEMA, true)
	layoutHistoryRail := formatLayoutTimingValue(a.layoutHistoryRailEMA, !a.fullscreen)
	layoutUpstreamCard := formatLayoutTimingValue(a.layoutUpstreamCardEMA, !a.fullscreen)
	layoutHistorySummary := formatLayoutTimingValue(a.layoutHistorySummaryEMA, !a.fullscreen)
	layoutLatestHistory := formatLayoutTimingValue(a.layoutLatestHistoryEMA, !a.fullscreen)
	layoutHistoryResults := formatLayoutTimingValue(a.layoutHistoryResultsEMA, !a.fullscreen)
	layoutTimeline := formatLayoutTimingValue(a.layoutTimelineModalEMA, a.historyTimelineOpen)
	slowestLayout := slowestLayoutSample([]layoutTimingSample{
		{name: "shell", duration: a.layoutShellEMA, visible: true},
		{name: "controls", duration: a.layoutControlsEMA, visible: !a.fullscreen},
		{name: "submit_dock", duration: a.layoutSubmitDockEMA, visible: !a.fullscreen},
		{name: "actions", duration: a.layoutActionsEMA, visible: !a.fullscreen},
		{name: "prompt_card", duration: a.layoutPromptCardEMA, visible: !a.fullscreen},
		{name: "compose_card", duration: a.layoutComposeCardEMA, visible: !a.fullscreen},
		{name: "advanced_card", duration: a.layoutAdvancedCardEMA, visible: !a.fullscreen},
		{name: "canvas", duration: a.layoutCanvasEMA, visible: true},
		{name: "canvas_toolbar", duration: a.layoutCanvasToolbarEMA, visible: true},
		{name: "result_surface", duration: a.layoutResultSurfaceEMA, visible: true},
		{name: "canvas_status", duration: a.layoutCanvasStatusEMA, visible: true},
		{name: "history_rail", duration: a.layoutHistoryRailEMA, visible: !a.fullscreen},
		{name: "upstream_card", duration: a.layoutUpstreamCardEMA, visible: !a.fullscreen},
		{name: "history_summary", duration: a.layoutHistorySummaryEMA, visible: !a.fullscreen},
		{name: "latest_history", duration: a.layoutLatestHistoryEMA, visible: !a.fullscreen},
		{name: "history_results", duration: a.layoutHistoryResultsEMA, visible: !a.fullscreen},
		{name: "history_timeline", duration: a.layoutTimelineModalEMA, visible: a.historyTimelineOpen},
	})
	message := fmt.Sprintf(
		"UI 帧率偏低: %s · %.1f FPS · %.1fms · history=%d · running=%t · fullscreen=%t · timeline=%t · grid=%t · layout_ms shell=%s controls=%s submit=%s actions=%s prompt=%s compose=%s advanced=%s canvas=%s canvas_toolbar=%s result_surface=%s canvas_status=%s rail=%s upstream=%s summary=%s latest=%s results=%s timeline=%s · slowest=%s · history_thumb=%d/%d(%.1f%%) · history_preview=%d/%d(%.1f%%) · history_backfill=%d · history_prewarm=%d/%d(%.1fms,%s) · thumb_src=%d/%d/%d · thumb_queue=%d/%d · thumb_busy=%d/%d · thumb_req=%d · thumb_hit=%d · thumb_miss=%d · thumb_hit_rate=%.1f%% · thumb_load=%d · canvas_src=%d/%d/%d/%d · current_result=%t/%t/%t target=%d managed=%t · goroutines=%d · alloc=%.1fMB",
		backend,
		frameFPS,
		float64(frameInterval)/float64(time.Millisecond),
		len(a.history),
		a.running,
		a.fullscreen,
		a.historyTimelineOpen,
		a.resultGridOpen,
		layoutShell,
		layoutControls,
		layoutSubmitDock,
		layoutActions,
		layoutPromptCard,
		layoutComposeCard,
		layoutAdvancedCard,
		layoutCanvas,
		layoutCanvasToolbar,
		layoutResultSurface,
		layoutCanvasStatus,
		layoutHistoryRail,
		layoutUpstreamCard,
		layoutHistorySummary,
		layoutLatestHistory,
		layoutHistoryResults,
		layoutTimeline,
		slowestLayout,
		historyThumbPathsPresent,
		historyThumbPathsMissing,
		historyThumbCoverage,
		historyPreviewPathsPresent,
		historyPreviewPathsMissing,
		historyPreviewCoverage,
		historyBackfillInFlight,
		lastHistoryThumbPrewarmLoad,
		lastHistoryThumbPrewarmFail,
		float64(lastHistoryThumbPrewarmMs)/float64(time.Millisecond),
		formatOptionalLogTime(lastHistoryThumbPrewarmAt),
		historyThumbPreviewHits,
		historyThumbThumbHits,
		historyThumbSavedHits,
		thumbQueueLen,
		thumbQueuePeak,
		thumbBusyCount,
		thumbBusyPeak,
		thumbRequests,
		thumbHits,
		thumbMisses,
		thumbHitRate,
		thumbLoadsQueued,
		canvasManagedPreviewHits,
		canvasPathThumbHits,
		canvasHistoryScaledHits,
		canvasInlineHits,
		currentResultSavedPresent,
		currentResultPreviewPresent,
		currentResultThumbPresent,
		currentResultCanvasTarget,
		currentResultManagedPreviewReady,
		runtime.NumGoroutine(),
		float64(mem.Alloc)/(1024*1024),
	)
	if currentResultManagedPreviewPath != "" {
		message += " · current_result_managed_path=" + currentResultManagedPreviewPath
	}
	if !a.reducedEffects {
		message += " · 建议切换低特效模式"
	}
	a.appendLogLocked(message)
	a.lowFPSLastLoggedAt = now
	return true
}

func formatOptionalLogTime(t time.Time) string {
	if t.IsZero() {
		return "none"
	}
	return t.Format("15:04:05")
}

func (a *App) captureLowFPSDiagnosticsSnapshot() {
	a.mu.Lock()
	if a.lowFPSSnapshotInFlight {
		a.mu.Unlock()
		return
	}
	a.lowFPSSnapshotInFlight = true
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.lowFPSSnapshotInFlight = false
			a.mu.Unlock()
		}()
		report := a.buildPerformanceDiagnosticsReport()
		path, err := writeLowFPSDiagnosticsSnapshot(report, time.Now())
		if err != nil {
			return
		}
		a.mu.Lock()
		a.lastLowFPSDiagnosticsPath = path
		a.mu.Unlock()
		a.appendLog("已保存低帧率诊断: " + path)
	}()
}

func writeLowFPSDiagnosticsSnapshot(report string, now time.Time) (string, error) {
	dir, err := diagnosticsDirPath()
	if err != nil {
		return "", err
	}
	if now.IsZero() {
		now = time.Now()
	}
	name := "low-fps-" + now.Format("20060102-150405.000") + ".txt"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(report), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func diagnosticsDirPath() (string, error) {
	root, err := gioCompat.StableDataRoot()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, "diagnostics")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Gio keeps its window context private, so runtime backend inspection has to read the field directly.
func readUnexportedField(target any, name string) (any, bool) {
	value := reflect.ValueOf(target)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return nil, false
	}
	elem := value.Elem()
	if !elem.IsValid() || elem.Kind() != reflect.Struct {
		return nil, false
	}
	field := elem.FieldByName(name)
	if !field.IsValid() || !field.CanAddr() {
		return nil, false
	}
	fieldValue := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	return fieldValue.Interface(), true
}
