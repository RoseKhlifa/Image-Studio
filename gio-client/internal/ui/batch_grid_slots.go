package ui

import (
	"sort"

	sharedCompat "image-studio/shared/compat"
)

type batchGridSlotKind int

const (
	batchGridSlotPending batchGridSlotKind = iota
	batchGridSlotResult
	batchGridSlotPreview
)

type batchGridSlot struct {
	Index int
	Kind  batchGridSlotKind
	Item  sharedCompat.HistoryItem
}

func orderedBatchPreviewItems(previews map[int]sharedCompat.HistoryItem) []sharedCompat.HistoryItem {
	if len(previews) == 0 {
		return nil
	}
	keys := make([]int, 0, len(previews))
	for index := range previews {
		keys = append(keys, index)
	}
	sort.Ints(keys)
	items := make([]sharedCompat.HistoryItem, 0, len(keys))
	for _, index := range keys {
		item := previews[index]
		if item.BatchIndex < 0 {
			item.BatchIndex = index
		}
		items = append(items, item)
	}
	return items
}

func batchPreviewItemsMap(items []sharedCompat.HistoryItem) map[int]sharedCompat.HistoryItem {
	if len(items) == 0 {
		return nil
	}
	previews := make(map[int]sharedCompat.HistoryItem, len(items))
	for index, item := range items {
		slotIndex := item.BatchIndex
		if slotIndex < 0 {
			slotIndex = index
			item.BatchIndex = slotIndex
		}
		previews[slotIndex] = item
	}
	return previews
}

func batchGridTotalSlots(results []sharedCompat.HistoryItem, previews []sharedCompat.HistoryItem, runningTotal int) int {
	total := max(0, runningTotal)
	for idx, item := range results {
		slotIndex := item.BatchIndex
		if slotIndex < 0 {
			slotIndex = idx
		}
		if slotIndex+1 > total {
			total = slotIndex + 1
		}
	}
	for idx, item := range previews {
		slotIndex := item.BatchIndex
		if slotIndex < 0 {
			slotIndex = idx
		}
		if slotIndex+1 > total {
			total = slotIndex + 1
		}
	}
	if total == 0 {
		if len(results) > total {
			total = len(results)
		}
		if len(previews) > total {
			total = len(previews)
		}
	}
	return total
}

func batchGridItemSlotIndex(item sharedCompat.HistoryItem, fallback int, preferPreviewSlot bool) int {
	if preferPreviewSlot {
		return max(0, item.PreviewSlotIndex)
	}
	if item.BatchIndex >= 0 {
		return item.BatchIndex
	}
	return fallback
}

func buildBatchGridSlots(results []sharedCompat.HistoryItem, previews []sharedCompat.HistoryItem, runningTotal int, preferPreviewSlot bool) []batchGridSlot {
	total := runningTotal
	if !(preferPreviewSlot && total > 0) {
		total = batchGridTotalSlots(results, previews, runningTotal)
	}
	if total == 0 {
		return nil
	}
	slots := make([]batchGridSlot, total)
	occupied := make([]bool, total)
	for i := range slots {
		slots[i] = batchGridSlot{Index: i, Kind: batchGridSlotPending}
	}
	place := func(item sharedCompat.HistoryItem, kind batchGridSlotKind, fallback int) {
		slotIndex := batchGridItemSlotIndex(item, fallback, preferPreviewSlot)
		if slotIndex < 0 || slotIndex >= total {
			slotIndex = nextPendingBatchGridSlot(occupied)
			if slotIndex < 0 {
				return
			}
		} else if occupied[slotIndex] {
			return
		}
		item.BatchIndex = slotIndex
		if preferPreviewSlot {
			item.PreviewSlotIndex = slotIndex
		}
		slots[slotIndex] = batchGridSlot{
			Index: slotIndex,
			Kind:  kind,
			Item:  item,
		}
		occupied[slotIndex] = true
	}
	for idx := len(results) - 1; idx >= 0; idx-- {
		place(results[idx], batchGridSlotResult, idx)
	}
	for idx := len(previews) - 1; idx >= 0; idx-- {
		place(previews[idx], batchGridSlotPreview, idx)
	}
	return slots
}

func nextPendingBatchGridSlot(occupied []bool) int {
	for index, used := range occupied {
		if !used {
			return index
		}
	}
	return -1
}
