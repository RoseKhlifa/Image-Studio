package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/kernel"
	"image-studio/gio-client/internal/windowing"
	sharedCompat "image-studio/shared/compat"

	_ "github.com/gen2brain/avif"
	"github.com/yuanhua/image-gptcodex/pkg/client"
	_ "golang.org/x/image/webp"
)

func (a *App) startRun() {
	a.syncLoopSettingsFromInputs()
	a.syncBatchSettingsFromInputs()
	workflowMode := normalizeExperienceMode(a.experienceMode) == experienceModeWorkflow
	graph := workflowGraphModel{}
	plan := workflowExecutionPlan{}
	workflowWorkspaceID := ""
	workflowOutputID := ""
	total := normalizeBatchCount(a.batchCount)
	if workflowMode {
		graph = a.workflowGraph(a.activeWorkspaceID)
		selectedID := a.selectedWorkflowNode(a.activeWorkspaceID)
		if selected, ok := graph.node(selectedID); ok {
			a.syncWorkflowNodeControls(a.activeWorkspaceID, selected, false)
			graph = a.workflowGraph(a.activeWorkspaceID)
		}
		preferredOutputID := workflowPreferredOutputNodeID(graph, selectedID)
		var err error
		plan, err = buildWorkflowExecutionPlan(graph, preferredOutputID, false)
		if err != nil {
			a.appendLog("工作流无效: " + err.Error())
			return
		}
		workflowWorkspaceID = a.activeWorkspaceID
		workflowOutputID = plan.Export.ID
	}
	cfg := a.currentConfig()
	if workflowMode {
		cfg, total = a.applyWorkflowExecutionPlan(cfg, plan)
		if (cfg.Mode == client.ModeEdit || a.batchMode) && len(plan.Sources) == 0 {
			a.appendLog("工作流无效: 生成节点 " + plan.Generate.Title + " 在图生图模式下需要参考图输入")
			return
		}
	}
	if !workflowMode && cfg.Mode == client.ModeEdit && len(cfg.SourcePaths) == 0 && len(cfg.SourceImageDataURLs) == 0 {
		snap := a.readSnapshot()
		if strings.TrimSpace(snap.Result.SavedPath) == "" && strings.TrimSpace(snap.Result.Item.ImageB64) != "" {
			if normalizeKernelRuntimeMode(a.kernelRuntimeMode) != "remote" {
				if _, err := a.ensureCurrentResultSavedPathIfNeeded(); err != nil {
					a.appendLog("当前结果未落盘，无法直接用作源图: " + err.Error())
					return
				}
			} else if len(a.currentEditFallbackSourceDataURLs()) == 0 {
				a.appendLog("当前结果未落盘，无法直接用作源图。")
				return
			}
			cfg = a.currentConfig()
		}
	}
	if cfg.Mode == client.ModeEdit && !a.batchMode && len(cfg.SourcePaths) == 0 && len(cfg.SourceImageDataURLs) == 0 {
		a.appendLog("图生图模式需要在当前分支的参考图节点中配置至少一张图像。")
		return
	}
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.BaseURL) == "" {
		return
	}
	if strings.TrimSpace(cfg.Prompt) == "" {
		a.appendLog("请先填写提示词，再开始生成。")
		return
	}
	if a.batchMode && strings.TrimSpace(a.batchInputDirInput.Text()) == "" && len(cfg.SourcePaths) == 0 {
		a.appendLog("请先为批处理选择输入目录或多张源图。")
		return
	}
	if a.loopEnabled && a.loopAutoSave && strings.TrimSpace(a.loopAutoSaveDirInput.Text()) == "" {
		a.appendLog("请先为循环出图配置自动另存为路径。")
		return
	}
	if a.loopEnabled {
		total = normalizeLoopGenerationCount(a.loopTotalCount)
	}
	if a.batchMode {
		batchSources, err := a.batchSourcePathsForRunWithManual(cfg.SourcePaths)
		if err != nil {
			a.appendLog("读取批处理目录失败: " + err.Error())
			return
		}
		if len(batchSources) == 0 {
			a.appendLog("批处理目录里没有可用图片。")
			return
		}
		total = len(batchSources)
	}
	if errMsg := validateKernelRuntimeForRun(a.kernelRuntimeMode, cfg); errMsg != "" {
		a.appendLog(errMsg)
		return
	}
	requiredConcurrency := requestedRunConcurrency(total, a.batchMode, a.batchConcurrency, a.loopEnabled, a.loopConcurrency)
	if limit := parseConcurrencyLimit(strings.TrimSpace(a.concurrencyLimitInput.Text())); limit > 0 && requiredConcurrency > limit {
		a.appendLog(runConcurrencyLimitError(cfg.APIMode, limit, requiredConcurrency, a.batchMode, a.loopEnabled))
		return
	}
	a.startRunWithConfig(cfg, total, workflowWorkspaceID, workflowOutputID)
}

