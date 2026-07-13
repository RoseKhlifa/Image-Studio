package ui

import (
	"fmt"
	"image"
	"strings"
	"sync"

	"image-studio/gio-client/internal/kernel"
	"image-studio/gio-client/internal/windowing"
)

// desktopSessionActor binds the detached-window command mailbox to the main
// Gio event loop. Wake is safe from worker and detached-window goroutines;
// Handle remains on the main window goroutine so root Gio widgets keep a
// single owner even when an inactive window produces a wakeup event instead of
// a FrameEvent.
type desktopSessionActor struct {
	mu      sync.Mutex
	wake    func()
	stopped bool
}

func newDesktopSessionActor(wake func()) *desktopSessionActor {
	return &desktopSessionActor{wake: wake}
}

func (actor *desktopSessionActor) Wake() bool {
	if actor == nil {
		return false
	}
	actor.mu.Lock()
	defer actor.mu.Unlock()
	if actor.stopped || actor.wake == nil {
		return false
	}
	actor.wake()
	return true
}

func (actor *desktopSessionActor) Stop() {
	if actor == nil {
		return
	}
	actor.mu.Lock()
	actor.stopped = true
	actor.wake = nil
	actor.mu.Unlock()
}

func (a *App) startDesktopSessionActor(wake func()) *desktopSessionActor {
	if a == nil {
		return nil
	}
	a.desktopSessionMu.Lock()
	defer a.desktopSessionMu.Unlock()
	if a.desktopSession != nil {
		return a.desktopSession
	}
	actor := newDesktopSessionActor(wake)
	a.desktopSession = actor
	a.desktopSessionClosed = false
	return actor
}

func (a *App) stopDesktopSessionActor(actor *desktopSessionActor) {
	if a == nil || actor == nil {
		return
	}
	a.desktopSessionMu.Lock()
	if a.desktopSession == actor {
		a.desktopSession = nil
		a.desktopSessionClosed = true
	}
	actor.Stop()
	a.desktopSessionMu.Unlock()
}

// requestDesktopSessionWake requests an event-loop turn without requiring a
// rendered frame. A stopped actor consumes the request so delayed callbacks do
// not touch a window that is already being destroyed.
func (a *App) requestDesktopSessionWake() bool {
	if a == nil {
		return false
	}
	a.desktopSessionMu.Lock()
	defer a.desktopSessionMu.Unlock()
	if a.desktopSessionClosed {
		return true
	}
	if a.desktopSession == nil {
		return false
	}
	return a.desktopSession.Wake()
}

func (a *App) requestWakeup() {
	if a == nil || a.requestDesktopSessionWake() {
		return
	}
	if a.invalidate != nil {
		a.invalidate()
	}
}

func (a *App) desktopSessionAcceptingCommands() bool {
	if a == nil {
		return false
	}
	a.desktopSessionMu.Lock()
	accepting := !a.desktopSessionClosed
	a.desktopSessionMu.Unlock()
	return accepting
}

func (a *App) handleDesktopSessionEvent(actor *desktopSessionActor) bool {
	if a == nil || actor == nil {
		return false
	}
	a.desktopSessionMu.Lock()
	active := a.desktopSession == actor && !a.desktopSessionClosed
	a.desktopSessionMu.Unlock()
	if !active {
		return false
	}
	a.processDesktopCommands()
	a.publishDesktopState(a.readSnapshot())
	return true
}

type desktopWorkspacePublication struct {
	ID              string
	Name            string
	DraftRevision   uint64
	Prompt          string
	NegativePrompt  string
	Mode            string
	Size            string
	Quality         string
	OutputFormat    string
	SourceCount     int
	SelectedNode    string
	Graph           workflowGraphModel
	Runtime         map[string]workflowNodeRuntime
	ResultImage     image.Image
	ResultRevision  int
	ResultSavedPath string
	ResultHistoryID string
	Running         bool
	Queued          bool
	Status          string
	LastError       string
	Completed       int
	Total           int
	CanUndo         bool
	CanRedo         bool
}

type desktopPublication struct {
	Revision       uint64
	DesktopStyle   string
	ColorMode      string
	FontScale      float64
	ExperienceMode string
	ActiveID       string
	Running        bool
	Status         string
	LastError      string
	Completed      int
	Total          int
	Logs           []string
	QueuedRuns     []string
	Workspaces     []desktopWorkspacePublication
}

