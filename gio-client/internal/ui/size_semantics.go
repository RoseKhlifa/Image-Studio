package ui

import (
	"fmt"
	"math"
	"strings"

	sharedCompat "image-studio/shared/compat"
)

const (
	openAIImageSizeAlignment  = 16
	minOpenAIImageSide        = 64
	maxOpenAIImageSide        = 3840
	maxOpenAIImagePixels      = 3840 * 2160
	maxOpenAIImageAspectRatio = 3.0
	minOpenAIImageAspectRatio = 1.0 / maxOpenAIImageAspectRatio
	customAspectTolerance     = 0.035
	builtinAspectTolerance    = 0.08
)

type sizeLimitConfig struct {
	maxSide   int
	maxPixels int
	maxAspect float64
	alignment int
}

type customResolutionReference struct {
	sizeLimitConfig
	area int
}

var builtinAspectDimensions = map[string]struct {
	width  int
	height int
}{
	"1:1":  {width: 1, height: 1},
	"3:2":  {width: 3, height: 2},
	"2:3":  {width: 2, height: 3},
	"16:9": {width: 16, height: 9},
	"9:16": {width: 9, height: 16},
	"7:4":  {width: 7, height: 4},
	"4:7":  {width: 4, height: 7},
}

var customResolutionReferences = map[string]customResolutionReference{
	"1k": {
		area: 1536 * 1024,
		sizeLimitConfig: sizeLimitConfig{
			maxSide:   1536,
			maxPixels: maxOpenAIImagePixels,
			maxAspect: maxOpenAIImageAspectRatio,
			alignment: openAIImageSizeAlignment,
		},
	},
	"2k": {
		area: 2048 * 1360,
		sizeLimitConfig: sizeLimitConfig{
			maxSide:   2048,
			maxPixels: maxOpenAIImagePixels,
			maxAspect: maxOpenAIImageAspectRatio,
			alignment: openAIImageSizeAlignment,
		},
	},
	"4k": {
		area: 3840 * 2160,
		sizeLimitConfig: sizeLimitConfig{
			maxSide:   3840,
			maxPixels: maxOpenAIImagePixels,
			maxAspect: maxOpenAIImageAspectRatio,
			alignment: openAIImageSizeAlignment,
		},
	},
}

func normalizeFlexibleCustomResolution(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "2k", "4k":
		return strings.ToLower(strings.TrimSpace(resolution))
	default:
		return "1k"
	}
}

func parseSizeSelectionValue(size string) (int, int, bool) {
	size = strings.TrimSpace(size)
	if size == "" {
		return 0, 0, false
	}
	var width, height int
	if _, err := fmt.Sscanf(size, "%dx%d", &width, &height); err != nil {
		return 0, 0, false
	}
	if width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func aspectRatioDistance(width int, height int, targetWidth int, targetHeight int) float64 {
	if width <= 0 || height <= 0 || targetWidth <= 0 || targetHeight <= 0 {
		return math.Inf(1)
	}
	left := float64(width) / float64(height)
	right := float64(targetWidth) / float64(targetHeight)
	return math.Abs(left-right) / right
}

func customSizeDistance(width int, height int, targetWidth float64, targetHeight float64) float64 {
	return math.Abs(float64(width)-targetWidth)/math.Max(targetWidth, 1) +
		math.Abs(float64(height)-targetHeight)/math.Max(targetHeight, 1)
}

func nearestBuiltinAspect(width int, height int) (string, float64) {
	bestValue := "1:1"
	bestDistance := math.Inf(1)
	for value, dims := range builtinAspectDimensions {
		distance := aspectRatioDistance(width, height, dims.width, dims.height)
		if distance < bestDistance {
			bestValue = value
			bestDistance = distance
		}
	}
	return bestValue, bestDistance
}

func nearestAllowedBuiltinAspect(width int, height int, imageModelID string) string {
	allowed := allowedBuiltinAspects(imageModelID)
	bestValue := "1:1"
	bestDistance := math.Inf(1)
	for value, dims := range builtinAspectDimensions {
		if !allowed[value] {
			continue
		}
		distance := aspectRatioDistance(width, height, dims.width, dims.height)
		if distance < bestDistance {
			bestValue = value
			bestDistance = distance
		}
	}
	return bestValue
}

func findMatchingCustomAspect(width int, height int, customRatios []sharedCompat.CustomAspectRatio) (sharedCompat.CustomAspectRatio, bool) {
	for _, ratio := range customRatios {
		if strings.TrimSpace(ratio.ID) == "" || ratio.Width <= 0 || ratio.Height <= 0 {
			continue
		}
		for _, resolution := range []string{"1k", "2k", "4k"} {
			expectedWidth, expectedHeight := buildCustomDimensionsForResolution(float64(ratio.Width)/float64(ratio.Height), resolution)
			if expectedWidth == width && expectedHeight == height {
				return ratio, true
			}
		}
	}
	bestIndex := -1
	bestDistance := math.Inf(1)
	for idx, ratio := range customRatios {
		if strings.TrimSpace(ratio.ID) == "" || ratio.Width <= 0 || ratio.Height <= 0 {
			continue
		}
		distance := aspectRatioDistance(width, height, ratio.Width, ratio.Height)
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = idx
		}
	}
	if bestIndex >= 0 && bestDistance <= customAspectTolerance {
		return customRatios[bestIndex], true
	}
	return sharedCompat.CustomAspectRatio{}, false
}

