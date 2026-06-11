package ui

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

func (a *App) loadCanvasDisplayImageForState(savedPath string, item resultState) image.Image {
	target := a.effectiveCanvasMaxDimension()
	if path := strings.TrimSpace(savedPath); path != "" {
		if cached, ok := a.getCachedImage(pathThumbCacheKey(path, target)); ok && !cached.Loading && !cached.Failed && cached.Image != nil {
			atomic.AddUint64(&canvasDisplaySourcePathThumb, 1)
			return cached.Image
		}
		if img, err := a.loadExistingManagedCanvasPreview(path, target); err == nil {
			atomic.AddUint64(&canvasDisplaySourceManagedPreview, 1)
			return img
		}
		img, err := a.imageForPathThumb(path, target)
		if err == nil {
			atomic.AddUint64(&canvasDisplaySourcePathThumb, 1)
			return img
		}
	}
	if historyItem := item.Item; historyItem.ID != "" || strings.TrimSpace(historyItem.SavedPath) != "" || strings.TrimSpace(historyItem.ImageB64) != "" {
		img, err := a.loadHistoryImageScaled(historyItem, target)
		if err == nil {
			atomic.AddUint64(&canvasDisplaySourceHistoryScaled, 1)
			return img
		}
	}
	if item.Image != nil {
		atomic.AddUint64(&canvasDisplaySourceInline, 1)
		return downscaleToMaxDimension(item.Image, target)
	}
	return nil
}

func (a *App) loadExistingManagedCanvasPreview(savedPath string, maxDimension int) (image.Image, error) {
	previewPath, err := managedSourcePreviewPath(savedPath, maxDimension)
	if err == nil && headlessPathReady(previewPath) {
		cached, err := a.cachedImageForPath(previewPath)
		if err != nil {
			return nil, err
		}
		a.setCachedImage(pathThumbCacheKey(savedPath, maxDimension), cached)
		return cached.Image, nil
	}
	if maxDimension < canvasDisplayMaxDimension {
		fallbackPath, fallbackErr := managedSourcePreviewPath(savedPath, canvasDisplayMaxDimension)
		if fallbackErr == nil && headlessPathReady(fallbackPath) {
			cached, err := a.cachedImageForPath(fallbackPath)
			if err != nil {
				return nil, err
			}
			derived := resizedCachedImage(cached, maxDimension)
			a.setCachedImage(pathThumbCacheKey(savedPath, maxDimension), derived)
			return derived.Image, nil
		}
	}
	return nil, errMissingPreview
}

func (a *App) persistManagedCanvasPreview(savedPath string, maxDimension int, img image.Image) {
	if strings.TrimSpace(savedPath) == "" || img == nil || maxDimension <= 0 {
		return
	}
	previewPath, err := managedSourcePreviewPath(savedPath, maxDimension)
	if err != nil {
		return
	}
	sourceInfo, err := os.Stat(savedPath)
	if err != nil || sourceInfo.IsDir() {
		return
	}
	if previewInfo, statErr := os.Stat(previewPath); statErr == nil && !previewInfo.IsDir() && !sourceInfo.ModTime().After(previewInfo.ModTime()) {
		return
	}
	if err := os.MkdirAll(filepath.Dir(previewPath), 0o700); err != nil {
		return
	}
	file, err := os.OpenFile(previewPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	ok := false
	defer func() {
		_ = file.Close()
		if !ok {
			_ = os.Remove(previewPath)
		}
	}()
	if err := png.Encode(file, img); err != nil {
		return
	}
	if err := file.Close(); err != nil {
		return
	}
	ok = true
}

func (a *App) persistManagedCanvasPreviewVariants(savedPath string, primaryTarget int, img image.Image) {
	if strings.TrimSpace(savedPath) == "" || img == nil || primaryTarget <= 0 {
		return
	}
	primary := downscaleToMaxDimension(img, primaryTarget)
	a.persistManagedCanvasPreview(savedPath, primaryTarget, primary)
	if primaryTarget > reducedEffectsCanvasDisplayMaxDimension {
		reduced := downscaleToMaxDimension(primary, reducedEffectsCanvasDisplayMaxDimension)
		a.persistManagedCanvasPreview(savedPath, reducedEffectsCanvasDisplayMaxDimension, reduced)
	}
}

func (a *App) loadCanvasImmediatePreviewForState(savedPath string, item resultState) image.Image {
	historyItem := item.Item
	if strings.TrimSpace(historyItem.ImageB64) != "" {
		if img, err := decodeImageB64(historyItem.ImageB64); err == nil {
			return downscaleToMaxDimension(img, historyThumbFallbackMaxDimension)
		}
	}
	if previewPath := strings.TrimSpace(historyItem.PreviewPath); previewPath != "" {
		if img, err := a.imageForPathThumb(previewPath, historyPreviewPathMaxDimension); err == nil {
			return img
		}
	}
	if thumbPath := strings.TrimSpace(historyItem.ThumbPath); thumbPath != "" {
		if img, err := a.imageForPathThumb(thumbPath, historyThumbFallbackMaxDimension); err == nil {
			return img
		}
	}
	if item.Image != nil {
		return downscaleToMaxDimension(item.Image, historyThumbFallbackMaxDimension)
	}
	return nil
}

func (a *App) applyReducedEffects(enabled bool) {
	a.mu.Lock()
	if a.reducedEffects == enabled {
		a.mu.Unlock()
		return
	}
	a.reducedEffects = enabled
	currentResult := a.result
	currentCompare := a.compare
	a.imageCache = map[string]cachedImage{}
	a.mu.Unlock()

	resultImg := a.loadCanvasDisplayImageForState(currentResult.SavedPath, currentResult)
	compareImg := a.loadCanvasDisplayImageForState(currentCompare.SavedPath, currentCompare)

	a.mu.Lock()
	if a.reducedEffects != enabled {
		a.mu.Unlock()
		return
	}
	if resultImg != nil {
		a.result.Image = resultImg
		a.result.Rev++
	}
	if compareImg != nil {
		a.compare.Image = compareImg
		a.compare.Rev++
	}
	a.imageOpRev = 0
	a.compareImageOpRev = 0
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	a.invalidateNow()
}
