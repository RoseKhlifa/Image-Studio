package ui

import (
	"strconv"
	"strings"

	"gioui.org/widget"
)

const (
	workflowPropertyPrompt        = "prompt"
	workflowPropertyNegative      = "negative_prompt"
	workflowPropertyStyleTag      = "style_tag"
	workflowPropertySourcePaths   = "source_paths"
	workflowPropertyMode          = "mode"
	workflowPropertyQuality       = "quality"
	workflowPropertySize          = "size"
	workflowPropertyImageModel    = "image_model"
	workflowPropertyBatchCount    = "batch_count"
	workflowPropertyPartialImages = "partial_images"
	workflowPropertyOutputFormat  = "output_format"
	workflowPropertyOutputDir     = "output_dir"
)

func workflowNodePropertyValue(node workflowNodeModel, key string) string {
	return node.Properties[key]
}

func workflowNodeEditorKey(workspaceID string, nodeID string) string {
	return strings.TrimSpace(workspaceID) + "|" + strings.TrimSpace(nodeID)
}

func (a *App) clearWorkflowNodeEditor(workspaceID string) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" || strings.HasPrefix(a.workflowEditingNodeKey, workspaceID+"|") {
		a.workflowEditingNodeKey = ""
	}
}

func (a *App) workflowPropertiesFromControls(kind workflowNodeKind, existing map[string]string) map[string]string {
	properties := cloneWorkflowProperties(existing)
	if properties == nil {
		properties = map[string]string{}
	}
	switch kind {
	case workflowNodePrompt:
		properties[workflowPropertyPrompt] = a.promptInput.Text()
		properties[workflowPropertyNegative] = a.negativePromptInput.Text()
		properties[workflowPropertyStyleTag] = a.styleTag
	case workflowNodeSource:
		properties[workflowPropertySourcePaths] = a.sourcePathsInput.Text()
	case workflowNodeGenerate:
		properties[workflowPropertyMode] = a.mode
		properties[workflowPropertyQuality] = a.quality
		properties[workflowPropertySize] = a.size
		properties[workflowPropertyImageModel] = a.imageModelInput.Text()
		properties[workflowPropertyBatchCount] = strconv.Itoa(max(a.batchCount, 1))
	case workflowNodePreview:
		properties[workflowPropertyPartialImages] = a.partialImagesInput.Text()
	case workflowNodeExport:
		properties[workflowPropertyOutputFormat] = a.format
		properties[workflowPropertyOutputDir] = a.outputDirInput.Text()
	}
	return properties
}

func (a *App) workflowGraphWithControlProperties(graph workflowGraphModel) workflowGraphModel {
	next := cloneWorkflowGraph(graph)
	for index := range next.Nodes {
		next.Nodes[index].Properties = a.workflowPropertiesFromControls(next.Nodes[index].Kind, next.Nodes[index].Properties)
	}
	return next
}

func (a *App) loadWorkflowNodeControls(workspaceID string, node workflowNodeModel) {
	a.workflowEditingNodeKey = workflowNodeEditorKey(workspaceID, node.ID)
	a.workflowNodeTitleInput.SetText(node.Title)
	switch node.Kind {
	case workflowNodePrompt:
		a.promptInput.SetText(workflowNodePropertyValue(node, workflowPropertyPrompt))
		a.negativePromptInput.SetText(workflowNodePropertyValue(node, workflowPropertyNegative))
		a.styleTag = workflowNodePropertyValue(node, workflowPropertyStyleTag)
	case workflowNodeSource:
		a.sourcePathsInput.SetText(workflowNodePropertyValue(node, workflowPropertySourcePaths))
		a.sourceButtons = map[string]*widget.Clickable{}
	case workflowNodeGenerate:
		a.mode = workflowNodePropertyValue(node, workflowPropertyMode)
		a.quality = workflowNodePropertyValue(node, workflowPropertyQuality)
		a.size = workflowNodePropertyValue(node, workflowPropertySize)
		a.imageModelInput.SetText(workflowNodePropertyValue(node, workflowPropertyImageModel))
		if count, err := strconv.Atoi(strings.TrimSpace(workflowNodePropertyValue(node, workflowPropertyBatchCount))); err == nil {
			a.batchCount = normalizeBatchCount(count)
		}
	case workflowNodePreview:
		a.partialImagesInput.SetText(workflowNodePropertyValue(node, workflowPropertyPartialImages))
	case workflowNodeExport:
		a.format = workflowNodePropertyValue(node, workflowPropertyOutputFormat)
		a.outputDirInput.SetText(workflowNodePropertyValue(node, workflowPropertyOutputDir))
	}
}

func (a *App) ensureWorkflowNodeControlsLoaded(workspaceID string, node workflowNodeModel) {
	if node.ID == "" {
		return
	}
	if a.workflowEditingNodeKey != workflowNodeEditorKey(workspaceID, node.ID) {
		a.loadWorkflowNodeControls(workspaceID, node)
	}
}

func (a *App) syncWorkflowNodeControls(workspaceID string, node workflowNodeModel, recordHistory bool) bool {
	if node.ID == "" || a.workflowEditingNodeKey != workflowNodeEditorKey(workspaceID, node.ID) {
		return false
	}
	properties := a.workflowPropertiesFromControls(node.Kind, node.Properties)
	return a.configureWorkflowNode(workspaceID, node.ID, a.workflowNodeTitleInput.Text(), properties, recordHistory)
}
