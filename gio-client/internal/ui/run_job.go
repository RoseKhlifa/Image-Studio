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
	sharedCompat "image-studio/shared/compat"

	_ "github.com/gen2brain/avif"
	"github.com/yuanhua/image-gptcodex/pkg/client"
	_ "golang.org/x/image/webp"
)

func (a *App) startRun() {
	a.syncLoopSettingsFromInputs()
	cfg := a.currentConfig()
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.BaseURL) == "" {
		return
	}
	if strings.TrimSpace(cfg.Prompt) == "" {
		a.appendLog("请先填写提示词，再开始生成。")
		return
	}
	if a.batchMode && strings.TrimSpace(a.batchInputDirInput.Text()) == "" && len(a.sourcePaths()) == 0 {
		a.appendLog("请先为批处理选择输入目录或多张源图。")
		return
	}
	total := normalizeBatchCount(a.batchCount)
	if a.loopEnabled {
		total = normalizeLoopGenerationCount(a.loopTotalCount)
	}
	if a.batchMode {
		batchSources, err := a.batchSourcePathsForRun()
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
	requiredConcurrency := requestedRunConcurrency(total, a.loopEnabled, a.loopConcurrency)
	if limit := parseConcurrencyLimit(strings.TrimSpace(a.concurrencyLimitInput.Text())); limit > 0 && requiredConcurrency > limit {
		a.appendLog(runConcurrencyLimitError(cfg.APIMode, limit, requiredConcurrency, a.batchMode, a.loopEnabled))
		return
	}
	a.startRunWithConfig(cfg, total)
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

func requestedRunConcurrency(total int, loopEnabled bool, loopConcurrency int) int {
	total = normalizeBatchCount(total)
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
	a.mu.Unlock()
	if !ok {
		return
	}
	a.startRunWithConfig(cfg, total)
}

func (a *App) startRunWithConfig(cfg kernel.Config, total int) {
	if a.isRunning() {
		return
	}
	total = normalizeBatchCount(total)
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
	a.lastRunValid = true
	a.lastErrorMessage = ""
	a.status = fmt.Sprintf("正在提交 1/%d", total)
	a.activePromptGroup = historyPromptGroup{}
	a.batchResultIDs = nil
	a.resultGridOpen = total > 1
	a.appendLogLocked(fmt.Sprintf("开始任务 1/%d: %s", total, shortPrompt(cfg.Prompt)))
	a.mu.Unlock()
	a.invalidateNow()

	go func() {
		batchStarted := time.Now()
		batchSources := a.batchSourcePaths()
		var once sync.Once
		var firstErr atomic.Pointer[error]
		var jobsDone atomic.Int32
		concurrency := total
		if a.loopEnabled {
			concurrency = normalizeLoopGenerationConcurrency(a.loopConcurrency)
			if concurrency > total {
				concurrency = total
			}
		}
		if concurrency < 1 {
			concurrency = 1
		}
		effectivePartialImages, streamPreviewDisableReason := a.effectivePartialImagesForRun(cfg, concurrency)
		if streamPreviewDisableReason == streamPreviewDisableReasonDesktopConcurrency {
			a.appendLog(fmt.Sprintf("高并发(%d)已自动关闭流式预览，优先保证最终图完整。", concurrency))
		}
		jobSem := make(chan struct{}, concurrency)
		var wg sync.WaitGroup
		cancelAll := func() {
			once.Do(func() {
				cancel()
			})
		}
		for i := 0; i < total; i++ {
			if ctx.Err() != nil {
				break
			}
			jobSem <- struct{}{}
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				defer func() { <-jobSem }()
				if err := ctx.Err(); err != nil {
					return
				}
				jobCfg := cfg
				jobCfg.BatchIndex = i
				jobCfg.Prompt = augmentPromptWithStyle(cfg.Prompt, cfg.StyleTag)
				jobCfg.PartialImages = effectivePartialImages
				if a.batchMode && i < len(batchSources) {
					jobCfg.SourcePaths = []string{batchSources[i]}
					jobCfg.AutoRetryEnabled = a.batchRetryOnFail
					if strings.TrimSpace(a.batchAutoAspect) != "" {
						if width, height, err := imageDimensionsFromFile(batchSources[i]); err == nil && width > 0 && height > 0 {
							jobCfg.Size = buildBatchAutoAspectSize(width, height, a.batchAutoAspect)
						}
					}
				}
				if cfg.Seed != 0 {
					jobCfg.Seed = cfg.Seed + int64(i)
				}
				jobLabel := fmt.Sprintf("%d/%d", i+1, total)
				jobStarted := time.Now()
				res, err := a.runner.Run(ctx, jobCfg, kernel.Callbacks{
					Log: func(line string) {
						a.appendLog("[" + jobLabel + "] " + line)
					},
					Progress: func(stage string, elapsed int, bytes int64) {
						a.setStatus(fmt.Sprintf("%s · %s · %ds · %s", jobLabel, stage, elapsed, client.FormatBytes(bytes)))
					},
					Partial: func(partial client.PartialImage) {
						a.setStatus(fmt.Sprintf("%s · 收到流式预览 #%d", jobLabel, partial.PartialImageIndex))
						a.applyPartialPreview(partial)
					},
				})
				if err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					if a.batchMode {
						a.appendLog(fmt.Sprintf("[%s] 失败并跳过: %v", jobLabel, err))
						completed := int(jobsDone.Add(1))
						a.mu.Lock()
						if completed == total {
							a.running = false
							a.cancel = nil
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
				if err := gioCompat.SaveConfigAndHistory(jobCfg, res, elapsedSec); err != nil {
					a.appendLog("兼容历史保存失败: " + err.Error())
				}
				compatState, _, _ := gioCompat.LoadState()
				compatState = sharedCompat.Normalize(compatState)
				selectedItem, hasSelected := historyItemBySavedPath(compatState.History, res.SavedPath)
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
				if strings.TrimSpace(displayItem.PreviewPath) == "" {
					displayItem.PreviewPath = res.PreviewPath
				}
				if strings.TrimSpace(displayItem.ThumbPath) == "" {
					displayItem.ThumbPath = res.ThumbPath
				}
				nextResult := resultState{
					SavedPath:     res.SavedPath,
					RawPath:       res.RawPath,
					RevisedPrompt: res.RevisedPrompt,
					SourceEvent:   res.SourceEvent,
					Item:          displayItem,
					HasItem:       hasSelected || strings.TrimSpace(displayItem.SavedPath) != "",
				}
				nextResult.Image = a.loadCanvasImmediatePreviewForState(res.SavedPath, nextResult)
				completed := int(jobsDone.Add(1))
				a.mu.Lock()
				a.status = fmt.Sprintf("完成 %s · %.1fs", jobLabel, time.Since(batchStarted).Seconds())
				a.lastErrorMessage = ""
				nextResult.Rev = a.result.Rev + 1
				a.result = nextResult
				rev := a.result.Rev
				a.setHistoryLocked(compatState.History)
				a.setProfilesLocked(compatState.Profiles)
				a.activeProfileID = activeProfileID
				if hasSelected {
					a.selectedHistoryID = selectedItem.ID
				}
				if total > 1 && hasSelected {
					a.batchResultIDs = append(a.batchResultIDs, selectedItem.ID)
				}
				if !a.savePromptSuppressed && res.SavedPath != "" && total == 1 {
					a.savePromptVisible = true
					a.savePromptSourcePath = res.SavedPath
					a.savePromptPathInput.SetText(res.SavedPath)
				}
				a.appendLogLocked(fmt.Sprintf("生成完成 %s: %s", jobLabel, res.SavedPath))
				if a.batchMode && i < len(batchSources) {
					if saved, err := a.copyBatchResultIfNeeded(res.SavedPath, batchSources[i]); err == nil && strings.TrimSpace(saved) != "" {
						a.appendLogLocked(fmt.Sprintf("批处理已落盘 %s -> %s", filepath.Base(batchSources[i]), filepath.Base(saved)))
					}
				} else if a.loopEnabled && a.loopAutoSave && strings.TrimSpace(a.loopAutoSaveDirInput.Text()) != "" {
					if saved, err := copyImageFile(res.SavedPath, a.loopAutoSaveDirInput.Text()); err == nil && strings.TrimSpace(saved) != "" {
						a.appendLogLocked(fmt.Sprintf("循环结果已自动另存为 -> %s", filepath.Base(saved)))
					}
				}
				if completed == total {
					a.running = false
					a.cancel = nil
					a.status = fmt.Sprintf("完成 - %.1fs", time.Since(batchStarted).Seconds())
				}
				a.mu.Unlock()
				if completed == total {
					a.maybePlayCompletionSound(completed, total)
					a.maybeSendCompletionNotification(displayItem, completed, total)
				}
				a.invalidateNow()
				a.startAsyncCurrentResultImageLoad(res.SavedPath, displayItem, res.SourceEvent, rev)
			}(i)
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
	sourcePaths := a.sourcePaths()
	if client.Mode(a.mode) == client.ModeEdit && len(sourcePaths) == 0 {
		if current := strings.TrimSpace(a.readSnapshot().Result.SavedPath); current != "" {
			sourcePaths = []string{current}
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
		Prompt:               a.promptInput.Text(),
		Mode:                 client.Mode(a.mode),
		APIMode:              client.APIMode(a.api),
		RequestPolicy:        client.RequestPolicy(a.policy),
		ResponsesTransport:   client.ResponsesTransport(a.responsesTransport),
		ImagesNewAPICompat:   a.imagesNewAPICompat,
		Size:                 a.size,
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
		OutputDir:            a.outputDirInput.Text(),
		Seed:                 seed,
		NegativePrompt:       a.negativePromptInput.Text(),
		PartialImages:        partial,
		StyleTag:             a.styleTag,
		FallbackProfileID:    fallbackID,
		FallbackProfile:      fallback,
	}
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
	manual := a.sourcePaths()
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

func (a *App) copyBatchResultIfNeeded(savedPath string, sourcePath string) (string, error) {
	savedPath = strings.TrimSpace(savedPath)
	sourcePath = strings.TrimSpace(sourcePath)
	if savedPath == "" || sourcePath == "" {
		return "", nil
	}
	targetDir := strings.TrimSpace(a.batchOutputDirInput.Text())
	if targetDir == "" {
		targetDir = filepath.Dir(sourcePath)
	}
	if targetDir == "" {
		return "", nil
	}
	base := filepath.Base(sourcePath)
	targetName := "processed-" + base
	dst := filepath.Join(targetDir, targetName)
	if _, err := os.Stat(dst); err == nil {
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		for idx := 2; ; idx++ {
			candidate := filepath.Join(targetDir, fmt.Sprintf("processed-%s-%d%s", stem, idx, ext))
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				dst = candidate
				break
			}
		}
	}
	return copyImageFile(savedPath, dst)
}

func imageDimensionsFromFile(path string) (int, int, error) {
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
	if width <= 0 || height <= 0 {
		return "1024x1024"
	}
	longSide := 1536
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "256":
		longSide = 256
	case "512":
		longSide = 512
	case "2k":
		longSide = 2048
	case "4k":
		longSide = 3840
	}
	if width >= height {
		shortSide := max(64, int(float64(longSide)*float64(height)/float64(width)+0.5))
		return fmt.Sprintf("%dx%d", longSide, shortSide)
	}
	shortSide := max(64, int(float64(longSide)*float64(width)/float64(height)+0.5))
	return fmt.Sprintf("%dx%d", shortSide, longSide)
}
