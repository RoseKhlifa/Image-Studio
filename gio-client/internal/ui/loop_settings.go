package ui

import (
	"strconv"
	"strings"
)

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
