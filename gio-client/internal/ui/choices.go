package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/yuanhua/image-gptcodex/pkg/client"
	sharedCompat "image-studio/shared/compat"
)

type choice struct {
	Label string
	Value string
}

type aspectChoice struct {
	Value  string
	Label  string
	W      int
	H      int
	Auto   bool
	Custom bool
}

var (
	modeChoices = []choice{
		{"📝 文生图", string(client.ModeGenerate)},
		{"🖼 图生图", string(client.ModeEdit)},
	}
	apiChoices = []choice{
		{"Responses", string(client.APIModeResponses)},
		{"Images", string(client.APIModeImages)},
	}
	sizeChoices = []choice{
		{"自适应 auto", "auto"},
		{"方形 256x256", "256x256"},
		{"方形 512x512", "512x512"},
		{"方形 1024x1024", "1024x1024"},
		{"横版 1536x1024", "1536x1024"},
		{"竖版 1024x1536", "1024x1536"},
		{"横版 1536x864", "1536x864"},
		{"竖版 864x1536", "864x1536"},
		{"宽画幅 1792x1024", "1792x1024"},
		{"高画幅 1024x1792", "1024x1792"},
		{"2K 方形 2048x2048", "2048x2048"},
		{"2K 横版 2048x1360", "2048x1360"},
		{"2K 竖版 1360x2048", "1360x2048"},
		{"2K 横版 2048x1152", "2048x1152"},
		{"2K 竖版 1152x2048", "1152x2048"},
		{"4K 方形 2880x2880", "2880x2880"},
		{"4K 横版 3520x2352", "3520x2352"},
		{"4K 竖版 2352x3520", "2352x3520"},
		{"4K 横版 3840x2160", "3840x2160"},
		{"4K 竖版 2160x3840", "2160x3840"},
	}
	aspectChoices = []aspectChoice{
		{Value: "auto", Label: "Auto", W: 18, H: 18, Auto: true},
		{Value: "1:1", Label: "1:1", W: 18, H: 18},
		{Value: "3:2", Label: "3:2", W: 22, H: 14},
		{Value: "2:3", Label: "2:3", W: 14, H: 20},
		{Value: "16:9", Label: "16:9", W: 24, H: 13},
		{Value: "9:16", Label: "9:16", W: 12, H: 22},
		{Value: "7:4", Label: "7:4", W: 24, H: 14},
		{Value: "4:7", Label: "4:7", W: 14, H: 24},
	}
	resolutionChoices = []choice{
		{"自动", "auto"},
		{"256", "256"},
		{"512", "512"},
		{"1K", "1k"},
		{"2K", "2k"},
		{"4K", "4k"},
	}
	qualityChoices = []choice{
		{"自动", "auto"},
		{"快速", "low"},
		{"标准", "medium"},
		{"精修", "high"},
	}
	formatChoices = []choice{
		{"PNG", "png"},
		{"JPEG", "jpeg"},
		{"WebP", "webp"},
	}
	policyChoices = []choice{
		{"OpenAI 标准", string(client.RequestPolicyOpenAI)},
		{"兼容中转扩展", string(client.RequestPolicyCompat)},
	}
	responsesTransportChoices = []settingsOptionChoice{
		{Title: "HTTP SSE", Detail: "默认，兼容性更稳", Value: string(client.ResponsesTransportSSE)},
		{Title: "WebSocket mode", Detail: "需要上游支持 Upgrade: websocket", Value: string(client.ResponsesTransportWebSocket)},
	}
	reasoningEffortChoices = []settingsOptionChoice{
		{Title: "low", Detail: "更快返回", Value: "low"},
		{Title: "medium", Detail: "平衡速度与质量", Value: "medium"},
		{Title: "high", Detail: "更认真推理", Value: "high"},
		{Title: "xhigh", Detail: "默认，长推理更稳", Value: "xhigh"},
	}
	proxyChoices = []choice{
		{"系统配置", client.ProxyModeSystem},
		{"不使用", client.ProxyModeNone},
		{"自定义", client.ProxyModeCustom},
	}
	backgroundChoices = []choice{
		{"自动", "auto"},
		{"实心", "opaque"},
		{"透明", "transparent"},
	}
	inputFidelityChoices = []choice{
		{"自动", "auto"},
		{"低", "low"},
		{"高", "high"},
	}
	imageStyleChoices = []choice{
		{"默认", "default"},
		{"鲜明", "vivid"},
		{"自然", "natural"},
	}
	moderationChoices = []choice{
		{"低", "low"},
		{"自动", "auto"},
	}
	partialPreviewChoices = []choice{
		{"仅最终图", "0"},
		{"1 帧", "1"},
		{"2 帧", "2"},
		{"3 帧", "3"},
	}
	styleChoices = []choice{
		{"赛博朋克", "cyberpunk"},
		{"二次元", "anime"},
		{"插画", "illust"},
		{"3D 渲染", "3d"},
		{"国风", "chinese"},
	}
	batchCountChoices = []choice{
		{"1", "1"},
		{"2", "2"},
		{"4", "4"},
		{"6", "6"},
		{"8", "8"},
		{"9", "9"},
	}
	styleSuffixes = map[string]string{
		"cyberpunk": "cyberpunk style, neon lights, glowing reflections, futuristic",
		"anime":     "anime style, cel shading, vibrant colors, detailed illustration",
		"illust":    "modern illustration, flat colors, clean lines",
		"3d":        "3D render, octane render, ray tracing, glossy surfaces, studio lighting",
		"chinese":   "traditional Chinese painting style, ink wash, misty landscape",
	}
	sizeMatrix = map[string]map[string]string{
		"1:1": {
			"256": "256x256",
			"512": "512x512",
			"1k":  "1024x1024",
			"2k":  "2048x2048",
			"4k":  "2880x2880",
		},
		"3:2": {
			"1k": "1536x1024",
			"2k": "2048x1360",
			"4k": "3520x2352",
		},
		"2:3": {
			"1k": "1024x1536",
			"2k": "1360x2048",
			"4k": "2352x3520",
		},
		"16:9": {
			"1k": "1536x864",
			"2k": "2048x1152",
			"4k": "3840x2160",
		},
		"9:16": {
			"1k": "864x1536",
			"2k": "1152x2048",
			"4k": "2160x3840",
		},
		"7:4": {
			"1k": "1792x1024",
		},
		"4:7": {
			"1k": "1024x1792",
		},
	}
	sizeToAspect = map[string]string{
		"auto":      "auto",
		"256x256":   "1:1",
		"512x512":   "1:1",
		"1024x1024": "1:1",
		"2048x2048": "1:1",
		"2880x2880": "1:1",
		"1536x1024": "3:2",
		"2048x1360": "3:2",
		"3520x2352": "3:2",
		"1024x1536": "2:3",
		"1360x2048": "2:3",
		"2352x3520": "2:3",
		"1536x864":  "16:9",
		"2048x1152": "16:9",
		"3840x2160": "16:9",
		"864x1536":  "9:16",
		"1152x2048": "9:16",
		"2160x3840": "9:16",
		"1792x1024": "7:4",
		"1024x1792": "4:7",
	}
	sizeToResolution = map[string]string{
		"auto":      "auto",
		"256x256":   "256",
		"512x512":   "512",
		"1024x1024": "1k",
		"1536x1024": "1k",
		"1024x1536": "1k",
		"1536x864":  "1k",
		"864x1536":  "1k",
		"1792x1024": "1k",
		"1024x1792": "1k",
		"2048x2048": "2k",
		"2048x1360": "2k",
		"1360x2048": "2k",
		"2048x1152": "2k",
		"1152x2048": "2k",
		"2880x2880": "4k",
		"3520x2352": "4k",
		"2352x3520": "4k",
		"3840x2160": "4k",
		"2160x3840": "4k",
	}
)

