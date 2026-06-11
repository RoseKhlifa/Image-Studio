package ui

import (
	"image/color"
	"path/filepath"
	"testing"

	"github.com/yuanhua/image-gptcodex/pkg/client"
	sharedCompat "image-studio/shared/compat"
)

func TestViewSourcePathOnCanvasLoadsSourcePreview(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source-a.png")
	writeSolidTestPNG(t, sourcePath, color.NRGBA{R: 0x55, G: 0x88, B: 0xcc, A: 0xff})

	app := &App{mode: string(client.ModeGenerate)}
	if err := app.viewSourcePathOnCanvas(sourcePath); err != nil {
		t.Fatalf("viewSourcePathOnCanvas: %v", err)
	}
	if app.mode != string(client.ModeEdit) {
		t.Fatalf("mode=%q want edit", app.mode)
	}
	if app.result.SourceEvent != "source-preview" || app.result.SavedPath != sourcePath {
		t.Fatalf("result=%#v", app.result)
	}
	if app.result.Item.ID != "source-preview:"+sourcePath {
		t.Fatalf("itemID=%q want source-preview:%s", app.result.Item.ID, sourcePath)
	}
	if app.result.Image == nil {
		t.Fatal("expected source preview image to be loaded")
	}
	if app.selectedHistoryID != "" {
		t.Fatalf("selectedHistoryID=%q want empty", app.selectedHistoryID)
	}
}

func TestCompareSourcePathOnCanvasTogglesMainSourceComparison(t *testing.T) {
	resultPath := filepath.Join(t.TempDir(), "result-a.png")
	sourcePath := filepath.Join(t.TempDir(), "source-a.png")
	writeSolidTestPNG(t, resultPath, color.NRGBA{R: 0xcc, G: 0x66, B: 0x44, A: 0xff})
	writeSolidTestPNG(t, sourcePath, color.NRGBA{R: 0x44, G: 0x99, B: 0x66, A: 0xff})

	app := &App{
		result: resultState{
			SavedPath:   resultPath,
			HasItem:     true,
			SourceEvent: "history",
			Item: sharedCompat.HistoryItem{
				ID:        "hist-result",
				SavedPath: resultPath,
			},
		},
	}
	if err := app.compareSourcePathOnCanvas(sourcePath); err != nil {
		t.Fatalf("compareSourcePathOnCanvas: %v", err)
	}
	if app.compare.Item.ID != "source-preview:"+sourcePath || app.compare.SavedPath != sourcePath {
		t.Fatalf("compare=%#v", app.compare)
	}
	if app.compare.Image == nil {
		t.Fatal("expected compare image to be loaded")
	}
	if err := app.compareSourcePathOnCanvas(sourcePath); err != nil {
		t.Fatalf("compareSourcePathOnCanvas toggle off: %v", err)
	}
	if app.compare.HasItem || app.compare.Item.ID != "" {
		t.Fatalf("compare should be cleared, got %#v", app.compare)
	}
}

func TestImportImagePathAsEditSourcePromotesImportedImageToSource(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "imported-source.png")
	writeSolidTestPNG(t, sourcePath, color.NRGBA{R: 0x88, G: 0x66, B: 0x44, A: 0xff})

	app := &App{mode: string(client.ModeGenerate), batchMode: true}
	if err := app.importImagePathAsEditSource(sourcePath); err != nil {
		t.Fatalf("importImagePathAsEditSource: %v", err)
	}
	if app.mode != string(client.ModeEdit) {
		t.Fatalf("mode=%q want edit", app.mode)
	}
	if app.batchMode {
		t.Fatal("batchMode should reset to false after import")
	}
	if app.result.SavedPath != sourcePath || app.result.SourceEvent != "import" {
		t.Fatalf("result=%#v", app.result)
	}
	paths := app.sourcePaths()
	if len(paths) != 1 || paths[0] != sourcePath {
		t.Fatalf("sourcePaths=%v want [%s]", paths, sourcePath)
	}
}
