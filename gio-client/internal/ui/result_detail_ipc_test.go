package ui

import (
	"testing"

	sharedCompat "image-studio/shared/compat"
)

func TestOpenResultDetailByIDOrSavedPathFindsHistoryItem(t *testing.T) {
	want := sharedCompat.HistoryItem{
		ID:        "history-1",
		SavedPath: "/tmp/history-1.png",
		Prompt:    "cat poster",
	}
	app := &App{
		history: []sharedCompat.HistoryItem{want},
	}
	if ok := app.OpenResultDetailByIDOrSavedPath("history-1", ""); !ok {
		t.Fatal("expected history item to open by id")
	}
	if app.activeResultDetail.ID != "history-1" {
		t.Fatalf("active result detail id=%q want history-1", app.activeResultDetail.ID)
	}

	app.closeResultDetail()
	if ok := app.OpenResultDetailByIDOrSavedPath("", "/tmp/history-1.png"); !ok {
		t.Fatal("expected history item to open by saved path")
	}
	if app.activeResultDetail.SavedPath != "/tmp/history-1.png" {
		t.Fatalf("active result detail path=%q want /tmp/history-1.png", app.activeResultDetail.SavedPath)
	}
}

func TestOpenResultDetailByIDOrSavedPathFallsBackToCurrentResult(t *testing.T) {
	current := sharedCompat.HistoryItem{
		ID:        "current-1",
		SavedPath: "/tmp/current-1.png",
		Prompt:    "dog poster",
	}
	app := &App{
		result: resultState{
			Item: current,
		},
	}
	if ok := app.OpenResultDetailByIDOrSavedPath("current-1", ""); !ok {
		t.Fatal("expected current result to open by id")
	}
	if app.activeResultDetail.ID != "current-1" {
		t.Fatalf("active result detail id=%q want current-1", app.activeResultDetail.ID)
	}
}
