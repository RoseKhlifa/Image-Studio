package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	gioCompat "image-studio/gio-client/internal/compat"
	sharedCompat "image-studio/shared/compat"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

type codexAPIConfig struct {
	Provider string
	BaseURL  string
	APIKey   string
}

type codexAuthFile struct {
	OpenAIAPIKey string `json:"OPENAI_API_KEY"`
}

func canLoadCodexAPIConfig() bool {
	_, err := os.UserHomeDir()
	return err == nil
}

func loadCodexAPIConfig() (codexAPIConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return codexAPIConfig{}, fmt.Errorf("无法定位用户目录: %w", err)
	}
	return loadCodexAPIConfigFromDir(filepath.Join(home, ".codex"))
}

func loadCodexAPIConfigFromDir(dir string) (codexAPIConfig, error) {
	configPath := filepath.Join(dir, "config.toml")
	authPath := filepath.Join(dir, "auth.json")

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return codexAPIConfig{}, fmt.Errorf("读取 Codex 配置失败(%s): %w", configPath, err)
	}
	provider, baseURL, err := parseCodexConfigTOML(string(configData))
	if err != nil {
		return codexAPIConfig{}, err
	}

	authData, err := os.ReadFile(authPath)
	if err != nil {
		return codexAPIConfig{}, fmt.Errorf("读取 Codex 凭据失败(%s): %w", authPath, err)
	}
	var auth codexAuthFile
	if err := json.Unmarshal(authData, &auth); err != nil {
		return codexAPIConfig{}, fmt.Errorf("解析 Codex 凭据失败: %w", err)
	}
	apiKey := strings.TrimSpace(auth.OpenAIAPIKey)
	if apiKey == "" {
		return codexAPIConfig{}, fmt.Errorf("Codex 凭据里未找到 OPENAI_API_KEY")
	}

	return codexAPIConfig{
		Provider: provider,
		BaseURL:  baseURL,
		APIKey:   apiKey,
	}, nil
}

func parseCodexConfigTOML(raw string) (provider string, baseURL string, err error) {
	lines := strings.Split(raw, "\n")
	provider = findCodexRootString(lines, "model_provider")
	if provider == "" {
		return "", "", fmt.Errorf("Codex config.toml 缺少 model_provider")
	}

	baseURL = findCodexProviderString(lines, provider, "base_url")
	if baseURL == "" {
		return "", "", fmt.Errorf("Codex provider %q 缺少 base_url", provider)
	}
	baseURL = normalizeCodexBaseURL(baseURL)
	if baseURL == "" {
		return "", "", fmt.Errorf("Codex provider %q 的 base_url 无效", provider)
	}

	return provider, baseURL, nil
}

func findCodexRootString(lines []string, targetKey string) string {
	currentSection := ""
	for _, rawLine := range lines {
		line := strings.TrimSpace(stripCodexTOMLComment(rawLine))
		if line == "" {
			continue
		}
		if section, ok := parseCodexTOMLSection(line); ok {
			currentSection = section
			continue
		}
		if currentSection != "" {
			continue
		}
		key, value, ok := parseCodexTOMLKeyValue(line)
		if !ok || key != targetKey {
			continue
		}
		return value
	}
	return ""
}

func findCodexProviderString(lines []string, provider string, targetKey string) string {
	currentSection := ""
	for _, rawLine := range lines {
		line := strings.TrimSpace(stripCodexTOMLComment(rawLine))
		if line == "" {
			continue
		}
		if section, ok := parseCodexTOMLSection(line); ok {
			currentSection = section
			continue
		}
		if !codexSectionMatchesProvider(currentSection, provider) {
			continue
		}
		key, value, ok := parseCodexTOMLKeyValue(line)
		if !ok || key != targetKey {
			continue
		}
		return value
	}
	return ""
}

func stripCodexTOMLComment(line string) string {
	var out strings.Builder
	inDouble := false
	inSingle := false
	escaped := false
	for _, r := range line {
		if escaped {
			out.WriteRune(r)
			escaped = false
			continue
		}
		switch r {
		case '\\':
			if inDouble {
				escaped = true
			}
			out.WriteRune(r)
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
			out.WriteRune(r)
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
			out.WriteRune(r)
		case '#':
			if !inDouble && !inSingle {
				return out.String()
			}
			out.WriteRune(r)
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

func parseCodexTOMLSection(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}
	section := strings.TrimSpace(line[1 : len(line)-1])
	if section == "" {
		return "", false
	}
	return section, true
}

func parseCodexTOMLKeyValue(line string) (string, string, bool) {
	eq := strings.IndexRune(line, '=')
	if eq <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:eq])
	if key == "" {
		return "", "", false
	}
	value, ok := parseCodexTOMLStringValue(line[eq+1:])
	if !ok {
		return "", "", false
	}
	return key, value, true
}

