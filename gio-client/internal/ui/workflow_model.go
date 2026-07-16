package ui

import (
	"errors"
	"fmt"
	"image"
	"strconv"
	"strings"

	"image-studio/gio-client/internal/kernel"

	"github.com/yuanhua/image-gptcodex/pkg/client"
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
	ID          string
	TypeID      string
	TypeVersion string
	Category    string
	Kind        workflowNodeKind
	Title       string
	Subtitle    string
	Position    image.Point
	Enabled     bool
	Inputs      []workflowPortModel
	Outputs     []workflowPortModel
	Properties  map[string]string
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
			ID:          "prompt",
			TypeID:      "prompt",
			TypeVersion: "1.0.0",
			Category:    "输入",
			Kind:        workflowNodePrompt,
			Title:       "提示词",
			Subtitle:    "构造生成意图与约束",
			Position:    image.Pt(72, 92),
			Enabled:     true,
			Outputs:     []workflowPortModel{{ID: "text", Name: "文本", Kind: workflowPortText}},
			Properties: map[string]string{
				workflowPropertyPrompt:   "",
				workflowPropertyNegative: "",
				workflowPropertyStyleTag: "",
			},
		},
		{
			ID:          "source",
			TypeID:      "source",
			TypeVersion: "1.0.0",
			Category:    "输入",
			Kind:        workflowNodeSource,
			Title:       "参考图",
			Subtitle:    "可选的图像输入队列",
			Position:    image.Pt(72, 326),
			Enabled:     true,
			Outputs:     []workflowPortModel{{ID: "image", Name: "图像", Kind: workflowPortImage}},
			Properties: map[string]string{
				workflowPropertySourcePaths: "",
			},
		},
		{
			ID:          "generate",
			TypeID:      "generate",
			TypeVersion: "1.0.0",
			Category:    "处理",
			Kind:        workflowNodeGenerate,
			Title:       "图像生成",
			Subtitle:    "调用当前上游与模型",
			Position:    image.Pt(416, 190),
			Enabled:     true,
			Inputs: []workflowPortModel{
				{ID: "prompt", Name: "提示词", Kind: workflowPortText},
				{ID: "source", Name: "参考图", Kind: workflowPortImage, Multi: true},
			},
			Outputs: []workflowPortModel{
				{ID: "job", Name: "预览任务", Kind: workflowPortJob},
				{ID: "image", Name: "最终图像", Kind: workflowPortImage},
			},
			Properties: map[string]string{
				workflowPropertyMode:       string(client.ModeGenerate),
				workflowPropertyQuality:    client.DefaultQuality,
				workflowPropertySize:       client.DefaultSize,
				workflowPropertyImageModel: client.ImageModel,
				workflowPropertyBatchCount: "1",
			},
		},
		{
			ID:          "preview",
			TypeID:      "preview",
			TypeVersion: "1.0.0",
			Category:    "处理",
			Kind:        workflowNodePreview,
			Title:       "实时预览",
			Subtitle:    "跟踪流式进度与结果",
			Position:    image.Pt(752, 190),
			Enabled:     true,
			Inputs:      []workflowPortModel{{ID: "job", Name: "任务", Kind: workflowPortJob}},
			Outputs:     []workflowPortModel{{ID: "image", Name: "图像", Kind: workflowPortImage}},
			Properties: map[string]string{
				workflowPropertyPartialImages: "0",
			},
		},
		{
			ID:          "export",
			TypeID:      "export",
			TypeVersion: "1.0.0",
			Category:    "输出",
			Kind:        workflowNodeExport,
			Title:       "导出",
			Subtitle:    "保存产物与历史记录",
			Position:    image.Pt(1088, 190),
			Enabled:     true,
			Inputs:      []workflowPortModel{{ID: "image", Name: "图像", Kind: workflowPortImage}},
			Properties: map[string]string{
				workflowPropertyOutputFormat: client.OutputFormat,
				workflowPropertyOutputDir:    kernel.DefaultOutputDir(),
			},
		},
	}
}

