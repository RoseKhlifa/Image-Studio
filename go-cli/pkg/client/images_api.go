package client

// images_api.go — 适配标准的 OpenAI Images API:
//   POST {base}/v1/images/generations  (JSON,文生图)
//   POST {base}/v1/images/edits        (multipart/form-data,图生图)
//
// 与 Responses API 路径(client.go / sse.go)的最大区别:
//   - 结果事件形态不同;支持官方 Images API 的 stream/partial_images 时可流式预览,
//     否则回退解析一次性 JSON 响应。
//   - 多图编辑能力受上游约束(OpenAI 官方仅接受 1 张 image,部分中转站允许 image[] 数组),
//     为最大兼容,这里默认只取第一张源图;如果上游支持多张,可后续扩展
//   - 默认优先走 OpenAI 官方公开字段;若请求策略切到 compat,可附带 relay 扩展字段

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func classifyImageModel(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(normalized, "dall-e-2"):
		return "dalle2"
	case strings.HasPrefix(normalized, "dall-e-3"):
		return "dalle3"
	case strings.HasPrefix(normalized, "gpt-image"), strings.HasPrefix(normalized, "chatgpt-image"):
		return "gpt-image"
	default:
		return "other"
	}
}

func supportsImagesResponseFormat(model string, mode Mode) bool {
	family := classifyImageModel(model)
	if mode == ModeEdit {
		return family == "dalle2"
	}
	return family == "dalle2" || family == "dalle3"
}

func supportsImageModeration(model string) bool {
	return classifyImageModel(model) == "gpt-image"
}

func supportsImageBackground(model string) bool {
	return classifyImageModel(model) == "gpt-image"
}

func supportsOutputCompression(model, outputFormat string) bool {
	return supportsImageBackground(model) && (outputFormat == "jpeg" || outputFormat == "webp")
}

func supportsInputFidelity(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if strings.HasPrefix(normalized, "gpt-image-2") {
		return false
	}
	if strings.HasPrefix(normalized, "gpt-image-1.5") {
		return true
	}
	if strings.HasPrefix(normalized, "gpt-image-1-mini") {
		return true
	}
	if strings.HasPrefix(normalized, "gpt-image-1") {
		return true
	}
	if strings.HasPrefix(normalized, "chatgpt-image-latest") {
		return true
	}
	return false
}

func supportsImageStyle(model string, mode Mode) bool {
	return mode != ModeEdit && classifyImageModel(model) == "dalle3"
}

func isGoogleImageModel(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.HasPrefix(normalized, "gemini-") ||
		strings.HasPrefix(normalized, "imagen-") ||
		strings.Contains(normalized, "nano-banana")
}

func shouldUseImagesNonStreamingCompat(model string, explicit bool) bool {
	return explicit || isGoogleImageModel(model)
}

func normalizeImageStyle(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "vivid":
		return "vivid"
	case "natural":
		return "natural"
	default:
		return DefaultImageStyle
	}
}

type imagesAPIDatum struct {
	B64JSON       string `json:"b64_json"`
	URL           string `json:"url"`
	RevisedPrompt string `json:"revised_prompt"`
}

