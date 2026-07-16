package ui

import (
	"strings"
	"testing"

	"github.com/yuanhua/image-gptcodex/pkg/client"
	sharedCompat "image-studio/shared/compat"
)

func TestVisibleResolutionChoicesDefaultsBlankImageModelToGPTImage2(t *testing.T) {
	choices := visibleResolutionChoices(string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "")
	if len(choices) != 4 {
		t.Fatalf("len(choices)=%d want 4", len(choices))
	}
	want := map[string]bool{
		"auto": true,
		"1k":   true,
		"2k":   true,
		"4k":   true,
	}
	for _, item := range choices {
		if !want[item.Value] {
			t.Fatalf("unexpected resolution choice %q", item.Value)
		}
		delete(want, item.Value)
	}
	if len(want) != 0 {
		t.Fatalf("missing resolution choices: %#v", want)
	}
}

func TestVisibleResolutionChoicesMatchesModelCapabilities(t *testing.T) {
	choices := visibleResolutionChoices(string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "dall-e-2")
	want := []string{"256", "512", "1k"}
	if len(choices) != len(want) {
		t.Fatalf("dalle2 choices=%v want %v", choices, want)
	}
	for idx, value := range want {
		if choices[idx].Value != value {
			t.Fatalf("dalle2 choice[%d]=%q want %q", idx, choices[idx].Value, value)
		}
	}

	choices = visibleResolutionChoices(string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "dall-e-3")
	if len(choices) != 1 || choices[0].Value != "1k" {
		t.Fatalf("dalle3 choices=%v want [1k]", choices)
	}

	choices = visibleResolutionChoices(string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "gpt-image-1.5")
	if len(choices) != 2 || choices[0].Value != "auto" || choices[1].Value != "1k" {
		t.Fatalf("legacy gpt-image choices=%v want [auto 1k]", choices)
	}
}

func TestVisibleAspectChoicesMatchesModelCapabilities(t *testing.T) {
	dalle3 := visibleAspectChoices(string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "dall-e-3", nil)
	if len(dalle3) != 3 {
		t.Fatalf("dalle3 aspect len=%d want 3", len(dalle3))
	}
	for _, choice := range dalle3 {
		if choice.Value == "auto" || choice.Value == "16:9" || choice.Value == "9:16" {
			t.Fatalf("unexpected dalle3 aspect %q", choice.Value)
		}
	}

	legacy := visibleAspectChoices(string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "gpt-image-1.5", nil)
	for _, choice := range legacy {
		if choice.Value == "16:9" || choice.Value == "9:16" || strings.HasPrefix(choice.Value, "custom:") {
			t.Fatalf("unexpected legacy aspect %q", choice.Value)
		}
	}

	custom := []sharedCompat.CustomAspectRatio{{ID: "wide", Label: "21:9", Width: 21, Height: 9}}
	compat := visibleAspectChoices(string(client.APIModeResponses), string(client.RequestPolicyCompat), "custom-relay-model", custom)
	foundCustom := false
	for _, choice := range compat {
		if choice.Value == "custom:wide" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Fatalf("expected custom aspect in compat relay choices: %v", compat)
	}
}

func TestBuildSizeSelectionUsesUpdatedDesktopParityMatrix(t *testing.T) {
	cases := []struct {
		aspect     string
		resolution string
		want       string
	}{
		{aspect: "7:4", resolution: "1k", want: "1792x1024"},
		{aspect: "4:7", resolution: "1k", want: "1024x1792"},
		{aspect: "3:2", resolution: "4k", want: "3520x2352"},
		{aspect: "2:3", resolution: "4k", want: "2352x3520"},
	}
	for _, tc := range cases {
		got := buildSizeSelection(tc.aspect, tc.resolution)
		want := tc.want
		if got != want {
			t.Fatalf("buildSizeSelection(%q, %q)=%q want %q", tc.aspect, tc.resolution, got, want)
		}
	}
}

