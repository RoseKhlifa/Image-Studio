package ui

import (
	"os"
	"strings"
	"time"

	"gioui.org/widget"
	"github.com/yuanhua/image-gptcodex/pkg/client"
	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/kernel"
	sharedCompat "image-studio/shared/compat"
)

func (a *App) saveCurrentConfig() {
	if a.settingsModalOpen && strings.TrimSpace(a.settingsSelectedProfileID) != "" && a.settingsSelectedProfileID != a.activeProfileID {
		_ = a.restoreActiveRuntimeConfig(false)
	}
	if err := gioCompat.SaveConfig(a.currentConfig()); err != nil {
		a.appendLog("兼容配置保存失败: " + err.Error())
	}
	if err := a.persistGeneralSettings(); err != nil {
		a.appendLog("通用设置保存失败: " + err.Error())
	}
	if err := a.saveActiveProfileMetadata(); err != nil {
		a.appendLog("配置元数据保存失败: " + err.Error())
	}
}

func (a *App) cancelRun() {
	a.mu.Lock()
	cancel := a.cancel
	if cancel != nil {
		a.cancel = nil
		a.running = false
		a.status = "已取消"
		a.appendLogLocked("任务已取消")
	}
	a.mu.Unlock()
	if cancel != nil {
		cancel()
		a.invalidateNow()
	}
}

func (a *App) finishWithError(err error, rawPath string) {
	a.mu.Lock()
	a.running = false
	a.cancel = nil
	a.status = "失败"
	a.lastErrorMessage = strings.TrimSpace(err.Error())
	if rawPath != "" {
		a.result.RawPath = rawPath
	}
	a.appendLogLocked("失败: " + err.Error())
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) finishCancelled() {
	a.mu.Lock()
	a.running = false
	a.cancel = nil
	a.status = "已取消"
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) appendLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	a.mu.Lock()
	a.appendLogLocked(line)
	a.mu.Unlock()
	a.invalidateSoon(33 * time.Millisecond)
}

func (a *App) appendLogLocked(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	a.logs = appendBounded(a.logs, line)
	a.logsRev++
}

func (a *App) setStatus(status string) {
	status = strings.TrimSpace(status)
	a.mu.Lock()
	if a.status == status {
		a.mu.Unlock()
		return
	}
	a.status = status
	a.mu.Unlock()
	a.invalidateSoon(33 * time.Millisecond)
}

func (a *App) clearLogs() {
	a.mu.Lock()
	a.clearLogsLocked()
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) clearLogsLocked() {
	if len(a.logs) == 0 {
		return
	}
	a.logs = nil
	a.logsRev++
}

