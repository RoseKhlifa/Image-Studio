package ui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sharedCompat "image-studio/shared/compat"
)

func encodeTestPNG(t *testing.T, fill color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return buf.Bytes()
}

func TestClipboardImageDataForResultSupportsVirtualImage(t *testing.T) {
	raw := encodeTestPNG(t, color.NRGBA{R: 0x55, G: 0x77, B: 0x99, A: 0xff})
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString(raw), "clipboard.png", "png")
	snap := snapshot{
		Result: resultState{
			SavedPath: virtualPath,
			Item: sharedCompat.HistoryItem{
				SavedPath:    virtualPath,
				OutputFormat: "png",
			},
		},
	}

	data, mimeType, err := clipboardImageDataForResult(snap)
	if err != nil {
		t.Fatalf("clipboardImageDataForResult: %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("mimeType=%q want image/png", mimeType)
	}
	if string(data) != string(raw) {
		t.Fatalf("clipboard bytes mismatch")
	}
}

func TestImportClipboardImageDataAddsVirtualSourceAndCanvasResult(t *testing.T) {
	raw := encodeTestPNG(t, color.NRGBA{R: 0xaa, G: 0x44, B: 0x22, A: 0xff})
	app := &App{}

	if err := app.importClipboardImageData(raw, "image/png"); err != nil {
		t.Fatalf("importClipboardImageData: %v", err)
	}
	paths := app.sourcePaths()
	if len(paths) != 1 || !strings.HasPrefix(paths[0], virtualImagePrefix) {
		t.Fatalf("sourcePaths=%v want one virtual image path", paths)
	}
	if app.result.SavedPath != paths[0] {
		t.Fatalf("result savedPath=%q want %q", app.result.SavedPath, paths[0])
	}
	if app.mode != "edit" || app.batchMode {
		t.Fatalf("mode=%q batchMode=%v want edit/false", app.mode, app.batchMode)
	}
	if app.result.Image == nil {
		t.Fatal("expected imported clipboard image preview")
	}
}

func TestClipboardImageDataForResultSupportsSavedPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "saved.png")
	raw := encodeTestPNG(t, color.NRGBA{R: 0x33, G: 0x66, B: 0x99, A: 0xff})
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	snap := snapshot{
		Result: resultState{
			SavedPath: path,
		},
	}
	data, mimeType, err := clipboardImageDataForResult(snap)
	if err != nil {
		t.Fatalf("clipboardImageDataForResult: %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("mimeType=%q want image/png", mimeType)
	}
	if string(data) != string(raw) {
		t.Fatalf("clipboard bytes mismatch")
	}
}
