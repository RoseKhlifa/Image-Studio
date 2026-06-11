package ui

import (
	"os"
	"strings"
	"time"

	sharedCompat "image-studio/shared/compat"
)

type HeadlessHistoryPerfReport struct {
	HistoryCount                        int     `json:"history_count"`
	HistoryPanelFiltered                int     `json:"history_panel_filtered"`
	HistoryPanelColdMs                  float64 `json:"history_panel_cold_ms"`
	HistoryPanelCachedMs                float64 `json:"history_panel_cached_ms"`
	HistoryTimelineFiltered             int     `json:"history_timeline_filtered"`
	HistoryTimelineColdMs               float64 `json:"history_timeline_cold_ms"`
	HistoryTimelineCachedMs             float64 `json:"history_timeline_cached_ms"`
	TimelineModalColdMs                 float64 `json:"timeline_modal_cold_ms"`
	VisibleThumbEligible                int     `json:"visible_thumb_eligible"`
	VisibleThumbColdMs                  float64 `json:"visible_thumb_cold_ms"`
	VisibleThumbColdErrors              int     `json:"visible_thumb_cold_errors"`
	VisibleThumbWarmMs                  float64 `json:"visible_thumb_warm_ms"`
	VisibleThumbWarmErrors              int     `json:"visible_thumb_warm_errors"`
	VisibleThumbStartupPrewarmLoaded    int     `json:"visible_thumb_startup_prewarm_loaded"`
	VisibleThumbStartupPrewarmFailed    int     `json:"visible_thumb_startup_prewarm_failed"`
	VisibleThumbStartupPrewarmMs        float64 `json:"visible_thumb_startup_prewarm_ms"`
	VisibleThumbAfterStartupPrewarmMs   float64 `json:"visible_thumb_after_startup_prewarm_ms"`
	VisibleThumbAfterStartupPrewarmFail int     `json:"visible_thumb_after_startup_prewarm_errors"`
	VisibleThumbPrewarmLoaded           int     `json:"visible_thumb_prewarm_loaded"`
	VisibleThumbPrewarmFailed           int     `json:"visible_thumb_prewarm_failed"`
	VisibleThumbPrewarmMs               float64 `json:"visible_thumb_prewarm_ms"`
	VisibleThumbAfterPrewarmMs          float64 `json:"visible_thumb_after_prewarm_ms"`
	VisibleThumbAfterPrewarmFail        int     `json:"visible_thumb_after_prewarm_errors"`
}

type HeadlessResultPerfReport struct {
	HasResult             bool    `json:"has_result"`
	SavedPath             string  `json:"saved_path,omitempty"`
	CanvasTargetPx        int     `json:"canvas_target_px"`
	ReducedCanvasTargetPx int     `json:"reduced_canvas_target_px"`
	ColdMs                float64 `json:"cold_ms"`
	WarmMs                float64 `json:"warm_ms"`
	ReducedColdMs         float64 `json:"reduced_cold_ms"`
	ReducedWarmMs         float64 `json:"reduced_warm_ms"`
	ManagedPreviewPath    string  `json:"managed_preview_path,omitempty"`
	ManagedPreviewReady   bool    `json:"managed_preview_ready"`
	ManagedPreviewMs      float64 `json:"managed_preview_ms"`
	OutputWidth           int     `json:"output_width"`
	OutputHeight          int     `json:"output_height"`
	ReducedOutputWidth    int     `json:"reduced_output_width"`
	ReducedOutputHeight   int     `json:"reduced_output_height"`
}

var headlessThumbBatchSizes = []int{48, 58, 88, 98, 104, 118, 152, 48, 58, 88, 98, 104, 118, 152, 48, 58, 88, 98}

