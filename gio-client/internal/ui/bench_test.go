package ui

import (
	"fmt"
	"image"
	"image-studio/gio-client/internal/kernel"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	sharedCompat "image-studio/shared/compat"
)

func benchmarkHistoryItems(total int) []sharedCompat.HistoryItem {
	items := make([]sharedCompat.HistoryItem, 0, total)
	now := time.Date(2026, time.June, 4, 12, 0, 0, 0, time.Local)
	for i := 0; i < total; i++ {
		mode := "generate"
		if i%4 == 0 {
			mode = "edit"
		}
		promptGroup := i % 120
		items = append(items, sharedCompat.HistoryItem{
			ID:            fmt.Sprintf("hist-%05d", i),
			Prompt:        fmt.Sprintf("prompt group %03d cinematic scene", promptGroup),
			RevisedPrompt: fmt.Sprintf("revised prompt group %03d", promptGroup),
			Mode:          mode,
			Size:          "1024x1024",
			Quality:       "high",
			CreatedAt:     now.Add(-time.Duration(i%480) * time.Minute).UnixMilli(),
			BatchIndex:    i % 4,
			SavedPath:     fmt.Sprintf("/tmp/generated-%05d.png", i),
		})
	}
	return items
}

func legacyShortPrompt(prompt string) string {
	prompt = strings.Join(strings.Fields(prompt), " ")
	if len([]rune(prompt)) <= 40 {
		return prompt
	}
	runes := []rune(prompt)
	return string(runes[:40]) + "..."
}

func benchmarkAppWithHistory(total int) *App {
	app := &App{
		historyModeFilter:         "all",
		historyDateFilter:         "all",
		historyTimelineModeFilter: "all",
		historyTimelineDateFilter: "all",
	}
	app.mu.Lock()
	app.logs = make([]string, 240)
	for i := range app.logs {
		app.logs[i] = fmt.Sprintf("12:00:%02d benchmark log line %03d", i%60, i)
	}
	app.profiles = []sharedCompat.UpstreamProfile{
		{ID: "p1", Name: "配置1"},
		{ID: "p2", Name: "配置2"},
	}
	app.promptHistory = []string{"prompt a", "prompt b", "prompt c"}
	app.presets = []sharedCompat.Preset{
		{Name: "默认", BatchCount: 1},
		{Name: "批量", BatchCount: 4},
	}
	app.batchResultIDs = []string{"hist-00000", "hist-00001", "hist-00002", "hist-00003"}
	app.setHistoryLocked(benchmarkHistoryItems(total))
	app.mu.Unlock()
	return app
}

func writeSizedSolidBenchmarkPNG(b *testing.B, path string, width int, height int, fill color.NRGBA) {
	b.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		b.Fatalf("create benchmark png: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		b.Fatalf("encode benchmark png: %v", err)
	}
}

func BenchmarkReadSnapshotLargeHistoryCold(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.mu.Lock()
		app.snapshotReady = false
		app.mu.Unlock()
		_ = app.readSnapshot()
	}
}

func BenchmarkReadSnapshotLargeHistoryCached(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	_ = app.readSnapshot()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = app.readSnapshot()
	}
}

func benchmarkDesktopFrame(b *testing.B, reducedEffects bool) {
	b.Helper()
	b.Setenv("HOME", b.TempDir())
	app := New()
	app.mu.Lock()
	app.reducedEffects = reducedEffects
	app.history = benchmarkHistoryItems(18)
	app.historyRev++
	app.snapshotReady = false
	app.mu.Unlock()

	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Constraints: layout.Exact(image.Pt(1920, 1080)),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Now:         time.Now(),
	}
	app.layout(gtx)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ops.Reset()
		gtx.Now = gtx.Now.Add(time.Second / 60)
		app.layout(gtx)
	}
}

func BenchmarkDesktopFrameStandardEffects(b *testing.B) {
	benchmarkDesktopFrame(b, false)
}

func BenchmarkDesktopFrameReducedEffects(b *testing.B) {
	benchmarkDesktopFrame(b, true)
}

func BenchmarkHistoryPanelDataLargeHistoryCold(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	history := app.readSnapshot().History
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.mu.Lock()
		app.historyRev++
		app.mu.Unlock()
		_ = app.historyPanelData(history)
	}
}