func (publication desktopPublication) workspace(id string) (desktopWorkspacePublication, bool) {
	id = strings.TrimSpace(id)
	for _, workspace := range publication.Workspaces {
		if workspace.ID == id {
			return workspace, true
		}
	}
	return desktopWorkspacePublication{}, false
}

func cloneDesktopPublication(publication desktopPublication) desktopPublication {
	clone := publication
	clone.Logs = append([]string(nil), publication.Logs...)
	clone.QueuedRuns = append([]string(nil), publication.QueuedRuns...)
	clone.Workspaces = make([]desktopWorkspacePublication, len(publication.Workspaces))
	for index, workspace := range publication.Workspaces {
		workspace.Graph = cloneWorkflowGraph(workspace.Graph)
		workspace.Runtime = cloneWorkflowRuntime(workspace.Runtime)
		clone.Workspaces[index] = workspace
	}
	return clone
}

func cloneWorkflowRuntime(runtime map[string]workflowNodeRuntime) map[string]workflowNodeRuntime {
	if runtime == nil {
		return nil
	}
	clone := make(map[string]workflowNodeRuntime, len(runtime))
	for key, value := range runtime {
		clone[key] = value
	}
	return clone
}

func (a *App) publishDesktopState(snap snapshot) {
	if a == nil {
		return
	}
	a.mu.Lock()
	activeSnapshot := a.buildWorkspaceSnapshot()
	a.mu.Unlock()
	workspaces := make([]workspaceState, len(a.workspaces))
	copy(workspaces, a.workspaces)
	for index := range workspaces {
		if workspaces[index].ID == activeSnapshot.ID {
			workspaces[index] = activeSnapshot
			break
		}
	}
	publication := desktopPublication{
		DesktopStyle:   normalizeDesktopStyle(a.desktopStyle),
		ColorMode:      normalizeDesktopColorMode(a.resolvedThemeMode),
		FontScale:      normalizeFontScale(a.fontScale),
		ExperienceMode: normalizeExperienceMode(a.experienceMode),
		ActiveID:       a.activeWorkspaceID,
		Running:        snap.Running,
		Status:         snap.Status,
		LastError:      snap.LastErrorMessage,
		Completed:      len(snap.BatchResults),
		Total:          max(snap.BatchTotal, 1),
		Logs:           append([]string(nil), snap.Logs...),
		QueuedRuns:     append([]string(nil), a.desktopQueuedWorkspaceRuns...),
		Workspaces:     make([]desktopWorkspacePublication, 0, len(workspaces)),
	}
	if publication.Completed == 0 && snap.Result.HasItem {
		publication.Completed = 1
	}
	for _, workspace := range workspaces {
		workspacePublication := desktopWorkspacePublication{
			ID:              workspace.ID,
			Name:            a.displayedWorkspaceName(workspace),
			Prompt:          workspace.Prompt,
			NegativePrompt:  workspace.NegativePrompt,
			Mode:            workspace.Mode,
			Size:            workspace.Size,
			Quality:         workspace.Quality,
			OutputFormat:    workspace.OutputFormat,
			SourceCount:     len(kernel.ParseSourcePaths(workspace.SourcePathsText)),
			SelectedNode:    a.selectedWorkflowNode(workspace.ID),
			Graph:           a.workflowGraph(workspace.ID),
			ResultSavedPath: workspace.ResultSavedPath,
			ResultHistoryID: workspace.ResultItem.ID,
			Queued:          desktopRunQueued(a.desktopQueuedWorkspaceRuns, workspace.ID),
			Status:          "就绪",
			Total:           1,
			CanUndo:         a.canUndoWorkflowGraph(workspace.ID),
			CanRedo:         a.canRedoWorkflowGraph(workspace.ID),
		}
		if workspace.ID == a.activeWorkspaceID {
			data := a.workflowCanvasData(snap, workspace.ID)
			workspacePublication.Prompt = a.promptInput.Text()
			workspacePublication.NegativePrompt = a.negativePromptInput.Text()
			workspacePublication.Mode = a.mode
			workspacePublication.Size = a.size
			workspacePublication.Quality = a.quality
			workspacePublication.OutputFormat = a.format
			workspacePublication.SourceCount = len(a.parseSourcePathsCached(a.sourcePathsInput.Text()))
			workspacePublication.SelectedNode = data.Selected
			workspacePublication.Graph = data.Graph
			workspacePublication.Runtime = data.Runtime
			workspacePublication.ResultImage = snap.Result.Image
			workspacePublication.ResultRevision = snap.Result.Rev
			workspacePublication.ResultSavedPath = snap.Result.SavedPath
			workspacePublication.ResultHistoryID = snap.Result.Item.ID
			workspacePublication.Running = snap.Running
			workspacePublication.Status = snap.Status
			workspacePublication.LastError = snap.LastErrorMessage
			workspacePublication.Completed = publication.Completed
			workspacePublication.Total = publication.Total
		} else {
			workspacePublication.Runtime = workflowRuntimeForInactiveWorkspace(workspace, workspacePublication.Graph)
			if workspace.ResultHasItem || strings.TrimSpace(workspace.ResultSavedPath) != "" {
				workspacePublication.Status = "已完成"
				workspacePublication.Completed = 1
			}
		}
		workspacePublication.DraftRevision = a.observeDesktopDraft(workspace.ID, desktopDraftUpdateFromPublication(workspacePublication))
		publication.Workspaces = append(publication.Workspaces, workspacePublication)
	}

	a.desktopPublishMu.Lock()
	a.desktopPublishRevision++
	publication.Revision = a.desktopPublishRevision
	a.desktopPublished = publication
	a.desktopPublishMu.Unlock()
	if a.desktopWindows != nil {
		a.desktopWindows.InvalidateAll()
	}
}