func (a *App) modeLabel() string {
	if a.mode == string(client.ModeEdit) {
		return "图生图"
	}
	return "文生图"
}

func sizeChoiceLabel(value string) string {
	return choiceLabel(sizeChoices, value)
}

func aspectChoiceLabel(value string) string {
	for _, item := range aspectChoices {
		if item.Value == value {
			return item.Label
		}
	}
	if strings.HasPrefix(value, "custom:") {
		return strings.TrimPrefix(value, "custom:")
	}
	return value
}

func sizeDisplayLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if value == "auto" {
		return "自动"
	}
	aspect := deriveAspectPreset(value, nil)
	resolution := deriveResolutionPreset(value, nil)
	if aspect == "" || resolution == "" {
		return value
	}
	aspectLabel := aspectChoiceLabel(aspect)
	resolutionLabel := choiceLabel(resolutionChoices, resolution)
	if aspectLabel == "" || resolutionLabel == "" {
		return value
	}
	return aspectLabel + " · " + resolutionLabel
}

func qualityChoiceLabel(value string) string {
	return choiceLabel(qualityChoices, value)
}

func backgroundChoiceLabel(value string) string {
	return choiceLabel(backgroundChoices, value)
}

func inputFidelityChoiceLabel(value string) string {
	return choiceLabel(inputFidelityChoices, value)
}

