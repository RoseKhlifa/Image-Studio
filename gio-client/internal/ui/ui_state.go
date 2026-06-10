package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/kernel"
	sharedCompat "image-studio/shared/compat"

	"gioui.org/app"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"github.com/yuanhua/image-gptcodex/pkg/client"
)

var errMissingPreview = errors.New("missing preview image")
var thumbDecodeQueue = make(chan func(), 256)
var thumbDecodeWorkersOnce sync.Once
var thumbDecodeBusy int32
var thumbDecodeBusyPeak int32
var thumbDecodeQueuePeak int32
var thumbDisplayRequests uint64
var thumbDisplayCacheHits uint64
var thumbDisplayLoadsQueued uint64
var historyThumbSourcePreview uint64
var historyThumbSourceThumb uint64
var historyThumbSourceSaved uint64
var canvasDisplaySourceManagedPreview uint64
var canvasDisplaySourcePathThumb uint64
var canvasDisplaySourceHistoryScaled uint64
var canvasDisplaySourceInline uint64

const reducedEffectsThumbMaxDimension = 256
const canvasDisplayMaxDimension = 2048
const reducedEffectsCanvasDisplayMaxDimension = 1536
const historyCacheRetention = 96
const thumbCacheBucket = 32
const thumbCacheMinDimension = 64
const pathThumbReuseBaseMinDimension = 224
const pathThumbDirectDecodeMaxDimension = 64
const historyThumbDisplayBaseMiniDimension = 64
const historyThumbDisplayBaseTinyDimension = 112
const historyThumbDisplayBaseMediumDimension = 128
const historyThumbDisplayBaseSmallDimension = 160
const historyPreviewPathMaxDimension = 128
const historyThumbBackfillLimit = 48
const historyPreviewWarmLimit = 48
const historyBackfillStartupDelay = 1500 * time.Millisecond
const historyBackfillChainDelay = 250 * time.Millisecond
const historyBackfillBusyDelay = 2 * time.Second
const historyThumbPrewarmDelay = 25 * time.Millisecond
const historyThumbStartupSyncPrewarmCount = 8
const historyThumbPrewarmCount = 18

var historyThumbPrewarmDimensions = []int{64, 96, 128, 160}

type historyMediaBackfillUpdate struct {
	ThumbPath   string
	PreviewPath string
}

var probeUpstreamFunc = kernel.ProbeUpstream

func isMissingPreview(err error) bool {
	return errors.Is(err, errMissingPreview)
}

func thumbDecodeWorkerCount() int {
	workers := runtime.GOMAXPROCS(0)
	if workers < 4 {
		return 4
	}
	if workers > 8 {
		return 8
	}
	return workers
}

func runThumbDecodeAsync(fn func()) {
	if fn == nil {
		return
	}
	thumbDecodeWorkersOnce.Do(func() {
		for i := 0; i < thumbDecodeWorkerCount(); i++ {
			go func() {
				for task := range thumbDecodeQueue {
					if task != nil {
						busy := atomic.AddInt32(&thumbDecodeBusy, 1)
						updateThumbDecodePeak(&thumbDecodeBusyPeak, busy)
						task()
						atomic.AddInt32(&thumbDecodeBusy, -1)
					}
				}
			}()
		}
	})
	select {
	case thumbDecodeQueue <- fn:
		updateThumbDecodePeak(&thumbDecodeQueuePeak, int32(len(thumbDecodeQueue)))
	default:
		go func() {
			thumbDecodeQueue <- fn
			updateThumbDecodePeak(&thumbDecodeQueuePeak, int32(len(thumbDecodeQueue)))
		}()
	}
}

func updateThumbDecodePeak(target *int32, value int32) {
	for {
		current := atomic.LoadInt32(target)
		if value <= current {
			return
		}
		if atomic.CompareAndSwapInt32(target, current, value) {
			return
		}
	}
}

func thumbDecodeQueueLen() int {
	return len(thumbDecodeQueue)
}

func thumbDecodeBusyCount() int {
	return int(atomic.LoadInt32(&thumbDecodeBusy))
}

func thumbDecodeQueuePeakCount() int {
	return int(atomic.LoadInt32(&thumbDecodeQueuePeak))
}

func thumbDecodeBusyPeakCount() int {
	return int(atomic.LoadInt32(&thumbDecodeBusyPeak))
}

func thumbDisplayRequestCount() uint64 {
	return atomic.LoadUint64(&thumbDisplayRequests)
}

func thumbDisplayCacheHitCount() uint64 {
	return atomic.LoadUint64(&thumbDisplayCacheHits)
}

func thumbDisplayLoadQueuedCount() uint64 {
	return atomic.LoadUint64(&thumbDisplayLoadsQueued)
}

func historyThumbSourcePreviewCount() uint64 {
	return atomic.LoadUint64(&historyThumbSourcePreview)
}

func historyThumbSourceThumbCount() uint64 {
	return atomic.LoadUint64(&historyThumbSourceThumb)
}

func historyThumbSourceSavedCount() uint64 {
	return atomic.LoadUint64(&historyThumbSourceSaved)
}

func canvasDisplaySourceManagedPreviewCount() uint64 {
	return atomic.LoadUint64(&canvasDisplaySourceManagedPreview)
}

func canvasDisplaySourcePathThumbCount() uint64 {
	return atomic.LoadUint64(&canvasDisplaySourcePathThumb)
}

func canvasDisplaySourceHistoryScaledCount() uint64 {
	return atomic.LoadUint64(&canvasDisplaySourceHistoryScaled)
}

func canvasDisplaySourceInlineCount() uint64 {
	return atomic.LoadUint64(&canvasDisplaySourceInline)
}

func resetThumbDiagnosticsCounters() {
	atomic.StoreInt32(&thumbDecodeBusy, 0)
	atomic.StoreInt32(&thumbDecodeBusyPeak, 0)
	atomic.StoreInt32(&thumbDecodeQueuePeak, 0)
	atomic.StoreUint64(&thumbDisplayRequests, 0)
	atomic.StoreUint64(&thumbDisplayCacheHits, 0)
	atomic.StoreUint64(&thumbDisplayLoadsQueued, 0)
	atomic.StoreUint64(&historyThumbSourcePreview, 0)
	atomic.StoreUint64(&historyThumbSourceThumb, 0)
	atomic.StoreUint64(&historyThumbSourceSaved, 0)
	atomic.StoreUint64(&canvasDisplaySourceManagedPreview, 0)
	atomic.StoreUint64(&canvasDisplaySourcePathThumb, 0)
	atomic.StoreUint64(&canvasDisplaySourceHistoryScaled, 0)
	atomic.StoreUint64(&canvasDisplaySourceInline, 0)
}

func (a *App) effectiveCanvasMaxDimension() int {
	a.mu.Lock()
	reduced := a.reducedEffects
	a.mu.Unlock()
	if reduced {
		return reducedEffectsCanvasDisplayMaxDimension
	}
	return canvasDisplayMaxDimension
}

func (a *App) prepareCanvasDisplayImage(img image.Image) image.Image {
	return downscaleToMaxDimension(img, a.effectiveCanvasMaxDimension())
}

func (a *App) effectiveThumbMaxDimension(maxDimension int) int {
	if maxDimension <= 0 {
		return maxDimension
	}
	a.mu.Lock()
	reduced := a.reducedEffects
	a.mu.Unlock()
	if reduced && maxDimension > reducedEffectsThumbMaxDimension {
		return reducedEffectsThumbMaxDimension
	}
	return maxDimension
}

func normalizeThumbCacheDimension(maxDimension int) int {
	if maxDimension <= 0 {
		return maxDimension
	}
	if maxDimension < thumbCacheMinDimension {
		maxDimension = thumbCacheMinDimension
	}
	if rem := maxDimension % thumbCacheBucket; rem != 0 {
		maxDimension += thumbCacheBucket - rem
	}
	return maxDimension
}

func newestHistoryItem(items []sharedCompat.HistoryItem) (sharedCompat.HistoryItem, bool) {
	if len(items) == 0 {
		return sharedCompat.HistoryItem{}, false
	}
	return items[0], true
}

func historyItemBySavedPath(items []sharedCompat.HistoryItem, savedPath string) (sharedCompat.HistoryItem, bool) {
	savedPath = strings.TrimSpace(savedPath)
	if savedPath == "" {
		return sharedCompat.HistoryItem{}, false
	}
	for _, item := range items {
		if strings.TrimSpace(item.SavedPath) == savedPath {
			return item, true
		}
	}
	return sharedCompat.HistoryItem{}, false
}

func historyItemByID(items []sharedCompat.HistoryItem, id string) (sharedCompat.HistoryItem, bool) {
	id = strings.TrimSpace(id)
	if id == "" {
		return sharedCompat.HistoryItem{}, false
	}
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return sharedCompat.HistoryItem{}, false
}

func historyCounts(items []sharedCompat.HistoryItem) (generate int, edit int) {
	for _, item := range items {
		if item.Mode == "edit" {
			edit++
			continue
		}
		generate++
	}
	return generate, edit
}

func todayHistoryCount(items []sharedCompat.HistoryItem, now time.Time) int {
	start := localDayStart(now)
	count := 0
	for _, item := range items {
		if item.CreatedAt >= start.UnixMilli() {
			count++
		}
	}
	return count
}

func (a *App) profileButton(id string) *widget.Clickable {
	if a.profileButtons == nil {
		a.profileButtons = map[string]*widget.Clickable{}
	}
	if btn, ok := a.profileButtons[id]; ok {
		return btn
	}
	btn := new(widget.Clickable)
	a.profileButtons[id] = btn
	return btn
}

func (a *App) historyButton(id string) *widget.Clickable {
	if a.historyButtons == nil {
		a.historyButtons = map[string]*widget.Clickable{}
	}
	if btn, ok := a.historyButtons[id]; ok {
		return btn
	}
	btn := new(widget.Clickable)
	a.historyButtons[id] = btn
	return btn
}

func (a *App) settingsProfileButton(id string) *widget.Clickable {
	if a.settingsProfileButtons == nil {
		a.settingsProfileButtons = map[string]*widget.Clickable{}
	}
	if btn, ok := a.settingsProfileButtons[id]; ok {
		return btn
	}
	btn := new(widget.Clickable)
	a.settingsProfileButtons[id] = btn
	return btn
}

func (a *App) promptButton(id string) *widget.Clickable {
	if a.promptButtons == nil {
		a.promptButtons = map[string]*widget.Clickable{}
	}
	if btn, ok := a.promptButtons[id]; ok {
		return btn
	}
	btn := new(widget.Clickable)
	a.promptButtons[id] = btn
	return btn
}

func (a *App) sourceButton(id string) *widget.Clickable {
	if a.sourceButtons == nil {
		a.sourceButtons = map[string]*widget.Clickable{}
	}
	if btn, ok := a.sourceButtons[id]; ok {
		return btn
	}
	btn := new(widget.Clickable)
	a.sourceButtons[id] = btn
	return btn
}

func (a *App) historyActionButton(id string) *widget.Clickable {
	if a.historyActionButtons == nil {
		a.historyActionButtons = map[string]*widget.Clickable{}
	}
	if btn, ok := a.historyActionButtons[id]; ok {
		return btn
	}
	btn := new(widget.Clickable)
	a.historyActionButtons[id] = btn
	return btn
}

func (a *App) filteredHistory(items []sharedCompat.HistoryItem) []sharedCompat.HistoryItem {
	return filteredHistoryItems(
		items,
		a.historyQueryInput.Text(),
		a.historyModeFilter,
		a.historyDateFilter,
		time.Now(),
	)
}

func (a *App) filteredTimelineHistory(items []sharedCompat.HistoryItem) []sharedCompat.HistoryItem {
	return filteredHistoryItems(
		items,
		a.historyTimelineQueryInput.Text(),
		a.historyTimelineModeFilter,
		a.historyTimelineDateFilter,
		time.Now(),
	)
}

func (a *App) loadHistoryPreview(item sharedCompat.HistoryItem, addLog bool) error {
	if strings.TrimSpace(item.ID) == "" && strings.TrimSpace(item.SavedPath) == "" && strings.TrimSpace(item.ImageB64) == "" {
		return errMissingPreview
	}
	state := resultState{
		SavedPath:     item.SavedPath,
		RawPath:       item.RawPath,
		RevisedPrompt: item.RevisedPrompt,
		SourceEvent:   "history",
		Item:          item,
		HasItem:       item.ID != "",
	}
	previewImg := a.loadCanvasImmediatePreviewForState(item.SavedPath, state)
	a.mu.Lock()
	state.Image = previewImg
	state.Rev = a.result.Rev + 1
	a.result = state
	a.compare = resultState{Rev: a.compare.Rev + 1}
	a.compareSplitSlider.Value = 0.5
	a.selectedHistoryID = item.ID
	a.status = "已载入历史结果"
	if addLog {
		a.appendLogLocked("载入历史结果: " + shortPrompt(item.Prompt))
	}
	rev := a.result.Rev
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	a.invalidateNow()
	a.startAsyncHistoryResultImageLoad(item, rev)
	return nil
}

func (a *App) imageForHistoryItem(item sharedCompat.HistoryItem) (image.Image, error) {
	return a.imageForHistorySource(item, false)
}

func (a *App) imageForHistoryThumb(item sharedCompat.HistoryItem) (image.Image, error) {
	return a.imageForHistorySource(item, true)
}

func (a *App) imageOpForHistoryItem(item sharedCompat.HistoryItem) (paint.ImageOp, error) {
	return a.imageOpForHistorySource(item, false)
}

func (a *App) imageOpForHistoryThumb(item sharedCompat.HistoryItem) (paint.ImageOp, error) {
	return a.imageOpForHistorySource(item, true)
}

func (a *App) imageForHistorySource(item sharedCompat.HistoryItem, preferThumb bool) (image.Image, error) {
	cached, err := a.cachedHistoryImage(item, preferThumb)
	if err != nil {
		return nil, err
	}
	return cached.Image, nil
}

