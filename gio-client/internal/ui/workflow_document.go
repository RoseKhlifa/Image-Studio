package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"image-studio/gio-client/internal/desktopstate"
)

const (
	workflowDocumentFormat   = "image-studio-workflow"
	workflowDocumentVersion  = 1
	maxWorkflowDocumentBytes = 2 << 20
)

type workflowDocument struct {
	Format  string                      `json:"format"`
	Version int                         `json:"version"`
	Name    string                      `json:"name,omitempty"`
	Draft   desktopstate.WorkspaceDraft `json:"draft"`
	Graph   desktopstate.WorkflowGraph  `json:"graph"`
}

func marshalWorkflowDocument(name string, draft desktopstate.WorkspaceDraft, graph workflowGraphModel) ([]byte, error) {
	document := workflowDocument{
		Format:  workflowDocumentFormat,
		Version: workflowDocumentVersion,
		Name:    strings.TrimSpace(name),
		Draft:   draft,
		Graph:   desktopWorkflowGraph(graph),
	}
	return json.MarshalIndent(document, "", "  ")
}

func parseWorkflowDocument(data []byte) (workflowDocument, workflowGraphModel, error) {
	if len(data) == 0 {
		return workflowDocument{}, workflowGraphModel{}, fmt.Errorf("工作流文件为空")
	}
	if len(data) > maxWorkflowDocumentBytes {
		return workflowDocument{}, workflowGraphModel{}, fmt.Errorf("工作流文件超过 %d MiB 限制", maxWorkflowDocumentBytes>>20)
	}
	var document workflowDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return workflowDocument{}, workflowGraphModel{}, fmt.Errorf("解析工作流 JSON: %w", err)
	}
	if strings.TrimSpace(document.Format) != workflowDocumentFormat {
		return workflowDocument{}, workflowGraphModel{}, fmt.Errorf("不支持的工作流格式 %q", document.Format)
	}
	if document.Version != workflowDocumentVersion {
		return workflowDocument{}, workflowGraphModel{}, fmt.Errorf("不支持的工作流版本 %d", document.Version)
	}
	seen := make(map[string]struct{}, len(document.Graph.Nodes))
	for _, saved := range document.Graph.Nodes {
		node, ok := workflowNodeTemplate(saved.ID)
		if !ok {
			return workflowDocument{}, workflowGraphModel{}, fmt.Errorf("不支持的工作流节点 %q", saved.ID)
		}
		if _, duplicate := seen[node.ID]; duplicate {
			return workflowDocument{}, workflowGraphModel{}, fmt.Errorf("工作流节点 %q 重复", node.ID)
		}
		seen[node.ID] = struct{}{}
		if kind := strings.TrimSpace(saved.Kind); kind != "" && kind != string(node.Kind) {
			return workflowDocument{}, workflowGraphModel{}, fmt.Errorf("工作流节点 %q 类型不匹配", node.ID)
		}
	}
	document.Graph.Explicit = true
	graph := workflowGraphFromDesktop(document.Graph)
	if len(graph.Nodes) != len(document.Graph.Nodes) || len(graph.Edges) != len(document.Graph.Edges) {
		return workflowDocument{}, workflowGraphModel{}, fmt.Errorf("工作流包含无效节点连接")
	}
	return document, graph, nil
}

func (a *App) exportWorkflowJSON() {
	a.saveActiveWorkspaceSnapshot()
	workspace, ok := a.activeWorkflowWorkspace()
	if !ok {
		a.appendLog("导出工作流失败: 当前工作区不可用")
		return
	}
	document := a.desktopWorkspaceDocument(workspace)
	data, err := marshalWorkflowDocument(a.displayedWorkspaceName(workspace), document.Draft, a.workflowGraph(workspace.ID))
	if err != nil {
		a.appendLog("导出工作流失败: " + err.Error())
		return
	}
	filename := fmt.Sprintf("image-studio-workflow-%s.json", time.Now().Format("20060102-150405"))
	dst, err := chooseSaveJSONFile(filename)
	if err != nil {
		a.appendLog("选择工作流导出文件失败: " + err.Error())
		return
	}
	if strings.TrimSpace(dst) == "" {
		return
	}
	if err := os.WriteFile(dst, data, 0o600); err != nil {
		a.appendLog("写入工作流文件失败: " + err.Error())
		return
	}
	a.appendLog("已导出工作流: " + filepath.Base(dst))
}

func (a *App) importWorkflowJSON() {
	src, err := chooseJSONFile()
	if err != nil {
		a.appendLog("选择工作流文件失败: " + err.Error())
		return
	}
	if strings.TrimSpace(src) == "" {
		return
	}
	data, err := readWorkflowDocumentFile(src)
	if err != nil {
		a.appendLog("读取工作流文件失败: " + err.Error())
		return
	}
	if err := a.applyWorkflowDocument(data); err != nil {
		a.appendLog("导入工作流失败: " + err.Error())
		return
	}
	a.appendLog("已导入工作流: " + filepath.Base(src))
}

func readWorkflowDocumentFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxWorkflowDocumentBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxWorkflowDocumentBytes {
		return nil, fmt.Errorf("工作流文件超过 %d MiB 限制", maxWorkflowDocumentBytes>>20)
	}
	return data, nil
}

func (a *App) applyWorkflowDocument(data []byte) error {
	document, graph, err := parseWorkflowDocument(data)
	if err != nil {
		return err
	}
	a.saveActiveWorkspaceSnapshot()
	workspace, ok := a.activeWorkflowWorkspace()
	if !ok {
		return fmt.Errorf("当前工作区不可用")
	}
	currentGraph := a.workflowGraph(workspace.ID)
	workspaceDocument := a.desktopWorkspaceDocument(workspace)
	workspaceDocument.Draft = document.Draft
	importedWorkspace := a.workspaceFromDesktopDocument(workspaceDocument)
	for index := range a.workspaces {
		if a.workspaces[index].ID == workspace.ID {
			a.workspaces[index] = importedWorkspace
			break
		}
	}
	a.applyWorkspace(importedWorkspace)
	graph.Revision = max(currentGraph.Revision+1, 1)
	if !workflowGraphContentEqual(currentGraph, graph) {
		a.finishWorkflowMove(workspace.ID)
		a.pushWorkflowUndo(workspace.ID, currentGraph)
	}
	a.applyWorkflowGraph(workspace.ID, graph)
	a.workflowCanvas.resetViewport()
	return nil
}

func (a *App) activeWorkflowWorkspace() (workspaceState, bool) {
	for _, workspace := range a.workspaces {
		if workspace.ID == a.activeWorkspaceID {
			return workspace, true
		}
	}
	return workspaceState{}, false
}
