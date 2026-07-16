package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	sharedCompat "image-studio/shared/compat"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

var workflowBottomTabs = []struct {
	ID    string
	Label string
}{
	{ID: "queue", Label: "队列"},
	{ID: "console", Label: "控制台"},
	{ID: "errors", Label: "错误"},
	{ID: "artifacts", Label: "产物"},
}

func (a *App) layoutWorkflowBottomDock(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	for index, tab := range workflowBottomTabs {
		for a.workflowBottomTabButtons[index].Clicked(gtx) {
			a.workflowBottomTab = tab.ID
			a.invalidateNow()
		}
	}
	for a.workflowCopyLogsButton.Clicked(gtx) {
		copyResultDetailText(gtx, strings.Join(snap.Logs, "\n"))
		a.appendLog("已复制控制台日志")
	}
	for a.clearLogButton.Clicked(gtx) {
		a.clearLogs()
	}
	return a.borderedSurface(gtx, spec.Colors.panel, unit.Dp(0), spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, unit.Dp(38), func(gtx layout.Context) layout.Dimensions {
					return a.layoutWorkflowBottomTabs(gtx, snap, spec)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return workflowDivider(gtx, spec.Colors.border)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.layoutWorkflowBottomContent(gtx, snap, spec)
				})
			}),
		)
	})
}

func (a *App) layoutWorkflowBottomTabs(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	errorCount := workflowErrorCount(snap.Logs)
	artifactCount := len(snap.BatchResults)
	if artifactCount == 0 && snap.Result.HasItem {
		artifactCount = 1
	}
	return layout.Inset{Top: 4, Bottom: 4, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, len(workflowBottomTabs)+4)
		for index, tab := range workflowBottomTabs {
			index := index
			tab := tab
			label := tab.Label
			switch tab.ID {
			case "queue":
				queueCount := len(a.desktopQueuedWorkspaceRuns)
				if snap.Running {
					queueCount++
				}
				if queueCount > 0 {
					label += fmt.Sprintf(" %d", queueCount)
				}
			case "errors":
				if errorCount > 0 {
					label += fmt.Sprintf(" %d", errorCount)
				}
			case "artifacts":
				if artifactCount > 0 {
					label += fmt.Sprintf(" %d", artifactCount)
				}
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				selected := a.workflowBottomTab == tab.ID
				return a.compactButton(gtx, &a.workflowBottomTabButtons[index], label, selected, selected)
			}))
		}
		children = append(children,
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.ghostIconTextButton(gtx, &a.workflowCopyLogsButton, uiIconCopy, "复制", false)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.ghostIconTextButton(gtx, &a.clearLogButton, uiIconClear, "清空", false)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.ghostIconTextButton(gtx, &a.workflowDockDetachConsoleButton, uiIconOpenWindow, "弹出", false)
			}),
		)
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx, children...)
	})
}

func (a *App) layoutWorkflowBottomContent(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	switch a.workflowBottomTab {
	case "queue":
		return a.layoutWorkflowQueue(gtx, snap, spec)
	case "errors":
		return a.layoutWorkflowErrors(gtx, snap, spec)
	case "artifacts":
		return a.layoutWorkflowArtifacts(gtx, snap, spec)
	default:
		return a.layoutWorkflowLogLines(gtx, snap.Logs, spec)
	}
}

func (a *App) layoutWorkflowQueue(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	queuedCount := len(a.desktopQueuedWorkspaceRuns)
	if !snap.Running && queuedCount == 0 {
		return a.workflowDockEmpty(gtx, "当前没有运行中的任务", "从命令栏运行工作流后，阶段与并发进度会显示在这里。", spec)
	}
	itemCount := queuedCount
	if snap.Running {
		itemCount++
	}
	a.workflowConsoleList.List.Axis = layout.Vertical
	return a.workflowConsoleList.Layout(gtx, itemCount, func(gtx layout.Context, index int) layout.Dimensions {
		if snap.Running && index == 0 {
			return a.layoutWorkflowRunningQueueItem(gtx, snap, spec)
		}
		queueIndex := index
		position := index + 1
		if snap.Running {
			queueIndex--
			position++
		}
		return a.layoutWorkflowWaitingQueueItem(gtx, a.desktopQueuedWorkspaceRuns[queueIndex], position, spec)
	})
}

func (a *App) layoutWorkflowRunningQueueItem(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	total := max(snap.BatchTotal, 1)
	completed := len(snap.BatchResults)
	progress := float32(completed) / float32(total)
	return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, a.currentWorkspaceDisplayName(), unit.Sp(10), spec.Colors.text, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.monoLabel(gtx, fmt.Sprintf("%d / %d", completed, total), unit.Sp(10), spec.Colors.accentText, font.Medium)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.singleLineLabel(gtx, snap.Status, unit.Sp(9), spec.Colors.textMuted, font.Normal)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return workflowProgressBar(gtx, progress, spec.Colors.surface2, spec.Colors.accent)
			}),
		)
	})
}

