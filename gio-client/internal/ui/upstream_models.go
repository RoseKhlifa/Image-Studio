package ui

import (
	"strings"

	"image-studio/gio-client/internal/kernel"
)

func uniqueProbeModels(input []kernel.UpstreamModelDescriptor) []kernel.UpstreamModelDescriptor {
	seen := map[string]struct{}{}
	out := make([]kernel.UpstreamModelDescriptor, 0, len(input))
	for _, item := range input {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
}

func probeModelLooksLikeImage(model kernel.UpstreamModelDescriptor) bool {
	haystack := strings.ToLower(strings.TrimSpace(model.ID + " " + model.DisplayName + " " + model.Object + " " + model.OwnedBy))
	return strings.Contains(haystack, "gpt-image") ||
		strings.Contains(haystack, "image-") ||
		strings.Contains(haystack, "images") ||
		strings.Contains(haystack, "dall-e") ||
		strings.Contains(haystack, "vision-image")
}

func probeModelLooksLikeText(model kernel.UpstreamModelDescriptor) bool {
	return !probeModelLooksLikeImage(model)
}

func probeModelLabel(model kernel.UpstreamModelDescriptor) string {
	display := strings.TrimSpace(model.DisplayName)
	id := strings.TrimSpace(model.ID)
	if display != "" && display != id {
		return display + " (" + id + ")"
	}
	return id
}

func preferredProbeModels(apiMode string, models []kernel.UpstreamModelDescriptor) (text []kernel.UpstreamModelDescriptor, image []kernel.UpstreamModelDescriptor) {
	all := uniqueProbeModels(models)
	for _, model := range all {
		if probeModelLooksLikeImage(model) {
			image = append(image, model)
		} else {
			text = append(text, model)
		}
	}
	if apiMode == "images" {
		if len(image) == 0 {
			image = all
		}
		return nil, image
	}
	if len(text) == 0 {
		text = all
	}
	if len(image) == 0 {
		image = all
	}
	return text, image
}
