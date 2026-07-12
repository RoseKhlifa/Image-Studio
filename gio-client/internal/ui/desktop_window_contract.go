package ui

import (
	"fmt"
	"runtime"
	"strings"

	"image-studio/gio-client/internal/windowing"

	"gioui.org/unit"
)

// DesktopWindowController is implemented by windowing.Manager. App depends on
// this narrow interface so its state and layouts remain testable without
// creating native windows.
type DesktopWindowController interface {
	Open(windowing.Request) (bool, error)
	CloseAll() int
	InvalidateAll() int
	Count() int
	Requests() []windowing.Request
}

// SetDesktopWindowController attaches the process-level window manager.
func (a *App) SetDesktopWindowController(controller DesktopWindowController) {
	a.desktopWindows = controller
}

// HandleDesktopWindowError records a detached-window failure without
// terminating the process or other windows.
func (a *App) HandleDesktopWindowError(request windowing.Request, err error) {
	if err == nil {
		return
	}
	a.appendLog(fmt.Sprintf("%s窗口异常: %v", desktopWindowRoleLabel(request.Role), err))
}

func (a *App) openDesktopWindow(role windowing.Role, workspaceID string) {
	if a.desktopWindows == nil {
		a.appendLog("桌面窗口协调器尚未就绪")
		return
	}
	workspaceID = strings.TrimSpace(workspaceID)
	workspaceName := a.workspaceDisplayNameByID(workspaceID)
	request := desktopWindowRequest(role, workspaceID, workspaceName)
	created, err := a.desktopWindows.Open(request)
	if err != nil {
		a.appendLog("打开独立窗口失败: " + err.Error())
		return
	}
	if created {
		a.appendLog(fmt.Sprintf("已打开%s窗口: %s", desktopWindowRoleLabel(role), workspaceName))
	}
}

func desktopWindowRequest(role windowing.Role, workspaceID string, workspaceName string) windowing.Request {
	workspaceName = strings.TrimSpace(workspaceName)
	if workspaceName == "" {
		workspaceName = "未命名工作区"
	}
	request := windowing.Request{Role: role, WorkspaceID: strings.TrimSpace(workspaceID)}
	switch role {
	case windowing.RoleCanvas:
		request.Title = "工作流画布 - " + workspaceName
		request.Size = windowing.DpSize{Width: unit.Dp(1180), Height: unit.Dp(820)}
		request.MinSize = windowing.DpSize{Width: unit.Dp(720), Height: unit.Dp(520)}
	case windowing.RoleConsole:
		request.Title = "控制台 - " + workspaceName
		request.Size = windowing.DpSize{Width: unit.Dp(880), Height: unit.Dp(620)}
		request.MinSize = windowing.DpSize{Width: unit.Dp(560), Height: unit.Dp(360)}
	case windowing.RoleProgress:
		request.Title = "任务进度 - " + workspaceName
		request.Size = windowing.DpSize{Width: unit.Dp(420), Height: unit.Dp(260)}
		request.MinSize = windowing.DpSize{Width: unit.Dp(360), Height: unit.Dp(220)}
		request.TopMost = runtime.GOOS == "darwin"
	case windowing.RoleWorkspace:
		request.Title = "工作区 - " + workspaceName
		request.Size = windowing.DpSize{Width: unit.Dp(1280), Height: unit.Dp(860)}
		request.MinSize = windowing.DpSize{Width: unit.Dp(840), Height: unit.Dp(560)}
	}
	return request
}

func desktopWindowRoleLabel(role windowing.Role) string {
	switch role {
	case windowing.RoleCanvas:
		return "画布"
	case windowing.RoleConsole:
		return "控制台"
	case windowing.RoleProgress:
		return "进度"
	case windowing.RoleWorkspace:
		return "工作区"
	default:
		return "桌面"
	}
}

func (a *App) workspaceDisplayNameByID(workspaceID string) string {
	for _, workspace := range a.workspaces {
		if workspace.ID == workspaceID {
			return a.displayedWorkspaceName(workspace)
		}
	}
	return "未命名工作区"
}
