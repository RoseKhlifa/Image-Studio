//go:build windows || (linux && !android) || (darwin && !ios)

package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/diagnostics"
	"image-studio/gio-client/internal/ui"
)

func runPerfCommand(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, fmt.Errorf("usage: image-studio-gio perf <history|result> [flags]")
	}
	switch args[0] {
	case "history":
		return runPerfHistory(args[1:], stdout, stderr)
	case "result":
		return runPerfResult(args[1:], stdout, stderr)
	case "diagnose":
		return runPerfDiagnose(args[1:], stdout, stderr)
	default:
		return 2, fmt.Errorf("unknown perf command: %s", args[0])
	}
}

func runPerfHistory(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("perf history", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if fs.NArg() != 0 {
		return 2, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return 1, err
	}
	report := ui.BuildHeadlessHistoryPerfReport(state.History)
	if *jsonOut {
		return 0, writeJSON(stdout, report)
	}
	_, err = io.WriteString(stdout, formatPerfHistoryReport(report))
	return 0, err
}

func runPerfResult(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("perf result", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if fs.NArg() != 0 {
		return 2, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return 1, err
	}
	report := ui.BuildHeadlessResultPerfReport(state.History)
	if *jsonOut {
		return 0, writeJSON(stdout, report)
	}
	_, err = io.WriteString(stdout, formatPerfResultReport(report))
	return 0, err
}

func runPerfDiagnose(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("perf diagnose", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if fs.NArg() != 0 {
		return 2, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	state, path, err := gioCompat.LoadState()
	if err != nil {
		return 1, err
	}
	report := diagnostics.BuildCombinedReport(state, path)
	if *jsonOut {
		return 0, writeJSON(stdout, report)
	}
	_, err = io.WriteString(stdout, formatPerfDiagnoseReport(report))
	return 0, err
}

func formatPerfHistoryReport(report ui.HeadlessHistoryPerfReport) string {
	lines := []string{
		"Image Studio Gio Headless History Perf",
		fmt.Sprintf("history_count: %d", report.HistoryCount),
		fmt.Sprintf("history_panel_filtered: %d", report.HistoryPanelFiltered),
		fmt.Sprintf("history_panel_cold_ms: %.3f", report.HistoryPanelColdMs),
		fmt.Sprintf("history_panel_cached_ms: %.3f", report.HistoryPanelCachedMs),
		fmt.Sprintf("history_timeline_filtered: %d", report.HistoryTimelineFiltered),
		fmt.Sprintf("history_timeline_cold_ms: %.3f", report.HistoryTimelineColdMs),
		fmt.Sprintf("history_timeline_cached_ms: %.3f", report.HistoryTimelineCachedMs),
		fmt.Sprintf("timeline_modal_cold_ms: %.3f", report.TimelineModalColdMs),
		fmt.Sprintf("visible_thumb_eligible: %d", report.VisibleThumbEligible),
		fmt.Sprintf("visible_thumb_cold_ms: %.3f", report.VisibleThumbColdMs),
		fmt.Sprintf("visible_thumb_cold_errors: %d", report.VisibleThumbColdErrors),
		fmt.Sprintf("visible_thumb_warm_ms: %.3f", report.VisibleThumbWarmMs),
		fmt.Sprintf("visible_thumb_warm_errors: %d", report.VisibleThumbWarmErrors),
		fmt.Sprintf("visible_thumb_startup_prewarm_loaded: %d", report.VisibleThumbStartupPrewarmLoaded),
		fmt.Sprintf("visible_thumb_startup_prewarm_failed: %d", report.VisibleThumbStartupPrewarmFailed),
		fmt.Sprintf("visible_thumb_startup_prewarm_ms: %.3f", report.VisibleThumbStartupPrewarmMs),
		fmt.Sprintf("visible_thumb_after_startup_prewarm_ms: %.3f", report.VisibleThumbAfterStartupPrewarmMs),
		fmt.Sprintf("visible_thumb_after_startup_prewarm_errors: %d", report.VisibleThumbAfterStartupPrewarmFail),
		fmt.Sprintf("visible_thumb_prewarm_loaded: %d", report.VisibleThumbPrewarmLoaded),
		fmt.Sprintf("visible_thumb_prewarm_failed: %d", report.VisibleThumbPrewarmFailed),
		fmt.Sprintf("visible_thumb_prewarm_ms: %.3f", report.VisibleThumbPrewarmMs),
		fmt.Sprintf("visible_thumb_after_prewarm_ms: %.3f", report.VisibleThumbAfterPrewarmMs),
		fmt.Sprintf("visible_thumb_after_prewarm_errors: %d", report.VisibleThumbAfterPrewarmFail),
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatPerfResultReport(report ui.HeadlessResultPerfReport) string {
	lines := []string{
		"Image Studio Gio Headless Result Perf",
		fmt.Sprintf("has_result: %t", report.HasResult),
		"saved_path: " + report.SavedPath,
		fmt.Sprintf("canvas_target_px: %d", report.CanvasTargetPx),
		fmt.Sprintf("reduced_canvas_target_px: %d", report.ReducedCanvasTargetPx),
		fmt.Sprintf("cold_ms: %.3f", report.ColdMs),
		fmt.Sprintf("warm_ms: %.3f", report.WarmMs),
		fmt.Sprintf("reduced_cold_ms: %.3f", report.ReducedColdMs),
		fmt.Sprintf("reduced_warm_ms: %.3f", report.ReducedWarmMs),
		"managed_preview_path: " + report.ManagedPreviewPath,
		fmt.Sprintf("managed_preview_ready: %t", report.ManagedPreviewReady),
		fmt.Sprintf("managed_preview_ms: %.3f", report.ManagedPreviewMs),
		fmt.Sprintf("output_size: %dx%d", report.OutputWidth, report.OutputHeight),
		fmt.Sprintf("reduced_output_size: %dx%d", report.ReducedOutputWidth, report.ReducedOutputHeight),
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatPerfDiagnoseReport(report diagnostics.CombinedReport) string {
	lines := []string{
		"Image Studio Gio Perf Diagnose",
		"likely_bottleneck: " + report.LikelyBottleneck,
		"startup_bottleneck: " + report.StartupBottleneck,
		"startup_summary: " + report.StartupSummary,
		"steady_state_bottleneck: " + report.SteadyStateBottleneck,
		"steady_state_summary: " + report.SteadyStateSummary,
		"",
		strings.TrimRight(formatHistoryMediaReport(report.HistoryMedia), "\n"),
		"",
		strings.TrimRight(formatPerfHistoryReport(report.HistoryPerf), "\n"),
		"",
		strings.TrimRight(formatPerfResultReport(report.ResultPerf), "\n"),
	}
	return strings.Join(lines, "\n") + "\n"
}