type imagesAPIResponse struct {
	Created int              `json:"created"`
	Data    []imagesAPIDatum `json:"data"`
	Error   *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

type imageStreamExtractor struct {
	partialB64 string
	final      ImageResult
	hasFinal   bool
	onPartial  func(PartialImage)
}

func (e *imageStreamExtractor) consume(line string) bool {
	stripped := strings.TrimSpace(line)
	if stripped == "" {
		return false
	}
	if !strings.HasPrefix(stripped, "data:") {
		return false
	}
	payload := strings.TrimSpace(stripped[len("data:"):])
	if payload == "" || payload == "[DONE]" {
		return true
	}
	var ev Event
	if err := decodeEvent(payload, &ev); err != nil {
		return false
	}
	evType, _ := ev["type"].(string)
	switch evType {
	case "image_generation.partial_image", "image_edit.partial_image":
		if b64, ok := ev["b64_json"].(string); ok && b64 != "" {
			e.partialB64 = b64
			partial := PartialImage{ImageB64: b64, PartialImageIndex: -1}
			if idx, ok := numberFromAny(ev["partial_image_index"]); ok {
				partial.PartialImageIndex = idx
			}
			if e.onPartial != nil {
				e.onPartial(partial)
			}
		}
		return true
	case "image_generation.completed", "image_edit.completed":
		if b64, ok := ev["b64_json"].(string); ok && b64 != "" {
			e.final = ImageResult{ImageB64: b64, SourceEvent: "images_api"}
			e.hasFinal = true
			return true
		}
	case "error":
		return true
	}
	if ev["object"] == "image.generation.result" || ev["object"] == "image.edit.result" {
		b, err := json.Marshal(ev)
		if err == nil {
			if result, err := parseImagesAPIResponseBytes(b, 200); err == nil {
				e.final = result
				e.hasFinal = true
				return true
			}
		}
	}
	if b, err := json.Marshal(ev); err == nil {
		if result, err := parseImagesAPIResponseBytes(b, http.StatusOK); err == nil {
			e.final = result
			e.hasFinal = true
			return true
		}
	}
	return true
}

func (e *imageStreamExtractor) result() (ImageResult, bool) {
	if e.hasFinal {
		return e.final, true
	}
	return ImageResult{}, false
}

// RequestImagesAPI executes a single (no-retry) request against the standard
// OpenAI Images API and returns the parsed image. Raw response body is teed
// to rawSink so callers can dump it for debugging.
func RequestImagesAPI(
	ctx context.Context,
	opts Options,
	rawSink io.Writer,
	onProgress func(stage string, elapsedSeconds int, bytesReceived int64),
) (ImageResult, error) {
	return RequestImagesAPIWithPartial(ctx, opts, rawSink, onProgress, nil)
}

func RequestImagesAPIWithPartial(
	ctx context.Context,
	opts Options,
	rawSink io.Writer,
	onProgress func(stage string, elapsedSeconds int, bytesReceived int64),
	onPartial func(PartialImage),
) (ImageResult, error) {
	if strings.TrimSpace(opts.APIKey) == "" {
		return ImageResult{}, ErrEmptyAPIKey
	}
	if strings.TrimSpace(opts.Prompt) == "" {
		return ImageResult{}, ErrEmptyPrompt
	}

	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		return ImageResult{}, errors.New("未配置上游 BASE_URL,请在「设置 → 上游 BASE_URL」中填入兼容 OpenAI Images API 的中转站地址")
	}
	baseURL, err := ValidateBaseURLWithSecurity(baseURL, opts.AllowInsecureConnection)
	if err != nil {
		return ImageResult{}, err
	}

	model := opts.ImageModelID
	if model == "" {
		model = ImageModel
	}
	size := opts.Size
	if size == "" {
		size = DefaultSize
	}
	quality := opts.Quality
	if quality == "" {
		quality = DefaultQuality
	}
	outputFormat := opts.OutputFormat
	if outputFormat == "" {
		outputFormat = OutputFormat
	}
	background := normalizeBackground(opts.Background)
	outputCompression := normalizeOutputCompression(opts.OutputCompression)
	inputFidelity := normalizeInputFidelity(opts.InputFidelity)
	imageStyle := normalizeImageStyle(opts.ImageStyle)
	moderation := normalizeModeration(opts.Moderation)
	userIdentifier := normalizeUserIdentifier(opts.UserIdentifier)
	partialImages := normalizePartialImages(opts.PartialImages)
	if opts.DisablePreview {
		partialImages = 0
	}
	includeExtended := shouldSendExtendedImageParameters(opts.RequestPolicy)
	useNewAPICompat := shouldUseImagesNonStreamingCompat(model, opts.ImagesNewAPICompat)
	useGoogleInteractions := shouldUseGoogleNativeInteractions(baseURL, model)

	var (
		url         string
		body        io.Reader
		contentType string
	)

	if useGoogleInteractions {
		paths := []string(nil)
		if opts.Mode == ModeEdit {
			paths = opts.imageSourcePathsForEdit()
			if len(paths) == 0 {
				return ImageResult{}, errors.New("Google Interactions 图生图需要至少一张源图")
			}
		}
		payload, err := buildGoogleInteractionPayload(opts, paths, model, size, outputFormat)
		if err != nil {
			return ImageResult{}, err
		}
		url, err = googleInteractionsEndpoint(baseURL)
		if err != nil {
			return ImageResult{}, err
		}
		body = bytes.NewReader(payload)
		contentType = "application/json"
	} else if opts.Mode == ModeEdit {
		paths := opts.imageSourcePathsForEdit()
		if len(paths) == 0 {
			return ImageResult{}, errors.New("图生图模式需要至少一张源图(请在面板里添加参考图)")
		}
		multipartBuf, mpType, err := buildEditsMultipart(paths, opts.MaskB64, opts.Prompt, model, size, quality, outputFormat, background, outputCompression, inputFidelity, moderation, userIdentifier, opts.NegativePrompt, opts.Seed, opts.RequestPolicy, partialImages, useNewAPICompat)
		if err != nil {
			return ImageResult{}, err
		}
		url = openAIAPIEndpoint(baseURL, "images/edits")
		body = multipartBuf
		contentType = mpType
	} else {
		payload := map[string]any{
			"model":         model,
			"prompt":        opts.Prompt,
			"n":             1,
			"size":          size,
			"quality":       quality,
			"output_format": outputFormat,
		}
		if supportsImageBackground(model) {
			payload["background"] = background
		}
		if supportsOutputCompression(model, outputFormat) {
			payload["output_compression"] = outputCompression
		}
		if supportsImageStyle(model, opts.Mode) && imageStyle != DefaultImageStyle {
			payload["style"] = imageStyle
		}
		if supportsImageModeration(model) {
			payload["moderation"] = moderation
		}
		if userIdentifier != "" {
			payload["user"] = userIdentifier
		}
		if useNewAPICompat || supportsImagesResponseFormat(model, opts.Mode) {
			payload["response_format"] = "b64_json"
		}
		if !useNewAPICompat {
			payload["stream"] = true
			payload["partial_images"] = partialImages
		}
		if includeExtended && opts.Seed != 0 {
			payload["seed"] = opts.Seed
		}
		if includeExtended && strings.TrimSpace(opts.NegativePrompt) != "" {
			payload["negative_prompt"] = opts.NegativePrompt
		}
		b, err := json.Marshal(payload)
		if err != nil {
			return ImageResult{}, fmt.Errorf("marshal payload: %w", err)
		}
		url = openAIAPIEndpoint(baseURL, "images/generations")
		body = bytes.NewReader(b)
		contentType = "application/json"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return ImageResult{}, err
	}
	req.Header.Set("Content-Type", contentType)
	if useGoogleInteractions {
		req.Header.Set("X-Goog-Api-Key", opts.APIKey)
		req.Header.Set("Accept", "application/json")
	} else {
		req.Header.Set("Authorization", "Bearer "+opts.APIKey)
		req.Header.Set("Accept", "text/event-stream, application/json")
	}
	req.Header.Set("User-Agent", UserAgent())

	transport, err := NewHTTPTransportWithSecurity(opts.Proxy, opts.AllowInsecureConnection)
	if err != nil {
		return ImageResult{}, err
	}
	httpClient := &http.Client{
		Timeout:   8 * time.Minute,
		Transport: transport,
	}

	startedAt := time.Now()
	progressStage := "等待 Images API 返回(无 SSE 保活)"
	if useGoogleInteractions {
		progressStage = "等待 Google Interactions 返回(无 SSE 保活)"
	}
	// Progress ticker — Images API has no streaming so we just tick elapsed time.
	stopProgress := make(chan struct{})
	if onProgress != nil {
		go func() {
			tick := time.NewTicker(time.Duration(StatusIntervalSecond) * time.Second)
			defer tick.Stop()
			for {
				select {
				case <-stopProgress:
					return
				case <-tick.C:
					onProgress(progressStage, int(time.Since(startedAt).Seconds()), 0)
				}
			}
		}()
	}
	defer close(stopProgress)

	resp, err := httpClient.Do(req)
	if err != nil {
		return ImageResult{}, err
	}
	defer resp.Body.Close()
	if useGoogleInteractions {
		return readGoogleInteractionResponse(ctx, resp, httpClient, rawSink, onProgress, startedAt)
	}

	contentTypeHeader := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentTypeHeader, "text/event-stream") {
		var rawBytes int64
		extractor := imageStreamExtractor{onPartial: onPartial}
		scanner := NewSSEScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			rawBytes += int64(len(line) + 1)
			if _, err := rawSink.Write(line); err != nil {
				return ImageResult{}, fmt.Errorf("write raw: %w", err)
			}
			if _, err := rawSink.Write([]byte("\n")); err != nil {
				return ImageResult{}, fmt.Errorf("write raw: %w", err)
			}
			if extractor.consume(string(line)) && onProgress != nil {
				onProgress("已收到 Images API 流式事件", int(time.Since(startedAt).Seconds()), rawBytes)
			}
		}
		if err := scanner.Err(); err != nil {
			if result, ok := extractor.result(); ok && result.ImageB64 != "" {
				return result, nil
			}
			return ImageResult{}, fmt.Errorf("read Images API stream: %w", err)
		}
		if resp.StatusCode/100 != 2 {
			return ImageResult{}, fmt.Errorf("上游返回 HTTP %d", resp.StatusCode)
		}
		if result, ok := extractor.result(); ok {
			return result, nil
		}
		return ImageResult{}, ErrNoImageInResponse
	}

	preview := newCappedPreviewBuffer(4096)
	teeReader := io.TeeReader(resp.Body, io.MultiWriter(rawSink, preview))

	dec := json.NewDecoder(teeReader)
	for {
		var parsed imagesAPIResponse
		if err := dec.Decode(&parsed); err != nil {
			if errors.Is(err, io.EOF) {
				if resp.StatusCode/100 != 2 {
					bodyPreview := preview.String()
					if len(bodyPreview) > 400 {
						bodyPreview = bodyPreview[:400] + "..."
					}
					return ImageResult{}, fmt.Errorf("上游返回 HTTP %d: %s", resp.StatusCode, bodyPreview)
				}
				return ImageResult{}, ErrNoImageInResponse
			}
			var typeErr *json.UnmarshalTypeError
			if useNewAPICompat && errors.As(err, &typeErr) && (typeErr.Value == "array" || typeErr.Value == "string") {
				continue
			}
			_, _ = io.Copy(io.MultiWriter(rawSink, preview), resp.Body)
			bodyPreview := preview.String()
			if len(bodyPreview) > 400 {
				bodyPreview = bodyPreview[:400] + "..."
			}
			if resp.StatusCode/100 != 2 {
				return ImageResult{}, fmt.Errorf("上游返回 HTTP %d: %s", resp.StatusCode, bodyPreview)
			}
			return ImageResult{}, fmt.Errorf("解析 Images API 响应失败:%w", err)
		}

		// Non-2xx with JSON body — decode has already captured the structured error.
		if resp.StatusCode/100 != 2 {
			if parsed.Error != nil {
				return ImageResult{}, fmt.Errorf("上游返回 %d:%s", resp.StatusCode, parsed.Error.Message)
			}
			bodyPreview := preview.String()
			if len(bodyPreview) > 400 {
				bodyPreview = bodyPreview[:400] + "..."
			}
			return ImageResult{}, fmt.Errorf("上游返回 HTTP %d: %s", resp.StatusCode, bodyPreview)
		}
		if parsed.Error != nil {
			return ImageResult{}, fmt.Errorf("上游返回错误:%s", parsed.Error.Message)
		}
		if len(parsed.Data) > 0 {
			d := parsed.Data[0]
			if d.B64JSON != "" {
				return imageResultFromImagesDatum(d), nil
			}
			if d.URL != "" {
				return downloadImagesAPIURL(ctx, httpClient, d.URL, d.RevisedPrompt, onProgress, startedAt)
			}
		}
		if !useNewAPICompat {
			return ImageResult{}, ErrNoImageInResponse
		}
	}
}

