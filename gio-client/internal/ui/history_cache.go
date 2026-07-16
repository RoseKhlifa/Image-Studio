package ui

import (
	"strconv"
	"strings"
	"time"

	sharedCompat "image-studio/shared/compat"
)

type historyPanelData struct {
	filteredCount int
	entries       []historyPromptEntry
	latest        sharedCompat.HistoryItem
	hasLatest     bool
	generateCount int
	editCount     int
}

type historyPanelCache struct {
	rev  int
	key  string
	data historyPanelData
}

type historyTimelineData struct {
	filteredCount    int
	dayGroups        []historyDayGroup
	selectedGroupKey string
}

type historyTimelineCache struct {
	rev  int
	key  string
	data historyTimelineData
}

func historyItemDisplayCacheKey(item sharedCompat.HistoryItem) string {
	if id := strings.TrimSpace(item.ID); id != "" {
		return "id:" + id
	}
	parts := []string{
		strings.TrimSpace(item.SavedPath),
		strings.TrimSpace(item.RawPath),
		strconv.FormatInt(item.CreatedAt, 10),
		strings.TrimSpace(item.Prompt),
		strings.TrimSpace(item.RevisedPrompt),
		strings.TrimSpace(item.Mode),
		strings.TrimSpace(item.Size),
		strings.TrimSpace(item.Quality),
		strings.TrimSpace(item.StyleTag),
		detailValue(item.Seed),
		detailValue(item.ElapsedSec),
		strings.TrimSpace(item.OutputFormat),
	}
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	var b strings.Builder
	b.Grow(total + len(parts) - 1)
	for i, part := range parts {
		if i > 0 {
			b.WriteByte('\x00')
		}
		b.WriteString(part)
	}
	return b.String()
}

func buildHistoryItemDisplay(item sharedCompat.HistoryItem) historyItemDisplay {
	sizeLabel := sizeDisplayLabel(item.Size)
	qualityLabel := qualityDisplayLabel(item.Quality)
	styleLabel := ""
	if strings.TrimSpace(item.StyleTag) != "" {
		styleLabel = "#" + styleChoiceLabel(item.StyleTag)
	}
	modeLabel := "文生图"
	if item.Mode == "edit" {
		modeLabel = "图生图"
	}
	formatLabel := strings.ToUpper(strings.TrimSpace(item.OutputFormat))
	metaBadges := make([]string, 0, 3)
	if size := strings.TrimSpace(sizeLabel); size != "" {
		metaBadges = append(metaBadges, size)
	}
	if quality := strings.TrimSpace(qualityLabel); quality != "" {
		metaBadges = append(metaBadges, quality)
	}
	if styleLabel != "" {
		metaBadges = append(metaBadges, styleLabel)
	}
	statusBadges := make([]string, 0, 5)
	statusBadges = append(statusBadges, metaBadges...)
	if item.ElapsedSec > 0 {
		statusBadges = append(statusBadges, detailValue(item.ElapsedSec)+"s")
	}
	if item.Seed != 0 {
		statusBadges = append(statusBadges, "seed "+detailValue(item.Seed))
	}
	return historyItemDisplay{
		ShortPrompt:      shortPrompt(item.Prompt),
		MetaBadges:       metaBadges,
		StatusMetaBadges: statusBadges,
		Clock:            formatHistoryClock(item.CreatedAt),
		ClockPrecise:     formatHistoryClockPrecise(item.CreatedAt),
		RailMetaText:     joinHistoryMetaParts(sizeLabel, qualityLabel, styleLabel),
		MetaText:         joinHistoryMetaParts(modeLabel, sizeLabel, qualityLabel, styleLabel, formatLabel),
	}
}

func (a *App) historyItemDisplay(item sharedCompat.HistoryItem) historyItemDisplay {
	key := historyItemDisplayCacheKey(item)
	a.mu.Lock()
	if a.historyItemDisplayCache.rev != a.historyRev {
		a.historyItemDisplayCache = historyItemDisplayCache{
			rev:   a.historyRev,
			items: map[string]historyItemDisplay{},
		}
	}
	if display, ok := a.historyItemDisplayCache.items[key]; ok {
		a.mu.Unlock()
		return display
	}
	a.mu.Unlock()

	display := buildHistoryItemDisplay(item)

	a.mu.Lock()
	if a.historyItemDisplayCache.rev != a.historyRev {
		a.historyItemDisplayCache = historyItemDisplayCache{
			rev:   a.historyRev,
			items: map[string]historyItemDisplay{},
		}
	}
	if existing, ok := a.historyItemDisplayCache.items[key]; ok {
		a.mu.Unlock()
		return existing
	}
	a.historyItemDisplayCache.items[key] = display
	a.mu.Unlock()
	return display
}

