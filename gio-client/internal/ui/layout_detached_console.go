package ui

import (
	"fmt"
	"image"
	"strings"

	"image-studio/gio-client/internal/windowing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

const detachedConsoleAllWorkspaces = "__all_workspaces__"

func (view *desktopWindowView) layoutDetachedConsole(
	gtx layout.Context,
	spec desktopThemeTokens,
	publication desktopPublication,
) layout.Dimensions {
	workspace, ok := view.boundWorkspace(publication)
	if !ok {
		return view.missingWorkspace(gtx, spec, publication)
	}

	for view.clearButton.Clicked(gtx) {
		view.enqueue(desktopCommand{Kind: desktopCommandClearLogs})
	}
	for view.runButton.Clicked(gtx) {
		view.enqueue(desktopCommand{Kind: desktopCommandRun, WorkspaceID: workspace.ID})
	}
	for view.cancelButton.Clicked(gtx) {
		view.enqueue(desktopCommand{Kind: desktopCommandCancel})
	}
	for view.openCanvasButton.Clicked(gtx) {
		view.enqueueOpen(windowing.RoleCanvas, workspace.ID)
	}
	view.updateConsoleWorkspaceFilter(gtx, publication)

	logs := view.filteredConsoleLogs(publication)
	context := fmt.Sprintf("%s · %d/%d · %d 条", detachedStatusLabel(publication), publication.Completed, max(publication.Total, 1), len(logs))
	if view.commandError != "" {
		context = view.commandError
	}
	runAction := func(gtx layout.Context) layout.Dimensions {
		if publication.Running {
			return view.button(gtx, spec, &view.cancelButton, view.icons.cancel, "取消", desktopButtonDanger)
		}
		return view.button(gtx, spec, &view.runButton, view.icons.play, "运行", desktopButtonPrimary)
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return view.layoutToolbar(gtx, spec, "控制台", context,
				runAction,
				func(gtx layout.Context) layout.Dimensions {
					return view.button(gtx, spec, &view.clearButton, view.icons.clear, "清空", desktopButtonNeutral)
				},
				func(gtx layout.Context) layout.Dimensions {
					return view.button(gtx, spec, &view.openCanvasButton, view.icons.canvas, "画布", desktopButtonNeutral)
				},
				func(gtx layout.Context) layout.Dimensions {
					return view.button(gtx, spec, &view.raiseMainButton, view.icons.main, "主窗口", desktopButtonNeutral)
				},
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return view.layoutConsoleWorkspaceSelector(gtx, spec, publication)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if publication.LastError == "" {
				return layout.Dimensions{}
			}
			height := gtx.Dp(unit.Dp(32))
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
			paint.FillShape(gtx.Ops, spec.Colors.dangerSoft, clip.Rect{Max: gtx.Constraints.Max}.Op())
			return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(7), Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return view.label(gtx, publication.LastError, unit.Sp(11), spec.Colors.dangerText, font.Medium, 1)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			paint.FillShape(gtx.Ops, spec.Colors.bg2, clip.Rect{Max: gtx.Constraints.Max}.Op())
			if len(logs) == 0 {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					message := "尚无控制台输出"
					if view.consoleFilterID != "" {
						message = "该工作区暂无可识别的专属日志"
					}
					return view.label(gtx, message, unit.Sp(12), spec.Colors.textDim, font.Medium, 2)
				})
			}
			return view.consoleList.Layout(gtx, len(logs), func(gtx layout.Context, index int) layout.Dimensions {
				return view.layoutConsoleLogRow(gtx, spec, index, logs[index])
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			height := gtx.Dp(spec.Metrics.StatusBarHeight)
			if height < gtx.Dp(unit.Dp(28)) {
				height = gtx.Dp(unit.Dp(28))
			}
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
			paint.FillShape(gtx.Ops, spec.Colors.toolbar, clip.Rect{Max: gtx.Constraints.Max}.Op())
			paint.FillShape(gtx.Ops, spec.Colors.border, clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, 1)).Op())
			return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return view.statusChip(gtx, spec, detachedStatusLabel(publication), detachedStatusColor(spec, publication))
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return view.label(gtx, workspace.Name, unit.Sp(10), spec.Colors.textMuted, font.Normal, 1)
					}),
				)
			})
		}),
	)
}