func readGoogleInteractionResponse(
	ctx context.Context,
	resp *http.Response,
	httpClient *http.Client,
	rawSink io.Writer,
	onProgress func(stage string, elapsedSeconds int, bytesReceived int64),
	startedAt time.Time,
) (ImageResult, error) {
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGoogleInteractionResponseBytes+1))
	if err != nil {
		return ImageResult{}, fmt.Errorf("读取 Google Interactions 响应失败:%w", err)
	}
	if len(data) > maxGoogleInteractionResponseBytes {
		return ImageResult{}, fmt.Errorf("Google Interactions 响应过大(>%dB 上限)", maxGoogleInteractionResponseBytes)
	}
	if _, err := rawSink.Write(data); err != nil {
		return ImageResult{}, fmt.Errorf("write raw: %w", err)
	}
	image, err := extractGoogleInteractionImage(data, resp.StatusCode)
	if err != nil {
		return ImageResult{}, err
	}
	if strings.TrimSpace(image.Data) != "" {
		result, err := imageResultFromGoogleInteraction(image)
		if err != nil {
			return ImageResult{}, err
		}
		if onProgress != nil {
			onProgress("已收到 Google Interactions 图片", int(time.Since(startedAt).Seconds()), int64(len(data)))
		}
		return result, nil
	}
	if strings.TrimSpace(image.URI) != "" {
		result, err := downloadImagesAPIURL(ctx, httpClient, image.URI, "", onProgress, startedAt)
		if err != nil {
			return ImageResult{}, fmt.Errorf("下载 Google Interactions URI 图片失败:%w", err)
		}
		result.SourceEvent = "google_interactions_url"
		return result, nil
	}
	return ImageResult{}, ErrNoImageInResponse
}

