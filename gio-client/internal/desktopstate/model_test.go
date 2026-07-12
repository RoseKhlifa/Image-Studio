package desktopstate

import (
	"math"
	"reflect"
	"testing"
)

func TestDefault(t *testing.T) {
	state := Default()
	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("schemaVersion=%d want %d", state.SchemaVersion, SchemaVersion)
	}
	if state.Revision != 0 || state.UpdatedAt != 0 {
		t.Fatalf("new state should be unsaved: revision=%d updatedAt=%d", state.Revision, state.UpdatedAt)
	}
	if state.Preferences.InterfaceStyle != InterfaceStyleWindows || state.Preferences.ExperienceMode != ExperienceModeSimple {
		t.Fatalf("unexpected default preferences: %#v", state.Preferences)
	}
	if state.Preferences.DefaultWindowLayout != WindowLayoutSingle {
		t.Fatalf("default layout=%q want %q", state.Preferences.DefaultWindowLayout, WindowLayoutSingle)
	}
	if !state.Preferences.AutoShowProgress || !state.Preferences.ReopenDetachedWindows || !state.Preferences.RestoreSession {
		t.Fatalf("desktop restore defaults should be enabled: %#v", state.Preferences)
	}
	if state.Windows == nil || state.Workspaces == nil {
		t.Fatal("default collections must be non-nil")
	}
}

func TestNormalizeRepairsDesktopAndWorkflowState(t *testing.T) {
	state := Normalize(State{
		SchemaVersion: 99,
		Preferences: Preferences{
			InterfaceStyle:      "invalid",
			ExperienceMode:      "invalid",
			DefaultWindowLayout: "invalid",
		},
		Windows: []Window{
			{Role: "invalid", WidthDp: -10, HeightDp: 0, Mode: "invalid"},
			{ID: "window-1", Role: WindowRoleProgress, WorkspaceID: " ws-1 "},
		},
		Workspaces: []Workspace{
			{
				Name: " ",
				Workflow: WorkflowGraph{
					Nodes: []WorkflowNode{
						{Kind: " ", X: math.NaN(), Y: math.Inf(1), WidthDp: -1, HeightDp: -2, Properties: map[string]string{"": "drop", " prompt ": "hello"}},
						{ID: "node-1", Kind: "image.generate"},
					},
					Edges: []WorkflowEdge{
						{FromNodeID: "node-1", ToNodeID: "node-1-2", FromPort: " out ", ToPort: " in "},
						{ID: "invalid", FromNodeID: "missing", ToNodeID: "node-1"},
					},
					Viewport: Viewport{OffsetX: math.NaN(), OffsetY: math.Inf(-1), Zoom: -1},
				},
			},
		},
	})

	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("schemaVersion=%d want %d", state.SchemaVersion, SchemaVersion)
	}
	if state.Preferences.InterfaceStyle != InterfaceStyleWindows || state.Preferences.ExperienceMode != ExperienceModeSimple || state.Preferences.DefaultWindowLayout != WindowLayoutSingle {
		t.Fatalf("preferences not normalized: %#v", state.Preferences)
	}
	if got := state.Windows[0]; got.ID != "window-1" || got.Role != WindowRoleMain || got.Mode != WindowModeWindowed || got.WidthDp != 1440 || got.HeightDp != 900 {
		t.Fatalf("first window not normalized: %#v", got)
	}
	if got := state.Windows[1]; got.ID != "window-1-2" || got.WorkspaceID != "ws-1" || got.WidthDp != 420 || got.HeightDp != 300 {
		t.Fatalf("second window not normalized: %#v", got)
	}
	workspace := state.Workspaces[0]
	if workspace.ID != "workspace-1" || workspace.Name != "Workspace 1" {
		t.Fatalf("workspace identity not normalized: %#v", workspace)
	}
	graph := workspace.Workflow
	if len(graph.Nodes) != 2 || graph.Nodes[0].ID != "node-1" || graph.Nodes[1].ID != "node-1-2" {
		t.Fatalf("node identifiers not normalized: %#v", graph.Nodes)
	}
	if graph.Nodes[0].Kind != "operation" || graph.Nodes[0].X != 0 || graph.Nodes[0].Y != 0 || graph.Nodes[0].WidthDp != 0 || graph.Nodes[0].HeightDp != 0 {
		t.Fatalf("first node not normalized: %#v", graph.Nodes[0])
	}
	if !reflect.DeepEqual(graph.Nodes[0].Properties, map[string]string{"prompt": "hello"}) {
		t.Fatalf("properties not normalized: %#v", graph.Nodes[0].Properties)
	}
	if len(graph.Edges) != 1 || graph.Edges[0].ID != "edge-1" || graph.Edges[0].FromPort != "out" || graph.Edges[0].ToPort != "in" {
		t.Fatalf("edges not normalized: %#v", graph.Edges)
	}
	if graph.Viewport != (Viewport{Zoom: 1}) {
		t.Fatalf("viewport not normalized: %#v", graph.Viewport)
	}
}

func TestNormalizeInitializesNilCollections(t *testing.T) {
	state := Normalize(State{})
	if state.Windows == nil || state.Workspaces == nil {
		t.Fatal("top-level collections must be non-nil")
	}
	workspace := Normalize(State{Workspaces: []Workspace{{ID: "ws"}}}).Workspaces[0]
	if workspace.Workflow.Nodes == nil || workspace.Workflow.Edges == nil {
		t.Fatal("workflow collections must be non-nil")
	}
}

func TestNormalizeDoesNotMutateWorkflowInput(t *testing.T) {
	input := State{Workspaces: []Workspace{{
		ID: "ws",
		Workflow: WorkflowGraph{
			Nodes: []WorkflowNode{{
				ID:         " node ",
				Kind:       " image.generate ",
				Properties: map[string]string{" prompt ": "cover"},
			}},
			Edges: []WorkflowEdge{{ID: " edge ", FromNodeID: "node", ToNodeID: "node"}},
		},
	}}}
	_ = Normalize(input)
	if input.Workspaces[0].Workflow.Nodes[0].ID != " node " || input.Workspaces[0].Workflow.Nodes[0].Kind != " image.generate " {
		t.Fatalf("Normalize mutated input node: %#v", input.Workspaces[0].Workflow.Nodes[0])
	}
	if _, ok := input.Workspaces[0].Workflow.Nodes[0].Properties[" prompt "]; !ok {
		t.Fatalf("Normalize mutated input properties: %#v", input.Workspaces[0].Workflow.Nodes[0].Properties)
	}
	if input.Workspaces[0].Workflow.Edges[0].ID != " edge " {
		t.Fatalf("Normalize mutated input edge: %#v", input.Workspaces[0].Workflow.Edges[0])
	}
}
