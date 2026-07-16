package ui

import (
	"image/color"
	"path/filepath"
	"testing"

	sharedCompat "image-studio/shared/compat"
)

func TestOrderedNavigationItemsForCurrentPrefersBatchResults(t *testing.T) {
	items := []sharedCompat.HistoryItem{
		{ID: "b", CreatedAt: 20},
		{ID: "a", CreatedAt: 10},
	}
	history := []sharedCompat.HistoryItem{
		{ID: "h2", CreatedAt: 40},
		{ID: "h1", CreatedAt: 30},
	}

	got := orderedNavigationItemsForCurrent("b", history, items)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("ordered batch navigation = %#v", got)
	}
}

func TestOrderedNavigationItemsForCurrentFallsBackToHistory(t *testing.T) {
	history := []sharedCompat.HistoryItem{
		{ID: "h2", CreatedAt: 40},
		{ID: "h1", CreatedAt: 30},
	}

	got := orderedNavigationItemsForCurrent("h2", history, nil)
	if len(got) != 2 || got[0].ID != "h1" || got[1].ID != "h2" {
		t.Fatalf("ordered history navigation = %#v", got)
	}
}

func TestStepBatchResultCyclesCurrentBatchResults(t *testing.T) {
	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.png")
	pathB := filepath.Join(dir, "b.png")
	pathC := filepath.Join(dir, "c.png")
	writeSolidTestPNG(t, pathA, color.NRGBA{R: 0xaa, G: 0x11, B: 0x11, A: 0xff})
	writeSolidTestPNG(t, pathB, color.NRGBA{R: 0x11, G: 0xaa, B: 0x11, A: 0xff})
	writeSolidTestPNG(t, pathC, color.NRGBA{R: 0x11, G: 0x11, B: 0xaa, A: 0xff})

	app := &App{
		history: []sharedCompat.HistoryItem{
			{ID: "c", Prompt: "c", SavedPath: pathC, CreatedAt: 30},
			{ID: "a", Prompt: "a", SavedPath: pathA, CreatedAt: 10},
			{ID: "b", Prompt: "b", SavedPath: pathB, CreatedAt: 20},
		},
		batchResultIDs: []string{"c", "a", "b"},
		result: resultState{
			Item: sharedCompat.HistoryItem{ID: "b", Prompt: "b", SavedPath: pathB, CreatedAt: 20},
		},
	}

	if err := app.stepBatchResult(1); err != nil {
		t.Fatalf("stepBatchResult(+1): %v", err)
	}
	if app.result.Item.ID != "c" || app.selectedHistoryID != "c" {
		t.Fatalf("after +1 result=%q selected=%q want c", app.result.Item.ID, app.selectedHistoryID)
	}
	if err := app.stepBatchResult(1); err != nil {
		t.Fatalf("stepBatchResult(+1 wrap): %v", err)
	}
	if app.result.Item.ID != "a" || app.selectedHistoryID != "a" {
		t.Fatalf("after wrap result=%q selected=%q want a", app.result.Item.ID, app.selectedHistoryID)
	}
	if err := app.stepBatchResult(-1); err != nil {
		t.Fatalf("stepBatchResult(-1): %v", err)
	}
	if app.result.Item.ID != "c" || app.selectedHistoryID != "c" {
		t.Fatalf("after -1 result=%q selected=%q want c", app.result.Item.ID, app.selectedHistoryID)
	}
}
