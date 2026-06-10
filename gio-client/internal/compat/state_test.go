package compat

import (
	"path/filepath"
	"testing"

	"image-studio/gio-client/internal/kernel"
	shared "image-studio/shared/compat"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

func TestConfigFromStateUsesActiveProfileAndSettings(t *testing.T) {
	state := shared.State{
		Settings: shared.Settings{
			OutputDir:    "/tmp/out",
			OutputFormat: "webp",
			ProxyMode:    client.ProxyModeCustom,
			ProxyURL:     "http://127.0.0.1:7890",
			CompletionSound: &shared.CompletionSoundSettings{
				Enabled:    false,
				Mode:       "custom",
				CustomName: "ding.wav",
				CustomData: "data:audio/wav;base64,AA==",
			},
		},
		Profiles: []shared.UpstreamProfile{
			{ID: "p1", Name: "配置1", BaseURL: "https://old.example", TextModelID: "old-text", ImageModelID: "old-image", APIMode: string(client.APIModeImages)},
			{ID: "p2", Name: "配置2", BaseURL: "https://new.example", TextModelID: "new-text", ImageModelID: "new-image", APIMode: string(client.APIModeResponses), ResponsesTransport: string(client.ResponsesTransportWebSocket), RequestPolicy: string(client.RequestPolicyCompat), ReasoningEffort: "high"},
		},
		ActiveProfile: "p2",
	}
	cfg := ConfigFromState(kernel.DefaultConfig(), state)
	if cfg.OutputDir != "/tmp/out" || cfg.OutputFormat != "webp" {
		t.Fatalf("settings not applied: %#v", cfg)
	}
	if cfg.BaseURL != "https://new.example" || cfg.TextModelID != "new-text" || cfg.ImageModelID != "new-image" {
		t.Fatalf("profile not applied: %#v", cfg)
	}
	if cfg.APIMode != client.APIModeResponses || cfg.RequestPolicy != client.RequestPolicyCompat {
		t.Fatalf("api settings not applied: %#v", cfg)
	}
	if cfg.ResponsesTransport != client.ResponsesTransportWebSocket || cfg.ReasoningEffort != "high" {
		t.Fatalf("responses config not applied: transport=%q reasoning=%q", cfg.ResponsesTransport, cfg.ReasoningEffort)
	}
	if cfg.ProxyMode != client.ProxyModeCustom || cfg.ProxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("proxy not applied: %#v", cfg)
	}
	if cfg.CompletionSound.Mode != "custom" || cfg.CompletionSound.CustomName != "ding.wav" || cfg.CompletionSound.CustomData != "data:audio/wav;base64,AA==" || cfg.CompletionSound.Enabled {
		t.Fatalf("completion sound not applied: %#v", cfg.CompletionSound)
	}
}