func (a *App) layoutWorkflowWaitingQueueItem(gtx layout.Context, workspaceID string, position int, spec desktopThemeTokens) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(2), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, unit.Dp(28), func(gtx layout.Context) layout.Dimensions {
					return a.monoLabel(gtx, fmt.Sprintf("%02d", position), unit.Sp(8), spec.Colors.textDim, font.Medium)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.singleLineLabel(gtx, a.workspaceDisplayNameByID(workspaceID), unit.Sp(9), spec.Colors.textMuted, font.Normal)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, "等待", unit.Sp(8), spec.Colors.warningText, font.Medium)
			}),
		)
	})
}

func (a *App) layoutWorkflowErrors(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	lines := make([]string, 0)
	for _, line := range snap.Logs {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "失败") || strings.Contains(lower, "错误") || strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
			lines = append(lines, line)
		}
	}
	if strings.TrimSpace(snap.LastErrorMessage) != "" {
		lines = append(lines, snap.LastErrorMessage)
	}
	if len(lines) == 0 {
		return a.workflowDockEmpty(gtx, "没有错误", "当前会话未检测到失败任务或上游错误。", spec)
	}
	return a.layoutWorkflowLogLines(gtx, lines, spec)
}

func (a *App) layoutWorkflowArtifacts(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	items := append([]sharedCompat.HistoryItem(nil), snap.BatchResults...)
	if len(items) == 0 && snap.Result.HasItem {
		items = append(items, snap.Result.Item)
	}
	if len(items) == 0 {
		return a.workflowDockEmpty(gtx, "还没有产物", "完成的图像会在这里按工作流输出顺序出现。", spec)
	}
	return a.workflowConsoleList.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
		item := items[index]
		return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.monoLabel(gtx, fmt.Sprintf("%02d", index+1), unit.Sp(9), spec.Colors.textDim, font.Medium)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.singleLineLabel(gtx, chooseNonEmpty(item.Prompt, item.SavedPath), unit.Sp(9), spec.Colors.textMuted, font.Normal)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.singleLineLabel(gtx, strings.ToUpper(item.OutputFormat), unit.Sp(8), spec.Colors.successText, font.Medium)
				}),
			)
		})
	})
}

func (a *App) layoutWorkflowLogLines(gtx layout.Context, lines []string, spec desktopThemeTokens) layout.Dimensions {
	if len(lines) == 0 {
		return a.workflowDockEmpty(gtx, "控制台等待事件", "生成阶段、网络状态和保存结果会按时间写入。", spec)
	}
	return a.workflowConsoleList.Layout(gtx, len(lines), func(gtx layout.Context, index int) layout.Dimensions {
		line := lines[index]
		lineColor := spec.Colors.textMuted
		lower := strings.ToLower(line)
		if strings.Contains(lower, "失败") || strings.Contains(lower, "错误") || strings.Contains(lower, "error") {
			lineColor = spec.Colors.dangerText
		}
		return layout.Inset{Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.monoLabel(gtx, fmt.Sprintf("%04d", index+1), unit.Sp(8), spec.Colors.textDim, font.Normal)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.monoLabel(gtx, line, unit.Sp(9), lineColor, font.Normal)
				}),
			)
		})
	})
}

func (a *App) workflowDockEmpty(gtx layout.Context, title string, detail string, spec desktopThemeTokens) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, title, unit.Sp(10), spec.Colors.textMuted, font.Medium)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, detail, unit.Sp(9), spec.Colors.textDim, font.Normal)
			}),
		)
	})
}

func workflowProgressBar(gtx layout.Context, progress float32, track color.NRGBA, fill color.NRGBA) layout.Dimensions {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	width := gtx.Constraints.Max.X
	height := max(3, gtx.Dp(unit.Dp(4)))
	paint.FillShape(gtx.Ops, track, clip.RRect{Rect: image.Rect(0, 0, width, height), NE: height / 2, NW: height / 2, SE: height / 2, SW: height / 2}.Op(gtx.Ops))
	filled := int(float32(width) * progress)
	if filled > 0 {
		paint.FillShape(gtx.Ops, fill, clip.RRect{Rect: image.Rect(0, 0, filled, height), NE: height / 2, NW: height / 2, SE: height / 2, SW: height / 2}.Op(gtx.Ops))
	}
	return layout.Dimensions{Size: image.Pt(width, height)}
}

func workflowErrorCount(lines []string) int {
	count := 0
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "失败") || strings.Contains(lower, "错误") || strings.Contains(lower, "error") || strings.Contains(lower, "failed") {
			count++
		}
	}
	return count
}
