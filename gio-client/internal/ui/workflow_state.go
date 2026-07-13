package ui

import (
	"fmt"
	"image"
	"reflect"
	"strings"
)

const workflowGraphHistoryLimit = 64

type workflowMoveTransaction struct {
	NodeID string
	Before workflowGraphModel
}

type workflowGraphHistory struct {
	Undo []workflowGraphModel
	Redo []workflowGraphModel
	Move *workflowMoveTransaction
}

func workflowGraphContentEqual(left workflowGraphModel, right workflowGraphModel) bool {
	left = cloneWorkflowGraph(left)
	right = cloneWorkflowGraph(right)
	left.Revision = 0
	right.Revision = 0
	return reflect.DeepEqual(left, right)
}

func appendWorkflowHistoryEntry(entries []workflowGraphModel, graph workflowGraphModel) []workflowGraphModel {
	entries = append(entries, cloneWorkflowGraph(graph))
	if len(entries) > workflowGraphHistoryLimit {
		entries = append([]workflowGraphModel(nil), entries[len(entries)-workflowGraphHistoryLimit:]...)
	}
	return entries
}

func (a *App) workflowHistory(workspaceID string) *workflowGraphHistory {
	if a.workflowGraphHistories == nil {
		a.workflowGraphHistories = map[string]*workflowGraphHistory{}
	}
	workspaceID = strings.TrimSpace(workspaceID)
	history := a.workflowGraphHistories[workspaceID]
	if history == nil {
		history = new(workflowGraphHistory)
		a.workflowGraphHistories[workspaceID] = history
	}
	return history
}

func (a *App) pushWorkflowUndo(workspaceID string, graph workflowGraphModel) {
	history := a.workflowHistory(workspaceID)
	history.Undo = appendWorkflowHistoryEntry(history.Undo, graph)
	history.Redo = nil
}

func (a *App) applyWorkflowGraph(workspaceID string, graph workflowGraphModel) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return
	}
	if a.workflowGraphs == nil {
		a.workflowGraphs = map[string]workflowGraphModel{}
	}
	a.workflowGraphs[workspaceID] = graph
	if a.workflowSelectedNodes == nil {
		a.workflowSelectedNodes = map[string]string{}
	}
	if _, ok := graph.node(a.workflowSelectedNodes[workspaceID]); !ok {
		a.workflowSelectedNodes[workspaceID] = workflowFallbackNodeID(graph)
	}
	if a.workflowCanvas.workspaceID == workspaceID {
		a.workflowCanvas.clearOverrides()
		a.workflowCanvas.graphRevision = graph.Revision
		a.workflowCanvas.connection = workflowConnectionDrag{}
	}
	a.invalidateNow()
}

func (a *App) beginWorkflowNodeMove(workspaceID string, nodeID string) {
	workspaceID = strings.TrimSpace(workspaceID)
	nodeID = strings.TrimSpace(nodeID)
	if workspaceID == "" || nodeID == "" {
		return
	}
	history := a.workflowHistory(workspaceID)
	if history.Move != nil {
		if history.Move.NodeID == nodeID {
			return
		}
		a.endWorkflowNodeMove(workspaceID, history.Move.NodeID)
	}
	history.Move = &workflowMoveTransaction{NodeID: nodeID, Before: a.workflowGraph(workspaceID)}
}

func (a *App) endWorkflowNodeMove(workspaceID string, nodeID string) {
	workspaceID = strings.TrimSpace(workspaceID)
	history := a.workflowHistory(workspaceID)
	if history.Move == nil || (strings.TrimSpace(nodeID) != "" && history.Move.NodeID != strings.TrimSpace(nodeID)) {
		return
	}
	move := history.Move
	history.Move = nil
	current := a.workflowGraph(workspaceID)
	if !workflowGraphContentEqual(move.Before, current) {
		history.Undo = appendWorkflowHistoryEntry(history.Undo, move.Before)
		history.Redo = nil
	}
}

func (a *App) finishWorkflowMove(workspaceID string) {
	history := a.workflowHistory(workspaceID)
	if history.Move != nil {
		a.endWorkflowNodeMove(workspaceID, history.Move.NodeID)
	}
}

func (a *App) canUndoWorkflowGraph(workspaceID string) bool {
	history := a.workflowHistory(workspaceID)
	if history.Move != nil && !workflowGraphContentEqual(history.Move.Before, a.workflowGraph(workspaceID)) {
		return true
	}
	return len(history.Undo) > 0
}

func (a *App) canRedoWorkflowGraph(workspaceID string) bool {
	return len(a.workflowHistory(workspaceID).Redo) > 0
}

