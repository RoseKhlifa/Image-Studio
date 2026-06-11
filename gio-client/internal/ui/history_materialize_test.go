package ui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/yuanhua/image-gptcodex/pkg/client"
	sharedCompat "image-studio/shared/compat"
)

func testPNGBase64(t *testing.T, fill color.NRGBA) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestReuseHistoryItemAsSourceMaterializesPreviewOnlyItem(t *testing.T) {
	dir := t.TempDir()
	app := &App{}
	app.outputDirInput.SetText(dir)

	item := sharedCompat.HistoryItem{
		Prompt:       "preview cat",
		Mode:         string(client.ModeGenerate),
		OutputFormat: "png",
		ImageB64:     testPNGBase64(t, color.NRGBA{R: 0x55, G: 0x88, B: 0xcc, A: 0xff}),
	}

	app.reuseHistoryItemAsSource(item)

	paths := app.sourcePaths()
	if len(paths) != 1 {
		t.Fatalf("source paths=%v want 1 materialized path", paths)
	}
	if !strings.HasPrefix(paths[0], dir) {
		t.Fatalf("materialized source path=%q want under %q", paths[0], dir)
	}
	if _, err := os.Stat(paths[0]); err != nil {
		t.Fatalf("materialized source missing: %v", err)
	}
}

func TestReuseHistoryItemAsSourceUsesVirtualImageInRemoteMode(t *testing.T) {
	app := &App{
		kernelRuntimeMode: "remote",
	}
	item := sharedCompat.HistoryItem{
		Prompt:       "preview cat",
		Mode:         string(client.ModeGenerate),
		OutputFormat: "png",
		ImageB64:     testPNGBase64(t, color.NRGBA{R: 0x33, G: 0x66, B: 0x99, A: 0xff}),
	}

	app.reuseHistoryItemAsSource(item)

	paths := app.sourcePaths()
	if len(paths) != 1 {
		t.Fatalf("source paths=%v want 1 virtual path", paths)
	}
	if !strings.HasPrefix(paths[0], virtualImagePrefix) {
		t.Fatalf("source path=%q want memory://image/... virtual path", paths[0])
	}
}

func TestStartRunMaterializesPreviewOnlyCurrentResultForEditMode(t *testing.T) {
	dir := t.TempDir()
	app := &App{
		mode: string(client.ModeEdit),
		result: resultState{
			Item: sharedCompat.HistoryItem{
				Prompt:       "preview dog",
				Mode:         string(client.ModeGenerate),
				OutputFormat: "png",
				ImageB64:     testPNGBase64(t, color.NRGBA{R: 0x22, G: 0x66, B: 0xaa, A: 0xff}),
			},
			HasItem: true,
		},
	}
	app.outputDirInput.SetText(dir)

	app.startRun()

	savedPath := strings.TrimSpace(app.readSnapshot().Result.SavedPath)
	if savedPath == "" {
		t.Fatal("startRun should materialize current preview-only result before edit-mode fallback")
	}
	if !strings.HasPrefix(savedPath, dir) {
		t.Fatalf("saved path=%q want under %q", savedPath, dir)
	}
	if _, err := os.Stat(savedPath); err != nil {
		t.Fatalf("materialized current result missing: %v", err)
	}
}
