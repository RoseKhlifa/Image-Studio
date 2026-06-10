package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/kernel"
	sharedCompat "image-studio/shared/compat"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

type upstreamConfigExportProfile struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	APIMode            string `json:"apiMode"`
	ResponsesTransport string `json:"responsesTransport"`
	RequestPolicy      string `json:"requestPolicy"`
	ImagesNewAPICompat bool   `json:"imagesNewAPICompat"`
	BaseURL            string `json:"baseURL"`
	TextModelID        string `json:"textModelID"`
	ImageModelID       string `json:"imageModelID"`
	ReasoningEffort    string `json:"reasoningEffort"`
	ConcurrencyLimit   int    `json:"concurrencyLimit"`
	FallbackProfileID  string `json:"fallbackProfileId,omitempty"`
	CreatedAt          int64  `json:"createdAt"`
	LastUsedAt         int64  `json:"lastUsedAt,omitempty"`
	APIKey             string `json:"apiKey,omitempty"`
}

type upstreamConfigExportFile struct {
	Version         int                           `json:"version"`
	ExportedAt      string                        `json:"exportedAt"`
	ActiveProfileID string                        `json:"activeProfileId"`
	Profiles        []upstreamConfigExportProfile `json:"profiles"`
}

type parsedUpstreamConfigImport struct {
	ActiveProfileID string
	Profiles        []upstreamConfigExportProfile
}

func numberFromAny(raw any) (int, bool) {
	switch value := raw.(type) {
	case float64:
		return int(value), true
	case float32:
		return int(value), true
	case int:
		return value, true
	case int64:
		return int(value), true
	case int32:
		return int(value), true
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed), true
		}
	}
	return 0, false
}

func buildUpstreamConfigExportFile(state sharedCompat.State) (upstreamConfigExportFile, error) {
	state = sharedCompat.Normalize(state)
	profiles := make([]upstreamConfigExportProfile, 0, len(state.Profiles))
	for _, profile := range state.Profiles {
		apiKey, err := gioCompat.ReadAPIKey(profile.ID)
		if err != nil {
			return upstreamConfigExportFile{}, err
		}
		profiles = append(profiles, upstreamConfigExportProfile{
			ID:                 profile.ID,
			Name:               strings.TrimSpace(profile.Name),
			APIMode:            normalizeProfileAPIMode(profile.APIMode),
			ResponsesTransport: normalizeProfileResponsesTransport(profile.ResponsesTransport),
			RequestPolicy:      normalizeProfilePolicy(profile.RequestPolicy),
			ImagesNewAPICompat: profile.ImagesNewAPICompat,
			BaseURL:            strings.TrimSpace(profile.BaseURL),
			TextModelID:        strings.TrimSpace(profile.TextModelID),
			ImageModelID:       strings.TrimSpace(profile.ImageModelID),
			ReasoningEffort:    normalizeReasoningEffort(profile.ReasoningEffort),
			ConcurrencyLimit:   profile.ConcurrencyLimit,
			FallbackProfileID:  strings.TrimSpace(profile.FallbackProfileID),
			CreatedAt:          profile.CreatedAt,
			LastUsedAt:         profile.LastUsedAt,
			APIKey:             strings.TrimSpace(apiKey),
		})
	}
	return upstreamConfigExportFile{
		Version:         1,
		ExportedAt:      time.Now().UTC().Format(time.RFC3339),
		ActiveProfileID: strings.TrimSpace(state.ActiveProfile),
		Profiles:        profiles,
	}, nil
}

func parseUpstreamConfigImportJSON(raw string) (parsedUpstreamConfigImport, error) {
	raw = stripWrappedCodeFence(raw)
	var anyJSON map[string]any
	if err := json.Unmarshal([]byte(raw), &anyJSON); err != nil {
		return parsedUpstreamConfigImport{}, err
	}
	if parsed, ok, err := parseNewAPIChannelConnTemplate(anyJSON); err != nil {
		return parsedUpstreamConfigImport{}, err
	} else if ok {
		return parsed, nil
	}
	if parsed, ok, err := parseOpenCodeProviderTemplate(anyJSON); err != nil {
		return parsedUpstreamConfigImport{}, err
	} else if ok {
		return parsed, nil
	}
	parsed, err := parseExportProfiles(anyJSON)
	if err != nil {
		return parsedUpstreamConfigImport{}, err
	}
	if len(parsed.Profiles) == 0 {
		return parsedUpstreamConfigImport{}, fmt.Errorf("暂不支持这类 JSON。当前支持：本应用导出文件、newapi_channel_conn、OpenCode provider 配置")
	}
	return parsed, nil
}

func stripWrappedCodeFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") && strings.HasSuffix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```JSON")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
	}
	return strings.TrimSpace(trimmed)
}

func parseExportProfiles(input map[string]any) (parsedUpstreamConfigImport, error) {
	rawProfiles, _ := input["profiles"].([]any)
	profiles := make([]upstreamConfigExportProfile, 0, len(rawProfiles))
	for _, item := range rawProfiles {
		profile, ok := parseExportProfile(item)
		if ok {
			profiles = append(profiles, profile)
		}
	}
	activeProfileID, _ := input["activeProfileId"].(string)
	return parsedUpstreamConfigImport{
		ActiveProfileID: strings.TrimSpace(activeProfileID),
		Profiles:        profiles,
	}, nil
}

func parseExportProfile(raw any) (upstreamConfigExportProfile, bool) {
	source, ok := raw.(map[string]any)
	if !ok {
		return upstreamConfigExportProfile{}, false
	}
	id, _ := source["id"].(string)
	name, _ := source["name"].(string)
	if strings.TrimSpace(id) == "" || strings.TrimSpace(name) == "" {
		return upstreamConfigExportProfile{}, false
	}
	createdAt := time.Now().UnixMilli()
	if value, ok := numberFromAny(source["createdAt"]); ok {
		createdAt = int64(value)
	}
	lastUsedAt := int64(0)
	if value, ok := numberFromAny(source["lastUsedAt"]); ok {
		lastUsedAt = int64(value)
	}
	concurrencyLimit := 0
	if value, ok := numberFromAny(source["concurrencyLimit"]); ok && value > 0 {
		concurrencyLimit = value
	}
	apiKey, _ := source["apiKey"].(string)
	return upstreamConfigExportProfile{
		ID:                 strings.TrimSpace(id),
		Name:               strings.TrimSpace(name),
		APIMode:            normalizeProfileAPIMode(stringValue(source["apiMode"])),
		ResponsesTransport: normalizeProfileResponsesTransport(stringValue(source["responsesTransport"])),
		RequestPolicy:      normalizeProfilePolicy(stringValue(source["requestPolicy"])),
		ImagesNewAPICompat: boolValue(source["imagesNewAPICompat"]),
		BaseURL:            normalizeImportedBaseURL(stringValue(source["baseURL"])),
		TextModelID:        strings.TrimSpace(stringValue(source["textModelID"])),
		ImageModelID:       strings.TrimSpace(stringValue(source["imageModelID"])),
		ReasoningEffort:    normalizeReasoningEffort(stringValue(source["reasoningEffort"])),
		ConcurrencyLimit:   concurrencyLimit,
		FallbackProfileID:  strings.TrimSpace(stringValue(source["fallbackProfileId"])),
		CreatedAt:          createdAt,
		LastUsedAt:         lastUsedAt,
		APIKey:             strings.TrimSpace(apiKey),
	}, true
}

func parseNewAPIChannelConnTemplate(input map[string]any) (parsedUpstreamConfigImport, bool, error) {
	if strings.TrimSpace(stringValue(input["_type"])) != "newapi_channel_conn" {
		return parsedUpstreamConfigImport{}, false, nil
	}
	apiKey := strings.TrimSpace(stringValue(input["key"]))
	baseURL := normalizeImportedBaseURL(stringValue(input["url"]))
	if apiKey == "" || baseURL == "" {
		return parsedUpstreamConfigImport{}, true, fmt.Errorf("newapi 模板缺少 key 或 url")
	}
	profileID := nextTemplateProfileID(1)
	return parsedUpstreamConfigImport{
		ActiveProfileID: profileID,
		Profiles: []upstreamConfigExportProfile{{
			ID:                 profileID,
			Name:               buildTemplateProfileName("NewAPI", baseURL, ""),
			APIMode:            string(client.APIModeResponses),
			ResponsesTransport: string(client.ResponsesTransportSSE),
			RequestPolicy:      string(client.RequestPolicyOpenAI),
			ImagesNewAPICompat: false,
			BaseURL:            baseURL,
			TextModelID:        "",
			ImageModelID:       "",
			ReasoningEffort:    client.DefaultReasoningEffort,
			CreatedAt:          time.Now().UnixMilli(),
			APIKey:             apiKey,
		}},
	}, true, nil
}