func TestBuildCustomSizeSelectionUsesAreaBasedSizing(t *testing.T) {
	custom := sharedCompat.CustomAspectRatio{ID: "21:9", Label: "21:9", Width: 21, Height: 9}
	cases := []struct {
		resolution string
		want       string
	}{
		{resolution: "1k", want: "1536x656"},
		{resolution: "2k", want: "2048x880"},
		{resolution: "4k", want: "3840x1648"},
	}
	for _, tc := range cases {
		if got := buildCustomSizeSelection(custom, tc.resolution); got != tc.want {
			t.Fatalf("buildCustomSizeSelection(%q)=%q want %q", tc.resolution, got, tc.want)
		}
	}
}

func TestDeriveAspectPresetRecognizesNewBuiltInSizes(t *testing.T) {
	if got := deriveAspectPreset("1792x1024", nil); got != "7:4" {
		t.Fatalf("deriveAspectPreset(1792x1024)=%q want 7:4", got)
	}
	if got := deriveAspectPreset("1024x1792", nil); got != "4:7" {
		t.Fatalf("deriveAspectPreset(1024x1792)=%q want 4:7", got)
	}
	if got := deriveAspectPreset("3520x2352", nil); got != "3:2" {
		t.Fatalf("deriveAspectPreset(3520x2352)=%q want 3:2", got)
	}
	if got := deriveAspectPreset("2352x3520", nil); got != "2:3" {
		t.Fatalf("deriveAspectPreset(2352x3520)=%q want 2:3", got)
	}
}

func TestDeriveResolutionPresetRecognizesCustomSizes(t *testing.T) {
	custom := []sharedCompat.CustomAspectRatio{{ID: "wide", Label: "21:9", Width: 21, Height: 9}}
	if got := deriveResolutionPreset(buildCustomSizeSelection(custom[0], "2k"), custom); got != "2k" {
		t.Fatalf("deriveResolutionPreset(custom 2k)=%q want 2k", got)
	}
	if got := deriveResolutionPreset(buildCustomSizeSelection(custom[0], "4k"), custom); got != "4k" {
		t.Fatalf("deriveResolutionPreset(custom 4k)=%q want 4k", got)
	}
}

func TestNormalizeSizeSelectionMatchesCapabilities(t *testing.T) {
	if got := normalizeSizeSelection("auto", string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "dall-e-3", nil); got != "1024x1024" {
		t.Fatalf("normalizeSizeSelection(dalle3 auto)=%q want 1024x1024", got)
	}
	if got := normalizeSizeSelection("2048x1152", string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "dall-e-3", nil); got != "1024x1024" {
		t.Fatalf("normalizeSizeSelection(dalle3 widescreen)=%q want 1024x1024", got)
	}
	if got := normalizeSizeSelection("auto", string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "gpt-image-1.5", nil); got != "auto" {
		t.Fatalf("normalizeSizeSelection(legacy auto)=%q want auto", got)
	}
}

func TestNormalizeBatchAutoAspectResolutionMatchesCapabilities(t *testing.T) {
	if got := normalizeBatchAutoAspectResolution("", string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "dall-e-2"); got != "256" {
		t.Fatalf("normalizeBatchAutoAspectResolution(dalle2 empty)=%q want 256", got)
	}
	if got := normalizeBatchAutoAspectResolution("4k", string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "dall-e-3"); got != "1k" {
		t.Fatalf("normalizeBatchAutoAspectResolution(dalle3 4k)=%q want 1k", got)
	}
	if got := normalizeBatchAutoAspectResolution("2k", string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "gpt-image-2"); got != "2k" {
		t.Fatalf("normalizeBatchAutoAspectResolution(gpt-image-2 2k)=%q want 2k", got)
	}
}

func TestBuildReferenceResolutionSizeSelectionMatchesSupportLevel(t *testing.T) {
	custom := []sharedCompat.CustomAspectRatio{{ID: "21:9", Label: "21:9", Width: 21, Height: 9}}
	if got := buildReferenceResolutionSizeSelection(2100, 900, "1k", string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "gpt-image-2", custom); got != "1536x656" {
		t.Fatalf("buildReferenceResolutionSizeSelection(custom-supported)=%q want 1536x656", got)
	}
	if got := buildReferenceResolutionSizeSelection(2100, 900, "1k", string(client.APIModeResponses), string(client.RequestPolicyOpenAI), "dall-e-3", custom); got != "1792x1024" {
		t.Fatalf("buildReferenceResolutionSizeSelection(dalle3 fallback)=%q want 1792x1024", got)
	}
}
