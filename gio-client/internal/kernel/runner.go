package kernel

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	sharedCompat "image-studio/shared/compat"

	_ "github.com/gen2brain/avif"
	"github.com/yuanhua/image-gptcodex/pkg/client"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type Config struct {
	APIKey               string
	BaseURL              string
	TextModelID          string
	ImageModelID         string
	Prompt               string
	Mode                 client.Mode
	APIMode              client.APIMode
	RequestPolicy        client.RequestPolicy
	ResponsesTransport   client.ResponsesTransport
	ImagesNewAPICompat   bool
	Size                 string
	Quality              string
	OutputFormat         string
	Background           string
	OutputCompression    int
	InputFidelity        string
	ImageStyle           string
	Moderation           string
	UserIdentifier       string
	ProxyMode            string
	ProxyURL             string
	ReasoningEffort      string
	ProtectStreamPreview bool
	AutoRetryEnabled     bool
	AutoRetryCount       int
	CompletionSound      sharedCompat.CompletionSoundSettings
	SourcePaths          []string
	OutputDir            string
	Seed                 int64
	NegativePrompt       string
	PartialImages        int
	StyleTag             string
	BatchIndex           int
	FallbackProfileID    string
	FallbackProfile      *FallbackProfileConfig
}

type FallbackProfileConfig struct {
	APIKey             string
	BaseURL            string
	TextModelID        string
	ImageModelID       string
	APIMode            client.APIMode
	ResponsesTransport client.ResponsesTransport
	RequestPolicy      client.RequestPolicy
	ImagesNewAPICompat bool
	ReasoningEffort    string
}

type Callbacks struct {
	Log      func(string)
	Progress func(stage string, elapsed int, bytes int64)
	Partial  func(client.PartialImage)
}

type Result struct {
	SavedPath     string
	PreviewPath   string
	ThumbPath     string
	RawPath       string
	ImageB64      string
	RevisedPrompt string
	SourceEvent   string
}

const historyThumbMaxEdge = 256
const historyPreviewMaxEdge = 128

type Runner struct{}

func DefaultConfig() Config {
	return Config{
		TextModelID:          client.TextModel,
		ImageModelID:         client.ImageModel,
		Mode:                 client.ModeGenerate,
		APIMode:              client.APIModeResponses,
		RequestPolicy:        client.RequestPolicyOpenAI,
		ResponsesTransport:   client.ResponsesTransportSSE,
		Size:                 client.DefaultSize,
		Quality:              client.DefaultQuality,
		OutputFormat:         client.OutputFormat,
		Background:           client.DefaultBackground,
		OutputCompression:    client.DefaultOutputCompression,
		InputFidelity:        client.DefaultInputFidelity,
		ImageStyle:           client.DefaultImageStyle,
		Moderation:           client.DefaultModeration,
		ProxyMode:            client.ProxyModeSystem,
		ReasoningEffort:      client.DefaultReasoningEffort,
		ProtectStreamPreview: true,
		AutoRetryEnabled:     true,
		AutoRetryCount:       client.DefaultAutoRetryCount,
		CompletionSound: sharedCompat.CompletionSoundSettings{
			Enabled: true,
			Mode:    "default",
		},
		OutputDir:     DefaultOutputDir(),
		PartialImages: 0,
	}
}

func DefaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, "Pictures", "Image Studio")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "gio-images"
	}
	return filepath.Join(cwd, "gio-images")
}

func ParseSourcePaths(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == '\n' || r == '\r' || r == '\t' || r == ','
	})
	paths := make([]string, 0, len(fields))
	seen := map[string]struct{}{}
	for _, field := range fields {
		path := strings.TrimSpace(field)
		path = strings.Trim(path, `"`)
		path = strings.Trim(path, `'`)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}
	return paths
}

