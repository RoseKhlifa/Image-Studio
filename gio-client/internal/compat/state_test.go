package compat

import (
	"fmt"
	"path/filepath"
	"sync"
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
		Prompt:           "cat",
		Mode:             client.ModeEdit,
		Size:             "1024x1536",
		Quality:          "high",
		OutputFormat:     "png",
		Seed:             42,
		NegativePrompt:   "blur",
		BatchIndex:       2,
		PreviewSlotIndex: 1,
		ParentID:         "/tmp/images/src-a.png",
		SourcePaths:      []string{"/tmp/images/src-a.png", "/tmp/images/src-b.png"},
	}, kernel.Result{
		SavedPath:     "/tmp/images/cat.png",
		PreviewPath:   "/tmp/images/previews/cat.png",
		ThumbPath:     "/tmp/images/thumbs/cat.png",
		RawPath:       "/tmp/log/raw.txt",
		RevisedPrompt: "cat revised",
	}, 1.25, false)
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
	if item.ParentID != "/tmp/images/src-a.png" {
		t.Fatalf("parent id = %q want /tmp/images/src-a.png", item.ParentID)
	}
	if item.BatchIndex != 2 || item.PreviewSlotIndex != 1 {
		t.Fatalf("batch metadata not mapped: batch=%d previewSlot=%d", item.BatchIndex, item.PreviewSlotIndex)
	}
}

func TestHistoryItemFromRunPreviewOnlyRemoteKeepsRawAndDefersSavedPaths(t *testing.T) {
	item := HistoryItemFromRun(kernel.Config{
		Prompt:       "cat",
		Mode:         client.ModeGenerate,
		OutputFormat: "png",
	}, kernel.Result{
		SavedPath:     "/tmp/images/cat.png",
		PreviewPath:   "/tmp/images/previews/cat.png",
		ThumbPath:     "/tmp/images/thumbs/cat.png",
		RawPath:       "/tmp/log/raw.txt",
		RevisedPrompt: "cat revised",
		ImageB64:      "aW1n",
	}, 1.25, true)
	if item.SavedPath != "/tmp/images/cat.png" || item.PreviewPath != "" || item.ThumbPath != "" {
		t.Fatalf("preview-only item should keep virtual/local savedPath but defer preview/thumb paths: %#v", item)
	}
	if item.RawPath != "/tmp/log/raw.txt" || item.RevisedPrompt != "cat revised" || !item.PreviewOnly {
		t.Fatalf("preview-only metadata not preserved: %#v", item)
	}
}

func TestSaveConfigAndHistoryWithPreviewModeStoresHistoryFullAndHydratesImageB64(t *testing.T) {
	root := t.TempDir()
	origStable := StableDataRootForTest()
	SetStableDataRootForTest(func() (string, error) { return root, nil })
	defer SetStableDataRootForTest(origStable)

	cfg := kernel.Config{
		Prompt:       "cat",
		Mode:         client.ModeGenerate,
		OutputFormat: "png",
		OutputDir:    filepath.Join(root, "images"),
	}
	result := kernel.Result{
		RawPath:       "/tmp/log/raw.txt",
		RevisedPrompt: "cat revised",
		ImageB64:      "aW1n",
	}
	if err := SaveConfigAndHistoryWithPreviewMode(cfg, result, 1.5, true); err != nil {
		t.Fatalf("SaveConfigAndHistoryWithPreviewMode: %v", err)
	}
	state, _, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(state.History) != 1 {
		t.Fatalf("history len=%d want 1", len(state.History))
	}
	if got := state.History[0].ImageB64; got != "aW1n" {
		t.Fatalf("history imageB64=%q want aW1n", got)
	}
	if got := len(state.HistoryFull); got != 1 {
		t.Fatalf("historyFull len=%d want 1", got)
	}
	if state.HistoryFull[0].ImageB64 != "aW1n" {
		t.Fatalf("historyFull image=%q want aW1n", state.HistoryFull[0].ImageB64)
	}
}

func TestSaveConfigAndHistorySerializesConcurrentHistoryUpdates(t *testing.T) {
	root := t.TempDir()
	origStable := StableDataRootForTest()
	SetStableDataRootForTest(func() (string, error) { return root, nil })
	defer SetStableDataRootForTest(origStable)

	const total = 24
	errCh := make(chan error, total)
	var wg sync.WaitGroup
	for index := 0; index < total; index++ {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			prompt := fmt.Sprintf("prompt-%02d", index)
			errCh <- SaveConfigAndHistoryWithPreviewMode(kernel.Config{
				Prompt:       prompt,
				Mode:         client.ModeGenerate,
				OutputFormat: "png",
				OutputDir:    filepath.Join(root, "images"),
			}, kernel.Result{
				SavedPath: filepath.Join(root, "images", fmt.Sprintf("result-%02d.png", index)),
			}, 1, false)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent history save: %v", err)
		}
	}

	state, _, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if len(state.History) != total {
		t.Fatalf("history len=%d want %d", len(state.History), total)
	}
	seen := make(map[string]bool, total)
	for _, item := range state.History {
		seen[item.Prompt] = true
	}
	for index := 0; index < total; index++ {
		prompt := fmt.Sprintf("prompt-%02d", index)
		if !seen[prompt] {
			t.Fatalf("missing concurrent history item %q", prompt)
		}
	}
}

func TestSaveStatePrunesHistoryFullForNonPreviewOnlyHistory(t *testing.T) {
	root := t.TempDir()
	origStable := StableDataRootForTest()
	SetStableDataRootForTest(func() (string, error) { return root, nil })
	defer SetStableDataRootForTest(origStable)

	state := shared.State{
		History: []shared.HistoryItem{
			{ID: "saved-1", SavedPath: "/tmp/saved-1.png"},
			{ID: "preview-1", PreviewOnly: true},
		},
		HistoryFull: []shared.HistoryFullItem{
			{ID: "saved-1", ImageB64: "b2xk"},
			{ID: "preview-1", ImageB64: "bmV3"},
		},
	}
	if err := SaveState(state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	loaded, _, err := LoadState()
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if got := len(loaded.HistoryFull); got != 1 {
		t.Fatalf("historyFull len=%d want 1", got)
	}
	if loaded.HistoryFull[0].ID != "preview-1" || loaded.HistoryFull[0].ImageB64 != "bmV3" {
		t.Fatalf("unexpected historyFull entry: %#v", loaded.HistoryFull[0])
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
