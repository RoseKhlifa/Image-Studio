package ui

import (
	"image"
	"testing"

	"image-studio/gio-client/internal/desktopstate"
)

func TestDefaultWorkflowGraphIsValid(t *testing.T) {
	graph := defaultWorkflowGraph()
	if len(graph.Nodes) != 5 || len(graph.Edges) != 4 {
		t.Fatalf("default graph nodes=%d edges=%d", len(graph.Nodes), len(graph.Edges))
	}
	normalized := normalizeWorkflowGraph(graph)
	if len(normalized.Edges) != len(graph.Edges) {
		t.Fatalf("normalized edges=%d want %d", len(normalized.Edges), len(graph.Edges))
	}
}

func TestWorkflowGraphRejectsPortMismatchAndCycles(t *testing.T) {
	graph := defaultWorkflowGraph()
	_, err := connectWorkflowNodes(graph, workflowEdgeModel{
		FromNode: "prompt",
		FromPort: "text",
		ToNode:   "preview",
		ToPort:   "job",
	})
	if err == nil {
		t.Fatal("expected port mismatch error")
	}

	preview, _ := graph.node("preview")
	preview.Outputs = append(preview.Outputs, workflowPortModel{ID: "job-loop", Name: "任务", Kind: workflowPortJob})
	for idx := range graph.Nodes {
		if graph.Nodes[idx].ID == preview.ID {
			graph.Nodes[idx] = preview
		}
	}
	generate, _ := graph.node("generate")
	generate.Inputs = append(generate.Inputs, workflowPortModel{ID: "loop", Name: "任务", Kind: workflowPortJob})
	for idx := range graph.Nodes {
		if graph.Nodes[idx].ID == generate.ID {
			graph.Nodes[idx] = generate
		}
	}
	_, err = connectWorkflowNodes(graph, workflowEdgeModel{
		FromNode: "preview",
		FromPort: "job-loop",
		ToNode:   "generate",
		ToPort:   "loop",
	})
	if err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestMoveWorkflowNodeClampsAndIncrementsRevision(t *testing.T) {
	graph := defaultWorkflowGraph()
	moved := moveWorkflowNode(graph, "prompt", image.Pt(9000, -9000))
	node, ok := moved.node("prompt")
	if !ok {
		t.Fatal("prompt node missing")
	}
	if node.Position != image.Pt(4096, -4096) {
		t.Fatalf("position=%v", node.Position)
	}
	if moved.Revision != graph.Revision+1 {
		t.Fatalf("revision=%d want %d", moved.Revision, graph.Revision+1)
	}
	original, _ := graph.node("prompt")
	if original.Position == node.Position {
		t.Fatal("move mutated the original graph")
	}
}

func TestNormalizeWorkflowGraphDropsDuplicateNodesAndInvalidEdges(t *testing.T) {
	graph := defaultWorkflowGraph()
	graph.Nodes = append(graph.Nodes, graph.Nodes[0])
	graph.Edges = append(graph.Edges, workflowEdgeModel{
		FromNode: "missing",
		FromPort: "out",
		ToNode:   "generate",
		ToPort:   "prompt",
	})
	normalized := normalizeWorkflowGraph(graph)
	if len(normalized.Nodes) != 5 {
		t.Fatalf("nodes=%d want 5", len(normalized.Nodes))
	}
	if len(normalized.Edges) != 4 {
		t.Fatalf("edges=%d want 4", len(normalized.Edges))
	}
}

func TestToggleWorkflowConnectionDisconnectsAndReplacesSingleInput(t *testing.T) {
	graph := defaultWorkflowGraph()
	promptEdge := workflowEdgeModel{FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"}
	disconnected, err := toggleWorkflowConnection(graph, promptEdge)
	if err != nil {
		t.Fatalf("disconnect prompt edge: %v", err)
	}
	if workflowEdgeConnected(disconnected, promptEdge) {
		t.Fatal("prompt edge remained connected")
	}
	if disconnected.Revision != graph.Revision+1 {
		t.Fatalf("disconnect revision=%d want %d", disconnected.Revision, graph.Revision+1)
	}

	prompt, _ := graph.node("prompt")
	prompt.ID = "prompt-2"
	prompt.Title = "提示词 2"
	graph.Nodes = append(graph.Nodes, prompt)
	replacement := workflowEdgeModel{FromNode: "prompt-2", FromPort: "text", ToNode: "generate", ToPort: "prompt"}
	replaced, err := toggleWorkflowConnection(graph, replacement)
	if err != nil {
		t.Fatalf("replace prompt edge: %v", err)
	}
	if workflowEdgeConnected(replaced, promptEdge) || !workflowEdgeConnected(replaced, replacement) {
		t.Fatalf("single input was not replaced atomically: %+v", replaced.Edges)
	}
	if got := workflowInputEdges(replaced, "generate", "prompt"); len(got) != 1 {
		t.Fatalf("generate prompt edges=%d want 1", len(got))
	}
}

func TestToggleWorkflowConnectionRejectsCycleWithoutMutatingGraph(t *testing.T) {
	graph := defaultWorkflowGraph()
	preview, _ := graph.node("preview")
	preview.Outputs = append(preview.Outputs, workflowPortModel{ID: "job-loop", Name: "任务", Kind: workflowPortJob})
	generate, _ := graph.node("generate")
	generate.Inputs = append(generate.Inputs, workflowPortModel{ID: "loop", Name: "任务", Kind: workflowPortJob})
	for index := range graph.Nodes {
		switch graph.Nodes[index].ID {
		case preview.ID:
			graph.Nodes[index] = preview
		case generate.ID:
			graph.Nodes[index] = generate
		}
	}
	originalRevision := graph.Revision
	_, err := toggleWorkflowConnection(graph, workflowEdgeModel{
		FromNode: "preview", FromPort: "job-loop", ToNode: "generate", ToPort: "loop",
	})
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if graph.Revision != originalRevision || len(graph.Edges) != 4 {
		t.Fatalf("rejected connection mutated graph: revision=%d edges=%d", graph.Revision, len(graph.Edges))
	}
}

func TestRewireWorkflowConnectionMovesAndDisconnectsAtomically(t *testing.T) {
	graph := defaultWorkflowGraph()
	prompt, _ := graph.node("prompt")
	prompt.ID = "prompt-2"
	prompt.Title = "提示词 2"
	graph.Nodes = append(graph.Nodes, prompt)
	previous := workflowEdgeModel{FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"}
	replacement := workflowEdgeModel{FromNode: "prompt-2", FromPort: "text", ToNode: "generate", ToPort: "prompt"}

	rewired, err := rewireWorkflowConnection(graph, &previous, &replacement)
	if err != nil {
		t.Fatalf("rewire prompt input: %v", err)
	}
	if rewired.Revision != graph.Revision+1 {
		t.Fatalf("rewire revision=%d want %d", rewired.Revision, graph.Revision+1)
	}
	if workflowEdgeConnected(rewired, previous) || !workflowEdgeConnected(rewired, replacement) {
		t.Fatalf("rewired edges=%+v", rewired.Edges)
	}
	if got := workflowInputEdges(rewired, "generate", "prompt"); len(got) != 1 {
		t.Fatalf("rewired input edges=%d want 1", len(got))
	}

	disconnected, err := rewireWorkflowConnection(rewired, &replacement, nil)
	if err != nil {
		t.Fatalf("disconnect rewired edge: %v", err)
	}
	if workflowEdgeConnected(disconnected, replacement) {
		t.Fatal("rewired edge remained after disconnect")
	}

	unchanged, err := rewireWorkflowConnection(graph, nil, &previous)
	if err != nil {
		t.Fatalf("connect existing edge: %v", err)
	}
	if unchanged.Revision != graph.Revision || len(unchanged.Edges) != len(graph.Edges) {
		t.Fatalf("existing connection changed graph: revision=%d edges=%d", unchanged.Revision, len(unchanged.Edges))
	}
}

func TestValidateWorkflowForRunRequiresOutputChainAndConditionalSource(t *testing.T) {
	graph := defaultWorkflowGraph()
	if err := validateWorkflowForRun(graph, true); err != nil {
		t.Fatalf("default workflow should run: %v", err)
	}

	sourceEdge := workflowEdgeModel{FromNode: "source", FromPort: "image", ToNode: "generate", ToPort: "source"}
	withoutSource, err := toggleWorkflowConnection(graph, sourceEdge)
	if err != nil {
		t.Fatalf("disconnect source: %v", err)
	}
	if err := validateWorkflowForRun(withoutSource, false); err != nil {
		t.Fatalf("generate workflow should allow disconnected source: %v", err)
	}
	if err := validateWorkflowForRun(withoutSource, true); err == nil {
		t.Fatal("edit workflow accepted disconnected source")
	}

	exportEdge := workflowEdgeModel{FromNode: "preview", FromPort: "image", ToNode: "export", ToPort: "image"}
	withoutExport, err := toggleWorkflowConnection(graph, exportEdge)
	if err != nil {
		t.Fatalf("disconnect export: %v", err)
	}
	if err := validateWorkflowForRun(withoutExport, false); err == nil {
		t.Fatal("workflow without an output chain was accepted")
	}
}

func TestWorkflowDesktopRoundTripPreservesExplicitEmptyEdges(t *testing.T) {
	document := desktopWorkflowGraph(defaultWorkflowGraph())
	document.Edges = []desktopstate.WorkflowEdge{}
	restored := workflowGraphFromDesktop(document)
	if len(restored.Edges) != 0 {
		t.Fatalf("restored edges=%d want explicit empty graph", len(restored.Edges))
	}
}

func TestWorkflowNodeCatalogSupportsDeleteAddAndDisable(t *testing.T) {
	graph := defaultWorkflowGraph()
	withoutGenerate := removeWorkflowNode(graph, "generate")
	if _, ok := withoutGenerate.node("generate"); ok {
		t.Fatal("removed generate node remained in graph")
	}
	for _, edge := range withoutGenerate.Edges {
		if edge.FromNode == "generate" || edge.ToNode == "generate" {
			t.Fatalf("edge survived removed node: %+v", edge)
		}
	}
	available := workflowAvailableNodes(withoutGenerate)
	if len(available) != 1 || available[0].ID != "generate" {
		t.Fatalf("available nodes=%+v want generate", available)
	}
	if err := validateWorkflowForRun(withoutGenerate, false); err == nil {
		t.Fatal("workflow without generate node was accepted")
	}

	restored, err := addWorkflowNode(withoutGenerate, "generate")
	if err != nil {
		t.Fatalf("add generate: %v", err)
	}
	generate, ok := restored.node("generate")
	if !ok || !generate.Enabled {
		t.Fatalf("restored generate=%+v ok=%t", generate, ok)
	}
	if len(restored.Edges) != len(withoutGenerate.Edges) {
		t.Fatal("adding a node unexpectedly rebuilt deleted connections")
	}

	disabled := setWorkflowNodeEnabled(restored, "generate", false)
	generate, _ = disabled.node("generate")
	if generate.Enabled || disabled.Revision != restored.Revision+1 {
		t.Fatalf("disabled generate=%+v revision=%d", generate, disabled.Revision)
	}
	if err := validateWorkflowForRun(disabled, false); err == nil {
		t.Fatal("workflow accepted a disabled required node")
	}
}

func TestNormalizeWorkflowGraphPreservesExplicitEmptyCanvas(t *testing.T) {
	graph := normalizeWorkflowGraph(workflowGraphModel{})
	if len(graph.Nodes) != 0 || len(graph.Edges) != 0 || graph.Revision != 1 {
		t.Fatalf("normalized empty graph=%+v", graph)
	}
}

func TestWorkflowDesktopRoundTripPreservesDeletedAndDisabledNodes(t *testing.T) {
	graph := removeWorkflowNode(defaultWorkflowGraph(), "source")
	graph = setWorkflowNodeEnabled(graph, "preview", false)
	document := desktopWorkflowGraph(graph)
	if !document.Explicit {
		t.Fatal("saved workflow was not marked explicit")
	}
	restored := workflowGraphFromDesktop(document)
	if _, ok := restored.node("source"); ok {
		t.Fatal("deleted source node returned after desktop round trip")
	}
	preview, ok := restored.node("preview")
	if !ok || preview.Enabled {
		t.Fatalf("restored preview=%+v ok=%t want disabled", preview, ok)
	}

	empty := defaultWorkflowGraph()
	for _, node := range workflowNodeCatalog() {
		empty = removeWorkflowNode(empty, node.ID)
	}
	restoredEmpty := workflowGraphFromDesktop(desktopWorkflowGraph(empty))
	if len(restoredEmpty.Nodes) != 0 {
		t.Fatalf("restored explicit empty nodes=%d", len(restoredEmpty.Nodes))
	}
	legacy := workflowGraphFromDesktop(desktopstate.WorkflowGraph{})
	if len(legacy.Nodes) != len(workflowNodeCatalog()) {
		t.Fatalf("legacy empty document nodes=%d want default catalog", len(legacy.Nodes))
	}
}