func parseCodexTOMLStringValue(raw string) (string, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", false
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		decoded, err := strconv.Unquote(value)
		if err != nil {
			return "", false
		}
		return decoded, true
	}
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1], true
	}
	return value, true
}

func codexSectionMatchesProvider(section string, provider string) bool {
	const prefix = "model_providers."
	if !strings.HasPrefix(section, prefix) {
		return false
	}
	name := strings.TrimSpace(strings.TrimPrefix(section, prefix))
	if len(name) >= 2 && ((name[0] == '"' && name[len(name)-1] == '"') || (name[0] == '\'' && name[len(name)-1] == '\'')) {
		name = name[1 : len(name)-1]
	}
	return name == provider
}

func normalizeCodexBaseURL(raw string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	if trimmed == "" {
		return ""
	}
	return strings.TrimSuffix(trimmed, "/v1")
}

func codexProfileName(provider string) string {
	provider = strings.TrimSpace(provider)
	if provider == "" {
		return "Codex"
	}
	return "Codex · " + provider
}

func (a *App) startCodexConfigSync() {
	a.mu.Lock()
	if a.syncingCodexConfig {
		a.mu.Unlock()
		return
	}
	a.syncingCodexConfig = true
	a.appendLogLocked("开始同步 Codex 配置")
	a.mu.Unlock()
	a.invalidateNow()

	go func() {
		imported, err := loadCodexAPIConfig()
		if err != nil {
			a.finishCodexConfigSync("", err)
			return
		}
		name := codexProfileName(imported.Provider)
		if err := a.applyCodexConfigSync(imported); err != nil {
			a.finishCodexConfigSync(name, err)
			return
		}
		a.finishCodexConfigSync(name, nil)
	}()
}

func (a *App) finishCodexConfigSync(name string, err error) {
	a.mu.Lock()
	a.syncingCodexConfig = false
	if err != nil {
		a.status = "同步 Codex 配置失败"
		a.appendLogLocked("同步 Codex 配置失败: " + err.Error())
		a.mu.Unlock()
		a.invalidateNow()
		return
	}
	if strings.TrimSpace(name) == "" {
		name = "Codex"
	}
	a.status = "已同步 " + name
	a.appendLogLocked("已同步 " + name)
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) applyCodexConfigSync(imported codexAPIConfig) error {
	name := codexProfileName(imported.Provider)
	now := time.Now().UnixMilli()
	profileID := ""
	err := gioCompat.UpdateState(func(state *sharedCompat.State) error {
		*state = sharedCompat.Normalize(*state)
		found := false
		for i := range state.Profiles {
			if strings.TrimSpace(state.Profiles[i].Name) != name {
				continue
			}
			state.Profiles[i].Name = name
			state.Profiles[i].Provider = string(client.ProviderOpenAI)
			state.Profiles[i].APIMode = string(client.APIModeResponses)
			state.Profiles[i].RequestPolicy = string(client.RequestPolicyOpenAI)
			state.Profiles[i].ImagesNewAPICompat = false
			state.Profiles[i].BaseURL = imported.BaseURL
			state.Profiles[i].LastUsedAt = now
			profileID = state.Profiles[i].ID
			found = true
			break
		}
		if !found {
			profileID = fmt.Sprintf("gio-%d", now)
			state.Profiles = append(state.Profiles, sharedCompat.UpstreamProfile{
				ID:                 profileID,
				Name:               name,
				Provider:           string(client.ProviderOpenAI),
				APIMode:            string(client.APIModeResponses),
				RequestPolicy:      string(client.RequestPolicyOpenAI),
				ImagesNewAPICompat: false,
				BaseURL:            imported.BaseURL,
				TextModelID:        client.TextModel,
				ImageModelID:       client.ImageModel,
				ReasoningEffort:    "xhigh",
				CreatedAt:          now,
				LastUsedAt:         now,
			})
		}
		state.ActiveProfile = profileID
		state.UpdatedAt = now
		return nil
	})
	if err != nil {
		return err
	}
	if err := gioCompat.WriteAPIKey(profileID, imported.APIKey); err != nil {
		return err
	}
	if err := a.restoreActiveRuntimeConfig(false); err != nil {
		return err
	}
	if err := a.loadSettingsProfileDraft(profileID); err != nil {
		return err
	}
	a.apiKeyVisible = false
	return nil
}