func normalizeCustomAspectRatioValue(aspect float64, maxAspect float64) float64 {
	if !isFinitePositive(aspect) {
		return 1
	}
	if maxAspect <= 1 {
		return aspect
	}
	minAspect := 1.0 / maxAspect
	if aspect > maxAspect {
		return maxAspect
	}
	if aspect < minAspect {
		return minAspect
	}
	return aspect
}

func buildCustomDimensionsForResolution(aspect float64, resolution string) (int, int) {
	reference, ok := customResolutionReferences[normalizeFlexibleCustomResolution(resolution)]
	if !ok {
		reference = customResolutionReferences["1k"]
	}
	aspect = normalizeCustomAspectRatioValue(aspect, reference.maxAspect)
	width := math.Sqrt(float64(reference.area) * aspect)
	height := math.Sqrt(float64(reference.area) / aspect)
	if exactWidth, exactHeight, ok := normalizeSizeWithinLimits(width, height, reference.sizeLimitConfig); ok {
		return exactWidth, exactHeight
	}
	return clampRoundedDimension(width, reference.alignment, reference.maxSide),
		clampRoundedDimension(height, reference.alignment, reference.maxSide)
}

func buildExactSizeValue(width int, height int) (string, bool) {
	exactWidth, exactHeight, ok := normalizeSizeWithinLimits(float64(width), float64(height), sizeLimitConfig{
		maxSide:   maxOpenAIImageSide,
		maxPixels: maxOpenAIImagePixels,
		maxAspect: maxOpenAIImageAspectRatio,
		alignment: openAIImageSizeAlignment,
	})
	if !ok {
		return "", false
	}
	return fmt.Sprintf("%dx%d", exactWidth, exactHeight), true
}

func normalizeSizeWithinLimits(width float64, height float64, limits sizeLimitConfig) (int, int, bool) {
	if !isFinitePositive(width) || !isFinitePositive(height) {
		return 0, 0, false
	}
	aspect := normalizeCustomAspectRatioValue(width/height, limits.maxAspect)
	targetWidth := width
	targetHeight := height
	if targetWidth/targetHeight != aspect {
		if targetWidth >= targetHeight {
			targetWidth = targetHeight * aspect
		} else {
			targetHeight = targetWidth / aspect
		}
	}
	if currentMax := math.Max(targetWidth, targetHeight); currentMax > float64(limits.maxSide) {
		scale := float64(limits.maxSide) / currentMax
		targetWidth *= scale
		targetHeight *= scale
	}
	if pixelCount := targetWidth * targetHeight; pixelCount > float64(limits.maxPixels) {
		scale := math.Sqrt(float64(limits.maxPixels) / pixelCount)
		targetWidth *= scale
		targetHeight *= scale
	}

	widthCandidates := alignedDimensionCandidates(targetWidth, minOpenAIImageSide, limits.maxSide, limits.alignment)
	heightCandidates := alignedDimensionCandidates(targetHeight, minOpenAIImageSide, limits.maxSide, limits.alignment)
	bestWidth, bestHeight := 0, 0
	bestDistance := math.Inf(1)
	bestAspectDistance := math.Inf(1)
	bestAreaDistance := math.Inf(1)
	for _, candidateWidth := range widthCandidates {
		for _, candidateHeight := range heightCandidates {
			if !sizeWithinLimits(candidateWidth, candidateHeight, limits) {
				continue
			}
			distance := customSizeDistance(candidateWidth, candidateHeight, targetWidth, targetHeight)
			aspectDistance := math.Abs((float64(candidateWidth) / float64(candidateHeight)) - (targetWidth / targetHeight))
			areaDistance := math.Abs(float64(candidateWidth*candidateHeight)-targetWidth*targetHeight) / math.Max(targetWidth*targetHeight, 1)
			if distance < bestDistance ||
				(distance == bestDistance && aspectDistance < bestAspectDistance) ||
				(distance == bestDistance && aspectDistance == bestAspectDistance && areaDistance < bestAreaDistance) {
				bestWidth = candidateWidth
				bestHeight = candidateHeight
				bestDistance = distance
				bestAspectDistance = aspectDistance
				bestAreaDistance = areaDistance
			}
		}
	}
	if bestWidth == 0 || bestHeight == 0 {
		return 0, 0, false
	}
	return bestWidth, bestHeight, true
}

