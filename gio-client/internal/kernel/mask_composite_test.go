package kernel

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

func TestNormalizeEditMaskB64ScalesOpaqueBlackWhiteMask(t *testing.T) {
	legacy := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	legacy.SetNRGBA(0, 0, color.NRGBA{A: 0xff})
	legacy.SetNRGBA(1, 0, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})

	normalizedB64, normalized, err := normalizeEditMaskB64(encodeKernelTestPNG(t, legacy), image.Pt(4, 2))
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Bounds().Size() != image.Pt(4, 2) {
		t.Fatalf("normalized bounds=%v want 4x2", normalized.Bounds())
	}
	if alpha := normalized.NRGBAAt(0, 0).A; alpha != 0xff {
		t.Fatalf("black legacy pixel alpha=%d want opaque protection", alpha)
	}
	if alpha := normalized.NRGBAAt(3, 1).A; alpha != 0 {
		t.Fatalf("white legacy pixel alpha=%d want transparent edit selection", alpha)
	}
	decoded := decodeKernelTestPNG(t, normalizedB64)
	if decoded.Bounds().Size() != image.Pt(4, 2) {
		t.Fatalf("encoded normalized bounds=%v want 4x2", decoded.Bounds())
	}
}

func TestNormalizeEditMaskB64PreservesAlphaSemantics(t *testing.T) {
	alphaMask := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	alphaMask.SetNRGBA(0, 0, color.NRGBA{R: 0xff, G: 0x80, B: 0x20, A: 0x40})

	_, normalized, err := normalizeEditMaskB64(encodeKernelTestPNG(t, alphaMask), image.Pt(2, 2))
	if err != nil {
		t.Fatal(err)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			pixel := normalized.NRGBAAt(x, y)
			if pixel.A != 0x40 || pixel.R != 0 || pixel.G != 0 || pixel.B != 0 {
				t.Fatalf("normalized pixel (%d,%d)=%v want RGB zero and alpha 64", x, y, pixel)
			}
		}
	}
}

