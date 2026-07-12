package ui

import (
	"reflect"
	"testing"

	"image-studio/gio-client/internal/desktopstate"
)

func TestDesktopWorkspaceDocumentDropsProcessLocalReferences(t *testing.T) {
	app := &App{}
	workspace := workspaceState{
		ID:              "workspace-1",
		Name:            "Transient sources",
		SourcePathsText: "/tmp/source.png\nmemory://image/clipboard.png\n/tmp/source.png\nMEMORY://text/raw.txt",
		ResultSavedPath: "memory://image/result.png",
		ResultRawPath:   "memory://text/result.json",
	}

	document := app.desktopWorkspaceDocument(workspace)
	if want := []string{"/tmp/source.png"}; !reflect.DeepEqual(document.Draft.SourcePaths, want) {
		t.Fatalf("persisted sources=%#v want %#v", document.Draft.SourcePaths, want)
	}
	if document.Result.SavedPath != "" || document.Result.RawPath != "" {
		t.Fatalf("persisted transient result references: %+v", document.Result)
	}
}

func TestWorkspaceFromDesktopDocumentSanitizesLegacyTransientReferences(t *testing.T) {
	app := &App{}
	document := desktopstate.Workspace{
		ID:   "workspace-1",
		Name: "Legacy transient state",
		Draft: desktopstate.WorkspaceDraft{
			SourcePaths: []string{
				"memory://image/expired.png",
				"/tmp/durable.png",
				"MEMORY://text/expired.txt",
			},
		},
		Result: desktopstate.WorkspaceResult{
			SavedPath: "memory://image/expired-result.png",
			RawPath:   "memory://text/expired-result.json",
		},
	}

	workspace := app.workspaceFromDesktopDocument(document)
	if workspace.SourcePathsText != "/tmp/durable.png" {
		t.Fatalf("restored sources=%q want only durable source", workspace.SourcePathsText)
	}
	if workspace.ResultSavedPath != "" || workspace.ResultRawPath != "" || workspace.ResultHasItem {
		t.Fatalf("restored transient result as durable: %+v", workspace)
	}
}

func TestDurableDesktopReferencesNormalizeAndDeduplicate(t *testing.T) {
	got := durableDesktopReferences([]string{
		" /tmp/a.png ",
		"memory://image/a.png",
		"/tmp/a.png",
		"/tmp/b.png",
		"MEMORY://text/raw.txt",
	})
	want := []string{"/tmp/a.png", "/tmp/b.png"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("durable references=%#v want %#v", got, want)
	}
}
