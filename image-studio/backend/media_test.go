package backend

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMediaHandlerPreservesAllGIFFrames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	fullPath := filepath.Join(imagesSubdir(root), "animation.gif")
	if err := os.MkdirAll(filepath.Dir(fullPath), secureDirMode); err != nil {
		t.Fatal(err)
	}
	palette := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	first := image.NewPaletted(image.Rect(0, 0, 24, 16), palette)
	second := image.NewPaletted(first.Bounds(), palette)
	for i := range second.Pix {
		second.Pix[i] = 1
	}
	var original bytes.Buffer
	if err := gif.EncodeAll(&original, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{8, 20}, LoopCount: 0}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fullPath, original.Bytes(), secureFileMode); err != nil {
		t.Fatal(err)
	}
	svc := NewService()
	if err := svc.RegisterTrustedOutputDir(root); err != nil {
		t.Fatal(err)
	}
	ref, err := svc.RegisterMediaAsset(fullPath, "")
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	svc.MediaHandler(http.NotFoundHandler()).ServeHTTP(response, httptest.NewRequest(http.MethodGet, ref.FullURL, nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/gif" {
		t.Fatalf("status=%d type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	if !bytes.Equal(response.Body.Bytes(), original.Bytes()) {
		t.Fatal("original animation bytes changed")
	}
	animation, err := gif.DecodeAll(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(animation.Image) != 2 || animation.Delay[0] != 8 || animation.Delay[1] != 20 || animation.LoopCount != 0 {
		t.Fatalf("animation frames or timing lost: %+v", animation)
	}
}

func TestListBatchInputImagesOnlyScansCurrentDirectoryImages(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, secureDirMode); err != nil {
		t.Fatal(err)
	}
	makePNG := func(path string) {
		t.Helper()
		f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, secureFileMode)
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 32, 16))
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	makePNG(filepath.Join(root, "a.png"))
	makePNG(filepath.Join(root, "b.webp"))
	makePNG(filepath.Join(nested, "c.png"))
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("x"), secureFileMode); err != nil {
		t.Fatal(err)
	}

	svc := NewService()
	result, err := svc.ListBatchInputImages(root)
	if err != nil {
		t.Fatal(err)
	}
	if result.Directory != root {
		t.Fatalf("directory = %q, want %q", result.Directory, root)
	}
	if len(result.Images) != 2 {
		t.Fatalf("images len = %d, want 2", len(result.Images))
	}
	got := []string{result.Images[0].Name, result.Images[1].Name}
	if !(got[0] == "a.png" && got[1] == "b.webp" || got[0] == "b.webp" && got[1] == "a.png") {
		t.Fatalf("unexpected files: %#v", got)
	}
}

func TestBuildBatchOutputPathAppliesPrefixAndAvoidsCollisions(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "sample.png")
	if err := os.WriteFile(src, []byte("png"), secureFileMode); err != nil {
		t.Fatal(err)
	}
	svc := NewService()
	first, err := svc.BuildBatchOutputPath(src, root, "processed-")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "processed-sample.png"); first != want {
		t.Fatalf("first path = %q, want %q", first, want)
	}
	if err := os.WriteFile(first, []byte("done"), secureFileMode); err != nil {
		t.Fatal(err)
	}
	second, err := svc.BuildBatchOutputPath(src, root, "processed-")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "processed-sample-2.png"); second != want {
		t.Fatalf("second path = %q, want %q", second, want)
	}
}

