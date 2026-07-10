package compat

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"image-studio/gio-client/internal/kernel"
	shared "image-studio/shared/compat"

	"github.com/yuanhua/image-gptcodex/pkg/client"
	keyring "github.com/zalando/go-keyring"
)

const keyringServiceName = "Image Studio"

var stateFileMu sync.Mutex

func LoadState() (shared.State, string, error) {
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	return loadStateUnlocked()
}

func loadStateUnlocked() (shared.State, string, error) {
	root, err := StableDataRoot()
	if err != nil {
		return shared.EmptyState(), "", err
	}
	path := shared.StatePath(root)
	state, err := shared.Load(path)
	if err == nil {
		state = hydrateHistoryFull(state)
	}
	return state, path, err
}

func SaveState(state shared.State) error {
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	return saveStateUnlocked(state)
}

func saveStateUnlocked(state shared.State) error {
	root, err := StableDataRoot()
	if err != nil {
		return err
	}
	state.Client = "gio"
	if state.UpdatedAt <= 0 {
		state.UpdatedAt = time.Now().UnixMilli()
	}
	state.HistoryFull = pruneHistoryFull(state.HistoryFull, state.History)
	state = dehydrateHistoryFull(state)
	return shared.Save(shared.StatePath(root), state)
}

func UpdateState(update func(*shared.State) error) error {
	if update == nil {
		return nil
	}
	stateFileMu.Lock()
	defer stateFileMu.Unlock()
	state, _, err := loadStateUnlocked()
	if err != nil {
		return err
	}
	if err := update(&state); err != nil {
		return err
	}
	return saveStateUnlocked(state)
}

func ConfigFromState(cfg kernel.Config, state shared.State) kernel.Config {
	cfg.OutputDir = DefaultOutputDir()
	if strings.TrimSpace(state.Settings.OutputDir) != "" {
		cfg.OutputDir = state.Settings.OutputDir
	}
	if strings.TrimSpace(state.Settings.OutputFormat) != "" {
		cfg.OutputFormat = state.Settings.OutputFormat
	}
	if strings.TrimSpace(state.Settings.ProxyMode) != "" {
		cfg.ProxyMode = state.Settings.ProxyMode
	}
	cfg.ProxyURL = state.Settings.ProxyURL
	if state.Settings.ProtectStreamPreview != nil {
		cfg.ProtectStreamPreview = *state.Settings.ProtectStreamPreview
	}
	if state.Settings.AutoRetryEnabled != nil {
		cfg.AutoRetryEnabled = *state.Settings.AutoRetryEnabled
	}
	if state.Settings.AutoRetryCount != nil {
		cfg.AutoRetryCount = *state.Settings.AutoRetryCount
	}

	profile, ok := ActiveProfile(state)
	if !ok {
		return cfg
	}
	cfg.BaseURL = profile.BaseURL
	cfg.TextModelID = profile.TextModelID
	cfg.ImageModelID = profile.ImageModelID
	cfg.APIMode = normaliseAPIMode(profile.APIMode)
	cfg.ResponsesTransport = client.ResponsesTransport(normalizeProfileResponsesTransport(profile.ResponsesTransport))
	cfg.FallbackProfileID = strings.TrimSpace(profile.FallbackProfileID)
	cfg.RequestPolicy = normalisePolicy(profile.RequestPolicy)
	cfg.ImagesNewAPICompat = profile.ImagesNewAPICompat
	cfg.ReasoningEffort = normalizeReasoningEffort(profile.ReasoningEffort)
	if strings.TrimSpace(state.Settings.Background) != "" {
		cfg.Background = state.Settings.Background
	}
	if state.Settings.OutputCompression != nil {
		cfg.OutputCompression = *state.Settings.OutputCompression
	}
	if strings.TrimSpace(state.Settings.InputFidelity) != "" {
		cfg.InputFidelity = state.Settings.InputFidelity
	}
	if strings.TrimSpace(state.Settings.ImageStyle) != "" {
		cfg.ImageStyle = state.Settings.ImageStyle
	}
	if strings.TrimSpace(state.Settings.Moderation) != "" {
		cfg.Moderation = state.Settings.Moderation
	}
	cfg.UserIdentifier = strings.TrimSpace(state.Settings.UserIdentifier)
	cfg.CompletionSound = NormaliseCompletionSoundSettings(state.Settings.CompletionSound)
	if state.Settings.PartialImages != nil {
		cfg.PartialImages = *state.Settings.PartialImages
	}
	cfg.APIKey, _ = ReadAPIKey(profile.ID)
	return cfg
}

