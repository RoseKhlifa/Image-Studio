package ui

import (
	"fmt"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

func (a *App) layoutWorkflowInspector(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	a.handleWorkflowInspectorEvents(gtx)
	data := a.workflowCanvasData(snap, a.activeWorkspaceID)
	node, ok := data.Graph.node(data.Selected)
	if !ok && len(data.Graph.Nodes) > 0 {
		node = data.Graph.Nodes[0]
	}
	return a.borderedSurface(gtx, spec.Colors.inspector, unit.Dp(0), spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return a.workflowInspectorList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			return layout.Inset{Top: 14, Bottom: 18, Left: 14, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(14))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.workflowInspectorHeader(gtx, node, data.Runtime[node.ID], spec)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return workflowDivider(gtx, spec.Colors.border)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.layoutWorkflowNodeInspector(gtx, snap, node, spec)
					}),
				)
			})
		})
	})
}

func (a *App) handleWorkflowInspectorEvents(gtx layout.Context) {
	for index, choice := range modeChoices {
		for a.workflowModeButtons[index].Clicked(gtx) {
			a.mode = choice.Value
			a.invalidateNow()
		}
	}
	for index, choice := range qualityChoices {
		for a.workflowQualityButtons[index].Clicked(gtx) {
			a.quality = choice.Value
			a.invalidateNow()
		}
	}
	for index, choice := range formatChoices {
		for a.workflowFormatButtons[index].Clicked(gtx) {
			a.format = choice.Value
			a.invalidateNow()
		}
	}
	for index, choice := range sizeChoices {
		for a.workflowSizeButtons[index].Clicked(gtx) {
			a.size = choice.Value
			a.invalidateNow()
		}
	}
	for index, choice := range partialPreviewChoices {
		for a.workflowPreviewButtons[index].Clicked(gtx) {
			a.partialImagesInput.SetText(choice.Value)
			a.invalidateNow()
		}
	}
	for a.workflowAddSourcesButton.Clicked(gtx) {
		paths, err := chooseImageFiles()
		if err != nil {
			a.appendLog("选择参考图失败: " + err.Error())
			continue
		}
		current := a.parseSourcePathsCached(a.sourcePathsInput.Text())
		a.setSourcePaths(append(current, paths...))
	}
	for a.workflowClearSourcesButton.Clicked(gtx) {
		a.setSourcePaths(nil)
	}
	for a.workflowOpenOutputButton.Clicked(gtx) {
		if err := openPath(a.outputDirInput.Text()); err != nil {
			a.appendLog("打开输出目录失败: " + err.Error())
		}
	}
	for a.optimizePromptButton.Clicked(gtx) {
		a.startPromptOptimize()
	}
}

func (a *App) workflowInspectorHeader(gtx layout.Context, node workflowNodeModel, runtimeState workflowNodeRuntime, spec desktopThemeTokens) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(9), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(9), func(gtx layout.Context) layout.Dimensions {
							return a.surface(gtx, workflowNodePhaseColor(spec, runtimeState.Phase), unit.Dp(5), layout.Spacer{}.Layout)
						})
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.singleLineLabel(gtx, chooseNonEmpty(node.Title, "节点属性"), unit.Sp(13), spec.Colors.text, font.SemiBold)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, chooseNonEmpty(runtimeState.Detail, node.Subtitle), unit.Sp(9), spec.Colors.textMuted, font.Normal)
		}),
	)
}

func (a *App) layoutWorkflowNodeInspector(gtx layout.Context, snap snapshot, node workflowNodeModel, spec desktopThemeTokens) layout.Dimensions {
	switch node.Kind {
	case workflowNodePrompt:
		return a.layoutWorkflowPromptInspector(gtx, snap, spec)
	case workflowNodeSource:
		return a.layoutWorkflowSourceInspector(gtx, spec)
	case workflowNodeGenerate:
		return a.layoutWorkflowGenerateInspector(gtx, spec)
	case workflowNodePreview:
		return a.layoutWorkflowPreviewInspector(gtx, snap, spec)
	case workflowNodeExport:
		return a.layoutWorkflowExportInspector(gtx, snap, spec)
	default:
		return a.label(gtx, "该节点没有可编辑属性。", unit.Sp(10), spec.Colors.textDim, font.Normal)
	}
}

func (a *App) layoutWorkflowPromptInspector(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	border := spec.Colors.border2
	if gtx.Focused(&a.promptInput) {
		border = spec.Colors.focusRing
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorSectionLabel(gtx, "提示词", fmt.Sprintf("%d 字符", len([]rune(a.promptInput.Text()))), spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, unit.Dp(166), func(gtx layout.Context) layout.Dimensions {
				return a.borderedSurface(gtx, spec.Colors.surface, spec.Metrics.InputRadius, border, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 10, Bottom: 10, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.editorText(gtx, &a.promptInput, "描述主体、场景、镜头、光线与风格", unit.Sp(12))
					})
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := "AI 优化"
			if snap.OptimizingPrompt {
				label = "优化中"
			}
			return a.compactIconTextButton(gtx, &a.optimizePromptButton, uiIconSpark, label, snap.OptimizingPrompt)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.technicalField(gtx, "负面提示", &a.negativePromptInput, "可选：排除不需要的元素", unit.Dp(72))
		}),
	)
}