func alignedDimensionCandidates(value float64, min int, max int, alignment int) []int {
	clamped := math.Max(float64(min), math.Min(float64(max), value))
	candidates := []int{
		int(math.Round(clamped/float64(alignment))) * alignment,
		int(math.Floor(clamped/float64(alignment))) * alignment,
		int(math.Ceil(clamped/float64(alignment))) * alignment,
	}
	seen := map[int]struct{}{}
	out := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate < min {
			candidate = min
		}
		if candidate > max {
			candidate = max
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	sortIntsByDistance(out, clamped)
	return out
}

func sortIntsByDistance(values []int, target float64) {
	for i := 0; i < len(values); i++ {
		for j := i + 1; j < len(values); j++ {
			left := math.Abs(float64(values[i]) - target)
			right := math.Abs(float64(values[j]) - target)
			if left > right || (left == right && values[i] < values[j]) {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}

func sizeWithinLimits(width int, height int, limits sizeLimitConfig) bool {
	if width < minOpenAIImageSide || height < minOpenAIImageSide {
		return false
	}
	if width > limits.maxSide || height > limits.maxSide {
		return false
	}
	if width*height > limits.maxPixels {
		return false
	}
	aspect := float64(width) / float64(height)
	return aspect <= limits.maxAspect && aspect >= (1.0/limits.maxAspect)
}

func clampRoundedDimension(value float64, alignment int, maxSide int) int {
	rounded := int(math.Round(value/float64(alignment))) * alignment
	if rounded < minOpenAIImageSide {
		return minOpenAIImageSide
	}
	if rounded > maxSide {
		return maxSide
	}
	return rounded
}

func isFinitePositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func supportsPreciseSizeControl(apiMode string, requestPolicy string, imageModelID string) bool {
	return supportsCustomAspectRatios(apiMode, requestPolicy, imageModelID)
}

func isExactSizeValue(size string, apiMode string, requestPolicy string, imageModelID string, customRatios []sharedCompat.CustomAspectRatio) bool {
	aspect := deriveAspectPreset(size, customRatios)
	resolution := deriveResolutionPreset(size, customRatios)
	return buildSupportedSizeSelection(aspect, resolution, apiMode, requestPolicy, imageModelID, customRatios) != strings.TrimSpace(size)
}

func referenceAspectRatio(width int, height int, customRatios []sharedCompat.CustomAspectRatio) (sharedCompat.CustomAspectRatio, bool) {
	if width <= 0 || height <= 0 {
		return sharedCompat.CustomAspectRatio{}, false
	}
	id := buildCustomAspectRatioID(width, height)
	if id == "" || isBuiltInAspectRatioID(id) {
		return sharedCompat.CustomAspectRatio{}, false
	}
	for _, ratio := range customRatios {
		if strings.TrimSpace(ratio.ID) == id {
			return ratio, true
		}
	}
	reducedWidth, reducedHeight := reduceCustomAspectRatio(width, height)
	if reducedWidth <= 0 || reducedHeight <= 0 {
		return sharedCompat.CustomAspectRatio{}, false
	}
	return sharedCompat.CustomAspectRatio{
		ID:        id,
		Label:     fmt.Sprintf("参考图 %d:%d", reducedWidth, reducedHeight),
		Width:     reducedWidth,
		Height:    reducedHeight,
		CreatedAt: 0,
	}, true
}

func buildReferenceResolutionSizeSelection(width int, height int, resolution string, apiMode string, requestPolicy string, imageModelID string, customRatios []sharedCompat.CustomAspectRatio) string {
	resolution = normalizeBatchAutoAspectResolution(resolution, apiMode, requestPolicy, imageModelID)
	if width <= 0 || height <= 0 {
		return buildResolutionSizeSelection("auto", resolution, apiMode, requestPolicy, imageModelID, customRatios)
	}
	if supportsCustomAspectRatios(apiMode, requestPolicy, imageModelID) {
		if ratio, ok := referenceAspectRatio(width, height, customRatios); ok {
			return buildCustomSizeSelection(ratio, resolution)
		}
	}
	nearestAspect := nearestAllowedBuiltinAspect(width, height, imageModelID)
	return buildSupportedSizeSelection(nearestAspect, resolution, apiMode, requestPolicy, imageModelID, customRatios)
}

func exactSizeLabel(size string, apiMode string, requestPolicy string, imageModelID string, customRatios []sharedCompat.CustomAspectRatio) string {
	size = strings.TrimSpace(size)
	if size == "" || size == "auto" {
		return ""
	}
	if !supportsPreciseSizeControl(apiMode, requestPolicy, imageModelID) {
		return ""
	}
	if !isExactSizeValue(size, apiMode, requestPolicy, imageModelID, customRatios) {
		return ""
	}
	width, height, ok := parseSizeSelectionValue(size)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%dx%d", width, height)
}
