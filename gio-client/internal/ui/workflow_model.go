package ui

import (
	"errors"
	"fmt"
	"image"
	"strings"
)

type workflowNodeKind string

const (
	workflowNodePrompt   workflowNodeKind = "prompt"
	workflowNodeSource   workflowNodeKind = "source"
	workflowNodeGenerate workflowNodeKind = "generate"
	workflowNodePreview  workflowNodeKind = "preview"
	workflowNodeExport   workflowNodeKind = "export"
)

type workflowPortKind string

const (
	workflowPortText  workflowPortKind = "text"
	workflowPortImage workflowPortKind = "image"
	workflowPortJob   workflowPortKind = "job"
)

type workflowPortModel struct {
	ID    string
	Name  string
	Kind  workflowPortKind
	Multi bool
}

type workflowNodeModel struct {
	ID       string
	Kind     workflowNodeKind
	Title    string
	Subtitle string
	Position image.Point
	Enabled  bool
	Inputs   []workflowPortModel
	Outputs  []workflowPortModel
}

type workflowEdgeModel struct {
	ID       string
	FromNode string
	FromPort string
	ToNode   string
	ToPort   string
}

type workflowGraphModel struct {
	Revision int
	Nodes    []workflowNodeModel
	Edges    []workflowEdgeModel
}

