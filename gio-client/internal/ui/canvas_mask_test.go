package ui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"

	"gioui.org/f32"
)

func decodeMaskImage(t *testing.T, rawB64 string) image.Image {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		t.Fatalf("decode base64: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	return img
}

func TestBuildCanvasMaskB64UsesAlphaMaskSemantics(t *testing.T) {
	maskB64 := buildCanvasMaskB64([]canvasMaskStroke{{
		Points:   []f32.Point{f32.Pt(0.1, 0.1), f32.Pt(0.9, 0.9)},
		SizeNorm: 0.1,
		Erase:    false,
	}}, image.Pt(64, 64))
	if maskB64 == "" {
		t.Fatal("expected non-empty mask base64")
	}
	img := decodeMaskImage(t, maskB64)
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("mask bounds=%v want 64x64", img.Bounds())
	}
	if alpha := color.NRGBAModel.Convert(img.At(32, 32)).(color.NRGBA).A; alpha != 0 {
		t.Fatalf("painted pixel alpha=%d want transparent editable pixel", alpha)
	}
	if alpha := color.NRGBAModel.Convert(img.At(63, 0)).(color.NRGBA).A; alpha != 0xff {
		t.Fatalf("unpainted pixel alpha=%d want opaque protected pixel", alpha)
	}
}

func TestBuildCanvasMaskB64EraseRestoresProtection(t *testing.T) {
	maskB64 := buildCanvasMaskB64([]canvasMaskStroke{
		{Points: []f32.Point{f32.Pt(0.5, 0.5)}, SizeNorm: 0.5},
		{Points: []f32.Point{f32.Pt(0.5, 0.5)}, SizeNorm: 0.2, Erase: true},
	}, image.Pt(20, 20))
	img := decodeMaskImage(t, maskB64)
	if alpha := color.NRGBAModel.Convert(img.At(10, 10)).(color.NRGBA).A; alpha != 0xff {
		t.Fatalf("erased center alpha=%d want opaque protected pixel", alpha)
	}
	if alpha := color.NRGBAModel.Convert(img.At(6, 10)).(color.NRGBA).A; alpha != 0 {
		t.Fatalf("painted ring alpha=%d want transparent editable pixel", alpha)
	}
}

func TestBuildCanvasMaskB64ReturnsEmptyWithoutEditablePixels(t *testing.T) {
	maskB64 := buildCanvasMaskB64([]canvasMaskStroke{{
		Points:   []f32.Point{f32.Pt(0.5, 0.5)},
		SizeNorm: 0.2,
		Erase:    true,
	}}, image.Pt(20, 20))
	if maskB64 != "" {
		t.Fatal("erase-only mask should not submit a fully protected image")
	}
}

func TestCurrentConfigIncludesMaskB64ForEditMode(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source.png")
	writeSolidTestPNG(t, sourcePath, color.NRGBA{R: 0x22, G: 0x44, B: 0x66, A: 0xff})
	app := &App{
		mode: "edit",
		canvasMaskStrokes: []canvasMaskStroke{{
			Points:   []f32.Point{f32.Pt(0.2, 0.2), f32.Pt(0.8, 0.8)},
			SizeNorm: 0.05,
			Erase:    false,
		}},
	}
	app.sourcePathsInput.SetText(sourcePath)

	cfg := app.currentConfig()
	if cfg.MaskB64 == "" {
		t.Fatal("expected maskB64 in current config")
	}
	img := decodeMaskImage(t, cfg.MaskB64)
	if img.Bounds().Dx() != 2 || img.Bounds().Dy() != 2 {
		t.Fatalf("mask bounds=%v want 2x2 source dims", img.Bounds())
	}
}

func TestImportedCanvasMaskFlowsIntoEditRequestAndCanBeCleared(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.png")
	maskPath := filepath.Join(dir, "mask.png")
	writeSolidTestPNG(t, sourcePath, color.NRGBA{R: 0x22, G: 0x44, B: 0x66, A: 0xff})
	writeSolidTestPNG(t, maskPath, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	app := &App{mode: "edit"}
	app.sourcePathsInput.SetText(sourcePath)

	if err := app.importCanvasMask(maskPath); err != nil {
		t.Fatal(err)
	}
	if !app.hasImportedCanvasMask() {
		t.Fatal("expected imported mask state")
	}
	cfg := app.currentConfig()
	if cfg.MaskB64 == "" {
		t.Fatal("expected imported mask in edit config")
	}
	decodeMaskImage(t, cfg.MaskB64)

	app.clearCanvasMask()
	if app.currentConfig().MaskB64 != "" {
		t.Fatal("clear mask should remove imported mask from edit config")
	}
}
