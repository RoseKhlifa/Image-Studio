package ui

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"

	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/desktopstate"
	"image-studio/gio-client/internal/kernel"
	"image-studio/gio-client/internal/windowing"
	sharedCompat "image-studio/shared/compat"

	"gioui.org/unit"
)

const (
	experienceModeSimple   = "simple"
	experienceModeWorkflow = "workflow"
)

func normalizeExperienceMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), experienceModeWorkflow) {
		return experienceModeWorkflow
	}
	return experienceModeSimple
}

func loadGioDesktopState() (*desktopstate.Store, desktopstate.State, error) {
	root, err := gioCompat.StableDataRoot()
	if err != nil {
		state := desktopstate.Default()
		state.Preferences.InterfaceStyle = desktopstate.InterfaceStyle(normalizeDesktopStyle(""))
		return nil, state, err
	}
	store := desktopstate.NewStore(filepath.Join(root, "gio", desktopstate.FileName))
	state, loadErr := store.Load()
	if state.Revision == 0 && state.UpdatedAt == 0 {
		state.Preferences.InterfaceStyle = desktopstate.InterfaceStyle(normalizeDesktopStyle(""))
	}
	return store, state, loadErr
}

func (a *App) persistExperienceMode(mode string) {
	mode = normalizeExperienceMode(mode)
	if a.experienceMode == mode {
		return
	}
	a.experienceMode = mode
	a.desktopState.Preferences.ExperienceMode = desktopstate.ExperienceMode(mode)
	if err := a.saveGioDesktopState(); err != nil {
		a.appendLog("保存桌面体验模式失败: " + err.Error())
	}
	a.invalidateNow()
}

func (a *App) persistDesktopStyle(style string) {
	style = normalizeDesktopStyle(style)
	if a.desktopStyle == style {
		return
	}
	a.desktopStyle = style
	a.desktopState.Preferences.InterfaceStyle = desktopstate.InterfaceStyle(style)
	a.installDesktopStyle(style)
	if err := a.saveGioDesktopState(); err != nil {
		a.appendLog("保存桌面界面风格失败: " + err.Error())
	}
	a.invalidateNow()
}

func (a *App) saveGioDesktopState() error {
	if a == nil || a.desktopStore == nil {
		return nil
	}
	a.saveActiveWorkspaceSnapshot()
	state := a.desktopState
	state.Preferences.InterfaceStyle = desktopstate.InterfaceStyle(normalizeDesktopStyle(a.desktopStyle))
	state.Preferences.ExperienceMode = desktopstate.ExperienceMode(normalizeExperienceMode(a.experienceMode))
	state.Workspaces = make([]desktopstate.Workspace, 0, len(a.workspaces))
	for _, workspace := range a.workspaces {
		state.Workspaces = append(state.Workspaces, a.desktopWorkspaceDocument(workspace))
	}
	if a.desktopWindows != nil {
		requests := a.desktopWindows.Requests()
		state.Windows = make([]desktopstate.Window, 0, len(requests))
		for _, request := range requests {
			state.Windows = append(state.Windows, a.desktopWindowState(request))
		}
	}
	next, err := a.desktopStore.Save(state)
	if err != nil {
		return err
	}
	a.desktopState = next
	return nil
}

func desktopWindowState(request windowing.Request) desktopstate.Window {
	return desktopstate.Window{
		ID:          string(request.Role) + ":" + strings.TrimSpace(request.WorkspaceID),
		Role:        desktopWindowRole(request.Role),
		WorkspaceID: strings.TrimSpace(request.WorkspaceID),
		WidthDp:     int(request.Size.Width),
		HeightDp:    int(request.Size.Height),
		Mode:        desktopstate.WindowModeWindowed,
		Visible:     true,
	}
}

func (a *App) desktopWindowState(request windowing.Request) desktopstate.Window {
	state := desktopWindowState(request)
	if a == nil {
		return state
	}
	a.desktopWindowSizeMu.RLock()
	size, ok := a.desktopWindowSizes[state.ID]
	a.desktopWindowSizeMu.RUnlock()
	if ok && size.X > 0 && size.Y > 0 {
		state.WidthDp = size.X
		state.HeightDp = size.Y
	}
	return state
}