func (Runner) Run(ctx context.Context, cfg Config, cb Callbacks) (Result, error) {
	cfg = normalizeConfig(cfg)
	if cfg.APIKey == "" {
		return Result{}, fmt.Errorf("API Key 不能为空")
	}
	if cfg.Prompt == "" {
		return Result{}, fmt.Errorf("提示词不能为空")
	}
	if cfg.BaseURL == "" {
		return Result{}, fmt.Errorf("上游 BASE_URL 不能为空")
	}
	if cfg.Mode == client.ModeEdit && len(cfg.SourcePaths) == 0 {
		return Result{}, fmt.Errorf("图生图模式需要至少一张源图")
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("创建输出目录失败:%w", err)
	}
	imagesDir := filepath.Join(cfg.OutputDir, "images")
	logDir := filepath.Join(cfg.OutputDir, "log")
	if err := os.MkdirAll(imagesDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("创建图片目录失败:%w", err)
	}
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return Result{}, fmt.Errorf("创建日志目录失败:%w", err)
	}

	variants := []Config{cfg}
	if fallback := normalizeFallbackProfileConfig(cfg.FallbackProfile); fallback != nil {
		fallbackCfg := cfg
		fallbackCfg.APIKey = fallback.APIKey
		fallbackCfg.BaseURL = fallback.BaseURL
		fallbackCfg.TextModelID = fallback.TextModelID
		fallbackCfg.ImageModelID = fallback.ImageModelID
		fallbackCfg.APIMode = fallback.APIMode
		fallbackCfg.ResponsesTransport = fallback.ResponsesTransport
		fallbackCfg.RequestPolicy = fallback.RequestPolicy
		fallbackCfg.ImagesNewAPICompat = fallback.ImagesNewAPICompat
		fallbackCfg.ReasoningEffort = fallback.ReasoningEffort
		fallbackCfg.FallbackProfile = nil
		variants = append(variants, fallbackCfg)
	}

	var lastErr error
	for idx, variant := range variants {
		if idx > 0 {
			nonNilLog(cb.Log)("主上游自动重试失败，切换到备用上游再试一次...")
		}
		result, err := runSingleConfig(ctx, variant, cb, imagesDir, logDir, idx)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if errors.Is(err, context.Canceled) {
			return Result{}, err
		}
	}
	if lastErr != nil {
		return Result{}, lastErr
	}
	return Result{}, fmt.Errorf("多次请求后仍未成功")
}

