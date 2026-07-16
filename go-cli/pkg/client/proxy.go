package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	ProxyModeNone   = "none"
	ProxyModeSystem = "system"
	ProxyModeCustom = "custom"
)

// ProxyConfig controls how outbound upstream requests pick a proxy.
// Empty Mode is treated as "system" for backward compatibility.
type ProxyConfig struct {
	Mode string
	URL  string
}

func NormalizeProxyConfig(mode, rawURL string) (ProxyConfig, error) {
	normalizedMode := strings.ToLower(strings.TrimSpace(mode))
	switch normalizedMode {
	case "", ProxyModeSystem:
		return ProxyConfig{Mode: ProxyModeSystem}, nil
	case ProxyModeNone:
		return ProxyConfig{Mode: ProxyModeNone}, nil
	case ProxyModeCustom:
		proxyURL, err := normalizeCustomProxyURL(rawURL)
		if err != nil {
			return ProxyConfig{}, err
		}
		return ProxyConfig{Mode: ProxyModeCustom, URL: proxyURL}, nil
	default:
		return ProxyConfig{}, fmt.Errorf("代理模式无效:%s", mode)
	}
}

func normalizeCustomProxyURL(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	if cleaned == "" {
		return "", fmt.Errorf("自定义代理地址不能为空")
	}
	parsed, err := url.Parse(cleaned)
	if err != nil {
		return "", fmt.Errorf("代理地址无效:%w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("代理地址仅支持 http:// 或 https://")
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("代理地址必须包含主机")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("代理地址不能包含 query 或 fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("代理地址不能包含路径")
	}
	parsed.Scheme = scheme
	parsed.Path = ""
	return parsed.String(), nil
}

func proxyFunc(config ProxyConfig) (func(*http.Request) (*url.URL, error), error) {
	normalized, err := NormalizeProxyConfig(config.Mode, config.URL)
	if err != nil {
		return nil, err
	}
	switch normalized.Mode {
	case ProxyModeNone:
		return nil, nil
	case ProxyModeCustom:
		parsed, err := url.Parse(normalized.URL)
		if err != nil {
			return nil, fmt.Errorf("代理地址无效:%w", err)
		}
		return http.ProxyURL(parsed), nil
	default:
		return systemProxyFunc(), nil
	}
}

func NewHTTPTransport(config ProxyConfig) (*http.Transport, error) {
	return NewHTTPTransportWithSecurity(config, false)
}

func NewHTTPTransportWithSecurity(config ProxyConfig, allowInsecureConnection bool) (*http.Transport, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	proxy, err := proxyFunc(config)
	if err != nil {
		return nil, err
	}
	transport.Proxy = proxy
	if allowInsecureConnection {
		tlsConfig := transport.TLSClientConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		} else {
			tlsConfig = tlsConfig.Clone()
		}
		// #nosec G402 -- this is restricted to an explicit per-upstream opt-in.
		tlsConfig.InsecureSkipVerify = true
		transport.TLSClientConfig = tlsConfig
	}
	return transport, nil
}