func workflowNodeTypeID(node workflowNodeModel) string {
	if typeID := strings.TrimSpace(node.TypeID); typeID != "" {
		return typeID
	}
	return string(node.Kind)
}

func workflowNodeTemplateFromCatalog(catalog []workflowNodeModel, typeID string) (workflowNodeModel, bool) {
	typeID = strings.TrimSpace(typeID)
	for _, node := range catalog {
		if workflowNodeTypeID(node) == typeID {
			return node, true
		}
	}
	return workflowNodeModel{}, false
}

func workflowNodeTemplate(nodeID string) (workflowNodeModel, bool) {
	return workflowNodeTemplateFromCatalog(workflowNodeCatalog(), nodeID)
}

func workflowNodeTemplateByKind(kind workflowNodeKind) (workflowNodeModel, bool) {
	for _, node := range workflowNodeCatalog() {
		if node.Kind == kind {
			return node, true
		}
	}
	return workflowNodeModel{}, false
}

func workflowNodeTemplateForInstance(nodeID string, kind string) (workflowNodeModel, bool) {
	return workflowNodeTemplateForDescriptor(nodeID, "", "", "", "", kind)
}

func workflowNodeTemplateForDescriptor(nodeID string, typeID string, typeVersion string, category string, subtitle string, kind string) (workflowNodeModel, bool) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return workflowNodeModel{}, false
	}
	if normalizedKind := workflowNodeKind(strings.TrimSpace(kind)); normalizedKind != "" {
		node, ok := workflowNodeTemplateByKind(normalizedKind)
		if !ok {
			return workflowNodeModel{}, false
		}
		node.ID = nodeID
		if typeID = strings.TrimSpace(typeID); typeID != "" {
			node.TypeID = typeID
		}
		if typeVersion = strings.TrimSpace(typeVersion); typeVersion != "" {
			node.TypeVersion = typeVersion
		}
		if category = strings.TrimSpace(category); category != "" {
			node.Category = category
		}
		if subtitle = strings.TrimSpace(subtitle); subtitle != "" {
			node.Subtitle = subtitle
		}
		return node, true
	}
	node, ok := workflowNodeTemplate(nodeID)
	return node, ok
}

func workflowAvailableNodes(graph workflowGraphModel) []workflowNodeModel {
	return workflowNodeCatalog()
}

func workflowAvailableNodesFromCatalog(graph workflowGraphModel, catalog []workflowNodeModel) []workflowNodeModel {
	return append([]workflowNodeModel(nil), catalog...)
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
		node.Properties = cloneWorkflowProperties(node.Properties)
		clone.Nodes[idx] = node
	}
	return clone
}

func cloneWorkflowProperties(properties map[string]string) map[string]string {
	if len(properties) == 0 {
		return nil
	}
	clone := make(map[string]string, len(properties))
	for key, value := range properties {
		key = strings.TrimSpace(key)
		if key != "" {
			clone[key] = value
		}
	}
	return clone
}

func mergeWorkflowProperties(defaults map[string]string, overrides map[string]string) map[string]string {
	merged := cloneWorkflowProperties(defaults)
	if merged == nil {
		merged = map[string]string{}
	}
	for key, value := range overrides {
		key = strings.TrimSpace(key)
		if key != "" {
			merged[key] = value
		}
	}
	return merged
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
	next, _, err := addWorkflowNodeInstance(graph, nodeID)
	return next, err
}

func addWorkflowNodeInstance(graph workflowGraphModel, templateID string) (workflowGraphModel, string, error) {
	return addWorkflowNodeInstanceFromCatalog(graph, templateID, workflowNodeCatalog())
}