func (a *App) imageOpForHistorySource(item sharedCompat.HistoryItem, preferThumb bool) (paint.ImageOp, error) {
	cached, err := a.cachedHistoryImage(item, preferThumb)
	if err != nil {
		return paint.ImageOp{}, err
	}
	return cached.Op, nil
}

func (a *App) displayHistoryThumb(item sharedCompat.HistoryItem, maxDimension int) (image.Image, paint.ImageOp) {
	a.ensureHistoryThumbBackfill(item)
	atomic.AddUint64(&thumbDisplayRequests, 1)
	maxDimension = normalizeThumbCacheDimension(a.effectiveThumbMaxDimension(maxDimension))
	cacheKey := historyImageDisplayCacheKey(item, maxDimension)
	if cached, ok := a.getCachedImage(cacheKey); ok {
		if cached.Failed || cached.Loading || cached.Image == nil {
			return nil, paint.ImageOp{}
		}
		atomic.AddUint64(&thumbDisplayCacheHits, 1)
		return cached.Image, cached.Op
	}
	atomic.AddUint64(&thumbDisplayLoadsQueued, 1)
	a.setCachedImage(cacheKey, cachedImage{Loading: true})
	runThumbDecodeAsync(func() {
		cached, err := a.loadDisplayHistoryThumb(item, maxDimension)
		if err != nil {
			a.finishCachedImageIfLoading(cacheKey, cachedImage{Failed: true})
		} else {
			a.finishCachedImageIfLoading(cacheKey, cached)
		}
		a.invalidateSoon(33 * time.Millisecond)
	})
	return nil, paint.ImageOp{}
}

func (a *App) displayPathThumb(path string, maxDimension int) (image.Image, paint.ImageOp) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, paint.ImageOp{}
	}
	atomic.AddUint64(&thumbDisplayRequests, 1)
	maxDimension = normalizeThumbCacheDimension(a.effectiveThumbMaxDimension(maxDimension))
	cacheKey := pathThumbCacheKey(path, maxDimension)
	if cached, ok := a.getCachedImage(cacheKey); ok {
		if cached.Failed || cached.Loading || cached.Image == nil {
			return nil, paint.ImageOp{}
		}
		atomic.AddUint64(&thumbDisplayCacheHits, 1)
		return cached.Image, cached.Op
	}
	atomic.AddUint64(&thumbDisplayLoadsQueued, 1)
	a.setCachedImage(cacheKey, cachedImage{Loading: true})
	runThumbDecodeAsync(func() {
		cached, err := a.loadDisplayPathThumb(path, maxDimension)
		if err != nil {
			a.finishCachedImageIfLoading(cacheKey, cachedImage{Failed: true})
		} else {
			a.finishCachedImageIfLoading(cacheKey, cached)
		}
		a.invalidateSoon(33 * time.Millisecond)
	})
	return nil, paint.ImageOp{}
}

func (a *App) loadDisplayPathThumb(path string, maxDimension int) (cachedImage, error) {
	if maxDimension <= pathThumbDirectDecodeMaxDimension {
		if previewPath, err := ensureManagedSourcePreview(path, maxDimension); err == nil && strings.TrimSpace(previewPath) != "" {
			if cached, err := a.cachedImageForPath(previewPath); err == nil {
				return resizedCachedImage(cached, maxDimension), nil
			}
		}
		if cached, ok := a.getCachedImage(pathCacheKey(path)); ok && !cached.Loading {
			if cached.Failed {
				return cachedImage{}, errMissingPreview
			}
			return resizedCachedImage(cached, maxDimension), nil
		}
		baseKey := pathThumbCacheKey(path, pathThumbReuseBaseMinDimension)
		if cached, ok := a.getCachedImage(baseKey); ok && !cached.Loading {
			if cached.Failed {
				return cachedImage{}, errMissingPreview
			}
			return resizedCachedImage(cached, maxDimension), nil
		}
		img, err := a.loadPathThumbUncached(path, maxDimension)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return cachedImage{}, fmt.Errorf("%w: %v", errMissingPreview, err)
			}
			return cachedImage{}, err
		}
		return cachedImage{
			Image: img,
			Op:    paint.NewImageOp(img),
		}, nil
	}
	return a.cachedImageForPathThumb(path, maxDimension)
}

func managedSourcePreviewPath(sourcePath string, maxDimension int) (string, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", errMissingPreview
	}
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", err
	}
	root, err := gioCompat.StableDataRoot()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(
		absSource + "\x00" +
			strconv.Itoa(maxDimension),
	))
	name := hex.EncodeToString(sum[:]) + ".png"
	return filepath.Join(root, "source-previews", name), nil
}

func ensureManagedSourcePreview(sourcePath string, maxDimension int) (string, error) {
	previewPath, err := managedSourcePreviewPath(sourcePath, maxDimension)
	if err != nil {
		return "", err
	}
	sourcePath = strings.TrimSpace(sourcePath)
	sourceInfo, sourceErr := os.Stat(sourcePath)
	if previewInfo, err := os.Stat(previewPath); err == nil && !previewInfo.IsDir() {
		if sourceErr != nil || !sourceInfo.ModTime().After(previewInfo.ModTime()) {
			return previewPath, nil
		}
	}
	if sourceErr != nil {
		return "", sourceErr
	}
	img, err := (&App{}).loadPathThumbUncached(sourcePath, maxDimension)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(previewPath), 0o700); err != nil {
		return "", err
	}
	file, err := os.OpenFile(previewPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(previewPath)
		}
	}()
	if err := png.Encode(file, img); err != nil {
		return "", fmt.Errorf("encode managed source preview: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	ok = true
	return previewPath, nil
}

func (a *App) loadDisplayHistoryThumb(item sharedCompat.HistoryItem, maxDimension int) (cachedImage, error) {
	baseDimension := historyDisplayBaseDimension(maxDimension)
	base, err := a.cachedHistoryDisplayBase(item, baseDimension)
	if err != nil {
		return cachedImage{}, err
	}
	img := base.Image
	if img == nil {
		return cachedImage{}, errMissingPreview
	}
	if maxDimension > 0 {
		scaled := downscaleToMaxDimension(img, maxDimension)
		if scaled != img {
			return cachedImage{
				Image: scaled,
				Op:    paint.NewImageOp(scaled),
			}, nil
		}
	}
	return cachedImage{
		Image: img,
		Op:    base.Op,
	}, nil
}

func historyImageDisplayBaseCacheKey(item sharedCompat.HistoryItem, baseDimension int) string {
	base := historyImageCacheKey(item, true)
	return base + ":display-base:" + strconv.Itoa(baseDimension)
}

func historyDisplayBaseDimension(maxDimension int) int {
	if maxDimension <= historyThumbDisplayBaseMiniDimension {
		return historyThumbDisplayBaseMiniDimension
	}
	if maxDimension <= historyThumbDisplayBaseTinyDimension {
		return historyThumbDisplayBaseTinyDimension
	}
	if maxDimension <= historyThumbDisplayBaseMediumDimension {
		return historyThumbDisplayBaseMediumDimension
	}
	if maxDimension <= historyThumbDisplayBaseSmallDimension {
		return historyThumbDisplayBaseSmallDimension
	}
	return historyThumbFallbackMaxDimension
}

func (a *App) cachedHistoryDisplayBase(item sharedCompat.HistoryItem, baseDimension int) (cachedImage, error) {
	if baseDimension >= historyThumbFallbackMaxDimension {
		return a.cachedHistoryImage(item, true)
	}
	cacheKey := historyImageDisplayBaseCacheKey(item, baseDimension)
	for {
		cached, found, waitCh := a.beginCachedImageLoad(cacheKey)
		if found {
			if cached.Loading {
				<-waitCh
				continue
			}
			if cached.Failed {
				return cachedImage{}, errMissingPreview
			}
			return cached, nil
		}
		if baseDimension <= historyPreviewPathMaxDimension {
			if previewPath := strings.TrimSpace(item.PreviewPath); previewPath != "" {
				if preview, err := a.cachedImageForPath(previewPath); err == nil {
					cached = resizedCachedImage(preview, baseDimension)
					a.completeCachedImageLoad(cacheKey, cached)
					return cached, nil
				}
			}
		}
		if thumbPath := strings.TrimSpace(item.ThumbPath); thumbPath != "" {
			if thumb, err := a.cachedImageForPath(thumbPath); err == nil {
				cached = resizedCachedImage(thumb, baseDimension)
				a.completeCachedImageLoad(cacheKey, cached)
				return cached, nil
			}
		}
		img, err := a.loadHistoryImageWithMax(item, true, baseDimension, false)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, errMissingPreview) {
				a.completeCachedImageLoad(cacheKey, cachedImage{Failed: true})
				return cachedImage{}, fmt.Errorf("%w: %v", errMissingPreview, err)
			}
			a.completeCachedImageLoad(cacheKey, cachedImage{Failed: true})
			return cachedImage{}, err
		}
		cached = cachedImage{
			Image: img,
			Op:    paint.NewImageOp(img),
		}
		a.completeCachedImageLoad(cacheKey, cached)
		return cached, nil
	}
}

func (a *App) cachedHistoryImage(item sharedCompat.HistoryItem, preferThumb bool) (cachedImage, error) {
	cacheKey := historyImageCacheKey(item, preferThumb)
	for {
		cached, found, waitCh := a.beginCachedImageLoad(cacheKey)
		if found {
			if cached.Loading {
				<-waitCh
				continue
			}
			if cached.Failed {
				return cachedImage{}, errMissingPreview
			}
			return cached, nil
		}
		img, err := a.loadHistoryImage(item, preferThumb)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) || errors.Is(err, errMissingPreview) {
				a.completeCachedImageLoad(cacheKey, cachedImage{Failed: true})
				return cachedImage{}, fmt.Errorf("%w: %v", errMissingPreview, err)
			}
			a.completeCachedImageLoad(cacheKey, cachedImage{Failed: true})
			return cachedImage{}, err
		}
		cached = cachedImage{
			Image: img,
			Op:    paint.NewImageOp(img),
		}
		a.completeCachedImageLoad(cacheKey, cached)
		return cached, nil
	}
}

func (a *App) loadHistoryImage(item sharedCompat.HistoryItem, preferThumb bool) (image.Image, error) {
	return a.loadHistoryImageWithMax(item, preferThumb, historyThumbFallbackMaxDimension, true)
}

func (a *App) loadHistoryImageScaled(item sharedCompat.HistoryItem, maxDimension int) (image.Image, error) {
	return a.loadHistoryImageWithMax(item, true, maxDimension, true)
}

func (a *App) loadHistoryImageScaledUncached(item sharedCompat.HistoryItem, maxDimension int) (image.Image, error) {
	return a.loadHistoryImageWithMax(item, true, maxDimension, false)
}

func (a *App) loadHistoryImageWithMax(item sharedCompat.HistoryItem, preferThumb bool, maxDimension int, usePathCache bool) (image.Image, error) {
	if strings.TrimSpace(item.ImageB64) != "" {
		img, err := decodeImageB64(item.ImageB64)
		if err != nil {
			return nil, err
		}
		if preferThumb {
			img = downscaleToMaxDimension(img, maxDimension)
		}
		return img, nil
	}
	thumbPath := strings.TrimSpace(item.ThumbPath)
	previewPath := strings.TrimSpace(item.PreviewPath)
	savedPath := strings.TrimSpace(item.SavedPath)
	loadThumb := func(path string) (image.Image, error) {
		if usePathCache {
			return a.imageForPathThumb(path, maxDimension)
		}
		return a.loadPathThumbUncached(path, maxDimension)
	}
	loadCandidate := func(path string, thumb bool) (image.Image, error) {
		if path == "" {
			return nil, errMissingPreview
		}
		if thumb {
			return loadThumb(path)
		}
		return a.imageForPath(path)
	}
	if preferThumb {
		if maxDimension > 0 && maxDimension <= historyPreviewPathMaxDimension {
			if img, err := loadCandidate(previewPath, true); err == nil {
				atomic.AddUint64(&historyThumbSourcePreview, 1)
				return img, nil
			}
		}
		if img, err := loadCandidate(thumbPath, true); err == nil {
			atomic.AddUint64(&historyThumbSourceThumb, 1)
			return img, nil
		} else if savedPath == "" || savedPath == thumbPath {
			return nil, err
		}
		if img, err := loadCandidate(savedPath, true); err == nil {
			atomic.AddUint64(&historyThumbSourceSaved, 1)
			return img, nil
		} else {
			return nil, err
		}
	}
	if img, err := loadCandidate(savedPath, false); err == nil {
		return img, nil
	} else if thumbPath == "" || thumbPath == savedPath {
		return nil, err
	}
	return loadCandidate(thumbPath, false)
}

