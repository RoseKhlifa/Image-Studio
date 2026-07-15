package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const websocketQuickReconnectAttempts = 1

type responsesWebSocketFallbackError struct {
	err error
}

func (e *responsesWebSocketFallbackError) Error() string {
	if e == nil || e.err == nil {
		return "responses websocket fallback"
	}
	return e.err.Error()
}

func (e *responsesWebSocketFallbackError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

type ResponsesWSRunStateSnapshot struct {
	AttemptIndex        int
	SocketEpoch         int
	CreatedAt           time.Time
	LastActivityAt      time.Time
	RequestPayload      []byte
	ResponseID          string
	LatestEventType     string
	ReceivedBytes       int64
	PartialPreviewCount int
	HasFinalImage       bool
	Cancelled           bool
	Completed           bool
}

type ProbeResponsesWebSocketOptions struct {
	BaseURL                 string
	APIKey                  string
	Proxy                   ProxyConfig
	Model                   string
	AllowInsecureConnection bool
}

func requestResponsesWithWebSocketReplay(
	ctx context.Context,
	opts Options,
	rawSink io.Writer,
	onProgress func(stage string, elapsedSeconds int, bytesReceived int64),
	onPartial func(PartialImage),
	attempt int,
	onLog func(string),
) (ImageResult, error) {
	httpPayload, err := BuildPayload(opts)
	if err != nil {
		return ImageResult{}, err
	}
	payload, err := buildResponsesWebSocketCreatePayload(httpPayload)
	if err != nil {
		return ImageResult{}, err
	}
	baseURL := strings.TrimSpace(opts.BaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(BaseURL)
	}
	if baseURL == "" {
		return ImageResult{}, errors.New("未配置上游 BASE_URL,请在「设置 → 上游 BASE_URL」中填入兼容 Responses API 的中转站地址")
	}
	snapshot := &ResponsesWSRunStateSnapshot{
		AttemptIndex:   attempt,
		CreatedAt:      time.Now(),
		LastActivityAt: time.Now(),
		RequestPayload: payload,
	}
	startedAt := time.Now()
	progressDone := make(chan struct{})
	defer close(progressDone)
	go func() {
		if onProgress == nil {
			return
		}
		ticker := time.NewTicker(time.Duration(StatusIntervalSecond) * time.Second)
		defer ticker.Stop()
		lastStage := "等待接口响应"
		for {
			select {
			case <-ctx.Done():
				return
			case <-progressDone:
				return
			case <-ticker.C:
				if snapshot.LatestEventType != "" {
					lastStage = SummarizeSSELine(`data: {"type":"` + snapshot.LatestEventType + `"}`)
					if lastStage == "" {
						lastStage = "模型处理中"
					}
				}
				onProgress(lastStage, int(time.Since(startedAt).Seconds()), snapshot.ReceivedBytes)
			}
		}
	}()
	if onLog != nil {
		onLog("使用 Responses WebSocket mode 发起请求...")
	}
	result, err := requestResponsesOverWebSocket(ctx, baseURL, opts.APIKey, opts.Proxy, opts.AllowInsecureConnection, payload, rawSink, onPartial, snapshot, startedAt, onProgress)
	var fallbackErr *responsesWebSocketFallbackError
	if errors.As(err, &fallbackErr) {
		if onLog != nil {
			onLog("Responses WebSocket 握手失败，当前上游不兼容该 WS 路径，自动切回 HTTP SSE...")
		}
		transport, terr := PickTransportWithProxyAndSecurity(opts.Proxy, opts.AllowInsecureConnection)
		if terr != nil {
			return ImageResult{}, terr
		}
		return RequestAndExtractWithPartial(ctx, transport, opts, rawSink, onProgress, onPartial)
	}
	if err != nil && !snapshot.HasFinalImage && onLog != nil {
		onLog("WebSocket 连接中断，正在重新连接并重放本次生成...")
	}
	return result, err
}

func NormalizeTextModel(modelID string) string {
	trimmed := strings.TrimSpace(modelID)
	if trimmed == "" {
		return TextModel
	}
	return trimmed
}

func NormalizeProxyTransportValue(value string) string {
	return string(normalizeResponsesTransport(ResponsesTransport(value)))
}

func ProbeResponsesWebSocket(ctx context.Context, opts ProbeResponsesWebSocketOptions) error {
	model := NormalizeTextModel(opts.Model)
	payload, err := json.Marshal(map[string]any{
		"type":  "response.create",
		"model": model,
		"store": false,
		"input": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "health check"},
				},
			},
		},
		"tools":    []map[string]any{},
		"generate": false,
	})
	if err != nil {
		return err
	}
	return probeResponsesWebSocketOnce(ctx, opts.BaseURL, opts.APIKey, opts.Proxy, opts.AllowInsecureConnection, payload)
}

