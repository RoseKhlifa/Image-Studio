package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// BuildPayload mirrors Python's build_payload. Returns canonical JSON bytes.
//
// When opts has one or more image data URLs (via ImageDataURLs or the legacy
// single ImageDataURL field), action becomes "edit" and each URL is appended
// as its own input_image content block, in order. When opts.MaskB64 is
// non-empty, it is embedded as the tool's "input_image_mask.image_url"
// parameter using a base64 data URL.
func BuildPayload(opts Options) ([]byte, error) {
	if strings.TrimSpace(opts.Prompt) == "" {
		return nil, ErrEmptyPrompt
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
	moderation := normalizeModeration(opts.Moderation)
	reasoningEffort := normalizeReasoningEffort(opts.ReasoningEffort)
	userIdentifier := normalizeUserIdentifier(opts.UserIdentifier)
	includeExtended := shouldSendExtendedImageParameters(opts.RequestPolicy)

	content := []map[string]any{
		{"type": "input_text", "text": opts.Prompt},
	}
	action := "generate"
	imageURLs := opts.EffectiveImageDataURLs()
	if err := validateResponsesMaskInput(opts.Mode, imageURLs, opts.MaskB64); err != nil {
		return nil, err
	}
	for _, url := range imageURLs {
		content = append(content, map[string]any{
			"type":      "input_image",
			"image_url": url,
		})
	}
	if len(imageURLs) > 0 {
		action = "edit"
	}

	imgModel := opts.ImageModelID
	if imgModel == "" {
		imgModel = ImageModel
	}
	tool := map[string]any{
		"type":           "image_generation",
		"action":         action,
		"size":           size,
		"quality":        quality,
		"output_format":  outputFormat,
		"partial_images": 0,
	}
	if supportsImageBackground(imgModel) {
		tool["background"] = background
	}
	if supportsOutputCompression(imgModel, outputFormat) {
		tool["output_compression"] = outputCompression
	}
	if supportsInputFidelity(imgModel) && len(imageURLs) > 0 && inputFidelity != DefaultInputFidelity {
		tool["input_fidelity"] = inputFidelity
	}
	if supportsImageModeration(imgModel) {
		tool["moderation"] = moderation
	}
	if strings.TrimSpace(opts.MaskB64) != "" {
		tool["input_image_mask"] = map[string]any{
			"image_url": imageDataURLFromBase64(strings.TrimSpace(opts.MaskB64), ""),
		}
	}
	if includeExtended && opts.Seed != 0 {
		tool["seed"] = opts.Seed
	}
	if includeExtended && strings.TrimSpace(opts.NegativePrompt) != "" {
		tool["negative_prompt"] = opts.NegativePrompt
	}
	tool["partial_images"] = normalizePartialImages(opts.PartialImages)
	if opts.DisablePreview {
		tool["partial_images"] = 0
	}

	textModel := opts.TextModelID
	if textModel == "" {
		textModel = TextModel
	}
	payload := map[string]any{
		"model": textModel,
		"input": []map[string]any{
			{"role": "user", "content": content},
		},
		"tools":       []map[string]any{tool},
		"tool_choice": map[string]any{"type": "image_generation"},
		"reasoning":   map[string]any{"effort": reasoningEffort},
		"store":       false,
		"stream":      true,
	}
	// 实测此条 instructions 能让 gpt-5.5 把用户 prompt 字字传给 image_generation,
	// 而不是惯常的「改写润色再生」流程。改 wording 可能失效 —— 经验值。
	payload["instructions"] = "You are a tool runner. Pass the user prompt to image_generation VERBATIM. DO NOT rewrite, expand, polish, or revise it in any way. Use the exact text the user gave."
	if userIdentifier != "" {
		payload["safety_identifier"] = userIdentifier
	}

	// Use a non-escaping encoder so 中文 prompts don't get \uXXXX-mangled.
	var buf strings.Builder
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		return nil, fmt.Errorf("encode payload: %w", err)
	}
	// Encoder appends a trailing '\n'; strip for cleanliness.
	out := strings.TrimRight(buf.String(), "\n")
	return []byte(out), nil
}

func normalizePartialImages(value int) int {
	if value <= 0 {
		return DefaultPartialImages
	}
	if value > 3 {
		return 3
	}
	return value
}

func normalizeUserIdentifier(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) > 64 {
		return string(runes[:64])
	}
	return trimmed
}

func normalizeModeration(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "auto") {
		return "auto"
	}
	return DefaultModeration
}

func normalizeReasoningEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	case "xhigh":
		return "xhigh"
	default:
		return DefaultReasoningEffort
	}
}