func imageResultFromImagesDatum(d imagesAPIDatum) ImageResult {
	return ImageResult{
		ImageB64:      d.B64JSON,
		RevisedPrompt: d.RevisedPrompt,
		SourceEvent:   "images_api",
	}
}

func downloadImagesAPIURL(
	ctx context.Context,
	httpClient *http.Client,
	rawURL string,
	revisedPrompt string,
	onProgress func(stage string, elapsedSeconds int, bytesReceived int64),
	startedAt time.Time,
) (ImageResult, error) {
	parsedURL, err := neturl.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return ImageResult{}, fmt.Errorf("上游返回的图片 URL 无效:%s", rawURL)
	}
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "https" && scheme != "http" {
		return ImageResult{}, fmt.Errorf("上游返回的图片 URL 协议不支持:%s", parsedURL.Scheme)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return ImageResult{}, err
	}
	req.Header.Set("Accept", "image/png, image/jpeg, image/webp, */*")
	req.Header.Set("User-Agent", UserAgent())

	resp, err := httpClient.Do(req)
	if err != nil {
		return ImageResult{}, fmt.Errorf("下载上游 URL 图片失败:%w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return ImageResult{}, fmt.Errorf("下载上游 URL 图片返回 HTTP %d", resp.StatusCode)
	}
	if resp.ContentLength > MaxInputImageBytes {
		return ImageResult{}, fmt.Errorf("上游 URL 图片过大(%dB > %dB 上限)", resp.ContentLength, MaxInputImageBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxInputImageBytes+1))
	if err != nil {
		return ImageResult{}, fmt.Errorf("读取上游 URL 图片失败:%w", err)
	}
	if int64(len(data)) > MaxInputImageBytes {
		return ImageResult{}, fmt.Errorf("上游 URL 图片过大(>%dB 上限)", MaxInputImageBytes)
	}
	if mimeType := detectImageMimeTypeFromBytes(data); mimeType == "" {
		return ImageResult{}, errors.New("上游 URL 没有返回支持的 PNG/JPEG/WebP 图片")
	}
	if onProgress != nil {
		onProgress("已下载 Images API URL 图片", int(time.Since(startedAt).Seconds()), int64(len(data)))
	}
	return ImageResult{
		ImageB64:      base64.StdEncoding.EncodeToString(data),
		RevisedPrompt: revisedPrompt,
		SourceEvent:   "images_api_url",
	}, nil
}

