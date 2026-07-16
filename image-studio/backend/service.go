// Package backend exposes the GUI-facing bindings for the Wails app.
// All gptcodex-specific logic lives in github.com/yuanhua/image-gptcodex/pkg/client;
// this package only wires it into Wails (context, events, file dialogs).
//
// File layout:
//
//	service.go   — Service struct, lifecycle, generation orchestration (Generate / Edit / Cancel)
//	types.go     — JSON-bound structs shared with the TS frontend
//	dialogs.go   — file picker / save / open URL / read image / import-export history
//	imports.go   — drag-drop / paste import + filename sanitisation
//	imageops.go  — rotate / flip / crop on disk via Go image stdlib
//	paths.go     — output / import dir resolution + filename helpers
//	open.go      — cross-platform "open in OS" shell-out
package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yuanhua/image-gptcodex/pkg/client"
)

// Service is the Wails-bound struct. Methods on it are exposed to the frontend
// via runtime/window/bindings.
type Service struct {
	ctx context.Context

	mu                        sync.Mutex
	jobs                      map[string]*job
	runningByAPIMode          map[string]int
	outputDir                 string // 用户自定义输出目录;空时回退到 defaultOutputDir()
	keepLogs                  bool
	cleanupPreviewCacheOnExit bool
	apiKeys                   apiKeyStore

	trustedOutputRoots map[string]struct{}
	mediaAssets        map[string]mediaAsset

	promptImportListenerReady       bool
	pendingPromptImportTokens       []string
	pendingPromptImportInvalidCount int
}

type job struct {
	cancel  context.CancelFunc
	done    chan struct{}
	apiMode string
}

// NewService constructs a fresh Service ready to be passed to wails.Run Bind.
func NewService() *Service {
	return &Service{
		jobs:               map[string]*job{},
		runningByAPIMode:   map[string]int{},
		apiKeys:            keyringAPIKeyStore{},
		trustedOutputRoots: map[string]struct{}{},
		mediaAssets:        map[string]mediaAsset{},
	}
}

// Startup is wired into wails.Options OnStartup; persists the runtime context.
func (s *Service) Startup(ctx context.Context) {
	s.ctx = ctx
	s.loadCompatibilitySettings()
	s.HandlePromptImportArgs(os.Args[1:])
	if strings.TrimSpace(os.Getenv(appUpdateProbePathEnv)) != "" || commandLineArgValue(os.Args[1:], appUpdateProbePathArg) != "" {
		go s.captureAppUpdateProbe()
	}
}

func (s *Service) captureAppUpdateProbe() {
	appVersion, err := currentDesktopAppVersion()
	if err != nil || strings.TrimSpace(appVersion) == "" {
		appVersion = defaultAppVersion
	}
	result := AppUpdateProbeResult{
		AppVersion:          appVersion,
		CurrentVersion:      appVersion,
		UpdateInfoAvailable: false,
		HasUpdate:           false,
		ShouldShowUpdate:    false,
		AppUpdateModalOpen:  false,
	}
	updateInfo, err := s.CheckForAppUpdate()
	if err == nil {
		result.CurrentVersion = updateInfo.CurrentVersion
		result.LatestVersion = updateInfo.LatestVersion
		result.ReleaseTag = updateInfo.ReleaseTag
		result.ReleaseURL = updateInfo.ReleaseURL
		result.UpdateInfoAvailable = true
		result.HasUpdate = updateInfo.HasUpdate
		result.ShouldShowUpdate = updateInfo.HasUpdate
		result.AppUpdateModalOpen = updateInfo.HasUpdate
	}
	_ = s.WriteAppUpdateProbe(result)
}

// resolvedOutputDir 返回当前生效的输出目录:用户自定义优先,否则默认。
// 不存在则尝试创建。
func (s *Service) resolvedOutputDir() (string, error) {
	s.mu.Lock()
	custom := s.outputDir
	s.mu.Unlock()
	if custom != "" {
		if err := os.MkdirAll(custom, secureDirMode); err != nil {
			return "", fmt.Errorf("无法创建输出目录 %s: %w", custom, err)
		}
		s.addTrustedOutputRoot(custom)
		return custom, nil
	}
	root, err := defaultOutputDir()
	if err == nil {
		s.addTrustedOutputRoot(root)
	}
	return root, err
}

