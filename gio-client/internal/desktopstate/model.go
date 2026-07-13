package desktopstate

import (
	"fmt"
	"math"
	"strings"
)

const (
	SchemaVersion = 1
	FileName      = "desktop-state.json"
)

type InterfaceStyle string

const (
	InterfaceStyleMacOS   InterfaceStyle = "macos"
	InterfaceStyleWindows InterfaceStyle = "windows"
)

type ExperienceMode string

const (
	ExperienceModeSimple   ExperienceMode = "simple"
	ExperienceModeWorkflow ExperienceMode = "workflow"
)

type WindowLayout string

const (
	WindowLayoutSingle WindowLayout = "single"
	WindowLayoutDual   WindowLayout = "dual"
	WindowLayoutMulti  WindowLayout = "multi"
)

type WindowRole string

const (
	WindowRoleMain      WindowRole = "main"
	WindowRoleWorkflow  WindowRole = "workflow"
	WindowRoleCanvas    WindowRole = "canvas"
	WindowRoleConsole   WindowRole = "console"
	WindowRoleProgress  WindowRole = "progress"
	WindowRoleWorkspace WindowRole = "workspace"
)

type WindowMode string

const (
	WindowModeWindowed   WindowMode = "windowed"
	WindowModeMaximized  WindowMode = "maximized"
	WindowModeFullscreen WindowMode = "fullscreen"
	WindowModeMinimized  WindowMode = "minimized"
)

type State struct {
	SchemaVersion int         `json:"schemaVersion"`
	Revision      uint64      `json:"revision"`
	UpdatedAt     int64       `json:"updatedAt"`
	Preferences   Preferences `json:"preferences"`
	Windows       []Window    `json:"windows"`
	Workspaces    []Workspace `json:"workspaces"`
}

type Preferences struct {
	InterfaceStyle        InterfaceStyle `json:"interfaceStyle"`
	ExperienceMode        ExperienceMode `json:"experienceMode"`
	DefaultWindowLayout   WindowLayout   `json:"defaultWindowLayout"`
	AutoShowProgress      bool           `json:"autoShowProgress"`
	ReopenDetachedWindows bool           `json:"reopenDetachedWindows"`
	RestoreSession        bool           `json:"restoreSession"`
}

type Window struct {
	ID          string     `json:"id"`
	Role        WindowRole `json:"role"`
	WorkspaceID string     `json:"workspaceId,omitempty"`
	WidthDp     int        `json:"widthDp"`
	HeightDp    int        `json:"heightDp"`
	Mode        WindowMode `json:"mode"`
	Visible     bool       `json:"visible"`
}

type Workspace struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Draft    WorkspaceDraft  `json:"draft"`
	Result   WorkspaceResult `json:"result"`
	Workflow WorkflowGraph   `json:"workflow"`
}

// WorkspaceDraft contains window-independent creation parameters. Secrets and
// upstream credentials remain in the shared compatibility state/keyring.
type WorkspaceDraft struct {
	Prompt                   string   `json:"prompt,omitempty"`
	NegativePrompt           string   `json:"negativePrompt,omitempty"`
	Mode                     string   `json:"mode,omitempty"`
	Size                     string   `json:"size,omitempty"`
	Quality                  string   `json:"quality,omitempty"`
	OutputFormat             string   `json:"outputFormat,omitempty"`
	Background               string   `json:"background,omitempty"`
	OutputCompression        string   `json:"outputCompression,omitempty"`
	InputFidelity            string   `json:"inputFidelity,omitempty"`
	ImageStyle               string   `json:"imageStyle,omitempty"`
	Moderation               string   `json:"moderation,omitempty"`
	UserIdentifier           string   `json:"userIdentifier,omitempty"`
	PartialImages            string   `json:"partialImages,omitempty"`
	StyleTag                 string   `json:"styleTag,omitempty"`
	SeedText                 string   `json:"seedText,omitempty"`
	SourcePaths              []string `json:"sourcePaths,omitempty"`
	SelectedPresetID         string   `json:"selectedPresetId,omitempty"`
	BatchCount               int      `json:"batchCount,omitempty"`
	LoopEnabled              bool     `json:"loopEnabled,omitempty"`
	LoopTotalCount           int      `json:"loopTotalCount,omitempty"`
	LoopConcurrency          int      `json:"loopConcurrency,omitempty"`
	LoopAutoSave             bool     `json:"loopAutoSave,omitempty"`
	LoopAutoSaveDir          string   `json:"loopAutoSaveDir,omitempty"`
	LoopLivePreview          bool     `json:"loopLivePreview,omitempty"`
	BatchMode                bool     `json:"batchMode,omitempty"`
	BatchInputDir            string   `json:"batchInputDir,omitempty"`
	BatchOutputDir           string   `json:"batchOutputDir,omitempty"`
	BatchOutputMode          string   `json:"batchOutputMode,omitempty"`
	BatchOutputPrefix        string   `json:"batchOutputPrefix,omitempty"`
	BatchConcurrency         int      `json:"batchConcurrency,omitempty"`
	BatchRetryOnFail         bool     `json:"batchRetryOnFail,omitempty"`
	BatchAutoAspect          string   `json:"batchAutoAspect,omitempty"`
	EditAutoAspectResolution string   `json:"editAutoAspectResolution,omitempty"`
}