func TestMediaHandlerServesRegisteredFullAndAVIFThumb(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root, err := defaultOutputDir()
	if err != nil {
		t.Fatal(err)
	}
	imagesDir := imagesSubdir(root)
	thumbsDir := thumbsSubdir(root)
	previewsDir := previewsSubdir(root)
	if err := os.MkdirAll(imagesDir, secureDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(thumbsDir, secureDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(previewsDir, secureDirMode); err != nil {
		t.Fatal(err)
	}
	fullPath := filepath.Join(imagesDir, "sample.png")
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, secureFileMode)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 640, 320))
	for y := 0; y < 320; y++ {
		for x := 0; x < 640; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 160, A: 255})
		}
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	thumbPath := filepath.Join(thumbsDir, "sample.avif")
	tw, th, err := createAVIFThumbnail(fullPath, thumbPath, 384)
	if err != nil {
		t.Fatal(err)
	}
	if tw != 384 || th != 192 {
		t.Fatalf("thumbnail size = %dx%d, want 384x192", tw, th)
	}

	svc := NewService()
	ref, err := svc.RegisterMediaAsset(fullPath, thumbPath)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ImageID == "" || ref.PreviewURL == "" || ref.FullURL == "" {
		t.Fatalf("incomplete media ref: %+v", ref)
	}
	handler := svc.MediaHandler(http.NotFoundHandler())

	req := httptest.NewRequest(http.MethodGet, ref.PreviewURL, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("thumb status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/avif" {
		t.Fatalf("thumb content-type = %q", got)
	}
	if got, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition")); err != nil || got != "inline" || params["filename"] != "sample.avif" {
		t.Fatalf("thumb content-disposition = %q (parsed=%q %+v err=%v)", rec.Header().Get("Content-Disposition"), got, params, err)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty thumb body")
	}

	req = httptest.NewRequest(http.MethodGet, ref.FullURL, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("full status = %d", rec.Code)
	}
	if got, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition")); err != nil || got != "inline" || params["filename"] != "sample.png" {
		t.Fatalf("full content-disposition = %q (parsed=%q %+v err=%v)", rec.Header().Get("Content-Disposition"), got, params, err)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty full body")
	}

	var previewBuf bytes.Buffer
	if err := png.Encode(&previewBuf, img); err != nil {
		t.Fatal(err)
	}
	previewPath := filepath.Join(previewsDir, "partial.avif")
	pw, ph, err := createAVIFThumbnailFromBase64(base64.StdEncoding.EncodeToString(previewBuf.Bytes()), previewPath, 384)
	if err != nil {
		t.Fatal(err)
	}
	if pw != 384 || ph != 192 {
		t.Fatalf("preview size = %dx%d, want 384x192", pw, ph)
	}
	previewAsset, err := svc.registerPreviewMedia(previewPath, pw, ph)
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, previewAsset.PreviewURL, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/avif" {
		t.Fatalf("preview content-type = %q", got)
	}
	if got, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition")); err != nil || got != "inline" || params["filename"] != "partial.avif" {
		t.Fatalf("preview content-disposition = %q (parsed=%q %+v err=%v)", rec.Header().Get("Content-Disposition"), got, params, err)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty preview body")
	}
}

func TestRegisterImportedImageAssetCreatesManagedAVIFPreview(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	importDir, err := importsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(importDir, secureDirMode); err != nil {
		t.Fatal(err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 320, 640))
	for y := 0; y < 640; y++ {
		for x := 0; x < 320; x++ {
			img.Set(x, y, color.RGBA{R: 90, G: uint8(x % 255), B: uint8(y % 255), A: 255})
		}
	}
	srcPath := filepath.Join(importDir, "source.png")
	f, err := os.OpenFile(srcPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, secureFileMode)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	svc := NewService()
	ref, err := svc.RegisterImportedImageAsset(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	if ref.ImageID == "" || ref.PreviewURL == "" || ref.FullURL == "" {
		t.Fatalf("unexpected imported media ref: %+v", ref)
	}
	resolvedSrc, err := filepath.EvalSymlinks(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	if ref.SavedPath != resolvedSrc {
		t.Fatalf("saved path = %q, want %q", ref.SavedPath, resolvedSrc)
	}
	if ref.PreviewWidth != 192 || ref.PreviewHeight != 384 {
		t.Fatalf("preview size = %dx%d, want 192x384", ref.PreviewWidth, ref.PreviewHeight)
	}

	handler := svc.MediaHandler(http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodGet, ref.PreviewURL, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/avif" {
		t.Fatalf("preview content-type = %q", got)
	}
	if got, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition")); err != nil || got != "inline" || !strings.HasSuffix(params["filename"], "-source.avif") {
		t.Fatalf("imported preview content-disposition = %q (parsed=%q %+v err=%v)", rec.Header().Get("Content-Disposition"), got, params, err)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty imported preview body")
	}

	req = httptest.NewRequest(http.MethodGet, ref.FullURL, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("full status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("full content-type = %q", got)
	}
	if got, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition")); err != nil || got != "inline" || params["filename"] != "source.png" {
		t.Fatalf("imported full content-disposition = %q (parsed=%q %+v err=%v)", rec.Header().Get("Content-Disposition"), got, params, err)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("empty imported full body")
	}
}

func TestRegisterMediaAssetRebuildsMissingThumb(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root, err := defaultOutputDir()
	if err != nil {
		t.Fatal(err)
	}
	imagesDir := imagesSubdir(root)
	if err := os.MkdirAll(imagesDir, secureDirMode); err != nil {
		t.Fatal(err)
	}
	fullPath := filepath.Join(imagesDir, "rebuilt.png")
	f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, secureFileMode)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 640, 320))
	for y := 0; y < 320; y++ {
		for x := 0; x < 640; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 160, A: 255})
		}
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	svc := NewService()
	ref, err := svc.RegisterMediaAsset(fullPath, filepath.Join(thumbsSubdir(root), "rebuilt.avif"))
	if err != nil {
		t.Fatal(err)
	}
	if ref.ThumbPath == "" {
		t.Fatalf("expected rebuilt thumb path in ref: %+v", ref)
	}
	if _, err := os.Stat(ref.ThumbPath); err != nil {
		t.Fatalf("expected rebuilt thumb to exist: %v", err)
	}
	if ref.PreviewWidth != 384 || ref.PreviewHeight != 192 {
		t.Fatalf("rebuilt preview size = %dx%d, want 384x192", ref.PreviewWidth, ref.PreviewHeight)
	}
}