func parseSizeValue(size string) (width int, height int, ok bool) {
	match := regexp.MustCompile(`^(\d+)x(\d+)$`).FindStringSubmatch(strings.TrimSpace(size))
	if len(match) != 3 {
		return 0, 0, false
	}
	w := 0
	h := 0
	if _, err := fmt.Sscanf(match[1], "%d", &w); err != nil {
		return 0, 0, false
	}
	if _, err := fmt.Sscanf(match[2], "%d", &h); err != nil {
		return 0, 0, false
	}
	if w <= 0 || h <= 0 {
		return 0, 0, false
	}
	return w, h, true
}

func normalizeOpenAIImageSize(width, height int) (int, int, bool) {
	const (
		minSide   = 64
		maxSide   = 3840
		maxPixels = 3840 * 2160
		maxAspect = 3.0
		alignment = 16
		minAspect = 1.0 / maxAspect
	)
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}

	targetWidth := float64(width)
	targetHeight := float64(height)
	aspect := targetWidth / targetHeight
	if aspect > maxAspect {
		aspect = maxAspect
	}
	if aspect < minAspect {
		aspect = minAspect
	}
	if targetWidth/targetHeight != aspect {
		if targetWidth >= targetHeight {
			targetWidth = targetHeight * aspect
		} else {
			targetHeight = targetWidth / aspect
		}
	}
	if currentMax := math.Max(targetWidth, targetHeight); currentMax > maxSide {
		scale := float64(maxSide) / currentMax
		targetWidth *= scale
		targetHeight *= scale
	}
	if pixelCount := targetWidth * targetHeight; pixelCount > maxPixels {
		scale := math.Sqrt(float64(maxPixels) / pixelCount)
		targetWidth *= scale
		targetHeight *= scale
	}

	widthCandidates := alignedDimensionCandidates(targetWidth, minSide, maxSide, alignment)
	heightCandidates := alignedDimensionCandidates(targetHeight, minSide, maxSide, alignment)
	bestW, bestH := 0, 0
	bestDistance := math.Inf(1)
	bestAspectDistance := math.Inf(1)
	bestAreaDistance := math.Inf(1)

	for _, candidateWidth := range widthCandidates {
		for _, candidateHeight := range heightCandidates {
			if !sizeWithinLimits(candidateWidth, candidateHeight, minSide, maxSide, maxPixels, maxAspect, minAspect) {
				continue
			}
			distance := sizeDistance(candidateWidth, candidateHeight, targetWidth, targetHeight)
			aspectDistance := math.Abs((float64(candidateWidth) / float64(candidateHeight)) - (targetWidth / targetHeight))
			areaDistance := math.Abs(float64(candidateWidth*candidateHeight)-targetWidth*targetHeight) / math.Max(targetWidth*targetHeight, 1)
			if distance < bestDistance ||
				(distance == bestDistance && aspectDistance < bestAspectDistance) ||
				(distance == bestDistance && aspectDistance == bestAspectDistance && areaDistance < bestAreaDistance) {
				bestW = candidateWidth
				bestH = candidateHeight
				bestDistance = distance
				bestAspectDistance = aspectDistance
				bestAreaDistance = areaDistance
			}
		}
	}
	if bestW == 0 || bestH == 0 {
		return 0, 0, false
	}
	return bestW, bestH, true
}

func alignedDimensionCandidates(value float64, min, max, alignment int) []int {
	clamped := math.Max(float64(min), math.Min(float64(max), value))
	candidates := []int{
		int(math.Round(clamped/float64(alignment))) * alignment,
		int(math.Floor(clamped/float64(alignment))) * alignment,
		int(math.Ceil(clamped/float64(alignment))) * alignment,
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate < min {
			candidate = min
		}
		if candidate > max {
			candidate = max
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	sort.Slice(out, func(i, j int) bool {
		left := math.Abs(float64(out[i]) - clamped)
		right := math.Abs(float64(out[j]) - clamped)
		if left == right {
			return out[i] > out[j]
		}
		return left < right
	})
	return out
}

func sizeDistance(width, height int, targetWidth, targetHeight float64) float64 {
	return math.Abs(float64(width)-targetWidth)/math.Max(targetWidth, 1) +
		math.Abs(float64(height)-targetHeight)/math.Max(targetHeight, 1)
}

func sizeWithinLimits(width, height, minSide, maxSide, maxPixels int, maxAspect, minAspect float64) bool {
	if width < minSide || height < minSide || width > maxSide || height > maxSide {
		return false
	}
	if width*height > maxPixels {
		return false
	}
	aspect := float64(width) / float64(height)
	return aspect <= maxAspect && aspect >= minAspect
}

func repairSizeForOpenAIOptions(opts Options) *Options {
	width, height, ok := parseSizeValue(opts.Size)
	if !ok {
		return nil
	}
	nextWidth, nextHeight, ok := normalizeOpenAIImageSize(width, height)
	if !ok {
		return nil
	}
	nextSize := fmt.Sprintf("%dx%d", nextWidth, nextHeight)
	if nextSize == strings.TrimSpace(opts.Size) {
		return nil
	}
	repaired := opts
	repaired.Size = nextSize
	return &repaired
}

func normalizeBackground(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "opaque":
		return "opaque"
	case "transparent":
		return "transparent"
	default:
		return DefaultBackground
	}
}

func normalizeOutputCompression(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizeInputFidelity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low":
		return "low"
	case "high":
		return "high"
	default:
		return DefaultInputFidelity
	}
}