func (a *App) undoWorkflowGraph(workspaceID string) bool {
	workspaceID = strings.TrimSpace(workspaceID)
	a.finishWorkflowMove(workspaceID)
	history := a.workflowHistory(workspaceID)
	if len(history.Undo) == 0 {
		return false
	}
	current := a.workflowGraph(workspaceID)
	restored := cloneWorkflowGraph(history.Undo[len(history.Undo)-1])
	history.Undo = history.Undo[:len(history.Undo)-1]
	history.Redo = appendWorkflowHistoryEntry(history.Redo, current)
	restored.Revision = current.Revision + 1
	a.applyWorkflowGraph(workspaceID, restored)
	return true
}

func (a *App) redoWorkflowGraph(workspaceID string) bool {
	workspaceID = strings.TrimSpace(workspaceID)
	a.finishWorkflowMove(workspaceID)
	history := a.workflowHistory(workspaceID)
	if len(history.Redo) == 0 {
		return false
	}
	current := a.workflowGraph(workspaceID)
	restored := cloneWorkflowGraph(history.Redo[len(history.Redo)-1])
	history.Redo = history.Redo[:len(history.Redo)-1]
	history.Undo = appendWorkflowHistoryEntry(history.Undo, current)
	restored.Revision = current.Revision + 1
	a.applyWorkflowGraph(workspaceID, restored)
	return true
}

func (a *App) ensureWorkflowGraph(workspaceID string) workflowGraphModel {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return defaultWorkflowGraph()
	}
	if a.workflowGraphs == nil {
		a.workflowGraphs = map[string]workflowGraphModel{}
	}
	graph, ok := a.workflowGraphs[workspaceID]
	if !ok {
		graph = defaultWorkflowGraph()
		a.workflowGraphs[workspaceID] = graph
	}
	if a.workflowSelectedNodes == nil {
		a.workflowSelectedNodes = map[string]string{}
	}
	if _, ok := graph.node(a.workflowSelectedNodes[workspaceID]); !ok {
		a.workflowSelectedNodes[workspaceID] = workflowFallbackNodeID(graph)
	}
	return graph
}

func workflowFallbackNodeID(graph workflowGraphModel) string {
	if _, ok := graph.node("generate"); ok {
		return "generate"
	}
	if len(graph.Nodes) > 0 {
		return graph.Nodes[0].ID
	}
	return ""
}

func (a *App) workflowGraph(workspaceID string) workflowGraphModel {
	return cloneWorkflowGraph(a.ensureWorkflowGraph(workspaceID))
}

func (a *App) selectedWorkflowNode(workspaceID string) string {
	a.ensureWorkflowGraph(workspaceID)
	return a.workflowSelectedNodes[workspaceID]
}

func (a *App) selectWorkflowNode(workspaceID string, nodeID string) {
	graph := a.ensureWorkflowGraph(workspaceID)
	if _, ok := graph.node(nodeID); !ok {
		return
	}
	a.workflowSelectedNodes[workspaceID] = nodeID
	a.invalidateNow()
}

func (a *App) setWorkflowNodePosition(workspaceID string, nodeID string, position image.Point) {
	graph := a.ensureWorkflowGraph(workspaceID)
	next := moveWorkflowNode(graph, nodeID, position)
	if next.Revision == graph.Revision {
		return
	}
	history := a.workflowHistory(workspaceID)
	if history.Move == nil || history.Move.NodeID != nodeID {
		a.finishWorkflowMove(workspaceID)
		a.pushWorkflowUndo(workspaceID, graph)
	}
	a.workflowGraphs[workspaceID] = next
	a.invalidateNow()
}

func (a *App) addWorkflowNode(workspaceID string, nodeID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("工作区不可用")
	}
	a.finishWorkflowMove(workspaceID)
	graph := a.ensureWorkflowGraph(workspaceID)
	next, err := addWorkflowNode(graph, nodeID)
	if err != nil {
		return err
	}
	a.pushWorkflowUndo(workspaceID, graph)
	a.workflowSelectedNodes[workspaceID] = strings.TrimSpace(nodeID)
	a.applyWorkflowGraph(workspaceID, next)
	return nil
}

func (a *App) deleteWorkflowNode(workspaceID string, nodeID string) bool {
	workspaceID = strings.TrimSpace(workspaceID)
	nodeID = strings.TrimSpace(nodeID)
	if workspaceID == "" || nodeID == "" {
		return false
	}
	a.finishWorkflowMove(workspaceID)
	graph := a.ensureWorkflowGraph(workspaceID)
	next := removeWorkflowNode(graph, nodeID)
	if next.Revision == graph.Revision {
		return false
	}
	a.pushWorkflowUndo(workspaceID, graph)
	a.applyWorkflowGraph(workspaceID, next)
	return true
}