func parseConcurrencyLimit(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func requestedRunConcurrency(total int, batchMode bool, batchConcurrency int, loopEnabled bool, loopConcurrency int) int {
	if total < 1 {
		total = 1
	}
	if batchMode {
		concurrency := normalizeBatchProcessConcurrency(batchConcurrency)
		if concurrency > total {
			concurrency = total
		}
		if concurrency < 1 {
			concurrency = 1
		}
		return concurrency
	}
	if loopEnabled {
		concurrency := normalizeLoopGenerationConcurrency(loopConcurrency)
		if concurrency > total {
			concurrency = total
		}
		if concurrency < 1 {
			concurrency = 1
		}
		return concurrency
	}
	if total < 1 {
		return 1
	}
	return total
}

func normalizeRunTotal(total int, batchMode bool, loopEnabled bool) int {
	if batchMode {
		if total < 1 {
			return 1
		}
		return total
	}
	if loopEnabled {
		return normalizeLoopGenerationCount(total)
	}
	return normalizeBatchCount(total)
}

func buildRunExecutionPlan(total int, batchMode bool, batchConcurrency int, loopEnabled bool, loopConcurrency int) (int, int) {
	total = normalizeRunTotal(total, batchMode, loopEnabled)
	concurrency := requestedRunConcurrency(total, batchMode, batchConcurrency, loopEnabled, loopConcurrency)
	return total, concurrency
}

func newRunPreviewSlotPool(concurrency int) chan int {
	concurrency = max(1, concurrency)
	slots := make(chan int, concurrency)
	for slotIndex := 0; slotIndex < concurrency; slotIndex++ {
		slots <- slotIndex
	}
	return slots
}

func validateKernelRuntimeForRun(kernelRuntimeMode string, cfg kernel.Config) string {
	if normalizeKernelRuntimeMode(kernelRuntimeMode) != "remote" {
		return ""
	}
	if cfg.ProxyMode != client.ProxyModeSystem {
		return "当前远程内核不能控制代理,请切回本地内核或使用 Android 原生运行"
	}
	if cfg.APIMode == client.APIModeResponses && cfg.ResponsesTransport == client.ResponsesTransportWebSocket {
		return "当前远程内核模式暂不支持 Responses WebSocket mode，请切回本地内核或关闭该开关。"
	}
	return ""
}

func runConcurrencyLimitError(apiMode client.APIMode, limit int, required int, batchMode bool, loopEnabled bool) string {
	apiLabel := "Responses API"
	if apiMode == client.APIModeImages {
		apiLabel = "Images API"
	}
	switch {
	case batchMode:
		return fmt.Sprintf("%s 并发限制 %d,当前还可提交 %d 个,批处理并发需要 %d 个。", apiLabel, limit, limit, required)
	case loopEnabled:
		return fmt.Sprintf("%s 并发限制 %d,当前还可提交 %d 个,循环模式并发需要 %d 个。", apiLabel, limit, limit, required)
	default:
		return fmt.Sprintf("%s 并发限制 %d,当前还可提交 %d 个,本次需要 %d 个。", apiLabel, limit, limit, required)
	}
}

func (a *App) retryLastRun() {
	a.mu.Lock()
	cfg := a.lastRunConfig
	total := a.lastRunBatchCount
	ok := a.lastRunValid
	workflowWorkspaceID := a.lastRunWorkflowWorkspace
	workflowOutputID := a.lastRunWorkflowOutput
	a.mu.Unlock()
	if !ok {
		return
	}
	if normalizeExperienceMode(a.experienceMode) == experienceModeWorkflow {
		graph := a.workflowGraph(a.activeWorkspaceID)
		if workflowWorkspaceID != a.activeWorkspaceID {
			a.appendLog("上次运行属于其他工作区，无法在当前工作区重试。")
			return
		}
		plan, err := buildWorkflowExecutionPlan(graph, workflowOutputID, cfg.Mode == client.ModeEdit || a.batchMode)
		if err != nil {
			a.appendLog("工作流无效: " + err.Error())
			return
		}
		if len(plan.Sources) == 0 {
			cfg.SourcePaths = nil
			cfg.SourceImageDataURLs = nil
		}
	}
	a.startRunWithConfig(cfg, total, workflowWorkspaceID, workflowOutputID)
}

func (a *App) startRunWithConfig(cfg kernel.Config, total int, workflowWorkspaceID string, workflowOutputID string) {
	if a.isRunning() {
		return
	}
	batchMode := a.batchMode
	loopEnabled := a.loopEnabled
	total, runConcurrency := buildRunExecutionPlan(total, batchMode, a.batchConcurrency, loopEnabled, a.loopConcurrency)
	batchSources := []string(nil)
	if batchMode {
		batchSources, _ = a.batchSourcePathsForRunWithManual(cfg.SourcePaths)
	}
	batchOutputDir := a.effectiveBatchOutputDir()
	batchOutputPrefix := a.effectiveBatchOutputPrefix()
	batchRetryOnFail := a.batchRetryOnFail
	batchAutoAspect := strings.TrimSpace(a.batchAutoAspect)
	batchAPI := a.api
	batchPolicy := a.policy
	batchImageModelID := a.imageModelInput.Text()
	batchCustomAspectRatios := append([]sharedCompat.CustomAspectRatio(nil), a.customAspectRatios...)
	loopAutoSave := a.loopAutoSave
	loopAutoSaveDir := strings.TrimSpace(a.loopAutoSaveDirInput.Text())
	kernelRuntimeMode := a.kernelRuntimeMode
	a.rememberPrompt(cfg.Prompt)
	if err := gioCompat.SaveConfig(cfg); err != nil {
		a.appendLog("兼容配置保存失败: " + err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.mu.Lock()
	a.running = true
	a.cancel = cancel
	a.lastRunConfig = cfg
	a.lastRunBatchCount = total
	a.lastRunConcurrency = runConcurrency
	a.lastRunValid = true
	a.lastRunWorkflowWorkspace = strings.TrimSpace(workflowWorkspaceID)
	a.lastRunWorkflowOutput = strings.TrimSpace(workflowOutputID)
	a.lastErrorMessage = ""
	a.status = fmt.Sprintf("正在提交 1/%d", total)
	a.activePromptGroup = historyPromptGroup{}
	a.batchResultIDs = nil
	a.batchPreviewItems = nil
	a.resultGridOpen = total > 1
	a.canvasTool = canvasToolPan
	a.resetCanvasMaskLocked()
	a.resetCanvasAnnotationsLocked()
	a.appendLogLocked(fmt.Sprintf("开始任务 1/%d: %s", total, shortPrompt(cfg.Prompt)))
	a.mu.Unlock()
	a.invalidateNow()
	if a.experienceMode == experienceModeWorkflow && a.desktopState.Preferences.AutoShowProgress {
		a.openDesktopWindow(windowing.RoleProgress, a.activeWorkspaceID)
	}

	go func() {
		batchStarted := time.Now()
		var once sync.Once
		var firstErr atomic.Pointer[error]
		var jobsDone atomic.Int32
		concurrency := runConcurrency
		if concurrency < 1 {
			concurrency = 1
		}
		effectivePartialImages, streamPreviewDisableReason := a.effectivePartialImagesForRun(cfg, concurrency)
		if streamPreviewDisableReason == streamPreviewDisableReasonDesktopConcurrency {
			a.appendLog(fmt.Sprintf("高并发(%d)已自动关闭流式预览，优先保证最终图完整。", concurrency))
		}
		previewSlots := newRunPreviewSlotPool(concurrency)
		var wg sync.WaitGroup
		cancelAll := func() {
			once.Do(func() {
				cancel()
			})
		}
	jobLoop:
		for i := 0; i < total; i++ {
			if ctx.Err() != nil {
				break
			}
			previewSlotIndex := 0
			select {
			case previewSlotIndex = <-previewSlots:
			case <-ctx.Done():
				break jobLoop
			}
			wg.Add(1)
			go func(i int, previewSlotIndex int) {
				defer wg.Done()
				defer func() { previewSlots <- previewSlotIndex }()
				if err := ctx.Err(); err != nil {
					return
				}
				jobCfg := cfg
				jobCfg.BatchIndex = i
				jobCfg.PreviewSlotIndex = previewSlotIndex
				jobCfg.Prompt = augmentPromptWithStyle(cfg.Prompt, cfg.StyleTag)
				jobCfg.PartialImages = effectivePartialImages
				if batchMode && i < len(batchSources) {
					jobCfg.SourcePaths = []string{batchSources[i]}
					jobCfg.AutoRetryEnabled = batchRetryOnFail
					if batchAutoAspect != "" {
						if width, height, err := imageDimensionsFromFile(batchSources[i]); err == nil && width > 0 && height > 0 {
							jobCfg.Size = buildReferenceResolutionSizeSelection(width, height, batchAutoAspect, batchAPI, batchPolicy, batchImageModelID, batchCustomAspectRatios)
						}
					}
				}
				if cfg.Seed != 0 {
					jobCfg.Seed = cfg.Seed + int64(i)
				}
				jobLabel := fmt.Sprintf("%d/%d", i+1, total)
				jobStarted := time.Now()
				lastProgressStage := ""
				res, err := a.runner.Run(ctx, jobCfg, kernel.Callbacks{
					Log: func(line string) {
						a.appendLog("[" + jobLabel + "] " + line)
					},
					Progress: func(stage string, elapsed int, bytes int64) {
						stage = strings.TrimSpace(stage)
						if stage != "" && stage != lastProgressStage {
							lastProgressStage = stage
							a.appendLog("[" + jobLabel + "] " + stage)
						}
						a.setStatus(fmt.Sprintf("%s · %s · %ds · %s", jobLabel, stage, elapsed, client.FormatBytes(bytes)))
					},
					Partial: func(partial client.PartialImage) {
						a.setStatus(fmt.Sprintf("%s · 收到流式预览 #%d", jobLabel, partial.PartialImageIndex))
						a.applyPartialPreview(i, jobCfg.PreviewSlotIndex, partial)
					},
				})
				previewOnlyRemote := normalizeKernelRuntimeMode(kernelRuntimeMode) == "remote"
				if previewOnlyRemote && strings.TrimSpace(res.SavedPath) == "" && strings.TrimSpace(res.ImageB64) != "" {
					res.SavedPath = registerVirtualImage(res.ImageB64, suggestedSaveNameForHistoryItem(sharedCompat.HistoryItem{
						Prompt:       jobCfg.Prompt,
						Mode:         string(jobCfg.Mode),
						OutputFormat: jobCfg.OutputFormat,
						CreatedAt:    time.Now().UnixMilli(),
					}), jobCfg.OutputFormat)
				}
				if previewOnlyRemote && strings.TrimSpace(res.RawText) != "" {
					res.RawPath = registerVirtualText(res.RawText, fmt.Sprintf("raw-response-%d-%d.txt", i+1, time.Now().UnixNano()))
				}
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					if batchMode {
						a.appendLog(fmt.Sprintf("[%s] 失败并跳过: %v", jobLabel, err))
						completed := int(jobsDone.Add(1))
						a.mu.Lock()
						a.removeBatchPreviewItemLocked(jobCfg.PreviewSlotIndex)
						if completed == total {
							a.running = false
							a.cancel = nil
							a.lastRunConcurrency = 0
							a.clearBatchPreviewItemsLocked()
							a.status = fmt.Sprintf("完成 - %.1fs", time.Since(batchStarted).Seconds())
						}
						a.mu.Unlock()
						a.invalidateNow()
						return
					}
					if firstErr.Load() == nil {
						errCopy := err
						firstErr.Store(&errCopy)
						a.finishWithError(err, res.RawPath)
					}
					cancelAll()
					return
				}
				elapsedSec := time.Since(jobStarted).Seconds()
				if err := gioCompat.SaveConfigAndHistoryWithPreviewMode(jobCfg, res, elapsedSec, previewOnlyRemote); err != nil {
					a.appendLog("兼容历史保存失败: " + err.Error())
				}
				compatState, _, _ := gioCompat.LoadState()
				compatState = sharedCompat.Normalize(compatState)
				selectedItem, hasSelected := historyItemByRawPath(compatState.History, res.RawPath)
				if !hasSelected {
					selectedItem, hasSelected = historyItemBySavedPath(compatState.History, res.SavedPath)
				}
				if !hasSelected {
					selectedItem, hasSelected = newestHistoryItem(compatState.History)
				}
				activeProfileID := ""
				if profile, ok := gioCompat.ActiveProfile(compatState); ok {
					activeProfileID = profile.ID
				}
				displayItem := selectedItem
				if !hasSelected {
					displayItem = sharedCompat.HistoryItem{}
				}
				if strings.TrimSpace(displayItem.SavedPath) == "" {
					displayItem.SavedPath = res.SavedPath
				}
				if strings.TrimSpace(displayItem.RawPath) == "" {
					displayItem.RawPath = res.RawPath
				}
				if strings.TrimSpace(displayItem.RevisedPrompt) == "" {
					displayItem.RevisedPrompt = res.RevisedPrompt
				}
				if !previewOnlyRemote && strings.TrimSpace(displayItem.PreviewPath) == "" {
					displayItem.PreviewPath = res.PreviewPath
				}
				if !previewOnlyRemote && strings.TrimSpace(displayItem.ThumbPath) == "" {
					displayItem.ThumbPath = res.ThumbPath
				}
				if strings.TrimSpace(displayItem.ImageB64) == "" {
					displayItem.ImageB64 = strings.TrimSpace(res.ImageB64)
				}
				resultSavedPath := res.SavedPath
				nextResult := resultState{
					SavedPath:     resultSavedPath,
					RawPath:       res.RawPath,
					RevisedPrompt: res.RevisedPrompt,
					SourceEvent:   res.SourceEvent,
					Item:          displayItem,
					HasItem:       hasSelected || strings.TrimSpace(displayItem.SavedPath) != "" || strings.TrimSpace(displayItem.ImageB64) != "",
				}
				nextResult.Image = a.loadCanvasImmediatePreviewForState(resultSavedPath, nextResult)
				completed := int(jobsDone.Add(1))
				openSavePromptAfterUnlock := false
				openBatchSavePromptAfterUnlock := false
				var batchSaveItems []sharedCompat.HistoryItem
				a.mu.Lock()
				a.status = fmt.Sprintf("完成 %s · %.1fs", jobLabel, time.Since(batchStarted).Seconds())
				a.lastErrorMessage = ""
				a.removeBatchPreviewItemLocked(jobCfg.PreviewSlotIndex)
				nextResult.Rev = a.result.Rev + 1
				a.result = nextResult
				rev := a.result.Rev
				a.setHistoryLocked(compatState.History)
				a.setProfilesLocked(compatState.Profiles)
				a.activeProfileID = activeProfileID
				if hasSelected {
					a.selectedHistoryID = selectedItem.ID
				}
				nextBatchResultIDs := append([]string(nil), a.batchResultIDs...)
				if total > 1 && hasSelected {
					nextBatchResultIDs = append(nextBatchResultIDs, selectedItem.ID)
					a.batchResultIDs = nextBatchResultIDs
				}
				if !a.savePromptSuppressed && total == 1 {
					if previewOnlyRemote {
						openSavePromptAfterUnlock = true
					} else if res.SavedPath != "" {
						a.savePromptVisible = true
						a.savePromptSourcePath = res.SavedPath
						a.savePromptSourceImageB64 = ""
						a.savePromptSuggestedName = filepath.Base(res.SavedPath)
						a.savePromptPathInput.SetText(res.SavedPath)
					}
				}
				if !a.savePromptSuppressed && completed == total && total > 1 && !batchMode && !loopEnabled {
					batchSaveItems = historyItemsByIDs(compatState.History, nextBatchResultIDs)
					openBatchSavePromptAfterUnlock = len(batchSaveItems) > 0
				}
				a.appendLogLocked(fmt.Sprintf("生成完成 %s: %s", jobLabel, res.SavedPath))
				if batchMode && i < len(batchSources) {
					saved := ""
					var saveErr error
					if previewOnlyRemote {
						targetPath := batchResultTargetPath(batchSources[i], batchOutputDir, batchOutputPrefix)
						if targetPath != "" {
							saved, saveErr = saveImageB64ToPath(res.ImageB64, filepath.Base(targetPath), targetPath)
						}
					} else {
						saved, saveErr = a.copyBatchResultIfNeeded(res.SavedPath, batchSources[i], batchOutputDir, batchOutputPrefix)
					}
					if saveErr == nil && strings.TrimSpace(saved) != "" {
						a.appendLogLocked(fmt.Sprintf("批处理已落盘 %s -> %s", filepath.Base(batchSources[i]), filepath.Base(saved)))
					}
				} else if loopEnabled && loopAutoSave && loopAutoSaveDir != "" {
					saved := ""
					var saveErr error
					if previewOnlyRemote {
						saved, saveErr = saveImageB64ToPath(res.ImageB64, suggestedSaveNameForHistoryItem(displayItem), loopAutoSaveDir)
					} else {
						saved, saveErr = copyImageFile(res.SavedPath, loopAutoSaveDir)
					}
					if saveErr == nil && strings.TrimSpace(saved) != "" {
						a.appendLogLocked(fmt.Sprintf("循环结果已自动另存为 -> %s", filepath.Base(saved)))
					}
				}
				if completed == total {
					a.running = false
					a.cancel = nil
					a.lastRunConcurrency = 0
					a.clearBatchPreviewItemsLocked()
					a.status = fmt.Sprintf("完成 - %.1fs", time.Since(batchStarted).Seconds())
				}
				a.mu.Unlock()
				if openSavePromptAfterUnlock {
					a.openSavePromptForItem(displayItem)
				}
				if openBatchSavePromptAfterUnlock {
					a.openBatchSavePrompt(batchSaveItems)
				}
				if completed == total {
					a.maybePlayCompletionSound(completed, total)
					a.maybeSendCompletionNotification(displayItem, completed, total)
				}
				a.invalidateNow()
				a.startAsyncCurrentResultImageLoad(resultSavedPath, displayItem, res.SourceEvent, rev)
			}(i, previewSlotIndex)
		}
		wg.Wait()
		if ctx.Err() != nil && firstErr.Load() == nil {
			a.finishCancelled()
		}
	}()
}

func (a *App) currentConfig() kernel.Config {
	seed, _ := strconv.ParseInt(strings.TrimSpace(a.seedInput.Text()), 10, 64)
	partial, _ := strconv.Atoi(strings.TrimSpace(a.partialImagesInput.Text()))
	outputCompression, _ := strconv.Atoi(strings.TrimSpace(a.outputCompressionInput.Text()))
	sourcePaths, sourceImageDataURLs := a.currentEditSourcesForConfig()
	parentID := a.currentEditParentID()
	prompt := a.currentCanvasAugmentedPrompt(a.promptInput.Text())
	if client.Mode(a.mode) != client.ModeEdit {
		sourcePaths = a.sourcePaths()
		sourceImageDataURLs = nil
		parentID = ""
	}
	maskB64 := ""
	if client.Mode(a.mode) == client.ModeEdit {
		maskB64 = a.currentCanvasMaskB64()
	}
	size := normalizeSizeSelection(a.size, a.api, a.policy, a.imageModelInput.Text(), a.customAspectRatios)
	if client.Mode(a.mode) == client.ModeEdit && !a.batchMode && strings.TrimSpace(a.editAutoAspectResolution) != "" {
		width, height, ok := a.currentEditReferenceDimensions()
		if ok {
			size = buildReferenceResolutionSizeSelection(width, height, a.editAutoAspectResolution, a.api, a.policy, a.imageModelInput.Text(), a.customAspectRatios)
		} else {
			size = buildReferenceResolutionSizeSelection(0, 0, a.editAutoAspectResolution, a.api, a.policy, a.imageModelInput.Text(), a.customAspectRatios)
		}
	}
	var fallback *kernel.FallbackProfileConfig
	fallbackID := strings.TrimSpace(a.fallbackProfileID)
	if fallbackID != "" {
		state, _, err := gioCompat.LoadState()
		if err == nil {
			state = sharedCompat.Normalize(state)
			fallback = resolveFallbackProfileConfig(state, fallbackID, gioCompat.ReadAPIKey)
		}
	}
	return kernel.Config{
		APIKey:               a.apiKeyInput.Text(),
		BaseURL:              a.baseURLInput.Text(),
		TextModelID:          a.textModelInput.Text(),
		ImageModelID:         a.imageModelInput.Text(),
		Prompt:               prompt,
		Mode:                 client.Mode(a.mode),
		APIMode:              client.APIMode(a.api),
		RequestPolicy:        client.RequestPolicy(a.policy),
		ResponsesTransport:   client.ResponsesTransport(a.responsesTransport),
		ImagesNewAPICompat:   a.imagesNewAPICompat,
		Size:                 size,
		Quality:              a.quality,
		OutputFormat:         a.format,
		Background:           a.background,
		OutputCompression:    outputCompression,
		InputFidelity:        a.inputFidelity,
		ImageStyle:           a.imageStyle,
		Moderation:           a.moderation,
		UserIdentifier:       a.userIdentifierInput.Text(),
		ProxyMode:            a.proxy,
		ProxyURL:             a.proxyURLInput.Text(),
		ReasoningEffort:      a.reasoningEffort,
		ProtectStreamPreview: a.protectStreamPreview,
		AutoRetryEnabled:     a.autoRetryEnabled,
		AutoRetryCount:       a.autoRetryCount,
		CompletionSound:      a.completionSound,
		SourcePaths:          sourcePaths,
		SourceImageDataURLs:  sourceImageDataURLs,
		ParentID:             parentID,
		OutputDir:            a.outputDirInput.Text(),
		Seed:                 seed,
		NegativePrompt:       a.negativePromptInput.Text(),
		MaskB64:              maskB64,
		PartialImages:        partial,
		StyleTag:             a.styleTag,
		FallbackProfileID:    fallbackID,
		FallbackProfile:      fallback,
		PreviewOnlyResult:    normalizeKernelRuntimeMode(a.kernelRuntimeMode) == "remote",
	}
}

func (a *App) effectiveEditAutoAspectResolution() string {
	if strings.TrimSpace(a.editAutoAspectResolution) == "" {
		return ""
	}
	return normalizeBatchAutoAspectResolution(a.editAutoAspectResolution, a.api, a.policy, a.imageModelInput.Text())
}

func (a *App) currentEditAutoAspectResolvedSize() string {
	resolution := a.effectiveEditAutoAspectResolution()
	if resolution == "" {
		return ""
	}
	width, height, ok := a.currentEditReferenceDimensions()
	if ok {
		return buildReferenceResolutionSizeSelection(width, height, resolution, a.api, a.policy, a.imageModelInput.Text(), a.customAspectRatios)
	}
	return buildReferenceResolutionSizeSelection(0, 0, resolution, a.api, a.policy, a.imageModelInput.Text(), a.customAspectRatios)
}

func (a *App) currentEditReferenceDimensions() (int, int, bool) {
	if client.Mode(a.mode) != client.ModeEdit {
		return 0, 0, false
	}
	for _, path := range a.sourcePaths() {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		width, height, err := imageDimensionsFromFile(path)
		if err == nil && width > 0 && height > 0 {
			return width, height, true
		}
		break
	}
	if current := strings.TrimSpace(a.result.SavedPath); current != "" {
		width, height, err := imageDimensionsFromFile(current)
		if err == nil && width > 0 && height > 0 {
			return width, height, true
		}
	}
	if imageB64 := strings.TrimSpace(a.result.Item.ImageB64); imageB64 != "" {
		if img, err := decodeImageB64(imageB64); err == nil && img != nil {
			bounds := img.Bounds()
			if bounds.Dx() > 0 && bounds.Dy() > 0 {
				return bounds.Dx(), bounds.Dy(), true
			}
		}
	}
	if a.result.Image != nil {
		bounds := a.result.Image.Bounds()
		if bounds.Dx() > 0 && bounds.Dy() > 0 {
			return bounds.Dx(), bounds.Dy(), true
		}
	}
	return 0, 0, false
}

func (a *App) currentEditSourcesForConfig() ([]string, []string) {
	sourcePaths := a.sourcePaths()
	if len(sourcePaths) > 0 {
		filePaths := make([]string, 0, len(sourcePaths))
		dataURLs := make([]string, 0, len(sourcePaths))
		for _, path := range sourcePaths {
			path = strings.TrimSpace(path)
			if path == "" {
				continue
			}
			if dataURL, ok := virtualImageDataURL(path); ok {
				dataURLs = append(dataURLs, dataURL)
				continue
			}
			filePaths = append(filePaths, path)
		}
		if len(filePaths) > 0 || len(dataURLs) > 0 {
			return filePaths, dataURLs
		}
	}
	if current := strings.TrimSpace(a.readSnapshot().Result.SavedPath); current != "" {
		if dataURL, ok := virtualImageDataURL(current); ok {
			return nil, []string{dataURL}
		}
		return []string{current}, nil
	}
	if normalizeKernelRuntimeMode(a.kernelRuntimeMode) == "remote" {
		return nil, a.currentEditFallbackSourceDataURLs()
	}
	return nil, nil
}

func (a *App) currentEditParentID() string {
	if client.Mode(a.mode) != client.ModeEdit {
		return ""
	}
	for _, path := range a.sourcePaths() {
		path = strings.TrimSpace(path)
		if path != "" {
			return path
		}
	}
	return strings.TrimSpace(a.readSnapshot().Result.SavedPath)
}

func (a *App) currentEditFallbackSourceDataURLs() []string {
	snap := a.readSnapshot()
	item := snap.Result.Item
	imageB64 := strings.TrimSpace(item.ImageB64)
	if imageB64 == "" {
		return nil
	}
	format := "." + client.FileExtForFormat(strings.TrimSpace(item.OutputFormat))
	mime := client.SupportedImageMime[format]
	if strings.TrimSpace(mime) == "" {
		mime = "image/png"
	}
	return []string{"data:" + mime + ";base64," + imageB64}
}

func resolveFallbackProfileConfig(state sharedCompat.State, fallbackProfileID string, readAPIKey func(string) (string, error)) *kernel.FallbackProfileConfig {
	fallbackProfileID = strings.TrimSpace(fallbackProfileID)
	if fallbackProfileID == "" {
		return nil
	}
	profile, ok := profileByID(state.Profiles, fallbackProfileID)
	if !ok {
		return nil
	}
	apiKey, _ := readAPIKey(profile.ID)
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(profile.BaseURL) == "" {
		return nil
	}
	return &kernel.FallbackProfileConfig{
		APIKey:             strings.TrimSpace(apiKey),
		BaseURL:            strings.TrimSpace(profile.BaseURL),
		TextModelID:        strings.TrimSpace(profile.TextModelID),
		ImageModelID:       strings.TrimSpace(profile.ImageModelID),
		APIMode:            client.APIMode(normalizeProfileAPIMode(profile.APIMode)),
		ResponsesTransport: client.ResponsesTransport(normalizeProfileResponsesTransport(profile.ResponsesTransport)),
		RequestPolicy:      client.RequestPolicy(normalizeProfilePolicy(profile.RequestPolicy)),
		ImagesNewAPICompat: profile.ImagesNewAPICompat,
		ReasoningEffort:    normalizeReasoningEffort(profile.ReasoningEffort),
	}
}

func (a *App) batchSourcePaths() []string {
	paths, _ := a.batchSourcePathsForRun()
	return paths
}

func (a *App) batchSourcePathsForRun() ([]string, error) {
	return a.batchSourcePathsForRunWithManual(a.sourcePaths())
}

func (a *App) batchSourcePathsForRunWithManual(manual []string) ([]string, error) {
	manual = append([]string(nil), manual...)
	if len(manual) > 0 {
		return manual, nil
	}
	dir := strings.TrimSpace(a.batchInputDirInput.Text())
	if dir == "" {
		return manual, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".webp":
			out = append(out, filepath.Join(dir, entry.Name()))
		}
	}
	return out, nil
}

func batchResultTargetPath(sourcePath string, batchOutputDir string, batchOutputPrefix string) string {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return ""
	}
	targetDir := strings.TrimSpace(batchOutputDir)
	if targetDir == "" {
		targetDir = filepath.Dir(sourcePath)
	}
	if targetDir == "" {
		return ""
	}
	base := filepath.Base(sourcePath)
	targetName := normalizeBatchOutputPrefix(batchOutputPrefix) + base
	dst := filepath.Join(targetDir, targetName)
	if _, err := os.Stat(dst); err == nil {
		ext := filepath.Ext(targetName)
		stem := strings.TrimSuffix(targetName, ext)
		for idx := 2; ; idx++ {
			candidate := filepath.Join(targetDir, fmt.Sprintf("%s-%d%s", stem, idx, ext))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				dst = candidate
				break
			}
		}
	}
	return dst
}

func (a *App) copyBatchResultIfNeeded(savedPath string, sourcePath string, batchOutputDir string, batchOutputPrefix string) (string, error) {
	savedPath = strings.TrimSpace(savedPath)
	sourcePath = strings.TrimSpace(sourcePath)
	if savedPath == "" || sourcePath == "" {
		return "", nil
	}
	dst := batchResultTargetPath(sourcePath, batchOutputDir, batchOutputPrefix)
	if dst == "" {
		return "", nil
	}
	return copyImageFile(savedPath, dst)
}

func imageDimensionsFromFile(path string) (int, int, error) {
	if imageB64, ok := readVirtualImageB64(path); ok {
		img, err := decodeImageB64(imageB64)
		if err != nil {
			return 0, 0, err
		}
		bounds := img.Bounds()
		return bounds.Dx(), bounds.Dy(), nil
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()
	cfg, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}

func buildBatchAutoAspectSize(width int, height int, resolution string) string {
	return buildReferenceResolutionSizeSelection(width, height, resolution, string(client.APIModeResponses), string(client.RequestPolicyOpenAI), client.ImageModel, nil)
}
