package ui

import (
	"fmt"
	"image"
	"image/color"

	"image-studio/gio-client/internal/windowing"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

func (a *App) layoutWorkflowLibrary(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	for a.workflowAddWorkspaceButton.Clicked(gtx) {
		a.createWorkspace()
	}
	for _, workspace := range a.workspaces {
		workspace := workspace
		for a.workflowSidebarWorkspaceButton(workspace.ID).Clicked(gtx) {
			a.switchWorkspace(workspace.ID)
		}
		for a.workflowWorkspaceWindowButton(workspace.ID).Clicked(gtx) {
			a.openDesktopWindow(windowing.RoleWorkspace, workspace.ID)
		}
	}
	data := a.workflowCanvasData(snap, a.activeWorkspaceID)
	for _, node := range data.Graph.Nodes {
		node := node
		for a.workflowNodeButton(node.ID).Clicked(gtx) {
			a.selectWorkflowNode(a.activeWorkspaceID, node.ID)
		}
	}

	return a.borderedSurface(gtx, spec.Colors.sidebar, unit.Dp(0), spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return a.workflowLibraryList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			return layout.Inset{Top: 14, Bottom: 16, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(14))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.workflowLibraryHeader(gtx, spec)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.workflowWorkspaceSection(gtx, spec)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return workflowDivider(gtx, spec.Colors.border)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.workflowNodeLibrarySection(gtx, data, spec)
					}),
				)
			})
		})
	})
}

func (a *App) workflowLibraryHeader(gtx layout.Context, spec desktopThemeTokens) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "工作流资源", unit.Sp(12), spec.Colors.text, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "工作区与真实执行节点", unit.Sp(9), spec.Colors.textDim, font.Normal)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.headerIconButtonIcon(gtx, &a.workflowAddWorkspaceButton, uiIconAdd, false, "新建工作区")
		}),
	)
}

func (a *App) workflowWorkspaceSection(gtx layout.Context, spec desktopThemeTokens) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowSectionTitle(gtx, "工作区", fmt.Sprintf("%d", len(a.workspaces)), spec)
		}),
	}
	for _, workspace := range a.workspaces {
		workspace := workspace
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowWorkspaceRow(gtx, workspace, workspace.ID == a.activeWorkspaceID, spec)
		}))
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx, children...)
}

func (a *App) workflowWorkspaceRow(gtx layout.Context, workspace workspaceState, active bool, spec desktopThemeTokens) layout.Dimensions {
	button := a.workflowSidebarWorkspaceButton(workspace.ID)
	openButton := a.workflowWorkspaceWindowButton(workspace.ID)
	bg := rgba(0xffffff, 0x00)
	if active {
		bg = withAlpha(spec.Colors.accent, 0x18)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.surfaceButton(gtx, button, bg, withAlpha(spec.Colors.accent, 0x12), withAlpha(spec.Colors.accent, 0x22), spec.Metrics.ControlRadius, layout.Inset{Top: 7, Bottom: 7, Left: 9, Right: 9}, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(7))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						colorValue := spec.Colors.textDim
						if active {
							colorValue = spec.Colors.accent
						}
						return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return uiIconWorkspace.Layout(gtx, colorValue)
							})
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, a.displayedWorkspaceName(workspace), unit.Sp(10), spec.Colors.text, chooseFontWeight(active))
					}),
				)
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
					return a.surfaceButton(gtx, openButton, rgba(0xffffff, 0x00), spec.Colors.surface2, rgba(0xffffff, 0x00), spec.Metrics.ControlRadius, layout.Inset{Top: 7, Bottom: 7, Left: 7, Right: 7}, func(gtx layout.Context) layout.Dimensions {
						semantic.LabelOp("在独立窗口打开" + a.displayedWorkspaceName(workspace)).Add(gtx.Ops)
						return uiIconOpenWindow.Layout(gtx, spec.Colors.textMuted)
					})
				})
			})
		}),
	)
}

func (a *App) workflowNodeLibrarySection(gtx layout.Context, data workflowCanvasData, spec desktopThemeTokens) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowSectionTitle(gtx, "执行节点", fmt.Sprintf("%d", len(data.Graph.Nodes)), spec)
		}),
	}
	for _, node := range data.Graph.Nodes {
		node := node
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowNodeLibraryRow(gtx, node, data.Runtime[node.ID], data.Selected == node.ID, spec)
		}))
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx, children...)
}

func (a *App) workflowNodeLibraryRow(gtx layout.Context, node workflowNodeModel, runtimeState workflowNodeRuntime, selected bool, spec desktopThemeTokens) layout.Dimensions {
	button := a.workflowNodeButton(node.ID)
	bg := rgba(0xffffff, 0x00)
	if selected {
		bg = spec.Colors.surface2
	}
	return a.surfaceButton(gtx, button, bg, spec.Colors.surface2, withAlpha(spec.Colors.accent, 0x14), spec.Metrics.ControlRadius, layout.Inset{Top: 8, Bottom: 8, Left: 9, Right: 9}, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				colorValue := workflowNodePhaseColor(spec, runtimeState.Phase)
				return fixedWidth(gtx, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
						size := gtx.Dp(unit.Dp(8))
						paint.FillShape(gtx.Ops, colorValue, clip.Ellipse(image.Rect(0, 0, size, size)).Op(gtx.Ops))
						return layout.Dimensions{Size: image.Pt(size, size)}
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, node.Title, unit.Sp(10), spec.Colors.text, chooseFontWeight(selected))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, node.Subtitle, unit.Sp(9), spec.Colors.textDim, font.Normal)
					}),
				)
			}),
		)
	})
}

func (a *App) workflowSectionTitle(gtx layout.Context, title string, count string, spec desktopThemeTokens) layout.Dimensions {
	return layout.Inset{Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, title, unit.Sp(9), spec.Colors.textMuted, font.SemiBold)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.monoLabel(gtx, count, unit.Sp(9), spec.Colors.textDim, font.Medium)
			}),
		)
	})
}

func workflowDivider(gtx layout.Context, colorValue color.NRGBA) layout.Dimensions {
	height := max(1, gtx.Dp(unit.Dp(1)))
	paint.FillShape(gtx.Ops, colorValue, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, height)}.Op())
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, height)}
}
