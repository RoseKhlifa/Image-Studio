package ui

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	defaultBatchProcessConcurrency = 2
	maxBatchProcessConcurrency     = 9
	batchOutputModeSourceDir       = "source_dir"
	batchOutputModeCustomDir       = "custom_dir"
)

func normalizeBatchProcessConcurrency(value int) int {
	if value < 1 {
		return defaultBatchProcessConcurrency
	}
	if value > maxBatchProcessConcurrency {
		return maxBatchProcessConcurrency
	}
	return value
}

func (a *App) setBatchConcurrency(value int) {
	a.batchConcurrency = normalizeBatchProcessConcurrency(value)
	a.batchConcurrencyInput.SetText(strconv.Itoa(a.batchConcurrency))
}

func (a *App) syncBatchSettingsFromInputs() {
	if raw := strings.TrimSpace(a.batchConcurrencyInput.Text()); raw == "" {
		a.batchConcurrency = defaultBatchProcessConcurrency
		a.batchConcurrencyInput.SetText(strconv.Itoa(a.batchConcurrency))
	} else if value, err := strconv.Atoi(raw); err == nil {
		normalized := normalizeBatchProcessConcurrency(value)
		a.batchConcurrency = normalized
		if normalizedText := strconv.Itoa(normalized); normalizedText != raw {
			a.batchConcurrencyInput.SetText(normalizedText)
		}
	}
}

func normalizeBatchOutputMode(value string) string {
	if strings.TrimSpace(value) == batchOutputModeCustomDir {
		return batchOutputModeCustomDir
	}
	return batchOutputModeSourceDir
}

func (a *App) effectiveBatchOutputDir() string {
	if normalizeBatchOutputMode(a.batchOutputMode) != batchOutputModeCustomDir {
		return ""
	}
	return strings.TrimSpace(a.batchOutputDirInput.Text())
}

func (a *App) setBatchOutputMode(mode string) {
	a.batchOutputMode = normalizeBatchOutputMode(mode)
	if a.batchOutputMode == batchOutputModeSourceDir {
		a.batchOutputDirInput.SetText("")
		a.batchOutputDir = ""
	}
}

func (a *App) refreshBatchInputDir(logPrefix string) {
	logPrefix = strings.TrimSpace(logPrefix)
	if logPrefix != "" && !strings.HasSuffix(logPrefix, " ") {
		logPrefix += " "
	}
	if manual := a.sourcePaths(); len(manual) > 0 {
		a.appendLog(fmt.Sprintf("%s当前使用手动队列，共 %d 张。", logPrefix, len(manual)))
		a.invalidateNow()
		return
	}
	dir := strings.TrimSpace(a.batchInputDirInput.Text())
	if dir == "" {
		a.appendLog(logPrefix + "请先选择批处理输入目录。")
		return
	}
	paths, err := a.batchSourcePathsForRun()
	if err != nil {
		a.appendLog(logPrefix + "刷新批处理目录失败: " + err.Error())
		return
	}
	a.appendLog(fmt.Sprintf("%s已刷新批处理目录，共 %d 张。", logPrefix, len(paths)))
	a.invalidateNow()
}