// SetOutputDir 由前端调用以应用用户选择的输出目录。空串表示恢复默认。
// 路径会被 MkdirAll 兜底创建;创建失败则不接受。
func (s *Service) SetOutputDir(path string) error {
	if strings.TrimSpace(path) == "" {
		s.mu.Lock()
		s.outputDir = ""
		s.mu.Unlock()
		return nil
	}
	clean, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("路径无效:%w", err)
	}
	if err := os.MkdirAll(clean, secureDirMode); err != nil {
		return fmt.Errorf("无法创建输出目录 %s: %w", clean, err)
	}
	s.mu.Lock()
	s.outputDir = clean
	s.mu.Unlock()
	s.addTrustedOutputRoot(clean)
	return nil
}

// ChooseOutputDir 弹出系统目录选择对话框,选中后立刻应用并返回新路径。
// 用户取消时返回空串(不报错)。
func (s *Service) ChooseOutputDir() (string, error) {
	if s.ctx == nil {
		return "", errors.New("服务未启动")
	}
	chosen, err := runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择生成图片的保存目录",
	})
	if err != nil {
		return "", err
	}
	if chosen == "" {
		return "", nil // 用户取消
	}
	if err := s.SetOutputDir(chosen); err != nil {
		return "", err
	}
	return chosen, nil
}

func (s *Service) ChooseDirectory(title string) (string, error) {
	if s.ctx == nil {
		return "", errors.New("服务未启动")
	}
	chosen, err := runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: strings.TrimSpace(title),
	})
	if err != nil {
		return "", err
	}
	return chosen, nil
}

func (s *Service) BuildBatchOutputPath(sourcePath, outputDir, prefix string) (string, error) {
	cleanSource := strings.TrimSpace(sourcePath)
	if cleanSource == "" {
		return "", errors.New("源文件不能为空")
	}
	targetRoot := strings.TrimSpace(outputDir)
	if targetRoot == "" {
		targetRoot = filepath.Dir(cleanSource)
	}
	root, err := ensureTargetDirectory(targetRoot)
	if err != nil {
		return "", err
	}
	return uniquePrefixedTargetPath(root, filepath.Base(cleanSource), prefix)
}

// --- Generation entry points -----------------------------------------------

// Generate starts a text-to-image job and returns its ID immediately. Progress
// and final result arrive as Wails events.
func (s *Service) Generate(opts GenerateOptions) (JobStarted, error) {
	opts.Mode = "generate"
	return s.startJob(opts)
}

// Edit starts an image-to-image job. opts.ImagePaths must list one or more
// existing local files (the frontend writes imports/generated PNGs to disk
// so we never push raw base64 across the JSON bridge for large files).
func (s *Service) Edit(opts GenerateOptions) (JobStarted, error) {
	opts.Mode = "edit"
	if len(opts.collectPaths()) == 0 {
		return JobStarted{}, errors.New("edit 模式必须提供至少一张源图片")
	}
	return s.startJob(opts)
}

// OptimizePrompt uses the configured LLM to rewrite the current prompt into a
// cleaner image prompt. If edit source images are provided, they are included
// as visual context. The original prompt is not mutated by the backend.
func (s *Service) OptimizePrompt(opts PromptOptimizeOptions) (string, error) {
	if s.ctx == nil {
		return "", errors.New("服务未启动")
	}
	if strings.TrimSpace(opts.APIKey) == "" {
		return "", errors.New("API Key 不能为空")
	}
	operation := strings.TrimSpace(opts.Mode)
	if operation != "describe" && strings.TrimSpace(opts.Prompt) == "" {
		return "", errors.New("提示词不能为空")
	}
	if operation == "describe" && len(opts.collectPaths()) == 0 {
		return "", errors.New("图片反推必须提供画布图片")
	}
	baseURL, err := client.ValidateBaseURLWithSecurity(opts.BaseURL, opts.AllowInsecureConnection)
	if err != nil {
		return "", err
	}
	refPaths, cleanup, err := prepareUploadSourcePaths(opts.collectPaths())
	if err != nil {
		return "", err
	}
	defer cleanup()
	modelID := strings.TrimSpace(opts.TextModelID)
	if modelID == "" {
		modelID = client.TextModel
	}
	proxyConfig, err := client.NormalizeProxyConfig(opts.ProxyMode, opts.ProxyURL)
	if err != nil {
		return "", err
	}
	return optimizePromptWithLLM(s.ctx, baseURL, opts.APIKey, modelID, opts.Mode, opts.Prompt, refPaths, proxyConfig, opts.AllowInsecureConnection)
}

