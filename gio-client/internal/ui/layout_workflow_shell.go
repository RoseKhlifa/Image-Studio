package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"image-studio/gio-client/internal/windowing"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

func (a *App) layoutWorkflowShell(gtx layout.Context, snap snapshot) layout.Dimensions {
	a.handleWorkflowCommandEvents(gtx, snap)
	spec := desktopThemeSpec(a.desktopStyle, a.resolvedThemeMode)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, spec.Metrics.CommandBarHeight, func(gtx layout.Context) layout.Dimensions {
				return a.layoutWorkflowCommandBar(gtx, snap, spec)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.layoutWorkflowBody(gtx, snap, spec)
		}),
	)
}

func (a *App) handleWorkflowCommandEvents(gtx layout.Context, snap snapshot) {
	for a.workflowRunButton.Clicked(gtx) {
		if !snap.Running {
			a.startRun()
		}
	}
	for a.workflowCancelButton.Clicked(gtx) {
		if snap.Running {
			a.cancelRun()
		}
	}
	for a.workflowZoomOutButton.Clicked(gtx) {
		a.workflowCanvas.zoomBy(-0.1)
		a.invalidateNow()
	}
	for a.workflowZoomInButton.Clicked(gtx) {
		a.workflowCanvas.zoomBy(0.1)
		a.invalidateNow()
	}
	for a.workflowFitButton.Clicked(gtx) {
		a.workflowCanvas.resetViewport()
		a.invalidateNow()
	}
	for a.workflowResetGraphButton.Clicked(gtx) {
		a.resetWorkflowGraph(a.activeWorkspaceID)
	}
	for a.workflowToggleConsoleButton.Clicked(gtx) {
		a.workflowConsoleOpen = !a.workflowConsoleOpen
		a.invalidateNow()
	}
	for a.workflowDetachCanvasButton.Clicked(gtx) {
		a.openDesktopWindow(windowing.RoleCanvas, a.activeWorkspaceID)
	}
	for a.workflowDetachConsoleButton.Clicked(gtx) {
		a.openDesktopWindow(windowing.RoleConsole, a.activeWorkspaceID)
	}
	for a.workflowDockDetachConsoleButton.Clicked(gtx) {
		a.openDesktopWindow(windowing.RoleConsole, a.activeWorkspaceID)
	}
	for a.workflowOpenProgressButton.Clicked(gtx) {
		a.openDesktopWindow(windowing.RoleProgress, a.activeWorkspaceID)
	}
}

func (a *App) layoutWorkflowCommandBar(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	compact := gtx.Constraints.Max.X < gtx.Dp(unit.Dp(1180))
	return a.borderedSurface(gtx, spec.Colors.toolbar, unit.Dp(0), spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.workflowCommandStatus(gtx, snap, spec, compact)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.workflowCommandGroup(gtx,
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, &a.workflowZoomOutButton, uiIconZoomOut, "缩小", false, compact)
						},
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, &a.workflowFitButton, uiIconFit, "适合画布", false, compact)
						},
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, &a.workflowZoomInButton, uiIconZoomIn, "放大", false, compact)
						},
					)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.workflowCommandGroup(gtx,
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, &a.workflowDetachCanvasButton, uiIconOpenWindow, "画布窗口", false, compact)
						},
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, &a.workflowDetachConsoleButton, uiIconConsole, "控制台窗口", false, compact)
						},
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, &a.workflowOpenProgressButton, uiIconProgress, "进度窗口", false, compact)
						},
					)
				}),
				layout.Flexed(1, layout.Spacer{}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.workflowToolbarButton(gtx, &a.workflowToggleConsoleButton, uiIconConsole, chooseString(a.workflowConsoleOpen, "隐藏底部", "显示底部"), a.workflowConsoleOpen, compact)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.workflowToolbarButton(gtx, &a.workflowResetGraphButton, uiIconRefresh, "重置布局", false, compact)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if snap.Running {
						return a.workflowPrimaryCommand(gtx, &a.workflowCancelButton, uiIconCancel, "停止", spec.Colors.danger)
					}
					return a.workflowPrimaryCommand(gtx, &a.workflowRunButton, uiIconPlay, chooseString(compact, "运行", "运行工作流"), spec.Colors.accent)
				}),
			)
		})
	})
}

func (a *App) workflowCommandStatus(gtx layout.Context, snap snapshot, spec desktopThemeTokens, compact bool) layout.Dimensions {
	stateFill := spec.Colors.textDim
	stateText := spec.Colors.textDim
	state := "就绪"
	if snap.Running {
		state = "运行中"
		stateFill = spec.Colors.accent
		stateText = spec.Colors.accentText
	} else if strings.TrimSpace(snap.LastErrorMessage) != "" {
		state = "需要处理"
		stateFill = spec.Colors.danger
		stateText = spec.Colors.dangerText
	} else if snap.Result.HasItem {
		state = "已完成"
		stateFill = spec.Colors.success
		stateText = spec.Colors.successText
	}
	width := unit.Dp(190)
	if compact {
		width = unit.Dp(150)
	}
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
						return a.surface(gtx, stateFill, unit.Dp(4), layout.Spacer{}.Layout)
					})
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, a.currentWorkspaceDisplayName(), unit.Sp(11), spec.Colors.text, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, state, unit.Sp(9), stateText, font.Medium)
					}),
				)
			}),
		)
	})
}

