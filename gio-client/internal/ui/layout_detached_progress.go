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

func (view *desktopWindowView) layoutDetachedProgress(
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
	for view.openCanvasButton.Clicked(gtx) {
		view.enqueueOpen(windowing.RoleCanvas, workspace.ID)
	}

	total := max(workspace.Total, 1)
	completed := max(workspace.Completed, 0)
	workspaceRunning := desktopWorkspaceRunning(publication, workspace.ID)
	if !workspaceRunning {
		total = 1
		completed = 0
		if workspace.ResultImage != nil || strings.TrimSpace(workspace.ResultSavedPath) != "" {
			completed = 1
		}
	}
	progress := float32(completed) / float32(total)
	status := detachedWorkspaceStatusLabel(publication, workspace.ID)
	if view.commandError != "" {
		status = view.commandError
	}

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return view.layoutToolbar(gtx, spec, workspace.Name, status,
				func(gtx layout.Context) layout.Dimensions {
					return view.button(gtx, spec, &view.raiseMainButton, view.icons.main, "主窗口", desktopButtonNeutral)
				},
			)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(10), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				previewSize := min(gtx.Constraints.Max.Y, gtx.Dp(unit.Dp(128)))
				previewSize = min(previewSize, max(gtx.Dp(unit.Dp(82)), gtx.Constraints.Max.X/3))
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints = layout.Exact(image.Pt(previewSize, previewSize))
						return view.imagePreview(gtx, spec, workspace.ResultImage, workspace.ResultRevision)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(14)}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return view.statusChip(gtx, spec, status, detachedWorkspaceStatusColor(spec, publication, workspace.ID))
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return view.label(gtx, fmt.Sprintf("%d / %d", completed, total), unit.Sp(12), spec.Colors.text, font.SemiBold, 1)
									}),
								)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return view.progressBar(gtx, spec, progress)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								meta := strings.Join([]string{workspace.Mode, workspace.Size, workspace.Quality}, " · ")
								return view.label(gtx, meta, unit.Sp(10), spec.Colors.textMuted, font.Normal, 1)
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(5)}.Layout),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								detail := workspace.Status
								colorValue := spec.Colors.textMuted
								if !workspaceRunning {
									detail = workspace.ResultSavedPath
									if desktopWorkspaceQueued(publication, workspace.ID) {
										detail = "当前任务结束后将自动运行此工作区"
									} else if strings.TrimSpace(detail) == "" {
										detail = "工作区就绪"
									}
								} else if workspace.LastError != "" {
									detail = workspace.LastError
									colorValue = spec.Colors.dangerText
								}
								return view.label(gtx, detail, unit.Sp(10), colorValue, font.Normal, 3)
							}),
						)
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			height := max(gtx.Dp(unit.Dp(42)), gtx.Dp(spec.Metrics.CommandBarHeight))
			gtx.Constraints = layout.Exact(image.Pt(gtx.Constraints.Max.X, height))
			paint.FillShape(gtx.Ops, spec.Colors.toolbar, clip.Rect{Max: gtx.Constraints.Max}.Op())
			paint.FillShape(gtx.Ops, spec.Colors.border, clip.Rect(image.Rect(0, 0, gtx.Constraints.Max.X, 1)).Op())
			return layout.Inset{Left: unit.Dp(12), Right: unit.Dp(12), Top: unit.Dp(5), Bottom: unit.Dp(5)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceEnd}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return view.button(gtx, spec, &view.openCanvasButton, view.icons.canvas, "打开画布", desktopButtonNeutral)
					}),
					layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if workspaceRunning {
							return view.button(gtx, spec, &view.cancelButton, view.icons.cancel, "取消任务", desktopButtonDanger)
						}
						label := "再次运行"
						if publication.Running {
							label = "加入队列"
						}
						return view.button(gtx, spec, &view.runButton, view.icons.play, label, desktopButtonPrimary)
					}),
				)
			})
		}),
	)
}