func (a *App) recordDesktopWindowSize(request windowing.Request, size image.Point) {
	if a == nil || size.X <= 0 || size.Y <= 0 {
		return
	}
	key := desktopWindowState(request).ID
	if strings.TrimSpace(key) == ":" {
		return
	}
	a.desktopWindowSizeMu.Lock()
	if a.desktopWindowSizes == nil {
		a.desktopWindowSizes = map[string]image.Point{}
	}
	a.desktopWindowSizes[key] = size
	a.desktopWindowSizeMu.Unlock()
}

func desktopWindowRole(role windowing.Role) desktopstate.WindowRole {
	switch role {
	case windowing.RoleCanvas:
		return desktopstate.WindowRoleCanvas
	case windowing.RoleConsole:
		return desktopstate.WindowRoleConsole
	case windowing.RoleProgress:
		return desktopstate.WindowRoleProgress
	case windowing.RoleWorkspace:
		return desktopstate.WindowRoleWorkspace
	default:
		return desktopstate.WindowRoleMain
	}
}

func windowingRole(role desktopstate.WindowRole) (windowing.Role, bool) {
	switch role {
	case desktopstate.WindowRoleCanvas, desktopstate.WindowRoleWorkflow:
		return windowing.RoleCanvas, true
	case desktopstate.WindowRoleConsole:
		return windowing.RoleConsole, true
	case desktopstate.WindowRoleProgress:
		return windowing.RoleProgress, true
	case desktopstate.WindowRoleWorkspace:
		return windowing.RoleWorkspace, true
	default:
		return "", false
	}
}

// RestoreDesktopWindows reopens the previous detached roles after the manager
// is attached. It is safe to call before the main Gio event loop starts.
func (a *App) RestoreDesktopWindows() {
	if a == nil || a.desktopWindows == nil || a.experienceMode != experienceModeWorkflow {
		return
	}
	if !a.desktopState.Preferences.ReopenDetachedWindows {
		return
	}
	restored := 0
	for _, saved := range a.desktopState.Windows {
		if !saved.Visible {
			continue
		}
		role, ok := windowingRole(saved.Role)
		if !ok {
			continue
		}
		workspaceID := strings.TrimSpace(saved.WorkspaceID)
		if workspaceID == "" || !a.hasWorkspace(workspaceID) {
			workspaceID = a.activeWorkspaceID
		}
		request := desktopWindowRequest(role, workspaceID, a.workspaceDisplayNameByID(workspaceID))
		if saved.WidthDp > 0 && saved.HeightDp > 0 {
			request.Size = windowing.DpSize{Width: unit.Dp(saved.WidthDp), Height: unit.Dp(saved.HeightDp)}
		}
		if created, err := a.desktopWindows.Open(request); err == nil && created {
			restored++
		}
	}
	if restored == 0 {
		a.applyDefaultWindowLayout()
	}
}

func (a *App) hasWorkspace(workspaceID string) bool {
	for _, workspace := range a.workspaces {
		if workspace.ID == workspaceID {
			return true
		}
	}
	return false
}

func (a *App) desktopWorkspaceDocument(workspace workspaceState) desktopstate.Workspace {
	graph := a.workflowGraph(workspace.ID)
	resultID := strings.TrimSpace(workspace.ResultItem.ID)
	if resultID == "" {
		resultID = strings.TrimSpace(workspace.SelectedHistoryID)
	}
	return desktopstate.Workspace{
		ID:   workspace.ID,
		Name: workspace.Name,
		Draft: desktopstate.WorkspaceDraft{
			Prompt:                   workspace.Prompt,
			NegativePrompt:           workspace.NegativePrompt,
			Mode:                     workspace.Mode,
			Size:                     workspace.Size,
			Quality:                  workspace.Quality,
			OutputFormat:             workspace.OutputFormat,
			Background:               workspace.Background,
			OutputCompression:        workspace.OutputCompression,
			InputFidelity:            workspace.InputFidelity,
			ImageStyle:               workspace.ImageStyle,
			Moderation:               workspace.Moderation,
			UserIdentifier:           workspace.UserIdentifier,
			PartialImages:            workspace.PartialImages,
			StyleTag:                 workspace.StyleTag,
			SeedText:                 workspace.SeedText,
			SourcePaths:              durableDesktopReferences(kernel.ParseSourcePaths(workspace.SourcePathsText)),
			SelectedPresetID:         workspace.SelectedPresetID,
			BatchCount:               workspace.BatchCount,
			LoopEnabled:              workspace.LoopEnabled,
			LoopTotalCount:           workspace.LoopTotalCount,
			LoopConcurrency:          workspace.LoopConcurrency,
			LoopAutoSave:             workspace.LoopAutoSave,
			LoopAutoSaveDir:          workspace.LoopAutoSaveDir,
			LoopLivePreview:          workspace.LoopLivePreview,
			BatchMode:                workspace.BatchMode,
			BatchInputDir:            workspace.BatchInputDir,
			BatchOutputDir:           workspace.BatchOutputDir,
			BatchOutputMode:          workspace.BatchOutputMode,
			BatchOutputPrefix:        workspace.BatchOutputPrefix,
			BatchConcurrency:         workspace.BatchConcurrency,
			BatchRetryOnFail:         workspace.BatchRetryOnFail,
			BatchAutoAspect:          workspace.BatchAutoAspect,
			EditAutoAspectResolution: workspace.EditAutoAspectResolution,
		},
		Result: desktopstate.WorkspaceResult{
			HistoryID:     resultID,
			SavedPath:     durableDesktopReference(workspace.ResultSavedPath),
			RawPath:       durableDesktopReference(workspace.ResultRawPath),
			RevisedPrompt: workspace.ResultRevisedPrompt,
		},
		Workflow: desktopWorkflowGraph(graph),
	}
}