func TestPrepareEditMaskUsesFirstSourceDataURLDimensions(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	mask := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	mask.SetNRGBA(0, 0, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	cfg := Config{
		Mode:                client.ModeEdit,
		SourceImageDataURLs: []string{"data:image/png;base64," + encodeKernelTestPNG(t, source)},
		MaskB64:             encodeKernelTestPNG(t, mask),
	}

	prepared, err := prepareEditMask(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if prepared == nil || prepared.mask.Bounds().Size() != image.Pt(3, 2) {
		t.Fatalf("prepared mask=%v want source dimensions 3x2", prepared)
	}
	if decoded := decodeKernelTestPNG(t, cfg.MaskB64); decoded.Bounds().Size() != image.Pt(3, 2) {
		t.Fatalf("request mask bounds=%v want source dimensions 3x2", decoded.Bounds())
	}
}

func TestCompositeMaskedEditB64RestoresProtectedPixels(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	source.SetNRGBA(0, 0, color.NRGBA{R: 0xff, A: 0xff})
	source.SetNRGBA(1, 0, color.NRGBA{B: 0xff, A: 0xff})
	mask := image.NewNRGBA(source.Bounds())
	mask.SetNRGBA(0, 0, color.NRGBA{A: 0xff})
	mask.SetNRGBA(1, 0, color.NRGBA{A: 0})
	generated := image.NewNRGBA(source.Bounds())
	generated.SetNRGBA(0, 0, color.NRGBA{G: 0xff, A: 0xff})
	generated.SetNRGBA(1, 0, color.NRGBA{R: 0xff, G: 0xff, A: 0xff})

	resultB64, err := compositeMaskedEditB64(&preparedEditMask{source: source, mask: mask}, encodeKernelTestPNG(t, generated))
	if err != nil {
		t.Fatal(err)
	}
	result := decodeKernelTestPNG(t, resultB64)
	assertKernelPixel(t, result, 0, 0, color.NRGBA{R: 0xff, A: 0xff})
	assertKernelPixel(t, result, 1, 0, color.NRGBA{R: 0xff, G: 0xff, A: 0xff})
}

func TestRunnerRunNormalizesAndCompositesMaskedEditBeforeSave(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	source.SetNRGBA(0, 0, color.NRGBA{R: 0xff, A: 0xff})
	source.SetNRGBA(1, 0, color.NRGBA{B: 0xff, A: 0xff})
	sourcePath := filepath.Join(t.TempDir(), "source.png")
	writeKernelTestPNG(t, sourcePath, source)

	legacyMask := image.NewNRGBA(source.Bounds())
	legacyMask.SetNRGBA(0, 0, color.NRGBA{A: 0xff})
	legacyMask.SetNRGBA(1, 0, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
	generated := image.NewNRGBA(source.Bounds())
	generated.SetNRGBA(0, 0, color.NRGBA{G: 0xff, A: 0xff})
	generated.SetNRGBA(1, 0, color.NRGBA{R: 0xff, G: 0xff, A: 0xff})
	generatedB64 := encodeKernelTestPNG(t, generated)

	type capturedMask struct {
		img image.Image
		err error
	}
	captured := make(chan capturedMask, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/images/edits" {
			captured <- capturedMask{err: fmt.Errorf("unexpected path %s", r.URL.Path)}
			http.NotFound(w, r)
			return
		}
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			captured <- capturedMask{err: err}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("mask")
		if err != nil {
			captured <- capturedMask{err: err}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mask, decodeErr := png.Decode(file)
		_ = file.Close()
		captured <- capturedMask{img: mask, err: decodeErr}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"` + generatedB64 + `"}]}`))
	}))
	defer server.Close()

	result, err := (Runner{}).Run(context.Background(), Config{
		APIKey:           "sk-mask-test",
		BaseURL:          server.URL,
		Prompt:           "replace selected pixel",
		Mode:             client.ModeEdit,
		APIMode:          client.APIModeImages,
		OutputDir:        t.TempDir(),
		OutputFormat:     "jpeg",
		SourcePaths:      []string{sourcePath},
		MaskB64:          encodeKernelTestPNG(t, legacyMask),
		AutoRetryEnabled: false,
	}, Callbacks{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	uploaded := <-captured
	if uploaded.err != nil {
		t.Fatalf("capture uploaded mask: %v", uploaded.err)
	}
	if uploaded.img.Bounds().Size() != source.Bounds().Size() {
		t.Fatalf("uploaded mask bounds=%v want %v", uploaded.img.Bounds(), source.Bounds())
	}
	if alpha := color.NRGBAModel.Convert(uploaded.img.At(0, 0)).(color.NRGBA).A; alpha != 0xff {
		t.Fatalf("uploaded protected alpha=%d want 255", alpha)
	}
	if alpha := color.NRGBAModel.Convert(uploaded.img.At(1, 0)).(color.NRGBA).A; alpha != 0 {
		t.Fatalf("uploaded editable alpha=%d want 0", alpha)
	}
	if result.OutputFormat != "png" || strings.ToLower(filepath.Ext(result.SavedPath)) != ".png" {
		t.Fatalf("masked result format=%q path=%q want PNG", result.OutputFormat, result.SavedPath)
	}
	savedFile, err := os.Open(result.SavedPath)
	if err != nil {
		t.Fatal(err)
	}
	saved, _, err := image.Decode(savedFile)
	_ = savedFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	assertKernelPixel(t, saved, 0, 0, color.NRGBA{R: 0xff, A: 0xff})
	assertKernelPixel(t, saved, 1, 0, color.NRGBA{R: 0xff, G: 0xff, A: 0xff})
}

func encodeKernelTestPNG(t *testing.T, img image.Image) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func decodeKernelTestPNG(t *testing.T, rawB64 string) image.Image {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func writeKernelTestPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertKernelPixel(t *testing.T, img image.Image, x, y int, want color.NRGBA) {
	t.Helper()
	got := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	if got != want {
		t.Fatalf("pixel (%d,%d)=%v want %v", x, y, got, want)
	}
}