func ActiveProfile(state shared.State) (shared.UpstreamProfile, bool) {
	if len(state.Profiles) == 0 {
		return shared.UpstreamProfile{}, false
	}
	if strings.TrimSpace(state.ActiveProfile) != "" {
		for _, profile := range state.Profiles {
			if profile.ID == state.ActiveProfile {
				return profile, true
			}
		}
	}
	return state.Profiles[0], true
}

func SaveConfigAndHistory(cfg kernel.Config, result kernel.Result, elapsedSec float64) error {
	return SaveConfigAndHistoryWithPreviewMode(cfg, result, elapsedSec, false)
}

func SaveConfigAndHistoryWithPreviewMode(cfg kernel.Config, result kernel.Result, elapsedSec float64, previewOnly bool) error {
	return UpdateState(func(state *shared.State) error {
		*state = UpsertConfig(*state, cfg)
		if strings.TrimSpace(result.SavedPath) != "" || strings.TrimSpace(result.ImageB64) != "" {
			item := HistoryItemFromRun(cfg, result, elapsedSec, previewOnly)
			state.History = mergeHistory(item, state.History)
			if previewOnly && strings.TrimSpace(result.ImageB64) != "" {
				state.HistoryFull = mergeHistoryFull(shared.HistoryFullItem{ID: item.ID, ImageB64: strings.TrimSpace(result.ImageB64)}, state.HistoryFull, state.History)
			} else {
				state.HistoryFull = pruneHistoryFull(state.HistoryFull, state.History)
			}
		}
		state.UpdatedAt = time.Now().UnixMilli()
		return nil
	})
}

func SaveConfig(cfg kernel.Config) error {
	return UpdateState(func(state *shared.State) error {
		*state = UpsertConfig(*state, cfg)
		state.UpdatedAt = time.Now().UnixMilli()
		return nil
	})
}

func SavePromptSuppressed(state shared.State) bool {
	return state.Settings.SavePromptSuppressed
}

func SetSavePromptSuppressed(value bool) error {
	return UpdateState(func(state *shared.State) error {
		state.Settings.SavePromptSuppressed = value
		state.UpdatedAt = time.Now().UnixMilli()
		return nil
	})
}

