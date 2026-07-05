package client

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateBaseURL normalizes and validates the relay base URL.
// HTTPS is required for non-loopback hosts because prompts, images, and API
// keys are all sent over this connection.
func ValidateBaseURL(raw string) (string, error) {
	cleaned := strings.TrimRight(strings.TrimSpace(raw), "/")
	if cleaned == "" {
		return "", fmt.Errorf("未配置上游 BASE_URL")
	}
	cleaned = strings.TrimSuffix(cleaned, "/v1")
	cleaned = strings.TrimRight(cleaned, "/")
	if cleaned == "" {
		return "", fmt.Errorf("未配置上游 BASE_URL")
	}
	u, err := url.Parse(cleaned)
	if err != nil {
		return "", fmt.Errorf("BASE_URL 无效: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("BASE_URL 必须包含协议和主机,例如 https://example.com")
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return cleaned, nil
	case "http":
		if isLoopbackHost(u.Hostname()) {
			return cleaned, nil
		}
		return "", fmt.Errorf("拒绝使用非 TLS 上游: %s。只有 localhost / 127.0.0.1 / ::1 允许 http://", cleaned)
	default:
		return "", fmt.Errorf("BASE_URL 仅支持 http:// 或 https://")
	}
}

func OpenAIAPIEndpoint(baseURL, endpointPath string) string {
	cleaned := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path := strings.Trim(strings.TrimSpace(endpointPath), "/")
	if path == "" {
		return cleaned
	}
	if isVersionedOpenAICompatibilityBaseURL(cleaned) {
		return cleaned + "/" + path
	}
	return cleaned + "/v1/" + path
}

func openAIAPIEndpoint(baseURL, endpointPath string) string {
	return OpenAIAPIEndpoint(baseURL, endpointPath)
}

func isVersionedOpenAICompatibilityBaseURL(raw string) bool {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil {
		return false
	}
	path := strings.ToLower(strings.TrimRight(u.Path, "/"))
	return strings.HasSuffix(path, "/openai")
}

func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
