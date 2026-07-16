package ui

import (
	"testing"

	sharedCompat "image-studio/shared/compat"
)

func TestBuildBatchGridSlotsUsesBatchIndexAndKeepsPreviewFallbacks(t *testing.T) {
	results := []sharedCompat.HistoryItem{
		{ID: "result-2", BatchIndex: 2},
		{ID: "result-0", BatchIndex: 0},
	}
	previews := []sharedCompat.HistoryItem{
		{ID: "preview-1", BatchIndex: 1, PreviewOnly: true},
		{ID: "preview-3", BatchIndex: 3, PreviewOnly: true},
	}

	slots := buildBatchGridSlots(results, previews, 4, false)
	if len(slots) != 4 {
		t.Fatalf("len(slots)=%d want 4", len(slots))
	}
	if slots[0].Kind != batchGridSlotResult || slots[0].Item.ID != "result-0" {
		t.Fatalf("slot0=%#v want result-0", slots[0])
	}
	if slots[1].Kind != batchGridSlotPreview || slots[1].Item.ID != "preview-1" {
		t.Fatalf("slot1=%#v want preview-1", slots[1])
	}
	if slots[2].Kind != batchGridSlotResult || slots[2].Item.ID != "result-2" {
		t.Fatalf("slot2=%#v want result-2", slots[2])
	}
	if slots[3].Kind != batchGridSlotPreview || slots[3].Item.ID != "preview-3" {
		t.Fatalf("slot3=%#v want preview-3", slots[3])
	}
}

func TestBuildBatchGridSlotsLetsResultOverridePreviewInSameSlot(t *testing.T) {
	results := []sharedCompat.HistoryItem{
		{ID: "result-1", BatchIndex: 1},
	}
	previews := []sharedCompat.HistoryItem{
		{ID: "preview-1", BatchIndex: 1, PreviewOnly: true},
	}

	slots := buildBatchGridSlots(results, previews, 2, false)
	if len(slots) != 2 {
		t.Fatalf("len(slots)=%d want 2", len(slots))
	}
	if slots[1].Kind != batchGridSlotResult || slots[1].Item.ID != "result-1" {
		t.Fatalf("slot1=%#v want result-1", slots[1])
	}
}

func TestBuildBatchGridSlotsPrefersPreviewSlotIndexWhileRunning(t *testing.T) {
	results := []sharedCompat.HistoryItem{
		{ID: "result-a", BatchIndex: 4, PreviewSlotIndex: 1},
	}
	previews := []sharedCompat.HistoryItem{
		{ID: "preview-b", BatchIndex: 5, PreviewSlotIndex: 0, PreviewOnly: true},
	}

	slots := buildBatchGridSlots(results, previews, 2, true)
	if len(slots) != 2 {
		t.Fatalf("len(slots)=%d want 2", len(slots))
	}
	if slots[0].Kind != batchGridSlotPreview || slots[0].Item.ID != "preview-b" {
		t.Fatalf("slot0=%#v want preview-b", slots[0])
	}
	if slots[1].Kind != batchGridSlotResult || slots[1].Item.ID != "result-a" {
		t.Fatalf("slot1=%#v want result-a", slots[1])
	}
}