func RememberTrustedOutputRoot(state shared.State, root string) shared.State {
	state = shared.Normalize(state)
	root = strings.TrimSpace(root)
	if root == "" {
		return state
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	root = filepath.Clean(root)
	for _, existing := range state.Settings.TrustedOutputRoots {
		if filepath.Clean(strings.TrimSpace(existing)) == root {
			return state
		}
	}
	state.Settings.TrustedOutputRoots = append(state.Settings.TrustedOutputRoots, root)
	return state
}

func UpsertConfig(state shared.State, cfg kernel.Config) shared.State {
	state = shared.Normalize(state)
	now := time.Now().UnixMilli()
	profileID := strings.TrimSpace(state.ActiveProfile)
	profileIndex := -1
	if profileID != "" {
		for i := range state.Profiles {
			if state.Profiles[i].ID == profileID {
				profileIndex = i
				break
			}
		}
	}
	if profileIndex < 0 && len(state.Profiles) > 0 {
		profileIndex = 0
		profileID = state.Profiles[0].ID
	}
	if profileID == "" {
		profileID = "gio-" + randomID()
	}
	profile := shared.UpstreamProfile{
		ID:                 profileID,
		Name:               nextDefaultProfileName(state.Profiles),
		APIMode:            string(normaliseAPIMode(string(cfg.APIMode))),
		ResponsesTransport: normalizeProfileResponsesTransport(string(cfg.ResponsesTransport)),
		RequestPolicy:      string(normalisePolicy(string(cfg.RequestPolicy))),
		ImagesNewAPICompat: cfg.ImagesNewAPICompat,
		BaseURL:            strings.TrimSpace(cfg.BaseURL),
		TextModelID:        strings.TrimSpace(cfg.TextModelID),
		ImageModelID:       strings.TrimSpace(cfg.ImageModelID),
		ReasoningEffort:    normalizeReasoningEffort(cfg.ReasoningEffort),
		ConcurrencyLimit:   0,
		CreatedAt:          now,
		LastUsedAt:         now,
	}
	if profileIndex >= 0 {
		profile.Name = state.Profiles[profileIndex].Name
		profile.CreatedAt = state.Profiles[profileIndex].CreatedAt
		profile.ConcurrencyLimit = state.Profiles[profileIndex].ConcurrencyLimit
		profile.FallbackProfileID = state.Profiles[profileIndex].FallbackProfileID
		state.Profiles[profileIndex] = profile
	} else {
		state.Profiles = append(state.Profiles, profile)
	}
	state.ActiveProfile = profile.ID
	state.Settings.ProxyMode = cfg.ProxyMode
	state.Settings.ProxyURL = strings.TrimSpace(cfg.ProxyURL)
	protectStreamPreview := cfg.ProtectStreamPreview
	state.Settings.ProtectStreamPreview = &protectStreamPreview
	autoRetryEnabled := cfg.AutoRetryEnabled
	state.Settings.AutoRetryEnabled = &autoRetryEnabled
	autoRetryCount := cfg.AutoRetryCount
	state.Settings.AutoRetryCount = &autoRetryCount
	completionSound := cfg.CompletionSound
	state.Settings.CompletionSound = &completionSound
	state.Settings.OutputFormat = strings.TrimSpace(cfg.OutputFormat)
	state.Settings.OutputDir = strings.TrimSpace(cfg.OutputDir)
	state = RememberTrustedOutputRoot(state, state.Settings.OutputDir)
	state.Settings.Background = strings.TrimSpace(cfg.Background)
	state.Settings.InputFidelity = strings.TrimSpace(cfg.InputFidelity)
	state.Settings.ImageStyle = strings.TrimSpace(cfg.ImageStyle)
	state.Settings.Moderation = strings.TrimSpace(cfg.Moderation)
	state.Settings.UserIdentifier = strings.TrimSpace(cfg.UserIdentifier)
	outputCompression := cfg.OutputCompression
	state.Settings.OutputCompression = &outputCompression
	partialImages := cfg.PartialImages
	state.Settings.PartialImages = &partialImages
	if state.Settings.Theme == "" {
		state.Settings.Theme = "system"
	}
	if state.Settings.FontScale == 0 {
		state.Settings.FontScale = 1
	}
	if cfg.APIKey != "" {
		_ = WriteAPIKey(profile.ID, cfg.APIKey)
	}
	return state
}

func HistoryItemFromRun(cfg kernel.Config, result kernel.Result, elapsedSec float64, previewOnlyResult bool) shared.HistoryItem {
	item := shared.HistoryItem{
		ID:               randomID(),
		Prompt:           cfg.Prompt,
		RevisedPrompt:    result.RevisedPrompt,
		Mode:             string(cfg.Mode),
		Size:             cfg.Size,
		Quality:          cfg.Quality,
		OutputFormat:     cfg.OutputFormat,
		CreatedAt:        time.Now().UnixMilli(),
		Seed:             cfg.Seed,
		NegativePrompt:   cfg.NegativePrompt,
		Background:       cfg.Background,
		InputFidelity:    cfg.InputFidelity,
		ImageStyle:       cfg.ImageStyle,
		Moderation:       cfg.Moderation,
		StyleTag:         cfg.StyleTag,
		BatchIndex:       cfg.BatchIndex,
		PreviewSlotIndex: cfg.PreviewSlotIndex,
		ElapsedSec:       elapsedSec,
		ParentID:         strings.TrimSpace(cfg.ParentID),
		SourcePaths:      append([]string(nil), cfg.SourcePaths...),
		RawPath:          result.RawPath,
		PreviewOnly:      true,
	}
	if previewOnlyResult {
		item.SavedPath = result.SavedPath
		item.ImageB64 = ""
	} else {
		item.SavedPath = result.SavedPath
		item.PreviewPath = result.PreviewPath
		item.ThumbPath = result.ThumbPath
	}
	if cfg.OutputCompression > 0 {
		compression := cfg.OutputCompression
		item.OutputCompression = &compression
	}
	return item
}

func hydrateHistoryFull(state shared.State) shared.State {
	if len(state.HistoryFull) == 0 || len(state.History) == 0 {
		return state
	}
	fullByID := make(map[string]string, len(state.HistoryFull))
	for _, item := range state.HistoryFull {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.ImageB64) == "" {
			continue
		}
		fullByID[strings.TrimSpace(item.ID)] = strings.TrimSpace(item.ImageB64)
	}
	for idx := range state.History {
		if (strings.TrimSpace(state.History[idx].SavedPath) != "" && !isVirtualImagePath(state.History[idx].SavedPath)) || strings.TrimSpace(state.History[idx].ImageB64) != "" {
			continue
		}
		if imageB64 := fullByID[strings.TrimSpace(state.History[idx].ID)]; imageB64 != "" {
			state.History[idx].ImageB64 = imageB64
		}
	}
	return state
}

