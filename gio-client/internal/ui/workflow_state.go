package ui

import (
	"fmt"
	"image"
	"strings"
)

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
	if strings.TrimSpace(a.workflowSelectedNodes[workspaceID]) == "" {
		a.workflowSelectedNodes[workspaceID] = "generate"
	}
	return graph
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
	a.workflowGraphs[workspaceID] = next
	a.invalidateNow()
}

func (a *App) resetWorkflowGraph(workspaceID string) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return
	}
	current := a.ensureWorkflowGraph(workspaceID)
	next := defaultWorkflowGraph()
	next.Revision = max(next.Revision, current.Revision+1)
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
	delete(a.desktopDraftModels, workspaceID)
}

func (a *App) workflowCanvasData(snap snapshot, workspaceID string) workflowCanvasData {
	graph := a.workflowGraph(workspaceID)
	selected := a.selectedWorkflowNode(workspaceID)
	runtime := make(map[string]workflowNodeRuntime, len(graph.Nodes))
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
		sourceDetail = fmt.Sprintf("已连接 %d 个图像输入", sourceCount)
	} else if a.mode == "edit" {
		sourcePhase = workflowNodePhaseWarning
		sourceDetail = "图生图需要至少一张参考图"
	}
	runtime["source"] = workflowNodeRuntime{Phase: sourcePhase, Detail: sourceDetail, Progress: chooseProgress(sourcePhase, 0)}

	promptPhase := workflowNodePhaseSuccess
	promptDetail := fmt.Sprintf("%d 字符 · %s", len([]rune(prompt)), chooseNonEmpty(a.styleTag, "无风格标签"))
	if prompt == "" {
		promptPhase = workflowNodePhaseWarning
		promptDetail = "等待输入提示词"
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
	runtime["preview"] = workflowNodeRuntime{Phase: previewPhase, Detail: previewDetail, Progress: progress}

	exportPhase := workflowNodePhaseIdle
	exportDetail := chooseNonEmpty(a.outputDirInput.Text(), "等待配置输出目录")
	if strings.TrimSpace(snap.Result.SavedPath) != "" {
		exportPhase = workflowNodePhaseSuccess
		exportDetail = snap.Result.SavedPath
	}
	runtime["export"] = workflowNodeRuntime{Phase: exportPhase, Detail: exportDetail, Progress: chooseProgress(exportPhase, progress)}

	return workflowCanvasData{
		Graph:     graph,
		Selected:  selected,
		Runtime:   runtime,
		Workspace: workspaceID,
	}
}

func chooseProgress(phase workflowNodePhase, fallback float32) float32 {
	if phase == workflowNodePhaseSuccess {
		return 1
	}
	return fallback
}