func requestResponsesOverWebSocket(
	ctx context.Context,
	baseURL string,
	apiKey string,
	proxy ProxyConfig,
	allowInsecureConnection bool,
	payload []byte,
	rawSink io.Writer,
	onPartial func(PartialImage),
	snapshot *ResponsesWSRunStateSnapshot,
	startedAt time.Time,
	onProgress func(stage string, elapsedSeconds int, bytesReceived int64),
) (ImageResult, error) {
	if snapshot == nil {
		snapshot = &ResponsesWSRunStateSnapshot{}
	}
	snapshot.CreatedAt = time.Now()
	snapshot.LastActivityAt = snapshot.CreatedAt
	snapshot.RequestPayload = append(snapshot.RequestPayload[:0], payload...)

	var lastErr error
	for reconnect := 0; reconnect <= websocketQuickReconnectAttempts; reconnect++ {
		snapshot.SocketEpoch = reconnect + 1
		if reconnect > 0 {
			if rawSink != nil {
				_, _ = io.WriteString(rawSink, fmt.Sprintf("--- websocket-reconnect-%d ---\n", reconnect))
			}
		}
		result, err := requestResponsesOverWebSocketOnce(ctx, baseURL, apiKey, proxy, allowInsecureConnection, payload, rawSink, onPartial, snapshot, startedAt, onProgress)
		if err == nil {
			return result, nil
		}
		if rawSink != nil {
			_, _ = io.WriteString(rawSink, fmt.Sprintf("--- websocket-error-%d: %v ---\n", snapshot.SocketEpoch, err))
		}
		if isResponsesWebSocketFallbackError(err) {
			return ImageResult{}, &responsesWebSocketFallbackError{err: err}
		}
		lastErr = err
		if snapshot.HasFinalImage {
			return result, nil
		}
		if reconnect < websocketQuickReconnectAttempts {
			continue
		}
	}
	return ImageResult{}, lastErr
}

func requestResponsesOverWebSocketOnce(
	ctx context.Context,
	baseURL string,
	apiKey string,
	proxy ProxyConfig,
	allowInsecureConnection bool,
	payload []byte,
	rawSink io.Writer,
	onPartial func(PartialImage),
	snapshot *ResponsesWSRunStateSnapshot,
	startedAt time.Time,
	onProgress func(stage string, elapsedSeconds int, bytesReceived int64),
) (ImageResult, error) {
	wsURL, err := responsesWebSocketURL(baseURL, allowInsecureConnection)
	if err != nil {
		return ImageResult{}, err
	}
	dialer, err := newResponsesWebSocketDialer(proxy, allowInsecureConnection)
	if err != nil {
		return ImageResult{}, err
	}
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+apiKey)
	headers.Set("User-Agent", UserAgent())
	headers.Set("Accept", "application/json")

	conn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return ImageResult{}, describeWebSocketDialError(err, resp)
	}
	defer conn.Close()

	conn.SetPingHandler(func(appData string) error {
		snapshot.LastActivityAt = time.Now()
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})
	conn.SetPongHandler(func(string) error {
		snapshot.LastActivityAt = time.Now()
		return conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	})
	_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))

	done := make(chan struct{})
	defer close(done)
	go responsesWebSocketKeepalive(ctx, conn, done)

	if rawSink != nil {
		_, _ = io.WriteString(rawSink, fmt.Sprintf("--- websocket-session-%d ---\n", snapshot.SocketEpoch))
	}
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return ImageResult{}, fmt.Errorf("websocket write: %w", err)
	}

	collector := newResponseCollectorWithPartial(rawSink, onPartial)
	for {
		if ctx.Err() != nil {
			snapshot.Cancelled = true
			return ImageResult{}, ctx.Err()
		}
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if res, rerr := collector.result(); rerr == nil && res.ImageB64 != "" {
				snapshot.HasFinalImage = true
				snapshot.Completed = true
				return res, nil
			}
			return ImageResult{}, fmt.Errorf("websocket read: %w", err)
		}
		if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
			continue
		}
		snapshot.LastActivityAt = time.Now()
		line := bytes.TrimSpace(data)
		if len(line) == 0 {
			continue
		}
		_, _ = collector.Write(append([]byte("data: "), append(line, '\n')...))
		snapshot.ReceivedBytes = collector.bytesReceived()
		var ev Event
		if err := decodeEvent(string(line), &ev); err == nil {
			if evType, _ := ev["type"].(string); evType != "" {
				snapshot.LatestEventType = evType
				if onProgress != nil {
					stage := SummarizeSSELine(`data: {"type":"` + evType + `"}`)
					if stage == "" {
						stage = "模型处理中"
					}
					onProgress(stage, int(time.Since(startedAt).Seconds()), snapshot.ReceivedBytes)
				}
				switch evType {
				case "response.created":
					if responseAny, ok := ev["response"].(map[string]any); ok {
						if id, _ := responseAny["id"].(string); id != "" {
							snapshot.ResponseID = id
						}
					}
					if snapshot.ResponseID == "" {
						if id, _ := ev["response_id"].(string); id != "" {
							snapshot.ResponseID = id
						}
					}
				case "response.image_generation_call.partial_image":
					snapshot.PartialPreviewCount++
				case "response.output_item.done":
					itemAny, _ := ev["item"]
					item, _ := itemAny.(map[string]any)
					if item != nil {
						if itemType, _ := item["type"].(string); itemType == "image_generation_call" {
							if result, _ := item["result"].(string); result != "" {
								snapshot.HasFinalImage = true
							}
						}
					}
				case "response.completed":
					snapshot.Completed = true
					return collector.result()
				case "error":
					return ImageResult{}, fmt.Errorf("%s", DescribeProblem(string(line)))
				}
			}
		}
		if snapshot.HasFinalImage {
			if res, rerr := collector.result(); rerr == nil && res.ImageB64 != "" {
				snapshot.Completed = true
				return res, nil
			}
		}
	}
}