func imageStyleChoiceLabel(value string) string {
	return choiceLabel(imageStyleChoices, value)
}

func moderationChoiceLabel(value string) string {
	return choiceLabel(moderationChoices, value)
}

func qualityDisplayLabel(value string) string {
	return qualityChoiceLabel(value)
}

func styleChoiceLabel(value string) string {
	return choiceLabel(styleChoices, value)
}

func chooseStyleSummary(value string) string {
	if value == "" {
		return "默认风格"
	}
	return styleChoiceLabel(value)
}

func deriveAspectPreset(size string, customRatios []sharedCompat.CustomAspectRatio) string {
	size = strings.TrimSpace(size)
	if size == "auto" {
		return "auto"
	}
	if aspect, _, ok := customSizeSelection(size, customRatios); ok {
		return aspect
	}
	if value, ok := sizeToAspect[size]; ok {
		return value
	}
	width, height, ok := parseSizeSelectionValue(size)
	if !ok {
		return "1:1"
	}
	if customMatch, ok := findMatchingCustomAspect(width, height, customRatios); ok {
		return "custom:" + customMatch.ID
	}
	if builtinAspect, distance := nearestBuiltinAspect(width, height); distance <= builtinAspectTolerance {
		return builtinAspect
	}
	return "1:1"
}

func deriveResolutionPreset(size string, customRatios []sharedCompat.CustomAspectRatio) string {
	size = strings.TrimSpace(size)
	if size == "auto" {
		return "auto"
	}
	if _, resolution, ok := customSizeSelection(size, customRatios); ok {
		return resolution
	}
	if value, ok := sizeToResolution[size]; ok {
		return value
	}
	width, height, ok := parseSizeSelectionValue(size)
	if !ok {
		return "1k"
	}
	bestResolution := "1k"
	bestDistance := math.Inf(1)
	aspect := float64(width) / float64(height)
	for _, resolution := range []string{"1k", "2k", "4k"} {
		expectedWidth, expectedHeight := buildCustomDimensionsForResolution(aspect, resolution)
		distance := customSizeDistance(width, height, float64(expectedWidth), float64(expectedHeight))
		if distance < bestDistance {
			bestDistance = distance
			bestResolution = resolution
		}
	}
	return bestResolution
}

func normalizeSizeSelection(size string, apiMode string, requestPolicy string, imageModelID string, customRatios []sharedCompat.CustomAspectRatio) string {
	size = strings.TrimSpace(size)
	if size == "auto" {
		if supportsAutomaticSizing(imageModelID) {
			return "auto"
		}
		return client.DefaultSize
	}
	if size == "" {
		return client.DefaultSize
	}
	width, height, ok := parseSizeSelectionValue(size)
	if !ok {
		return client.DefaultSize
	}
	if aspect, resolution, ok := customSizeSelection(size, customRatios); ok {
		return buildSupportedSizeSelection(aspect, resolution, apiMode, requestPolicy, imageModelID, customRatios)
	}
	if supportsPreciseSizeControl(apiMode, requestPolicy, imageModelID) && isExactSizeValue(size, apiMode, requestPolicy, imageModelID, customRatios) {
		if exactSize, ok := buildExactSizeValue(width, height); ok {
			return exactSize
		}
		return client.DefaultSize
	}
	aspect := deriveAspectPreset(size, customRatios)
	resolution := deriveResolutionPreset(size, customRatios)
	return buildSupportedSizeSelection(aspect, resolution, apiMode, requestPolicy, imageModelID, customRatios)
}

