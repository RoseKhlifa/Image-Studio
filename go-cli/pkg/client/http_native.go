package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// NativeTransport uses net/http to issue the request and streams the body
// line-by-line through a custom SSE scanner.
type NativeTransport struct {
	// Client is optional; if nil a sensible default is used.
	Client                  *http.Client
	Proxy                   ProxyConfig
	AllowInsecureConnection bool
}

func (t *NativeTransport) Stream(ctx context.Context, req Request, rawSink io.Writer, progress chan<- string) error {
	httpReq, err := http.NewRequestWithContext(ctx, "POST", req.URL, bytes.NewReader(req.Payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("User-Agent", UserAgent())

	cli := t.Client
	if cli == nil {
		transport, err := NewHTTPTransportWithSecurity(t.Proxy, t.AllowInsecureConnection)
		if err != nil {
			return err
		}
		transport.DisableCompression = true
		transport.MaxIdleConnsPerHost = 2
		transport.ResponseHeaderTimeout = 60 * time.Second
		cli = &http.Client{
			// No global timeout: SSE streams legitimately take minutes.
			// Cancellation is via ctx.
			Transport: transport,
		}
	}

	resp, err := cli.Do(httpReq)
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	scanner := NewSSEScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if _, err := rawSink.Write(line); err != nil {
			return fmt.Errorf("write raw: %w", err)
		}
		if _, err := rawSink.Write([]byte("\n")); err != nil {
			return fmt.Errorf("write raw: %w", err)
		}
		if progress != nil {
			if summary := SummarizeSSELine(string(line)); summary != "" {
				select {
				case progress <- summary:
				default:
					// Drop status updates if consumer is slow; never block streaming.
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	// If the response code wasn't 2xx and body was empty/non-SSE, surface it.
	if resp.StatusCode >= 400 {
		// We've already streamed whatever body came through; signal upstream failure.
		return fmt.Errorf("upstream HTTP %d", resp.StatusCode)
	}
	return nil
}