func responsesWebSocketKeepalive(ctx context.Context, conn *websocket.Conn, done <-chan struct{}) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			_ = conn.WriteControl(websocket.PingMessage, []byte("image-studio"), time.Now().Add(5*time.Second))
		}
	}
}

func responsesWebSocketURL(baseURL string, allowInsecureConnection bool) (string, error) {
	normalized, err := ValidateBaseURLWithSecurity(baseURL, allowInsecureConnection)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "https":
		parsed.Scheme = "wss"
	case "http":
		parsed.Scheme = "ws"
	default:
		return "", fmt.Errorf("BASE_URL 仅支持 http:// 或 https://")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/v1/responses"
	return parsed.String(), nil
}

func newResponsesWebSocketDialer(proxy ProxyConfig, allowInsecureConnection bool) (*websocket.Dialer, error) {
	proxyFn, err := proxyFunc(proxy)
	if err != nil {
		return nil, err
	}
	dialer := &websocket.Dialer{
		Proxy:            proxyFn,
		HandshakeTimeout: 30 * time.Second,
	}
	if allowInsecureConnection {
		// #nosec G402 -- this is restricted to an explicit per-upstream opt-in.
		dialer.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}
	}
	return dialer, nil
}

func buildResponsesWebSocketCreatePayload(httpPayload []byte) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(httpPayload, &body); err != nil {
		return nil, fmt.Errorf("decode responses payload: %w", err)
	}
	delete(body, "stream")
	delete(body, "background")
	body["type"] = "response.create"
	out, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode websocket payload: %w", err)
	}
	return out, nil
}

func probeResponsesWebSocketOnce(
	ctx context.Context,
	baseURL string,
	apiKey string,
	proxy ProxyConfig,
	allowInsecureConnection bool,
	payload []byte,
) error {
	wsURL, err := responsesWebSocketURL(baseURL, allowInsecureConnection)
	if err != nil {
		return err
	}
	dialer, err := newResponsesWebSocketDialer(proxy, allowInsecureConnection)
	if err != nil {
		return err
	}
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+apiKey)
	headers.Set("User-Agent", UserAgent())
	headers.Set("Accept", "application/json")
	conn, resp, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return describeWebSocketDialError(err, resp)
	}
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return fmt.Errorf("websocket write: %w", err)
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("websocket read: %w", err)
		}
		var ev Event
		if err := decodeEvent(string(bytes.TrimSpace(data)), &ev); err != nil {
			continue
		}
		switch evType, _ := ev["type"].(string); evType {
		case "response.created", "response.completed":
			return nil
		case "error":
			return fmt.Errorf("%s", DescribeProblem(string(data)))
		}
	}
}

func describeWebSocketDialError(err error, resp *http.Response) error {
	if err == nil {
		return nil
	}
	if resp != nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		summary := summarizeWebSocketHandshakeBody(body)
		if summary != "" {
			return fmt.Errorf("websocket handshake failed: HTTP %d: %s", resp.StatusCode, summary)
		}
		return fmt.Errorf("websocket handshake failed: HTTP %d", resp.StatusCode)
	}
	var handshakeErr websocket.HandshakeError
	if errors.As(err, &handshakeErr) {
		return fmt.Errorf("websocket handshake failed: %w", err)
	}
	text := err.Error()
	if strings.Contains(strings.ToLower(text), "bad handshake") {
		return fmt.Errorf("websocket handshake failed: %s", text)
	}
	return fmt.Errorf("websocket dial: %w", err)
}

func summarizeWebSocketHandshakeBody(body []byte) string {
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
	lower := strings.ToLower(text)
	if strings.Contains(lower, "websocket upgrade required") || strings.Contains(lower, "upgrade: websocket") {
		return "上游要求 WebSocket Upgrade,但当前链路没有正确转发 Upgrade: websocket。通常是中转站 / 反向代理 / 网关不支持或没放行 Responses WebSocket,建议切回 HTTP SSE。"
	}
	if len(text) > 160 {
		return text[:160]
	}
	return text
}

func isResponsesWebSocketFallbackError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "websocket handshake failed")
}
