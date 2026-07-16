package client

import (
	"context"
	"io"
)

// Transport is the abstraction over HTTP-with-SSE used by the client.
// Stream MUST write the raw response body (line-by-line) to rawSink,
// and SHOULD push human-readable status updates onto progress (best-effort).
// The progress channel is owned by the caller; Transport does not close it.
type Transport interface {
	Stream(ctx context.Context, req Request, rawSink io.Writer, progress chan<- string) error
}

// PickTransport returns the native HTTP implementation.
func PickTransport() (Transport, error) {
	return &NativeTransport{}, nil
}

func PickTransportWithProxy(proxy ProxyConfig) (Transport, error) {
	return PickTransportWithProxyAndSecurity(proxy, false)
}

func PickTransportWithProxyAndSecurity(proxy ProxyConfig, allowInsecureConnection bool) (Transport, error) {
	if _, err := NormalizeProxyConfig(proxy.Mode, proxy.URL); err != nil {
		return nil, err
	}
	return &NativeTransport{Proxy: proxy, AllowInsecureConnection: allowInsecureConnection}, nil
}
