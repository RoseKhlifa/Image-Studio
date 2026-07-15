package kernel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

func OptimizePrompt(ctx context.Context, cfg Config) (string, error) {
	prompt := strings.TrimSpace(cfg.Prompt)
	if prompt == "" {
		return "", errors.New("提示词不能为空")
	}
	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL == "" {
		return "", errors.New("未配置上游 BASE_URL")
	}
	baseURL, err := client.ValidateBaseURLWithSecurity(baseURL, cfg.AllowInsecureConnection)
	if err != nil {
		return "", err
	}
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return "", errors.New("API Key 不能为空")
	}
	modelID := strings.TrimSpace(cfg.TextModelID)
	if modelID == "" {
		modelID = client.TextModel
	}

	instruction := "Rewrite the user's image prompt into a clearer, more detailed prompt for image generation. Keep the meaning, preserve the requested subject, and only return the improved prompt text. Do not add explanations, labels, markdown, or quotes."
	if cfg.Mode == client.ModeEdit {
		instruction += " Treat any attached images as reference context and preserve edit intent."
	}

	content := []map[string]any{
		{
			"type": "input_text",
			"text": fmt.Sprintf("Original prompt:\n%s", prompt),
		},
	}
	for _, dataURL := range cfg.SourceImageDataURLs {
		dataURL = strings.TrimSpace(dataURL)
		if dataURL == "" {
			continue
		}
		content = append(content, map[string]any{
			"type":      "input_image",
			"image_url": dataURL,
		})
	}
	for _, p := range cfg.SourcePaths {
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
		"model":        modelID,
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

	proxyConfig, err := client.NormalizeProxyConfig(cfg.ProxyMode, cfg.ProxyURL)
	if err != nil {
		return "", err
	}
	transport, err := client.NewHTTPTransportWithSecurity(proxyConfig, cfg.AllowInsecureConnection)
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

	text := strings.TrimSpace(extractResponseText(raw))
	if text == "" {
		return "", errors.New("上游没有返回优化后的提示词")
	}
	return text, nil
}

func extractResponseText(raw []byte) string {
	type outputText struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type contentItem struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type response struct {
		Output []struct {
			Type    string        `json:"type"`
			Content []contentItem `json:"content"`
		} `json:"output"`
		OutputText string `json:"output_text"`
	}

	var parsed response
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ""
	}
	if strings.TrimSpace(parsed.OutputText) != "" {
		return parsed.OutputText
	}
	for _, item := range parsed.Output {
		for _, content := range item.Content {
			if content.Type == "output_text" || content.Type == "text" {
				if strings.TrimSpace(content.Text) != "" {
					return content.Text
				}
			}
		}
	}
	return ""
}

func extractResponseErrorMessage(raw []byte) string {
	var payload struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Error.Message)
}
