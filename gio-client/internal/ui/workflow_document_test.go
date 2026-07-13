package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"image-studio/gio-client/internal/desktopstate"
)

func TestWorkflowDocumentRoundTripPreservesGraphAndDraft(t *testing.T) {
	graph := removeWorkflowNode(defaultWorkflowGraph(), "source")
	graph = setWorkflowNodeEnabled(graph, "preview", false)
	draft := desktopstate.WorkspaceDraft{
		Prompt:       "cinematic city",
		Mode:         "generate",
		Size:         "1536x1024",
		Quality:      "high",
		OutputFormat: "png",
	}
	data, err := marshalWorkflowDocument("Demo", draft, graph)
	if err != nil {
		t.Fatalf("marshal workflow: %v", err)
	}
	document, restored, err := parseWorkflowDocument(data)
	if err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	if document.Format != workflowDocumentFormat || document.Version != workflowDocumentVersion || document.Name != "Demo" {
		t.Fatalf("document metadata=%+v", document)
	}
	if document.Draft.Prompt != draft.Prompt || document.Draft.Size != draft.Size {
		t.Fatalf("restored draft=%+v", document.Draft)
	}
	if _, ok := restored.node("source"); ok {
		t.Fatal("deleted source returned in workflow document")
	}
	preview, ok := restored.node("preview")
	if !ok || preview.Enabled {
		t.Fatalf("restored preview=%+v ok=%t", preview, ok)
	}
}

func TestParseWorkflowDocumentRejectsForeignInvalidAndOversizedFiles(t *testing.T) {
	if _, _, err := parseWorkflowDocument([]byte(`{"last_node_id": 5, "nodes": []}`)); err == nil {
		t.Fatal("foreign workflow format was accepted")
	}
	invalid := workflowDocument{
		Format:  workflowDocumentFormat,
		Version: workflowDocumentVersion,
		Graph: desktopstate.WorkflowGraph{
			Explicit: true,
			Nodes:    []desktopstate.WorkflowNode{{ID: "custom-node", Kind: "custom"}},
		},
	}
	data, err := json.Marshal(invalid)
	if err != nil {
		t.Fatalf("marshal invalid workflow: %v", err)
	}
	if _, _, err := parseWorkflowDocument(data); err == nil {
		t.Fatal("unsupported workflow node was accepted")
	}
	if _, _, err := parseWorkflowDocument([]byte(strings.Repeat("x", maxWorkflowDocumentBytes+1))); err == nil {
		t.Fatal("oversized workflow was accepted")
	}
}

func TestApplyWorkflowDocumentUpdatesCurrentWorkspaceAndCanUndoGraph(t *testing.T) {
	isolateGioStableDataRoot(t)
	app := New()
	workspaceID := app.activeWorkspaceID
	graph := removeWorkflowNode(defaultWorkflowGraph(), "source")
	graph = setWorkflowNodeEnabled(graph, "preview", false)
	draft := desktopstate.WorkspaceDraft{
		Prompt:       "imported prompt",
		Mode:         "generate",
		Size:         "1536x1024",
		Quality:      "high",
		OutputFormat: "webp",
		BatchCount:   3,
	}
	data, err := marshalWorkflowDocument("Imported", draft, graph)
	if err != nil {
		t.Fatalf("marshal workflow: %v", err)
	}
	if err := app.applyWorkflowDocument(data); err != nil {
		t.Fatalf("apply workflow: %v", err)
	}
	if app.activeWorkspaceID != workspaceID || app.promptInput.Text() != draft.Prompt || app.quality != draft.Quality || app.format != draft.OutputFormat {
		t.Fatalf("applied workspace id=%q prompt=%q quality=%q format=%q", app.activeWorkspaceID, app.promptInput.Text(), app.quality, app.format)
	}
	if _, ok := app.workflowGraph(workspaceID).node("source"); ok {
		t.Fatal("applied graph retained deleted source")
	}
	if !app.canUndoWorkflowGraph(workspaceID) || !app.undoWorkflowGraph(workspaceID) {
		t.Fatal("imported graph did not create an undo entry")
	}
	if _, ok := app.workflowGraph(workspaceID).node("source"); !ok {
		t.Fatal("undo import did not restore source")
	}
}