func defaultWorkflowGraph() workflowGraphModel {
	graph := workflowGraphModel{
		Revision: 1,
		Nodes:    workflowNodeCatalog(),
	}
	graph.Edges = []workflowEdgeModel{
		{ID: "prompt:text>generate:prompt", FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"},
		{ID: "source:image>generate:source", FromNode: "source", FromPort: "image", ToNode: "generate", ToPort: "source"},
		{ID: "generate:job>preview:job", FromNode: "generate", FromPort: "job", ToNode: "preview", ToPort: "job"},
		{ID: "preview:image>export:image", FromNode: "preview", FromPort: "image", ToNode: "export", ToPort: "image"},
	}
	return graph
}

func workflowNodeCatalog() []workflowNodeModel {
	return []workflowNodeModel{
		{
			ID:       "prompt",
			Kind:     workflowNodePrompt,
			Title:    "提示词",
			Subtitle: "构造生成意图与约束",
			Position: image.Pt(72, 92),
			Enabled:  true,
			Outputs:  []workflowPortModel{{ID: "text", Name: "文本", Kind: workflowPortText}},
		},
		{
			ID:       "source",
			Kind:     workflowNodeSource,
			Title:    "参考图",
			Subtitle: "可选的图像输入队列",
			Position: image.Pt(72, 326),
			Enabled:  true,
			Outputs:  []workflowPortModel{{ID: "image", Name: "图像", Kind: workflowPortImage}},
		},
		{
			ID:       "generate",
			Kind:     workflowNodeGenerate,
			Title:    "图像生成",
			Subtitle: "调用当前上游与模型",
			Position: image.Pt(416, 190),
			Enabled:  true,
			Inputs: []workflowPortModel{
				{ID: "prompt", Name: "提示词", Kind: workflowPortText},
				{ID: "source", Name: "参考图", Kind: workflowPortImage, Multi: true},
			},
			Outputs: []workflowPortModel{{ID: "job", Name: "任务", Kind: workflowPortJob}},
		},
		{
			ID:       "preview",
			Kind:     workflowNodePreview,
			Title:    "实时预览",
			Subtitle: "跟踪流式进度与结果",
			Position: image.Pt(752, 190),
			Enabled:  true,
			Inputs:   []workflowPortModel{{ID: "job", Name: "任务", Kind: workflowPortJob}},
			Outputs:  []workflowPortModel{{ID: "image", Name: "图像", Kind: workflowPortImage}},
		},
		{
			ID:       "export",
			Kind:     workflowNodeExport,
			Title:    "导出",
			Subtitle: "保存产物与历史记录",
			Position: image.Pt(1088, 190),
			Enabled:  true,
			Inputs:   []workflowPortModel{{ID: "image", Name: "图像", Kind: workflowPortImage}},
		},
	}
}

func workflowNodeTemplate(nodeID string) (workflowNodeModel, bool) {
	nodeID = strings.TrimSpace(nodeID)
	for _, node := range workflowNodeCatalog() {
		if node.ID == nodeID {
			return node, true
		}
	}
	return workflowNodeModel{}, false
}

func workflowAvailableNodes(graph workflowGraphModel) []workflowNodeModel {
	available := make([]workflowNodeModel, 0)
	for _, node := range workflowNodeCatalog() {
		if _, exists := graph.node(node.ID); !exists {
			available = append(available, node)
		}
	}
	return available
}

func cloneWorkflowGraph(graph workflowGraphModel) workflowGraphModel {
	clone := workflowGraphModel{
		Revision: graph.Revision,
		Nodes:    make([]workflowNodeModel, len(graph.Nodes)),
		Edges:    append([]workflowEdgeModel(nil), graph.Edges...),
	}
	for idx, node := range graph.Nodes {
		node.Inputs = append([]workflowPortModel(nil), node.Inputs...)
		node.Outputs = append([]workflowPortModel(nil), node.Outputs...)
		clone.Nodes[idx] = node
	}
	return clone
}

func normalizeWorkflowGraph(graph workflowGraphModel) workflowGraphModel {
	normalized := cloneWorkflowGraph(graph)
	nodeIDs := make(map[string]struct{}, len(normalized.Nodes))
	out := normalized.Nodes[:0]
	for _, node := range normalized.Nodes {
		node.ID = strings.TrimSpace(node.ID)
		if node.ID == "" {
			continue
		}
		if _, exists := nodeIDs[node.ID]; exists {
			continue
		}
		nodeIDs[node.ID] = struct{}{}
		node.Position.X = clampInt(node.Position.X, -4096, 4096)
		node.Position.Y = clampInt(node.Position.Y, -4096, 4096)
		out = append(out, node)
	}
	normalized.Nodes = out
	normalized.Edges = nil
	for _, edge := range graph.Edges {
		if err := validateWorkflowEdge(normalized, edge); err != nil {
			continue
		}
		if workflowGraphWouldCycle(normalized, edge) {
			continue
		}
		edge.ID = workflowEdgeID(edge)
		normalized.Edges = append(normalized.Edges, edge)
	}
	if normalized.Revision < 1 {
		normalized.Revision = 1
	}
	return normalized
}

func addWorkflowNode(graph workflowGraphModel, nodeID string) (workflowGraphModel, error) {
	node, ok := workflowNodeTemplate(nodeID)
	if !ok {
		return graph, fmt.Errorf("workflow node %q is not available", nodeID)
	}
	if _, exists := graph.node(node.ID); exists {
		return graph, fmt.Errorf("workflow node %q already exists", node.ID)
	}
	next := cloneWorkflowGraph(graph)
	next.Nodes = append(next.Nodes, node)
	next.Revision = max(graph.Revision+1, 1)
	return next, nil
}

func removeWorkflowNode(graph workflowGraphModel, nodeID string) workflowGraphModel {
	nodeID = strings.TrimSpace(nodeID)
	if _, ok := graph.node(nodeID); !ok {
		return graph
	}
	next := cloneWorkflowGraph(graph)
	nodes := next.Nodes[:0]
	for _, node := range next.Nodes {
		if node.ID != nodeID {
			nodes = append(nodes, node)
		}
	}
	next.Nodes = nodes
	edges := next.Edges[:0]
	for _, edge := range next.Edges {
		if edge.FromNode != nodeID && edge.ToNode != nodeID {
			edges = append(edges, edge)
		}
	}
	next.Edges = edges
	next.Revision = max(graph.Revision+1, 1)
	return next
}

func setWorkflowNodeEnabled(graph workflowGraphModel, nodeID string, enabled bool) workflowGraphModel {
	next := cloneWorkflowGraph(graph)
	for idx := range next.Nodes {
		if next.Nodes[idx].ID != strings.TrimSpace(nodeID) || next.Nodes[idx].Enabled == enabled {
			continue
		}
		next.Nodes[idx].Enabled = enabled
		next.Revision = max(graph.Revision+1, 1)
		return next
	}
	return graph
}

func workflowEdgeID(edge workflowEdgeModel) string {
	return fmt.Sprintf("%s:%s>%s:%s", edge.FromNode, edge.FromPort, edge.ToNode, edge.ToPort)
}

func (graph workflowGraphModel) node(id string) (workflowNodeModel, bool) {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return node, true
		}
	}
	return workflowNodeModel{}, false
}

func workflowOutputPort(node workflowNodeModel, id string) (workflowPortModel, bool) {
	for _, port := range node.Outputs {
		if port.ID == id {
			return port, true
		}
	}
	return workflowPortModel{}, false
}

func workflowInputPort(node workflowNodeModel, id string) (workflowPortModel, bool) {
	for _, port := range node.Inputs {
		if port.ID == id {
			return port, true
		}
	}
	return workflowPortModel{}, false
}