func buildAspectSizeSelection(aspect string, currentResolution string, apiMode string, requestPolicy string, imageModelID string, customRatios []sharedCompat.CustomAspectRatio) string {
	if strings.TrimSpace(aspect) == "auto" {
		if supportsAutomaticSizing(imageModelID) {
			return "auto"
		}
		return client.DefaultSize
	}
	currentResolution = normalizeResolutionChoice(currentResolution, apiMode, requestPolicy, imageModelID)
	if currentResolution == "auto" {
		currentResolution = "1k"
	}
	return buildSupportedSizeSelection(aspect, currentResolution, apiMode, requestPolicy, imageModelID, customRatios)
}

func buildResolutionSizeSelection(currentAspect string, resolution string, apiMode string, requestPolicy string, imageModelID string, customRatios []sharedCompat.CustomAspectRatio) string {
	if strings.TrimSpace(resolution) == "auto" {
		if supportsAutomaticSizing(imageModelID) {
			return "auto"
		}
		return client.DefaultSize
	}
	resolution = normalizeResolutionChoice(resolution, apiMode, requestPolicy, imageModelID)
	if strings.TrimSpace(currentAspect) == "auto" {
		currentAspect = "1:1"
	}
	return buildSupportedSizeSelection(currentAspect, resolution, apiMode, requestPolicy, imageModelID, customRatios)
}

func buildSupportedSizeSelection(aspect string, resolution string, apiMode string, requestPolicy string, imageModelID string, customRatios []sharedCompat.CustomAspectRatio) string {
	aspect = strings.TrimSpace(aspect)
	resolution = normalizeResolutionChoice(strings.TrimSpace(resolution), apiMode, requestPolicy, imageModelID)
	if aspect == "auto" || resolution == "auto" {
		if supportsAutomaticSizing(imageModelID) {
			return "auto"
		}
		return client.DefaultSize
	}
	if strings.HasPrefix(aspect, "custom:") {
		if !supportsCustomAspectRatios(apiMode, requestPolicy, imageModelID) {
			return client.DefaultSize
		}
		customID := strings.TrimPrefix(aspect, "custom:")
		for _, ratio := range customRatios {
			if strings.TrimSpace(ratio.ID) != customID {
				continue
			}
			return buildCustomSizeSelection(ratio, resolution)
		}
		return buildSizeSelection("1:1", resolution)
	}
	if !isAllowedBuiltinAspect(aspect, imageModelID) {
		return client.DefaultSize
	}
	return buildSizeSelection(aspect, resolution)
}

func buildSizeSelection(aspect string, resolution string) string {
	if aspect == "auto" || resolution == "auto" {
		return "auto"
	}
	if strings.HasPrefix(aspect, "custom:") {
		return "auto"
	}
	if sizes, ok := sizeMatrix[aspect]; ok {
		if value, ok := sizes[resolution]; ok {
			return value
		}
	}
	return "1024x1024"
}

func customSizeSelection(size string, customRatios []sharedCompat.CustomAspectRatio) (string, string, bool) {
	size = strings.TrimSpace(size)
	if size == "" {
		return "", "", false
	}
	for _, ratio := range customRatios {
		if strings.TrimSpace(ratio.ID) == "" || ratio.Width <= 0 || ratio.Height <= 0 {
			continue
		}
		for _, resolution := range []string{"1k", "2k", "4k"} {
			if buildCustomSizeSelection(ratio, resolution) == size {
				return "custom:" + ratio.ID, resolution, true
			}
		}
	}
	return "", "", false
}

func buildCustomSizeSelection(ratio sharedCompat.CustomAspectRatio, resolution string) string {
	if ratio.Width <= 0 || ratio.Height <= 0 {
		return client.DefaultSize
	}
	width, height := buildCustomDimensionsForResolution(float64(ratio.Width)/float64(ratio.Height), resolution)
	return fmt.Sprintf("%dx%d", width, height)
}

func buildCustomResolutionSize(width int, height int, longSide int) string {
	if width <= 0 || height <= 0 || longSide <= 0 {
		return "1024x1024"
	}
	if width >= height {
		shortSide := max(64, int(float64(longSide)*float64(height)/float64(width)+0.5))
		return fmt.Sprintf("%dx%d", longSide, shortSide)
	}
	shortSide := max(64, int(float64(longSide)*float64(width)/float64(height)+0.5))
	return fmt.Sprintf("%dx%d", shortSide, longSide)
}