func addWorkflowNodeInstanceFromCatalog(graph workflowGraphModel, templateID string, catalog []workflowNodeModel) (workflowGraphModel, string, error) {
	node, ok := workflowNodeTemplateFromCatalog(catalog, templateID)
	if !ok {
		return graph, "", fmt.Errorf("workflow node %q is not available", templateID)
	}
	baseID := workflowNodeTypeID(node)
	node.ID = baseID
	ordinal := 1
	if _, exists := graph.node(baseID); exists {
		for ordinal = 2; ; ordinal++ {
			candidate := baseID + "-" + strconv.Itoa(ordinal)
			if _, duplicate := graph.node(candidate); !duplicate {
				node.ID = candidate
				break
			}
		}
	}
	if ordinal > 1 {
		node.Title = fmt.Sprintf("%s %d", node.Title, ordinal)
		node.Position = node.Position.Add(image.Pt((ordinal-1)*36, (ordinal-1)*28))
	}
	next := cloneWorkflowGraph(graph)
	next.Nodes = append(next.Nodes, node)
	next.Revision = max(graph.Revision+1, 1)
	return next, node.ID, nil
}

func duplicateWorkflowNode(graph workflowGraphModel, nodeID string) (workflowGraphModel, string, error) {
	source, ok := graph.node(strings.TrimSpace(nodeID))
	if !ok {
		return graph, "", fmt.Errorf("workflow node %q does not exist", nodeID)
	}
	baseID := workflowNodeTypeID(source)
	duplicatedID := baseID
	for ordinal := 2; ; ordinal++ {
		if _, exists := graph.node(duplicatedID); !exists {
			break
		}
		duplicatedID = baseID + "-" + strconv.Itoa(ordinal)
	}
	duplicate := source
	duplicate.ID = duplicatedID
	duplicate.Title = source.Title + " 副本"
	duplicate.Position = source.Position.Add(image.Pt(44, 36))
	duplicate.Inputs = append([]workflowPortModel(nil), source.Inputs...)
	duplicate.Outputs = append([]workflowPortModel(nil), source.Outputs...)
	duplicate.Properties = cloneWorkflowProperties(source.Properties)
	next := cloneWorkflowGraph(graph)
	next.Nodes = append(next.Nodes, duplicate)
	next.Revision = max(graph.Revision+1, 1)
	return next, duplicatedID, nil
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

func setWorkflowNodeTitle(graph workflowGraphModel, nodeID string, title string) workflowGraphModel {
	nodeID = strings.TrimSpace(nodeID)
	title = strings.TrimSpace(title)
	if title == "" {
		return graph
	}
	next := cloneWorkflowGraph(graph)
	for index := range next.Nodes {
		if next.Nodes[index].ID != nodeID || next.Nodes[index].Title == title {
			continue
		}
		next.Nodes[index].Title = title
		next.Revision = max(graph.Revision+1, 1)
		return next
	}
	return graph
}

func setWorkflowNodeProperties(graph workflowGraphModel, nodeID string, properties map[string]string) workflowGraphModel {
	nodeID = strings.TrimSpace(nodeID)
	normalized := cloneWorkflowProperties(properties)
	next := cloneWorkflowGraph(graph)
	for index := range next.Nodes {
		if next.Nodes[index].ID != nodeID {
			continue
		}
		if workflowPropertiesEqual(next.Nodes[index].Properties, normalized) {
			return graph
		}
		next.Nodes[index].Properties = normalized
		next.Revision = max(graph.Revision+1, 1)
		return next
	}
	return graph
}

func configureWorkflowNode(graph workflowGraphModel, nodeID string, title string, properties map[string]string) workflowGraphModel {
	nodeID = strings.TrimSpace(nodeID)
	title = strings.TrimSpace(title)
	normalized := cloneWorkflowProperties(properties)
	next := cloneWorkflowGraph(graph)
	for index := range next.Nodes {
		if next.Nodes[index].ID != nodeID {
			continue
		}
		if title == "" {
			title = next.Nodes[index].Title
		}
		if next.Nodes[index].Title == title && workflowPropertiesEqual(next.Nodes[index].Properties, normalized) {
			return graph
		}
		next.Nodes[index].Title = title
		next.Nodes[index].Properties = normalized
		next.Revision = max(graph.Revision+1, 1)
		return next
	}
	return graph
}

func workflowPropertiesEqual(left map[string]string, right map[string]string) bool {
	left = cloneWorkflowProperties(left)
	right = cloneWorkflowProperties(right)
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
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