func parseImagesAPIResponseBytes(raw []byte, statusCode int) (ImageResult, error) {
	var parsed imagesAPIResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ImageResult{}, err
	}
	if statusCode/100 != 2 {
		if parsed.Error != nil {
			return ImageResult{}, fmt.Errorf("上游返回 %d:%s", statusCode, parsed.Error.Message)
		}
		return ImageResult{}, fmt.Errorf("上游返回 HTTP %d", statusCode)
	}
	if parsed.Error != nil {
		return ImageResult{}, fmt.Errorf("上游返回错误:%s", parsed.Error.Message)
	}
	if len(parsed.Data) == 0 || parsed.Data[0].B64JSON == "" {
		return ImageResult{}, ErrNoImageInResponse
	}
	return imageResultFromImagesDatum(parsed.Data[0]), nil
}

type cappedPreviewBuffer struct {
	buf []byte
	max int
}

func newCappedPreviewBuffer(max int) *cappedPreviewBuffer {
	return &cappedPreviewBuffer{max: max}
}

func (b *cappedPreviewBuffer) Write(p []byte) (int, error) {
	if len(b.buf) < b.max {
		remain := b.max - len(b.buf)
		if len(p) < remain {
			remain = len(p)
		}
		b.buf = append(b.buf, p[:remain]...)
	}
	return len(p), nil
}

