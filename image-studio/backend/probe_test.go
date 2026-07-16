package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProbeUpstreamRequestsModelsFromBackend(t *testing.T) {
	var gotAuth string
	var gotUserAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.5","owned_by":"openai","name":"GPT 5.5"},{"id":"gpt-image-2","object":"model"}]}`))
	}))
	defer srv.Close()

	got, err := probeUpstream(context.Background(), ProbeUpstreamOptions{
		APIKey:  " sk-test ",
		BaseURL: srv.URL + "/",
	})
	if err != nil {
		t.Fatalf("ProbeUpstream returned error: %v", err)
	}
	if got.ModelCount != 2 {
		t.Fatalf("model count = %d, want 2", got.ModelCount)
	}
	if len(got.Models) != 2 {
		t.Fatalf("models len = %d, want 2", len(got.Models))
	}
	if got.Models[0].ID != "gpt-5.5" || got.Models[0].OwnedBy != "openai" || got.Models[0].DisplayName != "GPT 5.5" {
		t.Fatalf("unexpected first model: %+v", got.Models[0])
	}
	if got.Models[1].ID != "gpt-image-2" || got.Models[1].Object != "model" {
		t.Fatalf("unexpected second model: %+v", got.Models[1])
	}
	if gotAuth != "Bearer sk-test" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	if !strings.HasPrefix(gotUserAgent, "image-studio/") {
		t.Fatalf("unexpected User-Agent %q", gotUserAgent)
	}
}

func TestProbeUpstreamSkipsModelItemsWithoutID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.5"},{"owned_by":"relay"},{"id":"gpt-image-2"}]}`))
	}))
	defer srv.Close()

	got, err := probeUpstream(context.Background(), ProbeUpstreamOptions{
		APIKey:  "sk-test",
		BaseURL: srv.URL,
	})
	if err != nil {
		t.Fatalf("ProbeUpstream returned error: %v", err)
	}
	if got.ModelCount != 3 {
		t.Fatalf("model count = %d, want 3", got.ModelCount)
	}
	if len(got.Models) != 2 {
		t.Fatalf("models len = %d, want 2", len(got.Models))
	}
}

func TestProbeUpstreamRejectsInvalidModelsJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list"}`))
	}))
	defer srv.Close()

	_, err := probeUpstream(context.Background(), ProbeUpstreamOptions{
		APIKey:  "sk-test",
		BaseURL: srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "缺少 data 数组") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProbeUpstreamSummarizesNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"message":"forbidden key"}}`))
	}))
	defer srv.Close()

	_, err := probeUpstream(context.Background(), ProbeUpstreamOptions{
		APIKey:  "sk-test",
		BaseURL: srv.URL,
	})
	if err == nil || !strings.Contains(err.Error(), "403: forbidden key") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProbeUpstreamUsesCustomProxy(t *testing.T) {
	var proxiedURL string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxiedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"proxied-model"}]}`))
	}))
	defer proxy.Close()

	got, err := probeUpstream(context.Background(), ProbeUpstreamOptions{
		APIKey:    "sk-test",
		BaseURL:   "http://127.0.0.1:65535",
		ProxyMode: "custom",
		ProxyURL:  proxy.URL,
	})
	if err != nil {
		t.Fatalf("ProbeUpstream returned error: %v", err)
	}
	if got.ModelCount != 1 {
		t.Fatalf("model count = %d, want 1", got.ModelCount)
	}
	if proxiedURL != "http://127.0.0.1:65535/v1/models" {
		t.Fatalf("proxied URL = %q", proxiedURL)
	}
}

func TestProbeUpstreamAllowsExplicitInsecureTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"self-signed-model"}]}`))
	}))
	defer srv.Close()

	_, strictErr := probeUpstream(context.Background(), ProbeUpstreamOptions{
		APIKey:  "sk-test",
		BaseURL: srv.URL,
	})
	if strictErr == nil {
		t.Fatal("strict probe unexpectedly accepted a self-signed certificate")
	}

	got, err := probeUpstream(context.Background(), ProbeUpstreamOptions{
		APIKey:                  "sk-test",
		BaseURL:                 srv.URL,
		AllowInsecureConnection: true,
	})
	if err != nil {
		t.Fatalf("insecure probe returned error: %v", err)
	}
	if got.ModelCount != 1 || len(got.Models) != 1 || got.Models[0].ID != "self-signed-model" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestProbeUpstreamAllowsExplicitRemoteHTTP(t *testing.T) {
	got, err := probeUpstream(context.Background(), ProbeUpstreamOptions{
		APIKey:                  "sk-test",
		BaseURL:                 "http://relay.example.com",
		AllowInsecureConnection: true,
		ProxyMode:               "custom",
		ProxyURL:                newProbeProxyServer(t),
	})
	if err != nil {
		t.Fatalf("insecure HTTP probe returned error: %v", err)
	}
	if got.ModelCount != 1 {
		t.Fatalf("model count = %d, want 1", got.ModelCount)
	}
}

func newProbeProxyServer(t *testing.T) string {
	t.Helper()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"http-model"}]}`))
	}))
	t.Cleanup(proxy.Close)
	return proxy.URL
}

func TestProbeUpstreamReportsStructuredWebSocketProbeFailure(t *testing.T) {
	var gotUpgrade bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.5"}]}`))
			return
		}
		if r.URL.Path == "/v1/responses" {
			if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				gotUpgrade = true
			}
			http.Error(w, "ws unsupported", http.StatusBadRequest)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	got, err := probeUpstream(context.Background(), ProbeUpstreamOptions{
		APIKey:             "sk-test",
		BaseURL:            strings.Replace(srv.URL, "http://", "http://", 1),
		APIMode:            "responses",
		ResponsesTransport: "websocket",
	})
	if err != nil {
		t.Fatalf("ProbeUpstream returned error: %v", err)
	}
	if got.ModelCount != 1 {
		t.Fatalf("model count = %d, want 1", got.ModelCount)
	}
	if got.ResponsesTransport != "websocket" {
		t.Fatalf("responsesTransport = %q, want websocket", got.ResponsesTransport)
	}
	if got.ResponsesTransportOK {
		t.Fatalf("responsesTransportOK = true, want false")
	}
	if strings.TrimSpace(got.ResponsesTransportError) == "" {
		t.Fatalf("responsesTransportError should not be empty")
	}
	if !gotUpgrade {
		t.Fatalf("expected websocket probe upgrade attempt")
	}
}

func TestServiceProbeUpstreamRequiresStartup(t *testing.T) {
	svc := NewService()
	_, err := svc.ProbeUpstream(ProbeUpstreamOptions{APIKey: "sk-test", BaseURL: "http://127.0.0.1:1"})
	if err == nil || !strings.Contains(err.Error(), "服务未启动") {
		t.Fatalf("unexpected error: %v", err)
	}
}