var slugRe = regexp.MustCompile(`-{2,}`)

// Slugify mirrors Python's slugify: keep ASCII word chars and CJK; collapse separators.
func Slugify(text, fallback string) string {
	text = strings.ToLower(strings.TrimSpace(text))

	var b strings.Builder
	for _, r := range text {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '_':
			b.WriteRune(r)
		case r >= 0x4e00 && r <= 0x9fff:
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	s := slugRe.ReplaceAllString(b.String(), "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		// Truncate by rune count, not byte count, to avoid splitting CJK.
		runes := []rune(s)
		if len(runes) > 40 {
			s = string(runes[:40])
		}
	}
	if s == "" {
		if fallback == "" {
			return "image"
		}
		return fallback
	}
	return s
}

// NormalizePath strips surrounding quotes and expands ~ like Python's normalize_path_input.
func NormalizePath(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.Trim(cleaned, `"`)
	cleaned = strings.Trim(cleaned, `'`)
	if cleaned == "" {
		return "", fmt.Errorf("image path must not be empty")
	}
	if strings.HasPrefix(cleaned, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			cleaned = filepath.Join(home, strings.TrimPrefix(cleaned, "~"))
		}
	}
	return cleaned, nil
}

// ImageFileToDataURL reads a local image and returns a base64 data: URL.
func ImageFileToDataURL(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("找不到图片文件:%s", path)
	}
	if info.IsDir() {
		return "", fmt.Errorf("路径不是文件:%s", path)
	}
	ext := strings.ToLower(filepath.Ext(path))
	mime, ok := SupportedImageMime[ext]
	if !ok {
		supported := strings.Join([]string{".jpeg", ".jpg", ".png", ".webp"}, ", ")
		extLabel := ext
		if extLabel == "" {
			extLabel = "(无扩展名)"
		}
		return "", fmt.Errorf("不支持的图片格式:%s。支持:%s", extLabel, supported)
	}
	if info.Size() > MaxInputImageBytes {
		return "", fmt.Errorf("图片文件超过 50MB,请换一张更小的图片")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

func imageDataURLFromBase64(raw, mime string) string {
	encoded := strings.TrimSpace(raw)
	if encoded == "" {
		return ""
	}
	cleanMime := strings.TrimSpace(mime)
	if cleanMime == "" {
		cleanMime = detectImageMimeTypeFromBase64(encoded)
	}
	if cleanMime == "" {
		cleanMime = "image/png"
	}
	return fmt.Sprintf("data:%s;base64,%s", cleanMime, encoded)
}

func detectImageMimeTypeFromBase64(raw string) string {
	encoded := strings.TrimSpace(raw)
	if encoded == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return detectImageMimeTypeFromBytes(decoded)
}

func detectImageMimeTypeFromBytes(data []byte) string {
	if len(data) >= 8 &&
		data[0] == 0x89 &&
		data[1] == 0x50 &&
		data[2] == 0x4e &&
		data[3] == 0x47 &&
		data[4] == 0x0d &&
		data[5] == 0x0a &&
		data[6] == 0x1a &&
		data[7] == 0x0a {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if len(data) >= 12 &&
		data[0] == 0x52 &&
		data[1] == 0x49 &&
		data[2] == 0x46 &&
		data[3] == 0x46 &&
		data[8] == 0x57 &&
		data[9] == 0x45 &&
		data[10] == 0x42 &&
		data[11] == 0x50 {
		return "image/webp"
	}
	return ""
}

func imageExtensionForMimeType(mime string) string {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg":
		return "jpg"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}

func normalizeRequestPolicy(policy RequestPolicy) RequestPolicy {
	if policy == RequestPolicyCompat {
		return RequestPolicyCompat
	}
	return RequestPolicyOpenAI
}

func shouldSendExtendedImageParameters(policy RequestPolicy) bool {
	return normalizeRequestPolicy(policy) == RequestPolicyCompat
}

// FormatBytes mirrors Python's format_bytes.
func FormatBytes(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(size)/1024/1024)
}