func (a *App) deleteSelectedWorkflowNode(workspaceID string) bool {
	return a.deleteWorkflowNode(workspaceID, a.selectedWorkflowNode(workspaceID))
}

func (a *App) toggleWorkflowNodeEnabled(workspaceID string, nodeID string) bool {
	workspaceID = strings.TrimSpace(workspaceID)
	nodeID = strings.TrimSpace(nodeID)
	if workspaceID == "" || nodeID == "" {
		return false
	}
	a.finishWorkflowMove(workspaceID)
	graph := a.ensureWorkflowGraph(workspaceID)
	node, ok := graph.node(nodeID)
	if !ok {
		return false
	}
	next := setWorkflowNodeEnabled(graph, nodeID, !node.Enabled)
	if next.Revision == graph.Revision {
		return false
	}
	a.pushWorkflowUndo(workspaceID, graph)
	a.applyWorkflowGraph(workspaceID, next)
	return true
}

func (a *App) toggleSelectedWorkflowNodeEnabled(workspaceID string) bool {
	return a.toggleWorkflowNodeEnabled(workspaceID, a.selectedWorkflowNode(workspaceID))
}

func (a *App) toggleWorkflowConnection(workspaceID string, edge workflowEdgeModel) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("工作区不可用")
	}
	a.finishWorkflowMove(workspaceID)
	graph := a.ensureWorkflowGraph(workspaceID)
	next, err := toggleWorkflowConnection(graph, edge)
	if err != nil {
		return err
	}
	a.pushWorkflowUndo(workspaceID, graph)
	a.applyWorkflowGraph(workspaceID, next)
	return nil
}

func (a *App) rewireWorkflowConnection(workspaceID string, previous *workflowEdgeModel, replacement *workflowEdgeModel) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return fmt.Errorf("工作区不可用")
	}
	a.finishWorkflowMove(workspaceID)
	graph := a.ensureWorkflowGraph(workspaceID)
	next, err := rewireWorkflowConnection(graph, previous, replacement)
	if err != nil {
		return err
	}
	if next.Revision == graph.Revision {
		return nil
	}
	a.pushWorkflowUndo(workspaceID, graph)
	a.applyWorkflowGraph(workspaceID, next)
	return nil
}

func (a *App) resetWorkflowGraph(workspaceID string) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return
	}
	a.finishWorkflowMove(workspaceID)
	current := a.ensureWorkflowGraph(workspaceID)
	next := defaultWorkflowGraph()
	next.Revision = max(next.Revision, current.Revision+1)
	if !workflowGraphContentEqual(current, next) {
		a.pushWorkflowUndo(workspaceID, current)
	}
	a.workflowGraphs[workspaceID] = next
	a.workflowSelectedNodes[workspaceID] = "generate"
	if a.workflowCanvas.workspaceID == workspaceID {
		a.workflowCanvas.clearOverrides()
		a.workflowCanvas.graphRevision = next.Revision
	}
	a.workflowCanvas.resetViewport()
	a.invalidateNow()
}

func (a *App) deleteWorkflowWorkspaceState(workspaceID string) {
	delete(a.workflowGraphs, workspaceID)
	delete(a.workflowSelectedNodes, workspaceID)
	delete(a.workflowGraphHistories, workspaceID)
	delete(a.desktopDraftModels, workspaceID)
}