func (b *cappedPreviewBuffer) String() string {
	return string(b.buf)
}

// imageSourcePathsForEdit picks the source-image paths for an Images API edit.
// Prefers ImagePaths (raw files, no decode needed). If only data URLs are
// provided, the caller is responsible for writing them to a temp file first
// — see writeDataURLToTemp below.
func (o Options) imageSourcePathsForEdit() []string {
	paths := make([]string, 0, len(o.ImagePaths)+1)
	for _, p := range o.ImagePaths {
		if strings.TrimSpace(p) != "" {
			paths = append(paths, p)
		}
	}
	if len(paths) > 0 {
		return paths
	}
	// Fallback: data URLs → temp files.
	for _, du := range o.EffectiveImageDataURLs() {
		if p, err := writeDataURLToTemp(du); err == nil {
			paths = append(paths, p)
		}
	}
	return paths
}

// writeDataURLToTemp materialises a `data:...;base64,...` URL to a temp file
// and returns its path. Caller is responsible for cleanup; we leave it for the
// OS temp sweeper since these are small and we want them to survive retries.
func writeDataURLToTemp(dataURL string) (string, error) {
	idx := strings.Index(dataURL, ",")
	if !strings.HasPrefix(dataURL, "data:") || idx < 0 {
		return "", errors.New("not a data URL")
	}
	header := dataURL[5:idx] // e.g. "image/png;base64"
	payload := dataURL[idx+1:]
	if !strings.Contains(header, "base64") {
		return "", errors.New("data URL not base64")
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", err
	}
	ext := ".png"
	if strings.HasPrefix(header, "image/jpeg") {
		ext = ".jpg"
	} else if strings.HasPrefix(header, "image/webp") {
		ext = ".webp"
	}
	f, err := os.CreateTemp("", "image-studio-edit-*"+ext)
	if err != nil {
		return "", err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Name(), nil
}

// buildEditsMultipart constructs the multipart/form-data body for /v1/images/edits.
// 多张源图按 image[] / image[1] / ... 形式串联 —— 不同中转站对多图编辑支持不一,
// 仅第一张是 OpenAI 官方接受的最小可用形态,其余作为兼容性 best-effort。
func buildEditsMultipart(
	paths []string, maskB64, prompt, model, size, quality, outputFormat, background string, outputCompression int, inputFidelity, moderation, userIdentifier, negativePrompt string, seed int64, requestPolicy RequestPolicy, partialImages int, useNewAPICompat bool,
) (*bytes.Buffer, string, error) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)

	for i, p := range paths {
		fieldName := "image"
		if i > 0 {
			// Some relays accept multiple `image` fields, others want image[] —
			// we send both to maximise compatibility. The extra field is cheap.
			fieldName = "image[]"
		}
		if err := writeMultipartFile(w, fieldName, p); err != nil {
			return nil, "", fmt.Errorf("attach %s: %w", filepath.Base(p), err)
		}
	}

	if strings.TrimSpace(maskB64) != "" {
		raw, err := base64.StdEncoding.DecodeString(maskB64)
		if err != nil {
			return nil, "", fmt.Errorf("蒙版图片 base64 无效:%w", err)
		}
		if len(raw) == 0 {
			return nil, "", errors.New("蒙版图片为空")
		}
		if len(raw) > MaxInputImageBytes {
			return nil, "", fmt.Errorf("蒙版图片过大(%dB > %dB 上限)", len(raw), MaxInputImageBytes)
		}
		// Preserve the actual PNG/JPEG/WebP type for compatible relays instead
		// of relabeling arbitrary bytes as PNG. Official OpenAI users can still
		// supply a PNG mask when their selected model requires it.
		maskMimeType := detectImageMimeTypeFromBytes(raw)
		if strings.TrimSpace(maskMimeType) == "" {
			return nil, "", errors.New("蒙版图片不是支持的 PNG/JPEG/WebP 格式")
		}
		maskExt := imageExtensionForMimeType(maskMimeType)
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="mask"; filename="mask.%s"`, maskExt))
		h.Set("Content-Type", maskMimeType)
		fw, err := w.CreatePart(h)
		if err != nil {
			return nil, "", err
		}
		if _, err := fw.Write(raw); err != nil {
			return nil, "", err
		}
	}

	_ = w.WriteField("prompt", prompt)
	_ = w.WriteField("model", model)
	_ = w.WriteField("n", "1")
	_ = w.WriteField("size", size)
	_ = w.WriteField("quality", quality)
	if strings.TrimSpace(outputFormat) != "" {
		_ = w.WriteField("output_format", outputFormat)
	}
	if supportsImageBackground(model) {
		_ = w.WriteField("background", background)
	}
	if supportsOutputCompression(model, outputFormat) {
		_ = w.WriteField("output_compression", fmt.Sprintf("%d", outputCompression))
	}
	if supportsInputFidelity(model) && inputFidelity != DefaultInputFidelity {
		_ = w.WriteField("input_fidelity", inputFidelity)
	}
	if supportsImageModeration(model) {
		_ = w.WriteField("moderation", moderation)
	}
	if userIdentifier != "" {
		_ = w.WriteField("user", userIdentifier)
	}
	if useNewAPICompat || supportsImagesResponseFormat(model, ModeEdit) {
		_ = w.WriteField("response_format", "b64_json")
	}
	if !useNewAPICompat {
		_ = w.WriteField("stream", "true")
		_ = w.WriteField("partial_images", fmt.Sprintf("%d", partialImages))
	}
	if shouldSendExtendedImageParameters(requestPolicy) && seed != 0 {
		_ = w.WriteField("seed", fmt.Sprintf("%d", seed))
	}
	if shouldSendExtendedImageParameters(requestPolicy) && strings.TrimSpace(negativePrompt) != "" {
		_ = w.WriteField("negative_prompt", negativePrompt)
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf, w.FormDataContentType(), nil
}

func writeMultipartFile(w *multipart.Writer, fieldName, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() > MaxInputImageBytes {
		return fmt.Errorf("源图过大(%dB > %dB 上限)", st.Size(), MaxInputImageBytes)
	}
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, filepath.Base(path)))
	h.Set("Content-Type", mimeForPath(path))
	fw, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(fw, f)
	return err
}

func mimeForPath(p string) string {
	ext := strings.ToLower(filepath.Ext(p))
	if m, ok := SupportedImageMime[ext]; ok {
		return m
	}
	return "application/octet-stream"
}
