package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/kernel"
	sharedCompat "image-studio/shared/compat"
)

func materializedHistoryPath(item sharedCompat.HistoryItem, outputDir string) string {
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		outputDir = kernel.DefaultOutputDir()
	}
	return filepath.Join(outputDir, "images", suggestedSaveNameForHistoryItem(item))
}

func mergeMaterializedHistoryItem(dst *sharedCompat.HistoryItem, src sharedCompat.HistoryItem) bool {
	if dst == nil {
		return false
	}
	changed := false
	if strings.TrimSpace(dst.SavedPath) == "" && strings.TrimSpace(src.SavedPath) != "" {
		dst.SavedPath = src.SavedPath
		changed = true
	}
	if strings.TrimSpace(dst.PreviewPath) == "" && strings.TrimSpace(src.PreviewPath) != "" {
		dst.PreviewPath = src.PreviewPath
		changed = true
	}
	if strings.TrimSpace(dst.ThumbPath) == "" && strings.TrimSpace(src.ThumbPath) != "" {
		dst.ThumbPath = src.ThumbPath
		changed = true
	}
	return changed
}

func (a *App) persistMaterializedHistoryItem(next sharedCompat.HistoryItem) {
	nextID := strings.TrimSpace(next.ID)
	if nextID != "" {
		err := gioCompat.UpdateState(func(state *sharedCompat.State) error {
			*state = sharedCompat.Normalize(*state)
			changed := false
			for idx := range state.History {
				if strings.TrimSpace(state.History[idx].ID) != nextID {
					continue
				}
				changed = mergeMaterializedHistoryItem(&state.History[idx], next) || changed
			}
			if changed {
				state.UpdatedAt = 0
			}
			return nil
		})
		if err != nil {
			a.appendLog("保存落盘历史失败: " + err.Error())
		}
	}

	a.mu.Lock()
	changed := false
	if nextID != "" {
		for idx := range a.history {
			if strings.TrimSpace(a.history[idx].ID) != nextID {
				continue
			}
			changed = mergeMaterializedHistoryItem(&a.history[idx], next) || changed
		}
	}
	if changed {
		a.setHistoryLocked(a.history)
	}
	if strings.TrimSpace(a.result.Item.ID) == nextID {
		changed = mergeMaterializedHistoryItem(&a.result.Item, next) || changed
		if strings.TrimSpace(a.result.SavedPath) == "" && strings.TrimSpace(next.SavedPath) != "" {
			a.result.SavedPath = next.SavedPath
			a.result.Rev++
		}
	}
	if strings.TrimSpace(a.compare.Item.ID) == nextID {
		changed = mergeMaterializedHistoryItem(&a.compare.Item, next) || changed
		if strings.TrimSpace(a.compare.SavedPath) == "" && strings.TrimSpace(next.SavedPath) != "" {
			a.compare.SavedPath = next.SavedPath
			a.compare.Rev++
		}
	}
	if strings.TrimSpace(a.activeResultDetail.ID) == nextID {
		changed = mergeMaterializedHistoryItem(&a.activeResultDetail, next) || changed
	}
	if strings.TrimSpace(a.activePromptGroup.Representative.ID) == nextID {
		changed = mergeMaterializedHistoryItem(&a.activePromptGroup.Representative, next) || changed
	}
	for _, item := range a.activePromptGroup.Items {
		if item == nil || strings.TrimSpace(item.ID) != nextID {
			continue
		}
		changed = mergeMaterializedHistoryItem(item, next) || changed
	}
	if changed {
		a.pruneImageCacheLocked()
	}
	a.mu.Unlock()
	if changed {
		a.invalidateNow()
	}
}

func (a *App) materializeHistoryItemForLocalPath(item sharedCompat.HistoryItem) (sharedCompat.HistoryItem, error) {
	if strings.TrimSpace(item.SavedPath) != "" {
		return item, nil
	}
	if strings.TrimSpace(item.ImageB64) == "" {
		return item, fmt.Errorf("当前结果没有可落盘图片数据")
	}
	target := materializedHistoryPath(item, a.outputDirInput.Text())
	savedPath, err := saveImageB64ToPath(item.ImageB64, filepath.Base(target), target)
	if err != nil {
		return item, err
	}
	previewPath, thumbPath, _ := kernel.EnsurePreviewAndThumbForPath(savedPath)
	next := item
	next.SavedPath = savedPath
	if strings.TrimSpace(previewPath) != "" {
		next.PreviewPath = previewPath
	}
	if strings.TrimSpace(thumbPath) != "" {
		next.ThumbPath = thumbPath
	}
	a.persistMaterializedHistoryItem(next)
	return next, nil
}

func (a *App) ensureCurrentResultSavedPathIfNeeded() (string, error) {
	snap := a.readSnapshot()
	if path := strings.TrimSpace(snap.Result.SavedPath); path != "" {
		return path, nil
	}
	next, err := a.materializeHistoryItemForLocalPath(snap.Result.Item)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(next.SavedPath), nil
}
