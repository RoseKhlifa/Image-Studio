package ui

import (
	"image"
	"testing"
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