func BenchmarkHistoryPanelDataLargeHistoryCached(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	history := app.readSnapshot().History
	_ = app.historyPanelData(history)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = app.historyPanelData(history)
	}
}

func BenchmarkHistoryTimelineDataLargeHistoryCold(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	history := app.readSnapshot().History
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.mu.Lock()
		app.historyRev++
		app.mu.Unlock()
		_ = app.historyTimelineData(history)
	}
}

func BenchmarkHistoryTimelineDataLargeHistoryCached(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	history := app.readSnapshot().History
	_ = app.historyTimelineData(history)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = app.historyTimelineData(history)
	}
}

func BenchmarkHistoryTimelineModalPathColdCurrent(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	app.selectedHistoryID = "hist-01000"
	history := app.readSnapshot().History
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.mu.Lock()
		app.historyRev++
		app.mu.Unlock()
		data := app.historyTimelineData(history)
		_ = data.selectedGroupKey
	}
}

func BenchmarkHistoryTimelineModalPathColdLegacy(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	app.selectedHistoryID = "hist-01000"
	history := app.readSnapshot().History
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.mu.Lock()
		app.historyRev++
		app.mu.Unlock()
		_ = app.historyTimelineData(history)
		_ = app.promptGroupKeyForHistoryItem(history, app.selectedHistoryID)
	}
}

func BenchmarkHistoryResultsSelectedGroupKeyCurrent(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	history := app.readSnapshot().History
	data := app.historyPanelData(history)
	selectedID := data.entries[0].Group.Items[0].ID
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = promptGroupKeyForEntries(data.entries, selectedID)
	}
}

func BenchmarkHistoryResultsSelectedGroupKeyLegacyCold(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	history := app.readSnapshot().History
	data := app.historyPanelData(history)
	selectedID := data.entries[0].Group.Items[0].ID
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.mu.Lock()
		app.historyRev++
		app.mu.Unlock()
		_ = app.promptGroupKeyForHistoryItem(history, selectedID)
	}
}

func BenchmarkHistoryResultsCompareStateCurrent(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	history := app.readSnapshot().History
	data := app.historyPanelData(history)
	visible := data.entries
	if len(visible) > 18 {
		visible = visible[:18]
	}
	compareID := visible[0].Group.Representative.ID
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, entry := range visible {
			if entry.Kind == "group" {
				_ = compareItemActive(entry.Group.Representative.ID, compareID)
				continue
			}
			_ = compareItemActive(entry.Item.ID, compareID)
		}
	}
}

func BenchmarkHistoryResultsCompareStateLegacy(b *testing.B) {
	app := benchmarkAppWithHistory(5000)
	history := app.readSnapshot().History
	data := app.historyPanelData(history)
	visible := data.entries
	if len(visible) > 18 {
		visible = visible[:18]
	}
	item := visible[0].Group.Representative
	app.compare = resultState{Item: item}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, entry := range visible {
			if entry.Kind == "group" {
				_ = app.isCompareItem(entry.Group.Representative)
				continue
			}
			_ = app.isCompareItem(*entry.Item)
		}
	}
}

func legacyHistoryImagePaths(item sharedCompat.HistoryItem, preferThumb bool) []string {
	thumbPath := strings.TrimSpace(item.ThumbPath)
	savedPath := strings.TrimSpace(item.SavedPath)
	paths := make([]string, 0, 2)
	if preferThumb {
		if thumbPath != "" {
			paths = append(paths, thumbPath)
		}
		if savedPath != "" && savedPath != thumbPath {
			paths = append(paths, savedPath)
		}
		return paths
	}
	if savedPath != "" {
		paths = append(paths, savedPath)
	}
	if thumbPath != "" && thumbPath != savedPath {
		paths = append(paths, thumbPath)
	}
	return paths
}

func legacyHistoryImageCacheKey(item sharedCompat.HistoryItem, preferThumb bool) string {
	if paths := legacyHistoryImagePaths(item, preferThumb); len(paths) > 0 {
		prefix := "history-full:"
		if preferThumb {
			prefix = "history-thumb:"
		}
		return prefix + strings.Join(paths, "|")
	}
	mode := "full"
	if preferThumb {
		mode = "thumb"
	}
	if strings.TrimSpace(item.ID) != "" {
		return "history:" + mode + ":" + item.ID
	}
	if strings.TrimSpace(item.ImageB64) != "" {
		return "history:" + mode + ":inline"
	}
	return "history:" + mode + ":missing"
}