func desktopWorkflowGraph(graph workflowGraphModel) desktopstate.WorkflowGraph {
	document := desktopstate.WorkflowGraph{
		Explicit: true,
		Nodes:    make([]desktopstate.WorkflowNode, 0, len(graph.Nodes)),
		Edges:    make([]desktopstate.WorkflowEdge, 0, len(graph.Edges)),
	}
	for _, node := range graph.Nodes {
		var properties map[string]string
		if !node.Enabled {
			properties = map[string]string{"enabled": "false"}
		}
		document.Nodes = append(document.Nodes, desktopstate.WorkflowNode{
			ID:         node.ID,
			Kind:       string(node.Kind),
			Title:      node.Title,
			X:          float64(node.Position.X),
			Y:          float64(node.Position.Y),
			Properties: properties,
		})
	}
	for _, edge := range graph.Edges {
		document.Edges = append(document.Edges, desktopstate.WorkflowEdge{
			ID:         edge.ID,
			FromNodeID: edge.FromNode,
			FromPort:   edge.FromPort,
			ToNodeID:   edge.ToNode,
			ToPort:     edge.ToPort,
		})
	}
	return document
}

func workflowGraphFromDesktop(document desktopstate.WorkflowGraph) workflowGraphModel {
	if !document.Explicit && len(document.Nodes) == 0 {
		return defaultWorkflowGraph()
	}
	graph := workflowGraphModel{Revision: 1}
	for _, saved := range document.Nodes {
		node, ok := workflowNodeTemplate(saved.ID)
		if !ok {
			continue
		}
		node.Position = image.Pt(int(saved.X), int(saved.Y))
		node.Enabled = !strings.EqualFold(strings.TrimSpace(saved.Properties["enabled"]), "false")
		graph.Nodes = append(graph.Nodes, node)
	}
	for _, edge := range document.Edges {
		graph.Edges = append(graph.Edges, workflowEdgeModel{
			ID:       edge.ID,
			FromNode: edge.FromNodeID,
			FromPort: edge.FromPort,
			ToNode:   edge.ToNodeID,
			ToPort:   edge.ToPort,
		})
	}
	return normalizeWorkflowGraph(graph)
}

func (a *App) restoreDesktopWorkspaces() bool {
	if a == nil || !a.desktopState.Preferences.RestoreSession || len(a.desktopState.Workspaces) == 0 {
		return false
	}
	workspaces := make([]workspaceState, 0, len(a.desktopState.Workspaces))
	for _, document := range a.desktopState.Workspaces {
		workspace := a.workspaceFromDesktopDocument(document)
		if strings.TrimSpace(workspace.ID) == "" {
			continue
		}
		workspaces = append(workspaces, workspace)
		a.workflowGraphs[workspace.ID] = workflowGraphFromDesktop(document.Workflow)
		a.workflowSelectedNodes[workspace.ID] = "generate"
	}
	if len(workspaces) == 0 {
		return false
	}
	a.workspaces = workspaces
	a.activeWorkspaceID = workspaces[0].ID
	a.applyWorkspace(workspaces[0])
	return true
}

