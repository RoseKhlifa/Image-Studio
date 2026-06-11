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

func TestBuildCanvasMaskB64ReturnsPNGWhenPaintStrokePresent(t *testing.T) {
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
	hasWhite := false
	for y := 0; y < img.Bounds().Dy() && !hasWhite; y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y > 0 {
				hasWhite = true
				break
			}
		}
	}
	if !hasWhite {
		t.Fatal("mask should contain white painted area")
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