func absPathOrRaw(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func normalizeConfig(cfg Config) Config {
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.TextModelID = strings.TrimSpace(cfg.TextModelID)
	cfg.ImageModelID = strings.TrimSpace(cfg.ImageModelID)
	cfg.Prompt = strings.TrimSpace(cfg.Prompt)
	cfg.Size = strings.TrimSpace(cfg.Size)
	cfg.Quality = strings.TrimSpace(cfg.Quality)
	cfg.OutputFormat = strings.TrimSpace(cfg.OutputFormat)
	cfg.Background = strings.TrimSpace(cfg.Background)
	cfg.ProxyMode = strings.TrimSpace(cfg.ProxyMode)
	cfg.ProxyURL = strings.TrimSpace(cfg.ProxyURL)
	cfg.OutputDir = strings.TrimSpace(cfg.OutputDir)
	cfg.NegativePrompt = strings.TrimSpace(cfg.NegativePrompt)
	cfg.InputFidelity = strings.TrimSpace(cfg.InputFidelity)
	cfg.ImageStyle = strings.TrimSpace(cfg.ImageStyle)
	cfg.Moderation = strings.TrimSpace(cfg.Moderation)
	cfg.UserIdentifier = strings.TrimSpace(cfg.UserIdentifier)
	if cfg.TextModelID == "" {
		cfg.TextModelID = client.TextModel
	}
	if cfg.ImageModelID == "" {
		cfg.ImageModelID = client.ImageModel
	}
	if cfg.Mode != client.ModeEdit {
		cfg.Mode = client.ModeGenerate
	}
	if cfg.APIMode != client.APIModeImages {
		cfg.APIMode = client.APIModeResponses
	}
	if cfg.RequestPolicy != client.RequestPolicyCompat {
		cfg.RequestPolicy = client.RequestPolicyOpenAI
	}
	if cfg.ResponsesTransport != client.ResponsesTransportWebSocket {
		cfg.ResponsesTransport = client.ResponsesTransportSSE
	}
	if cfg.Size == "" {
		cfg.Size = client.DefaultSize
	}
	if cfg.Quality == "" {
		cfg.Quality = client.DefaultQuality
	}
	if cfg.OutputFormat == "" {
		cfg.OutputFormat = client.OutputFormat
	}
	if cfg.Background == "" {
		cfg.Background = client.DefaultBackground
	}
	if cfg.OutputCompression <= 0 {
		cfg.OutputCompression = client.DefaultOutputCompression
	}
	if cfg.InputFidelity == "" {
		cfg.InputFidelity = client.DefaultInputFidelity
	}
	if cfg.ImageStyle == "" {
		cfg.ImageStyle = client.DefaultImageStyle
	}
	if cfg.Moderation == "" {
		cfg.Moderation = client.DefaultModeration
	}
	if cfg.ProxyMode == "" {
		cfg.ProxyMode = client.ProxyModeSystem
	}
	switch strings.ToLower(strings.TrimSpace(cfg.ReasoningEffort)) {
	case "low", "medium", "high", "xhigh":
	default:
		cfg.ReasoningEffort = client.DefaultReasoningEffort
	}
	if cfg.AutoRetryCount <= 0 {
		cfg.AutoRetryCount = client.DefaultAutoRetryCount
	}
	if cfg.AutoRetryCount > client.MaxAutoRetryCount {
		cfg.AutoRetryCount = client.MaxAutoRetryCount
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = DefaultOutputDir()
	}
	if cfg.PartialImages < 0 {
		cfg.PartialImages = client.DefaultPartialImages
	}
	cfg.FallbackProfileID = strings.TrimSpace(cfg.FallbackProfileID)
	cfg.FallbackProfile = normalizeFallbackProfileConfig(cfg.FallbackProfile)
	cfg.SourcePaths = normalizeSourcePaths(cfg.SourcePaths)
	return cfg
}

func normalizeFallbackProfileConfig(value *FallbackProfileConfig) *FallbackProfileConfig {
	if value == nil {
		return nil
	}
	next := *value
	next.APIKey = strings.TrimSpace(next.APIKey)
	next.BaseURL = strings.TrimSpace(next.BaseURL)
	next.TextModelID = strings.TrimSpace(next.TextModelID)
	next.ImageModelID = strings.TrimSpace(next.ImageModelID)
	next.RequestPolicy = normalizeRequestPolicy(next.RequestPolicy)
	if next.APIMode != client.APIModeImages {
		next.APIMode = client.APIModeResponses
	}
	if next.ResponsesTransport != client.ResponsesTransportWebSocket {
		next.ResponsesTransport = client.ResponsesTransportSSE
	}
	switch strings.ToLower(strings.TrimSpace(next.ReasoningEffort)) {
	case "low", "medium", "high", "xhigh":
	default:
		next.ReasoningEffort = client.DefaultReasoningEffort
	}
	if next.APIKey == "" || next.BaseURL == "" {
		return nil
	}
	return &next
}

func normalizeRequestPolicy(policy client.RequestPolicy) client.RequestPolicy {
	if policy == client.RequestPolicyCompat {
		return client.RequestPolicyCompat
	}
	return client.RequestPolicyOpenAI
}

func runSingleConfig(ctx context.Context, cfg Config, cb Callbacks, imagesDir string, logDir string, variantIndex int) (Result, error) {
	proxy, err := client.NormalizeProxyConfig(cfg.ProxyMode, cfg.ProxyURL)
	if err != nil {
		return Result{}, err
	}
	transport, err := client.PickTransportWithProxy(proxy)
	if err != nil {
		return Result{}, err
	}

	opts := client.Options{
		APIKey:             cfg.APIKey,
		BaseURL:            cfg.BaseURL,
		TextModelID:        cfg.TextModelID,
		ImageModelID:       cfg.ImageModelID,
		Prompt:             cfg.Prompt,
		Mode:               cfg.Mode,
		APIMode:            cfg.APIMode,
		ResponsesTransport: cfg.ResponsesTransport,
		RequestPolicy:      cfg.RequestPolicy,
		ImagesNewAPICompat: cfg.ImagesNewAPICompat,
		Size:               cfg.Size,
		Quality:            cfg.Quality,
		OutputFormat:       cfg.OutputFormat,
		Background:         cfg.Background,
		OutputCompression:  cfg.OutputCompression,
		InputFidelity:      cfg.InputFidelity,
		ImageStyle:         cfg.ImageStyle,
		Moderation:         cfg.Moderation,
		UserIdentifier:     cfg.UserIdentifier,
		Proxy:              proxy,
		ReasoningEffort:    cfg.ReasoningEffort,
		AutoRetryEnabled:   func() *bool { v := cfg.AutoRetryEnabled; return &v }(),
		AutoRetryCount:     cfg.AutoRetryCount,
		Seed:               cfg.Seed,
		NegativePrompt:     cfg.NegativePrompt,
		PartialImages:      cfg.PartialImages,
		DisablePreview:     cfg.PartialImages == 0,
	}
	if cfg.Mode == client.ModeEdit {
		if err := attachSourceImages(&opts, cfg.SourcePaths); err != nil {
			return Result{}, err
		}
	}

	timestamp := time.Now().Format("20060102-150405")
	if variantIndex > 0 {
		timestamp = fmt.Sprintf("%s-fallback%d", timestamp, variantIndex)
	}
	result, rawPath, err := client.RequestAndExtractWithRetriesAndPartial(
		ctx,
		transport,
		opts,
		logDir,
		timestamp,
		nonNilLog(cb.Log),
		cb.Progress,
		cb.Partial,
	)
	if err != nil {
		return Result{RawPath: absPathOrRaw(rawPath)}, err
	}
	rawPath = absPathOrRaw(rawPath)

	imageName := buildImageName(cfg.Mode, cfg.Prompt, timestamp, cfg.OutputFormat)
	savedPath, err := saveImage(result.ImageB64, filepath.Join(imagesDir, imageName))
	if err != nil {
		return Result{RawPath: rawPath, ImageB64: result.ImageB64}, err
	}
	previewPath := ""
	thumbPath := ""
	if nextPreview, nextThumb, mediaErr := EnsurePreviewAndThumbForPath(savedPath); mediaErr == nil {
		previewPath = nextPreview
		thumbPath = nextThumb
	} else {
		if nextPreview != "" {
			previewPath = nextPreview
		}
		if nextThumb != "" {
			thumbPath = nextThumb
		}
		nonNilLog(cb.Log)("生成历史预览/缩略图失败: " + mediaErr.Error())
	}
	return Result{
		SavedPath:     savedPath,
		PreviewPath:   previewPath,
		ThumbPath:     thumbPath,
		RawPath:       rawPath,
		RevisedPrompt: result.RevisedPrompt,
		SourceEvent:   result.SourceEvent,
	}, nil
}

func normalizeSourcePaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := map[string]struct{}{}
	for _, raw := range paths {
		path, err := client.NormalizePath(raw)
		if err != nil {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func attachSourceImages(opts *client.Options, paths []string) error {
	normalized := normalizeSourcePaths(paths)
	if len(normalized) == 0 {
		return fmt.Errorf("图生图模式需要至少一张源图")
	}
	if opts.APIMode == client.APIModeImages {
		opts.ImagePaths = normalized
		return nil
	}
	dataURLs := make([]string, 0, len(normalized))
	for _, path := range normalized {
		dataURL, err := client.ImageFileToDataURL(path)
		if err != nil {
			return err
		}
		dataURLs = append(dataURLs, dataURL)
	}
	opts.ImageDataURLs = dataURLs
	return nil
}

func buildImageName(mode client.Mode, prompt, timestamp, outputFormat string) string {
	prefix := "generate"
	if mode == client.ModeEdit {
		prefix = "edit"
	}
	slug := client.Slugify(prompt, "image")
	ext := client.FileExtForFormat(outputFormat)
	return fmt.Sprintf("image-%s-%s-%s.%s", prefix, slug, timestamp, ext)
}

func buildThumbName(imageName string) string {
	base := strings.TrimSuffix(filepath.Base(imageName), filepath.Ext(imageName))
	return base + ".png"
}

func previewOutputPathForSource(sourcePath string) string {
	sourcePath = absPathOrRaw(sourcePath)
	dir := filepath.Dir(sourcePath)
	previewsDir := filepath.Join(dir, "previews")
	if filepath.Base(dir) == "images" {
		previewsDir = filepath.Join(filepath.Dir(dir), "previews")
	}
	previewPath := filepath.Join(previewsDir, buildThumbName(filepath.Base(sourcePath)))
	if filepath.Clean(previewPath) == filepath.Clean(sourcePath) {
		stem := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
		previewPath = filepath.Join(previewsDir, stem+"-preview.png")
	}
	return previewPath
}

func thumbOutputPathForSource(sourcePath string) string {
	sourcePath = absPathOrRaw(sourcePath)
	dir := filepath.Dir(sourcePath)
	thumbsDir := filepath.Join(dir, "thumbs")
	if filepath.Base(dir) == "images" {
		thumbsDir = filepath.Join(filepath.Dir(dir), "thumbs")
	}
	thumbPath := filepath.Join(thumbsDir, buildThumbName(filepath.Base(sourcePath)))
	if filepath.Clean(thumbPath) == filepath.Clean(sourcePath) {
		stem := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
		thumbPath = filepath.Join(thumbsDir, stem+"-thumb.png")
	}
	return thumbPath
}

func saveImage(imageB64, outputPath string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(imageB64)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return "", fmt.Errorf("write image: %w", err)
	}
	abs, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return abs, nil
}

func EnsurePreviewForPath(sourcePath string) (string, error) {
	previewPath, _, err := EnsurePreviewAndThumbForPath(sourcePath)
	if strings.TrimSpace(previewPath) == "" {
		if err == nil {
			err = fmt.Errorf("preview path empty")
		}
		return "", err
	}
	return previewPath, err
}

func EnsurePreviewForPathWithFallback(sourcePath string, fallbackPath string) (string, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", fmt.Errorf("source path empty")
	}
	previewPath := previewOutputPathForSource(sourcePath)
	if info, statErr := os.Stat(previewPath); statErr == nil && !info.IsDir() {
		return absPathOrRaw(previewPath), nil
	}

	candidates := make([]string, 0, 2)
	if fallbackPath = strings.TrimSpace(fallbackPath); fallbackPath != "" {
		candidates = append(candidates, fallbackPath)
	}
	if len(candidates) == 0 || filepath.Clean(candidates[len(candidates)-1]) != filepath.Clean(sourcePath) {
		candidates = append(candidates, sourcePath)
	}

	var firstErr error
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, statErr := os.Stat(candidate); statErr != nil || info.IsDir() {
			if statErr != nil && firstErr == nil {
				firstErr = statErr
			}
			continue
		}
		src, err := decodeImageAtPath(candidate)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		resized, resizeErr := resizeThumbnailImage(src, historyPreviewMaxEdge)
		if resizeErr != nil {
			if firstErr == nil {
				firstErr = resizeErr
			}
			continue
		}
		next, writeErr := writeThumbnailImage(resized, previewPath)
		if writeErr != nil {
			if firstErr == nil {
				firstErr = writeErr
			}
			continue
		}
		return next, nil
	}
	if firstErr == nil {
		firstErr = os.ErrNotExist
	}
	return "", firstErr
}