func desktopRunQueued(queue []string, workspaceID string) bool {
	for _, queued := range queue {
		if queued == workspaceID {
			return true
		}
	}
	return false
}

func workflowRuntimeForInactiveWorkspace(workspace workspaceState, graph workflowGraphModel) map[string]workflowNodeRuntime {
	promptConnected := workflowEdgeConnected(graph, workflowEdgeModel{FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"})
	sourceConnected := workflowEdgeConnected(graph, workflowEdgeModel{FromNode: "source", FromPort: "image", ToNode: "generate", ToPort: "source"})
	previewConnected := workflowEdgeConnected(graph, workflowEdgeModel{FromNode: "generate", FromPort: "job", ToNode: "preview", ToPort: "job"})
	exportConnected := workflowEdgeConnected(graph, workflowEdgeModel{FromNode: "preview", FromPort: "image", ToNode: "export", ToPort: "image"})
	promptPhase := workflowNodePhaseSuccess
	promptDetail := strings.TrimSpace(workspace.Prompt)
	if promptDetail == "" {
		promptPhase = workflowNodePhaseWarning
		promptDetail = "等待输入提示词"
	} else if !promptConnected {
		promptPhase = workflowNodePhaseWarning
		promptDetail = "提示词端口未连接"
	}
	sourceCount := len(kernel.ParseSourcePaths(workspace.SourcePathsText))
	sourcePhase := workflowNodePhaseIdle
	sourceDetail := "无参考图"
	if sourceCount > 0 {
		sourcePhase = workflowNodePhaseSuccess
		sourceDetail = fmt.Sprintf("已载入 %d 个图像输入", sourceCount)
	}
	requireSource := workspace.Mode == "edit" || workspace.BatchMode
	if !sourceConnected && (sourceCount > 0 || requireSource) {
		sourcePhase = workflowNodePhaseWarning
		sourceDetail = "图像输入端口未连接"
	}
	resultPhase := workflowNodePhaseIdle
	resultDetail := "等待任务输出"
	if workspace.ResultHasItem || strings.TrimSpace(workspace.ResultSavedPath) != "" {
		resultPhase = workflowNodePhaseSuccess
		resultDetail = "已有工作区结果"
	}
	generatePhase := resultPhase
	generateDetail := resultDetail
	if !promptConnected || (requireSource && !sourceConnected) {
		generatePhase = workflowNodePhaseWarning
		generateDetail = "等待必需输入连接"
	}
	previewPhase := resultPhase
	previewDetail := resultDetail
	if !previewConnected {
		previewPhase = workflowNodePhaseWarning
		previewDetail = "任务输入端口未连接"
	}
	exportPhase := resultPhase
	exportDetail := chooseNonEmpty(workspace.ResultSavedPath, "等待导出")
	if !exportConnected {
		exportPhase = workflowNodePhaseWarning
		exportDetail = "图像输入端口未连接"
	}
	runtime := map[string]workflowNodeRuntime{
		"prompt":   {Phase: promptPhase, Detail: promptDetail, Progress: chooseProgress(promptPhase, 0)},
		"source":   {Phase: sourcePhase, Detail: sourceDetail, Progress: chooseProgress(sourcePhase, 0)},
		"generate": {Phase: generatePhase, Detail: generateDetail, Progress: chooseProgress(generatePhase, 0)},
		"preview":  {Phase: previewPhase, Detail: previewDetail, Progress: chooseProgress(previewPhase, 0)},
		"export":   {Phase: exportPhase, Detail: exportDetail, Progress: chooseProgress(exportPhase, 0)},
	}
	applyDisabledWorkflowRuntime(graph, runtime)
	return runtime
}

func (a *App) desktopSnapshot() desktopPublication {
	if a == nil {
		return desktopPublication{}
	}
	a.desktopPublishMu.RLock()
	publication := cloneDesktopPublication(a.desktopPublished)
	a.desktopPublishMu.RUnlock()
	return publication
}

type desktopCommandKind uint8

const (
	desktopCommandActivate desktopCommandKind = iota + 1
	desktopCommandRun
	desktopCommandCancel
	desktopCommandClearLogs
	desktopCommandSelectNode
	desktopCommandBeginNodeMove
	desktopCommandMoveNode
	desktopCommandEndNodeMove
	desktopCommandRewireConnection
	desktopCommandUndoWorkflow
	desktopCommandRedoWorkflow
	desktopCommandDeleteWorkflowNode
	desktopCommandToggleWorkflowNode
	desktopCommandUpdateDraft
	desktopCommandUpdateDraftAndRun
	desktopCommandOpenWindow
	desktopCommandRaiseMain
)

type desktopCommand struct {
	Kind            desktopCommandKind
	WorkspaceID     string
	NodeID          string
	Position        image.Point
	PreviousEdge    workflowEdgeModel
	Edge            workflowEdgeModel
	HasPreviousEdge bool
	HasEdge         bool
	WindowRole      windowing.Role
	Draft           desktopDraftUpdate
	DraftRevision   uint64
}

type desktopDraftUpdate struct {
	Prompt         string
	NegativePrompt string
	Mode           string
	Size           string
	Quality        string
	OutputFormat   string
}

type desktopDraftModel struct {
	Revision uint64
	Draft    desktopDraftUpdate
}

func desktopDraftUpdateFromPublication(workspace desktopWorkspacePublication) desktopDraftUpdate {
	return normalizeDesktopDraftUpdate(desktopDraftUpdate{
		Prompt:         workspace.Prompt,
		NegativePrompt: workspace.NegativePrompt,
		Mode:           workspace.Mode,
		Size:           workspace.Size,
		Quality:        workspace.Quality,
		OutputFormat:   workspace.OutputFormat,
	})
}

func normalizeDesktopDraftUpdate(update desktopDraftUpdate) desktopDraftUpdate {
	update.Mode = normalizeDesktopDraftChoice(update.Mode, modeChoices, "generate")
	update.Size = normalizeDesktopDraftChoice(update.Size, sizeChoices, "auto")
	update.Quality = normalizeDesktopDraftChoice(update.Quality, qualityChoices, "auto")
	update.OutputFormat = normalizeDesktopDraftChoice(update.OutputFormat, formatChoices, "png")
	return update
}

func (a *App) currentDesktopDraft(workspaceID string) (desktopDraftUpdate, bool) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return desktopDraftUpdate{}, false
	}
	if workspaceID == a.activeWorkspaceID && a.hasWorkspace(workspaceID) {
		return normalizeDesktopDraftUpdate(desktopDraftUpdate{
			Prompt:         a.promptInput.Text(),
			NegativePrompt: a.negativePromptInput.Text(),
			Mode:           a.mode,
			Size:           a.size,
			Quality:        a.quality,
			OutputFormat:   a.format,
		}), true
	}
	for _, workspace := range a.workspaces {
		if workspace.ID == workspaceID {
			return normalizeDesktopDraftUpdate(desktopDraftUpdate{
				Prompt:         workspace.Prompt,
				NegativePrompt: workspace.NegativePrompt,
				Mode:           workspace.Mode,
				Size:           workspace.Size,
				Quality:        workspace.Quality,
				OutputFormat:   workspace.OutputFormat,
			}), true
		}
	}
	return desktopDraftUpdate{}, false
}