func BuildHeadlessHistoryPerfReport(items []sharedCompat.HistoryItem) HeadlessHistoryPerfReport {
	report := HeadlessHistoryPerfReport{HistoryCount: len(items)}
	app := newHeadlessPerfApp(items)
	history := app.readSnapshot().History
	if len(history) == 0 {
		return report
	}

	start := time.Now()
	app.mu.Lock()
	app.historyRev++
	app.mu.Unlock()
	panel := app.historyPanelData(history)
	report.HistoryPanelColdMs = durationMillis(time.Since(start))
	report.HistoryPanelFiltered = panel.filteredCount

	start = time.Now()
	_ = app.historyPanelData(history)
	report.HistoryPanelCachedMs = durationMillis(time.Since(start))

	start = time.Now()
	app.mu.Lock()
	app.historyRev++
	app.mu.Unlock()
	timeline := app.historyTimelineData(history)
	report.HistoryTimelineColdMs = durationMillis(time.Since(start))
	report.HistoryTimelineFiltered = timeline.filteredCount

	start = time.Now()
	_ = app.historyTimelineData(history)
	report.HistoryTimelineCachedMs = durationMillis(time.Since(start))

	start = time.Now()
	app.mu.Lock()
	app.historyRev++
	app.mu.Unlock()
	timeline = app.historyTimelineData(history)
	_ = timeline.selectedGroupKey
	report.TimelineModalColdMs = durationMillis(time.Since(start))

	thumbItems := headlessThumbBatchItems(history, len(headlessThumbBatchSizes))
	report.VisibleThumbEligible = len(thumbItems)
	if len(thumbItems) == 0 {
		return report
	}
	thumbApp := &App{imageCache: map[string]cachedImage{}}
	start = time.Now()
	for idx, item := range thumbItems {
		if _, err := thumbApp.loadDisplayHistoryThumb(item, headlessThumbBatchSizes[idx]); err != nil {
			report.VisibleThumbColdErrors++
		}
	}
	report.VisibleThumbColdMs = durationMillis(time.Since(start))

	start = time.Now()
	for idx, item := range thumbItems {
		if _, err := thumbApp.loadDisplayHistoryThumb(item, headlessThumbBatchSizes[idx]); err != nil {
			report.VisibleThumbWarmErrors++
		}
	}
	report.VisibleThumbWarmMs = durationMillis(time.Since(start))

	startupPrewarmApp := &App{imageCache: map[string]cachedImage{}}
	start = time.Now()
	report.VisibleThumbStartupPrewarmLoaded, report.VisibleThumbStartupPrewarmFailed = startupPrewarmApp.prewarmHistoryThumbsWithLimit(thumbItems, historyThumbStartupSyncPrewarmCount)
	report.VisibleThumbStartupPrewarmMs = durationMillis(time.Since(start))
	start = time.Now()
	for idx, item := range thumbItems {
		if _, err := startupPrewarmApp.loadDisplayHistoryThumb(item, headlessThumbBatchSizes[idx]); err != nil {
			report.VisibleThumbAfterStartupPrewarmFail++
		}
	}
	report.VisibleThumbAfterStartupPrewarmMs = durationMillis(time.Since(start))

	prewarmApp := &App{imageCache: map[string]cachedImage{}}
	start = time.Now()
	report.VisibleThumbPrewarmLoaded, report.VisibleThumbPrewarmFailed = prewarmApp.prewarmHistoryThumbs(thumbItems)
	report.VisibleThumbPrewarmMs = durationMillis(time.Since(start))
	start = time.Now()
	for idx, item := range thumbItems {
		if _, err := prewarmApp.loadDisplayHistoryThumb(item, headlessThumbBatchSizes[idx]); err != nil {
			report.VisibleThumbAfterPrewarmFail++
		}
	}
	report.VisibleThumbAfterPrewarmMs = durationMillis(time.Since(start))

	return report
}