func (a *App) layoutWorkflowBody(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	leftWidth := spec.Metrics.LeftPaneWidth
	rightWidth := spec.Metrics.RightPaneWidth
	if gtx.Constraints.Max.X < gtx.Dp(unit.Dp(1260)) {
		leftWidth = unit.Dp(224)
		rightWidth = unit.Dp(284)
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, leftWidth, func(gtx layout.Context) layout.Dimensions {
				return a.layoutWorkflowLibrary(gtx, snap, spec)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.layoutWorkflowCenter(gtx, snap, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, rightWidth, func(gtx layout.Context) layout.Dimensions {
				return a.layoutWorkflowInspector(gtx, snap, spec)
			})
		}),
	)
}

func (a *App) layoutWorkflowCenter(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	data := a.workflowCanvasData(snap, a.activeWorkspaceID)
	children := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.workflowCanvas.Layout(gtx, a.th, spec, data, workflowCanvasCallbacks{
				Select: func(nodeID string) {
					a.selectWorkflowNode(a.activeWorkspaceID, nodeID)
				},
				Move: func(nodeID string, position image.Point) {
					a.setWorkflowNodePosition(a.activeWorkspaceID, nodeID, position)
				},
			})
		}),
	}
	if a.workflowConsoleOpen {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, spec.Metrics.ConsoleHeight, func(gtx layout.Context) layout.Dimensions {
				return a.layoutWorkflowBottomDock(gtx, snap, spec)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) workflowCommandGroup(gtx layout.Context, widgets ...layout.Widget) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(widgets))
	for _, child := range widgets {
		child := child
		children = append(children, layout.Rigid(child))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(2))}.Layout(gtx, children...)
}

func (a *App) workflowToolbarButton(gtx layout.Context, button *widget.Clickable, icon *widget.Icon, label string, selected bool, compact bool) layout.Dimensions {
	return fixedHeight(gtx, desktopThemeSpec(a.desktopStyle, a.resolvedThemeMode).Metrics.ControlHeight, func(gtx layout.Context) layout.Dimensions {
		if compact {
			return a.headerIconButtonIcon(gtx, button, icon, selected, label)
		}
		return a.compactIconTextButton(gtx, button, icon, label, selected)
	})
}

func (a *App) workflowPrimaryCommand(gtx layout.Context, button *widget.Clickable, icon *widget.Icon, label string, colorValue color.NRGBA) layout.Dimensions {
	foreground := desktopReadableText(colorValue)
	return a.surfaceButton(gtx, button, colorValue, withAlpha(colorValue, 0xe6), withAlpha(colorValue, 0xff), fluentControlRadius, layout.Inset{Top: 7, Bottom: 7, Left: 12, Right: 12}, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
						return icon.Layout(gtx, foreground)
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, label, unit.Sp(11), foreground, font.SemiBold)
			}),
		)
	})
}

func chooseString(condition bool, whenTrue string, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}

func (a *App) workflowNodeButton(id string) *widget.Clickable {
	if a.workflowNodeButtons == nil {
		a.workflowNodeButtons = map[string]*widget.Clickable{}
	}
	if button, ok := a.workflowNodeButtons[id]; ok {
		return button
	}
	button := new(widget.Clickable)
	a.workflowNodeButtons[id] = button
	return button
}

func (a *App) workflowSidebarWorkspaceButton(id string) *widget.Clickable {
	if a.workflowSidebarWorkspaceButtons == nil {
		a.workflowSidebarWorkspaceButtons = map[string]*widget.Clickable{}
	}
	if button, ok := a.workflowSidebarWorkspaceButtons[id]; ok {
		return button
	}
	button := new(widget.Clickable)
	a.workflowSidebarWorkspaceButtons[id] = button
	return button
}

func (a *App) workflowWorkspaceWindowButton(id string) *widget.Clickable {
	if a.workflowWorkspaceWindowButtons == nil {
		a.workflowWorkspaceWindowButtons = map[string]*widget.Clickable{}
	}
	if button, ok := a.workflowWorkspaceWindowButtons[id]; ok {
		return button
	}
	button := new(widget.Clickable)
	a.workflowWorkspaceWindowButtons[id] = button
	return button
}

func workflowPercent(value float32) string {
	return fmt.Sprintf("%d%%", int(value*100))
}