func historyFilterCacheKey(query string, modeFilter string, dateFilter string, pickedDate string, now time.Time) string {
	query = normalizeHistorySearchQuery(strings.Join(strings.Fields(query), " "))
	return query + "\x00" + strings.TrimSpace(modeFilter) + "\x00" + strings.TrimSpace(dateFilter) + "\x00" + strings.TrimSpace(pickedDate) + "\x00" + now.Format("2006-01-02")
}

func (a *App) historyPanelData(items []sharedCompat.HistoryItem) historyPanelData {
	now := time.Now()
	key := historyFilterCacheKey(a.historyQueryInput.Text(), a.historyModeFilter, a.historyDateFilter, "", now)

	a.mu.Lock()
	cached := a.historyPanelCache
	rev := a.historyRev
	a.mu.Unlock()
	if cached.rev == rev && cached.key == key {
		return cached.data
	}

	query := normalizeHistorySearchQuery(a.historyQueryInput.Text())
	modeFilter := strings.TrimSpace(a.historyModeFilter)
	dateFilter := strings.TrimSpace(a.historyDateFilter)
	dateKind, dateStart, dateEnd := prepareHistoryDateFilter(dateFilter, "", now)
	filteredCount := 0
	hasLatest := false
	latest := sharedCompat.HistoryItem{}
	generateCount := 0
	editCount := 0
	groups := make([]historyPromptGroup, 0, 18)
	indexByKey := make(map[string]int, 18)
	promptCache := make(map[string]historyPromptText, historyPromptGroupingCapacityHint(len(items)))
	for itemIdx := range items {
		item := &items[itemIdx]
		switch item.Mode {
		case "generate":
			generateCount++
		case "edit":
			editCount++
		}
		if modeFilter != "" && modeFilter != "all" && item.Mode != modeFilter {
			continue
		}
		if !matchHistoryDatePrepared(item.CreatedAt, dateKind, dateStart, dateEnd) {
			continue
		}
		if !matchHistoryQueryNormalized(*item, query) {
			continue
		}
		filteredCount++
		if !hasLatest {
			latest = *item
			hasLatest = true
		}
		promptText := historyPromptTextCached(promptCache, item.Prompt)
		groupKey := "prompt:" + promptText.Normalized
		if idx, ok := indexByKey[groupKey]; ok {
			groups[idx].Items = append(groups[idx].Items, item)
			continue
		}
		if len(groups) >= 18 {
			continue
		}
		indexByKey[groupKey] = len(groups)
		groups = append(groups, newHistoryPromptGroupWithText(groupKey, promptText.Compact, item))
	}
	finalizeHistoryPromptGroups(groups)
	entries := make([]historyPromptEntry, 0, len(groups))
	for idx := range groups {
		group := &groups[idx]
		if len(group.Items) > 1 {
			entries = append(entries, historyPromptEntry{
				Kind:  "group",
				Group: group,
			})
			continue
		}
		entries = append(entries, historyPromptEntry{
			Kind:  "item",
			Item:  group.Items[0],
			Group: group,
		})
	}
	data := historyPanelData{
		filteredCount: filteredCount,
		entries:       entries,
		latest:        latest,
		hasLatest:     hasLatest,
		generateCount: generateCount,
		editCount:     editCount,
	}

	a.mu.Lock()
	if a.historyRev == rev {
		a.historyPanelCache = historyPanelCache{rev: rev, key: key, data: data}
	}
	a.mu.Unlock()
	return data
}