func legacyHistoryImageDisplayCacheKey(item sharedCompat.HistoryItem, maxDimension int) string {
	return legacyHistoryImageCacheKey(item, true) + ":display:" + strconv.Itoa(maxDimension)
}

func BenchmarkHistoryImageDisplayCacheKeyCurrent(b *testing.B) {
	item := sharedCompat.HistoryItem{
		ID:        "hist-1",
		SavedPath: "/tmp/generated.png",
		ThumbPath: "/tmp/generated-thumb.png",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = historyImageDisplayCacheKey(item, 96)
	}
}

func BenchmarkHistoryImageDisplayCacheKeyLegacy(b *testing.B) {
	item := sharedCompat.HistoryItem{
		ID:        "hist-1",
		SavedPath: "/tmp/generated.png",
		ThumbPath: "/tmp/generated-thumb.png",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = legacyHistoryImageDisplayCacheKey(item, 96)
	}
}

func legacyHistoryItemDisplayCacheKey(item sharedCompat.HistoryItem) string {
	if id := strings.TrimSpace(item.ID); id != "" {
		return "id:" + id
	}
	return strings.Join([]string{
		strings.TrimSpace(item.SavedPath),
		strings.TrimSpace(item.RawPath),
		strconv.FormatInt(item.CreatedAt, 10),
		strings.TrimSpace(item.Prompt),
		strings.TrimSpace(item.RevisedPrompt),
		strings.TrimSpace(item.Mode),
		strings.TrimSpace(item.Size),
		strings.TrimSpace(item.Quality),
		strings.TrimSpace(item.StyleTag),
		detailValue(item.Seed),
		detailValue(item.ElapsedSec),
		strings.TrimSpace(item.OutputFormat),
	}, "\x00")
}

func BenchmarkHistoryItemDisplayCacheKeyCurrent(b *testing.B) {
	item := sharedCompat.HistoryItem{
		SavedPath:     "/tmp/generated.png",
		RawPath:       "/tmp/generated.json",
		CreatedAt:     1749038400000,
		Prompt:        "prompt group 001 cinematic scene",
		RevisedPrompt: "revised prompt group 001",
		Mode:          "generate",
		Size:          "1024x1024",
		Quality:       "high",
		StyleTag:      "anime",
		Seed:          42,
		ElapsedSec:    6.8,
		OutputFormat:  "png",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = historyItemDisplayCacheKey(item)
	}
}

func legacyHistoryMetaText(item sharedCompat.HistoryItem) string {
	mode := "文生图"
	if item.Mode == "edit" {
		mode = "图生图"
	}
	format := strings.ToUpper(strings.TrimSpace(item.OutputFormat))
	style := ""
	if strings.TrimSpace(item.StyleTag) != "" {
		style = "#" + styleChoiceLabel(item.StyleTag)
	}
	return strings.Join(compactNonEmpty([]string{mode, sizeDisplayLabel(item.Size), qualityDisplayLabel(item.Quality), style, format}), " · ")
}

func legacyHistoryRailMetaText(item sharedCompat.HistoryItem) string {
	style := ""
	if strings.TrimSpace(item.StyleTag) != "" {
		style = "#" + styleChoiceLabel(item.StyleTag)
	}
	return strings.Join(compactNonEmpty([]string{sizeDisplayLabel(item.Size), qualityDisplayLabel(item.Quality), style}), " · ")
}

func legacyHistoryMetaBadgeItems(item sharedCompat.HistoryItem) []string {
	style := ""
	if strings.TrimSpace(item.StyleTag) != "" {
		style = "#" + styleChoiceLabel(item.StyleTag)
	}
	return compactNonEmpty([]string{sizeDisplayLabel(item.Size), qualityDisplayLabel(item.Quality), style})
}

