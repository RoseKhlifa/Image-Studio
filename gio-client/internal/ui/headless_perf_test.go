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

	sharedCompat "image-studio/shared/compat"
)

func writeHeadlessPerfPNG(t *testing.T, path string, width int, height int, fill color.NRGBA) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer file.Close()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
}

func encodeHeadlessPerfPNGBase64(t *testing.T, width int, height int, fill color.NRGBA) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode virtual png: %v", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestBuildHeadlessHistoryPerfReport(t *testing.T) {
	dir := t.TempDir()
	items := make([]sharedCompat.HistoryItem, 0, 3)
	for i := 0; i < 3; i++ {
		savedPath := filepath.Join(dir, "images", "item-"+string(rune('a'+i))+".png")
		previewPath := filepath.Join(dir, "previews", "item-"+string(rune('a'+i))+".png")
		thumbPath := filepath.Join(dir, "thumbs", "item-"+string(rune('a'+i))+".png")
		writeHeadlessPerfPNG(t, savedPath, 512, 384, color.NRGBA{R: uint8(0x30 + i), G: 0x66, B: 0xaa, A: 0xff})
		writeHeadlessPerfPNG(t, previewPath, 128, 96, color.NRGBA{R: uint8(0x40 + i), G: 0x77, B: 0xbb, A: 0xff})
		writeHeadlessPerfPNG(t, thumbPath, 256, 192, color.NRGBA{R: uint8(0x50 + i), G: 0x88, B: 0xcc, A: 0xff})
		items = append(items, sharedCompat.HistoryItem{
			ID:          "item-" + string(rune('a'+i)),
			Prompt:      "prompt group alpha",
			Mode:        "generate",
			Size:        "1024x1024",
			Quality:     "high",
			CreatedAt:   1780657541382 + int64(i),
			SavedPath:   savedPath,
			PreviewPath: previewPath,
			ThumbPath:   thumbPath,
		})
	}

	report := BuildHeadlessHistoryPerfReport(items)
	if report.HistoryCount != 3 {
		t.Fatalf("history_count=%d want 3", report.HistoryCount)
	}
	if report.HistoryPanelFiltered != 3 || report.HistoryTimelineFiltered != 3 {
		t.Fatalf("report=%+v", report)
	}
	if report.VisibleThumbEligible != 3 {
		t.Fatalf("report=%+v", report)
	}
	if report.VisibleThumbColdErrors != 0 || report.VisibleThumbWarmErrors != 0 || report.VisibleThumbStartupPrewarmFailed != 0 || report.VisibleThumbAfterStartupPrewarmFail != 0 || report.VisibleThumbPrewarmFailed != 0 || report.VisibleThumbAfterPrewarmFail != 0 {
		t.Fatalf("report=%+v", report)
	}
	if report.VisibleThumbStartupPrewarmLoaded == 0 {
		t.Fatalf("report=%+v want startup prewarm load", report)
	}
	if report.VisibleThumbPrewarmLoaded != 3 {
		t.Fatalf("report=%+v", report)
	}
	if report.HistoryPanelColdMs < 0 || report.HistoryTimelineColdMs < 0 || report.VisibleThumbColdMs < 0 || report.VisibleThumbWarmMs < 0 || report.VisibleThumbStartupPrewarmMs < 0 || report.VisibleThumbAfterStartupPrewarmMs < 0 || report.VisibleThumbPrewarmMs < 0 || report.VisibleThumbAfterPrewarmMs < 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestBuildHeadlessResultPerfReport(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	savedPath := filepath.Join(dir, "images", "latest.png")
	previewPath := filepath.Join(dir, "previews", "latest.png")
	thumbPath := filepath.Join(dir, "thumbs", "latest.png")
	writeHeadlessPerfPNG(t, savedPath, 2200, 1600, color.NRGBA{R: 0x44, G: 0x66, B: 0x88, A: 0xff})
	writeHeadlessPerfPNG(t, previewPath, 128, 96, color.NRGBA{R: 0x55, G: 0x77, B: 0x99, A: 0xff})
	writeHeadlessPerfPNG(t, thumbPath, 256, 192, color.NRGBA{R: 0x66, G: 0x88, B: 0xaa, A: 0xff})

	report := BuildHeadlessResultPerfReport([]sharedCompat.HistoryItem{{
		ID:          "latest",
		SavedPath:   savedPath,
		PreviewPath: previewPath,
		ThumbPath:   thumbPath,
	}})
	if !report.HasResult {
		t.Fatalf("report=%+v want HasResult", report)
	}
	if report.SavedPath != savedPath {
		t.Fatalf("saved_path=%q want %q", report.SavedPath, savedPath)
	}
	if report.OutputWidth <= 0 || report.OutputHeight <= 0 || report.ReducedOutputWidth <= 0 || report.ReducedOutputHeight <= 0 {
		t.Fatalf("report=%+v", report)
	}
	if !report.ManagedPreviewReady || report.ManagedPreviewPath == "" {
		t.Fatalf("report=%+v", report)
	}
	if report.ColdMs < 0 || report.WarmMs < 0 || report.ReducedColdMs < 0 || report.ReducedWarmMs < 0 || report.ManagedPreviewMs < 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestBuildHeadlessResultPerfReportSupportsVirtualSavedPath(t *testing.T) {
	virtualPath := registerVirtualImage(encodeHeadlessPerfPNGBase64(t, 1200, 800, color.NRGBA{R: 0x77, G: 0x55, B: 0x33, A: 0xff}), "remote-result.png", "png")
	report := BuildHeadlessResultPerfReport([]sharedCompat.HistoryItem{{
		ID:        "latest-virtual",
		SavedPath: virtualPath,
	}})
	if !report.HasResult {
		t.Fatalf("report=%+v want HasResult", report)
	}
	if report.SavedPath != virtualPath {
		t.Fatalf("saved_path=%q want %q", report.SavedPath, virtualPath)
	}
	if report.OutputWidth <= 0 || report.OutputHeight <= 0 {
		t.Fatalf("report=%+v want decoded virtual dimensions", report)
	}
	if report.ManagedPreviewReady || report.ManagedPreviewPath != "" {
		t.Fatalf("report=%+v want no managed preview for virtual saved path", report)
	}
}