func (a *App) historyTimelineData(items []sharedCompat.HistoryItem) historyTimelineData {
	now := time.Now()
	pickedDate := strings.TrimSpace(a.historyTimelinePickedDateInput.Text())
	key := historyFilterCacheKey(a.historyTimelineQueryInput.Text(), a.historyTimelineModeFilter, a.historyTimelineDateFilter, pickedDate, now)

	a.mu.Lock()
	cached := a.historyTimelineCache
	rev := a.historyRev
	selectedHistoryID := strings.TrimSpace(a.selectedHistoryID)
	a.mu.Unlock()
	if cached.rev == rev && cached.key == key {
		return cached.data
	}

	query := normalizeHistorySearchQuery(a.historyTimelineQueryInput.Text())
	modeFilter := strings.TrimSpace(a.historyTimelineModeFilter)
	dateFilter := strings.TrimSpace(a.historyTimelineDateFilter)
	dateKind, dateStart, dateEnd := prepareHistoryDateFilter(dateFilter, pickedDate, now)
	capHint := historyPromptGroupingCapacityHint(len(items))
	groups := make([]historyPromptGroup, 0, capHint)
	indexByKey := make(map[string]int, capHint)
	promptCache := make(map[string]historyPromptText, capHint)
	filteredCount := 0
	selectedGroupKey := ""
	for itemIdx := range items {
		item := &items[itemIdx]
		if modeFilter != "" && modeFilter != "all" && item.Mode != modeFilter {
			continue
		}
		if !matchHistoryDatePrepared(item.CreatedAt, dateKind, dateStart, dateEnd) {
			continue
		}
		if !matchHistoryQueryNormalized(*item, query) {
			continue
		}
		filteredCount++
		promptText := historyPromptTextCached(promptCache, item.Prompt)
		groupKey := "prompt:" + promptText.Normalized
		if idx, ok := indexByKey[groupKey]; ok {
			groups[idx].Items = append(groups[idx].Items, item)
			if selectedGroupKey == "" && strings.TrimSpace(item.ID) == selectedHistoryID {
				selectedGroupKey = groupKey
			}
			continue
		}
		indexByKey[groupKey] = len(groups)
		groups = append(groups, newHistoryPromptGroupWithText(groupKey, promptText.Compact, item))
		if selectedGroupKey == "" && strings.TrimSpace(item.ID) == selectedHistoryID {
			selectedGroupKey = groupKey
		}
	}
	finalizeHistoryPromptGroups(groups)
	dayGroups := make([]historyDayGroup, 0, len(groups))
	dayIndexByKey := map[string]int{}
	for idx := range groups {
		group := &groups[idx]
		entry := historyPromptEntry{
			Kind:  "item",
			Item:  group.Items[0],
			Group: group,
		}
		if len(group.Items) > 1 {
			entry.Kind = "group"
		}
		dayKey := formatHistoryDay(group.Representative.CreatedAt)
		if idx, ok := dayIndexByKey[dayKey]; ok {
			dayGroups[idx].Entries = append(dayGroups[idx].Entries, entry)
			continue
		}
		dayIndexByKey[dayKey] = len(dayGroups)
		dayGroups = append(dayGroups, historyDayGroup{
			Key:     dayKey,
			Label:   dayKey,
			Entries: []historyPromptEntry{entry},
		})
	}
	data := historyTimelineData{
		filteredCount:    filteredCount,
		dayGroups:        dayGroups,
		selectedGroupKey: selectedGroupKey,
	}

	a.mu.Lock()
	if a.historyRev == rev {
		a.historyTimelineCache = historyTimelineCache{rev: rev, key: key, data: data}
	}
	a.mu.Unlock()
	return data
}

func (a *App) promptGroupForHistoryItem(items []sharedCompat.HistoryItem, itemID string) (historyPromptGroup, bool) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return historyPromptGroup{}, false
	}

	a.mu.Lock()
	cached := a.historyGroupLookup
	rev := a.historyRev
	a.mu.Unlock()
	if cached.rev == rev {
		if idx, ok := cached.index[itemID]; ok && idx >= 0 && idx < len(cached.groups) {
			return cached.groups[idx], true
		}
		return historyPromptGroup{}, false
	}

	capHint := historyPromptGroupingCapacityHint(len(items))
	groups := make([]historyPromptGroup, 0, capHint)
	indexByPrompt := make(map[string]int, capHint)
	indexByItemID := make(map[string]int, capHint)
	promptCache := make(map[string]historyPromptText, capHint)
	for itemIdx := range items {
		item := &items[itemIdx]
		promptText := historyPromptTextCached(promptCache, item.Prompt)
		key := "prompt:" + promptText.Normalized
		if idx, ok := indexByPrompt[key]; ok {
			groups[idx].Items = append(groups[idx].Items, item)
			if id := strings.TrimSpace(item.ID); id != "" {
				indexByItemID[id] = idx
			}
			continue
		}
		indexByPrompt[key] = len(groups)
		groups = append(groups, newHistoryPromptGroupWithText(key, promptText.Compact, item))
		if id := strings.TrimSpace(item.ID); id != "" {
			indexByItemID[id] = len(groups) - 1
		}
	}
	finalizeHistoryPromptGroups(groups)

	a.mu.Lock()
	if a.historyRev == rev {
		a.historyGroupLookup = historyGroupLookupCache{
			rev:    rev,
			groups: groups,
			index:  indexByItemID,
		}
	}
	a.mu.Unlock()

	if idx, ok := indexByItemID[itemID]; ok && idx >= 0 && idx < len(groups) {
		return groups[idx], true
	}
	return historyPromptGroup{}, false
}

func (a *App) promptGroupKeyForHistoryItem(items []sharedCompat.HistoryItem, itemID string) string {
	group, ok := a.promptGroupForHistoryItem(items, itemID)
	if !ok {
		return ""
	}
	return group.Key
}