// Cancel terminates a running job. Safe to call with unknown IDs.
func (s *Service) Cancel(jobID string) error {
	s.mu.Lock()
	j, ok := s.jobs[jobID]
	s.mu.Unlock()
	if !ok {
		return nil
	}
	j.cancel()
	return nil
}

// collectPaths merges legacy ImagePath into ImagePaths and drops blanks.
func (o GenerateOptions) collectPaths() []string {
	paths := make([]string, 0, len(o.ImagePaths)+1)
	for _, p := range o.ImagePaths {
		if strings.TrimSpace(p) != "" {
			paths = append(paths, p)
		}
	}
	if strings.TrimSpace(o.ImagePath) != "" {
		paths = append(paths, o.ImagePath)
	}
	return paths
}

// --- Internal job lifecycle ------------------------------------------------

func (s *Service) startJob(opts GenerateOptions) (JobStarted, error) {
	if strings.TrimSpace(opts.APIKey) == "" {
		return JobStarted{}, errors.New("API Key 不能为空")
	}
	if strings.TrimSpace(opts.Prompt) == "" {
		return JobStarted{}, errors.New("提示词/修改要求不能为空")
	}
	apiMode := normaliseAPIMode(opts.APIMode)
	limit := normaliseConcurrencyLimit(opts.ConcurrencyLimit)
	if s.ctx == nil {
		return JobStarted{}, errors.New("服务未启动")
	}

	s.mu.Lock()
	jobID := strings.TrimSpace(opts.RequestedJobID)
	if jobID == "" {
		var err error
		jobID, err = newJobID()
		if err != nil {
			s.mu.Unlock()
			return JobStarted{}, err
		}
	}
	if _, exists := s.jobs[jobID]; exists {
		s.mu.Unlock()
		return JobStarted{}, fmt.Errorf("job id 已存在,请稍后重试")
	}
	if !s.canStartJobLocked(apiMode, limit) {
		s.mu.Unlock()
		return JobStarted{}, fmt.Errorf("%s 已达到并发限制 %d,请等待当前任务完成后再提交", apiModeLabel(apiMode), limit)
	}
	ctx, cancel := context.WithCancel(s.ctx)
	done := make(chan struct{})
	s.jobs[jobID] = &job{cancel: cancel, done: done, apiMode: apiMode}
	s.runningByAPIMode[apiMode]++
	s.mu.Unlock()

	go s.runJob(ctx, jobID, opts, done)

	return JobStarted{JobID: jobID}, nil
}

func (s *Service) canStartJobLocked(apiMode string, limit int) bool {
	return limit <= 0 || s.runningByAPIMode[apiMode] < limit
}

