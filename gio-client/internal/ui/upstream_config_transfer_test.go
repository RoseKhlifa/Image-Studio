package ui

import "testing"

func TestParseUpstreamConfigImportJSONParsesExportFile(t *testing.T) {
	raw := `{
  "version": 1,
  "activeProfileId": "p1",
  "profiles": [
    {
      "id": "p1",
      "name": "Responses · 默认",
      "apiMode": "responses",
      "responsesTransport": "websocket",
      "requestPolicy": "compat",
      "imagesNewAPICompat": false,
      "baseURL": "https://example.com/v1",
      "textModelID": "gpt-5.5",
      "imageModelID": "gpt-image-2",
      "reasoningEffort": "high",
      "concurrencyLimit": 3,
      "fallbackProfileId": "p2",
      "createdAt": 1,
      "apiKey": "sk-test"
    }
  ]
}`
	parsed, err := parseUpstreamConfigImportJSON(raw)
	if err != nil {
		t.Fatalf("parseUpstreamConfigImportJSON: %v", err)
	}
	if parsed.ActiveProfileID != "p1" {
		t.Fatalf("activeProfileID=%q want p1", parsed.ActiveProfileID)
	}
	if len(parsed.Profiles) != 1 {
		t.Fatalf("profiles len=%d want 1", len(parsed.Profiles))
	}
	profile := parsed.Profiles[0]
	if profile.BaseURL != "https://example.com" || profile.ResponsesTransport != "websocket" || profile.RequestPolicy != "compat" || profile.ConcurrencyLimit != 3 || profile.FallbackProfileID != "p2" || profile.APIKey != "sk-test" {
		t.Fatalf("unexpected parsed profile: %#v", profile)
	}
}

func TestParseUpstreamConfigImportJSONParsesNewAPIChannelConn(t *testing.T) {
	raw := `{"_type":"newapi_channel_conn","url":"https://relay.example.com/v1","key":"sk-newapi"}`
	parsed, err := parseUpstreamConfigImportJSON(raw)
	if err != nil {
		t.Fatalf("parseUpstreamConfigImportJSON: %v", err)
	}
	if len(parsed.Profiles) != 1 {
		t.Fatalf("profiles len=%d want 1", len(parsed.Profiles))
	}
	profile := parsed.Profiles[0]
	if profile.Name != "NewAPI · relay.example.com" {
		t.Fatalf("name=%q want NewAPI · relay.example.com", profile.Name)
	}
	if profile.BaseURL != "https://relay.example.com" {
		t.Fatalf("baseURL=%q want https://relay.example.com", profile.BaseURL)
	}
	if profile.APIKey != "sk-newapi" {
		t.Fatalf("apiKey=%q want sk-newapi", profile.APIKey)
	}
}
