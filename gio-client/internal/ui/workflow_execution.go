package ui

import (
	"fmt"
	"strconv"
	"strings"

	"image-studio/gio-client/internal/kernel"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

type workflowExecutionPlan struct {
	Prompt   workflowNodeModel
	Sources  []workflowNodeModel
	Generate workflowNodeModel
	Preview  *workflowNodeModel
	Export   workflowNodeModel
	Direct   bool
}

func workflowEnabledNode(graph workflowGraphModel, nodeID string) (workflowNodeModel, bool) {
	node, ok := graph.node(nodeID)
	return node, ok && node.Enabled
}

func workflowInputEdge(graph workflowGraphModel, nodeID string, portID string) (workflowEdgeModel, bool) {
	edges := workflowInputEdges(graph, nodeID, portID)
	if len(edges) != 1 {
		return workflowEdgeModel{}, false
	}
	return edges[0], true
}

func workflowNodeHasProperty(node workflowNodeModel, key string) bool {
	_, ok := node.Properties[key]
	return ok
}

func workflowChoiceContains(choices []choice, value string) bool {
	for _, item := range choices {
		if item.Value == value {
			return true
		}
	}
	return false
}

func validateWorkflowExecutionPlanProperties(plan workflowExecutionPlan) error {
	for _, key := range []string{workflowPropertyPrompt, workflowPropertyNegative, workflowPropertyStyleTag} {
		if !workflowNodeHasProperty(plan.Prompt, key) {
			return fmt.Errorf("提示词节点 %s 缺少属性 %s", plan.Prompt.Title, key)
		}
	}
	for _, source := range plan.Sources {
		if !workflowNodeHasProperty(source, workflowPropertySourcePaths) {
			return fmt.Errorf("参考图节点 %s 缺少属性 %s", source.Title, workflowPropertySourcePaths)
		}
	}
	mode := strings.TrimSpace(workflowNodePropertyValue(plan.Generate, workflowPropertyMode))
	if !workflowChoiceContains(modeChoices, mode) {
		return fmt.Errorf("生成节点 %s 的模式无效", plan.Generate.Title)
	}
	quality := strings.TrimSpace(workflowNodePropertyValue(plan.Generate, workflowPropertyQuality))
	if !workflowChoiceContains(qualityChoices, quality) {
		return fmt.Errorf("生成节点 %s 的质量无效", plan.Generate.Title)
	}
	if strings.TrimSpace(workflowNodePropertyValue(plan.Generate, workflowPropertySize)) == "" {
		return fmt.Errorf("生成节点 %s 缺少尺寸", plan.Generate.Title)
	}
	if strings.TrimSpace(workflowNodePropertyValue(plan.Generate, workflowPropertyImageModel)) == "" {
		return fmt.Errorf("生成节点 %s 缺少图像模型", plan.Generate.Title)
	}
	batchCount, err := strconv.Atoi(strings.TrimSpace(workflowNodePropertyValue(plan.Generate, workflowPropertyBatchCount)))
	if err != nil || batchCount < 1 || batchCount > 9 {
		return fmt.Errorf("生成节点 %s 的生成张数无效", plan.Generate.Title)
	}
	if plan.Preview != nil {
		partialImages := strings.TrimSpace(workflowNodePropertyValue(*plan.Preview, workflowPropertyPartialImages))
		if !workflowChoiceContains(partialPreviewChoices, partialImages) {
			return fmt.Errorf("预览节点 %s 的预览帧数无效", plan.Preview.Title)
		}
	}
	format := strings.TrimSpace(workflowNodePropertyValue(plan.Export, workflowPropertyOutputFormat))
	if !workflowChoiceContains(formatChoices, format) {
		return fmt.Errorf("导出节点 %s 的输出格式无效", plan.Export.Title)
	}
	if strings.TrimSpace(workflowNodePropertyValue(plan.Export, workflowPropertyOutputDir)) == "" {
		return fmt.Errorf("导出节点 %s 缺少输出目录", plan.Export.Title)
	}
	return nil
}

func connectedWorkflowExports(graph workflowGraphModel) []workflowNodeModel {
	outputs := make([]workflowNodeModel, 0, 1)
	for _, node := range graph.Nodes {
		if !node.Enabled || node.Kind != workflowNodeExport {
			continue
		}
		if edge, ok := workflowInputEdge(graph, node.ID, "image"); ok {
			if _, enabled := workflowEnabledNode(graph, edge.FromNode); enabled {
				outputs = append(outputs, node)
			}
		}
	}
	return outputs
}

func buildWorkflowExecutionPlan(graph workflowGraphModel, preferredOutputNodeID string, requireSource bool) (workflowExecutionPlan, error) {
	graph = normalizeWorkflowGraph(graph)
	outputs := connectedWorkflowExports(graph)
	if len(outputs) == 0 {
		return workflowExecutionPlan{}, fmt.Errorf("至少需要一个已连接的导出节点")
	}
	output := workflowNodeModel{}
	preferredOutputNodeID = strings.TrimSpace(preferredOutputNodeID)
	if preferredOutputNodeID != "" {
		preferred, exists := workflowEnabledNode(graph, preferredOutputNodeID)
		if !exists || preferred.Kind != workflowNodeExport {
			return workflowExecutionPlan{}, fmt.Errorf("选中的导出节点不可用")
		}
		for _, candidate := range outputs {
			if candidate.ID == preferredOutputNodeID {
				output = candidate
				break
			}
		}
		if output.ID == "" {
			return workflowExecutionPlan{}, fmt.Errorf("选中的导出节点 %s 缺少图像输入", preferred.Title)
		}
	}
	if output.ID == "" {
		if len(outputs) > 1 {
			return workflowExecutionPlan{}, fmt.Errorf("存在 %d 个输出分支，请先选中要运行的导出节点", len(outputs))
		}
		output = outputs[0]
	}

	plan := workflowExecutionPlan{Export: output}
	exportEdge, _ := workflowInputEdge(graph, output.ID, "image")
	upstream, ok := workflowEnabledNode(graph, exportEdge.FromNode)
	if !ok {
		return workflowExecutionPlan{}, fmt.Errorf("导出节点 %s 的上游不可用", output.Title)
	}
	switch upstream.Kind {
	case workflowNodeGenerate:
		if exportEdge.FromPort != "image" {
			return workflowExecutionPlan{}, fmt.Errorf("生成节点 %s 未使用最终图像端口", upstream.Title)
		}
		plan.Generate = upstream
		plan.Direct = true
	case workflowNodePreview:
		if exportEdge.FromPort != "image" {
			return workflowExecutionPlan{}, fmt.Errorf("预览节点 %s 未使用图像端口", upstream.Title)
		}
		preview := upstream
		plan.Preview = &preview
		previewEdge, connected := workflowInputEdge(graph, preview.ID, "job")
		if !connected {
			return workflowExecutionPlan{}, fmt.Errorf("预览节点 %s 缺少任务输入", preview.Title)
		}
		generate, enabled := workflowEnabledNode(graph, previewEdge.FromNode)
		if !enabled || generate.Kind != workflowNodeGenerate || previewEdge.FromPort != "job" {
			return workflowExecutionPlan{}, fmt.Errorf("预览节点 %s 必须连接图像生成任务", preview.Title)
		}
		plan.Generate = generate
	default:
		return workflowExecutionPlan{}, fmt.Errorf("导出节点 %s 不能接收 %s", output.Title, upstream.Title)
	}

	promptEdge, connected := workflowInputEdge(graph, plan.Generate.ID, "prompt")
	if !connected {
		return workflowExecutionPlan{}, fmt.Errorf("生成节点 %s 缺少提示词输入", plan.Generate.Title)
	}
	prompt, enabled := workflowEnabledNode(graph, promptEdge.FromNode)
	if !enabled || prompt.Kind != workflowNodePrompt || promptEdge.FromPort != "text" {
		return workflowExecutionPlan{}, fmt.Errorf("生成节点 %s 必须连接提示词节点", plan.Generate.Title)
	}
	plan.Prompt = prompt

	for _, edge := range workflowInputEdges(graph, plan.Generate.ID, "source") {
		source, enabled := workflowEnabledNode(graph, edge.FromNode)
		if !enabled || source.Kind != workflowNodeSource || edge.FromPort != "image" {
			return workflowExecutionPlan{}, fmt.Errorf("生成节点 %s 包含无效参考图输入", plan.Generate.Title)
		}
		plan.Sources = append(plan.Sources, source)
	}
	if requireSource && len(plan.Sources) == 0 {
		return workflowExecutionPlan{}, fmt.Errorf("生成节点 %s 在图生图模式下需要参考图输入", plan.Generate.Title)
	}
	if err := validateWorkflowExecutionPlanProperties(plan); err != nil {
		return workflowExecutionPlan{}, err
	}
	return plan, nil
}

func validateWorkflowForRun(graph workflowGraphModel, requireSource bool) error {
	_, err := buildWorkflowExecutionPlan(graph, "", requireSource)
	return err
}

func workflowPreferredOutputNodeID(graph workflowGraphModel, selectedNodeID string) string {
	selected, ok := graph.node(strings.TrimSpace(selectedNodeID))
	if !ok || !selected.Enabled || selected.Kind != workflowNodeExport {
		return ""
	}
	return selected.ID
}

func appendWorkflowSourceValues(paths []string, dataURLs []string, values []string) ([]string, []string) {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if dataURL, ok := virtualImageDataURL(value); ok {
			dataURLs = append(dataURLs, dataURL)
			continue
		}
		paths = append(paths, value)
	}
	return paths, dataURLs
}

