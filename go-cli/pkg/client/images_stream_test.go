package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestImagesAPIMultilineCompletedEvent(t *testing.T) {
	finalB64 := base64.StdEncoding.EncodeToString(testPNGBytes(t, 3, 2))
	finalJSON := fmt.Sprintf(`{"type":"image_edit.completed","b64_json":%q,"revised_prompt":"finished edit","usage":{
  "input_tokens": 907,
  "input_tokens_details": {
    "image_tokens": 884,
    "text_tokens": 23
  },
  "output_tokens": 386,
  "total_tokens": 1293
}}`, finalB64)
	tests := []struct {
		name        string
		body        string
		contentType string
	}{
		{name: "relay raw continuation lines", body: "data: {}\n\ndata: {}\n\nevent: image_edit.completed\ndata: " + finalJSON + "\n\n"},
		{name: "standard SSE data fields", body: "event: image_edit.completed\ndata: " + strings.ReplaceAll(finalJSON, "\n", "\ndata: ") + "\n\n"},
		{name: "nested object on its own data field", body: "data: " + strings.ReplaceAll(strings.Replace(finalJSON, `"usage":{`, "\"usage\":\n{", 1), "\n", "\ndata: ") + "\n\n"},
		{name: "final event at EOF", body: "data: " + finalJSON},
		{name: "malformed event before final", body: "data: {broken\n\nevent: image_edit.completed\ndata: " + finalJSON + "\n\n"},
		{name: "SSE labeled as JSON", contentType: "application/json", body: "data: {}\n\nevent: image_edit.completed\ndata: " + finalJSON + "\n\n"},
		{name: "SSE labeled as plain text", contentType: "text/plain", body: "event: image_edit.completed\ndata: " + finalJSON},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				contentType := tt.contentType
				if contentType == "" {
					contentType = "text/event-stream"
				}
				w.Header().Set("Content-Type", contentType)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer srv.Close()

			var raw bytes.Buffer
			result, err := RequestImagesAPIWithPartial(context.Background(), Options{
				APIKey: "sk-test", Prompt: "edit image", BaseURL: srv.URL, APIMode: APIModeImages,
			}, &raw, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.ImageB64 != finalB64 || result.RevisedPrompt != "finished edit" || result.SourceEvent != "images_api" {
				t.Fatalf("final image or metadata mismatch: source=%q revised=%q", result.SourceEvent, result.RevisedPrompt)
			}
			if requests != 1 || strings.TrimSpace(raw.String()) != strings.TrimSpace(tt.body) {
				t.Fatal("expected one request and the complete original response log")
			}
		})
	}
}

func TestRequestImagesAPITruncatedCompletedEventDoesNotReturnImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"image_edit.completed\",\"b64_json\":\"ZmluYWw=\",\"usage\":{\n  \"input_tokens\": 907\n")
	}))
	defer srv.Close()
	_, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey: "sk-test", Prompt: "edit image", BaseURL: srv.URL, APIMode: APIModeImages,
	}, &bytes.Buffer{}, nil, nil)
	if !errors.Is(err, ErrNoImageInResponse) {
		t.Fatalf("err=%v, want ErrNoImageInResponse", err)
	}
}
