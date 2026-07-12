package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"image-studio/gio-client/internal/windowing"

	"gioui.org/font"
	"gioui.org/io/semantic"
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
	verticalPadding := unit.Dp(6)
	if spec.Style == desktopStyleMacOS {
		verticalPadding = unit.Dp(4)
	}
	return a.borderedSurface(gtx, spec.Colors.toolbar, unit.Dp(0), spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Inset{Top: verticalPadding, Bottom: verticalPadding, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.workflowCommandStatus(gtx, snap, spec, compact)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.workflowCommandGroup(gtx, spec,
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, spec, &a.workflowZoomOutButton, uiIconZoomOut, "缩小", false, compact)
						},
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, spec, &a.workflowFitButton, uiIconFit, "适合画布", false, compact)
						},
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, spec, &a.workflowZoomInButton, uiIconZoomIn, "放大", false, compact)
						},
					)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.workflowCommandGroup(gtx, spec,
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, spec, &a.workflowDetachCanvasButton, uiIconOpenWindow, "画布窗口", false, compact)
						},
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, spec, &a.workflowDetachConsoleButton, uiIconConsole, "控制台窗口", false, compact)
						},
						func(gtx layout.Context) layout.Dimensions {
							return a.workflowToolbarButton(gtx, spec, &a.workflowOpenProgressButton, uiIconProgress, "进度窗口", false, compact)
						},
					)
				}),
				layout.Flexed(1, layout.Spacer{}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.workflowToolbarButton(gtx, spec, &a.workflowToggleConsoleButton, uiIconConsole, chooseString(a.workflowConsoleOpen, "隐藏底部", "显示底部"), a.workflowConsoleOpen, compact)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.workflowToolbarButton(gtx, spec, &a.workflowResetGraphButton, uiIconRefresh, "重置布局", false, compact)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if snap.Running {
						return a.workflowPrimaryCommand(gtx, spec, &a.workflowCancelButton, uiIconCancel, "停止", spec.Colors.danger)
					}
					return a.workflowPrimaryCommand(gtx, spec, &a.workflowRunButton, uiIconPlay, chooseString(compact, "运行", "运行工作流"), spec.Colors.accent)
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
						return a.singleLineLabel(gtx, a.currentWorkspaceDisplayName(), workflowTextSize(spec, 12, 11), spec.Colors.text, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, state, workflowTextSize(spec, 11, 9), stateText, font.Medium)
					}),
				)
			}),
		)
	})
}

func (a *App) layoutWorkflowBody(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	panes := resolveWorkflowPaneWidths(gtx.Metric.PxToDp(gtx.Constraints.Max.X), spec)
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, panes.Left, func(gtx layout.Context) layout.Dimensions {
				return a.layoutWorkflowLibrary(gtx, snap, spec)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.layoutWorkflowCenter(gtx, snap, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, panes.Right, func(gtx layout.Context) layout.Dimensions {
				return a.layoutWorkflowInspector(gtx, snap, spec)
			})
		}),
	)
}

type workflowPaneWidths struct {
	Left   unit.Dp
	Center unit.Dp
	Right  unit.Dp
}

