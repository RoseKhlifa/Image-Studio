package client

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

var grokAspectRatios = []string{
	"1:1", "16:9", "9:16", "4:3", "3:4", "3:2", "2:3", "2:1", "1:2", "19.5:9", "9:19.5", "20:9", "9:20",
}

func buildGrokImagePayload(opts Options, paths []string, model, size string) ([]byte, error) {
	if strings.TrimSpace(opts.MaskB64) != "" {
		return nil, fmt.Errorf("Grok Imagine 不支持 OpenAI mask 参数；请清除蒙版后重试")
	}
	payload := map[string]any{
		"model":           model,
		"prompt":          opts.Prompt,
		"response_format": "b64_json",
	}
	if ratio, resolution := grokImageDimensions(size); ratio != "" {
		payload["aspect_ratio"] = ratio
		payload["resolution"] = resolution
	}
	if opts.Mode == ModeEdit {
		if len(paths) == 0 {
			return nil, fmt.Errorf("Grok Imagine 图生图需要至少一张源图")
		}
		images := make([]map[string]string, 0, len(paths))
		for _, path := range paths {
			dataURL, err := ImageFileToDataURL(path)
			if err != nil {
				return nil, err
			}
			images = append(images, map[string]string{
				"type": "image_url",
				"url":  dataURL,
			})
		}
		if len(images) == 1 {
			payload["image"] = images[0]
		} else {
			payload["image"] = images
		}
	}
	return json.Marshal(payload)
}

func grokImageDimensions(size string) (string, string) {
	width, height, ok := parseGoogleInteractionSize(size)
	if !ok {
		return "", ""
	}
	maxSide := width
	if height > maxSide {
		maxSide = height
	}
	resolution := "1k"
	if maxSide >= 1536 {
		resolution = "2k"
	}
	return closestGrokAspectRatio(float64(width) / float64(height)), resolution
}

func closestGrokAspectRatio(target float64) string {
	best := "1:1"
	bestDistance := math.Inf(1)
	for _, candidate := range grokAspectRatios {
		parts := strings.Split(candidate, ":")
		left, _ := parseAspectNumber(parts[0])
		right, _ := parseAspectNumber(parts[1])
		distance := math.Abs(math.Log((left / right) / target))
		if distance < bestDistance {
			best = candidate
			bestDistance = distance
		}
	}
	return best
}

func parseAspectNumber(value string) (float64, error) {
	var out float64
	_, err := fmt.Sscanf(value, "%f", &out)
	return out, err
}
