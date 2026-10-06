package backend

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeEditMaskB64ConvertsWhiteSelectionToTransparentAlpha(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.png")
	writeMaskTestPNG(t, sourcePath, image.NewUniform(color.NRGBA{R: 20, G: 30, B: 40, A: 255}), image.Rect(0, 0, 2, 1))

	mask := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	mask.SetNRGBA(0, 0, color.NRGBA{A: 255})
	mask.SetNRGBA(1, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	maskB64 := encodeMaskTestPNG(t, mask)

	normalizedB64, err := normalizeEditMaskB64(maskB64, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	normalized := decodeMaskTestPNG(t, normalizedB64)
	if got := color.NRGBAModel.Convert(normalized.At(0, 0)).(color.NRGBA).A; got != 255 {
		t.Fatalf("protected black alpha=%d want 255", got)
	}
	if got := color.NRGBAModel.Convert(normalized.At(1, 0)).(color.NRGBA).A; got != 0 {
		t.Fatalf("editable white alpha=%d want 0", got)
	}
}

func TestCompositeMaskedEditB64KeepsProtectedPixels(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.png")
	source := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	source.SetNRGBA(0, 0, color.NRGBA{R: 200, A: 255})
	source.SetNRGBA(1, 0, color.NRGBA{G: 200, A: 255})
	writeMaskTestPNG(t, sourcePath, source, source.Bounds())

	mask := image.NewNRGBA(source.Bounds())
	mask.SetNRGBA(0, 0, color.NRGBA{A: 255})
	mask.SetNRGBA(1, 0, color.NRGBA{A: 0})
	generated := image.NewNRGBA(source.Bounds())
	generated.SetNRGBA(0, 0, color.NRGBA{B: 220, A: 255})
	generated.SetNRGBA(1, 0, color.NRGBA{B: 220, A: 255})

	resultB64, err := compositeMaskedEditB64(sourcePath, encodeMaskTestPNG(t, mask), encodeMaskTestPNG(t, generated))
	if err != nil {
		t.Fatal(err)
	}
	result := decodeMaskTestPNG(t, resultB64)
	if got := color.NRGBAModel.Convert(result.At(0, 0)).(color.NRGBA); got != source.NRGBAAt(0, 0) {
		t.Fatalf("protected pixel=%v want source %v", got, source.NRGBAAt(0, 0))
	}
	if got := color.NRGBAModel.Convert(result.At(1, 0)).(color.NRGBA); got != generated.NRGBAAt(1, 0) {
		t.Fatalf("editable pixel=%v want generated %v", got, generated.NRGBAAt(1, 0))
	}
}

func TestNormalizeEditMaskB64PreservesFullyTransparentFullEditMask(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.png")
	writeMaskTestPNG(t, sourcePath, image.NewUniform(color.NRGBA{A: 255}), image.Rect(0, 0, 1, 1))
	mask := image.NewNRGBA(image.Rect(0, 0, 1, 1))

	normalizedB64, err := normalizeEditMaskB64(encodeMaskTestPNG(t, mask), sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	normalized := decodeMaskTestPNG(t, normalizedB64)
	if got := color.NRGBAModel.Convert(normalized.At(0, 0)).(color.NRGBA).A; got != 0 {
		t.Fatalf("full edit alpha=%d want 0", got)
	}
}

func encodeMaskTestPNG(t *testing.T, img image.Image) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func decodeMaskTestPNG(t *testing.T, value string) image.Image {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func writeMaskTestPNG(t *testing.T, path string, src image.Image, bounds image.Rectangle) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			img.Set(x, y, src.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}