func historyImagePaths(item sharedCompat.HistoryItem, preferThumb bool) []string {
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

func historyImageCacheKey(item sharedCompat.HistoryItem, preferThumb bool) string {
	thumbPath := strings.TrimSpace(item.ThumbPath)
	savedPath := strings.TrimSpace(item.SavedPath)
	if preferThumb {
		if thumbPath != "" {
			if savedPath != "" && savedPath != thumbPath {
				return "history-thumb:" + thumbPath + "|" + savedPath
			}
			return "history-thumb:" + thumbPath
		}
		if savedPath != "" {
			return "history-thumb:" + savedPath
		}
	} else {
		if savedPath != "" {
			if thumbPath != "" && thumbPath != savedPath {
				return "history-full:" + savedPath + "|" + thumbPath
			}
			return "history-full:" + savedPath
		}
		if thumbPath != "" {
			return "history-full:" + thumbPath
		}
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

func historyImageDisplayCacheKey(item sharedCompat.HistoryItem, maxDimension int) string {
	base := historyImageCacheKey(item, true)
	return base + ":display:" + strconv.Itoa(maxDimension)
}

func (a *App) imageForPath(path string) (image.Image, error) {
	cached, err := a.cachedImageForPath(path)
	if err != nil {
		return nil, err
	}
	return cached.Image, nil
}

func (a *App) imageOpForPath(path string) (paint.ImageOp, error) {
	cached, err := a.cachedImageForPath(path)
	if err != nil {
		return paint.ImageOp{}, err
	}
	return cached.Op, nil
}

func (a *App) imageForPathThumb(path string, maxDimension int) (image.Image, error) {
	cached, err := a.cachedImageForPathThumb(path, maxDimension)
	if err != nil {
		return nil, err
	}
	return cached.Image, nil
}

func (a *App) imageOpForPathThumb(path string, maxDimension int) (paint.ImageOp, error) {
	cached, err := a.cachedImageForPathThumb(path, maxDimension)
	if err != nil {
		return paint.ImageOp{}, err
	}
	return cached.Op, nil
}

func pathCacheKey(path string) string {
	return "path:" + path
}

func pathThumbCacheKey(path string, maxDimension int) string {
	return "path-thumb:" + strconv.Itoa(maxDimension) + ":" + path
}

func resizedCachedImage(base cachedImage, maxDimension int) cachedImage {
	if maxDimension <= 0 || base.Image == nil {
		return base
	}
	bounds := base.Image.Bounds()
	if bounds.Dx() <= maxDimension && bounds.Dy() <= maxDimension {
		return base
	}
	scaled := downscaleToMaxDimension(base.Image, maxDimension)
	return cachedImage{
		Image: scaled,
		Op:    paint.NewImageOp(scaled),
	}
}

func (a *App) loadPathThumbUncached(path string, maxDimension int) (image.Image, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errMissingPreview
	}
	if maxDimension <= 0 {
		img, err := decodeImageFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("%w: %v", errMissingPreview, err)
			}
			return nil, err
		}
		return img, nil
	}
	img, err := decodeImageFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %v", errMissingPreview, err)
		}
		return nil, err
	}
	return downscaleToMaxDimension(img, maxDimension), nil
}

func (a *App) cachedImageForPath(path string) (cachedImage, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return cachedImage{}, errMissingPreview
	}
	cacheKey := pathCacheKey(path)
	for {
		cached, found, waitCh := a.beginCachedImageLoad(cacheKey)
		if found {
			if cached.Loading {
				<-waitCh
				continue
			}
			if cached.Failed {
				return cachedImage{}, errMissingPreview
			}
			return cached, nil
		}
		img, err := decodeImageFile(path)
		if err != nil {
			a.completeCachedImageLoad(cacheKey, cachedImage{Failed: true})
			if errors.Is(err, os.ErrNotExist) {
				return cachedImage{}, fmt.Errorf("%w: %v", errMissingPreview, err)
			}
			return cachedImage{}, err
		}
		cached = cachedImage{
			Image: img,
			Op:    paint.NewImageOp(img),
		}
		a.completeCachedImageLoad(cacheKey, cached)
		return cached, nil
	}
}

func (a *App) cachedImageForPathThumb(path string, maxDimension int) (cachedImage, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return cachedImage{}, errMissingPreview
	}
	if maxDimension <= 0 {
		return a.cachedImageForPath(path)
	}
	cacheKey := pathThumbCacheKey(path, maxDimension)
	if cached, ok := a.getCachedImage(cacheKey); ok && !cached.Loading {
		if cached.Failed {
			return cachedImage{}, errMissingPreview
		}
		return cached, nil
	}
	if maxDimension <= pathThumbReuseBaseMinDimension {
		baseKey := pathThumbCacheKey(path, pathThumbReuseBaseMinDimension)
		if cached, ok := a.getCachedImage(baseKey); ok && !cached.Loading {
			if cached.Failed {
				return cachedImage{}, errMissingPreview
			}
			derived := resizedCachedImage(cached, maxDimension)
			a.setCachedImage(cacheKey, derived)
			return derived, nil
		}
	}
	if cached, ok := a.getCachedImage(pathCacheKey(path)); ok && !cached.Loading {
		if cached.Failed {
			return cachedImage{}, errMissingPreview
		}
		derived := resizedCachedImage(cached, maxDimension)
		a.setCachedImage(cacheKey, derived)
		return derived, nil
	}
	if maxDimension <= pathThumbReuseBaseMinDimension {
		base, err := a.cachedPathThumbBase(path, pathThumbReuseBaseMinDimension)
		if err != nil {
			a.setCachedImage(cacheKey, cachedImage{Failed: true})
			return cachedImage{}, err
		}
		derived := resizedCachedImage(base, maxDimension)
		a.setCachedImage(cacheKey, derived)
		return derived, nil
	}
	if maxDimension <= historyThumbFallbackMaxDimension {
		base, err := a.cachedPathThumbBase(path, historyThumbFallbackMaxDimension)
		if err != nil {
			a.setCachedImage(cacheKey, cachedImage{Failed: true})
			return cachedImage{}, err
		}
		a.setCachedImage(cacheKey, base)
		return base, nil
	}
	base, err := a.cachedImageForPath(path)
	if err != nil {
		a.setCachedImage(cacheKey, cachedImage{Failed: true})
		return cachedImage{}, err
	}
	derived := resizedCachedImage(base, maxDimension)
	a.setCachedImage(cacheKey, derived)
	return derived, nil
}

func (a *App) cachedPathThumbBase(path string, baseDimension int) (cachedImage, error) {
	baseKey := pathThumbCacheKey(path, baseDimension)
	for {
		cached, found, waitCh := a.beginCachedImageLoad(baseKey)
		if found {
			if cached.Loading {
				<-waitCh
				continue
			}
			if cached.Failed {
				return cachedImage{}, errMissingPreview
			}
			return cached, nil
		}
		img, err := a.loadPathThumbUncached(path, baseDimension)
		if err != nil {
			a.completeCachedImageLoad(baseKey, cachedImage{Failed: true})
			if errors.Is(err, os.ErrNotExist) {
				return cachedImage{}, fmt.Errorf("%w: %v", errMissingPreview, err)
			}
			return cachedImage{}, err
		}
		cached = cachedImage{
			Image: img,
			Op:    paint.NewImageOp(img),
		}
		a.completeCachedImageLoad(baseKey, cached)
		return cached, nil
	}
}

func (a *App) getCachedImage(key string) (cachedImage, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.imageCache == nil {
		return cachedImage{}, false
	}
	cached, ok := a.imageCache[key]
	return cached, ok
}

func (a *App) beginCachedImageLoad(key string) (cachedImage, bool, chan struct{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.imageCache == nil {
		a.imageCache = map[string]cachedImage{}
	}
	if cached, ok := a.imageCache[key]; ok {
		var waitCh chan struct{}
		if cached.Loading {
			if a.imageLoadWaiters == nil {
				a.imageLoadWaiters = map[string]chan struct{}{}
			}
			waitCh = a.imageLoadWaiters[key]
			if waitCh == nil {
				waitCh = make(chan struct{})
				a.imageLoadWaiters[key] = waitCh
			}
		}
		return cached, true, waitCh
	}
	a.imageCache[key] = cachedImage{Loading: true}
	if a.imageLoadWaiters == nil {
		a.imageLoadWaiters = map[string]chan struct{}{}
	}
	waitCh := make(chan struct{})
	a.imageLoadWaiters[key] = waitCh
	return cachedImage{Loading: true}, false, waitCh
}

func (a *App) completeCachedImageLoad(key string, value cachedImage) {
	var waitCh chan struct{}
	a.mu.Lock()
	if a.imageCache == nil {
		a.imageCache = map[string]cachedImage{}
	}
	a.imageCache[key] = value
	if a.imageLoadWaiters != nil {
		waitCh = a.imageLoadWaiters[key]
		delete(a.imageLoadWaiters, key)
	}
	a.mu.Unlock()
	if waitCh != nil {
		close(waitCh)
	}
}

func (a *App) setCachedImage(key string, value cachedImage) {
	a.mu.Lock()
	if a.imageCache == nil {
		a.imageCache = map[string]cachedImage{}
	}
	a.imageCache[key] = value
	a.mu.Unlock()
}

func (a *App) finishCachedImageIfLoading(key string, value cachedImage) {
	var waitCh chan struct{}
	a.mu.Lock()
	if a.imageCache == nil {
		a.mu.Unlock()
		return
	}
	current, ok := a.imageCache[key]
	if !ok || !current.Loading {
		a.mu.Unlock()
		return
	}
	a.imageCache[key] = value
	if a.imageLoadWaiters != nil {
		waitCh = a.imageLoadWaiters[key]
		delete(a.imageLoadWaiters, key)
	}
	a.mu.Unlock()
	if waitCh != nil {
		close(waitCh)
	}
}

func (a *App) pruneImageCacheLocked() {
	if len(a.imageCache) == 0 {
		return
	}
	keepExact := make(map[string]struct{}, len(a.imageCache))
	keepHistoryDisplayPrefixes := make([]string, 0, len(a.history))
	keepPathSuffixes := make([]string, 0, len(a.history)+len(a.workspaces)+8)

	addPath := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		keepExact["path:"+path] = struct{}{}
		keepPathSuffixes = append(keepPathSuffixes, ":"+path)
	}
	addHistoryItem := func(item sharedCompat.HistoryItem) {
		if strings.TrimSpace(item.ID) == "" && strings.TrimSpace(item.SavedPath) == "" && strings.TrimSpace(item.PreviewPath) == "" && strings.TrimSpace(item.ThumbPath) == "" && strings.TrimSpace(item.ImageB64) == "" {
			return
		}
		keepExact[historyImageCacheKey(item, false)] = struct{}{}
		thumbBase := historyImageCacheKey(item, true)
		keepExact[thumbBase] = struct{}{}
		keepHistoryDisplayPrefixes = append(keepHistoryDisplayPrefixes, thumbBase+":display:")
		keepHistoryDisplayPrefixes = append(keepHistoryDisplayPrefixes, thumbBase+":display-base:")
		addPath(item.PreviewPath)
		for _, path := range historyImagePaths(item, true) {
			addPath(path)
		}
		for _, path := range historyImagePaths(item, false) {
			addPath(path)
		}
	}

	for i, item := range a.history {
		if i >= historyCacheRetention {
			break
		}
		addHistoryItem(item)
	}
	addHistoryItem(a.result.Item)
	addHistoryItem(a.compare.Item)
	addHistoryItem(a.activeResultDetail)
	for _, item := range a.activePromptGroup.Items {
		if item != nil {
			addHistoryItem(*item)
		}
	}
	if a.selectedHistoryID != "" {
		if item, ok := historyItemByID(a.history, a.selectedHistoryID); ok {
			addHistoryItem(item)
		}
	}
	for _, id := range a.batchResultIDs {
		if item, ok := historyItemByID(a.history, id); ok {
			addHistoryItem(item)
		}
	}
	addPath(a.result.SavedPath)
	addPath(a.compare.SavedPath)
	addPath(a.activeResultDetail.SavedPath)
	for _, path := range a.parseSourcePathsCachedLocked(a.sourcePathsInput.Text()) {
		addPath(path)
	}
	for _, ws := range a.workspaces {
		addPath(ws.ResultSavedPath)
		for _, path := range a.parseSourcePathsCachedLocked(ws.SourcePathsText) {
			addPath(path)
		}
	}

	for key := range a.imageCache {
		if _, ok := keepExact[key]; ok {
			continue
		}
		keep := false
		for _, prefix := range keepHistoryDisplayPrefixes {
			if strings.HasPrefix(key, prefix) {
				keep = true
				break
			}
		}
		if !keep && strings.HasPrefix(key, "path-thumb:") {
			for _, suffix := range keepPathSuffixes {
				if strings.HasSuffix(key, suffix) {
					keep = true
					break
				}
			}
		}
		if !keep {
			delete(a.imageCache, key)
		}
	}
}

func (a *App) switchActiveProfile(profileID string) {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return
	}
	a.saveCurrentConfig()
	state, _, err := gioCompat.LoadState()
	if err != nil {
		a.appendLog("读取上游配置失败: " + err.Error())
		return
	}
	state = sharedCompat.Normalize(state)
	if _, ok := historyItemByID(state.History, a.selectedHistoryID); !ok && len(state.History) > 0 {
		a.selectedHistoryID = state.History[0].ID
	}
	found := false
	for _, profile := range state.Profiles {
		if profile.ID == profileID {
			found = true
			break
		}
	}
	if !found {
		return
	}
	state.ActiveProfile = profileID
	state.UpdatedAt = time.Now().UnixMilli()
	if err := gioCompat.SaveState(state); err != nil {
		a.appendLog("切换上游失败: " + err.Error())
		return
	}
	cfg := gioCompat.ConfigFromState(kernel.DefaultConfig(), state)
	a.applyRuntimeConfig(cfg)
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	a.activeProfileID = profileID
	a.settingsSelectedProfileID = profileID
	if profile, ok := profileByID(state.Profiles, profileID); ok {
		a.fallbackProfileID = strings.TrimSpace(profile.FallbackProfileID)
	}
	a.status = "已切换上游: " + activeProfileName(state.Profiles, profileID)
	a.appendLogLocked("切换上游配置: " + activeProfileName(state.Profiles, profileID))
	a.mu.Unlock()
	a.profileNameInput.SetText(activeProfileName(state.Profiles, profileID))
	if limit := activeProfileConcurrencyLimit(state.Profiles, profileID); limit > 0 {
		a.concurrencyLimitInput.SetText(strconv.Itoa(limit))
	} else {
		a.concurrencyLimitInput.SetText("")
	}
	a.profilePickerOpen = false
	a.invalidateNow()
}

func activeProfileName(profiles []sharedCompat.UpstreamProfile, profileID string) string {
	for _, profile := range profiles {
		if profile.ID == profileID {
			return strings.TrimSpace(profile.Name)
		}
	}
	return ""
}

func activeProfileAPIMode(profiles []sharedCompat.UpstreamProfile, profileID string) string {
	for _, profile := range profiles {
		if profile.ID == profileID {
			return strings.TrimSpace(profile.APIMode)
		}
	}
	return ""
}

func activeProfileConcurrencyLimit(profiles []sharedCompat.UpstreamProfile, profileID string) int {
	for _, profile := range profiles {
		if profile.ID == profileID {
			return profile.ConcurrencyLimit
		}
	}
	return 0
}