func legacyStatusBarMetaBadgeItems(item sharedCompat.HistoryItem) []string {
	items := []string{
		sizeDisplayLabel(item.Size),
		qualityDisplayLabel(item.Quality),
	}
	if item.ElapsedSec > 0 {
		items = append(items, detailValue(item.ElapsedSec)+"s")
	}
	if item.Seed != 0 {
		items = append(items, "seed "+detailValue(item.Seed))
	}
	if strings.TrimSpace(item.StyleTag) != "" {
		items = append(items, "#"+styleChoiceLabel(item.StyleTag))
	}
	return compactNonEmpty(items)
}

func legacyFormatHistoryClock(createdAt int64) string {
	if createdAt <= 0 {
		return ""
	}
	return time.UnixMilli(createdAt).Format("15:04")
}

func legacyFormatHistoryClockPrecise(createdAt int64) string {
	if createdAt <= 0 {
		return ""
	}
	return time.UnixMilli(createdAt).Format("15:04:05")
}

func legacyBuildHistoryItemDisplay(item sharedCompat.HistoryItem) historyItemDisplay {
	return historyItemDisplay{
		ShortPrompt:      shortPrompt(item.Prompt),
		MetaBadges:       legacyHistoryMetaBadgeItems(item),
		StatusMetaBadges: legacyStatusBarMetaBadgeItems(item),
		Clock:            legacyFormatHistoryClock(item.CreatedAt),
		ClockPrecise:     legacyFormatHistoryClockPrecise(item.CreatedAt),
		RailMetaText:     legacyHistoryRailMetaText(item),
		MetaText:         legacyHistoryMetaText(item),
	}
}

func BenchmarkHistoryItemDisplayCacheKeyLegacy(b *testing.B) {
	item := sharedCompat.HistoryItem{
		SavedPath:     "/tmp/generated.png",
		RawPath:       "/tmp/generated.json",
		CreatedAt:     1749038400000,
		Prompt:        "prompt group 001 cinematic scene",
		RevisedPrompt: "revised prompt group 001",
		Mode:          "generate",
		Size:          "1024x1024",
		Quality:       "high",
		StyleTag:      "anime",
		Seed:          42,
		ElapsedSec:    6.8,
		OutputFormat:  "png",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = legacyHistoryItemDisplayCacheKey(item)
	}
}

func BenchmarkBuildHistoryItemDisplayCurrent(b *testing.B) {
	item := sharedCompat.HistoryItem{
		Prompt:        "prompt group 001 cinematic scene",
		RevisedPrompt: "revised prompt group 001",
		Mode:          "generate",
		Size:          "1024x1024",
		Quality:       "high",
		CreatedAt:     1749038400000,
		StyleTag:      "anime",
		Seed:          42,
		ElapsedSec:    6.8,
		OutputFormat:  "png",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildHistoryItemDisplay(item)
	}
}

func BenchmarkBuildHistoryItemDisplayLegacy(b *testing.B) {
	item := sharedCompat.HistoryItem{
		Prompt:        "prompt group 001 cinematic scene",
		RevisedPrompt: "revised prompt group 001",
		Mode:          "generate",
		Size:          "1024x1024",
		Quality:       "high",
		CreatedAt:     1749038400000,
		StyleTag:      "anime",
		Seed:          42,
		ElapsedSec:    6.8,
		OutputFormat:  "png",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = legacyBuildHistoryItemDisplay(item)
	}
}

func BenchmarkShortPromptCurrent(b *testing.B) {
	prompt := "  cinematic   portrait   of   a traveler   standing in rain-soaked neon street with reflective puddles and distant signage  "
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = shortPrompt(prompt)
	}
}

func BenchmarkShortPromptLegacy(b *testing.B) {
	prompt := "  cinematic   portrait   of   a traveler   standing in rain-soaked neon street with reflective puddles and distant signage  "
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = legacyShortPrompt(prompt)
	}
}

func BenchmarkLoadDisplayHistoryThumbFromBaseCurrent(b *testing.B) {
	dir := b.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	item := sharedCompat.HistoryItem{ID: "hist-large", SavedPath: fullPath}
	sizes := []int{64, 96, 224}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		if _, err := app.loadDisplayHistoryThumb(item, sizes[i%len(sizes)]); err != nil {
			b.Fatalf("loadDisplayHistoryThumb: %v", err)
		}
	}
}