func aspectChoicesWithCustom(customRatios []sharedCompat.CustomAspectRatio) []aspectChoice {
	if len(customRatios) == 0 {
		return aspectChoices
	}
	items := make([]aspectChoice, 0, len(aspectChoices)+len(customRatios))
	items = append(items, aspectChoices...)
	for _, ratio := range customRatios {
		if strings.TrimSpace(ratio.ID) == "" || ratio.Width <= 0 || ratio.Height <= 0 {
			continue
		}
		w, h := aspectShapeFromRatio(ratio.Width, ratio.Height)
		items = append(items, aspectChoice{
			Value:  "custom:" + ratio.ID,
			Label:  ratio.Label,
			W:      w,
			H:      h,
			Custom: true,
		})
	}
	return items
}

func visibleAspectChoices(apiMode string, requestPolicy string, imageModelID string, customRatios []sharedCompat.CustomAspectRatio) []aspectChoice {
	_ = apiMode
	allowed := allowedBuiltinAspects(imageModelID)
	items := make([]aspectChoice, 0, len(aspectChoices)+len(customRatios))
	for _, choice := range aspectChoices {
		if !allowed[choice.Value] {
			continue
		}
		items = append(items, choice)
	}
	if !supportsCustomAspectRatios(apiMode, requestPolicy, imageModelID) {
		return items
	}
	for _, ratio := range customRatios {
		if strings.TrimSpace(ratio.ID) == "" || ratio.Width <= 0 || ratio.Height <= 0 {
			continue
		}
		w, h := aspectShapeFromRatio(ratio.Width, ratio.Height)
		items = append(items, aspectChoice{
			Value:  "custom:" + ratio.ID,
			Label:  ratio.Label,
			W:      w,
			H:      h,
			Custom: true,
		})
	}
	return items
}

func aspectShapeFromRatio(width int, height int) (int, int) {
	if width <= 0 || height <= 0 {
		return 18, 18
	}
	const maxW = 24
	const maxH = 22
	scaleW := float64(maxW) / float64(width)
	scaleH := float64(maxH) / float64(height)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	if scale <= 0 {
		scale = 1
	}
	w := max(10, int(float64(width)*scale+0.5))
	h := max(10, int(float64(height)*scale+0.5))
	return w, h
}

func visibleResolutionChoices(apiMode string, requestPolicy string, imageModelID string) []choice {
	family := classifyImageModel(imageModelID)
	switch {
	case family == "dalle2":
		return filterResolutionChoices("256", "512", "1k")
	case family == "dalle3":
		return filterResolutionChoices("1k")
	case isLegacyGPTImageModel(imageModelID):
		return filterResolutionChoices("auto", "1k")
	case supportsExplicitLargeSizes(apiMode, requestPolicy, imageModelID):
		return filterResolutionChoices("auto", "1k", "2k", "4k")
	default:
		return filterResolutionChoices("auto", "1k")
	}
}