func validateWorkflowEdge(graph workflowGraphModel, edge workflowEdgeModel) error {
	if edge.FromNode == edge.ToNode {
		return errors.New("workflow edge cannot connect a node to itself")
	}
	from, ok := graph.node(edge.FromNode)
	if !ok {
		return fmt.Errorf("workflow source node %q does not exist", edge.FromNode)
	}
	to, ok := graph.node(edge.ToNode)
	if !ok {
		return fmt.Errorf("workflow target node %q does not exist", edge.ToNode)
	}
	output, ok := workflowOutputPort(from, edge.FromPort)
	if !ok {
		return fmt.Errorf("workflow output port %q does not exist", edge.FromPort)
	}
	input, ok := workflowInputPort(to, edge.ToPort)
	if !ok {
		return fmt.Errorf("workflow input port %q does not exist", edge.ToPort)
	}
	if output.Kind != input.Kind {
		return fmt.Errorf("workflow port mismatch: %s cannot connect to %s", output.Kind, input.Kind)
	}
	for _, existing := range graph.Edges {
		if workflowEdgeID(existing) == workflowEdgeID(edge) {
			return errors.New("workflow edge already exists")
		}
		if existing.ToNode == edge.ToNode && existing.ToPort == edge.ToPort && !input.Multi {
			return errors.New("workflow input already has a connection")
		}
	}
	return nil
}

func workflowGraphWouldCycle(graph workflowGraphModel, candidate workflowEdgeModel) bool {
	adjacency := make(map[string][]string, len(graph.Nodes))
	for _, edge := range graph.Edges {
		adjacency[edge.FromNode] = append(adjacency[edge.FromNode], edge.ToNode)
	}
	adjacency[candidate.FromNode] = append(adjacency[candidate.FromNode], candidate.ToNode)
	visiting := make(map[string]bool, len(graph.Nodes))
	visited := make(map[string]bool, len(graph.Nodes))
	var visit func(string) bool
	visit = func(node string) bool {
		if visiting[node] {
			return true
		}
		if visited[node] {
			return false
		}
		visiting[node] = true
		for _, next := range adjacency[node] {
			if visit(next) {
				return true
			}
		}
		visiting[node] = false
		visited[node] = true
		return false
	}
	for _, node := range graph.Nodes {
		if visit(node.ID) {
			return true
		}
	}
	return false
}

func connectWorkflowNodes(graph workflowGraphModel, edge workflowEdgeModel) (workflowGraphModel, error) {
	graph = normalizeWorkflowGraph(graph)
	if err := validateWorkflowEdge(graph, edge); err != nil {
		return graph, err
	}
	if workflowGraphWouldCycle(graph, edge) {
		return graph, errors.New("workflow edge would create a cycle")
	}
	edge.ID = workflowEdgeID(edge)
	graph.Edges = append(graph.Edges, edge)
	graph.Revision++
	return graph, nil
}

func workflowEdgeConnected(graph workflowGraphModel, candidate workflowEdgeModel) bool {
	candidateID := workflowEdgeID(candidate)
	for _, edge := range graph.Edges {
		if workflowEdgeID(edge) == candidateID {
			return true
		}
	}
	return false
}

func workflowInputEdges(graph workflowGraphModel, nodeID string, portID string) []workflowEdgeModel {
	edges := make([]workflowEdgeModel, 0, 1)
	for _, edge := range graph.Edges {
		if edge.ToNode == nodeID && edge.ToPort == portID {
			edges = append(edges, edge)
		}
	}
	return edges
}

func workflowCompatibleOutputs(graph workflowGraphModel, targetNodeID string, targetPortID string) []workflowEdgeModel {
	target, ok := graph.node(targetNodeID)
	if !ok {
		return nil
	}
	input, ok := workflowInputPort(target, targetPortID)
	if !ok {
		return nil
	}
	candidates := make([]workflowEdgeModel, 0)
	for _, node := range graph.Nodes {
		if node.ID == targetNodeID || !node.Enabled {
			continue
		}
		for _, output := range node.Outputs {
			if output.Kind != input.Kind {
				continue
			}
			candidates = append(candidates, workflowEdgeModel{
				FromNode: node.ID,
				FromPort: output.ID,
				ToNode:   targetNodeID,
				ToPort:   targetPortID,
			})
		}
	}
	return candidates
}

