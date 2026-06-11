package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yuanhua/image-gptcodex/pkg/promptimport"
	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/diagnostics"
	"image-studio/gio-client/internal/historymedia"
	"image-studio/gio-client/internal/promptipc"
	"image-studio/gio-client/internal/promptscheme"
	"image-studio/gio-client/internal/ui"
)

func runCLICommand(args []string, stdout io.Writer, stderr io.Writer) (bool, int, error) {
	if len(args) == 0 {
		return false, 0, nil
	}
	switch args[0] {
	case "history-media":
		code, err := runHistoryMediaCommand(args[1:], stdout, stderr)
		return true, code, err
	case "perf":
		code, err := runPerfCommand(args[1:], stdout, stderr)
		return true, code, err
	case "protocol":
		code, err := runProtocolCommand(args[1:], stdout, stderr)
		return true, code, err
	case "import-token":
		code, err := runImportTokenCommand(args[1:], stdout, stderr)
		return true, code, err
	case "open-result":
		code, err := runOpenResultCommand(args[1:], stdout, stderr)
		return true, code, err
	default:
		return false, 0, nil
	}
}

func runProtocolCommand(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, fmt.Errorf("usage: image-studio-gio protocol <register|unregister|status>")
	}
	switch args[0] {
	case "register":
		if err := promptscheme.RegisterCurrentExecutable(); err != nil {
			return 1, err
		}
		_, err := io.WriteString(stdout, "protocol registered\n")
		return 0, err
	case "unregister":
		if err := promptscheme.UnregisterCurrentExecutable(); err != nil {
			return 1, err
		}
		_, err := io.WriteString(stdout, "protocol unregistered\n")
		return 0, err
	case "status":
		status, err := promptscheme.StatusForCurrentExecutable()
		if err != nil {
			return 1, err
		}
		return 0, writeJSON(stdout, status)
	default:
		return 2, fmt.Errorf("unknown protocol command: %s", args[0])
	}
}

func runImportTokenCommand(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, fmt.Errorf("usage: image-studio-gio import-token <token|image-studio://import?...>")
	}
	token, err := normalizePromptImportTokenArg(args[0])
	if err != nil {
		return 2, err
	}
	if err := promptipc.SendToken(token); err == nil {
		_, writeErr := io.WriteString(stdout, "token sent to running instance\n")
		return 0, writeErr
	}
	payload, fetchErr := promptimport.Fetch(context.Background(), token, promptimport.FetchOptions{})
	if fetchErr != nil {
		return 1, fetchErr
	}
	return 0, writeJSON(stdout, payload)
}

func runOpenResultCommand(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("open-result", flag.ContinueOnError)
	fs.SetOutput(stderr)
	resultID := fs.String("id", "", "history item id")
	savedPath := fs.String("path", "", "saved image path")
	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	if fs.NArg() != 0 {
		return 2, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if err := promptipc.SendOpenResult(*resultID, *savedPath); err != nil {
		return 1, err
	}
	_, writeErr := io.WriteString(stdout, "result detail request sent to running instance\n")
	return 0, writeErr
}

func promptImportMessageFromArgs(args []string) promptipc.Message {
	for _, arg := range args {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			continue
		}
		if !strings.Contains(trimmed, "image-studio://") {
			continue
		}
		token, err := promptimport.ParseTokenFromURL(trimmed)
		if err != nil {
			if promptimport.ErrorCode(err) == promptimport.TokenInvalid {
				return promptipc.Message{Type: promptipc.MessageTypeInvalid}
			}
			continue
		}
		return promptipc.Message{Type: promptipc.MessageTypeToken, Token: token}
	}
	return promptipc.Message{}
}

func normalizePromptImportTokenArg(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", fmt.Errorf("token is empty")
	}
	if strings.Contains(trimmed, "image-studio://") {
		return promptimport.ParseTokenFromURL(trimmed)
	}
	if !promptimport.IsValidToken(trimmed) {
		return "", fmt.Errorf("invalid token")
	}
	return trimmed, nil
}

