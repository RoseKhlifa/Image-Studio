package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

type responseText struct {
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
	OutputText string `json:"output_text"`
	Error      *struct {
		Message string `json:"message"`
	} `json:"error"`
	Message string `json:"message"`
}

// prepareUploadSourcePaths flattens transparent PNG sources onto white
// backgrounds before upload so the upstream model sees the actual content.
// It returns the possibly rewritten paths plus a cleanup function for any temp
// files that were created.
func prepareUploadSourcePaths(paths []string) ([]string, func(), error) {
	out := make([]string, 0, len(paths))
	tmpFiles := make([]string, 0, len(paths))
	cleanup := func() {
		for _, p := range tmpFiles {
			_ = os.Remove(p)
		}
	}

	for _, rawPath := range paths {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			continue
		}
		rewrite, tmp, err := flattenTransparentImage(path)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		if tmp != "" {
			tmpFiles = append(tmpFiles, tmp)
		}
		out = append(out, rewrite)
	}

	return out, cleanup, nil
}

func flattenTransparentImage(path string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".png" {
		return path, "", nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return path, "", nil
	}
	if !imageHasTransparency(src) {
		return path, "", nil
	}

	// PNG references with transparency tend to be the problematic case in
	// issue #3, so always flatten them before upload.
	bounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), src, bounds.Min, draw.Over)

	tmp, err := os.CreateTemp("", "image-studio-flat-*.png")
	if err != nil {
		return "", "", err
	}
	if err := png.Encode(tmp, dst); err != nil {
		tmp.Close()
		_ = os.Remove(tmp.Name())
		return "", "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", "", err
	}
	return tmp.Name(), tmp.Name(), nil
}

func imageHasTransparency(src image.Image) bool {
	b := src.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			if a != 0xffff {
				return true
			}
		}
	}
	return false
}

func optimizePromptWithLLM(
	ctx context.Context,
	baseURL, apiKey, textModelID, mode, prompt string,
	sourcePaths []string,
	proxyConfig client.ProxyConfig,
) (string, error) {
	operation := strings.TrimSpace(mode)
	isDescribe := operation == "describe"
	if !isDescribe && strings.TrimSpace(prompt) == "" {
		return "", errors.New("提示词不能为空")
	}
	if isDescribe && len(sourcePaths) == 0 {
		return "", errors.New("图片反推必须提供画布图片")
	}
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", errors.New("未配置上游 BASE_URL")
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", errors.New("API Key 不能为空")
	}
	textModelID = strings.TrimSpace(textModelID)
	if textModelID == "" {
		textModelID = client.TextModel
	}

	instruction := "Rewrite the user's image prompt into a clearer, more detailed prompt for image generation. Keep the meaning, preserve the requested subject, and only return the improved prompt text. Do not add explanations, labels, markdown, or quotes."
	inputText := fmt.Sprintf("Original prompt:\n%s", strings.TrimSpace(prompt))
	if isDescribe {
		instruction = "Analyze the attached image and reconstruct a detailed image-generation prompt that could reproduce it. Describe the subject, composition, perspective, lighting, colors, materials, environment, and visual style. Return the prompt in Simplified Chinese. Only return the prompt text; do not add explanations, labels, markdown, or quotes."
		inputText = "为所附图片反推一段可用于重新生成相似画面的完整提示词。"
	} else if operation == "edit" {
		instruction += " Treat any attached images as reference context and preserve edit intent."
	}

	content := []map[string]any{
		{
			"type": "input_text",
			"text": inputText,
		},
	}
	for _, p := range sourcePaths {
		dataURL, err := client.ImageFileToDataURL(p)
		if err != nil {
			return "", err
		}
		content = append(content, map[string]any{
			"type":      "input_image",
			"image_url": dataURL,
		})
	}

	payload := map[string]any{
		"model":        textModelID,
		"instructions": instruction,
		"input": []map[string]any{
			{
				"role":    "user",
				"content": content,
			},
		},
		"reasoning": map[string]any{"effort": "low"},
		"store":     false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal prompt optimization payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/v1/responses", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", client.UserAgent())

	transport, err := client.NewHTTPTransport(proxyConfig)
	if err != nil {
		return "", err
	}
	httpClient := &http.Client{Timeout: 3 * time.Minute, Transport: transport}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode/100 != 2 {
		if msg := extractResponseErrorMessage(raw); msg != "" {
			return "", fmt.Errorf("上游返回 %d:%s", resp.StatusCode, msg)
		}
		return "", fmt.Errorf("上游返回 HTTP %d", resp.StatusCode)
	}

	text := extractResponseText(raw)
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("上游没有返回可用的优化结果")
	}
	return text, nil
}

func extractResponseText(raw []byte) string {
	var parsed responseText
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ""
	}
	if s := strings.TrimSpace(parsed.OutputText); s != "" {
		return s
	}
	for _, out := range parsed.Output {
		for _, content := range out.Content {
			if content.Type == "output_text" && strings.TrimSpace(content.Text) != "" {
				return content.Text
			}
		}
	}
	return ""
}

func extractResponseErrorMessage(raw []byte) string {
	var parsed responseText
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return strings.TrimSpace(string(raw))
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return parsed.Error.Message
	}
	if msg := strings.TrimSpace(parsed.Message); msg != "" {
		return msg
	}
	if s := strings.TrimSpace(string(raw)); s != "" {
		return s
	}
	return ""
}
