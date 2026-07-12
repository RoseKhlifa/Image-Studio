package desktopstate

import "testing"

func TestNormalizeWorkspaceDraftKeepsStructuredSessionData(t *testing.T) {
	state := Normalize(State{Workspaces: []Workspace{{
		ID:   " ws-1 ",
		Name: " Board ",
		Draft: WorkspaceDraft{
			Prompt:           "do not trim authored prompt  ",
			Mode:             " generate ",
			BatchCount:       0,
			LoopTotalCount:   0,
			LoopConcurrency:  0,
			BatchConcurrency: 0,
			SourcePaths:      []string{" /tmp/a.png ", "", "/tmp/a.png", "/tmp/b.png"},
		},
		Result: WorkspaceResult{HistoryID: " item-1 ", SavedPath: " /tmp/out.png "},
	}}})
	if len(state.Workspaces) != 1 {
		t.Fatalf("workspaces=%d", len(state.Workspaces))
	}
	workspace := state.Workspaces[0]
	if workspace.ID != "ws-1" || workspace.Name != "Board" {
		t.Fatalf("workspace identity=%#v", workspace)
	}
	if workspace.Draft.Prompt != "do not trim authored prompt  " {
		t.Fatalf("authored prompt was changed: %q", workspace.Draft.Prompt)
	}
	if workspace.Draft.Mode != "generate" || workspace.Draft.BatchCount != 1 || workspace.Draft.LoopTotalCount != 10 || workspace.Draft.LoopConcurrency != 2 || workspace.Draft.BatchConcurrency != 2 {
		t.Fatalf("draft defaults=%#v", workspace.Draft)
	}
	if len(workspace.Draft.SourcePaths) != 2 || workspace.Draft.SourcePaths[0] != "/tmp/a.png" || workspace.Draft.SourcePaths[1] != "/tmp/b.png" {
		t.Fatalf("sources=%#v", workspace.Draft.SourcePaths)
	}
	if workspace.Result.HistoryID != "item-1" || workspace.Result.SavedPath != "/tmp/out.png" {
		t.Fatalf("result=%#v", workspace.Result)
	}
}