func visibleNonAutoResolutionChoices(apiMode string, requestPolicy string, imageModelID string) []choice {
	choices := visibleResolutionChoices(apiMode, requestPolicy, imageModelID)
	filtered := make([]choice, 0, len(choices))
	for _, item := range choices {
		if item.Value == "auto" {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func filterResolutionChoices(values ...string) []choice {
	if len(values) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		allowed[strings.TrimSpace(value)] = struct{}{}
	}
	filtered := make([]choice, 0, len(values))
	for _, item := range resolutionChoices {
		if _, ok := allowed[item.Value]; !ok {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func normalizeBatchCount(value int) int {
	if value < 1 {
		return 1
	}
	if value > 9 {
		return 9
	}
	return value
}

func choiceLabel(choices []choice, value string) string {
	for _, item := range choices {
		if item.Value == value {
			return item.Label
		}
	}
	return value
}

func classifyImageModel(modelID string) string {
	value := normalizedImageModelID(modelID)
	switch {
	case strings.HasPrefix(value, "dall-e-2"):
		return "dalle2"
	case strings.HasPrefix(value, "dall-e-3"), strings.HasPrefix(value, "dalle-3"), strings.HasPrefix(value, "dall-e3"), strings.HasPrefix(value, "dalle3"):
		return "dalle3"
	case strings.HasPrefix(value, "gpt-image"), strings.HasPrefix(value, "chatgpt-image"):
		return "gpt-image"
	default:
		return "other"
	}
}

func normalizedImageModelID(modelID string) string {
	value := strings.ToLower(strings.TrimSpace(modelID))
	if value == "" {
		return strings.ToLower(client.ImageModel)
	}
	return value
}

func isFlexibleGPTImageModel(imageModelID string) bool {
	return strings.HasPrefix(normalizedImageModelID(imageModelID), "gpt-image-2")
}

func isLegacyGPTImageModel(imageModelID string) bool {
	value := normalizedImageModelID(imageModelID)
	if strings.HasPrefix(value, "gpt-image-2") {
		return false
	}
	return strings.HasPrefix(value, "gpt-image-1") || strings.HasPrefix(value, "chatgpt-image")
}

func supportsAutomaticSizing(imageModelID string) bool {
	return isFlexibleGPTImageModel(imageModelID) || isLegacyGPTImageModel(imageModelID)
}

func supportsCustomAspectRatios(apiMode string, requestPolicy string, imageModelID string) bool {
	_ = apiMode
	if isFlexibleGPTImageModel(imageModelID) {
		return true
	}
	return classifyImageModel(imageModelID) == "other" && strings.TrimSpace(requestPolicy) == string(client.RequestPolicyCompat)
}

func supportsExplicitLargeSizes(apiMode string, requestPolicy string, imageModelID string) bool {
	_ = apiMode
	if isFlexibleGPTImageModel(imageModelID) {
		return true
	}
	return classifyImageModel(imageModelID) == "other" && strings.TrimSpace(requestPolicy) == string(client.RequestPolicyCompat)
}

func allowedBuiltinAspects(imageModelID string) map[string]bool {
	switch {
	case classifyImageModel(imageModelID) == "dalle2":
		return map[string]bool{"1:1": true}
	case classifyImageModel(imageModelID) == "dalle3":
		return map[string]bool{"1:1": true, "7:4": true, "4:7": true}
	case isLegacyGPTImageModel(imageModelID):
		return map[string]bool{"auto": true, "1:1": true, "3:2": true, "2:3": true}
	default:
		return map[string]bool{"auto": true, "1:1": true, "3:2": true, "2:3": true, "16:9": true, "9:16": true}
	}
}

func isAllowedBuiltinAspect(aspect string, imageModelID string) bool {
	return allowedBuiltinAspects(imageModelID)[strings.TrimSpace(aspect)]
}

func normalizeResolutionChoice(resolution string, apiMode string, requestPolicy string, imageModelID string) string {
	allowed := visibleResolutionChoices(apiMode, requestPolicy, imageModelID)
	resolution = strings.TrimSpace(resolution)
	for _, item := range allowed {
		if item.Value == resolution {
			return resolution
		}
	}
	for _, item := range allowed {
		if item.Value != "auto" {
			return item.Value
		}
	}
	if len(allowed) > 0 {
		return allowed[0].Value
	}
	return "1k"
}

func normalizeBatchAutoAspectResolution(resolution string, apiMode string, requestPolicy string, imageModelID string) string {
	allowed := visibleNonAutoResolutionChoices(apiMode, requestPolicy, imageModelID)
	resolution = strings.TrimSpace(resolution)
	for _, item := range allowed {
		if item.Value == resolution {
			return item.Value
		}
	}
	if len(allowed) > 0 {
		return allowed[0].Value
	}
	return "1k"
}

func sizeCapabilityHint(apiMode string, requestPolicy string, imageModelID string) string {
	switch classifyImageModel(imageModelID) {
	case "dalle2":
		return "当前模型仅支持 256 / 512 / 1024 的正方形尺寸。"
	case "dalle3":
		return "当前模型仅支持 1024x1024、1792x1024、1024x1792。"
	}
	if supportsExplicitLargeSizes(apiMode, requestPolicy, imageModelID) {
		return ""
	}
	return "当前链路只保证基础尺寸稳定可用。"
}