func (a *App) applyWorkflowExecutionPlan(cfg kernel.Config, plan workflowExecutionPlan) (kernel.Config, int) {
	cfg.Prompt = a.currentCanvasAugmentedPrompt(workflowNodePropertyValue(plan.Prompt, workflowPropertyPrompt))
	cfg.NegativePrompt = workflowNodePropertyValue(plan.Prompt, workflowPropertyNegative)
	cfg.StyleTag = workflowNodePropertyValue(plan.Prompt, workflowPropertyStyleTag)

	cfg.Mode = client.Mode(workflowNodePropertyValue(plan.Generate, workflowPropertyMode))
	cfg.Quality = workflowNodePropertyValue(plan.Generate, workflowPropertyQuality)
	cfg.ImageModelID = strings.TrimSpace(workflowNodePropertyValue(plan.Generate, workflowPropertyImageModel))
	size := workflowNodePropertyValue(plan.Generate, workflowPropertySize)
	cfg.Size = normalizeSizeSelection(size, a.api, a.policy, cfg.ImageModelID, a.customAspectRatios)

	total, _ := strconv.Atoi(strings.TrimSpace(workflowNodePropertyValue(plan.Generate, workflowPropertyBatchCount)))

	sourcePaths := make([]string, 0)
	sourceDataURLs := make([]string, 0)
	for _, source := range plan.Sources {
		raw := workflowNodePropertyValue(source, workflowPropertySourcePaths)
		sourcePaths, sourceDataURLs = appendWorkflowSourceValues(sourcePaths, sourceDataURLs, kernel.ParseSourcePaths(raw))
	}
	cfg.SourcePaths = sourcePaths
	cfg.SourceImageDataURLs = sourceDataURLs
	if cfg.Mode == client.ModeEdit {
		cfg.MaskB64 = a.currentCanvasMaskB64()
		cfg.ParentID = ""
		if len(sourcePaths) > 0 {
			cfg.ParentID = sourcePaths[0]
		}
	} else {
		cfg.MaskB64 = ""
		cfg.ParentID = ""
	}

	if plan.Preview != nil {
		cfg.PartialImages, _ = strconv.Atoi(strings.TrimSpace(workflowNodePropertyValue(*plan.Preview, workflowPropertyPartialImages)))
	} else {
		cfg.PartialImages = 0
	}
	cfg.OutputFormat = workflowNodePropertyValue(plan.Export, workflowPropertyOutputFormat)
	cfg.OutputDir = strings.TrimSpace(workflowNodePropertyValue(plan.Export, workflowPropertyOutputDir))
	return cfg, total
}