func profileByID(profiles []sharedCompat.UpstreamProfile, profileID string) (sharedCompat.UpstreamProfile, bool) {
	for _, profile := range profiles {
		if profile.ID == profileID {
			return profile, true
		}
	}
	return sharedCompat.UpstreamProfile{}, false
}

func normalizeProfileAPIMode(mode string) string {
	if strings.TrimSpace(mode) == string(client.APIModeImages) {
		return string(client.APIModeImages)
	}
	return string(client.APIModeResponses)
}

func normalizeProfilePolicy(policy string) string {
	if strings.TrimSpace(policy) == string(client.RequestPolicyCompat) {
		return string(client.RequestPolicyCompat)
	}
	return string(client.RequestPolicyOpenAI)
}

func normalizeProfileResponsesTransport(value string) string {
	if strings.TrimSpace(value) == string(client.ResponsesTransportWebSocket) {
		return string(client.ResponsesTransportWebSocket)
	}
	return string(client.ResponsesTransportSSE)
}

func normalizeReasoningEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	default:
		return client.DefaultReasoningEffort
	}
}

func normalizeSettingsSelectedProfileID(state sharedCompat.State, profileID string) string {
	profileID = strings.TrimSpace(profileID)
	if profileID != "" {
		if _, ok := profileByID(state.Profiles, profileID); ok {
			return profileID
		}
	}
	if strings.TrimSpace(state.ActiveProfile) != "" {
		if _, ok := profileByID(state.Profiles, state.ActiveProfile); ok {
			return state.ActiveProfile
		}
	}
	if len(state.Profiles) > 0 {
		return state.Profiles[0].ID
	}
	return ""
}

func (a *App) settingsDraftReady() bool {
	return strings.TrimSpace(a.baseURLInput.Text()) != "" && strings.TrimSpace(a.apiKeyInput.Text()) != ""
}

func (a *App) applySettingsProfileDraft(state sharedCompat.State, profile sharedCompat.UpstreamProfile) {
	a.settingsSelectedProfileID = profile.ID
	a.profileNameInput.SetText(strings.TrimSpace(profile.Name))
	a.api = normalizeProfileAPIMode(profile.APIMode)
	a.policy = normalizeProfilePolicy(profile.RequestPolicy)
	a.responsesTransport = normalizeProfileResponsesTransport(profile.ResponsesTransport)
	a.reasoningEffort = normalizeReasoningEffort(profile.ReasoningEffort)
	a.fallbackProfileID = strings.TrimSpace(profile.FallbackProfileID)
	a.imagesNewAPICompat = profile.ImagesNewAPICompat
	a.baseURLInput.SetText(strings.TrimSpace(profile.BaseURL))
	a.textModelInput.SetText(strings.TrimSpace(profile.TextModelID))
	a.imageModelInput.SetText(strings.TrimSpace(profile.ImageModelID))
	if profile.ConcurrencyLimit > 0 {
		a.concurrencyLimitInput.SetText(strconv.Itoa(profile.ConcurrencyLimit))
	} else {
		a.concurrencyLimitInput.SetText("")
	}
	key, _ := gioCompat.ReadAPIKey(profile.ID)
	a.apiKeyInput.SetText(key)
	a.proxy = strings.TrimSpace(state.Settings.ProxyMode)
	if a.proxy == "" {
		a.proxy = client.ProxyModeSystem
	}
	a.proxyURLInput.SetText(strings.TrimSpace(state.Settings.ProxyURL))
	outputDir := strings.TrimSpace(state.Settings.OutputDir)
	if outputDir == "" {
		outputDir = kernel.DefaultOutputDir()
	}
	a.outputDirInput.SetText(outputDir)
	a.background = strings.TrimSpace(state.Settings.Background)
	if a.background == "" {
		a.background = client.DefaultBackground
	}
	if state.Settings.OutputCompression != nil {
		a.outputCompressionInput.SetText(strconv.Itoa(*state.Settings.OutputCompression))
	} else {
		a.outputCompressionInput.SetText(strconv.Itoa(client.DefaultOutputCompression))
	}
	a.inputFidelity = strings.TrimSpace(state.Settings.InputFidelity)
	if a.inputFidelity == "" {
		a.inputFidelity = client.DefaultInputFidelity
	}
	a.imageStyle = strings.TrimSpace(state.Settings.ImageStyle)
	if a.imageStyle == "" {
		a.imageStyle = client.DefaultImageStyle
	}
	a.moderation = strings.TrimSpace(state.Settings.Moderation)
	if a.moderation == "" {
		a.moderation = client.DefaultModeration
	}
	a.userIdentifierInput.SetText(strings.TrimSpace(state.Settings.UserIdentifier))
	if state.Settings.PartialImages != nil {
		a.partialImagesInput.SetText(strconv.Itoa(*state.Settings.PartialImages))
	} else {
		a.partialImagesInput.SetText(strconv.Itoa(kernel.DefaultConfig().PartialImages))
	}
	a.completionSound = normaliseCompletionSoundSettings(state.Settings.CompletionSound)
	a.completionNotification = normaliseCompletionNotificationSettings(state.Settings.CompletionNotification)
	a.completionNotificationPermission = readSystemNotificationPermission()
	a.keepLogs = state.Settings.KeepLogs
	a.cleanupPreviewCacheOnExit = state.Settings.CleanupPreviewCacheOnExit
	if state.Settings.ProtectStreamPreview != nil {
		a.protectStreamPreview = *state.Settings.ProtectStreamPreview
	} else {
		a.protectStreamPreview = kernel.DefaultConfig().ProtectStreamPreview
	}
	a.apiKeyVisible = false
}

func (a *App) loadSettingsProfileDraft(profileID string) error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return err
	}
	state = sharedCompat.Normalize(state)
	selectedID := normalizeSettingsSelectedProfileID(state, profileID)
	if selectedID == "" {
		a.settingsSelectedProfileID = ""
		a.profileNameInput.SetText("")
		a.baseURLInput.SetText("")
		a.apiKeyInput.SetText("")
		a.textModelInput.SetText(client.TextModel)
		a.imageModelInput.SetText(client.ImageModel)
		a.concurrencyLimitInput.SetText("")
		a.responsesTransport = string(client.ResponsesTransportSSE)
		a.reasoningEffort = client.DefaultReasoningEffort
		a.fallbackProfileID = ""
		a.proxy = strings.TrimSpace(state.Settings.ProxyMode)
		if a.proxy == "" {
			a.proxy = client.ProxyModeSystem
		}
		a.proxyURLInput.SetText(strings.TrimSpace(state.Settings.ProxyURL))
		outputDir := strings.TrimSpace(state.Settings.OutputDir)
		if outputDir == "" {
			outputDir = kernel.DefaultOutputDir()
		}
		a.outputDirInput.SetText(outputDir)
		background := strings.TrimSpace(state.Settings.Background)
		if background == "" {
			background = client.DefaultBackground
		}
		a.background = background
		if state.Settings.OutputCompression != nil {
			a.outputCompressionInput.SetText(strconv.Itoa(*state.Settings.OutputCompression))
		} else {
			a.outputCompressionInput.SetText(strconv.Itoa(client.DefaultOutputCompression))
		}
		inputFidelity := strings.TrimSpace(state.Settings.InputFidelity)
		if inputFidelity == "" {
			inputFidelity = client.DefaultInputFidelity
		}
		a.inputFidelity = inputFidelity
		imageStyle := strings.TrimSpace(state.Settings.ImageStyle)
		if imageStyle == "" {
			imageStyle = client.DefaultImageStyle
		}
		a.imageStyle = imageStyle
		moderation := strings.TrimSpace(state.Settings.Moderation)
		if moderation == "" {
			moderation = client.DefaultModeration
		}
		a.moderation = moderation
		a.userIdentifierInput.SetText(strings.TrimSpace(state.Settings.UserIdentifier))
		if state.Settings.PartialImages != nil {
			a.partialImagesInput.SetText(strconv.Itoa(*state.Settings.PartialImages))
		} else {
			a.partialImagesInput.SetText(strconv.Itoa(kernel.DefaultConfig().PartialImages))
		}
		a.completionSound = normaliseCompletionSoundSettings(state.Settings.CompletionSound)
		a.completionNotification = normaliseCompletionNotificationSettings(state.Settings.CompletionNotification)
		a.completionNotificationPermission = readSystemNotificationPermission()
		a.keepLogs = state.Settings.KeepLogs
		a.cleanupPreviewCacheOnExit = state.Settings.CleanupPreviewCacheOnExit
		if state.Settings.ProtectStreamPreview != nil {
			a.protectStreamPreview = *state.Settings.ProtectStreamPreview
		} else {
			a.protectStreamPreview = kernel.DefaultConfig().ProtectStreamPreview
		}
		a.imagesNewAPICompat = false
		a.api = string(client.APIModeResponses)
		a.policy = string(client.RequestPolicyOpenAI)
		a.apiKeyVisible = false
		return nil
	}
	profile, ok := profileByID(state.Profiles, selectedID)
	if !ok {
		return nil
	}
	a.applySettingsProfileDraft(state, profile)
	return nil
}

func (a *App) restoreActiveRuntimeConfig(logErrors bool) error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		if logErrors {
			a.appendLog("读取上游配置失败: " + err.Error())
		}
		return err
	}
	state = sharedCompat.Normalize(state)
	cfg := gioCompat.ConfigFromState(kernel.DefaultConfig(), state)
	a.applyRuntimeConfig(cfg)
	a.profileNameInput.SetText(activeProfileName(state.Profiles, state.ActiveProfile))
	if profile, ok := profileByID(state.Profiles, state.ActiveProfile); ok {
		a.fallbackProfileID = strings.TrimSpace(profile.FallbackProfileID)
	} else {
		a.fallbackProfileID = ""
	}
	if limit := activeProfileConcurrencyLimit(state.Profiles, state.ActiveProfile); limit > 0 {
		a.concurrencyLimitInput.SetText(strconv.Itoa(limit))
	} else {
		a.concurrencyLimitInput.SetText("")
	}
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	a.activeProfileID = state.ActiveProfile
	a.settingsSelectedProfileID = state.ActiveProfile
	a.completionNotification = normaliseCompletionNotificationSettings(state.Settings.CompletionNotification)
	a.completionNotificationPermission = readSystemNotificationPermission()
	a.keepLogs = state.Settings.KeepLogs
	a.cleanupPreviewCacheOnExit = state.Settings.CleanupPreviewCacheOnExit
	a.ignoredReleaseTag = strings.TrimSpace(state.Settings.IgnoredReleaseTag)
	a.mu.Unlock()
	a.apiKeyVisible = false
	return nil
}

func (a *App) openSettingsModal() {
	a.settingsModalOpen = true
	a.settingsHelpOpen = false
	if err := a.loadSettingsProfileDraft(a.activeProfileID); err != nil {
		a.appendLog("读取上游配置失败: " + err.Error())
	}
	a.invalidateNow()
}

func (a *App) closeSettingsModal() {
	a.settingsModalOpen = false
	a.settingsHelpOpen = false
	_ = a.restoreActiveRuntimeConfig(false)
	a.invalidateNow()
}

func (a *App) saveSettingsSelection() error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return err
	}
	state = sharedCompat.Normalize(state)
	selectedID := normalizeSettingsSelectedProfileID(state, a.settingsSelectedProfileID)
	if selectedID == "" {
		return nil
	}
	now := time.Now().UnixMilli()
	updated := false
	for i := range state.Profiles {
		if state.Profiles[i].ID != selectedID {
			continue
		}
		name := strings.TrimSpace(a.profileNameInput.Text())
		if name == "" {
			name = strings.TrimSpace(state.Profiles[i].Name)
		}
		if name == "" {
			name = nextProfileName(state.Profiles)
		}
		concurrencyLimit := 0
		if raw := strings.TrimSpace(a.concurrencyLimitInput.Text()); raw != "" {
			if value, err := strconv.Atoi(raw); err == nil && value > 0 {
				concurrencyLimit = value
			}
		}
		state.Profiles[i].Name = name
		state.Profiles[i].APIMode = normalizeProfileAPIMode(a.api)
		state.Profiles[i].RequestPolicy = normalizeProfilePolicy(a.policy)
		state.Profiles[i].ResponsesTransport = normalizeProfileResponsesTransport(a.responsesTransport)
		state.Profiles[i].ReasoningEffort = normalizeReasoningEffort(a.reasoningEffort)
		state.Profiles[i].FallbackProfileID = strings.TrimSpace(a.fallbackProfileID)
		state.Profiles[i].ImagesNewAPICompat = a.imagesNewAPICompat
		state.Profiles[i].BaseURL = strings.TrimSpace(a.baseURLInput.Text())
		state.Profiles[i].TextModelID = strings.TrimSpace(a.textModelInput.Text())
		state.Profiles[i].ImageModelID = strings.TrimSpace(a.imageModelInput.Text())
		state.Profiles[i].ConcurrencyLimit = concurrencyLimit
		updated = true
		break
	}
	if !updated {
		return nil
	}
	state.Settings.ProxyMode = strings.TrimSpace(a.proxy)
	if state.Settings.ProxyMode == "" {
		state.Settings.ProxyMode = client.ProxyModeSystem
	}
	state.Settings.ProxyURL = strings.TrimSpace(a.proxyURLInput.Text())
	state.Settings.OutputDir = strings.TrimSpace(a.outputDirInput.Text())
	state = gioCompat.RememberTrustedOutputRoot(state, state.Settings.OutputDir)
	state.Settings.Background = strings.TrimSpace(a.background)
	outputCompression := client.DefaultOutputCompression
	if raw := strings.TrimSpace(a.outputCompressionInput.Text()); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			outputCompression = value
		}
	}
	state.Settings.OutputCompression = &outputCompression
	state.Settings.InputFidelity = strings.TrimSpace(a.inputFidelity)
	state.Settings.ImageStyle = strings.TrimSpace(a.imageStyle)
	state.Settings.Moderation = strings.TrimSpace(a.moderation)
	state.Settings.UserIdentifier = strings.TrimSpace(a.userIdentifierInput.Text())
	partialImages := kernel.DefaultConfig().PartialImages
	if raw := strings.TrimSpace(a.partialImagesInput.Text()); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			partialImages = value
		}
	}
	state.Settings.PartialImages = &partialImages
	completionSound := a.completionSound
	state.Settings.CompletionSound = &completionSound
	completionNotification := a.completionNotification
	state.Settings.CompletionNotification = &completionNotification
	protectStreamPreview := a.protectStreamPreview
	state.Settings.ProtectStreamPreview = &protectStreamPreview
	state.Settings.KeepLogs = a.keepLogs
	state.Settings.CleanupPreviewCacheOnExit = a.cleanupPreviewCacheOnExit
	state.UpdatedAt = now
	if err := gioCompat.SaveState(state); err != nil {
		return err
	}
	if err := gioCompat.WriteAPIKey(selectedID, a.apiKeyInput.Text()); err != nil {
		return err
	}
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	a.settingsSelectedProfileID = selectedID
	a.status = "已保存配置: " + activeProfileName(state.Profiles, selectedID)
	a.appendLogLocked("已保存配置: " + activeProfileName(state.Profiles, selectedID))
	a.mu.Unlock()
	if selectedID == state.ActiveProfile {
		_ = a.restoreActiveRuntimeConfig(false)
	}
	return nil
}