func (a *App) observeDesktopDraft(workspaceID string, draft desktopDraftUpdate) uint64 {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return 0
	}
	if a.desktopDraftModels == nil {
		a.desktopDraftModels = map[string]desktopDraftModel{}
	}
	draft = normalizeDesktopDraftUpdate(draft)
	model, exists := a.desktopDraftModels[workspaceID]
	if !exists {
		model = desktopDraftModel{Revision: 1, Draft: draft}
	} else if model.Draft != draft {
		model.Revision++
		model.Draft = draft
	}
	a.desktopDraftModels[workspaceID] = model
	return model.Revision
}

func (a *App) enqueueDesktopCommand(command desktopCommand) bool {
	if a == nil || a.desktopCommands == nil || !a.desktopSessionAcceptingCommands() {
		return false
	}
	if command.Kind == desktopCommandMoveNode {
		key := strings.TrimSpace(command.WorkspaceID) + ":" + strings.TrimSpace(command.NodeID)
		a.desktopPendingMoveMu.Lock()
		if a.desktopPendingMoves == nil {
			a.desktopPendingMoves = map[string]desktopCommand{}
		}
		a.desktopPendingMoves[key] = command
		a.desktopPendingMoveMu.Unlock()
		a.requestWakeup()
		return true
	}
	select {
	case a.desktopCommands <- command:
		a.requestWakeup()
		return true
	default:
		return false
	}
}