func (a *App) workspaceFromDesktopDocument(document desktopstate.Workspace) workspaceState {
	draft := document.Draft
	defaults := kernel.DefaultConfig()
	workspace := workspaceState{
		ID:                       strings.TrimSpace(document.ID),
		Name:                     strings.TrimSpace(document.Name),
		Prompt:                   draft.Prompt,
		NegativePrompt:           draft.NegativePrompt,
		Mode:                     chooseNonEmpty(draft.Mode, string(defaults.Mode)),
		Size:                     chooseNonEmpty(draft.Size, defaults.Size),
		Quality:                  chooseNonEmpty(draft.Quality, defaults.Quality),
		OutputFormat:             chooseNonEmpty(draft.OutputFormat, defaults.OutputFormat),
		Background:               chooseNonEmpty(draft.Background, defaults.Background),
		OutputCompression:        chooseNonEmpty(draft.OutputCompression, fmt.Sprintf("%d", defaults.OutputCompression)),
		InputFidelity:            chooseNonEmpty(draft.InputFidelity, defaults.InputFidelity),
		ImageStyle:               chooseNonEmpty(draft.ImageStyle, defaults.ImageStyle),
		Moderation:               chooseNonEmpty(draft.Moderation, defaults.Moderation),
		UserIdentifier:           draft.UserIdentifier,
		PartialImages:            chooseNonEmpty(draft.PartialImages, fmt.Sprintf("%d", defaults.PartialImages)),
		StyleTag:                 draft.StyleTag,
		SeedText:                 draft.SeedText,
		SourcePathsText:          strings.Join(durableDesktopReferences(draft.SourcePaths), "\n"),
		SelectedPresetID:         draft.SelectedPresetID,
		BatchCount:               normalizeBatchCount(draft.BatchCount),
		LoopEnabled:              draft.LoopEnabled,
		LoopTotalCount:           normalizeLoopGenerationCount(draft.LoopTotalCount),
		LoopConcurrency:          normalizeLoopGenerationConcurrency(draft.LoopConcurrency),
		LoopAutoSave:             draft.LoopAutoSave,
		LoopAutoSaveDir:          draft.LoopAutoSaveDir,
		LoopLivePreview:          draft.LoopLivePreview,
		BatchMode:                draft.BatchMode,
		BatchInputDir:            draft.BatchInputDir,
		BatchOutputDir:           draft.BatchOutputDir,
		BatchOutputMode:          normalizeBatchOutputMode(draft.BatchOutputMode),
		BatchOutputPrefix:        normalizeBatchOutputPrefix(draft.BatchOutputPrefix),
		BatchConcurrency:         normalizeBatchProcessConcurrency(draft.BatchConcurrency),
		BatchRetryOnFail:         draft.BatchRetryOnFail,
		BatchAutoAspect:          draft.BatchAutoAspect,
		EditAutoAspectResolution: draft.EditAutoAspectResolution,
		ResultSavedPath:          durableDesktopReference(document.Result.SavedPath),
		ResultRawPath:            durableDesktopReference(document.Result.RawPath),
		ResultRevisedPrompt:      document.Result.RevisedPrompt,
	}
	if item, ok := historyItemByID(a.history, document.Result.HistoryID); ok {
		item.SavedPath = durableDesktopReference(item.SavedPath)
		item.RawPath = durableDesktopReference(item.RawPath)
		workspace.ResultItem = item
		workspace.ResultHasItem = true
		workspace.SelectedHistoryID = item.ID
		if workspace.ResultSavedPath == "" {
			workspace.ResultSavedPath = item.SavedPath
		}
		if workspace.ResultRawPath == "" {
			workspace.ResultRawPath = item.RawPath
		}
		if workspace.ResultRevisedPrompt == "" {
			workspace.ResultRevisedPrompt = item.RevisedPrompt
		}
	} else if workspace.ResultSavedPath != "" {
		workspace.ResultItem = sharedCompat.HistoryItem{
			ID:            document.Result.HistoryID,
			SavedPath:     workspace.ResultSavedPath,
			RawPath:       workspace.ResultRawPath,
			RevisedPrompt: workspace.ResultRevisedPrompt,
		}
		workspace.ResultHasItem = true
	}
	return workspace
}

func chooseNonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// Gio's memory:// references point into process-local stores. Persisting them
// would turn a restored clipboard image or raw response into a dead file path.
func durableDesktopReference(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(value), "memory://") {
		return ""
	}
	return value
}

func durableDesktopReferences(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	next := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = durableDesktopReference(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		next = append(next, value)
	}
	return next
}