func (a *App) activateStoredProfile(profileID string) error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return err
	}
	state = sharedCompat.Normalize(state)
	selectedID := normalizeSettingsSelectedProfileID(state, profileID)
	if selectedID == "" {
		return nil
	}
	name := activeProfileName(state.Profiles, selectedID)
	state.ActiveProfile = selectedID
	for i := range state.Profiles {
		if state.Profiles[i].ID == selectedID {
			state.Profiles[i].LastUsedAt = time.Now().UnixMilli()
			break
		}
	}
	state.UpdatedAt = time.Now().UnixMilli()
	if err := gioCompat.SaveState(state); err != nil {
		return err
	}
	if err := a.restoreActiveRuntimeConfig(false); err != nil {
		return err
	}
	a.mu.Lock()
	a.status = "已切换上游: " + name
	a.appendLogLocked("切换上游配置: " + name)
	a.mu.Unlock()
	a.profilePickerOpen = false
	return nil
}

func (a *App) createSettingsProfile(apiMode string) error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return err
	}
	state = sharedCompat.Normalize(state)
	now := time.Now().UnixMilli()
	profileID := fmt.Sprintf("gio-%d", now)
	profile := sharedCompat.UpstreamProfile{
		ID:                 profileID,
		Name:               nextProfileName(state.Profiles),
		APIMode:            normalizeProfileAPIMode(apiMode),
		ResponsesTransport: string(client.ResponsesTransportSSE),
		RequestPolicy:      string(client.RequestPolicyOpenAI),
		ImagesNewAPICompat: false,
		TextModelID:        client.TextModel,
		ImageModelID:       client.ImageModel,
		ReasoningEffort:    client.DefaultReasoningEffort,
		CreatedAt:          now,
		LastUsedAt:         now,
	}
	state.Profiles = append(state.Profiles, profile)
	activate := len(state.Profiles) == 1 || strings.TrimSpace(state.ActiveProfile) == ""
	if activate {
		state.ActiveProfile = profileID
	}
	state.UpdatedAt = now
	if err := gioCompat.SaveState(state); err != nil {
		return err
	}
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	if activate {
		a.activeProfileID = profileID
	}
	a.settingsSelectedProfileID = profileID
	a.status = "已创建配置: " + profile.Name
	a.appendLogLocked("已创建配置: " + profile.Name)
	a.mu.Unlock()
	if activate {
		if err := a.restoreActiveRuntimeConfig(false); err != nil {
			return err
		}
	} else if err := a.loadSettingsProfileDraft(profileID); err != nil {
		return err
	}
	return nil
}

func (a *App) duplicateSettingsProfile() error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return err
	}
	state = sharedCompat.Normalize(state)
	selectedID := normalizeSettingsSelectedProfileID(state, a.settingsSelectedProfileID)
	current, ok := profileByID(state.Profiles, selectedID)
	if !ok {
		return nil
	}
	now := time.Now().UnixMilli()
	profileID := fmt.Sprintf("gio-%d", now)
	clone := current
	clone.ID = profileID
	clone.Name = nextProfileName(state.Profiles)
	clone.CreatedAt = now
	clone.LastUsedAt = now
	state.Profiles = append(state.Profiles, clone)
	state.UpdatedAt = now
	if key, _ := gioCompat.ReadAPIKey(current.ID); strings.TrimSpace(key) != "" {
		_ = gioCompat.WriteAPIKey(profileID, key)
	}
	if err := gioCompat.SaveState(state); err != nil {
		return err
	}
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	a.settingsSelectedProfileID = profileID
	a.status = "已复制配置: " + clone.Name
	a.appendLogLocked("已复制配置: " + clone.Name)
	a.mu.Unlock()
	return a.loadSettingsProfileDraft(profileID)
}

func (a *App) deleteSettingsProfile() error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return err
	}
	state = sharedCompat.Normalize(state)
	selectedID := normalizeSettingsSelectedProfileID(state, a.settingsSelectedProfileID)
	current, ok := profileByID(state.Profiles, selectedID)
	if !ok {
		return nil
	}
	nextProfiles := make([]sharedCompat.UpstreamProfile, 0, len(state.Profiles)-1)
	for _, profile := range state.Profiles {
		if profile.ID == current.ID {
			continue
		}
		nextProfiles = append(nextProfiles, profile)
	}
	state.Profiles = nextProfiles
	if current.ID == state.ActiveProfile {
		if len(nextProfiles) > 0 {
			state.ActiveProfile = nextProfiles[0].ID
		} else {
			state.ActiveProfile = ""
		}
	}
	state.UpdatedAt = time.Now().UnixMilli()
	_ = gioCompat.WriteAPIKey(current.ID, "")
	if err := gioCompat.SaveState(state); err != nil {
		return err
	}
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	a.activeProfileID = state.ActiveProfile
	a.status = "已删除配置: " + current.Name
	a.appendLogLocked("已删除配置: " + current.Name)
	a.mu.Unlock()
	nextSelectedID := ""
	if len(state.Profiles) > 0 {
		nextSelectedID = state.Profiles[0].ID
	}
	if state.ActiveProfile != "" {
		_ = a.restoreActiveRuntimeConfig(false)
	}
	return a.loadSettingsProfileDraft(nextSelectedID)
}

func (a *App) saveActiveProfileMetadata() error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return err
	}
	state = sharedCompat.Normalize(state)
	current, ok := currentActiveProfile(state)
	if !ok {
		return nil
	}
	name := strings.TrimSpace(a.profileNameInput.Text())
	if name == "" {
		name = strings.TrimSpace(current.Name)
	}
	if name == "" {
		name = nextProfileName(state.Profiles)
	}
	concurrencyLimit := 0
	if raw := strings.TrimSpace(a.concurrencyLimitInput.Text()); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil && value > 0 {
			concurrencyLimit = value
		}
	}
	updated := false
	for i := range state.Profiles {
		if state.Profiles[i].ID != current.ID {
			continue
		}
		state.Profiles[i].Name = name
		state.Profiles[i].ConcurrencyLimit = concurrencyLimit
		state.Profiles[i].LastUsedAt = time.Now().UnixMilli()
		updated = true
		break
	}
	if !updated {
		return nil
	}
	state.UpdatedAt = time.Now().UnixMilli()
	if err := gioCompat.SaveState(state); err != nil {
		return err
	}
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	a.mu.Unlock()
	return nil
}

func filteredHistoryItems(
	items []sharedCompat.HistoryItem,
	query string,
	modeFilter string,
	dateFilter string,
	now time.Time,
) []sharedCompat.HistoryItem {
	query = normalizeHistorySearchQuery(query)
	modeFilter = strings.TrimSpace(modeFilter)
	dateFilter = strings.TrimSpace(dateFilter)
	dateKind, dateCutoff := prepareHistoryDateFilter(dateFilter, now)
	if query == "" && modeFilter == "all" && dateFilter == "all" {
		return items
	}
	filtered := make([]sharedCompat.HistoryItem, 0, len(items))
	for _, item := range items {
		if modeFilter != "" && modeFilter != "all" && item.Mode != modeFilter {
			continue
		}
		if !matchHistoryDatePrepared(item.CreatedAt, dateKind, dateCutoff) {
			continue
		}
		if !matchHistoryQueryNormalized(item, query) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func nextProfileName(profiles []sharedCompat.UpstreamProfile) string {
	used := map[int]struct{}{}
	for _, profile := range profiles {
		raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(profile.Name), "配置"))
		n, err := strconv.Atoi(raw)
		if err == nil && n > 0 {
			used[n] = struct{}{}
		}
	}
	for i := 1; ; i++ {
		if _, ok := used[i]; !ok {
			return "配置" + strconv.Itoa(i)
		}
	}
}

func (a *App) createBlankProfile() {
	a.createBlankProfileWithMode(string(client.APIModeResponses))
}

func (a *App) createBlankProfileWithMode(apiMode string) {
	a.saveCurrentConfig()
	state, _, err := gioCompat.LoadState()
	if err != nil {
		a.appendLog("创建配置失败: " + err.Error())
		return
	}
	state = sharedCompat.Normalize(state)
	now := time.Now().UnixMilli()
	profileID := fmt.Sprintf("gio-%d", now)
	profile := sharedCompat.UpstreamProfile{
		ID:                 profileID,
		Name:               nextProfileName(state.Profiles),
		APIMode:            apiMode,
		ResponsesTransport: string(client.ResponsesTransportSSE),
		RequestPolicy:      string(client.RequestPolicyOpenAI),
		ImagesNewAPICompat: false,
		TextModelID:        client.TextModel,
		ImageModelID:       client.ImageModel,
		ReasoningEffort:    client.DefaultReasoningEffort,
		CreatedAt:          now,
		LastUsedAt:         now,
	}
	state.Profiles = append(state.Profiles, profile)
	state.ActiveProfile = profileID
	state.UpdatedAt = now
	if err := gioCompat.SaveState(state); err != nil {
		a.appendLog("创建配置失败: " + err.Error())
		return
	}
	cfg := gioCompat.ConfigFromState(kernel.DefaultConfig(), state)
	a.applyRuntimeConfig(cfg)
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	a.activeProfileID = profileID
	a.fallbackProfileID = strings.TrimSpace(profile.FallbackProfileID)
	a.status = "已创建配置: " + profile.Name
	a.appendLogLocked("已创建配置: " + profile.Name)
	a.mu.Unlock()
	a.profileNameInput.SetText(profile.Name)
	a.concurrencyLimitInput.SetText("")
	a.invalidateNow()
}

func (a *App) duplicateActiveProfile() {
	a.saveCurrentConfig()
	state, _, err := gioCompat.LoadState()
	if err != nil {
		a.appendLog("复制配置失败: " + err.Error())
		return
	}
	state = sharedCompat.Normalize(state)
	current, ok := currentActiveProfile(state)
	if !ok {
		return
	}
	now := time.Now().UnixMilli()
	profileID := fmt.Sprintf("gio-%d", now)
	clone := current
	clone.ID = profileID
	clone.Name = nextProfileName(state.Profiles)
	clone.CreatedAt = now
	clone.LastUsedAt = now
	state.Profiles = append(state.Profiles, clone)
	state.ActiveProfile = profileID
	state.UpdatedAt = now
	if key, _ := gioCompat.ReadAPIKey(current.ID); strings.TrimSpace(key) != "" {
		_ = gioCompat.WriteAPIKey(profileID, key)
	}
	if err := gioCompat.SaveState(state); err != nil {
		a.appendLog("复制配置失败: " + err.Error())
		return
	}
	cfg := gioCompat.ConfigFromState(kernel.DefaultConfig(), state)
	a.applyRuntimeConfig(cfg)
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	a.activeProfileID = profileID
	a.fallbackProfileID = strings.TrimSpace(clone.FallbackProfileID)
	a.status = "已复制配置: " + clone.Name
	a.appendLogLocked("已复制配置: " + clone.Name)
	a.mu.Unlock()
	a.profileNameInput.SetText(clone.Name)
	if clone.ConcurrencyLimit > 0 {
		a.concurrencyLimitInput.SetText(strconv.Itoa(clone.ConcurrencyLimit))
	} else {
		a.concurrencyLimitInput.SetText("")
	}
	a.invalidateNow()
}

func (a *App) deleteActiveProfile() {
	a.saveCurrentConfig()
	state, _, err := gioCompat.LoadState()
	if err != nil {
		a.appendLog("删除配置失败: " + err.Error())
		return
	}
	state = sharedCompat.Normalize(state)
	if len(state.Profiles) == 0 {
		return
	}
	current, ok := currentActiveProfile(state)
	if !ok {
		return
	}
	nextProfiles := make([]sharedCompat.UpstreamProfile, 0, len(state.Profiles)-1)
	for _, profile := range state.Profiles {
		if profile.ID == current.ID {
			continue
		}
		nextProfiles = append(nextProfiles, profile)
	}
	state.Profiles = nextProfiles
	if len(nextProfiles) > 0 {
		state.ActiveProfile = nextProfiles[0].ID
	} else {
		state.ActiveProfile = ""
	}
	state.UpdatedAt = time.Now().UnixMilli()
	_ = gioCompat.WriteAPIKey(current.ID, "")
	if err := gioCompat.SaveState(state); err != nil {
		a.appendLog("删除配置失败: " + err.Error())
		return
	}
	cfg := gioCompat.ConfigFromState(kernel.DefaultConfig(), state)
	a.applyRuntimeConfig(cfg)
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	a.activeProfileID = state.ActiveProfile
	if profile, ok := profileByID(state.Profiles, state.ActiveProfile); ok {
		a.fallbackProfileID = strings.TrimSpace(profile.FallbackProfileID)
	} else {
		a.fallbackProfileID = ""
	}
	a.status = "已删除配置: " + current.Name
	a.appendLogLocked("已删除配置: " + current.Name)
	a.mu.Unlock()
	a.profileNameInput.SetText(activeProfileName(state.Profiles, state.ActiveProfile))
	if limit := activeProfileConcurrencyLimit(state.Profiles, state.ActiveProfile); limit > 0 {
		a.concurrencyLimitInput.SetText(strconv.Itoa(limit))
	} else {
		a.concurrencyLimitInput.SetText("")
	}
	a.invalidateNow()
}

