package ui

import (
	"testing"

	"image-studio/gio-client/internal/kernel"
)

func TestPreferredProbeModelsSplitsTextAndImage(t *testing.T) {
	text, image := preferredProbeModels("responses", []kernel.UpstreamModelDescriptor{
		{ID: "gpt-5.5"},
		{ID: "gpt-image-2"},
		{ID: "gpt-5.5"},
	})
	if len(text) != 1 || text[0].ID != "gpt-5.5" {
		t.Fatalf("text=%v want [gpt-5.5]", text)
	}
	if len(image) != 1 || image[0].ID != "gpt-image-2" {
		t.Fatalf("image=%v want [gpt-image-2]", image)
	}
}

func TestPreferredProbeModelsFallsBackToAllForImagesMode(t *testing.T) {
	text, image := preferredProbeModels("images", []kernel.UpstreamModelDescriptor{
		{ID: "custom-model"},
	})
	if text != nil {
		t.Fatalf("text=%v want nil for images mode", text)
	}
	if len(image) != 1 || image[0].ID != "custom-model" {
		t.Fatalf("image=%v want [custom-model]", image)
	}
}