func (a *App) processDesktopCommands() {
	if a == nil || a.desktopCommands == nil {
		return
	}
	for processed := 0; processed < 256; processed++ {
		select {
		case command := <-a.desktopCommands:
			a.applyDesktopCommand(command)
		default:
			a.applyPendingDesktopMoves()
			a.maybeRunQueuedWorkspace()
			return
		}
	}
	a.applyPendingDesktopMoves()
	a.maybeRunQueuedWorkspace()
}

func (a *App) applyPendingDesktopMoves() {
	a.desktopPendingMoveMu.Lock()
	pending := a.desktopPendingMoves
	a.desktopPendingMoves = map[string]desktopCommand{}
	a.desktopPendingMoveMu.Unlock()
	for _, command := range pending {
		a.applyDesktopCommand(command)
	}
}

func (a *App) applyDesktopCommand(command desktopCommand) {
	switch command.Kind {
	case desktopCommandActivate:
		a.switchWorkspace(command.WorkspaceID)
	case desktopCommandRun:
		a.runDesktopWorkspace(command.WorkspaceID)
	case desktopCommandCancel:
		a.cancelRun()
	case desktopCommandClearLogs:
		a.clearLogs()
	case desktopCommandSelectNode:
		a.selectWorkflowNode(command.WorkspaceID, command.NodeID)
	case desktopCommandBeginNodeMove:
		a.beginWorkflowNodeMove(command.WorkspaceID, command.NodeID)
	case desktopCommandMoveNode:
		a.setWorkflowNodePosition(command.WorkspaceID, command.NodeID, command.Position)
	case desktopCommandEndNodeMove:
		a.applyPendingDesktopMoves()
		a.endWorkflowNodeMove(command.WorkspaceID, command.NodeID)
	case desktopCommandRewireConnection:
		a.applyPendingDesktopMoves()
		var previous *workflowEdgeModel
		var replacement *workflowEdgeModel
		if command.HasPreviousEdge {
			previous = &command.PreviousEdge
		}
		if command.HasEdge {
			replacement = &command.Edge
		}
		if err := a.rewireWorkflowConnection(command.WorkspaceID, previous, replacement); err != nil {
			a.appendLog("连接节点失败: " + err.Error())
		}
	case desktopCommandUndoWorkflow:
		a.applyPendingDesktopMoves()
		a.undoWorkflowGraph(command.WorkspaceID)
	case desktopCommandRedoWorkflow:
		a.applyPendingDesktopMoves()
		a.redoWorkflowGraph(command.WorkspaceID)
	case desktopCommandDeleteWorkflowNode:
		a.applyPendingDesktopMoves()
		a.deleteWorkflowNode(command.WorkspaceID, command.NodeID)
	case desktopCommandToggleWorkflowNode:
		a.applyPendingDesktopMoves()
		a.toggleWorkflowNodeEnabled(command.WorkspaceID, command.NodeID)
	case desktopCommandUpdateDraft:
		a.applyDesktopDraftUpdateAtRevision(command.WorkspaceID, command.Draft, command.DraftRevision)
	case desktopCommandUpdateDraftAndRun:
		if a.applyDesktopDraftUpdateAtRevision(command.WorkspaceID, command.Draft, command.DraftRevision) {
			a.runDesktopWorkspace(command.WorkspaceID)
		}
	case desktopCommandOpenWindow:
		a.openDesktopWindow(command.WindowRole, command.WorkspaceID)
	case desktopCommandRaiseMain:
		a.performMainWindowRaise()
	}
}