func BenchmarkLoadDisplayHistoryThumbFromBaseLegacy(b *testing.B) {
	dir := b.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	item := sharedCompat.HistoryItem{ID: "hist-large", SavedPath: fullPath}
	sizes := []int{64, 96, 224}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		base, err := app.cachedHistoryImage(item, true)
		if err != nil {
			b.Fatalf("cachedHistoryImage: %v", err)
		}
		if got := resizedCachedImage(base, sizes[i%len(sizes)]); got.Image == nil {
			b.Fatalf("resizedCachedImage returned nil image")
		}
	}
}

func BenchmarkLoadDisplayHistoryThumbSmallOnlyCurrent(b *testing.B) {
	dir := b.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	item := sharedCompat.HistoryItem{ID: "hist-large", SavedPath: fullPath}
	sizes := []int{64, 96, 160}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		if _, err := app.loadDisplayHistoryThumb(item, sizes[i%len(sizes)]); err != nil {
			b.Fatalf("loadDisplayHistoryThumb: %v", err)
		}
	}
}

func BenchmarkLoadDisplayHistoryThumbSmallOnlyLegacy(b *testing.B) {
	dir := b.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	item := sharedCompat.HistoryItem{ID: "hist-large", SavedPath: fullPath}
	sizes := []int{64, 96, 160}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		base, err := app.cachedHistoryImage(item, true)
		if err != nil {
			b.Fatalf("cachedHistoryImage: %v", err)
		}
		if got := resizedCachedImage(base, sizes[i%len(sizes)]); got.Image == nil {
			b.Fatalf("resizedCachedImage returned nil image")
		}
	}
}

func BenchmarkLoadDisplayHistoryThumbRowOnlyCurrent(b *testing.B) {
	dir := b.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	item := sharedCompat.HistoryItem{ID: "hist-large", SavedPath: fullPath}
	sizes := []int{48, 58}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		if _, err := app.loadDisplayHistoryThumb(item, sizes[i%len(sizes)]); err != nil {
			b.Fatalf("loadDisplayHistoryThumb: %v", err)
		}
	}
}

func BenchmarkLoadDisplayHistoryThumbRowOnlyLegacy(b *testing.B) {
	dir := b.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	item := sharedCompat.HistoryItem{ID: "hist-large", SavedPath: fullPath}
	sizes := []int{48, 58}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		base, err := app.cachedHistoryDisplayBase(item, historyThumbDisplayBaseTinyDimension)
		if err != nil {
			b.Fatalf("cachedHistoryDisplayBase: %v", err)
		}
		if got := resizedCachedImage(base, sizes[i%len(sizes)]); got.Image == nil {
			b.Fatalf("resizedCachedImage returned nil image")
		}
	}
}

func BenchmarkCachedImageForPathThumbMultiSizeCurrent(b *testing.B) {
	dir := b.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	app := &App{imageCache: map[string]cachedImage{}}
	sizes := []int{64, 96, 224}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		for _, size := range sizes {
			if _, err := app.cachedImageForPathThumb(fullPath, size); err != nil {
				b.Fatalf("cachedImageForPathThumb(%d): %v", size, err)
			}
		}
	}
}

func BenchmarkCachedImageForPathThumbMultiSizeLegacy(b *testing.B) {
	dir := b.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	sizes := []int{64, 96, 224}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, size := range sizes {
			if _, err := (&App{}).loadPathThumbUncached(fullPath, size); err != nil {
				b.Fatalf("loadPathThumbUncached(%d): %v", size, err)
			}
		}
	}
}

func BenchmarkCachedImageForPathThumbConcurrentMultiSizeCurrent(b *testing.B) {
	dir := b.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	sizes := []int{64, 96, 224, 64, 96, 224}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app := &App{imageCache: map[string]cachedImage{}}
		var wg sync.WaitGroup
		wg.Add(len(sizes))
		for _, size := range sizes {
			size := size
			go func() {
				defer wg.Done()
				if _, err := app.cachedImageForPathThumb(fullPath, size); err != nil {
					b.Errorf("cachedImageForPathThumb(%d): %v", size, err)
				}
			}()
		}
		wg.Wait()
	}
}