func (a *App) layoutWorkflowSourceInspector(gtx layout.Context, spec desktopThemeTokens) layout.Dimensions {
	sources := a.parseSourcePathsCached(a.sourcePathsInput.Text())
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorSectionLabel(gtx, "图像输入", fmt.Sprintf("%d 个", len(sources)), spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.workflowAddSourcesButton, uiIconAdd, "添加参考图", false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.workflowClearSourcesButton, uiIconClear, "清空", false)
				}),
			)
		}),
	}
	if len(sources) == 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, "当前没有参考图；文生图工作流会直接跳过该输入。", unit.Sp(10), spec.Colors.textDim, font.Normal)
		}))
	} else {
		limit := min(5, len(sources))
		for index := 0; index < limit; index++ {
			path := sources[index]
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.borderedSurface(gtx, spec.Colors.surface, spec.Metrics.ControlRadius, spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 8, Bottom: 8, Left: 9, Right: 9}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, path, unit.Sp(9), spec.Colors.textMuted, font.Normal)
					})
				})
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, children...)
}

func (a *App) layoutWorkflowGenerateInspector(gtx layout.Context, spec desktopThemeTokens) layout.Dimensions {
	commonSizes := []int{0, 3, 4, 5}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowChoiceSection(gtx, "模式", modeChoices, a.workflowModeButtons, a.mode, nil, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return workflowDivider(gtx, spec.Colors.border)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowChoiceSection(gtx, "质量", qualityChoices, a.workflowQualityButtons, a.quality, nil, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return workflowDivider(gtx, spec.Colors.border)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowChoiceSection(gtx, "常用尺寸", sizeChoices, a.workflowSizeButtons, a.size, commonSizes, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorKeyValue(gtx, "模型", chooseNonEmpty(a.imageModelInput.Text(), "未配置"), spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorKeyValue(gtx, "批量", fmt.Sprintf("%d 张", a.batchCount), spec)
		}),
	)
}

func (a *App) layoutWorkflowPreviewInspector(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowChoiceSection(gtx, "流式预览", partialPreviewChoices, a.workflowPreviewButtons, strings.TrimSpace(a.partialImagesInput.Text()), nil, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return workflowDivider(gtx, spec.Colors.border)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorKeyValue(gtx, "状态", snap.Status, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorKeyValue(gtx, "已完成", fmt.Sprintf("%d / %d", len(snap.BatchResults), max(snap.BatchTotal, 1)), spec)
		}),
	)
}

func (a *App) layoutWorkflowExportInspector(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	path := strings.TrimSpace(a.outputDirInput.Text())
	if strings.TrimSpace(snap.Result.SavedPath) != "" {
		path = snap.Result.SavedPath
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowChoiceSection(gtx, "输出格式", formatChoices, a.workflowFormatButtons, a.format, nil, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return workflowDivider(gtx, spec.Colors.border)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorSectionLabel(gtx, "输出位置", "", spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.borderedSurface(gtx, spec.Colors.surface, spec.Metrics.ControlRadius, spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 9, Bottom: 9, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, chooseNonEmpty(path, "未设置"), unit.Sp(9), spec.Colors.textMuted, font.Normal)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.compactIconTextButton(gtx, &a.workflowOpenOutputButton, uiIconFolder, "打开输出目录", false)
		}),
	)
}

func (a *App) workflowChoiceSection(gtx layout.Context, title string, choices []choice, buttons []widget.Clickable, selected string, indexes []int, spec desktopThemeTokens) layout.Dimensions {
	if len(indexes) == 0 {
		indexes = make([]int, len(choices))
		for index := range choices {
			indexes[index] = index
		}
	}
	rows := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorSectionLabel(gtx, title, "", spec)
		}),
	}
	for start := 0; start < len(indexes); start += 2 {
		end := min(start+2, len(indexes))
		rowIndexes := append([]int(nil), indexes[start:end]...)
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(rowIndexes))
			for _, index := range rowIndexes {
				index := index
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					isSelected := selected == choices[index].Value
					return a.compactButton(gtx, &buttons[index], choices[index].Label, isSelected, isSelected)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, children...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, rows...)
}

func (a *App) workflowInspectorSectionLabel(gtx layout.Context, label string, value string, spec desktopThemeTokens) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, label, unit.Sp(9), spec.Colors.textMuted, font.SemiBold)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.monoLabel(gtx, value, unit.Sp(9), spec.Colors.textDim, font.Normal)
		}),
	)
}

func (a *App) workflowInspectorKeyValue(gtx layout.Context, label string, value string, spec desktopThemeTokens) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(72), func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, label, unit.Sp(9), spec.Colors.textDim, font.Normal)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, chooseNonEmpty(value, "-"), unit.Sp(9), spec.Colors.textMuted, font.Medium)
		}),
	)
}

func partialPreviewCount(value string) int {
	count, _ := strconv.Atoi(strings.TrimSpace(value))
	return count
}
