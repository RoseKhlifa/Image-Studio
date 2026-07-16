package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

const maxGoogleInteractionResponseBytes = MaxInputImageBytes*4/3 + 2*1024*1024

var googleInteractionAspectRatios = []string{
	"1:8", "1:4", "2:3", "3:4", "4:5", "1:1", "5:4", "4:3", "3:2", "16:9", "21:9", "4:1", "8:1",
}

type googleInteractionImage struct {
	Type     string `json:"type"`
	Data     string `json:"data"`
	URI      string `json:"uri"`
	MimeType string `json:"mime_type"`
}

type googleInteractionStep struct {
	Type    string                   `json:"type"`
	Content []googleInteractionImage `json:"content"`
}

type googleInteractionResponse struct {
	Object      string                  `json:"object"`
	Status      string                  `json:"status"`
	OutputImage *googleInteractionImage `json:"output_image,omitempty"`
	Steps       []googleInteractionStep `json:"steps"`
	Error       *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

func buildGoogleInteractionPayload(opts Options, paths []string, model, size, outputFormat string) ([]byte, error) {
	if strings.TrimSpace(opts.MaskB64) != "" {
		return nil, fmt.Errorf("Google Interactions 不支持 OpenAI mask 参数；请清除蒙版，或改用支持 /v1/images/edits 的中转站")
	}
	input, err := buildGoogleInteractionInput(opts.Prompt, paths)
	if err != nil {
		return nil, err
	}
	responseFormat, err := googleInteractionResponseFormat(size, outputFormat)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"model":           model,
		"input":           input,
		"response_format": responseFormat,
		"store":           false,
	}
	return json.Marshal(payload)
}

func buildGoogleInteractionInput(prompt string, paths []string) (any, error) {
	if len(paths) == 0 {
		return prompt, nil
	}
	content := make([]map[string]string, 0, len(paths)+1)
	content = append(content, map[string]string{"type": "text", "text": prompt})
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.Size() > MaxInputImageBytes {
			return nil, fmt.Errorf("源图过大(%dB > %dB 上限)", info.Size(), MaxInputImageBytes)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		mimeType := detectImageMimeTypeFromBytes(data)
		if mimeType == "" {
			return nil, fmt.Errorf("Google Interactions 不支持源图格式:%s", path)
		}
		content = append(content, map[string]string{
			"type":      "image",
			"mime_type": mimeType,
			"data":      base64.StdEncoding.EncodeToString(data),
		})
	}
	return content, nil
}

func googleInteractionResponseFormat(size, outputFormat string) (map[string]string, error) {
	format := map[string]string{
		"type":     "image",
		"delivery": "inline",
	}
	switch strings.ToLower(strings.TrimSpace(outputFormat)) {
	case "", "png":
		format["mime_type"] = "image/png"
	case "jpg", "jpeg":
		format["mime_type"] = "image/jpeg"
	default:
		return nil, fmt.Errorf("Google Interactions 当前只支持 PNG/JPEG 输出，请调整输出格式后重试")
	}
	width, height, ok := parseGoogleInteractionSize(size)
	if !ok {
		return format, nil
	}
	format["aspect_ratio"] = closestGoogleInteractionAspectRatio(float64(width) / float64(height))
	maxSide := width
	if height > maxSide {
		maxSide = height
	}
	switch {
	case maxSide <= 768:
		format["image_size"] = "512"
	case maxSide >= 3072:
		format["image_size"] = "4K"
	case maxSide >= 1536:
		format["image_size"] = "2K"
	default:
		format["image_size"] = "1K"
	}
	return format, nil
}

func parseGoogleInteractionSize(raw string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(raw)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}
	width, errWidth := strconv.Atoi(parts[0])
	height, errHeight := strconv.Atoi(parts[1])
	return width, height, errWidth == nil && errHeight == nil && width > 0 && height > 0
}

func closestGoogleInteractionAspectRatio(target float64) string {
	best := "1:1"
	bestDistance := math.Inf(1)
	for _, candidate := range googleInteractionAspectRatios {
		parts := strings.Split(candidate, ":")
		left, _ := strconv.ParseFloat(parts[0], 64)
		right, _ := strconv.ParseFloat(parts[1], 64)
		distance := math.Abs(math.Log((left / right) / target))
		if distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}
	return best
}

func extractGoogleInteractionImage(raw []byte, statusCode int) (googleInteractionImage, error) {
	var parsed googleInteractionResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		if statusCode/100 != 2 {
			return googleInteractionImage{}, fmt.Errorf("Google Interactions 返回 HTTP %d: %s", statusCode, strings.TrimSpace(string(raw)))
		}
		return googleInteractionImage{}, fmt.Errorf("解析 Google Interactions 响应失败:%w", err)
	}
	if statusCode/100 != 2 || parsed.Error != nil {
		if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
			return googleInteractionImage{}, fmt.Errorf("Google Interactions 返回错误:%s", parsed.Error.Message)
		}
		return googleInteractionImage{}, fmt.Errorf("Google Interactions 返回 HTTP %d", statusCode)
	}
	candidates := make([]googleInteractionImage, 0, 4)
	if parsed.OutputImage != nil {
		candidates = append(candidates, *parsed.OutputImage)
	}
	for stepIndex := len(parsed.Steps) - 1; stepIndex >= 0; stepIndex-- {
		content := parsed.Steps[stepIndex].Content
		for contentIndex := len(content) - 1; contentIndex >= 0; contentIndex-- {
			if content[contentIndex].Type == "image" {
				candidates = append(candidates, content[contentIndex])
			}
		}
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Data) != "" || strings.TrimSpace(candidate.URI) != "" {
			return candidate, nil
		}
	}
	status := strings.TrimSpace(parsed.Status)
	if status != "" {
		return googleInteractionImage{}, fmt.Errorf("Google Interactions 响应未包含 image data/uri(status=%s)", status)
	}
	return googleInteractionImage{}, fmt.Errorf("Google Interactions 响应未包含 image data/uri")
}

func imageResultFromGoogleInteraction(image googleInteractionImage) (ImageResult, error) {
	data := strings.TrimSpace(image.Data)
	if data == "" {
		return ImageResult{}, ErrNoImageInResponse
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return ImageResult{}, fmt.Errorf("Google Interactions 图片 base64 无效:%w", err)
	}
	if len(raw) > MaxInputImageBytes {
		return ImageResult{}, fmt.Errorf("Google Interactions 图片过大(%dB > %dB 上限)", len(raw), MaxInputImageBytes)
	}
	if detectImageMimeTypeFromBytes(raw) == "" {
		return ImageResult{}, fmt.Errorf("Google Interactions 没有返回支持的 PNG/JPEG/WebP 图片")
	}
	return ImageResult{ImageB64: data, SourceEvent: "google_interactions"}, nil
}
