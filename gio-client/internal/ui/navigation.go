package ui

import (
	"sort"
	"strings"

	sharedCompat "image-studio/shared/compat"
)

func sortHistoryItemsByCreatedAtAsc(items []sharedCompat.HistoryItem) []sharedCompat.HistoryItem {
	if len(items) == 0 {
		return nil
	}
	ordered := append([]sharedCompat.HistoryItem(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].CreatedAt == ordered[j].CreatedAt {
			return strings.TrimSpace(ordered[i].ID) < strings.TrimSpace(ordered[j].ID)
		}
		return ordered[i].CreatedAt < ordered[j].CreatedAt
	})
	return ordered
}

func historyItemsContainID(items []sharedCompat.HistoryItem, itemID string) bool {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return false
	}
	for _, item := range items {
		if strings.TrimSpace(item.ID) == itemID {
			return true
		}
	}
	return false
}

func historyItemIndexByID(items []sharedCompat.HistoryItem, itemID string) int {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return -1
	}
	for idx, item := range items {
		if strings.TrimSpace(item.ID) == itemID {
			return idx
		}
	}
	return -1
}

func orderedNavigationItemsForCurrent(currentID string, historyItems []sharedCompat.HistoryItem, batchItems []sharedCompat.HistoryItem) []sharedCompat.HistoryItem {
	orderedBatchItems := sortHistoryItemsByCreatedAtAsc(batchItems)
	if historyItemsContainID(orderedBatchItems, currentID) {
		return orderedBatchItems
	}

	orderedHistoryItems := sortHistoryItemsByCreatedAtAsc(historyItems)
	if historyItemsContainID(orderedHistoryItems, currentID) {
		return orderedHistoryItems
	}

	if len(orderedBatchItems) > 1 {
		return orderedBatchItems
	}
	return orderedHistoryItems
}

func canStepBatchResultSnapshot(snap snapshot) bool {
	if snap.ResultGridOpen {
		return false
	}
	currentID := strings.TrimSpace(snap.Result.Item.ID)
	if currentID == "" {
		return false
	}
	navigationItems := orderedNavigationItemsForCurrent(currentID, snap.History, snap.BatchResults)
	if len(navigationItems) <= 1 {
		return false
	}
	return historyItemIndexByID(navigationItems, currentID) >= 0
}

func (a *App) stepBatchResult(delta int) error {
	if delta != -1 && delta != 1 {
		return nil
	}
	snap := a.readSnapshot()
	if !canStepBatchResultSnapshot(snap) {
		return nil
	}
	currentID := strings.TrimSpace(snap.Result.Item.ID)
	navigationItems := orderedNavigationItemsForCurrent(currentID, snap.History, snap.BatchResults)
	currentIndex := historyItemIndexByID(navigationItems, currentID)
	if currentIndex < 0 {
		return nil
	}
	nextIndex := (currentIndex + delta + len(navigationItems)) % len(navigationItems)
	nextItem := navigationItems[nextIndex]
	return a.loadHistoryPreview(nextItem, true)
}
