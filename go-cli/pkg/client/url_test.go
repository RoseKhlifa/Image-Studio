package client

import "testing"

func TestValidateBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "https ok", raw: " https://relay.example.com/ ", want: "https://relay.example.com"},
		{name: "https trailing v1 trimmed", raw: "https://relay.example.com/v1", want: "https://relay.example.com"},
		{name: "https nested path trailing v1 trimmed", raw: "https://relay.example.com/proxy/v1/", want: "https://relay.example.com/proxy"},
		{name: "http localhost ok", raw: "http://localhost:8787/api", want: "http://localhost:8787/api"},
		{name: "http loopback ip ok", raw: "http://127.0.0.1:8080", want: "http://127.0.0.1:8080"},
		{name: "http remote rejected", raw: "http://relay.example.com", wantErr: true},
		{name: "missing scheme rejected", raw: "relay.example.com", wantErr: true},
		{name: "empty rejected", raw: "  ", wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateBaseURL(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got success %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOpenAIAPIEndpointKeepsVersionedOpenAICompatibilityBase(t *testing.T) {
	t.Parallel()

	if got := openAIAPIEndpoint("https://generativelanguage.googleapis.com/v1beta/openai", "images/generations"); got != "https://generativelanguage.googleapis.com/v1beta/openai/images/generations" {
		t.Fatalf("google endpoint = %q", got)
	}
	if got := openAIAPIEndpoint("https://relay.example.com/api", "/images/edits"); got != "https://relay.example.com/api/v1/images/edits" {
		t.Fatalf("relay endpoint = %q", got)
	}
}
