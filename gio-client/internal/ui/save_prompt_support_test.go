package ui

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sharedCompat "image-studio/shared/compat"
)

func TestOpenSavePromptForCurrentSupportsPreviewOnlyResult(t *testing.T) {
	dir := t.TempDir()
	app := &App{
		result: resultState{
			Item: sharedCompat.HistoryItem{
				ID:           "hist-1",
				Prompt:       "snow mountain",
				Mode:         "generate",
				OutputFormat: "png",
				ImageB64:     base64.StdEncoding.EncodeToString([]byte("image-bytes")),
			},
			HasItem: true,
		},
	}
	app.outputDirInput.SetText(dir)

	app.openSavePromptForCurrent()

	if !app.savePromptVisible {
		t.Fatal("save prompt should be visible for preview-only result")
	}
	if app.savePromptSourceImageB64 == "" {
		t.Fatal("save prompt should retain preview-only image data")
	}
	target := app.savePromptPathInput.Text()
	if !strings.HasPrefix(target, dir) {
		t.Fatalf("save prompt target=%q want under %q", target, dir)
	}
	if filepath.Ext(target) != ".png" {
		t.Fatalf("save prompt target=%q want .png suffix", target)
	}
}

func TestSavePromptCopyWritesPreviewOnlyImageData(t *testing.T) {
	dir := t.TempDir()
	want := []byte("preview-only-image")
	app := &App{
		savePromptVisible:        true,
		savePromptSourceImageB64: base64.StdEncoding.EncodeToString(want),
		savePromptSuggestedName:  "preview.png",
	}
	dst := filepath.Join(dir, "nested", "saved-preview")
	app.savePromptPathInput.SetText(dst)

	app.savePromptCopy()

	savedPath := filepath.Join(dir, "nested", "saved-preview.png")
	data, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("read saved preview: %v", err)
	}
	if string(data) != string(want) {
		t.Fatalf("saved preview=%q want %q", data, want)
	}
	if app.savePromptVisible {
		t.Fatal("save prompt should close after saving preview-only image")
	}
}

func TestOpenSavePromptForCurrentUsesVirtualSavedPathName(t *testing.T) {
	dir := t.TempDir()
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString([]byte("virtual-image")), "remote-result.png", "png")
	app := &App{
		result: resultState{
			SavedPath: virtualPath,
			Item: sharedCompat.HistoryItem{
				ID:           "hist-virtual",
				SavedPath:    virtualPath,
				Prompt:       "remote snow mountain",
				Mode:         "generate",
				OutputFormat: "png",
			},
			HasItem: true,
		},
	}
	app.outputDirInput.SetText(dir)

	app.openSavePromptForCurrent()

	if !app.savePromptVisible {
		t.Fatal("save prompt should be visible for virtual saved result")
	}
	if app.savePromptSourceImageB64 == "" {
		t.Fatal("save prompt should hydrate image data from virtual saved path")
	}
	if app.savePromptSuggestedName != "remote-result.png" {
		t.Fatalf("suggested name=%q want remote-result.png", app.savePromptSuggestedName)
	}
	target := app.savePromptPathInput.Text()
	if target != filepath.Join(dir, "remote-result.png") {
		t.Fatalf("save prompt target=%q want %q", target, filepath.Join(dir, "remote-result.png"))
	}
}

func TestOpenBatchSavePromptSelectsAllItems(t *testing.T) {
	dir := t.TempDir()
	items := []sharedCompat.HistoryItem{
		{ID: "a", Prompt: "a", Mode: "generate", OutputFormat: "png", ImageB64: base64.StdEncoding.EncodeToString([]byte("a"))},
		{ID: "b", Prompt: "b", Mode: "generate", OutputFormat: "png", ImageB64: base64.StdEncoding.EncodeToString([]byte("b"))},
	}
	app := &App{}
	app.outputDirInput.SetText(dir)

	app.openBatchSavePrompt(items)

	if !app.savePromptVisible {
		t.Fatal("batch save prompt should be visible")
	}
	if len(app.savePromptBatchItems) != 2 {
		t.Fatalf("batch items=%d want 2", len(app.savePromptBatchItems))
	}
	if !app.savePromptBatchSelection["a"] || !app.savePromptBatchSelection["b"] {
		t.Fatalf("batch selection=%v want both selected", app.savePromptBatchSelection)
	}
	if app.savePromptPathInput.Text() != dir {
		t.Fatalf("save prompt dir=%q want %q", app.savePromptPathInput.Text(), dir)
	}
}

func TestSavePromptCopyWritesSelectedBatchItemsToDirectory(t *testing.T) {
	dir := t.TempDir()
	srcDir := t.TempDir()
	src := filepath.Join(srcDir, "source.png")
	if err := os.WriteFile(src, []byte("source-image"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	app := &App{
		savePromptVisible: true,
		savePromptBatchItems: []sharedCompat.HistoryItem{
			{ID: "a", Prompt: "alpha", Mode: "generate", OutputFormat: "png", SavedPath: src},
			{ID: "b", Prompt: "beta", Mode: "generate", OutputFormat: "png", ImageB64: base64.StdEncoding.EncodeToString([]byte("beta-image"))},
		},
		savePromptBatchSelection: map[string]bool{"a": true, "b": false},
	}
	app.savePromptPathInput.SetText(dir)

	app.savePromptCopy()

	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("saved files=%d want 1", len(files))
	}
	if app.savePromptVisible {
		t.Fatal("batch save prompt should close after saving")
	}
}
