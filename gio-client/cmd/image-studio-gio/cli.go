package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/yuanhua/image-gptcodex/pkg/promptimport"
	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/historymedia"
	"image-studio/gio-client/internal/promptipc"
	"image-studio/gio-client/internal/promptscheme"
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

func promptImportMessageFromURL(uri *url.URL) promptipc.Message {
	if uri == nil {
		return promptipc.Message{}
	}
	msg := promptImportMessageFromArgs([]string{uri.String()})
	switch msg.Type {
	case promptipc.MessageTypeToken:
		return msg
	case promptipc.MessageTypeInvalid:
		return msg
	default:
		return promptipc.Message{}
	}
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
