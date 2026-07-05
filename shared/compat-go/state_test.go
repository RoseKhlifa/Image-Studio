package compat

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsEmptyState(t *testing.T) {
	state, err := Load(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("schema=%d want %d", state.SchemaVersion, SchemaVersion)
	}
	if state.UpdatedAt != 0 {
		t.Fatalf("updatedAt=%d want 0", state.UpdatedAt)
	}
	if state.Profiles == nil || state.History == nil {
		t.Fatalf("expected initialized slices: %#v", state)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := StatePath(t.TempDir())
	input := State{
		Client:    "test",
		UpdatedAt: 123,
		Settings: Settings{
			ProxyMode:                 "custom",
			ProxyURL:                  "http://127.0.0.1:7890",
			Theme:                     "dark",
			OutputFormat:              "webp",
			OutputDir:                 "/tmp/images",
			PromptHistory:             []string{"cat"},
			ReducedEffects:            true,
			SavePromptSuppressed:      true,
			KeepLogs:                  true,
			CleanupPreviewCacheOnExit: true,
			IgnoredReleaseTag:         "1.1.6",
			AutoRetryEnabled:          func() *bool { v := false; return &v }(),
			AutoRetryCount:            func() *int { v := 8; return &v }(),
			CompletionSound: &CompletionSoundSettings{
				Enabled:    true,
				Mode:       "custom",
				CustomName: "ding.wav",
				CustomData: "data:audio/wav;base64,AAAA",
			},
			CompletionNotification: &CompletionNotificationSettings{
				Enabled: false,
			},
			AdvancedFloatingPanel: &AdvancedFloatingPanelPrefs{
				X: func() *int { v := 880; return &v }(),
				Y: func() *int { v := 108; return &v }(),
				Groups: map[string]bool{
					"core":     true,
					"output":   true,
					"strategy": false,
					"stream":   true,
				},
			},
			Presets: []Preset{{
				ID:                "preset-1",
				Name:              "配置1",
				Size:              "1536x1024",
				Quality:           "high",
				OutputFormat:      "webp",
				NegativePrompt:    "no watermark",
				Background:        "transparent",
				InputFidelity:     "high",
				ImageStyle:        "vivid",
				Moderation:        "auto",
				StyleTag:          "anime",
				KernelRuntimeMode: "remote",
				BatchCount:        4,
			}},
			CustomAspectRatios: []CustomAspectRatio{{
				ID:        "5:4",
				Label:     "5:4",
				Width:     5,
				Height:    4,
				CreatedAt: 111,
			}},
		},
		Profiles: []UpstreamProfile{{
			ID:              "p1",
			Name:            "配置1",
			APIMode:         "responses",
			RequestPolicy:   "openai",
			BaseURL:         "https://upstream.example",
			TextModelID:     "gpt-5.5",
			ImageModelID:    "gpt-image-2",
			ReasoningEffort: "xhigh",
			CreatedAt:       100,
		}},
		ActiveProfile: "p1",
		History: []HistoryItem{{
			ID:           "h1",
			Prompt:       "cat",
			Mode:         "generate",
			Size:         "1024x1024",
			Quality:      "high",
			OutputFormat: "png",
			CreatedAt:    200,
			SourcePaths:  []string{"/tmp/a.png", "/tmp/b.png"},
			SavedPath:    "/tmp/images/cat.png",
		}},
		HistoryFull: []HistoryFullItem{{ID: "h1", ImageB64: "aW1n"}},
	}
	if err := Save(path, input); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Client != "test" || loaded.ActiveProfile != "p1" || loaded.Settings.OutputDir != "/tmp/images" || !loaded.Settings.ReducedEffects || !loaded.Settings.SavePromptSuppressed || !loaded.Settings.KeepLogs || !loaded.Settings.CleanupPreviewCacheOnExit || loaded.Settings.IgnoredReleaseTag != "1.1.6" {
		t.Fatalf("unexpected state: %#v", loaded)
	}
	if loaded.Settings.AutoRetryEnabled == nil || *loaded.Settings.AutoRetryEnabled != false || loaded.Settings.AutoRetryCount == nil || *loaded.Settings.AutoRetryCount != 8 {
		t.Fatalf("auto retry not preserved: enabled=%v count=%v", loaded.Settings.AutoRetryEnabled, loaded.Settings.AutoRetryCount)
	}
	if loaded.Settings.CompletionSound == nil || !loaded.Settings.CompletionSound.Enabled || loaded.Settings.CompletionSound.Mode != "custom" || loaded.Settings.CompletionSound.CustomName != "ding.wav" || loaded.Settings.CompletionSound.CustomData != "data:audio/wav;base64,AAAA" {
		t.Fatalf("completion sound not preserved: %#v", loaded.Settings.CompletionSound)
	}
	if loaded.Settings.CompletionNotification == nil || loaded.Settings.CompletionNotification.Enabled {
		t.Fatalf("completion notification not preserved: %#v", loaded.Settings.CompletionNotification)
	}
	if loaded.Settings.AdvancedFloatingPanel == nil || loaded.Settings.AdvancedFloatingPanel.X == nil || *loaded.Settings.AdvancedFloatingPanel.X != 880 || loaded.Settings.AdvancedFloatingPanel.Y == nil || *loaded.Settings.AdvancedFloatingPanel.Y != 108 || !loaded.Settings.AdvancedFloatingPanel.Groups["output"] || !loaded.Settings.AdvancedFloatingPanel.Groups["stream"] {
		t.Fatalf("advanced floating panel prefs not preserved: %#v", loaded.Settings.AdvancedFloatingPanel)
	}
	if len(loaded.Settings.Presets) != 1 {
		t.Fatalf("presets not preserved: %#v", loaded.Settings.Presets)
	}
	if len(loaded.Settings.CustomAspectRatios) != 1 || loaded.Settings.CustomAspectRatios[0].ID != "5:4" || loaded.Settings.CustomAspectRatios[0].Width != 5 || loaded.Settings.CustomAspectRatios[0].Height != 4 {
		t.Fatalf("custom aspect ratios not preserved: %#v", loaded.Settings.CustomAspectRatios)
	}
	preset := loaded.Settings.Presets[0]
	if preset.StyleTag != "anime" || preset.Background != "transparent" || preset.InputFidelity != "high" || preset.ImageStyle != "vivid" || preset.Moderation != "auto" || preset.KernelRuntimeMode != "remote" || preset.BatchCount != 4 {
		t.Fatalf("preset fields not preserved: %#v", preset)
	}
	if len(loaded.Profiles) != 1 || loaded.Profiles[0].BaseURL != "https://upstream.example" || loaded.Profiles[0].ReasoningEffort != "xhigh" {
		t.Fatalf("profiles not preserved: %#v", loaded.Profiles)
	}
	if len(loaded.History) != 1 || loaded.History[0].SavedPath != "/tmp/images/cat.png" {
		t.Fatalf("history not preserved: %#v", loaded.History)
	}
	if len(loaded.History[0].SourcePaths) != 2 || loaded.History[0].SourcePaths[0] != "/tmp/a.png" || loaded.History[0].SourcePaths[1] != "/tmp/b.png" {
		t.Fatalf("sourcePaths not preserved: %#v", loaded.History[0].SourcePaths)
	}
	if len(loaded.HistoryFull) != 1 || loaded.HistoryFull[0].ImageB64 != "aW1n" {
		t.Fatalf("historyFull not preserved: %#v", loaded.HistoryFull)
	}
}

func TestLoadInvalidJSONReturnsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := Load(path); err == nil || errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected parse error, got %v", err)
	}
}
