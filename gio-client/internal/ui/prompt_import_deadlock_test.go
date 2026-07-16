package ui

import (
	"testing"
	"time"

	"github.com/yuanhua/image-gptcodex/pkg/promptimport"
)

func TestConfirmPromptImportDoesNotReenterAppMutex(t *testing.T) {
	app := &App{
		activeWorkspaceID:        "ws-1",
		workspaces:               []workspaceState{{ID: "ws-1", Name: "Workspace 1"}},
		promptImportOpen:         true,
		promptImportToken:        "AB12cd34",
		promptImportResolvedSize: "auto",
		promptImportPayload: &promptimport.ImportPayload{
			Prompt: promptimport.BilingualText{Zh: "城市夜景"},
		},
	}

	done := make(chan struct{})
	go func() {
		app.confirmPromptImport()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("confirmPromptImport deadlocked while saving the active workspace")
	}
	if got := app.promptInput.Text(); got != "城市夜景" {
		t.Fatalf("prompt=%q want imported prompt", got)
	}
	if app.promptImportOpen || app.promptImportPayload != nil || app.promptImportToken != "" {
		t.Fatal("prompt import modal state was not cleared")
	}
	if len(app.workspaces) != 1 || app.workspaces[0].Prompt != "城市夜景" {
		t.Fatalf("workspace snapshot=%+v want imported prompt", app.workspaces)
	}
}
