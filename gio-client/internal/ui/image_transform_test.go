package ui

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

func TestRotateImageFileSwapsDimensions(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.png")
	writeTestPNG(t, src, 4, 2)

	out, err := rotateImageFile(src, 90)
	if err != nil {
		t.Fatalf("rotateImageFile: %v", err)
	}
	img, err := decodeImageFile(out)
	if err != nil {
		t.Fatalf("decodeImageFile: %v", err)
	}
	if got, want := img.Bounds().Dx(), 2; got != want {
		t.Fatalf("rotated width=%d want %d", got, want)
	}
	if got, want := img.Bounds().Dy(), 4; got != want {
		t.Fatalf("rotated height=%d want %d", got, want)
	}
}

func TestFlipImageFileKeepsDimensions(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.png")
	writeTestPNG(t, src, 3, 5)

	out, err := flipImageFile(src, true)
	if err != nil {
		t.Fatalf("flipImageFile: %v", err)
	}
	img, err := decodeImageFile(out)
	if err != nil {
		t.Fatalf("decodeImageFile: %v", err)
	}
	if got, want := img.Bounds().Dx(), 3; got != want {
		t.Fatalf("flipped width=%d want %d", got, want)
	}
	if got, want := img.Bounds().Dy(), 5; got != want {
		t.Fatalf("flipped height=%d want %d", got, want)
	}
}

func TestRotateImageFileSupportsVirtualImagePath(t *testing.T) {
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString(fakePNGBytes()), "preview.png", "png")

	out, err := rotateImageFile(virtualPath, 90)
	if err != nil {
		t.Fatalf("rotateImageFile virtual: %v", err)
	}
	if !isVirtualImagePath(out) {
		t.Fatalf("rotated path=%q want virtual image path", out)
	}
	img, err := decodeImageFile(out)
	if err != nil {
		t.Fatalf("decodeImageFile virtual output: %v", err)
	}
	if got, want := img.Bounds().Dx(), 2; got != want {
		t.Fatalf("virtual rotated width=%d want %d", got, want)
	}
}

func TestCropImageFileKeepsRequestedRect(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.png")
	writeTestPNG(t, src, 6, 5)

	out, err := cropImageFile(src, image.Rect(1, 1, 4, 4))
	if err != nil {
		t.Fatalf("cropImageFile: %v", err)
	}
	img, err := decodeImageFile(out)
	if err != nil {
		t.Fatalf("decodeImageFile: %v", err)
	}
	if got, want := img.Bounds().Dx(), 3; got != want {
		t.Fatalf("cropped width=%d want %d", got, want)
	}
	if got, want := img.Bounds().Dy(), 3; got != want {
		t.Fatalf("cropped height=%d want %d", got, want)
	}
}

func TestCropImageFileSupportsVirtualImagePath(t *testing.T) {
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString(fakePNGBytes()), "preview.png", "png")

	out, err := cropImageFile(virtualPath, image.Rect(1, 0, 3, 2))
	if err != nil {
		t.Fatalf("cropImageFile virtual: %v", err)
	}
	if !isVirtualImagePath(out) {
		t.Fatalf("cropped path=%q want virtual image path", out)
	}
	img, err := decodeImageFile(out)
	if err != nil {
		t.Fatalf("decodeImageFile virtual crop: %v", err)
	}
	if got, want := img.Bounds().Dx(), 2; got != want {
		t.Fatalf("virtual cropped width=%d want %d", got, want)
	}
	if got, want := img.Bounds().Dy(), 2; got != want {
		t.Fatalf("virtual cropped height=%d want %d", got, want)
	}
}

func writeTestPNG(t *testing.T, path string, width int, height int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 10), G: uint8(y * 10), B: 120, A: 255})
		}
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("os.Create: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
}

func fakePNGBytes() []byte {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 10), G: uint8(y * 10), B: 120, A: 255})
		}
	}
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