func (a *App) workflowCanvasData(snap snapshot, workspaceID string) workflowCanvasData {
	graph := a.workflowGraph(workspaceID)
	selected := a.selectedWorkflowNode(workspaceID)
	runtime := make(map[string]workflowNodeRuntime, len(graph.Nodes))
	promptConnected := workflowEdgeConnected(graph, workflowEdgeModel{FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"})
	sourceConnected := workflowEdgeConnected(graph, workflowEdgeModel{FromNode: "source", FromPort: "image", ToNode: "generate", ToPort: "source"})
	previewConnected := workflowEdgeConnected(graph, workflowEdgeModel{FromNode: "generate", FromPort: "job", ToNode: "preview", ToPort: "job"})
	exportConnected := workflowEdgeConnected(graph, workflowEdgeModel{FromNode: "preview", FromPort: "image", ToNode: "export", ToPort: "image"})
	sourceCount := len(a.parseSourcePathsCached(a.sourcePathsInput.Text()))
	prompt := strings.TrimSpace(a.promptInput.Text())
	completed := len(snap.BatchResults)
	if completed == 0 && snap.Result.HasItem {
		completed = 1
	}
	total := snap.BatchTotal
	if total < 1 {
		total = max(a.batchCount, 1)
	}
	progress := float32(completed) / float32(max(total, 1))
	if progress > 1 {
		progress = 1
	}

	sourcePhase := workflowNodePhaseIdle
	sourceDetail := "无需参考图，当前为文生图"
	if sourceCount > 0 {
		sourcePhase = workflowNodePhaseSuccess
		sourceDetail = fmt.Sprintf("已载入 %d 个图像输入", sourceCount)
	} else if a.mode == "edit" {
		sourcePhase = workflowNodePhaseWarning
		sourceDetail = "图生图需要至少一张参考图"
	}
	if !sourceConnected && (sourceCount > 0 || a.mode == "edit" || a.batchMode) {
		sourcePhase = workflowNodePhaseWarning
		sourceDetail = "图像输入端口未连接"
	}
	runtime["source"] = workflowNodeRuntime{Phase: sourcePhase, Detail: sourceDetail, Progress: chooseProgress(sourcePhase, 0)}

	promptPhase := workflowNodePhaseSuccess
	promptDetail := fmt.Sprintf("%d 字符 · %s", len([]rune(prompt)), chooseNonEmpty(a.styleTag, "无风格标签"))
	if prompt == "" {
		promptPhase = workflowNodePhaseWarning
		promptDetail = "等待输入提示词"
	} else if !promptConnected {
		promptPhase = workflowNodePhaseWarning
		promptDetail = "提示词端口未连接"
	}
	runtime["prompt"] = workflowNodeRuntime{Phase: promptPhase, Detail: promptDetail, Progress: chooseProgress(promptPhase, 0)}

	generatePhase := workflowNodePhaseIdle
	generateDetail := fmt.Sprintf("%s · %s · %s", strings.ToUpper(a.size), a.quality, a.imageModelInput.Text())
	if snap.Running {
		generatePhase = workflowNodePhaseRunning
		generateDetail = snap.Status
	} else if strings.TrimSpace(snap.LastErrorMessage) != "" {
		generatePhase = workflowNodePhaseError
		generateDetail = snap.LastErrorMessage
	} else if snap.Result.HasItem || strings.TrimSpace(snap.Result.SavedPath) != "" {
		generatePhase = workflowNodePhaseSuccess
		generateDetail = "生成任务已完成"
	}
	if !snap.Running && (!promptConnected || ((a.mode == "edit" || a.batchMode) && !sourceConnected)) {
		generatePhase = workflowNodePhaseWarning
		generateDetail = "等待必需输入连接"
	}
	runtime["generate"] = workflowNodeRuntime{Phase: generatePhase, Detail: generateDetail, Progress: progress}

	previewPhase := workflowNodePhaseIdle
	previewDetail := "等待任务输出"
	if snap.Running && (len(snap.BatchPreviewItems) > 0 || snap.Result.Image != nil) {
		previewPhase = workflowNodePhaseRunning
		previewDetail = fmt.Sprintf("实时预览 %d/%d", max(completed, len(snap.BatchPreviewItems)), total)
	} else if snap.Result.Image != nil || snap.Result.HasItem {
		previewPhase = workflowNodePhaseSuccess
		previewDetail = "画布结果可用"
	}
	if !snap.Running && !previewConnected {
		previewPhase = workflowNodePhaseWarning
		previewDetail = "任务输入端口未连接"
	}
	runtime["preview"] = workflowNodeRuntime{Phase: previewPhase, Detail: previewDetail, Progress: progress}

	exportPhase := workflowNodePhaseIdle
	exportDetail := chooseNonEmpty(a.outputDirInput.Text(), "等待配置输出目录")
	if strings.TrimSpace(snap.Result.SavedPath) != "" {
		exportPhase = workflowNodePhaseSuccess
		exportDetail = snap.Result.SavedPath
	}
	if !snap.Running && !exportConnected {
		exportPhase = workflowNodePhaseWarning
		exportDetail = "图像输入端口未连接"
	}
	runtime["export"] = workflowNodeRuntime{Phase: exportPhase, Detail: exportDetail, Progress: chooseProgress(exportPhase, progress)}
	applyDisabledWorkflowRuntime(graph, runtime)

	return workflowCanvasData{
		Graph:     graph,
		Selected:  selected,
		Runtime:   runtime,
		Workspace: workspaceID,
	}
}

func applyDisabledWorkflowRuntime(graph workflowGraphModel, runtime map[string]workflowNodeRuntime) {
	for _, node := range graph.Nodes {
		if !node.Enabled {
			runtime[node.ID] = workflowNodeRuntime{Phase: workflowNodePhaseWarning, Detail: "节点已停用"}
		}
	}
}

func chooseProgress(phase workflowNodePhase, fallback float32) float32 {
	if phase == workflowNodePhaseSuccess {
		return 1
	}
	return fallback
}
