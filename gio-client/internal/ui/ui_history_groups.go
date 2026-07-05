package ui

import (
	"strconv"
	"strings"

	sharedCompat "image-studio/shared/compat"
)

type historyPromptGroup struct {
	Key                   string
	Prompt                string
	Title                 string
	PromptPreview         string
	CountValue            string
	CountText             string
	Representative        sharedCompat.HistoryItem
	RepresentativeDisplay historyItemDisplay
	Items                 []*sharedCompat.HistoryItem
}

type historyPromptEntry struct {
	Kind  string
	Key   string
	Item  *sharedCompat.HistoryItem
	Group *historyPromptGroup
}

type historyDayGroup struct {
	Key     string
	Label   string
	Entries []historyPromptEntry
}

type historyPromptText struct {
	Normalized string
	Compact    string
}

const historyPromptGroupingCapacityMax = 128

func historyPromptGroupingCapacityHint(total int) int {
	if total <= 0 {
		return 0
	}
	return min(total, historyPromptGroupingCapacityMax)
}

func historyPromptTextCached(cache map[string]historyPromptText, prompt string) historyPromptText {
	if cache != nil {
		if text, ok := cache[prompt]; ok {
			return text
		}
	}
	compact := strings.Join(strings.Fields(strings.TrimSpace(prompt)), " ")
	text := historyPromptText{
		Normalized: strings.ToLower(compact),
		Compact:    compact,
	}
	if cache != nil {
		cache[prompt] = text
	}
	return text
}

func normalizeHistoryPrompt(prompt string) string {
	return historyPromptTextCached(nil, prompt).Normalized
}

func compactHistoryPrompt(prompt string) string {
	return historyPromptTextCached(nil, prompt).Compact
}

func newHistoryPromptGroup(key string, item *sharedCompat.HistoryItem) historyPromptGroup {
	return newHistoryPromptGroupWithText(key, compactHistoryPrompt(item.Prompt), item)
}

func newHistoryPromptGroupWithText(key string, compactPrompt string, item *sharedCompat.HistoryItem) historyPromptGroup {
	return historyPromptGroup{
		Key:            key,
		Prompt:         compactPrompt,
		Representative: *item,
		Items:          []*sharedCompat.HistoryItem{item},
	}
}

func finalizeHistoryPromptGroup(group *historyPromptGroup) {
	prompt := strings.TrimSpace(group.Prompt)
	group.Title = "(无 prompt)"
	group.PromptPreview = "(无 prompt)"
	if prompt != "" {
		group.Title = prompt
		group.PromptPreview = shortPrompt(prompt)
	}
	group.CountValue = strconv.Itoa(len(group.Items))
	group.CountText = group.CountValue + " 张"
	group.RepresentativeDisplay = buildHistoryItemDisplay(group.Representative)
}

func finalizeHistoryPromptGroups(groups []historyPromptGroup) {
	for idx := range groups {
		finalizeHistoryPromptGroup(&groups[idx])
	}
}

