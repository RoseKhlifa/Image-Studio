package ui

import (
	"testing"

	sharedCompat "image-studio/shared/compat"
)

func TestPruneWorkspaceHistoryReferencesKeepsLivePreviewAndDropsRemovedResults(t *testing.T) {
	kept := map[string]struct{}{"keep": {}}
	removed := map[string]struct{}{"drop": {}}
	ws := workspaceState{
		ResultItem:        sharedCompat.HistoryItem{ID: "keep", SavedPath: "/tmp/keep.png"},
		ResultHasItem:     true,
		SelectedHistoryID: "drop",
		CompareHistoryID:  "drop",
		CompareSplit:      0.2,
		BatchResultIDs:    []string{"keep", "drop"},
		BatchPreviewItems: []sharedCompat.HistoryItem{{ID: "drop"}, {ID: "preview-live", PreviewOnly: true}},
		ResultGridOpen:    true,
	}

	got := pruneWorkspaceHistoryReferences(ws, kept, removed)
	if !got.ResultHasItem || got.ResultItem.ID != "keep" {
		t.Fatalf("kept workspace result=%#v hasItem=%v", got.ResultItem, got.ResultHasItem)
	}
	if got.SelectedHistoryID != "" || got.CompareHistoryID != "" || got.CompareSplit != 0.5 {
		t.Fatalf("stale selection/compare survived: selected=%q compare=%q split=%v", got.SelectedHistoryID, got.CompareHistoryID, got.CompareSplit)
	}
	if len(got.BatchResultIDs) != 1 || got.BatchResultIDs[0] != "keep" {
		t.Fatalf("batch result ids=%v want [keep]", got.BatchResultIDs)
	}
	if len(got.BatchPreviewItems) != 1 || got.BatchPreviewItems[0].ID != "preview-live" {
		t.Fatalf("batch previews=%v want live preview only", got.BatchPreviewItems)
	}
	if !got.ResultGridOpen {
		t.Fatal("result grid should stay open for one result plus one live preview")
	}

	cleared := pruneWorkspaceHistoryReferences(workspaceState{
		ResultSavedPath:     "/tmp/drop.png",
		ResultRawPath:       "/tmp/drop.txt",
		ResultRevisedPrompt: "drop",
		ResultSourceEvent:   "event",
		ResultItem:          sharedCompat.HistoryItem{ID: "drop"},
		ResultHasItem:       true,
	}, kept, removed)
	if cleared.ResultHasItem || cleared.ResultItem.ID != "" || cleared.ResultSavedPath != "" || cleared.ResultRawPath != "" {
		t.Fatalf("removed workspace result survived: %#v", cleared)
	}
}

func TestReplaceHistoryStateClearsCrossWorkspaceAndModalReferences(t *testing.T) {
	drop := sharedCompat.HistoryItem{ID: "drop", SavedPath: "/tmp/drop.png"}
	app := &App{
		history:                  []sharedCompat.HistoryItem{drop},
		batchResultIDs:           []string{"drop"},
		selectedHistoryID:        "drop",
		compare:                  resultState{Item: drop, HasItem: true},
		activeResultDetail:       drop,
		result:                   resultState{Item: drop, HasItem: true, SavedPath: drop.SavedPath},
		activePromptGroup:        historyPromptGroup{Key: "drop", Items: []*sharedCompat.HistoryItem{&drop}},
		savePromptVisible:        true,
		savePromptBatchItems:     []sharedCompat.HistoryItem{drop},
		savePromptBatchSelection: map[string]bool{"drop": true},
		historyActionMenuItem:    drop,
		historyActionMenuContext: "rail",
		workspaces: []workspaceState{{
			ID:                "ws-2",
			ResultSavedPath:   drop.SavedPath,
			ResultItem:        drop,
			ResultHasItem:     true,
			SelectedHistoryID: "drop",
			CompareHistoryID:  "drop",
			BatchResultIDs:    []string{"drop"},
			ResultGridOpen:    true,
		}},
	}

	app.replaceHistoryState(nil, "")

	if len(app.history) != 0 || len(app.batchResultIDs) != 0 || app.selectedHistoryID != "" {
		t.Fatalf("active history refs survived: history=%v batch=%v selected=%q", app.history, app.batchResultIDs, app.selectedHistoryID)
	}
	if app.result.HasItem || app.compare.HasItem || app.activeResultDetail.ID != "" || app.activePromptGroup.Key != "" {
		t.Fatalf("active result refs survived: result=%#v compare=%#v detail=%#v group=%#v", app.result, app.compare, app.activeResultDetail, app.activePromptGroup)
	}
	if app.savePromptVisible || len(app.savePromptBatchItems) != 0 || len(app.savePromptBatchSelection) != 0 {
		t.Fatalf("save prompt refs survived: visible=%v items=%v selection=%v", app.savePromptVisible, app.savePromptBatchItems, app.savePromptBatchSelection)
	}
	if app.historyActionMenuItem.ID != "" || app.historyActionMenuContext != "" {
		t.Fatalf("history action menu survived: item=%#v context=%q", app.historyActionMenuItem, app.historyActionMenuContext)
	}
	if len(app.workspaces) != 1 || app.workspaces[0].ResultHasItem || len(app.workspaces[0].BatchResultIDs) != 0 || app.workspaces[0].ResultGridOpen {
		t.Fatalf("workspace refs survived: %#v", app.workspaces)
	}
}