func EnsureThumbForPath(sourcePath string) (string, error) {
	_, thumbPath, err := EnsurePreviewAndThumbForPath(sourcePath)
	if strings.TrimSpace(thumbPath) == "" {
		if err == nil {
			err = fmt.Errorf("thumb path empty")
		}
		return "", err
	}
	return thumbPath, err
}

func EnsurePreviewAndThumbForPath(sourcePath string) (previewPath string, thumbPath string, err error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "", "", fmt.Errorf("source path empty")
	}
	previewPath = previewOutputPathForSource(sourcePath)
	thumbPath = thumbOutputPathForSource(sourcePath)

	previewReady := false
	if info, statErr := os.Stat(previewPath); statErr == nil && !info.IsDir() {
		previewPath = absPathOrRaw(previewPath)
		previewReady = true
	}
	thumbReady := false
	if info, statErr := os.Stat(thumbPath); statErr == nil && !info.IsDir() {
		thumbPath = absPathOrRaw(thumbPath)
		thumbReady = true
	}
	if previewReady && thumbReady {
		return previewPath, thumbPath, nil
	}

	srcPath := sourcePath
	if !previewReady && thumbReady {
		srcPath = thumbPath
	}
	if _, statErr := os.Stat(srcPath); statErr != nil {
		if previewReady {
			srcPath = previewPath
		} else if thumbReady {
			srcPath = thumbPath
		} else {
			return previewPath, thumbPath, statErr
		}
	}
	src, err := decodeImageAtPath(srcPath)
	if err != nil {
		return previewPath, thumbPath, err
	}

	var firstErr error
	var thumbImage image.Image
	if !thumbReady {
		resized, resizeErr := resizeThumbnailImage(src, historyThumbMaxEdge)
		if resizeErr != nil {
			if firstErr == nil {
				firstErr = resizeErr
			}
		} else {
			thumbImage = resized
			next, thumbErr := writeThumbnailImage(thumbImage, thumbPath)
			if thumbErr != nil {
				if firstErr == nil {
					firstErr = thumbErr
				}
				thumbPath = ""
			} else {
				thumbPath = next
				thumbReady = true
			}
		}
	}
	if !previewReady {
		previewSource := src
		if thumbImage != nil {
			previewSource = thumbImage
		}
		resized, resizeErr := resizeThumbnailImage(previewSource, historyPreviewMaxEdge)
		if resizeErr != nil {
			if firstErr == nil {
				firstErr = resizeErr
			}
		} else {
			next, previewErr := writeThumbnailImage(resized, previewPath)
			if previewErr != nil {
				if firstErr == nil {
					firstErr = previewErr
				}
				previewPath = ""
			} else {
				previewPath = next
			}
		}
	}
	if strings.TrimSpace(previewPath) == "" && strings.TrimSpace(thumbPath) == "" && firstErr == nil {
		firstErr = fmt.Errorf("preview/thumb generation produced no outputs")
	}
	return previewPath, thumbPath, firstErr
}

