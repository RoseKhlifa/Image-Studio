package backend

import (
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

const probeUpstreamTimeout = 20 * time.Second
const probeUpstreamMaxBody = 1 << 20

type modelsListProbeResponse struct {
	Data   []upstreamModelDescriptorJSON `json:"data"`
	Models []struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"models"`
}

type upstreamModelDescriptorJSON struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	OwnedBy     string `json:"owned_by"`
	DisplayName string `json:"name"`
}

func (s *Service) ProbeUpstream(opts ProbeUpstreamOptions) (ProbeUpstreamResult, error) {
	if s.ctx == nil {
		return ProbeUpstreamResult{}, errors.New("服务未启动")
	}
	return probeUpstream(s.ctx, opts)
}

func probeUpstream(parent context.Context, opts ProbeUpstreamOptions) (ProbeUpstreamResult, error) {
	apiKey := strings.TrimSpace(opts.APIKey)
	if apiKey == "" {
		return ProbeUpstreamResult{}, fmt.Errorf("API Key 不能为空")
	}
	baseURL, err := client.ValidateBaseURLWithSecurity(opts.BaseURL, opts.AllowInsecureConnection)
	if err != nil {
		return ProbeUpstreamResult{}, err
	}
	ctx, cancel := context.WithTimeout(parent, probeUpstreamTimeout)
	defer cancel()

	provider := client.NormalizeProvider(client.Provider(opts.Provider))
	modelsEndpoint := client.OpenAIAPIEndpoint(baseURL, "models")
	endpointLabel := "/v1/models"
	if provider == client.ProviderGoogle {
		modelsEndpoint = client.GoogleAPIEndpoint(baseURL, "models")
		endpointLabel = "/v1beta/models"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsEndpoint, nil)
	if err != nil {
		return ProbeUpstreamResult{}, fmt.Errorf("构造测活请求失败: %w", err)
	}
	if provider == client.ProviderGoogle {
		req.Header.Set("X-Goog-Api-Key", apiKey)
	} else {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", client.UserAgent())

	transport, err := client.NewHTTPTransportWithSecurity(client.ProxyConfig{Mode: opts.ProxyMode, URL: opts.ProxyURL}, opts.AllowInsecureConnection)
	if err != nil {
		return ProbeUpstreamResult{}, err
	}
	httpClient := &http.Client{Timeout: probeUpstreamTimeout, Transport: transport}
	resp, err := httpClient.Do(req)
	if err != nil {
		return ProbeUpstreamResult{}, fmt.Errorf("连接上游失败: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, probeUpstreamMaxBody))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		summary := summarizeProbeBody(body)
		if summary == "" && readErr != nil {
			summary = readErr.Error()
		}
		if summary != "" {
			return ProbeUpstreamResult{}, fmt.Errorf("上游 %s 返回 %d: %s", endpointLabel, resp.StatusCode, summary)
		}
		return ProbeUpstreamResult{}, fmt.Errorf("上游 %s 返回 %d", endpointLabel, resp.StatusCode)
	}
	if readErr != nil {
		return ProbeUpstreamResult{}, fmt.Errorf("读取上游响应失败: %w", readErr)
	}

	var parsed modelsListProbeResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ProbeUpstreamResult{}, fmt.Errorf("上游 %s 返回的 JSON 无效: %w", endpointLabel, err)
	}
	if provider == client.ProviderGoogle {
		for _, item := range parsed.Models {
			id := strings.TrimPrefix(strings.TrimSpace(item.Name), "models/")
			if id == "" {
				continue
			}
			parsed.Data = append(parsed.Data, upstreamModelDescriptorJSON{ID: id, Object: "model", OwnedBy: "google", DisplayName: item.DisplayName})
		}
	} else if parsed.Data == nil {
		return ProbeUpstreamResult{}, fmt.Errorf("上游 %s 响应缺少 data 数组", endpointLabel)
	}
	models := make([]UpstreamModelDescriptor, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		models = append(models, UpstreamModelDescriptor{
			ID:          id,
			Object:      strings.TrimSpace(item.Object),
			OwnedBy:     strings.TrimSpace(item.OwnedBy),
			DisplayName: strings.TrimSpace(item.DisplayName),
		})
	}
	result := ProbeUpstreamResult{
		ModelCount: len(parsed.Data),
		Models:     models,
	}
	if provider == client.ProviderOpenAI && strings.TrimSpace(opts.APIMode) == string(client.APIModeResponses) &&
		client.NormalizeProxyTransportValue(strings.TrimSpace(opts.ResponsesTransport)) == string(client.ResponsesTransportWebSocket) {
		result.ResponsesTransport = string(client.ResponsesTransportWebSocket)
		if wsErr := client.ProbeResponsesWebSocket(ctx, client.ProbeResponsesWebSocketOptions{
			BaseURL:                 baseURL,
			APIKey:                  apiKey,
			Proxy:                   client.ProxyConfig{Mode: opts.ProxyMode, URL: opts.ProxyURL},
			Model:                   client.TextModel,
			AllowInsecureConnection: opts.AllowInsecureConnection,
		}); wsErr != nil {
			result.ResponsesTransportOK = false
			result.ResponsesTransportError = wsErr.Error()
			return result, nil
		}
		result.ResponsesTransportOK = true
	}
	return result, nil
}

func summarizeProbeBody(body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return ""
	}
	var parsed struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if msg := strings.TrimSpace(parsed.Error.Message); msg != "" {
			text = msg
		} else if msg := strings.TrimSpace(parsed.Message); msg != "" {
			text = msg
		}
	}
	if len(text) > 160 {
		return text[:160]
	}
	return text
}