func runHistoryMediaCommand(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, fmt.Errorf("usage: image-studio-gio history-media <report|backfill> [flags]")
	}
	switch args[0] {
	case "report":
		return runHistoryMediaReport(args[1:], stdout, stderr)
	case "backfill":
		return runHistoryMediaBackfill(args[1:], stdout, stderr)
	default:
		return 2, fmt.Errorf("unknown history-media command: %s", args[0])
	}
}

func runHistoryMediaReport(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("history-media report", flag.ContinueOnError)
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
	report := historymedia.BuildReport(state, path)
	if *jsonOut {
		return 0, writeJSON(stdout, report)
	}
	_, err = io.WriteString(stdout, formatHistoryMediaReport(report))
	return 0, err
}

func runHistoryMediaBackfill(args []string, stdout io.Writer, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("history-media backfill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "output JSON")
	previewOnly := fs.Bool("preview-only", false, "only backfill previewPath from existing thumbPath")
	limit := fs.Int("limit", 0, "limit unique saved paths to process (0 = no limit)")
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
	updates, summary := historymedia.BuildBackfillUpdates(state.History, *limit, *previewOnly)
	if len(updates) > 0 {
		changed := historymedia.ApplyBackfillUpdates(&state, updates)
		if changed > 0 {
			state.UpdatedAt = time.Now().UnixMilli()
			if err := gioCompat.SaveState(state); err != nil {
				return 1, err
			}
		}
	}
	report := historymedia.BuildReport(state, path)
	output := struct {
		Summary historymedia.BackfillSummary `json:"summary"`
		Report  historymedia.Report          `json:"report"`
	}{
		Summary: summary,
		Report:  report,
	}
	if *jsonOut {
		return 0, writeJSON(stdout, output)
	}
	_, err = io.WriteString(stdout, formatHistoryMediaBackfill(output.Summary, output.Report))
	return 0, err
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

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

func formatHistoryMediaBackfill(summary historymedia.BackfillSummary, report historymedia.Report) string {
	lines := []string{
		fmt.Sprintf("backfill candidate paths: %d", summary.CandidatePaths),
		fmt.Sprintf("backfill preview-only paths: %d", summary.PreviewOnlyPaths),
		fmt.Sprintf("backfill heavy paths: %d", summary.HeavyPaths),
		fmt.Sprintf("backfill failed paths: %d", summary.FailedPaths),
		fmt.Sprintf("updated items: %d", summary.UpdatedItems),
		fmt.Sprintf("preview paths added: %d", summary.PreviewPathsAdded),
		fmt.Sprintf("thumb paths added: %d", summary.ThumbPathsAdded),
		"",
		strings.TrimRight(formatHistoryMediaReport(report), "\n"),
	}
	return strings.Join(lines, "\n") + "\n"
}

func formatHistoryMediaReport(report historymedia.Report) string {
	lines := []string{
		"Image Studio Gio History Media Report",
		"state_path: " + report.StatePath,
		"client: " + report.Client,
		fmt.Sprintf("updated_at: %d", report.UpdatedAt),
		fmt.Sprintf("history_count: %d", report.HistoryCount),
		fmt.Sprintf("saved_paths: %d present, %d files ok, %d files missing", report.SavedPathPresent, report.SavedFilePresent, report.SavedFileMissing),
		fmt.Sprintf("thumb_paths: %d present, %d files ok, %d files missing", report.ThumbPathPresent, report.ThumbFilePresent, report.ThumbFileMissing),
		fmt.Sprintf("preview_paths: %d present, %d files ok, %d files missing", report.PreviewPathPresent, report.PreviewFilePresent, report.PreviewFileMissing),
		fmt.Sprintf("preview_only_backfill_candidates: %d", report.PreviewOnlyBackfillCandidates),
		fmt.Sprintf("heavy_backfill_candidates: %d", report.HeavyBackfillCandidates),
	}
	return strings.Join(lines, "\n") + "\n"
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
