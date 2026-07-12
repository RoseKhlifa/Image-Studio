package ui

import (
	"fmt"
	"image"
	"strings"

	"image-studio/gio-client/internal/windowing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

func (view *desktopWindowView) layoutDetachedWorkspace(
	gtx layout.Context,
	spec desktopThemeTokens,
	publication desktopPublication,
) layout.Dimensions {
	workspace, ok := view.boundWorkspace(publication)
	if !ok {
		return view.missingWorkspace(gtx, spec, publication)
	}
	view.syncDetachedWorkspaceDraft(workspace)
	view.handleDetachedWorkspaceDraftButtons(gtx)

	for view.activateButton.Clicked(gtx) {
		view.enqueue(desktopCommand{Kind: desktopCommandActivate, WorkspaceID: workspace.ID})
	}
	for view.runButton.Clicked(gtx) {
		view.enqueueDetachedDraftAndRun(workspace.ID)
	}
	for view.applyDraftButton.Clicked(gtx) {
		view.enqueueDetachedDraft(workspace.ID)
	}
	for view.cancelButton.Clicked(gtx) {
		view.enqueue(desktopCommand{Kind: desktopCommandCancel})
	}
	for view.openCanvasButton.Clicked(gtx) {
		view.enqueueOpen(windowing.RoleCanvas, workspace.ID)
	}
	for view.openConsoleButton.Clicked(gtx) {
		view.enqueueOpen(windowing.RoleConsole, workspace.ID)
	}
	for view.openProgressButton.Clicked(gtx) {
		view.enqueueOpen(windowing.RoleProgress, workspace.ID)
	}

	actions := make([]layout.Widget, 0, 6)
	if workspace.ID != publication.ActiveID {
		actions = append(actions, func(gtx layout.Context) layout.Dimensions {
			return view.button(gtx, spec, &view.activateButton, view.icons.activate, "激活", desktopButtonNeutral)
		})
	}
	actions = append(actions,
		func(gtx layout.Context) layout.Dimensions {
			if desktopWorkspaceRunning(publication, workspace.ID) {
				return view.button(gtx, spec, &view.cancelButton, view.icons.cancel, "取消", desktopButtonDanger)
			}
			label := "运行"
			if publication.Running {
				label = "加入队列"
			}
			return view.button(gtx, spec, &view.runButton, view.icons.play, label, desktopButtonPrimary)
		},
		func(gtx layout.Context) layout.Dimensions {
			return view.button(gtx, spec, &view.openCanvasButton, view.icons.canvas, "画布", desktopButtonNeutral)
		},
		func(gtx layout.Context) layout.Dimensions {
			return view.button(gtx, spec, &view.openConsoleButton, view.icons.console, "控制台", desktopButtonNeutral)
		},
		func(gtx layout.Context) layout.Dimensions {
			return view.button(gtx, spec, &view.openProgressButton, view.icons.progress, "进度", desktopButtonNeutral)
		},
		func(gtx layout.Context) layout.Dimensions {
			return view.button(gtx, spec, &view.raiseMainButton, view.icons.main, "主窗口", desktopButtonNeutral)
		},
	)
	context := detachedWorkspaceStatusLabel(publication, workspace.ID)
	if view.commandError != "" {
		context = view.commandError
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return view.layoutToolbar(gtx, spec, workspace.Name, context, actions...)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return view.layoutWorkspaceStatusBand(gtx, spec, publication, workspace)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
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
					width := gtx.Dp(spec.Metrics.RightPaneWidth)
					width = max(width, gtx.Dp(unit.Dp(292)))
					gtx.Constraints = layout.Exact(image.Pt(width, gtx.Constraints.Max.Y))
					return view.layoutDetachedWorkspaceInspector(gtx, spec, workspace)
				}),
			)
		}),
	)
}

func (view *desktopWindowView) layoutWorkspaceStatusBand(
	gtx layout.Context,
	spec desktopThemeTokens,
	publication desktopPublication,
	workspace desktopWorkspacePublication,
) layout.Dimensions {
	height := max(gtx.Dp(unit.Dp(38)), gtx.Dp(spec.Metrics.WorkspaceBarHeight))
	gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
	paint.FillShape(gtx.Ops, spec.Colors.panel, clip.Rect{Max: gtx.Constraints.Max}.Op())
	paint.FillShape(gtx.Ops, spec.Colors.border, clip.Rect(image.Rect(0, height-1, gtx.Constraints.Max.X, height)).Op())
	prompt := strings.TrimSpace(workspace.Prompt)
	if prompt == "" {
		prompt = "未填写提示词"
	}
	meta := fmt.Sprintf("%s · %s · %s · %d 个输入", workspace.Mode, workspace.Size, workspace.Quality, workspace.SourceCount)
	return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.statusChip(gtx, spec, detachedWorkspaceStatusLabel(publication, workspace.ID), detachedWorkspaceStatusColor(spec, publication, workspace.ID))
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.label(gtx, meta, unit.Sp(10), spec.Colors.textMuted, font.Medium, 1)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return view.label(gtx, prompt, unit.Sp(11), spec.Colors.text, font.Normal, 1)
			}),
		)
	})
}