func (a *App) applyDesktopDraftUpdate(workspaceID string, update desktopDraftUpdate) {
	a.applyDesktopDraftUpdateAtRevision(workspaceID, update, 0)
}

func (a *App) applyDesktopDraftUpdateAtRevision(workspaceID string, update desktopDraftUpdate, baseRevision uint64) bool {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" || !a.hasWorkspace(workspaceID) {
		return false
	}
	current, ok := a.currentDesktopDraft(workspaceID)
	if !ok {
		return false
	}
	currentRevision := a.observeDesktopDraft(workspaceID, current)
	if baseRevision != 0 && baseRevision != currentRevision {
		a.appendLog(fmt.Sprintf("工作区参数已在其他窗口更新，已拒绝旧草稿: %s", a.workspaceDisplayNameByID(workspaceID)))
		return false
	}
	update = normalizeDesktopDraftUpdate(update)
	if workspaceID == a.activeWorkspaceID {
		a.promptInput.SetText(update.Prompt)
		a.negativePromptInput.SetText(update.NegativePrompt)
		a.mode = update.Mode
		a.size = update.Size
		a.quality = update.Quality
		a.format = update.OutputFormat
		a.saveActiveWorkspaceSnapshot()
	} else {
		for index := range a.workspaces {
			if a.workspaces[index].ID != workspaceID {
				continue
			}
			a.workspaces[index].Prompt = update.Prompt
			a.workspaces[index].NegativePrompt = update.NegativePrompt
			a.workspaces[index].Mode = update.Mode
			a.workspaces[index].Size = update.Size
			a.workspaces[index].Quality = update.Quality
			a.workspaces[index].OutputFormat = update.OutputFormat
			break
		}
	}
	a.observeDesktopDraft(workspaceID, update)
	if err := a.saveGioDesktopState(); err != nil {
		a.appendLog("保存独立工作区参数失败: " + err.Error())
	}
	a.invalidateNow()
	return true
}

func (a *App) runDesktopWorkspace(workspaceID string) {
	if a.isRunning() {
		a.queueDesktopWorkspaceRun(workspaceID)
		return
	}
	if workspaceID != "" && workspaceID != a.activeWorkspaceID {
		a.switchWorkspace(workspaceID)
	}
	if workspaceID == "" || workspaceID == a.activeWorkspaceID {
		a.startRun()
	}
}

func normalizeDesktopDraftChoice(value string, choices []choice, fallback string) string {
	value = strings.TrimSpace(value)
	for _, option := range choices {
		if option.Value == value {
			return value
		}
	}
	return fallback
}

func (a *App) queueDesktopWorkspaceRun(workspaceID string) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" || !a.hasWorkspace(workspaceID) {
		return
	}
	for _, queued := range a.desktopQueuedWorkspaceRuns {
		if queued == workspaceID {
			return
		}
	}
	a.desktopQueuedWorkspaceRuns = append(a.desktopQueuedWorkspaceRuns, workspaceID)
	a.appendLog("已加入工作区队列: " + a.workspaceDisplayNameByID(workspaceID))
}

func (a *App) maybeRunQueuedWorkspace() {
	if a.isRunning() || len(a.desktopQueuedWorkspaceRuns) == 0 {
		return
	}
	workspaceID := a.desktopQueuedWorkspaceRuns[0]
	a.desktopQueuedWorkspaceRuns = append([]string(nil), a.desktopQueuedWorkspaceRuns[1:]...)
	if workspaceID != a.activeWorkspaceID {
		a.switchWorkspace(workspaceID)
	}
	if workspaceID == a.activeWorkspaceID {
		a.startRun()
	}
}