func (a *App) closeSavePrompt() {
	a.mu.Lock()
	a.savePromptVisible = false
	a.savePromptSourcePath = ""
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) openHistoryTimeline() {
	a.mu.Lock()
	a.historyTimelineOpen = true
	a.historyTimelineModeFilter = a.historyModeFilter
	a.historyTimelineDateFilter = a.historyDateFilter
	a.historyTimelineModePickerOpen = false
	a.historyTimelineDatePickerOpen = false
	a.historyTimelineQueryInput.SetText(a.historyQueryInput.Text())
	a.expandedPromptGroups = map[string]bool{}
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closeHistoryTimeline() {
	a.mu.Lock()
	a.historyTimelineOpen = false
	a.historyTimelineModePickerOpen = false
	a.historyTimelineDatePickerOpen = false
	a.expandedPromptGroups = map[string]bool{}
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) batchGridCountLocked() int {
	total := len(a.batchResultIDs)
	if a.running && a.lastRunBatchCount > total {
		total = a.lastRunBatchCount
	}
	return total
}

func (a *App) canOpenResultGridLocked() bool {
	return a.batchGridCountLocked() > 1
}

func (a *App) openResultGrid() {
	a.mu.Lock()
	if !a.canOpenResultGridLocked() {
		a.mu.Unlock()
		return
	}
	a.resultGridOpen = true
	a.compare = resultState{Rev: a.compare.Rev + 1}
	a.compareSplitSlider.Value = 0.5
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closeResultGrid() {
	a.mu.Lock()
	a.resultGridOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) openSavePromptForCurrent() {
	a.mu.Lock()
	src := strings.TrimSpace(a.result.SavedPath)
	a.mu.Unlock()
	a.openSavePromptForPath(src)
}

func (a *App) openSavePromptForPath(path string) {
	src := strings.TrimSpace(path)
	if src == "" {
		return
	}
	a.mu.Lock()
	a.savePromptVisible = true
	a.savePromptSourcePath = src
	a.savePromptPathInput.SetText(src)
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) setSavePromptSuppressed(value bool) {
	a.mu.Lock()
	a.savePromptSuppressed = value
	a.savePromptNeverAsk.Value = value
	a.mu.Unlock()
	if err := gioCompat.SetSavePromptSuppressed(value); err != nil {
		a.appendLog("保存提示设置失败: " + err.Error())
	}
	a.invalidateNow()
}

func (a *App) savePromptCopy() {
	a.mu.Lock()
	src := a.savePromptSourcePath
	dst := a.savePromptPathInput.Text()
	a.mu.Unlock()
	saved, err := copyImageFile(src, dst)
	if err != nil {
		a.appendLog("另存失败: " + err.Error())
		return
	}
	a.appendLog("已另存图片: " + saved)
	a.closeSavePrompt()
}

func (a *App) openRawResponseModal(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	content, err := os.ReadFile(path)
	text := ""
	readErr := ""
	if err != nil {
		readErr = err.Error()
	} else {
		text = string(content)
		const maxPreview = 200_000
		if len(text) > maxPreview {
			text = text[:maxPreview] + "\n\n... [截断,完整内容请查看文件]"
		}
	}
	a.mu.Lock()
	a.rawResponseModalPath = path
	a.rawResponseModalError = readErr
	a.rawResponseModalText = text
	a.rawResponseViewerInput.SetText(text)
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closeRawResponseModal() {
	a.mu.Lock()
	a.rawResponseModalPath = ""
	a.rawResponseModalError = ""
	a.rawResponseModalText = ""
	a.rawResponseViewerInput.SetText("")
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) readSnapshot() snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.snapshotReady {
		return a.snapshotCache
	}
	logs := a.logsSnapshotCache
	if a.logsSnapshotRev != a.logsRev {
		logs = append([]string(nil), a.logs...)
		a.logsSnapshotCache = logs
		a.logsSnapshotRev = a.logsRev
	}
	history := a.history
	batchResults := a.batchResultsSnapshotLocked(history)
	profiles := a.profiles
	promptHistory := a.promptHistory
	promptTemplates := a.promptTemplates
	presets := a.presets
	todayCount := a.todayHistoryCountLocked()
	lastProbeModels := append([]kernel.UpstreamModelDescriptor(nil), a.lastProbeModels...)
	snap := snapshot{
		Running:                   a.running,
		ProcessingImageTransform:  a.processingImageTransform,
		Status:                    a.status,
		Logs:                      logs,
		RenderBackend:             a.renderBackend,
		RenderFrameTime:           a.frameIntervalEMA,
		RenderFPS:                 a.frameFPS,
		RenderActive:              a.renderActive,
		TodayHistoryCount:         todayCount,
		History:                   history,
		BatchResults:              batchResults,
		BatchTotal:                a.lastRunBatchCount,
		Profiles:                  profiles,
		ActiveProfileID:           a.activeProfileID,
		SettingsSelectedProfileID: a.settingsSelectedProfileID,
		SelectedHistoryID:         a.selectedHistoryID,
		PromptHistory:             promptHistory,
		PromptTemplates:           promptTemplates,
		Presets:                   presets,
		OptimizingPrompt:          a.optimizingPrompt,
		TestingUpstream:           a.testingUpstream,
		SyncingCodexConfig:        a.syncingCodexConfig,
		LastProbeSummary:          a.lastProbeSummary,
		LastProbeModels:           lastProbeModels,
		ActivePromptGroup:         a.activePromptGroup,
		ActiveResultDetail:        a.activeResultDetail,
		HistoryTimelineOpen:       a.historyTimelineOpen,
		Fullscreen:                a.fullscreen,
		LastErrorMessage:          a.lastErrorMessage,
		LastRunAvailable:          a.lastRunValid,
		LastLowFPSSnapshotPath:    a.lastLowFPSDiagnosticsPath,
		RawResponseModalPath:      a.rawResponseModalPath,
		RawResponseModalText:      a.rawResponseModalText,
		RawResponseModalError:     a.rawResponseModalError,
		ResultGridOpen:            a.resultGridOpen,
		Compare:                   a.compare,
		CompareSplit:              a.compareSplitSlider.Value,
		Result:                    a.result,
		SavePromptVisible:         a.savePromptVisible,
		PromptImportVisible:       a.promptImportOpen,
		PromptImportLoading:       a.promptImportLoading,
		PromptImportToken:         a.promptImportToken,
		PromptImportPayload:       a.promptImportPayload,
		PromptImportResolvedSize:  a.promptImportResolvedSize,
		PromptImportRegisterOpen:  a.promptImportRegisterOpen,
		PromptImportRegisterBusy:  a.promptImportRegisterBusy,
		PromptImportRegisterNote:  a.promptImportRegisterNote,
	}
	a.snapshotCache = snap
	a.snapshotReady = true
	return snap
}

func (a *App) batchResultsSnapshotLocked(history []sharedCompat.HistoryItem) []sharedCompat.HistoryItem {
	key := strings.Join(a.batchResultIDs, "\x00")
	if a.batchResultsRev == a.historyRev && a.batchResultsKey == key {
		return a.batchResultsSnapshot
	}
	a.batchResultsSnapshot = historyItemsByIDs(history, a.batchResultIDs)
	a.batchResultsRev = a.historyRev
	a.batchResultsKey = key
	return a.batchResultsSnapshot
}

func (a *App) todayHistoryCountLocked() int {
	now := time.Now()
	day := now.Format("2006-01-02")
	if a.historyTodayRev == a.historyRev && a.historyTodayDay == day {
		return a.historyTodayCount
	}
	count := todayHistoryCount(a.history, now)
	a.historyTodayRev = a.historyRev
	a.historyTodayDay = day
	a.historyTodayCount = count
	return count
}

func (a *App) setHistoryLocked(items []sharedCompat.HistoryItem) {
	a.history = append([]sharedCompat.HistoryItem(nil), items...)
	a.historyRev++
	a.historyItemDisplayCache = historyItemDisplayCache{}
	a.historyButtons = map[string]*widget.Clickable{}
	a.historyActionButtons = map[string]*widget.Clickable{}
	a.expandedPromptGroups = map[string]bool{}
	a.pruneImageCacheLocked()
	go a.startHistoryThumbBackfill()
}

func (a *App) setProfilesLocked(items []sharedCompat.UpstreamProfile) {
	a.profiles = append([]sharedCompat.UpstreamProfile(nil), items...)
	a.profileButtons = map[string]*widget.Clickable{}
	a.settingsProfileButtons = map[string]*widget.Clickable{}
}

func (a *App) setPromptHistoryLocked(items []string) {
	a.promptHistory = append([]string(nil), items...)
	a.promptHistoryRev++
	a.promptButtons = map[string]*widget.Clickable{}
}

func (a *App) setPromptTemplatesLocked(items []sharedCompat.PromptTemplate) {
	a.promptTemplates = append([]sharedCompat.PromptTemplate(nil), items...)
	a.promptButtons = map[string]*widget.Clickable{}
}

func (a *App) setPresetsLocked(items []sharedCompat.Preset) {
	a.presets = append([]sharedCompat.Preset(nil), items...)
	a.promptButtons = map[string]*widget.Clickable{}
}

func (a *App) openGeneralSettingsModal() {
	a.mu.Lock()
	a.generalSettingsOpen = true
	a.generalRuntimePickerOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closeGeneralSettingsModal() {
	if err := a.persistGeneralSettings(); err != nil {
		a.appendLog("保存通用设置失败: " + err.Error())
	}
	a.mu.Lock()
	a.generalSettingsOpen = false
	a.generalRuntimePickerOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) persistGeneralSettings() error {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return err
	}
	state = sharedCompat.Normalize(state)
	state.Settings.ProxyMode = strings.TrimSpace(a.proxy)
	if state.Settings.ProxyMode == "" {
		state.Settings.ProxyMode = "system"
	}
	protectStreamPreview := a.protectStreamPreview
	state.Settings.ProtectStreamPreview = &protectStreamPreview
	autoRetryEnabled := a.autoRetryEnabled
	state.Settings.AutoRetryEnabled = &autoRetryEnabled
	autoRetryCount := normalizeAutoRetryCount(a.autoRetryCount)
	state.Settings.AutoRetryCount = &autoRetryCount
	completionSound := a.completionSound
	state.Settings.CompletionSound = &completionSound
	completionNotification := a.completionNotification
	state.Settings.CompletionNotification = &completionNotification
	state.Settings.CleanupPreviewCacheOnExit = a.cleanupPreviewCacheOnExit
	state.Settings.KernelRuntimeMode = normalizeKernelRuntimeMode(a.kernelRuntimeMode)
	state.Settings.FontScale = normalizeFontScale(a.fontScale)
	state.Settings.ReducedEffects = a.reducedEffects
	state.Settings.ProxyURL = strings.TrimSpace(a.proxyURLInput.Text())
	state.Settings.OutputDir = strings.TrimSpace(a.outputDirInput.Text())
	state = gioCompat.RememberTrustedOutputRoot(state, state.Settings.OutputDir)
	state.Settings.KeepLogs = a.keepLogs
	state.Settings.IgnoredReleaseTag = strings.TrimSpace(a.ignoredReleaseTag)
	state.Settings.UserIdentifier = strings.TrimSpace(a.userIdentifierInput.Text())
	state.UpdatedAt = time.Now().UnixMilli()
	if err := gioCompat.SaveState(state); err != nil {
		return err
	}
	return nil
}

func (a *App) dismissFailureState() {
	a.mu.Lock()
	a.lastErrorMessage = ""
	if a.status == "失败" {
		if a.result.HasItem {
			a.status = "已载入历史结果"
		} else {
			a.status = "准备就绪"
		}
	}
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) applyPartialPreview(partial client.PartialImage) {
	imageB64 := strings.TrimSpace(partial.ImageB64)
	if imageB64 == "" {
		return
	}
	img, err := decodeImageB64(imageB64)
	if err != nil {
		a.appendLog("解析流式预览失败: " + err.Error())
		return
	}
	preview := a.prepareCanvasDisplayImage(img)
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return
	}
	a.result = resultState{
		Image:         preview,
		RevisedPrompt: strings.TrimSpace(partial.RevisedPrompt),
		SourceEvent:   "partial",
		Rev:           a.result.Rev + 1,
	}
	a.compare = resultState{Rev: a.compare.Rev + 1}
	a.compareSplitSlider.Value = 0.5
	a.selectedHistoryID = ""
	a.imageOpRev = 0
	a.compareImageOpRev = 0
	a.mu.Unlock()
	a.invalidateSoon(33 * time.Millisecond)
}

func (a *App) isRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

func (a *App) invalidateNow() {
	a.mu.Lock()
	a.noteRenderActivityLocked(time.Now())
	a.snapshotReady = false
	a.mu.Unlock()
	if a.invalidate != nil {
		a.invalidate()
	}
}

func (a *App) invalidateSoon(delay time.Duration) {
	a.mu.Lock()
	a.noteRenderActivityLocked(time.Now())
	a.snapshotReady = false
	if a.invalidate == nil {
		a.mu.Unlock()
		return
	}
	if a.invalidateQueued {
		a.mu.Unlock()
		return
	}
	a.invalidateQueued = true
	a.mu.Unlock()

	time.AfterFunc(delay, func() {
		a.mu.Lock()
		a.invalidateQueued = false
		current := a.invalidate
		a.mu.Unlock()
		if current == nil {
			return
		}
		current()
	})
}