func (view *desktopWindowView) updateConsoleWorkspaceFilter(gtx layout.Context, publication desktopPublication) {
	for view.workspaceButton(detachedConsoleAllWorkspaces).Clicked(gtx) {
		view.consoleFilterID = ""
	}
	for _, workspace := range publication.Workspaces {
		workspace := workspace
		for view.workspaceButton("console:" + workspace.ID).Clicked(gtx) {
			view.consoleFilterID = workspace.ID
			view.enqueue(desktopCommand{Kind: desktopCommandActivate, WorkspaceID: workspace.ID})
		}
	}
}

func (view *desktopWindowView) layoutConsoleWorkspaceSelector(
	gtx layout.Context,
	spec desktopThemeTokens,
	publication desktopPublication,
) layout.Dimensions {
	height := max(gtx.Dp(unit.Dp(38)), gtx.Dp(spec.Metrics.WorkspaceBarHeight))
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
	paint.FillShape(gtx.Ops, spec.Colors.panel, clip.Rect{Max: gtx.Constraints.Max}.Op())
	paint.FillShape(gtx.Ops, spec.Colors.border, clip.Rect(image.Rect(0, height-1, gtx.Constraints.Max.X, height)).Op())
	return layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8), Top: unit.Dp(4), Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		length := len(publication.Workspaces) + 1
		return view.workspaceList.Layout(gtx, length, func(gtx layout.Context, index int) layout.Dimensions {
			if index == 0 {
				tone := desktopButtonNeutral
				if view.consoleFilterID == "" {
					tone = desktopButtonSelected
				}
				return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return view.button(gtx, spec, view.workspaceButton(detachedConsoleAllWorkspaces), view.icons.console, "全部", tone)
				})
			}
			workspace := publication.Workspaces[index-1]
			tone := desktopButtonNeutral
			if view.consoleFilterID == workspace.ID {
				tone = desktopButtonSelected
			}
			return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return view.button(gtx, spec, view.workspaceButton("console:"+workspace.ID), view.icons.workspace, workspace.Name, tone)
			})
		})
	})
}

func (view *desktopWindowView) filteredConsoleLogs(publication desktopPublication) []string {
	if view.consoleFilterID == "" {
		return publication.Logs
	}
	workspace, ok := publication.workspace(view.consoleFilterID)
	if !ok {
		view.consoleFilterID = ""
		return publication.Logs
	}
	idNeedle := strings.ToLower(workspace.ID)
	nameNeedle := strings.ToLower(strings.TrimSpace(workspace.Name))
	logs := make([]string, 0, len(publication.Logs))
	for _, line := range publication.Logs {
		lower := strings.ToLower(line)
		if strings.Contains(lower, idNeedle) || (nameNeedle != "" && strings.Contains(lower, nameNeedle)) {
			logs = append(logs, line)
		}
	}
	return logs
}

func (view *desktopWindowView) layoutConsoleLogRow(
	gtx layout.Context,
	spec desktopThemeTokens,
	index int,
	line string,
) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	dims := layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(7), Bottom: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		lineColor := spec.Colors.textMuted
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(line, "错误") || strings.Contains(line, "失败") || strings.Contains(line, "异常") {
			lineColor = spec.Colors.dangerText
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.monoLabel(gtx, fmt.Sprintf("%04d", index+1), unit.Sp(10), spec.Colors.textDim, 1)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return view.monoLabel(gtx, line, unit.Sp(11), lineColor, 3)
			}),
		)
	})
	call := macro.Stop()
	background := spec.Colors.bg2
	if index%2 == 1 {
		background = spec.Colors.panel2
	}
	paint.FillShape(gtx.Ops, background, clip.Rect{Max: dims.Size}.Op())
	paint.FillShape(gtx.Ops, spec.Colors.border, clip.Rect(image.Rect(0, dims.Size.Y-1, dims.Size.X, dims.Size.Y)).Op())
	call.Add(gtx.Ops)
	return dims
}