// WorkspaceResult stores stable references only; image bytes stay in history
// or on disk and are never duplicated into desktop-state.json.
type WorkspaceResult struct {
	HistoryID     string `json:"historyId,omitempty"`
	SavedPath     string `json:"savedPath,omitempty"`
	RawPath       string `json:"rawPath,omitempty"`
	RevisedPrompt string `json:"revisedPrompt,omitempty"`
}

type WorkflowGraph struct {
	Explicit bool           `json:"explicit,omitempty"`
	Nodes    []WorkflowNode `json:"nodes"`
	Edges    []WorkflowEdge `json:"edges"`
	Viewport Viewport       `json:"viewport"`
}

type WorkflowNode struct {
	ID         string            `json:"id"`
	Kind       string            `json:"kind"`
	Title      string            `json:"title,omitempty"`
	X          float64           `json:"x"`
	Y          float64           `json:"y"`
	WidthDp    int               `json:"widthDp,omitempty"`
	HeightDp   int               `json:"heightDp,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

type WorkflowEdge struct {
	ID         string `json:"id"`
	FromNodeID string `json:"fromNodeId"`
	FromPort   string `json:"fromPort,omitempty"`
	ToNodeID   string `json:"toNodeId"`
	ToPort     string `json:"toPort,omitempty"`
}

type Viewport struct {
	OffsetX float64 `json:"offsetX"`
	OffsetY float64 `json:"offsetY"`
	Zoom    float64 `json:"zoom"`
}

func Default() State {
	return State{
		SchemaVersion: SchemaVersion,
		Preferences: Preferences{
			InterfaceStyle:        InterfaceStyleWindows,
			ExperienceMode:        ExperienceModeSimple,
			DefaultWindowLayout:   WindowLayoutSingle,
			AutoShowProgress:      true,
			ReopenDetachedWindows: true,
			RestoreSession:        true,
		},
		Windows:    []Window{},
		Workspaces: []Workspace{},
	}
}

func Normalize(state State) State {
	state.SchemaVersion = SchemaVersion
	state.Preferences = normalizePreferences(state.Preferences)
	state.Windows = normalizeWindows(state.Windows)
	state.Workspaces = normalizeWorkspaces(state.Workspaces)
	return state
}

func normalizePreferences(preferences Preferences) Preferences {
	if !validInterfaceStyle(preferences.InterfaceStyle) {
		preferences.InterfaceStyle = InterfaceStyleWindows
	}
	if !validExperienceMode(preferences.ExperienceMode) {
		preferences.ExperienceMode = ExperienceModeSimple
	}
	if !validWindowLayout(preferences.DefaultWindowLayout) {
		preferences.DefaultWindowLayout = WindowLayoutSingle
	}
	return preferences
}

func normalizeWindows(windows []Window) []Window {
	if windows == nil {
		return []Window{}
	}
	next := make([]Window, 0, len(windows))
	usedIDs := make(map[string]struct{}, len(windows))
	for index, window := range windows {
		window.ID = uniqueID(strings.TrimSpace(window.ID), "window", index+1, usedIDs)
		window.WorkspaceID = strings.TrimSpace(window.WorkspaceID)
		if !validWindowRole(window.Role) {
			window.Role = WindowRoleMain
		}
		if !validWindowMode(window.Mode) {
			window.Mode = WindowModeWindowed
		}
		defaultWidth, defaultHeight := defaultWindowSize(window.Role)
		if window.WidthDp <= 0 {
			window.WidthDp = defaultWidth
		}
		if window.HeightDp <= 0 {
			window.HeightDp = defaultHeight
		}
		next = append(next, window)
	}
	return next
}

func normalizeWorkspaces(workspaces []Workspace) []Workspace {
	if workspaces == nil {
		return []Workspace{}
	}
	next := make([]Workspace, 0, len(workspaces))
	usedIDs := make(map[string]struct{}, len(workspaces))
	for index, workspace := range workspaces {
		workspace.ID = uniqueID(strings.TrimSpace(workspace.ID), "workspace", index+1, usedIDs)
		workspace.Name = strings.TrimSpace(workspace.Name)
		if workspace.Name == "" {
			workspace.Name = fmt.Sprintf("Workspace %d", index+1)
		}
		workspace.Draft = normalizeWorkspaceDraft(workspace.Draft)
		workspace.Result.HistoryID = strings.TrimSpace(workspace.Result.HistoryID)
		workspace.Result.SavedPath = strings.TrimSpace(workspace.Result.SavedPath)
		workspace.Result.RawPath = strings.TrimSpace(workspace.Result.RawPath)
		workspace.Result.RevisedPrompt = strings.TrimSpace(workspace.Result.RevisedPrompt)
		workspace.Workflow = normalizeWorkflowGraph(workspace.Workflow)
		next = append(next, workspace)
	}
	return next
}

func normalizeWorkspaceDraft(draft WorkspaceDraft) WorkspaceDraft {
	draft.Mode = strings.TrimSpace(draft.Mode)
	draft.Size = strings.TrimSpace(draft.Size)
	draft.Quality = strings.TrimSpace(draft.Quality)
	draft.OutputFormat = strings.TrimSpace(draft.OutputFormat)
	draft.Background = strings.TrimSpace(draft.Background)
	draft.OutputCompression = strings.TrimSpace(draft.OutputCompression)
	draft.InputFidelity = strings.TrimSpace(draft.InputFidelity)
	draft.ImageStyle = strings.TrimSpace(draft.ImageStyle)
	draft.Moderation = strings.TrimSpace(draft.Moderation)
	draft.UserIdentifier = strings.TrimSpace(draft.UserIdentifier)
	draft.PartialImages = strings.TrimSpace(draft.PartialImages)
	draft.StyleTag = strings.TrimSpace(draft.StyleTag)
	draft.SeedText = strings.TrimSpace(draft.SeedText)
	draft.SelectedPresetID = strings.TrimSpace(draft.SelectedPresetID)
	draft.LoopAutoSaveDir = strings.TrimSpace(draft.LoopAutoSaveDir)
	draft.BatchInputDir = strings.TrimSpace(draft.BatchInputDir)
	draft.BatchOutputDir = strings.TrimSpace(draft.BatchOutputDir)
	draft.BatchOutputMode = strings.TrimSpace(draft.BatchOutputMode)
	draft.BatchOutputPrefix = strings.TrimSpace(draft.BatchOutputPrefix)
	draft.BatchAutoAspect = strings.TrimSpace(draft.BatchAutoAspect)
	draft.EditAutoAspectResolution = strings.TrimSpace(draft.EditAutoAspectResolution)
	if draft.BatchCount < 1 {
		draft.BatchCount = 1
	}
	if draft.LoopTotalCount < 1 {
		draft.LoopTotalCount = 10
	}
	if draft.LoopConcurrency < 1 {
		draft.LoopConcurrency = 2
	}
	if draft.BatchConcurrency < 1 {
		draft.BatchConcurrency = 2
	}
	sources := make([]string, 0, len(draft.SourcePaths))
	seen := make(map[string]struct{}, len(draft.SourcePaths))
	for _, source := range draft.SourcePaths {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if _, exists := seen[source]; exists {
			continue
		}
		seen[source] = struct{}{}
		sources = append(sources, source)
	}
	draft.SourcePaths = sources
	return draft
}

func normalizeWorkflowGraph(graph WorkflowGraph) WorkflowGraph {
	if graph.Nodes == nil {
		graph.Nodes = []WorkflowNode{}
	} else {
		graph.Nodes = append([]WorkflowNode(nil), graph.Nodes...)
	}
	if graph.Edges == nil {
		graph.Edges = []WorkflowEdge{}
	} else {
		graph.Edges = append([]WorkflowEdge(nil), graph.Edges...)
	}
	usedNodeIDs := make(map[string]struct{}, len(graph.Nodes))
	for index := range graph.Nodes {
		node := &graph.Nodes[index]
		node.ID = uniqueID(strings.TrimSpace(node.ID), "node", index+1, usedNodeIDs)
		node.Kind = strings.TrimSpace(node.Kind)
		if node.Kind == "" {
			node.Kind = "operation"
		}
		node.Title = strings.TrimSpace(node.Title)
		node.X = finiteOrZero(node.X)
		node.Y = finiteOrZero(node.Y)
		if node.WidthDp < 0 {
			node.WidthDp = 0
		}
		if node.HeightDp < 0 {
			node.HeightDp = 0
		}
		if node.Properties != nil {
			properties := make(map[string]string, len(node.Properties))
			for key, value := range node.Properties {
				key = strings.TrimSpace(key)
				if key != "" {
					properties[key] = value
				}
			}
			node.Properties = properties
		}
	}

	usedEdgeIDs := make(map[string]struct{}, len(graph.Edges))
	edges := make([]WorkflowEdge, 0, len(graph.Edges))
	for index, edge := range graph.Edges {
		edge.FromNodeID = strings.TrimSpace(edge.FromNodeID)
		edge.ToNodeID = strings.TrimSpace(edge.ToNodeID)
		if _, ok := usedNodeIDs[edge.FromNodeID]; !ok {
			continue
		}
		if _, ok := usedNodeIDs[edge.ToNodeID]; !ok {
			continue
		}
		edge.ID = uniqueID(strings.TrimSpace(edge.ID), "edge", index+1, usedEdgeIDs)
		edge.FromPort = strings.TrimSpace(edge.FromPort)
		edge.ToPort = strings.TrimSpace(edge.ToPort)
		edges = append(edges, edge)
	}
	graph.Edges = edges
	graph.Viewport.OffsetX = finiteOrZero(graph.Viewport.OffsetX)
	graph.Viewport.OffsetY = finiteOrZero(graph.Viewport.OffsetY)
	if !isFinite(graph.Viewport.Zoom) || graph.Viewport.Zoom <= 0 {
		graph.Viewport.Zoom = 1
	}
	return graph
}

func uniqueID(value, prefix string, ordinal int, used map[string]struct{}) string {
	if value == "" {
		value = fmt.Sprintf("%s-%d", prefix, ordinal)
	}
	base := value
	for suffix := 2; ; suffix++ {
		if _, exists := used[value]; !exists {
			used[value] = struct{}{}
			return value
		}
		value = fmt.Sprintf("%s-%d", base, suffix)
	}
}

func defaultWindowSize(role WindowRole) (int, int) {
	switch role {
	case WindowRoleProgress:
		return 420, 300
	case WindowRoleConsole:
		return 760, 600
	case WindowRoleCanvas:
		return 1100, 760
	default:
		return 1440, 900
	}
}

func finiteOrZero(value float64) float64 {
	if !isFinite(value) {
		return 0
	}
	return value
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func validInterfaceStyle(value InterfaceStyle) bool {
	return value == InterfaceStyleMacOS || value == InterfaceStyleWindows
}

func validExperienceMode(value ExperienceMode) bool {
	return value == ExperienceModeSimple || value == ExperienceModeWorkflow
}

func validWindowLayout(value WindowLayout) bool {
	return value == WindowLayoutSingle || value == WindowLayoutDual || value == WindowLayoutMulti
}

func validWindowRole(value WindowRole) bool {
	switch value {
	case WindowRoleMain, WindowRoleWorkflow, WindowRoleCanvas, WindowRoleConsole, WindowRoleProgress, WindowRoleWorkspace:
		return true
	default:
		return false
	}
}

func validWindowMode(value WindowMode) bool {
	switch value {
	case WindowModeWindowed, WindowModeMaximized, WindowModeFullscreen, WindowModeMinimized:
		return true
	default:
		return false
	}
}