func saveThumbnail(sourcePath, outputPath string, maxEdge int) (string, error) {
	src, err := decodeImageAtPath(sourcePath)
	if err != nil {
		return "", err
	}
	return saveThumbnailFromImage(src, outputPath, maxEdge)
}

func decodeImageAtPath(sourcePath string) (image.Image, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	src, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return src, nil
}

func saveThumbnailFromImage(src image.Image, outputPath string, maxEdge int) (string, error) {
	dst, err := resizeThumbnailImage(src, maxEdge)
	if err != nil {
		return "", err
	}
	return writeThumbnailImage(dst, outputPath)
}

func resizeThumbnailImage(src image.Image, maxEdge int) (image.Image, error) {
	if maxEdge <= 0 {
		maxEdge = historyThumbMaxEdge
	}
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 {
		return nil, fmt.Errorf("invalid image bounds: %dx%d", srcW, srcH)
	}
	scale := float64(maxEdge) / float64(max(srcW, srcH))
	if scale > 1 {
		scale = 1
	}
	dstW := max(1, int(float64(srcW)*scale+0.5))
	dstH := max(1, int(float64(srcH)*scale+0.5))
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Src, nil)
	return dst, nil
}

func writeThumbnailImage(dst image.Image, outputPath string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o700); err != nil {
		return "", err
	}
	out, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		_ = out.Close()
		if !ok {
			_ = os.Remove(outputPath)
		}
	}()
	if err := png.Encode(out, dst); err != nil {
		return "", fmt.Errorf("encode thumbnail: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	ok = true
	abs, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return abs, nil
}

func nonNilLog(log func(string)) func(string) {
	if log != nil {
		return log
	}
	return func(string) {}
}