func dehydrateHistoryFull(state shared.State) shared.State {
	if len(state.HistoryFull) == 0 || len(state.History) == 0 {
		return state
	}
	fullIDs := make(map[string]struct{}, len(state.HistoryFull))
	for _, item := range state.HistoryFull {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.ImageB64) == "" {
			continue
		}
		fullIDs[strings.TrimSpace(item.ID)] = struct{}{}
	}
	for idx := range state.History {
		if strings.TrimSpace(state.History[idx].SavedPath) != "" && !isVirtualImagePath(state.History[idx].SavedPath) {
			continue
		}
		if _, ok := fullIDs[strings.TrimSpace(state.History[idx].ID)]; ok {
			state.History[idx].ImageB64 = ""
		}
	}
	return state
}

func mergeHistoryFull(item shared.HistoryFullItem, items []shared.HistoryFullItem, history []shared.HistoryItem) []shared.HistoryFullItem {
	item.ID = strings.TrimSpace(item.ID)
	item.ImageB64 = strings.TrimSpace(item.ImageB64)
	if item.ID == "" || item.ImageB64 == "" {
		return pruneHistoryFull(items, history)
	}
	next := make([]shared.HistoryFullItem, 0, len(items)+1)
	next = append(next, item)
	for _, existing := range items {
		existing.ID = strings.TrimSpace(existing.ID)
		existing.ImageB64 = strings.TrimSpace(existing.ImageB64)
		if existing.ID == "" || existing.ImageB64 == "" || existing.ID == item.ID {
			continue
		}
		next = append(next, existing)
	}
	return pruneHistoryFull(next, history)
}

func pruneHistoryFull(items []shared.HistoryFullItem, history []shared.HistoryItem) []shared.HistoryFullItem {
	if len(items) == 0 || len(history) == 0 {
		return nil
	}
	keepIDs := make(map[string]struct{}, len(history))
	for _, item := range history {
		id := strings.TrimSpace(item.ID)
		if id == "" || (strings.TrimSpace(item.SavedPath) != "" && !isVirtualImagePath(item.SavedPath)) {
			continue
		}
		keepIDs[id] = struct{}{}
	}
	next := make([]shared.HistoryFullItem, 0, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		imageB64 := strings.TrimSpace(item.ImageB64)
		if id == "" || imageB64 == "" {
			continue
		}
		if _, ok := keepIDs[id]; !ok {
			continue
		}
		next = append(next, shared.HistoryFullItem{ID: id, ImageB64: imageB64})
	}
	if len(next) == 0 {
		return nil
	}
	return next
}

func isVirtualImagePath(path string) bool {
	return strings.HasPrefix(strings.TrimSpace(path), "memory://image/")
}

func ReadAPIKey(profileID string) (string, error) {
	value, err := keyring.Get(keyringServiceName, "api-key:profile:"+profileID)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return value, err
}

func WriteAPIKey(profileID, value string) error {
	value = strings.TrimSpace(value)
	user := "api-key:profile:" + profileID
	if value == "" {
		err := keyring.Delete(keyringServiceName, user)
		if errors.Is(err, keyring.ErrNotFound) {
			return nil
		}
		return err
	}
	return keyring.Set(keyringServiceName, user, value)
}

func mergeHistory(item shared.HistoryItem, items []shared.HistoryItem) []shared.HistoryItem {
	out := make([]shared.HistoryItem, 0, min(len(items)+1, 120))
	seen := map[string]struct{}{item.ID: {}}
	out = append(out, item)
	for _, existing := range items {
		if existing.ID == "" {
			continue
		}
		if _, ok := seen[existing.ID]; ok {
			continue
		}
		seen[existing.ID] = struct{}{}
		out = append(out, existing)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	if len(out) > 120 {
		out = out[:120]
	}
	return out
}

func normaliseAPIMode(mode string) client.APIMode {
	if mode == string(client.APIModeImages) {
		return client.APIModeImages
	}
	return client.APIModeResponses
}

func normalisePolicy(policy string) client.RequestPolicy {
	if policy == string(client.RequestPolicyCompat) {
		return client.RequestPolicyCompat
	}
	return client.RequestPolicyOpenAI
}

func normalizeProfileResponsesTransport(value string) string {
	if strings.TrimSpace(value) == string(client.ResponsesTransportWebSocket) {
		return string(client.ResponsesTransportWebSocket)
	}
	return string(client.ResponsesTransportSSE)
}

func normalizeReasoningEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	default:
		return client.DefaultReasoningEffort
	}
}

func nextDefaultProfileName(profiles []shared.UpstreamProfile) string {
	used := map[int]struct{}{}
	for _, profile := range profiles {
		raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(profile.Name), "配置"))
		n, err := strconv.Atoi(raw)
		if err == nil && n > 0 {
			used[n] = struct{}{}
		}
	}
	for i := 1; ; i++ {
		if _, ok := used[i]; !ok {
			return "配置" + strconv.Itoa(i)
		}
	}
}

func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}
