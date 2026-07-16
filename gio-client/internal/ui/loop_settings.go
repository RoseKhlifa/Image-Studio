package ui

import (
	"strconv"
	"strings"
)

func (a *App) openLoopModal() {
	a.loopModalOpen = true
	a.invalidateNow()
}

func (a *App) closeLoopModal() {
	a.loopModalOpen = false
	a.invalidateNow()
}

func (a *App) setLoopEnabled(enabled bool) {
	wasEnabled := a.loopEnabled
	a.loopEnabled = enabled
	if enabled && !wasEnabled {
		a.openLoopModal()
		return
	}
	a.invalidateNow()
}

func (a *App) setLoopTotalCount(value int) {
	a.loopTotalCount = normalizeLoopGenerationCount(value)
	a.loopTotalCountInput.SetText(strconv.Itoa(a.loopTotalCount))
}

func (a *App) setLoopConcurrency(value int) {
	a.loopConcurrency = normalizeLoopGenerationConcurrency(value)
	a.loopConcurrencyInput.SetText(strconv.Itoa(a.loopConcurrency))
}

func (a *App) syncLoopInputsFromState() {
	a.loopTotalCount = normalizeLoopGenerationCount(a.loopTotalCount)
	a.loopConcurrency = normalizeLoopGenerationConcurrency(a.loopConcurrency)
	a.loopTotalCountInput.SetText(strconv.Itoa(a.loopTotalCount))
	a.loopConcurrencyInput.SetText(strconv.Itoa(a.loopConcurrency))
}

func (a *App) syncLoopSettingsFromInputs() {
	if raw := strings.TrimSpace(a.loopTotalCountInput.Text()); raw == "" {
		a.loopTotalCount = defaultLoopGenerationCount
	} else if value, err := strconv.Atoi(raw); err == nil {
		normalized := normalizeLoopGenerationCount(value)
		a.loopTotalCount = normalized
		if normalizedText := strconv.Itoa(normalized); normalizedText != raw {
			a.loopTotalCountInput.SetText(normalizedText)
		}
	}
	if raw := strings.TrimSpace(a.loopConcurrencyInput.Text()); raw == "" {
		a.loopConcurrency = defaultLoopGenerationConcurrency
	} else if value, err := strconv.Atoi(raw); err == nil {
		normalized := normalizeLoopGenerationConcurrency(value)
		a.loopConcurrency = normalized
		if normalizedText := strconv.Itoa(normalized); normalizedText != raw {
			a.loopConcurrencyInput.SetText(normalizedText)
		}
	}
}

func (a *App) setLoopAutoSaveEnabled(enabled bool) {
	a.loopAutoSave = enabled
	if !enabled {
		return
	}
	if strings.TrimSpace(a.loopAutoSaveDirInput.Text()) != "" {
		return
	}
	if dir := strings.TrimSpace(a.outputDirInput.Text()); dir != "" {
		a.loopAutoSaveDirInput.SetText(dir)
	}
}

func (a *App) useCurrentOutputDirForLoopAutoSave() {
	dir := strings.TrimSpace(a.outputDirInput.Text())
	if dir == "" {
		return
	}
	a.loopAutoSaveDirInput.SetText(dir)
	a.loopAutoSave = true
}

func (a *App) chooseLoopAutoSaveDir(logPrefix string) {
	dir, err := chooseDirectory()
	if err != nil {
		a.appendLog(logPrefix + err.Error())
		return
	}
	if strings.TrimSpace(dir) == "" {
		return
	}
	a.loopAutoSaveDirInput.SetText(dir)
	a.loopAutoSave = true
}

func loopAutoSaveDirPlaceholder(currentOutputDir string) string {
	currentOutputDir = strings.TrimSpace(currentOutputDir)
	if currentOutputDir != "" {
		return currentOutputDir
	}
	return "请输入或选择自动另存为目录"
}

func loopAutoSaveDirLabel(dir string) string {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "待选路径"
	}
	parts := strings.FieldsFunc(dir, func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(parts) == 0 {
		return dir
	}
	return parts[len(parts)-1]
}

func (a *App) loopSummaryText() string {
	if !a.loopEnabled {
		return "关闭"
	}
	parts := []string{
		strconv.Itoa(normalizeLoopGenerationCount(a.loopTotalCount)) + " 张",
		"并发 " + strconv.Itoa(normalizeLoopGenerationConcurrency(a.loopConcurrency)),
	}
	if a.loopLivePreview {
		parts = append(parts, "实时预览开")
	} else {
		parts = append(parts, "实时预览关")
	}
	if a.loopAutoSave {
		parts = append(parts, "自动另存为 · "+loopAutoSaveDirLabel(a.loopAutoSaveDirInput.Text()))
	}
	return strings.Join(parts, " · ")
}