func buildHistoryPromptEntries(items []sharedCompat.HistoryItem) []historyPromptEntry {
	capHint := historyPromptGroupingCapacityHint(len(items))
	groups := make([]historyPromptGroup, 0, capHint)
	indexByKey := make(map[string]int, capHint)
	promptCache := make(map[string]historyPromptText, capHint)
	for idx := range items {
		item := &items[idx]
		promptText := historyPromptTextCached(promptCache, item.Prompt)
		key := "prompt:" + promptText.Normalized
		if idx, ok := indexByKey[key]; ok {
			groups[idx].Items = append(groups[idx].Items, item)
			continue
		}
		indexByKey[key] = len(groups)
		groups = append(groups, newHistoryPromptGroupWithText(key, promptText.Compact, item))
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
	return entries
}

func buildHistoryPromptEntriesLimited(items []sharedCompat.HistoryItem, limit int) []historyPromptEntry {
	if limit <= 0 || len(items) == 0 {
		return nil
	}
	if limit >= len(items) {
		return buildHistoryPromptEntries(items)
	}
	groups := make([]historyPromptGroup, 0, limit)
	indexByKey := make(map[string]int, limit)
	promptCache := make(map[string]historyPromptText, historyPromptGroupingCapacityHint(len(items)))
	for idx := range items {
		item := &items[idx]
		promptText := historyPromptTextCached(promptCache, item.Prompt)
		key := "prompt:" + promptText.Normalized
		if idx, ok := indexByKey[key]; ok {
			groups[idx].Items = append(groups[idx].Items, item)
			continue
		}
		if len(groups) >= limit {
			continue
		}
		indexByKey[key] = len(groups)
		groups = append(groups, newHistoryPromptGroupWithText(key, promptText.Compact, item))
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
	return entries
}

func buildHistoryDayGroups(items []sharedCompat.HistoryItem) []historyDayGroup {
	entries := buildHistoryPromptEntries(items)
	groups := make([]historyDayGroup, 0, len(entries))
	indexByKey := map[string]int{}
	for _, entry := range entries {
		representative := historyEntryRepresentative(entry)
		key := formatHistoryDay(representative.CreatedAt)
		if idx, ok := indexByKey[key]; ok {
			groups[idx].Entries = append(groups[idx].Entries, entry)
			continue
		}
		indexByKey[key] = len(groups)
		groups = append(groups, historyDayGroup{
			Key:     key,
			Label:   key,
			Entries: []historyPromptEntry{entry},
		})
	}
	return groups
}

func historyEntryRepresentative(entry historyPromptEntry) sharedCompat.HistoryItem {
	if entry.Kind == "group" && entry.Group != nil {
		return entry.Group.Representative
	}
	if entry.Item != nil {
		return *entry.Item
	}
	return sharedCompat.HistoryItem{}
}

func historyItemsByIDs(items []sharedCompat.HistoryItem, ids []string) []sharedCompat.HistoryItem {
	if len(ids) == 0 {
		return nil
	}
	index := make(map[string]sharedCompat.HistoryItem, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		index[item.ID] = item
	}
	out := make([]sharedCompat.HistoryItem, 0, len(ids))
	for _, id := range ids {
		if item, ok := index[strings.TrimSpace(id)]; ok {
			out = append(out, item)
		}
	}
	return out
}

func historyPromptGroupContains(group historyPromptGroup, itemID string) bool {
	if strings.TrimSpace(itemID) == "" {
		return false
	}
	for _, item := range group.Items {
		if item != nil && item.ID == itemID {
			return true
		}
	}
	return false
}

func promptGroupKeyForEntries(entries []historyPromptEntry, itemID string) string {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return ""
	}
	for _, entry := range entries {
		if entry.Kind == "group" {
			if entry.Group != nil && historyPromptGroupContains(*entry.Group, itemID) {
				return entry.Group.Key
			}
			continue
		}
		if entry.Item != nil && entry.Item.ID == itemID && entry.Group != nil {
			return entry.Group.Key
		}
	}
	return ""
}

func findPromptGroupForItem(items []sharedCompat.HistoryItem, itemID string) (historyPromptGroup, bool) {
	if strings.TrimSpace(itemID) == "" {
		return historyPromptGroup{}, false
	}
	for _, entry := range buildHistoryPromptEntries(items) {
		if entry.Group == nil {
			continue
		}
		group := *entry.Group
		if group.Key == "" && entry.Item != nil && entry.Item.ID != "" {
			group = newHistoryPromptGroup("prompt:"+normalizeHistoryPrompt(entry.Item.Prompt), entry.Item)
			finalizeHistoryPromptGroup(&group)
		}
		if historyPromptGroupContains(group, itemID) {
			return group, len(group.Items) > 0
		}
	}
	return historyPromptGroup{}, false
}
