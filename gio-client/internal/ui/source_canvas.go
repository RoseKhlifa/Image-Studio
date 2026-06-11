package ui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	sharedCompat "image-studio/shared/compat"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

func sourceCanvasItemFromPath(path string) (sharedCompat.HistoryItem, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return sharedCompat.HistoryItem{}, fmt.Errorf("参考图路径为空")
	}
	if !isVirtualImagePath(path) {
		if _, err := os.Stat(path); err != nil {
			return sharedCompat.HistoryItem{}, err
		}
	}
	name := filepath.Base(path)
	if name == "." || name == string(os.PathSeparator) || strings.TrimSpace(name) == "" {
		name = "source"
	}
	item := sharedCompat.HistoryItem{
		ID:          "source-preview:" + path,
		Prompt:      "(参考图)" + name,
		Mode:        string(client.ModeEdit),
		Size:        "auto",
		Quality:     "medium",
		CreatedAt:   time.Now().UnixMilli(),
		SavedPath:   path,
		PreviewOnly: true,
	}
	if isVirtualImagePath(path) {
		if imageB64, ok := readVirtualImageB64(path); ok {
			item.ImageB64 = imageB64
		}
	} else if previewPath, err := ensureManagedSourcePreview(path, historyPreviewPathMaxDimension); err == nil {
		item.PreviewPath = previewPath
	}
	return item, nil
}

func (a *App) viewSourcePathOnCanvas(path string) error {
	item, err := sourceCanvasItemFromPath(path)
	if err != nil {
		return err
	}
	state := resultState{
		SavedPath:   item.SavedPath,
		SourceEvent: "source-preview",
		Item:        item,
		HasItem:     true,
	}
	state.Image = a.loadSourceImmediatePreview(item.SavedPath, state)
	a.mu.Lock()
	a.mode = string(client.ModeEdit)
	state.Rev = a.result.Rev + 1
	a.result = state
	a.selectedHistoryID = ""
	a.status = "已查看参考图"
	rev := a.result.Rev
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	a.invalidateNow()
	a.startAsyncCurrentResultImageLoad(item.SavedPath, item, state.SourceEvent, rev)
	return nil
}

func (a *App) importImagePathAsEditSource(path string) error {
	item, err := sourceCanvasItemFromPath(path)
	if err != nil {
		return err
	}
	state := resultState{
		SavedPath:   item.SavedPath,
		SourceEvent: "import",
		Item:        item,
		HasItem:     true,
	}
	state.Image = a.loadSourceImmediatePreview(item.SavedPath, state)
	a.mu.Lock()
	a.mode = string(client.ModeEdit)
	a.batchMode = false
	state.Rev = a.result.Rev + 1
	a.result = state
	a.selectedHistoryID = ""
	a.status = "已导入本地图片"
	rev := a.result.Rev
	a.pruneImageCacheLocked()
	a.mu.Unlock()
	existing := a.sourcePaths()
	alreadyIn := false
	for _, candidate := range existing {
		if strings.TrimSpace(candidate) == strings.TrimSpace(path) {
			alreadyIn = true
			break
		}
	}
	if !alreadyIn {
		if len(existing) == 0 {
			a.setSourcePaths([]string{path})
		} else {
			a.appendSourcePath(path)
		}
	}
	a.clearCompare()
	a.resetCanvasView()
	a.invalidateNow()
	a.startAsyncCurrentResultImageLoad(item.SavedPath, item, state.SourceEvent, rev)
	return nil
}

func (a *App) compareSourcePathOnCanvas(path string) error {
	item, err := sourceCanvasItemFromPath(path)
	if err != nil {
		return err
	}
	snap := a.readSnapshot()
	if !snap.Result.HasItem && strings.TrimSpace(snap.Result.SavedPath) == "" && strings.TrimSpace(snap.Result.Item.ImageB64) == "" {
		return fmt.Errorf("先在画板显示结果图后再对比参考图")
	}
	if compareItemActive(item.ID, snap.Compare.Item.ID) {
		a.clearCompare()
		return nil
	}
	state := resultState{
		SavedPath:     item.SavedPath,
		RawPath:       item.RawPath,
		RevisedPrompt: item.RevisedPrompt,
		SourceEvent:   "compare",
		Item:          item,
		HasItem:       true,
	}
	state.Image = a.loadSourceImmediatePreview(item.SavedPath, state)
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

func (a *App) loadSourceImmediatePreview(savedPath string, state resultState) image.Image {
	if img := a.loadCanvasImmediatePreviewForState(savedPath, state); img != nil {
		return img
	}
	if strings.TrimSpace(savedPath) == "" || isVirtualImagePath(savedPath) {
		return nil
	}
	img, err := a.imageForPathThumb(savedPath, historyPreviewPathMaxDimension)
	if err != nil {
		return nil
	}
	return img
}