// toggleWorkflowConnection mirrors node editors such as ComfyUI: selecting an
// already connected source disconnects it, while selecting another source for
// a single-input port replaces the old edge atomically.
func toggleWorkflowConnection(graph workflowGraphModel, candidate workflowEdgeModel) (workflowGraphModel, error) {
	graph = normalizeWorkflowGraph(graph)
	if workflowEdgeConnected(graph, candidate) {
		next := cloneWorkflowGraph(graph)
		next.Edges = next.Edges[:0]
		candidateID := workflowEdgeID(candidate)
		for _, edge := range graph.Edges {
			if workflowEdgeID(edge) != candidateID {
				next.Edges = append(next.Edges, edge)
			}
		}
		next.Revision++
		return next, nil
	}

	target, ok := graph.node(candidate.ToNode)
	if !ok {
		return graph, fmt.Errorf("workflow target node %q does not exist", candidate.ToNode)
	}
	input, ok := workflowInputPort(target, candidate.ToPort)
	if !ok {
		return graph, fmt.Errorf("workflow input port %q does not exist", candidate.ToPort)
	}

	next := cloneWorkflowGraph(graph)
	if !input.Multi {
		filtered := next.Edges[:0]
		for _, edge := range next.Edges {
			if edge.ToNode == candidate.ToNode && edge.ToPort == candidate.ToPort {
				continue
			}
			filtered = append(filtered, edge)
		}
		next.Edges = filtered
	}
	if err := validateWorkflowEdge(next, candidate); err != nil {
		return graph, err
	}
	if workflowGraphWouldCycle(next, candidate) {
		return graph, errors.New("workflow edge would create a cycle")
	}
	candidate.ID = workflowEdgeID(candidate)
	next.Edges = append(next.Edges, candidate)
	next.Revision = graph.Revision + 1
	return next, nil
}

func rewireWorkflowConnection(graph workflowGraphModel, previous *workflowEdgeModel, replacement *workflowEdgeModel) (workflowGraphModel, error) {
	graph = normalizeWorkflowGraph(graph)
	next := cloneWorkflowGraph(graph)
	removed := false
	if previous != nil {
		previousID := workflowEdgeID(*previous)
		filtered := next.Edges[:0]
		for _, edge := range next.Edges {
			if workflowEdgeID(edge) == previousID {
				removed = true
				continue
			}
			filtered = append(filtered, edge)
		}
		next.Edges = filtered
	}
	if replacement == nil {
		if !removed {
			return graph, nil
		}
		next.Revision = graph.Revision + 1
		return next, nil
	}
	if workflowEdgeConnected(next, *replacement) {
		if !removed {
			return graph, nil
		}
		next.Revision = graph.Revision + 1
		return next, nil
	}

	target, ok := next.node(replacement.ToNode)
	if !ok {
		return graph, fmt.Errorf("workflow target node %q does not exist", replacement.ToNode)
	}
	input, ok := workflowInputPort(target, replacement.ToPort)
	if !ok {
		return graph, fmt.Errorf("workflow input port %q does not exist", replacement.ToPort)
	}
	if !input.Multi {
		filtered := next.Edges[:0]
		for _, edge := range next.Edges {
			if edge.ToNode == replacement.ToNode && edge.ToPort == replacement.ToPort {
				continue
			}
			filtered = append(filtered, edge)
		}
		next.Edges = filtered
	}
	if err := validateWorkflowEdge(next, *replacement); err != nil {
		return graph, err
	}
	if workflowGraphWouldCycle(next, *replacement) {
		return graph, errors.New("workflow edge would create a cycle")
	}
	edge := *replacement
	edge.ID = workflowEdgeID(edge)
	next.Edges = append(next.Edges, edge)
	next.Revision = graph.Revision + 1
	return next, nil
}

func validateWorkflowForRun(graph workflowGraphModel, requireSource bool) error {
	graph = normalizeWorkflowGraph(graph)
	requiredNodes := []string{"prompt", "generate", "preview", "export"}
	if requireSource {
		requiredNodes = append(requiredNodes, "source")
	}
	for _, nodeID := range requiredNodes {
		node, ok := graph.node(nodeID)
		if !ok || !node.Enabled {
			return fmt.Errorf("节点 %s 不可用", nodeID)
		}
	}
	requiredEdges := []workflowEdgeModel{
		{FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"},
		{FromNode: "generate", FromPort: "job", ToNode: "preview", ToPort: "job"},
		{FromNode: "preview", FromPort: "image", ToNode: "export", ToPort: "image"},
	}
	if requireSource {
		requiredEdges = append(requiredEdges, workflowEdgeModel{FromNode: "source", FromPort: "image", ToNode: "generate", ToPort: "source"})
	}
	for _, edge := range requiredEdges {
		if !workflowEdgeConnected(graph, edge) {
			from, _ := graph.node(edge.FromNode)
			to, _ := graph.node(edge.ToNode)
			return fmt.Errorf("缺少 %s 到 %s 的连接", from.Title, to.Title)
		}
	}
	return nil
}

func moveWorkflowNode(graph workflowGraphModel, nodeID string, position image.Point) workflowGraphModel {
	graph = cloneWorkflowGraph(graph)
	for idx := range graph.Nodes {
		if graph.Nodes[idx].ID != nodeID {
			continue
		}
		position.X = clampInt(position.X, -4096, 4096)
		position.Y = clampInt(position.Y, -4096, 4096)
		if graph.Nodes[idx].Position == position {
			return graph
		}
		graph.Nodes[idx].Position = position
		graph.Revision++
		return graph
	}
	return graph
}
