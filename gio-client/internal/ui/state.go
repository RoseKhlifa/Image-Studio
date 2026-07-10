package ui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
		a.lastRunConcurrency = 0
		a.status = "已取消"
		a.clearBatchPreviewItemsLocked()
		if !a.canOpenResultGridLocked() {
			a.resultGridOpen = false
		}
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
	a.lastRunConcurrency = 0
	a.clearBatchPreviewItemsLocked()
	if !a.canOpenResultGridLocked() {
		a.resultGridOpen = false
	}
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
	a.lastRunConcurrency = 0
	a.clearBatchPreviewItemsLocked()
	if !a.canOpenResultGridLocked() {
		a.resultGridOpen = false
	}
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
	a.resetSavePromptStateLocked()
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) openHistoryTimeline() {
	a.mu.Lock()
	a.historyTimelineOpen = true
	a.historyTimelineModePickerOpen = false
	a.historyTimelineDatePickerOpen = false
	a.expandedPromptGroups = map[string]bool{}
	a.historyActionMenuItem = sharedCompat.HistoryItem{}
	a.historyActionMenuContext = ""
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closeHistoryTimeline() {
	a.mu.Lock()
	a.historyTimelineOpen = false
	a.historyTimelineModePickerOpen = false
	a.historyTimelineDatePickerOpen = false
	a.expandedPromptGroups = map[string]bool{}
	a.historyActionMenuItem = sharedCompat.HistoryItem{}
	a.historyActionMenuContext = ""
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) batchGridCountLocked() int {
	total := batchGridTotalSlots(
		a.batchResultsSnapshotLocked(a.history),
		a.batchPreviewItemsSnapshotLocked(),
		0,
	)
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
	item := a.result.Item
	if strings.TrimSpace(item.SavedPath) == "" {
		item.SavedPath = strings.TrimSpace(a.result.SavedPath)
	}
	a.mu.Unlock()
	a.openSavePromptForItem(item)
}

func (a *App) openSavePromptForPath(path string) {
	src := strings.TrimSpace(path)
	if src == "" {
		return
	}
	if isVirtualImagePath(src) {
		imageB64, ok := readVirtualImageB64(src)
		if !ok || strings.TrimSpace(imageB64) == "" {
			return
		}
		a.mu.Lock()
		a.savePromptVisible = true
		a.savePromptSourcePath = ""
		a.savePromptSourceImageB64 = imageB64
		a.savePromptSuggestedName = virtualImageDisplayName(src)
		a.savePromptPathInput.SetText(filepath.Join(strings.TrimSpace(a.outputDirInput.Text()), virtualImageDisplayName(src)))
		a.mu.Unlock()
		a.invalidateNow()
		return
	}
	a.mu.Lock()
	a.savePromptVisible = true
	a.savePromptSourcePath = src
	a.savePromptSourceImageB64 = ""
	a.savePromptSuggestedName = filepath.Base(src)
	a.savePromptPathInput.SetText(src)
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) openSavePromptForItem(item sharedCompat.HistoryItem) {
	suggestedName := suggestedSaveNameForHistoryItem(item)
	if src := strings.TrimSpace(item.SavedPath); src != "" {
		if !isVirtualImagePath(src) {
			a.openSavePromptForPath(src)
			return
		}
		if name := strings.TrimSpace(virtualImageDisplayName(src)); name != "" {
			suggestedName = name
		}
	}
	imageB64 := strings.TrimSpace(item.ImageB64)
	if imageB64 == "" && strings.TrimSpace(item.SavedPath) != "" {
		if b64, ok := readVirtualImageB64(item.SavedPath); ok {
			imageB64 = b64
		}
	}
	if imageB64 == "" {
		return
	}
	target := defaultSavePromptTargetForHistoryItem(item, a.outputDirInput.Text())
	a.mu.Lock()
	a.savePromptVisible = true
	a.savePromptSourcePath = ""
	a.savePromptSourceImageB64 = imageB64
	a.savePromptSuggestedName = suggestedName
	a.savePromptPathInput.SetText(target)
	a.mu.Unlock()
	a.invalidateNow()
}

func filterSavePromptBatchItems(items []sharedCompat.HistoryItem) []sharedCompat.HistoryItem {
	if len(items) == 0 {
		return nil
	}
	filtered := make([]sharedCompat.HistoryItem, 0, len(items))
	for _, item := range items {
		if canSaveHistoryItem(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (a *App) openBatchSavePrompt(items []sharedCompat.HistoryItem) {
	items = filterSavePromptBatchItems(items)
	if len(items) == 0 {
		return
	}
	selected := make(map[string]bool, len(items))
	for _, item := range items {
		if id := strings.TrimSpace(item.ID); id != "" {
			selected[id] = true
		}
	}
	targetDir := strings.TrimSpace(a.outputDirInput.Text())
	if targetDir == "" {
		targetDir = kernel.DefaultOutputDir()
	}
	a.mu.Lock()
	a.savePromptVisible = true
	a.savePromptSourcePath = ""
	a.savePromptSourceImageB64 = ""
	a.savePromptSuggestedName = ""
	a.savePromptBatchItems = append([]sharedCompat.HistoryItem(nil), items...)
	a.savePromptBatchSelection = selected
	a.savePromptPathInput.SetText(targetDir)
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) savePromptBatchSelectedItemsLocked() []sharedCompat.HistoryItem {
	if len(a.savePromptBatchItems) == 0 {
		return nil
	}
	selected := make([]sharedCompat.HistoryItem, 0, len(a.savePromptBatchItems))
	for _, item := range a.savePromptBatchItems {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if a.savePromptBatchSelection[id] {
			selected = append(selected, item)
		}
	}
	return selected
}

func (a *App) savePromptBatchSelectedCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.savePromptBatchSelectedItemsLocked())
}

func (a *App) savePromptBatchSelected(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.savePromptBatchSelection[id]
}

func (a *App) toggleSavePromptBatchSelection(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	a.mu.Lock()
	if a.savePromptBatchSelection == nil {
		a.savePromptBatchSelection = map[string]bool{}
	}
	a.savePromptBatchSelection[id] = !a.savePromptBatchSelection[id]
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) setAllSavePromptBatchSelections(value bool) {
	a.mu.Lock()
	if len(a.savePromptBatchItems) == 0 {
		a.mu.Unlock()
		return
	}
	if a.savePromptBatchSelection == nil {
		a.savePromptBatchSelection = map[string]bool{}
	}
	for _, item := range a.savePromptBatchItems {
		if id := strings.TrimSpace(item.ID); id != "" {
			a.savePromptBatchSelection[id] = value
		}
	}
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
	imageB64 := a.savePromptSourceImageB64
	suggestedName := a.savePromptSuggestedName
	dst := a.savePromptPathInput.Text()
	batchMode := len(a.savePromptBatchItems) > 0
	batchItems := a.savePromptBatchSelectedItemsLocked()
	a.mu.Unlock()
	if batchMode {
		if len(batchItems) == 0 {
			a.appendLog("批量另存失败: 请先勾选要另存的图片")
			return
		}
		saved, err := saveHistoryItemsToDirectory(batchItems, dst)
		if err != nil {
			a.appendLog("批量另存失败: " + err.Error())
			return
		}
		dir := strings.TrimSpace(strings.Trim(dst, `"'`))
		label := dir
		if base := filepath.Base(dir); base != "" && base != "." && base != string(os.PathSeparator) {
			label = base
		}
		a.appendLog(fmt.Sprintf("已另存 %d 张到 %s", len(saved), label))
		a.closeSavePrompt()
		return
	}
	var (
		saved string
		err   error
	)
	if strings.TrimSpace(src) != "" {
		saved, err = copyImageFile(src, dst)
	} else {
		saved, err = saveImageB64ToPath(imageB64, suggestedName, dst)
	}
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
	text := ""
	readErr := ""
	if virtualText, ok := readVirtualText(path); ok {
		text = virtualText
	} else {
		content, err := os.ReadFile(path)
		if err != nil {
			readErr = err.Error()
		} else {
			text = string(content)
		}
	}
	const maxPreview = 200_000
	if len(text) > maxPreview {
		text = text[:maxPreview] + "\n\n... [截断,完整内容请查看文件]"
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

func (a *App) openAdvancedPanel() {
	a.mu.Lock()
	a.advancedOpen = true
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closeAdvancedPanel() {
	if err := a.persistAdvancedPanelPrefs(); err != nil {
		a.appendLog("保存高级参数面板偏好失败: " + err.Error())
	}
	a.mu.Lock()
	a.advancedOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) toggleAdvancedGroup(group string) {
	a.mu.Lock()
	switch strings.TrimSpace(group) {
	case "core":
		a.advancedCoreGroupOpen = !a.advancedCoreGroupOpen
	case "output":
		a.advancedOutputGroupOpen = !a.advancedOutputGroupOpen
	case "strategy":
		a.advancedStrategyGroupOpen = !a.advancedStrategyGroupOpen
	case "stream":
		a.advancedStreamGroupOpen = !a.advancedStreamGroupOpen
	default:
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	if err := a.persistAdvancedPanelPrefs(); err != nil {
		a.appendLog("保存高级参数面板偏好失败: " + err.Error())
	}
	a.invalidateNow()
}

func (a *App) openHistoryActionMenu(item sharedCompat.HistoryItem, context string) {
	if strings.TrimSpace(item.ID) == "" && strings.TrimSpace(item.SavedPath) == "" {
		return
	}
	a.mu.Lock()
	anchor := a.lastGlobalPressPos
	if anchor == (image.Point{}) {
		anchor = a.lastGlobalPointer
	}
	if anchor == (image.Point{}) {
		anchor = image.Pt(240, 180)
	}
	a.historyActionMenuItem = item
	a.historyActionMenuContext = strings.TrimSpace(context)
	a.historyActionMenuPos = anchor
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closeHistoryActionMenu() {
	a.mu.Lock()
	a.historyActionMenuItem = sharedCompat.HistoryItem{}
	a.historyActionMenuContext = ""
	a.historyActionMenuPos = image.Point{}
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) beginNativeFileDrag(path string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("当前平台不支持原生文件拖出")
	}
	path = strings.TrimSpace(path)
	if path == "" || isVirtualImagePath(path) {
		return fmt.Errorf("当前结果没有可拖出的本地文件")
	}
	a.mu.Lock()
	view := a.darwinAppKitView
	window := a.window
	a.mu.Unlock()
	if view == 0 {
		return fmt.Errorf("当前窗口还没有可用的原生视图句柄")
	}
	if window == nil {
		return fmt.Errorf("当前窗口未初始化")
	}
	var dragErr error
	window.Run(func() {
		dragErr = beginNativeFileDragDarwin(view, path)
	})
	return dragErr
}

func (a *App) prepareHistoryItemForNativeDrag(item sharedCompat.HistoryItem) (sharedCompat.HistoryItem, string, error) {
	path := strings.TrimSpace(item.SavedPath)
	if path != "" && !isVirtualImagePath(path) {
		return item, path, nil
	}
	if !canSaveHistoryItem(item) {
		return item, "", fmt.Errorf("当前结果没有可拖出的本地文件")
	}
	next, err := a.materializeHistoryItemForLocalPath(item)
	if err != nil {
		return item, "", err
	}
	return next, strings.TrimSpace(next.SavedPath), nil
}

func (a *App) dragOutHistoryItem(item sharedCompat.HistoryItem) (sharedCompat.HistoryItem, error) {
	next, path, err := a.prepareHistoryItemForNativeDrag(item)
	if err != nil {
		return item, err
	}
	if err := a.beginNativeFileDrag(path); err != nil {
		return next, err
	}
	return next, nil
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
	batchPreviewItems := a.batchPreviewItemsSnapshotLocked()
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
		BatchPreviewItems:         batchPreviewItems,
		BatchTotal:                a.lastRunBatchCount,
		BatchLiveSlotCount:        a.lastRunConcurrency,
		SavePromptBatchItems:      append([]sharedCompat.HistoryItem(nil), a.savePromptBatchItems...),
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
		HistoryActionMenuItem:     a.historyActionMenuItem,
		HistoryActionMenuContext:  a.historyActionMenuContext,
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

func (a *App) batchPreviewItemsSnapshotLocked() []sharedCompat.HistoryItem {
	return orderedBatchPreviewItems(a.batchPreviewItems)
}

func (a *App) removeBatchPreviewItemLocked(index int) {
	if len(a.batchPreviewItems) == 0 {
		return
	}
	delete(a.batchPreviewItems, index)
	if len(a.batchPreviewItems) == 0 {
		a.batchPreviewItems = nil
	}
}

func (a *App) clearBatchPreviewItemsLocked() {
	a.batchPreviewItems = nil
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
	historySnapshot := append([]sharedCompat.HistoryItem(nil), a.history...)
	a.historyRev++
	a.historyItemDisplayCache = historyItemDisplayCache{}
	a.historyButtons = map[string]*widget.Clickable{}
	a.historyActionButtons = map[string]*widget.Clickable{}
	a.expandedPromptGroups = map[string]bool{}
	a.pruneImageCacheLocked()
	go a.startHistoryThumbBackfillItems(historySnapshot, true)
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
	return gioCompat.UpdateState(func(state *sharedCompat.State) error {
		*state = sharedCompat.Normalize(*state)
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
		*state = gioCompat.RememberTrustedOutputRoot(*state, state.Settings.OutputDir)
		state.Settings.KeepLogs = a.keepLogs
		state.Settings.IgnoredReleaseTag = strings.TrimSpace(a.ignoredReleaseTag)
		state.Settings.UserIdentifier = strings.TrimSpace(a.userIdentifierInput.Text())
		state.UpdatedAt = time.Now().UnixMilli()
		return nil
	})
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

func (a *App) applyPartialPreview(batchIndex int, previewSlotIndex int, partial client.PartialImage) {
	imageB64 := strings.TrimSpace(partial.ImageB64)
	if imageB64 == "" {
		return
	}
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return
	}
	cfg := a.lastRunConfig
	if batchIndex < 0 {
		batchIndex = partial.PartialImageIndex
	}
	if batchIndex < 0 {
		batchIndex = 0
	}
	if previewSlotIndex < 0 {
		previewSlotIndex = batchIndex
	}
	itemID := "preview:current"
	if a.lastRunBatchCount > 1 {
		itemID = "preview:slot:" + strconv.Itoa(previewSlotIndex)
	} else if currentID := strings.TrimSpace(a.result.Item.ID); currentID != "" {
		itemID = currentID
	}
	previewItem := sharedCompat.HistoryItem{
		ID:               itemID,
		Prompt:           strings.TrimSpace(cfg.Prompt),
		RevisedPrompt:    strings.TrimSpace(partial.RevisedPrompt),
		Mode:             string(cfg.Mode),
		Size:             strings.TrimSpace(cfg.Size),
		Quality:          strings.TrimSpace(cfg.Quality),
		OutputFormat:     strings.TrimSpace(cfg.OutputFormat),
		ParentID:         strings.TrimSpace(cfg.ParentID),
		CreatedAt:        time.Now().UnixMilli(),
		BatchIndex:       batchIndex,
		PreviewSlotIndex: previewSlotIndex,
		ImageB64:         imageB64,
		PreviewOnly:      true,
	}
	if a.lastRunBatchCount > 1 {
		if a.batchPreviewItems == nil {
			a.batchPreviewItems = map[int]sharedCompat.HistoryItem{}
		}
		a.batchPreviewItems[previewSlotIndex] = previewItem
		a.resultGridOpen = true
		a.compare = resultState{Rev: a.compare.Rev + 1}
		a.compareSplitSlider.Value = 0.5
		a.selectedHistoryID = ""
		a.imageOpRev = 0
		a.compareImageOpRev = 0
		a.mu.Unlock()
		a.invalidateSoon(33 * time.Millisecond)
		return
	}
	a.mu.Unlock()
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
		Item:          previewItem,
		HasItem:       true,
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