func currentActiveProfile(state sharedCompat.State) (sharedCompat.UpstreamProfile, bool) {
	if strings.TrimSpace(state.ActiveProfile) != "" {
		for _, profile := range state.Profiles {
			if profile.ID == state.ActiveProfile {
				return profile, true
			}
		}
	}
	if len(state.Profiles) == 0 {
		return sharedCompat.UpstreamProfile{}, false
	}
	return state.Profiles[0], true
}

func augmentPromptWithStyle(prompt string, styleTag string) string {
	prompt = strings.TrimSpace(prompt)
	suffix := strings.TrimSpace(styleSuffixes[strings.TrimSpace(styleTag)])
	if prompt == "" || suffix == "" {
		return prompt
	}
	return prompt + ", " + suffix
}

func buildPromptSuggestions(promptHistory []string, history []sharedCompat.HistoryItem) []string {
	out := make([]string, 0, 8)
	seen := map[string]struct{}{}
	push := func(text string) {
		text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
		if text == "" {
			return
		}
		if _, ok := seen[text]; ok {
			return
		}
		seen[text] = struct{}{}
		out = append(out, text)
	}
	for _, text := range promptHistory {
		push(text)
		if len(out) >= 8 {
			return out
		}
	}
	for _, item := range history {
		push(item.Prompt)
		if len(out) >= 8 {
			return out
		}
	}
	return out
}

func (a *App) promptSuggestions(history []sharedCompat.HistoryItem) []string {
	a.mu.Lock()
	cached := a.promptSuggestionsCache
	historyRev := a.historyRev
	promptHistoryRev := a.promptHistoryRev
	if cached.historyRev == historyRev && cached.promptHistoryRev == promptHistoryRev {
		a.mu.Unlock()
		return cached.items
	}
	promptHistory := append([]string(nil), a.promptHistory...)
	a.mu.Unlock()
	items := buildPromptSuggestions(promptHistory, history)
	a.mu.Lock()
	if a.historyRev == historyRev && a.promptHistoryRev == promptHistoryRev {
		a.promptSuggestionsCache = promptSuggestionsCache{
			historyRev:       historyRev,
			promptHistoryRev: promptHistoryRev,
			items:            items,
		}
	}
	a.mu.Unlock()
	return items
}

func (a *App) ensureHistoryThumbBackfill(item sharedCompat.HistoryItem) {
	if (strings.TrimSpace(item.ThumbPath) != "" && strings.TrimSpace(item.PreviewPath) != "") || strings.TrimSpace(item.SavedPath) == "" || strings.TrimSpace(item.ID) == "" {
		return
	}
	a.startHistoryThumbBackfillItems([]sharedCompat.HistoryItem{item}, false)
}

func collectHistoryThumbBackfillCandidates(items []sharedCompat.HistoryItem, inFlight map[string]struct{}) []sharedCompat.HistoryItem {
	return collectHistoryThumbBackfillCandidatesWithLimit(items, inFlight, historyThumbBackfillLimit)
}

func collectHistoryPreviewOnlyBackfillCandidatesWithLimit(items []sharedCompat.HistoryItem, inFlight map[string]struct{}, limit int) []sharedCompat.HistoryItem {
	limit = min(len(items), limit)
	out := make([]sharedCompat.HistoryItem, 0, limit)
	seenSaved := make(map[string]struct{}, limit)
	for i := 0; i < len(items) && len(out) < limit; i++ {
		item := items[i]
		savedPath := strings.TrimSpace(item.SavedPath)
		if strings.TrimSpace(item.ID) == "" || savedPath == "" || strings.TrimSpace(item.PreviewPath) != "" || strings.TrimSpace(item.ThumbPath) == "" {
			continue
		}
		if _, ok := inFlight[savedPath]; ok {
			continue
		}
		if _, ok := seenSaved[savedPath]; ok {
			continue
		}
		if _, err := os.Stat(savedPath); err != nil {
			continue
		}
		seenSaved[savedPath] = struct{}{}
		out = append(out, item)
	}
	return out
}

func collectHistoryThumbBackfillCandidatesWithLimit(items []sharedCompat.HistoryItem, inFlight map[string]struct{}, limit int) []sharedCompat.HistoryItem {
	limit = min(len(items), limit)
	previewOnly := make([]sharedCompat.HistoryItem, 0, limit)
	heavy := make([]sharedCompat.HistoryItem, 0, limit)
	seenSaved := make(map[string]struct{}, limit)
	for i := 0; i < len(items); i++ {
		item := items[i]
		savedPath := strings.TrimSpace(item.SavedPath)
		if strings.TrimSpace(item.ID) == "" || savedPath == "" || (strings.TrimSpace(item.ThumbPath) != "" && strings.TrimSpace(item.PreviewPath) != "") {
			continue
		}
		if _, ok := inFlight[savedPath]; ok {
			continue
		}
		if _, ok := seenSaved[savedPath]; ok {
			continue
		}
		if _, err := os.Stat(savedPath); err != nil {
			continue
		}
		seenSaved[savedPath] = struct{}{}
		if strings.TrimSpace(item.PreviewPath) == "" && strings.TrimSpace(item.ThumbPath) != "" {
			previewOnly = append(previewOnly, item)
		} else {
			heavy = append(heavy, item)
		}
	}
	out := make([]sharedCompat.HistoryItem, 0, min(limit, len(previewOnly)+len(heavy)))
	out = append(out, previewOnly...)
	if len(out) < limit {
		out = append(out, heavy[:min(len(heavy), limit-len(out))]...)
	} else if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func buildHistoryMediaBackfillUpdates(items []sharedCompat.HistoryItem) map[string]historyMediaBackfillUpdate {
	thumbUpdates := make(map[string]historyMediaBackfillUpdate, len(items))
	pathThumbs := make(map[string]historyMediaBackfillUpdate, len(items))
	for _, item := range items {
		savedPath := strings.TrimSpace(item.SavedPath)
		if savedPath == "" || strings.TrimSpace(item.ID) == "" {
			continue
		}
		update, ok := pathThumbs[savedPath]
		if !ok {
			update = historyMediaBackfillUpdate{}
			previewMissing := strings.TrimSpace(item.PreviewPath) == ""
			thumbMissing := strings.TrimSpace(item.ThumbPath) == ""
			if previewMissing && !thumbMissing {
				previewPath, err := kernel.EnsurePreviewForPathWithFallback(savedPath, item.ThumbPath)
				if err == nil && strings.TrimSpace(previewPath) != "" {
					update.PreviewPath = previewPath
				}
			}
			if previewMissing && strings.TrimSpace(update.PreviewPath) == "" || thumbMissing {
				previewPath, thumbPath, err := kernel.EnsurePreviewAndThumbForPath(savedPath)
				if err == nil || strings.TrimSpace(previewPath) != "" || strings.TrimSpace(thumbPath) != "" {
					if previewMissing && strings.TrimSpace(update.PreviewPath) == "" {
						update.PreviewPath = previewPath
					}
					if thumbMissing {
						update.ThumbPath = thumbPath
					}
				}
			}
			pathThumbs[savedPath] = update
		}
		if update == (historyMediaBackfillUpdate{}) {
			continue
		}
		thumbUpdates[item.ID] = update
	}
	return thumbUpdates
}

func summarizeHistoryMediaBackfill(updates map[string]historyMediaBackfillUpdate) string {
	if len(updates) == 0 {
		return ""
	}
	previewCount := 0
	thumbCount := 0
	for _, update := range updates {
		if strings.TrimSpace(update.PreviewPath) != "" {
			previewCount++
		}
		if strings.TrimSpace(update.ThumbPath) != "" {
			thumbCount++
		}
	}
	return fmt.Sprintf("历史媒体回填完成: %d 项 · 预览 %d · 缩略图 %d", len(updates), previewCount, thumbCount)
}

func (a *App) startHistoryThumbBackfill() {
	a.mu.Lock()
	history := append([]sharedCompat.HistoryItem(nil), a.history...)
	a.mu.Unlock()
	a.startHistoryThumbBackfillItems(history, true)
}

func (a *App) scheduleHistoryThumbBackfill(delay time.Duration) {
	if delay <= 0 {
		a.startHistoryThumbBackfill()
		return
	}
	go func() {
		time.Sleep(delay)
		a.startHistoryThumbBackfill()
	}()
}

func (a *App) scheduleHistoryThumbPrewarm(delay time.Duration) {
	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}
		for {
			a.mu.Lock()
			running := a.running
			processing := a.processingImageTransform
			history := append([]sharedCompat.HistoryItem(nil), a.history...)
			a.mu.Unlock()
			if running || processing {
				time.Sleep(historyBackfillBusyDelay)
				continue
			}
			started := time.Now()
			loaded, failed := a.prewarmHistoryThumbs(history)
			a.mu.Lock()
			a.lastHistoryThumbPrewarmAt = time.Now()
			a.lastHistoryThumbPrewarmMs = time.Since(started)
			a.lastHistoryThumbPrewarmLoad = loaded
			a.lastHistoryThumbPrewarmFail = failed
			a.mu.Unlock()
			if loaded > 0 || failed > 0 {
				a.appendLog(fmt.Sprintf("历史缩略图预热完成: %d 项 · 失败 %d", loaded, failed))
			}
			return
		}
	}()
}

func (a *App) nextHistoryBackfillDelay() time.Duration {
	a.mu.Lock()
	running := a.running
	processing := a.processingImageTransform
	a.mu.Unlock()
	if running || processing {
		return historyBackfillBusyDelay
	}
	return historyBackfillChainDelay
}

func (a *App) prewarmHistoryThumbsWithLimit(items []sharedCompat.HistoryItem, limit int) (loaded int, failed int) {
	if len(items) == 0 {
		return 0, 0
	}
	limit = min(len(items), limit)
	eligible := make([]sharedCompat.HistoryItem, 0, limit)
	for idx := 0; idx < len(items) && len(eligible) < limit; idx++ {
		item := items[idx]
		if !headlessHistoryItemThumbEligible(item) {
			continue
		}
		eligible = append(eligible, item)
	}
	if len(eligible) == 0 {
		return 0, 0
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, item := range eligible {
		item := item
		wg.Add(1)
		runThumbDecodeAsync(func() {
			defer wg.Done()
			itemFailed := false
			for _, dimension := range historyThumbPrewarmDimensions {
				if _, err := a.loadDisplayHistoryThumb(item, dimension); err != nil {
					itemFailed = true
					break
				}
			}
			mu.Lock()
			if itemFailed {
				failed++
			} else {
				loaded++
			}
			mu.Unlock()
		})
	}
	wg.Wait()
	return loaded, failed
}

func (a *App) prewarmHistoryThumbs(items []sharedCompat.HistoryItem) (loaded int, failed int) {
	return a.prewarmHistoryThumbsWithLimit(items, historyThumbPrewarmCount)
}

func (a *App) runStartupHistoryThumbPrewarm() {
	a.mu.Lock()
	history := append([]sharedCompat.HistoryItem(nil), a.history...)
	a.mu.Unlock()
	started := time.Now()
	loaded, failed := a.prewarmHistoryThumbsWithLimit(history, historyThumbStartupSyncPrewarmCount)
	a.mu.Lock()
	a.lastHistoryThumbPrewarmAt = time.Now()
	a.lastHistoryThumbPrewarmMs = time.Since(started)
	a.lastHistoryThumbPrewarmLoad = loaded
	a.lastHistoryThumbPrewarmFail = failed
	a.mu.Unlock()
	if loaded > 0 || failed > 0 {
		a.appendLog("启动缩略图预热完成: " + fmt.Sprintf("%d 项 · 失败 %d", loaded, failed))
	}
}

func (a *App) startHistoryPreviewWarmup() {
	a.mu.Lock()
	history := append([]sharedCompat.HistoryItem(nil), a.history...)
	a.mu.Unlock()
	a.startHistoryPreviewWarmupItems(history)
}

func cloneHistoryBackfillInFlight(src map[string]struct{}) map[string]struct{} {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]struct{}, len(src))
	for key := range src {
		dst[key] = struct{}{}
	}
	return dst
}

func (a *App) currentHistoryBackfillInFlight() map[string]struct{} {
	a.mu.Lock()
	inFlight := cloneHistoryBackfillInFlight(a.historyThumbBackfillInFlight)
	a.mu.Unlock()
	return inFlight
}

func (a *App) reserveHistoryBackfillItems(candidates []sharedCompat.HistoryItem) ([]sharedCompat.HistoryItem, []string) {
	if len(candidates) == 0 {
		return nil, nil
	}
	paths := make([]string, 0, len(candidates))
	reserved := make([]sharedCompat.HistoryItem, 0, len(candidates))
	a.mu.Lock()
	if a.historyThumbBackfillInFlight == nil {
		a.historyThumbBackfillInFlight = map[string]struct{}{}
	}
	for _, item := range candidates {
		savedPath := strings.TrimSpace(item.SavedPath)
		if savedPath == "" {
			continue
		}
		if _, ok := a.historyThumbBackfillInFlight[savedPath]; ok {
			continue
		}
		a.historyThumbBackfillInFlight[savedPath] = struct{}{}
		reserved = append(reserved, item)
		paths = append(paths, savedPath)
	}
	a.mu.Unlock()
	if len(paths) == 0 {
		return nil, nil
	}
	return reserved, paths
}

