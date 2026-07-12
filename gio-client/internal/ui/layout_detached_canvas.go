package ui

import (
	"fmt"
	"image"

	"image-studio/gio-client/internal/windowing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

func (view *desktopWindowView) layoutDetachedCanvas(
	gtx layout.Context,
	spec desktopThemeTokens,
	publication desktopPublication,
) layout.Dimensions {
	workspace, ok := view.boundWorkspace(publication)
	if !ok {
		return view.missingWorkspace(gtx, spec, publication)
	}

	for view.runButton.Clicked(gtx) {
		view.enqueue(desktopCommand{Kind: desktopCommandRun, WorkspaceID: workspace.ID})
	}
	for view.cancelButton.Clicked(gtx) {
		view.enqueue(desktopCommand{Kind: desktopCommandCancel})
	}
	for view.openConsoleButton.Clicked(gtx) {
		view.enqueueOpen(windowing.RoleConsole, workspace.ID)
	}
	for view.openProgressButton.Clicked(gtx) {
		view.enqueueOpen(windowing.RoleProgress, workspace.ID)
	}
	for view.openWorkspaceButton.Clicked(gtx) {
		view.enqueueOpen(windowing.RoleWorkspace, workspace.ID)
	}

	workspaceRunning := desktopWorkspaceRunning(publication, workspace.ID)
	runAction := func(gtx layout.Context) layout.Dimensions {
		if workspaceRunning {
			return view.button(gtx, spec, &view.cancelButton, view.icons.cancel, "取消", desktopButtonDanger)
		}
		label := "运行"
		if publication.Running {
			label = "加入队列"
		}
		return view.button(gtx, spec, &view.runButton, view.icons.play, label, desktopButtonPrimary)
	}
	context := fmt.Sprintf("%s · %d/%d", detachedWorkspaceStatusLabel(publication, workspace.ID), workspace.Completed, max(workspace.Total, 1))
	if view.commandError != "" {
		context = view.commandError
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return view.layoutToolbar(gtx, spec, workspace.Name, context,
				runAction,
				func(gtx layout.Context) layout.Dimensions {
					return view.button(gtx, spec, &view.openConsoleButton, view.icons.console, "控制台", desktopButtonNeutral)
				},
				func(gtx layout.Context) layout.Dimensions {
					return view.button(gtx, spec, &view.openProgressButton, view.icons.progress, "进度", desktopButtonNeutral)
				},
				func(gtx layout.Context) layout.Dimensions {
					return view.button(gtx, spec, &view.openWorkspaceButton, view.icons.workspace, "工作区", desktopButtonNeutral)
				},
				func(gtx layout.Context) layout.Dimensions {
					return view.button(gtx, spec, &view.raiseMainButton, view.icons.main, "主窗口", desktopButtonNeutral)
				},
			)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return view.canvas.Layout(
				gtx,
				view.theme,
				spec,
				workflowCanvasData{
					Graph:     workspace.Graph,
					Selected:  workspace.SelectedNode,
					Runtime:   workspace.Runtime,
					Workspace: workspace.ID,
				},
				view.canvasCallbacks(workspace.ID),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if workspace.LastError == "" {
				return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
			}
			return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return view.label(gtx, workspace.LastError, unit.Sp(10), spec.Colors.dangerText, font.Normal, 1)
			})
		}),
	)
}