func BuildHeadlessResultPerfReport(items []sharedCompat.HistoryItem) HeadlessResultPerfReport {
	report := HeadlessResultPerfReport{
		CanvasTargetPx:        canvasDisplayMaxDimension,
		ReducedCanvasTargetPx: reducedEffectsCanvasDisplayMaxDimension,
	}
	latest, ok := newestHistoryItem(items)
	if !ok || strings.TrimSpace(latest.SavedPath) == "" {
		return report
	}
	report.HasResult = true
	report.SavedPath = strings.TrimSpace(latest.SavedPath)
	state := resultState{
		SavedPath: report.SavedPath,
		Item:      latest,
		HasItem:   true,
	}

	app := &App{imageCache: map[string]cachedImage{}}
	start := time.Now()
	img := app.loadCanvasDisplayImageForState(report.SavedPath, state)
	report.ColdMs = durationMillis(time.Since(start))
	if img != nil {
		report.OutputWidth = img.Bounds().Dx()
		report.OutputHeight = img.Bounds().Dy()
		app.persistManagedCanvasPreviewVariants(report.SavedPath, report.CanvasTargetPx, img)
	}
	start = time.Now()
	_ = app.loadCanvasDisplayImageForState(report.SavedPath, state)
	report.WarmMs = durationMillis(time.Since(start))

	reducedApp := &App{imageCache: map[string]cachedImage{}, reducedEffects: true}
	start = time.Now()
	reduced := reducedApp.loadCanvasDisplayImageForState(report.SavedPath, state)
	report.ReducedColdMs = durationMillis(time.Since(start))
	if reduced != nil {
		report.ReducedOutputWidth = reduced.Bounds().Dx()
		report.ReducedOutputHeight = reduced.Bounds().Dy()
	}
	start = time.Now()
	_ = reducedApp.loadCanvasDisplayImageForState(report.SavedPath, state)
	report.ReducedWarmMs = durationMillis(time.Since(start))

	if !isVirtualImagePath(report.SavedPath) {
		if managedPreviewPath, err := managedSourcePreviewPath(report.SavedPath, report.CanvasTargetPx); err == nil {
			report.ManagedPreviewPath = managedPreviewPath
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if headlessPathReady(managedPreviewPath) {
					report.ManagedPreviewReady = true
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if report.ManagedPreviewReady {
				managedApp := &App{imageCache: map[string]cachedImage{}}
				start = time.Now()
				_ = managedApp.loadCanvasDisplayImageForState(report.SavedPath, state)
				report.ManagedPreviewMs = durationMillis(time.Since(start))
			}
		}
	}

	return report
}

func newHeadlessPerfApp(items []sharedCompat.HistoryItem) *App {
	app := &App{
		historyModeFilter:         "all",
		historyDateFilter:         "all",
		historyTimelineModeFilter: "all",
		historyTimelineDateFilter: "all",
		imageCache:                map[string]cachedImage{},
	}
	if len(items) > 0 {
		app.selectedHistoryID = items[min(10, len(items)-1)].ID
	}
	app.mu.Lock()
	app.history = append([]sharedCompat.HistoryItem(nil), items...)
	app.historyRev = 1
	app.mu.Unlock()
	return app
}

func headlessThumbBatchItems(items []sharedCompat.HistoryItem, limit int) []sharedCompat.HistoryItem {
	if limit <= 0 || len(items) == 0 {
		return nil
	}
	out := make([]sharedCompat.HistoryItem, 0, min(len(items), limit))
	for _, item := range items {
		if len(out) >= limit {
			break
		}
		if !headlessHistoryItemThumbEligible(item) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func headlessHistoryItemThumbEligible(item sharedCompat.HistoryItem) bool {
	savedPath := strings.TrimSpace(item.SavedPath)
	if savedPath == "" || !headlessPathReady(savedPath) {
		return false
	}
	previewPath := strings.TrimSpace(item.PreviewPath)
	thumbPath := strings.TrimSpace(item.ThumbPath)
	return headlessPathReady(previewPath) || headlessPathReady(thumbPath) || headlessPathReady(savedPath)
}

func headlessPathReady(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if imageB64, ok := readVirtualImageB64(path); ok {
		return strings.TrimSpace(imageB64) != ""
	}
	if text, ok := readVirtualText(path); ok {
		return strings.TrimSpace(text) != ""
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func durationMillis(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}