func (s *Service) runJob(ctx context.Context, jobID string, opts GenerateOptions, done chan struct{}) {
	defer close(done)
	defer func() {
		s.mu.Lock()
		if j, ok := s.jobs[jobID]; ok {
			if s.runningByAPIMode[j.apiMode] > 0 {
				s.runningByAPIMode[j.apiMode]--
			}
			delete(s.jobs, jobID)
		}
		s.mu.Unlock()
	}()

	mode := client.ModeGenerate
	if opts.Mode == "edit" {
		mode = client.ModeEdit
	}

	apiMode := client.APIMode(opts.APIMode)
	if apiMode == "" {
		apiMode = client.APIModeResponses
	}
	fallbackAPIMode := client.APIModeResponses
	if opts.FallbackProfile != nil && strings.TrimSpace(opts.FallbackProfile.APIMode) != "" {
		fallbackAPIMode = client.APIMode(strings.TrimSpace(opts.FallbackProfile.APIMode))
		if fallbackAPIMode == "" {
			fallbackAPIMode = client.APIModeResponses
		}
	}

	clientOpts := client.Options{
		APIKey:                  opts.APIKey,
		Prompt:                  opts.Prompt,
		Mode:                    mode,
		Size:                    opts.Size,
		Quality:                 opts.Quality,
		OutputFormat:            opts.OutputFormat,
		MaskB64:                 opts.MaskB64,
		Seed:                    opts.Seed,
		NegativePrompt:          opts.NegativePrompt,
		Background:              opts.Background,
		OutputCompression:       opts.OutputCompression,
		InputFidelity:           opts.InputFidelity,
		ImageStyle:              opts.ImageStyle,
		Moderation:              opts.Moderation,
		UserIdentifier:          opts.UserIdentifier,
		BaseURL:                 opts.BaseURL,
		TextModelID:             opts.TextModelID,
		ImageModelID:            opts.ImageModelID,
		ReasoningEffort:         opts.ReasoningEffort,
		Proxy:                   client.ProxyConfig{Mode: opts.ProxyMode, URL: opts.ProxyURL},
		APIMode:                 apiMode,
		ResponsesTransport:      client.ResponsesTransport(strings.TrimSpace(opts.ResponsesTransport)),
		RequestPolicy:           client.RequestPolicy(strings.TrimSpace(opts.RequestPolicy)),
		ImagesNewAPICompat:      opts.ImagesNewAPICompat,
		AllowInsecureConnection: opts.AllowInsecureConnection,
		NoPromptRevision:        opts.NoPromptRevision,
		DisablePreview:          opts.DisablePreview,
		AutoRetryEnabled:        &opts.AutoRetryEnabled,
		AutoRetryCount:          opts.AutoRetryCount,
		PartialImages:           client.DefaultPartialImages,
	}
	if opts.PartialImages > 0 {
		clientOpts.PartialImages = opts.PartialImages
	}
	var fallbackClientOpts *client.Options
	if opts.AutoRetryEnabled &&
		opts.FallbackProfile != nil &&
		strings.TrimSpace(opts.FallbackProfile.APIKey) != "" &&
		strings.TrimSpace(opts.FallbackProfile.BaseURL) != "" {
		fallbackClientOpts = &client.Options{
			APIKey:                  strings.TrimSpace(opts.FallbackProfile.APIKey),
			Prompt:                  opts.Prompt,
			Mode:                    mode,
			Size:                    opts.Size,
			Quality:                 opts.Quality,
			OutputFormat:            opts.OutputFormat,
			MaskB64:                 opts.MaskB64,
			Seed:                    opts.Seed,
			NegativePrompt:          opts.NegativePrompt,
			Background:              opts.Background,
			OutputCompression:       opts.OutputCompression,
			InputFidelity:           opts.InputFidelity,
			ImageStyle:              opts.ImageStyle,
			Moderation:              opts.Moderation,
			UserIdentifier:          opts.UserIdentifier,
			BaseURL:                 strings.TrimSpace(opts.FallbackProfile.BaseURL),
			TextModelID:             strings.TrimSpace(opts.FallbackProfile.TextModelID),
			ImageModelID:            strings.TrimSpace(opts.FallbackProfile.ImageModelID),
			ReasoningEffort:         strings.TrimSpace(opts.FallbackProfile.ReasoningEffort),
			Proxy:                   client.ProxyConfig{Mode: opts.ProxyMode, URL: opts.ProxyURL},
			APIMode:                 fallbackAPIMode,
			ResponsesTransport:      client.ResponsesTransport(strings.TrimSpace(opts.FallbackProfile.ResponsesTransport)),
			RequestPolicy:           client.RequestPolicy(strings.TrimSpace(opts.FallbackProfile.RequestPolicy)),
			ImagesNewAPICompat:      opts.FallbackProfile.ImagesNewAPICompat,
			AllowInsecureConnection: opts.FallbackProfile.AllowInsecureConnection,
			NoPromptRevision:        opts.NoPromptRevision,
			DisablePreview:          opts.DisablePreview,
			AutoRetryEnabled:        &opts.AutoRetryEnabled,
			AutoRetryCount:          opts.AutoRetryCount,
			PartialImages:           clientOpts.PartialImages,
		}
	}
	if mode == client.ModeEdit {
		paths, cleanup, prepErr := prepareUploadSourcePaths(opts.collectPaths())
		if prepErr != nil {
			s.emitError(jobID, prepErr)
			return
		}
		defer cleanup()
		clientOpts.ImagePaths = paths
		if fallbackClientOpts != nil {
			fallbackClientOpts.ImagePaths = paths
		}
		// Responses API 仍需 data URL(走 input_image 形态);
		// Images API 直接 multipart 上传文件,跳过 base64 编码节省往返开销。
		if apiMode == client.APIModeResponses || (fallbackClientOpts != nil && fallbackClientOpts.APIMode == client.APIModeResponses) {
			urls := make([]string, 0, len(paths))
			for _, p := range paths {
				dataURL, err := client.ImageFileToDataURL(p)
				if err != nil {
					s.emitError(jobID, fmt.Errorf("加载源图片 %s 失败:%w", filepath.Base(p), err))
					return
				}
				urls = append(urls, dataURL)
			}
			clientOpts.ImageDataURLs = urls
			if fallbackClientOpts != nil && fallbackClientOpts.APIMode == client.APIModeResponses {
				fallbackClientOpts.ImageDataURLs = urls
			}
		}
	}

	transport, err := client.PickTransportWithProxyAndSecurity(clientOpts.Proxy, clientOpts.AllowInsecureConnection)
	if err != nil {
		s.emitError(jobID, err)
		return
	}

	rootDir, err := s.resolvedOutputDir()
	if err != nil {
		s.emitError(jobID, err)
		return
	}
	// 拆 PNG 和 raw response 到两个子目录,避免单目录文件混杂。
	imagesDir := imagesSubdir(rootDir)
	thumbsDir := thumbsSubdir(rootDir)
	previewsDir := previewsSubdir(rootDir)
	logDir := logSubdir(rootDir)
	if err := os.MkdirAll(imagesDir, secureDirMode); err != nil {
		s.emitError(jobID, err)
		return
	}
	if err := os.MkdirAll(thumbsDir, secureDirMode); err != nil {
		s.emitError(jobID, err)
		return
	}
	if err := os.MkdirAll(previewsDir, secureDirMode); err != nil {
		s.emitError(jobID, err)
		return
	}
	if err := os.MkdirAll(logDir, secureDirMode); err != nil {
		s.emitError(jobID, err)
		return
	}

	// ★ 文件名时间戳精度只到秒,9 并发 batch 同一秒触发 → 9 个 savedPath 完全
	// 一样,os.WriteFile 互相覆盖,前 8 张图被最后一个 job 写的覆盖掉,前端拿
	// HistoryItem.savedPath 去磁盘读永远是同一张图。塞 6 字符 jobID 后缀让 PNG
	// 和 sse-response/images-response 日志文件都唯一。
	timestamp := time.Now().Format("20060102-150405")
	if len(jobID) >= 6 {
		timestamp = timestamp + "-" + jobID[:6]
	}
	logFn := func(msg string) {
		runtime.EventsEmit(s.ctx, "log:"+jobID, msg)
	}
	progressFn := func(stage string, elapsed int, bytes int64) {
		runtime.EventsEmit(s.ctx, "progress:"+jobID, ProgressPayload{
			Stage: stage, Elapsed: elapsed, Bytes: bytes,
		})
	}
	previewFn := func(partial client.PartialImage) {
		payload := PreviewPayload{
			RevisedPrompt:     partial.RevisedPrompt,
			PartialImageIndex: partial.PartialImageIndex,
			Mode:              string(mode),
			Prompt:            opts.Prompt,
		}
		if strings.TrimSpace(partial.ImageB64) == "" {
			return
		}
		previewName := fmt.Sprintf("preview-%s-%03d-%d.avif", timestamp, partial.PartialImageIndex, time.Now().UnixNano())
		previewPath := filepath.Join(previewsDir, previewName)
		previewW, previewH, previewErr := createAVIFThumbnailFromBase64(partial.ImageB64, previewPath, mediaPreviewMaxEdge)
		if previewErr != nil {
			logFn(fmt.Sprintf("生成中间预览 AVIF 失败:%v", previewErr))
			return
		}
		asset, mediaErr := s.registerPreviewMedia(previewPath, previewW, previewH)
		if mediaErr != nil {
			logFn(fmt.Sprintf("登记中间预览失败:%v", mediaErr))
			return
		}
		payload.ImageID = asset.ID
		payload.PreviewURL = asset.PreviewURL
		payload.PreviewWidth = asset.PreviewWidth
		payload.PreviewHeight = asset.PreviewHeight
		runtime.EventsEmit(s.ctx, "preview:"+jobID, PreviewPayload{
			ImageID:           payload.ImageID,
			PreviewURL:        payload.PreviewURL,
			PreviewWidth:      payload.PreviewWidth,
			PreviewHeight:     payload.PreviewHeight,
			RevisedPrompt:     payload.RevisedPrompt,
			PartialImageIndex: payload.PartialImageIndex,
			Mode:              payload.Mode,
			Prompt:            payload.Prompt,
		})
	}

	// raw response(SSE 文本 / Images API JSON)落到 log 子目录;PNG 落到 images 子目录。
	result, rawPath, err := client.RequestAndExtractWithRetriesAndPartial(
		ctx, transport, clientOpts, logDir, timestamp, logFn, progressFn, previewFn,
	)
	if err != nil && fallbackClientOpts != nil && shouldRouteFallbackAttempt(err, rawPath) {
		logFn("主上游自动重试失败，切换到备用上游再试一次...")
		fallbackTimestamp := timestamp + "-fallback"
		fallbackTransport, transportErr := client.PickTransportWithProxyAndSecurity(fallbackClientOpts.Proxy, fallbackClientOpts.AllowInsecureConnection)
		if transportErr != nil {
			s.emitError(jobID, transportErr)
			return
		}
		result, rawPath, err = client.RequestAndExtractWithRetriesAndPartial(
			ctx, fallbackTransport, *fallbackClientOpts, logDir, fallbackTimestamp, logFn, progressFn, previewFn,
		)
	}
	if err != nil {
		// 即使失败也把 rawPath 透给前端,「查看日志」按钮直接打开它。
		s.emitErrorWithRaw(jobID, err, rawPath)
		return
	}

	imageName := buildImageName(mode, opts.Prompt, timestamp, opts.OutputFormat)
	savedPath := filepath.Join(imagesDir, imageName)
	absSaved, werr := writeBase64PNG(result.ImageB64, savedPath)
	if werr != nil {
		s.emitErrorWithRaw(jobID, fmt.Errorf("保存结果图片失败:%w", werr), rawPath)
		return
	}
	savedPath = absSaved
	thumbName := strings.TrimSuffix(filepath.Base(imageName), filepath.Ext(imageName)) + ".avif"
	thumbPath := filepath.Join(thumbsDir, thumbName)
	thumbW, thumbH, thumbErr := createAVIFThumbnail(savedPath, thumbPath, mediaThumbMaxEdge)
	if thumbErr != nil {
		s.emitErrorWithRaw(jobID, fmt.Errorf("生成 AVIF 缩略图失败:%w", thumbErr), rawPath)
		return
	}
	asset, mediaErr := s.registerGeneratedMedia(savedPath, thumbPath, thumbW, thumbH)
	if mediaErr != nil {
		s.emitErrorWithRaw(jobID, fmt.Errorf("登记本地图片失败:%w", mediaErr), rawPath)
		return
	}
	absRaw, _ := filepath.Abs(rawPath)

	runtime.EventsEmit(s.ctx, "result:"+jobID, ResultPayload{
		RevisedPrompt: result.RevisedPrompt,
		SourceEvent:   result.SourceEvent,
		ImageID:       asset.ID,
		SavedPath:     savedPath,
		ThumbPath:     asset.ThumbPath,
		PreviewURL:    asset.PreviewURL,
		FullURL:       asset.FullURL,
		PreviewWidth:  asset.PreviewWidth,
		PreviewHeight: asset.PreviewHeight,
		RawPath:       absRaw,
		Mode:          string(mode),
		Prompt:        opts.Prompt,
	})
}