func TestUpsertConfigPreservesActiveProfileIdentity(t *testing.T) {
	state := shared.State{
		Profiles: []shared.UpstreamProfile{{
			ID:                 "p1",
			Name:               "主配置",
			APIMode:            string(client.APIModeImages),
			RequestPolicy:      string(client.RequestPolicyOpenAI),
			BaseURL:            "https://old.example",
			TextModelID:        "old-text",
			ImageModelID:       "old-image",
			ResponsesTransport: string(client.ResponsesTransportSSE),
			ReasoningEffort:    "xhigh",
			ConcurrencyLimit:   3,
			FallbackProfileID:  "backup-1",
			CreatedAt:          100,
		}},
		ActiveProfile: "p1",
	}
	cfg := kernel.Config{
		BaseURL:            "https://new.example",
		TextModelID:        "new-text",
		ImageModelID:       "new-image",
		APIMode:            client.APIModeResponses,
		ResponsesTransport: client.ResponsesTransportWebSocket,
		RequestPolicy:      client.RequestPolicyCompat,
		ReasoningEffort:    "medium",
		OutputFormat:       "jpeg",
		OutputDir:          "/tmp/images",
		ProxyMode:          client.ProxyModeNone,
		CompletionSound: shared.CompletionSoundSettings{
			Enabled:    true,
			Mode:       "custom",
			CustomName: "done.wav",
			CustomData: "data:audio/wav;base64,BB==",
		},
	}
	next := UpsertConfig(state, cfg)
	if next.ActiveProfile != "p1" || len(next.Profiles) != 1 {
		t.Fatalf("unexpected profiles: %#v", next.Profiles)
	}
	profile := next.Profiles[0]
	if profile.Name != "主配置" || profile.CreatedAt != 100 || profile.ConcurrencyLimit != 3 {
		t.Fatalf("profile identity fields changed: %#v", profile)
	}
	if profile.BaseURL != "https://new.example" || profile.APIMode != string(client.APIModeResponses) || profile.RequestPolicy != string(client.RequestPolicyCompat) {
		t.Fatalf("profile config not updated: %#v", profile)
	}
	if profile.ResponsesTransport != string(client.ResponsesTransportWebSocket) || profile.ReasoningEffort != "medium" || profile.FallbackProfileID != "backup-1" {
		t.Fatalf("extended profile config not updated: %#v", profile)
	}
	if next.Settings.OutputFormat != "jpeg" || next.Settings.OutputDir != "/tmp/images" || next.Settings.ProxyMode != client.ProxyModeNone {
		t.Fatalf("settings not updated: %#v", next.Settings)
	}
	if len(next.Settings.TrustedOutputRoots) != 1 || next.Settings.TrustedOutputRoots[0] != filepath.Clean("/tmp/images") {
		t.Fatalf("trusted output roots not updated: %#v", next.Settings.TrustedOutputRoots)
	}
	if next.Settings.CompletionSound == nil || !next.Settings.CompletionSound.Enabled || next.Settings.CompletionSound.Mode != "custom" || next.Settings.CompletionSound.CustomName != "done.wav" || next.Settings.CompletionSound.CustomData != "data:audio/wav;base64,BB==" {
		t.Fatalf("completion sound not updated: %#v", next.Settings.CompletionSound)
	}
	if next.Settings.Theme != "system" || next.Settings.FontScale != 1 {
		t.Fatalf("default visual settings not set: %#v", next.Settings)
	}
}

func TestHistoryItemFromRunUsesWebViewCompatibleFields(t *testing.T) {
	item := HistoryItemFromRun(kernel.Config{
		Prompt:         "cat",
		Mode:           client.ModeEdit,
		Size:           "1024x1536",
		Quality:        "high",
		OutputFormat:   "png",
		Seed:           42,
		NegativePrompt: "blur",
		SourcePaths:    []string{"/tmp/images/src-a.png", "/tmp/images/src-b.png"},
	}, kernel.Result{
		SavedPath:     "/tmp/images/cat.png",
		PreviewPath:   "/tmp/images/previews/cat.png",
		ThumbPath:     "/tmp/images/thumbs/cat.png",
		RawPath:       "/tmp/log/raw.txt",
		RevisedPrompt: "cat revised",
	}, 1.25)
	if item.ID == "" || item.CreatedAt == 0 {
		t.Fatalf("missing identity fields: %#v", item)
	}
	if item.Prompt != "cat" || item.Mode != string(client.ModeEdit) || item.SavedPath != "/tmp/images/cat.png" || item.PreviewPath != "/tmp/images/previews/cat.png" || item.ThumbPath != "/tmp/images/thumbs/cat.png" || item.RawPath != "/tmp/log/raw.txt" {
		t.Fatalf("history item not mapped: %#v", item)
	}
	if !item.PreviewOnly || item.ElapsedSec != 1.25 || item.Seed != 42 || item.NegativePrompt != "blur" {
		t.Fatalf("history metadata not mapped: %#v", item)
	}
	if len(item.SourcePaths) != 2 || item.SourcePaths[0] != "/tmp/images/src-a.png" || item.SourcePaths[1] != "/tmp/images/src-b.png" {
		t.Fatalf("source paths not mapped: %#v", item.SourcePaths)
	}
}

func TestRememberTrustedOutputRootDeduplicatesAndNormalizes(t *testing.T) {
	state := RememberTrustedOutputRoot(shared.State{}, "./tmp/out")
	if len(state.Settings.TrustedOutputRoots) != 1 {
		t.Fatalf("trusted output roots len=%d want 1", len(state.Settings.TrustedOutputRoots))
	}
	first := state.Settings.TrustedOutputRoots[0]
	state = RememberTrustedOutputRoot(state, first)
	if len(state.Settings.TrustedOutputRoots) != 1 {
		t.Fatalf("trusted output roots duplicated: %#v", state.Settings.TrustedOutputRoots)
	}
}