func BenchmarkCachedImageForPathThumbConcurrentMultiSizeLegacy(b *testing.B) {
	dir := b.TempDir()
	fullPath := filepath.Join(dir, "source-large.png")
	writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: 0x55, G: 0x99, B: 0xdd, A: 0xff})

	sizes := []int{64, 96, 224, 64, 96, 224}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(len(sizes))
		for _, size := range sizes {
			size := size
			go func() {
				defer wg.Done()
				if _, err := (&App{}).loadPathThumbUncached(fullPath, size); err != nil {
					b.Errorf("loadPathThumbUncached(%d): %v", size, err)
				}
			}()
		}
		wg.Wait()
	}
}

func BenchmarkSourceStripThumbBatchCurrent(b *testing.B) {
	dir := b.TempDir()
	b.Setenv("HOME", dir)
	paths := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		path := filepath.Join(dir, fmt.Sprintf("source-%d.png", i))
		writeSizedSolidBenchmarkPNG(b, path, 1800, 1200, color.NRGBA{R: uint8(0x50 + i), G: 0x99, B: 0xdd, A: 0xff})
		paths = append(paths, path)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app := &App{imageCache: map[string]cachedImage{}}
		for _, path := range paths {
			if _, err := app.loadDisplayPathThumb(path, 48); err != nil {
				b.Fatalf("loadDisplayPathThumb(48): %v", err)
			}
		}
	}
}

func BenchmarkSourceStripThumbBatchLegacy(b *testing.B) {
	dir := b.TempDir()
	b.Setenv("HOME", dir)
	paths := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		path := filepath.Join(dir, fmt.Sprintf("source-%d.png", i))
		writeSizedSolidBenchmarkPNG(b, path, 1800, 1200, color.NRGBA{R: uint8(0x50 + i), G: 0x99, B: 0xdd, A: 0xff})
		paths = append(paths, path)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app := &App{imageCache: map[string]cachedImage{}}
		for _, path := range paths {
			if _, err := app.cachedImageForPathThumb(path, 48); err != nil {
				b.Fatalf("cachedImageForPathThumb(48): %v", err)
			}
		}
	}
}

func benchmarkHistoryThumbBatchItems(b *testing.B, withThumbs bool) []sharedCompat.HistoryItem {
	dir := b.TempDir()
	items := make([]sharedCompat.HistoryItem, 0, 18)
	for i := 0; i < 18; i++ {
		fullPath := filepath.Join(dir, fmt.Sprintf("history-%02d.png", i))
		writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: uint8(0x40 + i), G: 0x88, B: 0xcc, A: 0xff})
		item := sharedCompat.HistoryItem{
			ID:        fmt.Sprintf("hist-%02d", i),
			SavedPath: fullPath,
		}
		if withThumbs {
			thumbPath, err := kernel.EnsureThumbForPath(fullPath)
			if err != nil {
				b.Fatalf("EnsureThumbForPath(%q): %v", fullPath, err)
			}
			item.ThumbPath = thumbPath
		}
		items = append(items, item)
	}
	return items
}

func BenchmarkHistoryVisibleThumbBatchWithThumbPathCurrent(b *testing.B) {
	items := benchmarkHistoryThumbBatchItems(b, true)
	sizes := []int{48, 58, 88, 98, 104, 118, 152, 48, 58, 88, 98, 104, 118, 152, 48, 58, 88, 98}
	app := &App{imageCache: map[string]cachedImage{}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		for idx, item := range items {
			if _, err := app.loadDisplayHistoryThumb(item, sizes[idx]); err != nil {
				b.Fatalf("loadDisplayHistoryThumb(%d): %v", sizes[idx], err)
			}
		}
	}
}

func BenchmarkHistoryVisibleThumbBatchWithoutThumbPathCurrent(b *testing.B) {
	items := benchmarkHistoryThumbBatchItems(b, false)
	sizes := []int{48, 58, 88, 98, 104, 118, 152, 48, 58, 88, 98, 104, 118, 152, 48, 58, 88, 98}
	app := &App{imageCache: map[string]cachedImage{}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		for idx, item := range items {
			if _, err := app.loadDisplayHistoryThumb(item, sizes[idx]); err != nil {
				b.Fatalf("loadDisplayHistoryThumb(%d): %v", sizes[idx], err)
			}
		}
	}
}

