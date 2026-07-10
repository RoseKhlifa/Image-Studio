package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncBatchSettingsFromInputsParsesAndClamps(t *testing.T) {
	app := &App{}
	app.batchConcurrencyInput.SetText("0")

	app.syncBatchSettingsFromInputs()

	if app.batchConcurrency != defaultBatchProcessConcurrency {
		t.Fatalf("batchConcurrency=%d want %d", app.batchConcurrency, defaultBatchProcessConcurrency)
	}
	if app.batchConcurrencyInput.Text() != "2" {
		t.Fatalf("batchConcurrencyInput=%q want 2", app.batchConcurrencyInput.Text())
	}

	app.batchConcurrencyInput.SetText("12")
	app.syncBatchSettingsFromInputs()
	if app.batchConcurrency != maxBatchProcessConcurrency {
		t.Fatalf("batchConcurrency=%d want %d", app.batchConcurrency, maxBatchProcessConcurrency)
	}
	if app.batchConcurrencyInput.Text() != "9" {
		t.Fatalf("batchConcurrencyInput=%q want 9", app.batchConcurrencyInput.Text())
	}
}

func TestWorkspaceSnapshotPreservesBatchConcurrency(t *testing.T) {
	app := &App{
		activeWorkspaceID: "ws-1",
		workspaces:        []workspaceState{{ID: "ws-1", Name: "图片 1"}},
		batchConcurrency:  4,
	}

	snapshot := app.buildWorkspaceSnapshot()
	if snapshot.BatchConcurrency != 4 {
		t.Fatalf("snapshot.BatchConcurrency=%d want 4", snapshot.BatchConcurrency)
	}

	app.batchConcurrency = 0
	app.batchConcurrencyInput.SetText("")
	app.applyWorkspace(snapshot)
	if app.batchConcurrency != 4 {
		t.Fatalf("app.batchConcurrency=%d want 4", app.batchConcurrency)
	}
	if app.batchConcurrencyInput.Text() != "4" {
		t.Fatalf("app.batchConcurrencyInput=%q want 4", app.batchConcurrencyInput.Text())
	}
}

func TestEffectiveBatchOutputDirHonorsOutputMode(t *testing.T) {
	app := &App{batchOutputMode: batchOutputModeSourceDir}
	app.batchOutputDirInput.SetText("/tmp/custom")
	if got := app.effectiveBatchOutputDir(); got != "" {
		t.Fatalf("effectiveBatchOutputDir(source_dir)=%q want empty", got)
	}

	app.setBatchOutputMode(batchOutputModeCustomDir)
	app.batchOutputDirInput.SetText("/tmp/custom")
	if got := app.effectiveBatchOutputDir(); got != "/tmp/custom" {
		t.Fatalf("effectiveBatchOutputDir(custom_dir)=%q want /tmp/custom", got)
	}
}

func TestWorkspaceSnapshotPreservesBatchOutputMode(t *testing.T) {
	app := &App{
		activeWorkspaceID: "ws-1",
		workspaces:        []workspaceState{{ID: "ws-1", Name: "图片 1"}},
		batchOutputMode:   batchOutputModeCustomDir,
	}
	app.batchOutputDirInput.SetText("/tmp/custom")
	app.batchOutputPrefixInput.SetText("render-")

	snapshot := app.buildWorkspaceSnapshot()
	if snapshot.BatchOutputMode != batchOutputModeCustomDir {
		t.Fatalf("snapshot.BatchOutputMode=%q want custom_dir", snapshot.BatchOutputMode)
	}
	if snapshot.BatchOutputPrefix != "render-" {
		t.Fatalf("snapshot.BatchOutputPrefix=%q want render-", snapshot.BatchOutputPrefix)
	}

	app.batchOutputMode = batchOutputModeSourceDir
	app.batchOutputDirInput.SetText("")
	app.batchOutputPrefixInput.SetText("")
	app.applyWorkspace(snapshot)
	if app.batchOutputMode != batchOutputModeCustomDir {
		t.Fatalf("app.batchOutputMode=%q want custom_dir", app.batchOutputMode)
	}
	if app.batchOutputDirInput.Text() != "/tmp/custom" {
		t.Fatalf("batchOutputDirInput=%q want /tmp/custom", app.batchOutputDirInput.Text())
	}
	if app.batchOutputPrefixInput.Text() != "render-" {
		t.Fatalf("batchOutputPrefixInput=%q want render-", app.batchOutputPrefixInput.Text())
	}
}

func TestNormalizeBatchOutputPrefixKeepsEmptyAndReplacesPathCharacters(t *testing.T) {
	if got := normalizeBatchOutputPrefix("   "); got != "" {
		t.Fatalf("normalizeBatchOutputPrefix(empty)=%q want empty", got)
	}
	if got := normalizeBatchOutputPrefix(` render/name:* `); got != "render-name--" {
		t.Fatalf("normalizeBatchOutputPrefix(invalid)=%q want render-name--", got)
	}
}

func TestBatchResultTargetPathUsesPrefixAndAvoidsCollisions(t *testing.T) {
	sourceDir := t.TempDir()
	outputDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "photo.png")
	if err := os.WriteFile(sourcePath, []byte("source"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	first := batchResultTargetPath(sourcePath, outputDir, "render-")
	if want := filepath.Join(outputDir, "render-photo.png"); first != want {
		t.Fatalf("first target=%q want %q", first, want)
	}
	if err := os.WriteFile(first, []byte("existing"), 0o600); err != nil {
		t.Fatalf("write collision: %v", err)
	}
	if got, want := batchResultTargetPath(sourcePath, outputDir, "render-"), filepath.Join(outputDir, "render-photo-2.png"); got != want {
		t.Fatalf("collision target=%q want %q", got, want)
	}

	if got, want := batchResultTargetPath(sourcePath, "", ""), filepath.Join(sourceDir, "photo-2.png"); got != want {
		t.Fatalf("empty-prefix source target=%q want %q", got, want)
	}
	if got := batchResultTargetPath(sourcePath, outputDir, `../escape/`); filepath.Dir(got) != outputDir {
		t.Fatalf("sanitized target escaped output dir: %q", got)
	}
}

func TestBatchModeSummaryTextMatchesDesktopWebviewSemantics(t *testing.T) {
	app := &App{
		batchConcurrency: 3,
		batchOutputMode:  batchOutputModeCustomDir,
		batchAutoAspect:  "1k",
		batchRetryOnFail: true,
	}

	got := app.batchModeSummaryText([]string{"/tmp/a.png", "/tmp/b.png"})
	want := "2 张 · 并发 3 · 独立输出目录 · 按源图比例 + 1K · 失败自动重试"
	if got != want {
		t.Fatalf("batchModeSummaryText=%q want %q", got, want)
	}
}