func (a *App) startHistoryPreviewWarmupItems(items []sharedCompat.HistoryItem) {
	candidates := collectHistoryPreviewOnlyBackfillCandidatesWithLimit(items, a.currentHistoryBackfillInFlight(), historyPreviewWarmLimit)
	candidates, paths := a.reserveHistoryBackfillItems(candidates)
	if len(paths) == 0 {
		return
	}
	go func(items []sharedCompat.HistoryItem, inFlightPaths []string) {
		updates := buildHistoryMediaBackfillUpdates(items)
		if len(updates) == 0 {
			a.finishHistoryThumbBackfill(inFlightPaths)
			return
		}
		if err := a.persistHistoryThumbBackfill(updates); err != nil {
			a.finishHistoryThumbBackfill(inFlightPaths)
			a.appendLog("历史缩略图回填失败: " + err.Error())
			return
		}
		a.applyHistoryThumbBackfill(updates)
		a.appendLog(summarizeHistoryMediaBackfill(updates))
		a.finishHistoryThumbBackfill(inFlightPaths)
	}(candidates, paths)
}

func (a *App) startHistoryThumbBackfillItems(items []sharedCompat.HistoryItem, chain bool) {
	candidates := collectHistoryThumbBackfillCandidates(items, a.currentHistoryBackfillInFlight())
	candidates, paths := a.reserveHistoryBackfillItems(candidates)
	if len(paths) == 0 {
		return
	}
	go func(items []sharedCompat.HistoryItem, inFlightPaths []string) {
		thumbUpdates := buildHistoryMediaBackfillUpdates(items)
		if len(thumbUpdates) == 0 {
			a.finishHistoryThumbBackfill(inFlightPaths)
			return
		}
		if err := a.persistHistoryThumbBackfill(thumbUpdates); err != nil {
			a.finishHistoryThumbBackfill(inFlightPaths)
			a.appendLog("历史缩略图回填失败: " + err.Error())
			return
		}
		a.applyHistoryThumbBackfill(thumbUpdates)
		a.appendLog(summarizeHistoryMediaBackfill(thumbUpdates))
		a.finishHistoryThumbBackfill(inFlightPaths)
		if chain {
			a.scheduleHistoryThumbBackfill(a.nextHistoryBackfillDelay())
		}
	}(candidates, paths)
}

func (a *App) finishHistoryThumbBackfill(paths []string) {
	if len(paths) == 0 {
		return
	}
	a.mu.Lock()
	if a.historyThumbBackfillInFlight != nil {
		for _, path := range paths {
			delete(a.historyThumbBackfillInFlight, path)
		}
	}
	a.mu.Unlock()
}

func applyHistoryMediaUpdate(item *sharedCompat.HistoryItem, update historyMediaBackfillUpdate) bool {
	if item == nil {
		return false
	}
	changed := false
	if strings.TrimSpace(update.ThumbPath) != "" && strings.TrimSpace(item.ThumbPath) == "" {
		item.ThumbPath = update.ThumbPath
		changed = true
	}
	if strings.TrimSpace(update.PreviewPath) != "" && strings.TrimSpace(item.PreviewPath) == "" {
		item.PreviewPath = update.PreviewPath
		changed = true
	}
	return changed
}

func applyHistoryMediaUpdatesToEntries(entries []historyPromptEntry, updates map[string]historyMediaBackfillUpdate) bool {
	changed := false
	for idx := range entries {
		if entries[idx].Item != nil {
			if update, ok := updates[entries[idx].Item.ID]; ok {
				changed = applyHistoryMediaUpdate(entries[idx].Item, update) || changed
			}
		}
		if entries[idx].Group != nil {
			if update, ok := updates[entries[idx].Group.Representative.ID]; ok {
				changed = applyHistoryMediaUpdate(&entries[idx].Group.Representative, update) || changed
			}
			for _, item := range entries[idx].Group.Items {
				if item == nil {
					continue
				}
				if update, ok := updates[item.ID]; ok {
					changed = applyHistoryMediaUpdate(item, update) || changed
				}
			}
		}
	}
	return changed
}

func applyHistoryMediaUpdatesToDayGroups(groups []historyDayGroup, updates map[string]historyMediaBackfillUpdate) bool {
	changed := false
	for idx := range groups {
		changed = applyHistoryMediaUpdatesToEntries(groups[idx].Entries, updates) || changed
	}
	return changed
}

func applyHistoryMediaUpdatesToHistoryItems(items []sharedCompat.HistoryItem, updates map[string]historyMediaBackfillUpdate) bool {
	changed := false
	for idx := range items {
		if update, ok := updates[items[idx].ID]; ok {
			changed = applyHistoryMediaUpdate(&items[idx], update) || changed
		}
	}
	return changed
}

func (a *App) persistHistoryThumbBackfill(thumbUpdates map[string]historyMediaBackfillUpdate) error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return err
	}
	changed := false
	for i := range state.History {
		update, ok := thumbUpdates[state.History[i].ID]
		if !ok {
			continue
		}
		if strings.TrimSpace(update.ThumbPath) != "" && strings.TrimSpace(state.History[i].ThumbPath) == "" {
			state.History[i].ThumbPath = update.ThumbPath
			changed = true
		}
		if strings.TrimSpace(update.PreviewPath) != "" && strings.TrimSpace(state.History[i].PreviewPath) == "" {
			state.History[i].PreviewPath = update.PreviewPath
			changed = true
		}
	}
	if !changed {
		return nil
	}
	state.UpdatedAt = time.Now().UnixMilli()
	return gioCompat.SaveState(state)
}

func (a *App) applyHistoryThumbBackfill(thumbUpdates map[string]historyMediaBackfillUpdate) {
	a.mu.Lock()
	changed := false
	for i := range a.history {
		update, ok := thumbUpdates[a.history[i].ID]
		if !ok {
			continue
		}
		changed = applyHistoryMediaUpdate(&a.history[i], update) || changed
	}
	if update, ok := thumbUpdates[a.result.Item.ID]; ok {
		changed = applyHistoryMediaUpdate(&a.result.Item, update) || changed
	}
	if update, ok := thumbUpdates[a.compare.Item.ID]; ok {
		changed = applyHistoryMediaUpdate(&a.compare.Item, update) || changed
	}
	if update, ok := thumbUpdates[a.activeResultDetail.ID]; ok {
		changed = applyHistoryMediaUpdate(&a.activeResultDetail, update) || changed
	}
	if update, ok := thumbUpdates[a.activePromptGroup.Representative.ID]; ok {
		changed = applyHistoryMediaUpdate(&a.activePromptGroup.Representative, update) || changed
	}
	for _, item := range a.activePromptGroup.Items {
		if item == nil {
			continue
		}
		update, ok := thumbUpdates[item.ID]
		if !ok {
			continue
		}
		changed = applyHistoryMediaUpdate(item, update) || changed
	}
	changed = applyHistoryMediaUpdate(&a.historyPanelCache.data.latest, thumbUpdates[a.historyPanelCache.data.latest.ID]) || changed
	changed = applyHistoryMediaUpdatesToEntries(a.historyPanelCache.data.entries, thumbUpdates) || changed
	changed = applyHistoryMediaUpdatesToDayGroups(a.historyTimelineCache.data.dayGroups, thumbUpdates) || changed
	changed = applyHistoryMediaUpdatesToHistoryItems(a.batchResultsSnapshot, thumbUpdates) || changed
	for idx := range a.historyGroupLookup.groups {
		if update, ok := thumbUpdates[a.historyGroupLookup.groups[idx].Representative.ID]; ok {
			changed = applyHistoryMediaUpdate(&a.historyGroupLookup.groups[idx].Representative, update) || changed
		}
		for _, item := range a.historyGroupLookup.groups[idx].Items {
			if item == nil {
				continue
			}
			if update, ok := thumbUpdates[item.ID]; ok {
				changed = applyHistoryMediaUpdate(item, update) || changed
			}
		}
	}
	if changed {
		a.pruneImageCacheLocked()
		a.snapshotReady = false
	}
	a.mu.Unlock()
	if changed {
		a.invalidateSoon(33 * time.Millisecond)
	}
}

func (a *App) applyPromptSuggestion(text string) {
	current := strings.TrimSpace(a.promptInput.Text())
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if current == "" {
		a.promptInput.SetText(text)
	} else {
		a.promptInput.SetText(current + "\n" + text)
	}
	a.promptHelperOpen = false
	a.invalidateNow()
}

func (a *App) useResultPrompt(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	a.promptInput.SetText(text)
	a.appendLog("已应用为下次提示词")
	a.closeResultDetail()
	a.invalidateNow()
}

func (a *App) prefillControlsFromHistoryItem(item sharedCompat.HistoryItem) {
	if prompt := strings.TrimSpace(item.Prompt); prompt != "" {
		a.promptInput.SetText(prompt)
	}
	if negative := strings.TrimSpace(item.NegativePrompt); negative != "" {
		a.negativePromptInput.SetText(negative)
	}
	if mode := strings.TrimSpace(item.Mode); mode == string(client.ModeEdit) || mode == string(client.ModeGenerate) {
		a.mode = mode
	}
	if size := strings.TrimSpace(item.Size); size != "" {
		a.size = size
	}
	if quality := strings.TrimSpace(item.Quality); quality != "" {
		a.quality = quality
	}
	if outputFormat := strings.TrimSpace(item.OutputFormat); outputFormat != "" {
		a.format = outputFormat
	}
	if background := strings.TrimSpace(item.Background); background != "" {
		a.background = background
	}
	if item.OutputCompression != nil && *item.OutputCompression > 0 {
		a.outputCompressionInput.SetText(strconv.Itoa(*item.OutputCompression))
	}
	if fidelity := strings.TrimSpace(item.InputFidelity); fidelity != "" {
		a.inputFidelity = fidelity
	}
	if imageStyle := strings.TrimSpace(item.ImageStyle); imageStyle != "" {
		a.imageStyle = imageStyle
	}
	if moderation := strings.TrimSpace(item.Moderation); moderation != "" {
		a.moderation = moderation
	}
	if item.Seed != 0 {
		a.seedInput.SetText(strconv.FormatInt(item.Seed, 10))
	}
	if styleTag := strings.TrimSpace(item.StyleTag); styleTag != "" {
		a.styleTag = styleTag
	}
	if item.BatchIndex > 0 {
		a.batchCount = normalizeBatchCount(item.BatchIndex + 1)
	}
	if strings.TrimSpace(item.Mode) == string(client.ModeEdit) {
		if len(item.SourcePaths) > 0 {
			a.setSourcePaths(item.SourcePaths)
		} else if strings.TrimSpace(item.SavedPath) != "" {
			a.sourcePathsInput.SetText(strings.TrimSpace(item.SavedPath))
			a.sourceButtons = map[string]*widget.Clickable{}
		}
	}
}

func (a *App) applyPreset(preset sharedCompat.Preset) {
	if strings.TrimSpace(preset.Size) != "" {
		a.size = preset.Size
	}
	if strings.TrimSpace(preset.Quality) != "" {
		a.quality = preset.Quality
	}
	if strings.TrimSpace(preset.OutputFormat) != "" {
		a.format = preset.OutputFormat
	}
	a.negativePromptInput.SetText(strings.TrimSpace(preset.NegativePrompt))
	if background := strings.TrimSpace(preset.Background); background != "" {
		a.background = background
	}
	if preset.OutputCompression != nil {
		a.outputCompressionInput.SetText(strconv.Itoa(*preset.OutputCompression))
	}
	if fidelity := strings.TrimSpace(preset.InputFidelity); fidelity != "" {
		a.inputFidelity = fidelity
	}
	if imageStyle := strings.TrimSpace(preset.ImageStyle); imageStyle != "" {
		a.imageStyle = imageStyle
	}
	if moderation := strings.TrimSpace(preset.Moderation); moderation != "" {
		a.moderation = moderation
	}
	a.styleTag = strings.TrimSpace(preset.StyleTag)
	if runtimeMode := strings.TrimSpace(preset.KernelRuntimeMode); runtimeMode != "" {
		a.kernelRuntimeMode = normalizeKernelRuntimeMode(runtimeMode)
	}
	a.batchCount = normalizeBatchCount(preset.BatchCount)
	a.promptHelperOpen = false
	a.appendLog("已应用预设: " + strings.TrimSpace(preset.Name))
	a.invalidateNow()
}

func (a *App) applyPresetByID(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	a.mu.Lock()
	presets := append([]sharedCompat.Preset(nil), a.presets...)
	a.mu.Unlock()
	for _, preset := range presets {
		if strings.TrimSpace(preset.ID) != id {
			continue
		}
		a.applyPreset(preset)
		return true
	}
	return false
}

func (a *App) rememberPrompt(text string) {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return
	}
	state, _, err := gioCompat.LoadState()
	if err != nil {
		a.appendLog("更新提示词历史失败: " + err.Error())
		return
	}
	next := []string{text}
	for _, existing := range state.Settings.PromptHistory {
		normalized := strings.Join(strings.Fields(strings.TrimSpace(existing)), " ")
		if normalized == "" || normalized == text {
			continue
		}
		next = append(next, normalized)
		if len(next) >= 50 {
			break
		}
	}
	state.Settings.PromptHistory = next
	state.UpdatedAt = time.Now().UnixMilli()
	if err := gioCompat.SaveState(state); err != nil {
		a.appendLog("保存提示词历史失败: " + err.Error())
		return
	}
	a.mu.Lock()
	a.setPromptHistoryLocked(next)
	a.setPresetsLocked(state.Settings.Presets)
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) sourcePaths() []string {
	return a.parseSourcePathsCached(a.sourcePathsInput.Text())
}

func (a *App) parseSourcePathsCached(text string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.parseSourcePathsCachedLocked(text)
}

func (a *App) parseSourcePathsCachedLocked(text string) []string {
	if a.sourcePathParseCache == nil {
		a.sourcePathParseCache = map[string][]string{}
	}
	if paths, ok := a.sourcePathParseCache[text]; ok {
		return paths
	}
	paths := kernel.ParseSourcePaths(text)
	a.sourcePathParseCache[text] = paths
	return paths
}

func (a *App) setSourcePaths(paths []string) {
	a.mu.Lock()
	a.sourcePathsInput.SetText(strings.Join(paths, "\n"))
	a.sourceButtons = map[string]*widget.Clickable{}
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) removeSourcePath(target string) {
	target = strings.TrimSpace(target)
	if target == "" {
		return
	}
	next := make([]string, 0, len(a.sourcePaths()))
	for _, path := range a.sourcePaths() {
		if strings.TrimSpace(path) == target {
			continue
		}
		next = append(next, path)
	}
	a.setSourcePaths(next)
}