func parseOpenCodeProviderTemplate(input map[string]any) (parsedUpstreamConfigImport, bool, error) {
	providerRoot, ok := input["provider"].(map[string]any)
	if !ok {
		return parsedUpstreamConfigImport{}, false, nil
	}
	profiles := make([]upstreamConfigExportProfile, 0, len(providerRoot))
	index := 1
	for providerName, rawProvider := range providerRoot {
		provider, ok := rawProvider.(map[string]any)
		if !ok {
			continue
		}
		options, _ := provider["options"].(map[string]any)
		apiKey := strings.TrimSpace(stringValue(options["apiKey"]))
		baseURL := normalizeImportedBaseURL(firstString(options["baseURL"], options["baseUrl"]))
		if apiKey == "" || baseURL == "" {
			continue
		}
		modelCatalog := buildOpenCodeModelCatalog(provider["models"])
		textModels, imageModels := preferredProbeModels(string(client.APIModeResponses), modelCatalog)
		apiMode := string(client.APIModeResponses)
		textModelID := ""
		imageModelID := ""
		if len(textModels) > 0 {
			textModelID = textModels[0].ID
		}
		if len(imageModels) > 0 {
			imageModelID = imageModels[0].ID
		}
		if len(textModels) == 0 && len(imageModels) > 0 {
			apiMode = string(client.APIModeImages)
		}
		profiles = append(profiles, upstreamConfigExportProfile{
			ID:                 nextTemplateProfileID(index),
			Name:               buildTemplateProfileName("OpenCode", baseURL, providerName),
			APIMode:            apiMode,
			ResponsesTransport: string(client.ResponsesTransportSSE),
			RequestPolicy:      string(client.RequestPolicyOpenAI),
			ImagesNewAPICompat: false,
			BaseURL:            baseURL,
			TextModelID:        textModelID,
			ImageModelID:       imageModelID,
			ReasoningEffort:    inferReasoningEffortFromOpenCodeModels(provider["models"], textModelID),
			CreatedAt:          time.Now().UnixMilli(),
			APIKey:             apiKey,
		})
		index++
	}
	if len(profiles) == 0 {
		return parsedUpstreamConfigImport{}, true, fmt.Errorf("OpenCode 模板里没有可用的 provider 配置")
	}
	return parsedUpstreamConfigImport{
		ActiveProfileID: profiles[0].ID,
		Profiles:        profiles,
	}, true, nil
}