func BenchmarkHistoryVisibleThumbBatchWithPreviewPathCurrent(b *testing.B) {
	dir := b.TempDir()
	b.Setenv("HOME", dir)
	items := make([]sharedCompat.HistoryItem, 0, 18)
	for i := 0; i < 18; i++ {
		fullPath := filepath.Join(dir, fmt.Sprintf("history-preview-%02d.png", i))
		writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: uint8(0x50 + i), G: 0x88, B: 0xcc, A: 0xff})
		previewPath, err := ensureManagedSourcePreview(fullPath, historyPreviewPathMaxDimension)
		if err != nil {
			b.Fatalf("ensureManagedSourcePreview(%q): %v", fullPath, err)
		}
		item := sharedCompat.HistoryItem{
			ID:          fmt.Sprintf("hist-prev-%02d", i),
			SavedPath:   fullPath,
			PreviewPath: previewPath,
		}
		items = append(items, item)
	}
	sizes := []int{48, 58, 88, 98, 104, 118, 152, 48, 58, 88, 98, 104, 118, 152, 48, 58, 88, 98}
	app := &App{imageCache: map[string]cachedImage{}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		for idx, item := range items {
			if _, err := app.loadDisplayHistoryThumb(item, sizes[idx]); err != nil {
				b.Fatalf("loadDisplayHistoryThumb(%d): %v", sizes[idx], err)
			}
		}
	}
}

func BenchmarkHistoryVisibleThumbBatchWithPreviewAndThumbPathCurrent(b *testing.B) {
	dir := b.TempDir()
	b.Setenv("HOME", dir)
	items := make([]sharedCompat.HistoryItem, 0, 18)
	for i := 0; i < 18; i++ {
		fullPath := filepath.Join(dir, fmt.Sprintf("history-preview-thumb-%02d.png", i))
		writeSizedSolidBenchmarkPNG(b, fullPath, 1800, 1200, color.NRGBA{R: uint8(0x60 + i), G: 0x88, B: 0xcc, A: 0xff})
		previewPath, err := ensureManagedSourcePreview(fullPath, historyPreviewPathMaxDimension)
		if err != nil {
			b.Fatalf("ensureManagedSourcePreview(%q): %v", fullPath, err)
		}
		thumbPath, err := kernel.EnsureThumbForPath(fullPath)
		if err != nil {
			b.Fatalf("EnsureThumbForPath(%q): %v", fullPath, err)
		}
		item := sharedCompat.HistoryItem{
			ID:          fmt.Sprintf("hist-prev-thumb-%02d", i),
			SavedPath:   fullPath,
			PreviewPath: previewPath,
			ThumbPath:   thumbPath,
		}
		items = append(items, item)
	}
	sizes := []int{48, 58, 88, 98, 104, 118, 152, 48, 58, 88, 98, 104, 118, 152, 48, 58, 88, 98}
	app := &App{imageCache: map[string]cachedImage{}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		app.imageCache = map[string]cachedImage{}
		for idx, item := range items {
			if _, err := app.loadDisplayHistoryThumb(item, sizes[idx]); err != nil {
				b.Fatalf("loadDisplayHistoryThumb(%d): %v", sizes[idx], err)
			}
		}
	}
}

func benchmarkThumbDecodeWork() {
	var sum uint64
	for i := 0; i < 512; i++ {
		sum ^= uint64(i * 3)
	}
	if sum == ^uint64(0) {
		panic("unreachable")
	}
}

func BenchmarkThumbDecodeDispatchCurrent(b *testing.B) {
	const total = 64
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(total)
		for j := 0; j < total; j++ {
			runThumbDecodeAsync(func() {
				benchmarkThumbDecodeWork()
				wg.Done()
			})
		}
		wg.Wait()
	}
}

func BenchmarkThumbDecodeDispatchLegacy(b *testing.B) {
	const total = 64
	limiter := make(chan struct{}, 4)
	dispatch := func(fn func()) {
		go func() {
			limiter <- struct{}{}
			defer func() { <-limiter }()
			fn()
		}()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(total)
		for j := 0; j < total; j++ {
			dispatch(func() {
				benchmarkThumbDecodeWork()
				wg.Done()
			})
		}
		wg.Wait()
	}
}