func (s *Service) emitError(jobID string, err error) {
	runtime.EventsEmit(s.ctx, "error:"+jobID, ErrorPayload{Message: err.Error()})
}

// emitErrorWithRaw 跟 emitError 一样,但额外带上原始响应日志的绝对路径,
// 前端「查看日志」按钮用它一键打开。请求都没发出去的早期失败走 emitError 即可。
func (s *Service) emitErrorWithRaw(jobID string, err error, rawPath string) {
	abs := rawPath
	if rawPath != "" {
		if a, e := filepath.Abs(rawPath); e == nil {
			abs = a
		}
	}
	runtime.EventsEmit(s.ctx, "error:"+jobID, ErrorPayload{
		Message: err.Error(),
		RawPath: abs,
	})
}

func normaliseAPIMode(mode string) string {
	switch strings.TrimSpace(mode) {
	case string(client.APIModeImages):
		return string(client.APIModeImages)
	default:
		return string(client.APIModeResponses)
	}
}

func normaliseConcurrencyLimit(limit int) int {
	if limit < 0 {
		return 0
	}
	return limit
}

func apiModeLabel(mode string) string {
	if mode == string(client.APIModeImages) {
		return "Images API"
	}
	return "Responses API"
}

func shouldRouteFallbackAttempt(err error, rawPath string) bool {
	if err == nil {
		return false
	}
	if rawPath != "" {
		if rawBytes, readErr := os.ReadFile(rawPath); readErr == nil && client.IsRetryable(string(rawBytes)) {
			return true
		}
	}
	lower := strings.ToLower(err.Error())
	for _, marker := range []string{
		"connection reset",
		"eof",
		"timeout",
		"deadline exceeded",
		"i/o timeout",
		"tls handshake",
		"no such host",
		"upstream connect error",
		"gateway",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func newJobID() (string, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
