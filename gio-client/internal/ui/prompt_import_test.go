package ui

import (
	"testing"

	sharedCompat "image-studio/shared/compat"
)

func TestNormalizeImportedPromptSizeRecognizesParitySizes(t *testing.T) {
	if got := normalizeImportedPromptSize("1792x1024", nil); got != "1792x1024" {
		t.Fatalf("normalizeImportedPromptSize(1792x1024)=%q want 1792x1024", got)
	}

	customRatios := []sharedCompat.CustomAspectRatio{{
		ID:     "wide-21-9",
		Label:  "21:9",
		Width:  21,
		Height: 9,
	}}
	customSize := buildCustomSizeSelection(customRatios[0], "2k")
	if got := normalizeImportedPromptSize(customSize, customRatios); got != customSize {
		t.Fatalf("normalizeImportedPromptSize(custom)=%q want %q", got, customSize)
	}

	if got := normalizeImportedPromptSize("999x999", customRatios); got != "auto" {
		t.Fatalf("normalizeImportedPromptSize(unknown)=%q want auto", got)
	}
}

func TestNormalizeImportedPromptSizeForAppRespectsCurrentModelCapabilities(t *testing.T) {
	app := &App{
		api:    "responses",
		policy: "openai",
	}
	app.imageModelInput.SetText("dall-e-3")
	if got := normalizeImportedPromptSizeForApp(app, "2048x1152"); got != "1024x1024" {
		t.Fatalf("normalizeImportedPromptSizeForApp(dalle3 widescreen)=%q want 1024x1024", got)
	}

	app.imageModelInput.SetText("gpt-image-1.5")
	if got := normalizeImportedPromptSizeForApp(app, "auto"); got != "auto" {
		t.Fatalf("normalizeImportedPromptSizeForApp(legacy auto)=%q want auto", got)
	}
}