func resolveWorkflowPaneWidths(available unit.Dp, spec desktopThemeTokens) workflowPaneWidths {
	if available < 0 {
		available = 0
	}
	left := spec.Metrics.LeftPaneWidth
	right := spec.Metrics.RightPaneWidth
	if spec.Style == desktopStyleMacOS {
		left = unit.Dp(408)
		right = unit.Dp(352)
		const (
			centerMinimum = unit.Dp(400)
			leftMinimum   = unit.Dp(304)
			rightMinimum  = unit.Dp(320)
		)
		overflow := left + right + centerMinimum - available
		if overflow > 0 {
			reduction := min(overflow, left-leftMinimum)
			left -= reduction
			overflow -= reduction
		}
		if overflow > 0 {
			reduction := min(overflow, right-rightMinimum)
			right -= reduction
		}
	} else if available < unit.Dp(1260) {
		left = unit.Dp(224)
		right = unit.Dp(284)
	}
	center := available - left - right
	if center < 0 {
		center = 0
	}
	return workflowPaneWidths{Left: left, Center: center, Right: right}
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

func (a *App) workflowCommandGroup(gtx layout.Context, spec desktopThemeTokens, widgets ...layout.Widget) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(widgets))
	for _, child := range widgets {
		child := child
		children = append(children, layout.Rigid(child))
	}
	content := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(2))}.Layout(gtx, children...)
	}
	if spec.Style != desktopStyleMacOS {
		return content(gtx)
	}
	return a.borderedSurface(gtx, spec.Colors.surface, unit.Dp(12), spec.Colors.border, content)
}

func (a *App) workflowToolbarButton(gtx layout.Context, spec desktopThemeTokens, button *widget.Clickable, icon *widget.Icon, label string, selected bool, compact bool) layout.Dimensions {
	if spec.Style != desktopStyleMacOS {
		return fixedHeight(gtx, spec.Metrics.ControlHeight, func(gtx layout.Context) layout.Dimensions {
			if compact {
				return a.headerIconButtonIcon(gtx, button, icon, selected, label)
			}
			return a.compactIconTextButton(gtx, button, icon, label, selected)
		})
	}
	textSize := workflowTextSize(spec, 12, 11)
	height := minimumTextControlHeight(gtx, spec.Metrics.ControlHeight, a.scaledSp(textSize), unit.Dp(8))
	background := rgba(0xffffff, 0x00)
	hoverBackground := spec.Colors.toolHoverBg
	border := rgba(0xffffff, 0x00)
	foreground := spec.Colors.textMuted
	if button.Hovered() {
		foreground = spec.Colors.toolHoverText
	}
	if selected {
		background = spec.Colors.accentSoft
		hoverBackground = withAlpha(spec.Colors.accent, 0x28)
		border = withAlpha(spec.Colors.accent, 0x24)
		foreground = spec.Colors.accentText
	}
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		buttonLayout := func(gtx layout.Context) layout.Dimensions {
			return a.surfaceButton(gtx, button, background, hoverBackground, border, spec.Metrics.ControlRadius, layout.Inset{Left: 8, Right: 8}, func(gtx layout.Context) layout.Dimensions {
				semantic.LabelOp(label).Add(gtx.Ops)
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					children := []layout.FlexChild{
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
									return icon.Layout(gtx, foreground)
								})
							})
						}),
					}
					if !compact {
						children = append(children,
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, label, textSize, foreground, font.Medium)
							}),
						)
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				})
			}, selected)
		}
		if compact {
			return fixedWidth(gtx, spec.Metrics.IconTargetSize, buttonLayout)
		}
		return buttonLayout(gtx)
	})
}

func (a *App) workflowPrimaryCommand(gtx layout.Context, spec desktopThemeTokens, button *widget.Clickable, icon *widget.Icon, label string, colorValue color.NRGBA) layout.Dimensions {
	foreground := desktopReadableText(colorValue)
	textSize := workflowTextSize(spec, 12, 11)
	height := minimumTextControlHeight(gtx, spec.Metrics.ControlHeight, a.scaledSp(textSize), unit.Dp(8))
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		return a.surfaceButton(gtx, button, colorValue, withAlpha(colorValue, 0xe6), withAlpha(colorValue, 0xff), spec.Metrics.ControlRadius, layout.Inset{Left: 12, Right: 12}, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return icon.Layout(gtx, foreground)
							})
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, label, textSize, foreground, font.SemiBold)
					}),
				)
			})
		})
	})
}

func workflowTextSize(spec desktopThemeTokens, macOS unit.Sp, standard unit.Sp) unit.Sp {
	if spec.Style == desktopStyleMacOS {
		return macOS
	}
	return standard
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
