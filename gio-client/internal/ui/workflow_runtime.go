package ui

import (
	"fmt"
	"strings"

	"image-studio/gio-client/internal/kernel"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

type workflowRuntimeContext struct {
	PreferredOutputID string
	BatchMode         bool
	Running           bool
	Status            string
	LastError         string
	ResultAvailable   bool
	ResultSavedPath   string
	PreviewAvailable  bool
	PreviewCount      int
	Completed         int
	Total             int
}

func workflowNodeHasOutputConnection(graph workflowGraphModel, nodeID string, portID string, targetKind workflowNodeKind, targetPortID string) bool {
	for _, edge := range graph.Edges {
		if edge.FromNode != nodeID || edge.FromPort != portID || edge.ToPort != targetPortID {
			continue
		}
		target, ok := workflowEnabledNode(graph, edge.ToNode)
		if ok && target.Kind == targetKind {
			return true
		}
	}
	return false
}

func workflowValidPromptInput(graph workflowGraphModel, generate workflowNodeModel) bool {
	edge, ok := workflowInputEdge(graph, generate.ID, "prompt")
	if !ok || edge.FromPort != "text" {
		return false
	}
	prompt, enabled := workflowEnabledNode(graph, edge.FromNode)
	return enabled && prompt.Kind == workflowNodePrompt && strings.TrimSpace(workflowNodePropertyValue(prompt, workflowPropertyPrompt)) != ""
}

func workflowValidSourceInputs(graph workflowGraphModel, generate workflowNodeModel) (int, int, bool) {
	edges := workflowInputEdges(graph, generate.ID, "source")
	connected := 0
	configured := 0
	for _, edge := range edges {
		source, enabled := workflowEnabledNode(graph, edge.FromNode)
		if !enabled || source.Kind != workflowNodeSource || edge.FromPort != "image" {
			return connected, configured, false
		}
		connected++
		configured += len(kernel.ParseSourcePaths(workflowNodePropertyValue(source, workflowPropertySourcePaths)))
	}
	return connected, configured, true
}

func workflowValidPreviewInput(graph workflowGraphModel, preview workflowNodeModel) bool {
	edge, ok := workflowInputEdge(graph, preview.ID, "job")
	if !ok || edge.FromPort != "job" {
		return false
	}
	generate, enabled := workflowEnabledNode(graph, edge.FromNode)
	return enabled && generate.Kind == workflowNodeGenerate
}

func workflowValidExportInput(graph workflowGraphModel, export workflowNodeModel) bool {
	edge, ok := workflowInputEdge(graph, export.ID, "image")
	if !ok {
		return false
	}
	upstream, enabled := workflowEnabledNode(graph, edge.FromNode)
	if !enabled || edge.FromPort != "image" {
		return false
	}
	return upstream.Kind == workflowNodeGenerate || upstream.Kind == workflowNodePreview
}

func workflowPlanNodeSet(plan workflowExecutionPlan) map[string]bool {
	active := map[string]bool{
		plan.Prompt.ID:   true,
		plan.Generate.ID: true,
		plan.Export.ID:   true,
	}
	for _, source := range plan.Sources {
		active[source.ID] = true
	}
	if plan.Preview != nil {
		active[plan.Preview.ID] = true
	}
	delete(active, "")
	return active
}

func workflowRuntimeForGraph(graph workflowGraphModel, ctx workflowRuntimeContext) map[string]workflowNodeRuntime {
	graph = normalizeWorkflowGraph(graph)
	runtime := make(map[string]workflowNodeRuntime, len(graph.Nodes))
	plan, planErr := buildWorkflowExecutionPlan(graph, ctx.PreferredOutputID, false)
	active := map[string]bool{}
	if planErr == nil {
		active = workflowPlanNodeSet(plan)
	}
	connectedExports := connectedWorkflowExports(graph)
	total := max(ctx.Total, 1)
	progress := float32(ctx.Completed) / float32(total)
	if progress > 1 {
		progress = 1
	}

	for _, node := range graph.Nodes {
		switch node.Kind {
		case workflowNodePrompt:
			prompt := strings.TrimSpace(workflowNodePropertyValue(node, workflowPropertyPrompt))
			style := strings.TrimSpace(workflowNodePropertyValue(node, workflowPropertyStyleTag))
			connected := workflowNodeHasOutputConnection(graph, node.ID, "text", workflowNodeGenerate, "prompt")
			phase := workflowNodePhaseSuccess
			detail := fmt.Sprintf("%d 字符 · %s", len([]rune(prompt)), chooseNonEmpty(style, "无风格标签"))
			if prompt == "" {
				phase = workflowNodePhaseWarning
				detail = "等待输入提示词"
			} else if !connected {
				phase = workflowNodePhaseWarning
				detail = "提示词端口未连接"
			}
			runtime[node.ID] = workflowNodeRuntime{Phase: phase, Detail: detail, Progress: chooseProgress(phase, 0)}

		case workflowNodeSource:
			raw := workflowNodePropertyValue(node, workflowPropertySourcePaths)
			count := len(kernel.ParseSourcePaths(raw))
			connected := workflowNodeHasOutputConnection(graph, node.ID, "image", workflowNodeGenerate, "source")
			phase := workflowNodePhaseIdle
			detail := "没有配置参考图"
			if count > 0 {
				phase = workflowNodePhaseSuccess
				detail = fmt.Sprintf("已载入 %d 个图像输入", count)
			}
			if count > 0 && !connected {
				phase = workflowNodePhaseWarning
				detail = "图像输入端口未连接"
			}
			if active[node.ID] && plan.Generate.ID != "" {
				mode := client.Mode(workflowNodePropertyValue(plan.Generate, workflowPropertyMode))
				if (mode == client.ModeEdit || ctx.BatchMode) && count == 0 {
					phase = workflowNodePhaseWarning
					detail = "当前分支需要至少一张参考图"
				}
			}
			runtime[node.ID] = workflowNodeRuntime{Phase: phase, Detail: detail, Progress: chooseProgress(phase, 0)}

		case workflowNodeGenerate:
			mode := workflowNodePropertyValue(node, workflowPropertyMode)
			size := workflowNodePropertyValue(node, workflowPropertySize)
			quality := workflowNodePropertyValue(node, workflowPropertyQuality)
			model := workflowNodePropertyValue(node, workflowPropertyImageModel)
			phase := workflowNodePhaseIdle
			detail := fmt.Sprintf("%s · %s · %s", strings.ToUpper(size), quality, model)
			promptConnected := workflowValidPromptInput(graph, node)
			sourceCount, configuredSources, sourcesValid := workflowValidSourceInputs(graph, node)
			requireSource := client.Mode(mode) == client.ModeEdit || ctx.BatchMode
			missingConfiguredSource := client.Mode(mode) == client.ModeEdit && !ctx.BatchMode && configuredSources == 0
			if !promptConnected || !sourcesValid || (requireSource && sourceCount == 0) || missingConfiguredSource {
				phase = workflowNodePhaseWarning
				detail = "等待必需输入连接"
			} else if active[node.ID] && ctx.Running {
				phase = workflowNodePhaseRunning
				detail = ctx.Status
			} else if active[node.ID] && strings.TrimSpace(ctx.LastError) != "" {
				phase = workflowNodePhaseError
				detail = ctx.LastError
			} else if active[node.ID] && ctx.ResultAvailable {
				phase = workflowNodePhaseSuccess
				detail = "生成任务已完成"
			}
			runtime[node.ID] = workflowNodeRuntime{Phase: phase, Detail: detail, Progress: chooseProgress(phase, progress)}

		case workflowNodePreview:
			phase := workflowNodePhaseIdle
			detail := "等待任务输出"
			if !workflowValidPreviewInput(graph, node) {
				phase = workflowNodePhaseWarning
				detail = "任务输入端口未连接"
			} else if active[node.ID] && ctx.Running && ctx.PreviewAvailable {
				phase = workflowNodePhaseRunning
				detail = fmt.Sprintf("实时预览 %d/%d", max(ctx.Completed, ctx.PreviewCount), total)
			} else if active[node.ID] && ctx.ResultAvailable {
				phase = workflowNodePhaseSuccess
				detail = "画布结果可用"
			}
			runtime[node.ID] = workflowNodeRuntime{Phase: phase, Detail: detail, Progress: chooseProgress(phase, progress)}

		case workflowNodeExport:
			outputDir := workflowNodePropertyValue(node, workflowPropertyOutputDir)
			phase := workflowNodePhaseIdle
			detail := chooseNonEmpty(outputDir, "等待配置输出目录")
			if !workflowValidExportInput(graph, node) {
				phase = workflowNodePhaseWarning
				detail = "图像输入端口未连接"
			} else if active[node.ID] && strings.TrimSpace(ctx.ResultSavedPath) != "" {
				phase = workflowNodePhaseSuccess
				detail = ctx.ResultSavedPath
			} else if len(connectedExports) > 1 && !active[node.ID] {
				detail = "选择此导出节点以运行该分支"
			}
			runtime[node.ID] = workflowNodeRuntime{Phase: phase, Detail: detail, Progress: chooseProgress(phase, progress)}
		}
	}
	applyDisabledWorkflowRuntime(graph, runtime)
	return runtime
}