func buildOpenCodeModelCatalog(raw any) []kernel.UpstreamModelDescriptor {
	models, _ := raw.(map[string]any)
	out := make([]kernel.UpstreamModelDescriptor, 0, len(models))
	for id, rawModel := range models {
		model, _ := rawModel.(map[string]any)
		out = append(out, kernel.UpstreamModelDescriptor{
			ID:          strings.TrimSpace(id),
			DisplayName: strings.TrimSpace(stringValue(model["name"])),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func inferReasoningEffortFromOpenCodeModels(raw any, textModelID string) string {
	models, _ := raw.(map[string]any)
	model, _ := models[textModelID].(map[string]any)
	variants, _ := model["variants"].(map[string]any)
	if len(variants) == 0 {
		return client.DefaultReasoningEffort
	}
	for _, candidate := range []string{"xhigh", "high", "medium", "low"} {
		if _, ok := variants[candidate]; ok {
			return candidate
		}
	}
	return client.DefaultReasoningEffort
}

func nextTemplateProfileID(index int) string {
	return fmt.Sprintf("template-%d", index)
}

func buildTemplateProfileName(prefix string, baseURL string, providerName string) string {
	host := baseURL
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	parts := []string{strings.TrimSpace(prefix), strings.TrimSpace(providerName), strings.TrimSpace(host)}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return "快捷导入"
	}
	return strings.Join(out, " · ")
}

func normalizeImportedBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimRight(trimmed, "/")
	if trimmed == "" {
		return ""
	}
	return strings.TrimSuffix(trimmed, "/v1")
}

func stringValue(raw any) string {
	value, _ := raw.(string)
	return value
}

func firstString(values ...any) string {
	for _, raw := range values {
		if value := strings.TrimSpace(stringValue(raw)); value != "" {
			return value
		}
	}
	return ""
}

func boolValue(raw any) bool {
	value, _ := raw.(bool)
	return value
}

func (a *App) applyParsedUpstreamConfigImport(parsed parsedUpstreamConfigImport) (int, string, error) {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		return 0, "", err
	}
	state = sharedCompat.Normalize(state)
	existingByName := map[string]int{}
	for i, profile := range state.Profiles {
		existingByName[strings.TrimSpace(profile.Name)] = i
	}
	originalToActualID := map[string]string{}
	type fallbackLink struct {
		actualID           string
		originalFallbackID string
	}
	links := make([]fallbackLink, 0, len(parsed.Profiles))
	importedIDs := make([]string, 0, len(parsed.Profiles))
	now := time.Now().UnixMilli()

	for idx, incoming := range parsed.Profiles {
		if existingIndex, ok := existingByName[strings.TrimSpace(incoming.Name)]; ok {
			profile := state.Profiles[existingIndex]
			profile.Name = strings.TrimSpace(incoming.Name)
			profile.APIMode = normalizeProfileAPIMode(incoming.APIMode)
			profile.ResponsesTransport = normalizeProfileResponsesTransport(incoming.ResponsesTransport)
			profile.RequestPolicy = normalizeProfilePolicy(incoming.RequestPolicy)
			profile.ImagesNewAPICompat = incoming.ImagesNewAPICompat
			profile.BaseURL = normalizeImportedBaseURL(incoming.BaseURL)
			profile.TextModelID = strings.TrimSpace(incoming.TextModelID)
			profile.ImageModelID = strings.TrimSpace(incoming.ImageModelID)
			profile.ReasoningEffort = normalizeReasoningEffort(incoming.ReasoningEffort)
			profile.ConcurrencyLimit = incoming.ConcurrencyLimit
			profile.LastUsedAt = incoming.LastUsedAt
			state.Profiles[existingIndex] = profile
			if strings.TrimSpace(incoming.APIKey) != "" {
				_ = gioCompat.WriteAPIKey(profile.ID, strings.TrimSpace(incoming.APIKey))
			}
			originalToActualID[incoming.ID] = profile.ID
			importedIDs = append(importedIDs, profile.ID)
			links = append(links, fallbackLink{actualID: profile.ID, originalFallbackID: strings.TrimSpace(incoming.FallbackProfileID)})
			continue
		}

		actualID := fmt.Sprintf("gio-import-%d-%d", now, idx+1)
		profile := sharedCompat.UpstreamProfile{
			ID:                 actualID,
			Name:               strings.TrimSpace(incoming.Name),
			APIMode:            normalizeProfileAPIMode(incoming.APIMode),
			ResponsesTransport: normalizeProfileResponsesTransport(incoming.ResponsesTransport),
			RequestPolicy:      normalizeProfilePolicy(incoming.RequestPolicy),
			ImagesNewAPICompat: incoming.ImagesNewAPICompat,
			BaseURL:            normalizeImportedBaseURL(incoming.BaseURL),
			TextModelID:        strings.TrimSpace(incoming.TextModelID),
			ImageModelID:       strings.TrimSpace(incoming.ImageModelID),
			ReasoningEffort:    normalizeReasoningEffort(incoming.ReasoningEffort),
			ConcurrencyLimit:   incoming.ConcurrencyLimit,
			CreatedAt:          now,
			LastUsedAt:         incoming.LastUsedAt,
		}
		state.Profiles = append(state.Profiles, profile)
		if strings.TrimSpace(incoming.APIKey) != "" {
			_ = gioCompat.WriteAPIKey(actualID, strings.TrimSpace(incoming.APIKey))
		}
		existingByName[profile.Name] = len(state.Profiles) - 1
		originalToActualID[incoming.ID] = actualID
		importedIDs = append(importedIDs, actualID)
		links = append(links, fallbackLink{actualID: actualID, originalFallbackID: strings.TrimSpace(incoming.FallbackProfileID)})
	}

	for _, link := range links {
		if strings.TrimSpace(link.originalFallbackID) == "" {
			continue
		}
		resolved := originalToActualID[strings.TrimSpace(link.originalFallbackID)]
		if resolved == "" {
			for _, profile := range state.Profiles {
				if profile.ID == strings.TrimSpace(link.originalFallbackID) {
					resolved = profile.ID
					break
				}
			}
		}
		if resolved == "" {
			continue
		}
		for i := range state.Profiles {
			if state.Profiles[i].ID == link.actualID {
				state.Profiles[i].FallbackProfileID = resolved
				break
			}
		}
	}

	activeProfileID := ""
	if strings.TrimSpace(parsed.ActiveProfileID) != "" {
		activeProfileID = originalToActualID[strings.TrimSpace(parsed.ActiveProfileID)]
	}
	if activeProfileID != "" {
		state.ActiveProfile = activeProfileID
	}
	state.UpdatedAt = now
	if err := gioCompat.SaveState(state); err != nil {
		return 0, "", err
	}
	if activeProfileID != "" {
		if err := a.restoreActiveRuntimeConfig(false); err != nil {
			return 0, "", err
		}
		if err := a.loadSettingsProfileDraft(activeProfileID); err != nil {
			return 0, "", err
		}
	} else if len(importedIDs) > 0 {
		if err := a.loadSettingsProfileDraft(importedIDs[0]); err != nil {
			return 0, "", err
		}
	}
	a.mu.Lock()
	a.setProfilesLocked(state.Profiles)
	a.settingsSelectedProfileID = normalizeSettingsSelectedProfileID(state, activeProfileID)
	a.mu.Unlock()
	return len(parsed.Profiles), activeProfileID, nil
}

func (a *App) exportUpstreamConfigs() {
	state, _, err := gioCompat.LoadState()
	if err != nil {
		a.appendLog("导出上游配置失败: " + err.Error())
		return
	}
	state = sharedCompat.Normalize(state)
	payload, err := buildUpstreamConfigExportFile(state)
	if err != nil {
		a.appendLog("导出上游配置失败: " + err.Error())
		return
	}
	filename := fmt.Sprintf("image-studio-upstream-config-%s.json", time.Now().Format("20060102-150405"))
	dst, err := chooseSaveJSONFile(filename)
	if err != nil {
		a.appendLog("选择导出文件失败: " + err.Error())
		return
	}
	if strings.TrimSpace(dst) == "" {
		return
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		a.appendLog("导出上游配置失败: " + err.Error())
		return
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		a.appendLog("写入上游配置导出文件失败: " + err.Error())
		return
	}
	a.appendLog("已导出上游配置: " + filepath.Base(dst))
}

func (a *App) importUpstreamConfigsFromFile() {
	src, err := chooseJSONFile()
	if err != nil {
		a.appendLog("选择上游配置文件失败: " + err.Error())
		return
	}
	if strings.TrimSpace(src) == "" {
		return
	}
	data, err := os.ReadFile(src)
	if err != nil {
		a.appendLog("读取上游配置文件失败: " + err.Error())
		return
	}
	if err := a.importUpstreamConfigsFromRaw(string(data)); err != nil {
		a.appendLog("导入上游配置失败: " + err.Error())
		return
	}
	a.appendLog("已导入上游配置: " + filepath.Base(src))
}

func (a *App) importUpstreamConfigsFromRaw(raw string) error {
	parsed, err := parseUpstreamConfigImportJSON(raw)
	if err != nil {
		return err
	}
	count, _, err := a.applyParsedUpstreamConfigImport(parsed)
	if err != nil {
		return err
	}
	a.appendLog(fmt.Sprintf("已导入 %d 个上游配置", count))
	return nil
}