func (a *App) appendSourcePath(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	paths := a.sourcePaths()
	for _, existing := range paths {
		if strings.TrimSpace(existing) == path {
			return
		}
	}
	paths = append(paths, path)
	a.setSourcePaths(paths)
}

func (a *App) reuseHistoryItemAsSource(item sharedCompat.HistoryItem) {
	if strings.TrimSpace(item.SavedPath) == "" {
		return
	}
	a.appendSourcePath(item.SavedPath)
}

func (a *App) deleteHistoryItem(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	state, _, err := gioCompat.LoadState()
	if err != nil {
		a.appendLog("删除历史失败: " + err.Error())
		return
	}
	next := make([]sharedCompat.HistoryItem, 0, len(state.History))
	for _, item := range state.History {
		if item.ID == id {
			continue
		}
		next = append(next, item)
	}
	state.History = next
	state.UpdatedAt = time.Now().UnixMilli()
	if err := gioCompat.SaveState(state); err != nil {
		a.appendLog("删除历史失败: " + err.Error())
		return
	}
	a.mu.Lock()
	a.setHistoryLocked(next)
	if len(a.batchResultIDs) > 0 {
		kept := make([]string, 0, len(a.batchResultIDs))
		for _, batchID := range a.batchResultIDs {
			if batchID != id {
				kept = append(kept, batchID)
			}
		}
		a.batchResultIDs = kept
		if !a.canOpenResultGridLocked() {
			a.resultGridOpen = false
		}
	}
	if a.selectedHistoryID == id {
		a.selectedHistoryID = ""
	}
	if a.compare.Item.ID == id {
		a.compare = resultState{Rev: a.compare.Rev + 1}
		a.compareSplitSlider.Value = 0.5
	}
	if a.activeResultDetail.ID == id {
		a.activeResultDetail = sharedCompat.HistoryItem{}
	}
	if a.result.Item.ID == id {
		a.result = resultState{Rev: a.result.Rev + 1}
	}
	if a.activePromptGroup.Key != "" && historyPromptGroupContains(a.activePromptGroup, id) {
		a.activePromptGroup = historyPromptGroup{}
	}
	a.appendLogLocked("已删除历史项: " + id)
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) persistThemeMode(mode string) {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		a.appendLog("保存主题失败: " + err.Error())
		return
	}
	state.Settings.Theme = normalizeThemeMode(mode)
	state.UpdatedAt = time.Now().UnixMilli()
	if err := gioCompat.SaveState(state); err != nil {
		a.appendLog("保存主题失败: " + err.Error())
		return
	}
	a.applyThemeMode(state.Settings.Theme)
}

func (a *App) openPromptGroup(group historyPromptGroup) {
	a.mu.Lock()
	a.activePromptGroup = group
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closePromptGroup() {
	a.mu.Lock()
	a.activePromptGroup = historyPromptGroup{}
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) startPromptOptimize() {
	if a.isRunning() {
		return
	}
	cfg := a.currentConfig()
	if normalizeKernelRuntimeMode(a.kernelRuntimeMode) == "remote" && cfg.ProxyMode != client.ProxyModeSystem {
		a.appendLog("优化提示词失败: 当前远程内核不能控制代理,请切回本地内核或使用 Android 原生运行")
		return
	}
	a.mu.Lock()
	if a.optimizingPrompt {
		a.mu.Unlock()
		return
	}
	a.optimizingPrompt = true
	a.appendLogLocked("开始优化提示词")
	a.mu.Unlock()
	a.invalidateNow()

	go func() {
		optimized, err := kernel.OptimizePrompt(context.Background(), cfg)
		a.mu.Lock()
		a.optimizingPrompt = false
		a.mu.Unlock()
		if err != nil {
			a.appendLog("优化提示词失败: " + err.Error())
			return
		}
		optimized = strings.TrimSpace(optimized)
		if optimized == "" {
			a.appendLog("优化提示词失败: 上游返回空结果")
			return
		}
		a.promptInput.SetText(optimized)
		a.rememberPrompt(optimized)
		a.appendLog("提示词已优化")
		a.invalidateNow()
	}()
}

func (a *App) startUpstreamProbe() {
	if a.isRunning() {
		return
	}
	cfg := a.currentConfig()
	a.mu.Lock()
	if a.testingUpstream {
		a.mu.Unlock()
		return
	}
	a.testingUpstream = true
	a.lastProbeSummary = ""
	a.lastProbeModels = nil
	a.appendLogLocked("开始测试上游连接")
	a.mu.Unlock()
	a.invalidateNow()

	go func() {
		result, err := probeUpstreamFunc(context.Background(), cfg)
		a.mu.Lock()
		a.testingUpstream = false
		if err != nil {
			a.lastProbeSummary = "测试失败"
			a.mu.Unlock()
			a.appendLog("上游测试失败: " + err.Error())
			return
		}
		switch {
		case result.ResponsesTransport == string(client.ResponsesTransportWebSocket) && !result.ResponsesTransportOK:
			a.lastProbeSummary = fmt.Sprintf("已连接 · %d models · WebSocket 不可用", result.ModelCount)
			a.appendLogLocked(fmt.Sprintf("上游测试成功: 发现 %d 个 models，但 Responses WebSocket 不可用: %s", result.ModelCount, strings.TrimSpace(result.ResponsesTransportError)))
		case result.ResponsesTransport == string(client.ResponsesTransportWebSocket):
			a.lastProbeSummary = fmt.Sprintf("已连接 · %d models · WebSocket 可用", result.ModelCount)
			a.appendLogLocked(fmt.Sprintf("上游测试成功: 发现 %d 个 models，Responses WebSocket 可用", result.ModelCount))
		default:
			a.lastProbeSummary = fmt.Sprintf("已连接 · %d models", result.ModelCount)
			a.appendLogLocked(fmt.Sprintf("上游测试成功: 发现 %d 个 models", result.ModelCount))
		}
		a.lastProbeModels = append([]kernel.UpstreamModelDescriptor(nil), result.Models...)
		a.mu.Unlock()
		a.invalidateNow()
	}()
}

func (a *App) testSettingsSelection() error {
	if err := a.saveSettingsSelection(); err != nil {
		return err
	}
	a.startUpstreamProbe()
	return nil
}

func (a *App) replaceCurrentResultWithPath(path string, sourceEvent string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("变换输出路径为空")
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	a.mu.Lock()
	item := a.result.Item
	item.SavedPath = path
	if item.ID == "" {
		item.ID = "derived:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	a.result = resultState{
		SavedPath:     path,
		RawPath:       a.result.RawPath,
		RevisedPrompt: a.result.RevisedPrompt,
		SourceEvent:   sourceEvent,
		Item:          item,
		HasItem:       true,
		Rev:           a.result.Rev + 1,
	}
	rev := a.result.Rev
	a.compare = resultState{Rev: a.compare.Rev + 1}
	a.compareSplitSlider.Value = 0.5
	a.selectedHistoryID = ""
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	a.invalidateNow()
	a.startAsyncCurrentResultImageLoad(path, item, sourceEvent, rev)
	return nil
}

func (a *App) startCurrentImageTransform(action string, sourceEvent string, transform func(path string) (string, error)) {
	currentPath := strings.TrimSpace(a.readSnapshot().Result.SavedPath)
	if currentPath == "" {
		return
	}
	a.mu.Lock()
	if a.processingImageTransform {
		a.mu.Unlock()
		return
	}
	a.processingImageTransform = true
	a.status = "正在" + action + "当前图片..."
	a.mu.Unlock()
	a.invalidateNow()

	go func(path string) {
		next, err := transform(path)
		if err != nil {
			a.mu.Lock()
			a.processingImageTransform = false
			a.mu.Unlock()
			a.appendLog(action + "失败: " + err.Error())
			a.invalidateNow()
			return
		}
		if err := a.replaceCurrentResultWithPath(next, sourceEvent); err != nil {
			a.mu.Lock()
			a.processingImageTransform = false
			a.mu.Unlock()
			a.appendLog("载入变换结果失败: " + err.Error())
			a.invalidateNow()
			return
		}
		a.mu.Lock()
		a.processingImageTransform = false
		a.status = action + "完成"
		a.appendLogLocked(action + "完成: " + next)
		a.mu.Unlock()
		a.invalidateNow()
	}(currentPath)
}

func (a *App) clearCurrentResult() {
	a.mu.Lock()
	a.result = resultState{Rev: a.result.Rev + 1}
	a.compare = resultState{Rev: a.compare.Rev + 1}
	a.compareSplitSlider.Value = 0.5
	a.selectedHistoryID = ""
	a.activePromptGroup = historyPromptGroup{}
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) isCompareItem(item sharedCompat.HistoryItem) bool {
	id := strings.TrimSpace(item.ID)
	if id == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.compare.Item.ID == id
}

func compareItemActive(itemID string, compareItemID string) bool {
	itemID = strings.TrimSpace(itemID)
	compareItemID = strings.TrimSpace(compareItemID)
	return itemID != "" && itemID == compareItemID
}

func (a *App) toggleCompareItem(item sharedCompat.HistoryItem) error {
	if strings.TrimSpace(item.ID) == "" {
		a.clearCompare()
		return nil
	}
	if a.isCompareItem(item) {
		a.clearCompare()
		return nil
	}
	return a.setCompareItem(item)
}

func (a *App) setCompareItem(item sharedCompat.HistoryItem) error {
	if strings.TrimSpace(item.ID) == "" && strings.TrimSpace(item.SavedPath) == "" {
		a.clearCompare()
		return nil
	}
	state := resultState{
		SavedPath:     item.SavedPath,
		RawPath:       item.RawPath,
		RevisedPrompt: item.RevisedPrompt,
		SourceEvent:   "compare",
		Item:          item,
		HasItem:       item.ID != "",
	}
	state.Image = a.loadCanvasImmediatePreviewForState(item.SavedPath, state)
	a.mu.Lock()
	state.Rev = a.compare.Rev + 1
	a.compare = state
	rev := a.compare.Rev
	a.compareSplitSlider.Value = 0.5
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	a.invalidateNow()
	a.startAsyncCompareImageLoad(item, rev)
	return nil
}

func (a *App) startAsyncHistoryResultImageLoad(item sharedCompat.HistoryItem, rev int) {
	runThumbDecodeAsync(func() {
		state := resultState{
			SavedPath:     item.SavedPath,
			RawPath:       item.RawPath,
			RevisedPrompt: item.RevisedPrompt,
			SourceEvent:   "history",
			Item:          item,
			HasItem:       item.ID != "",
		}
		img := a.loadCanvasDisplayImageForState(item.SavedPath, state)
		if img == nil {
			return
		}
		a.persistManagedCanvasPreviewVariants(item.SavedPath, a.effectiveCanvasMaxDimension(), img)
		a.mu.Lock()
		if a.result.Rev != rev || a.result.Item.ID != item.ID || a.selectedHistoryID != item.ID {
			a.mu.Unlock()
			return
		}
		a.result.Image = img
		a.result.Rev++
		a.imageOpRev = 0
		a.mu.Unlock()
		a.invalidateSoon(33 * time.Millisecond)
	})
}

func (a *App) startAsyncCompareImageLoad(item sharedCompat.HistoryItem, rev int) {
	runThumbDecodeAsync(func() {
		state := resultState{
			SavedPath:     item.SavedPath,
			RawPath:       item.RawPath,
			RevisedPrompt: item.RevisedPrompt,
			SourceEvent:   "compare",
			Item:          item,
			HasItem:       item.ID != "",
		}
		img := a.loadCanvasDisplayImageForState(item.SavedPath, state)
		if img == nil {
			return
		}
		a.persistManagedCanvasPreviewVariants(item.SavedPath, a.effectiveCanvasMaxDimension(), img)
		a.mu.Lock()
		if a.compare.Rev != rev || a.compare.Item.ID != item.ID {
			a.mu.Unlock()
			return
		}
		a.compare.Image = img
		a.compare.Rev++
		a.compareImageOpRev = 0
		a.mu.Unlock()
		a.invalidateSoon(33 * time.Millisecond)
	})
}

func (a *App) startAsyncCurrentResultImageLoad(path string, item sharedCompat.HistoryItem, sourceEvent string, rev int) {
	runThumbDecodeAsync(func() {
		state := resultState{
			SavedPath:   path,
			SourceEvent: sourceEvent,
			Item:        item,
			HasItem:     true,
		}
		img := a.loadCanvasDisplayImageForState(path, state)
		if img == nil {
			return
		}
		a.persistManagedCanvasPreviewVariants(path, a.effectiveCanvasMaxDimension(), img)
		a.mu.Lock()
		if a.result.Rev != rev || a.result.SavedPath != path || a.result.Item.ID != item.ID {
			a.mu.Unlock()
			return
		}
		a.result.Image = img
		a.result.Rev++
		a.imageOpRev = 0
		a.mu.Unlock()
		a.invalidateSoon(33 * time.Millisecond)
	})
}

func (a *App) clearCompare() {
	a.mu.Lock()
	a.compare = resultState{Rev: a.compare.Rev + 1}
	a.compareSplitSlider.Value = 0.5
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) toggleFullscreen() {
	a.mu.Lock()
	next := !a.fullscreen
	a.fullscreen = next
	window := a.window
	a.mu.Unlock()
	if window != nil {
		if next {
			window.Option(app.Fullscreen.Option())
		} else {
			window.Option(app.Windowed.Option())
		}
	}
	a.invalidateNow()
}

func (a *App) openResultDetail(item sharedCompat.HistoryItem) {
	if strings.TrimSpace(item.ID) == "" && strings.TrimSpace(item.SavedPath) == "" {
		return
	}
	a.mu.Lock()
	a.activeResultDetail = item
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closeResultDetail() {
	a.mu.Lock()
	a.activeResultDetail = sharedCompat.HistoryItem{}
	a.mu.Unlock()
	a.invalidateNow()
}
