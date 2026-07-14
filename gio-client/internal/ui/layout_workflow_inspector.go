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
	data := a.workflowCanvasData(snap, a.activeWorkspaceID)
	node, ok := data.Graph.node(data.Selected)
	if !ok && len(data.Graph.Nodes) > 0 {
		node = data.Graph.Nodes[0]
	}
	a.ensureWorkflowNodeControlsLoaded(a.activeWorkspaceID, node)
	if a.syncWorkflowNodeControls(a.activeWorkspaceID, node, false) {
		data = a.workflowCanvasData(snap, a.activeWorkspaceID)
		node, _ = data.Graph.node(data.Selected)
	}
	a.handleWorkflowInspectorEvents(gtx, node)
	data = a.workflowCanvasData(snap, a.activeWorkspaceID)
	node, _ = data.Graph.node(data.Selected)
	if a.handleWorkflowConnectionEvents(gtx, data.Graph, node) {
		data = a.workflowCanvasData(snap, a.activeWorkspaceID)
		node, _ = data.Graph.node(data.Selected)
	}
	return a.borderedSurface(gtx, spec.Colors.inspector, unit.Dp(0), spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return a.workflowInspectorList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
			return layout.Inset{Top: 14, Bottom: 18, Left: 14, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.workflowInspectorHeader(gtx, node, data.Runtime[node.ID], spec)
					}),
				}
				if spec.Style == desktopStyleMacOS {
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.workflowInspectorSectionCard(gtx, spec, func(gtx layout.Context) layout.Dimensions {
								return a.layoutWorkflowInspectorContent(gtx, snap, data.Graph, node, spec)
							})
						}),
					)
				} else {
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return workflowDivider(gtx, spec.Colors.border)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.layoutWorkflowInspectorContent(gtx, snap, data.Graph, node, spec)
						}),
					)
				}
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(14))}.Layout(gtx, children...)
			})
		})
	})
}

func (a *App) layoutWorkflowInspectorContent(gtx layout.Context, snap snapshot, graph workflowGraphModel, node workflowNodeModel, spec desktopThemeTokens) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.technicalField(gtx, "节点名称", &a.workflowNodeTitleInput, "输入节点名称", unit.Dp(42))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutWorkflowNodeInspector(gtx, snap, node, spec)
		}),
	}
	if len(node.Inputs) > 0 {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return workflowDivider(gtx, spec.Colors.border)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutWorkflowConnections(gtx, graph, node, spec)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx, children...)
}

func (a *App) handleWorkflowConnectionEvents(gtx layout.Context, graph workflowGraphModel, node workflowNodeModel) bool {
	for _, input := range node.Inputs {
		for _, candidate := range workflowCompatibleOutputs(graph, node.ID, input.ID) {
			button := a.workflowConnectionButton(a.activeWorkspaceID, candidate)
			for button.Clicked(gtx) {
				if err := a.toggleWorkflowConnection(a.activeWorkspaceID, candidate); err != nil {
					a.appendLog("连接节点失败: " + err.Error())
					return false
				}
				return true
			}
		}
	}
	return false
}

func (a *App) layoutWorkflowConnections(gtx layout.Context, graph workflowGraphModel, node workflowNodeModel, spec desktopThemeTokens) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorSectionLabel(gtx, "输入连接", fmt.Sprintf("%d 个端口", len(node.Inputs)), spec)
		}),
	}
	for _, input := range node.Inputs {
		input := input
		connected := workflowInputEdges(graph, node.ID, input.ID)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			status := "未连接"
			if len(connected) > 0 {
				status = fmt.Sprintf("已连接 %d", len(connected))
			}
			return a.workflowInspectorSectionLabel(gtx, input.Name, status, spec)
		}))
		for _, candidate := range workflowCompatibleOutputs(graph, node.ID, input.ID) {
			candidate := candidate
			source, _ := graph.node(candidate.FromNode)
			output, _ := workflowOutputPort(source, candidate.FromPort)
			label := source.Title
			if len(source.Outputs) > 1 {
				label += " · " + output.Name
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				selected := workflowEdgeConnected(graph, candidate)
				return a.workflowChoiceButton(gtx, spec, a.workflowConnectionButton(a.activeWorkspaceID, candidate), label, selected)
			}))
		}
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, children...)
}

func (a *App) workflowConnectionButton(workspaceID string, edge workflowEdgeModel) *widget.Clickable {
	if a.workflowConnectionButtons == nil {
		a.workflowConnectionButtons = map[string]*widget.Clickable{}
	}
	key := strings.TrimSpace(workspaceID) + "|" + workflowEdgeID(edge)
	if button := a.workflowConnectionButtons[key]; button != nil {
		return button
	}
	button := new(widget.Clickable)
	a.workflowConnectionButtons[key] = button
	return button
}

func (a *App) workflowInspectorSectionCard(gtx layout.Context, spec desktopThemeTokens, body layout.Widget) layout.Dimensions {
	return a.borderedSurface(gtx, spec.Colors.surfaceElevated, workflowSectionRadius(spec), spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(14)).Layout(gtx, body)
	})
}

func (a *App) handleWorkflowInspectorEvents(gtx layout.Context, node workflowNodeModel) {
	recordNodeChange := false
	for a.workflowDeleteNodeButton.Clicked(gtx) {
		a.deleteSelectedWorkflowNode(a.activeWorkspaceID)
	}
	for a.workflowDuplicateNodeButton.Clicked(gtx) {
		a.duplicateSelectedWorkflowNode(a.activeWorkspaceID)
	}
	for a.workflowToggleNodeButton.Clicked(gtx) {
		a.toggleSelectedWorkflowNodeEnabled(a.activeWorkspaceID)
	}
	for index, choice := range modeChoices {
		for a.workflowModeButtons[index].Clicked(gtx) {
			a.mode = choice.Value
			recordNodeChange = true
		}
	}
	for index, choice := range qualityChoices {
		for a.workflowQualityButtons[index].Clicked(gtx) {
			a.quality = choice.Value
			recordNodeChange = true
		}
	}
	for index, choice := range formatChoices {
		for a.workflowFormatButtons[index].Clicked(gtx) {
			a.format = choice.Value
			recordNodeChange = true
		}
	}
	for index, choice := range sizeChoices {
		for a.workflowSizeButtons[index].Clicked(gtx) {
			a.size = choice.Value
			recordNodeChange = true
		}
	}
	for index, choice := range batchCountChoices {
		for a.workflowBatchCountButtons[index].Clicked(gtx) {
			count, err := strconv.Atoi(choice.Value)
			if err == nil {
				a.batchCount = normalizeBatchCount(count)
				recordNodeChange = true
			}
		}
	}
	for index, choice := range partialPreviewChoices {
		for a.workflowPreviewButtons[index].Clicked(gtx) {
			a.partialImagesInput.SetText(choice.Value)
			recordNodeChange = true
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
		recordNodeChange = true
	}
	for a.workflowClearSourcesButton.Clicked(gtx) {
		a.setSourcePaths(nil)
		recordNodeChange = true
	}
	for a.workflowOpenOutputButton.Clicked(gtx) {
		if err := openPath(a.outputDirInput.Text()); err != nil {
			a.appendLog("打开输出目录失败: " + err.Error())
		}
	}
	for a.optimizePromptButton.Clicked(gtx) {
		a.startPromptOptimize()
	}
	if recordNodeChange {
		graph := a.workflowGraph(a.activeWorkspaceID)
		current, ok := graph.node(node.ID)
		if ok {
			a.syncWorkflowNodeControls(a.activeWorkspaceID, current, true)
		}
	}
}

func (a *App) workflowInspectorHeader(gtx layout.Context, node workflowNodeModel, runtimeState workflowNodeRuntime, spec desktopThemeTokens) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
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
			}
			if node.ID != "" {
				toggleIcon := uiIconVisibilityOff
				toggleLabel := "停用" + node.Title
				if !node.Enabled {
					toggleIcon = uiIconVisibility
					toggleLabel = "启用" + node.Title
				}
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.headerIconButtonIcon(gtx, &a.workflowToggleNodeButton, toggleIcon, !node.Enabled, toggleLabel)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.headerIconButtonIcon(gtx, &a.workflowDuplicateNodeButton, uiIconCopy, false, "复制"+node.Title)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.headerIconButtonIcon(gtx, &a.workflowDeleteNodeButton, uiIconDelete, false, "删除"+node.Title)
					}),
				)
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, children...)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, chooseNonEmpty(runtimeState.Detail, node.Subtitle), workflowTextSize(spec, 11, 9), spec.Colors.textMuted, font.Normal)
		}),
	}
	if typeID := workflowNodeTypeID(node); typeID != string(node.Kind) {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			metadata := typeID
			if version := strings.TrimSpace(node.TypeVersion); version != "" {
				metadata += " · v" + version
			}
			if category := strings.TrimSpace(node.Category); category != "" {
				metadata = category + " · " + metadata
			}
			return a.singleLineLabel(gtx, metadata, workflowTextSize(spec, 10, 9), spec.Colors.textDim, font.Normal)
		}))
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx, children...)
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
		return a.label(gtx, "该节点没有可编辑属性。", workflowTextSize(spec, 11, 10), spec.Colors.textDim, font.Normal)
	}
}

func (a *App) layoutWorkflowPromptInspector(gtx layout.Context, snap snapshot, spec desktopThemeTokens) layout.Dimensions {
	promptMetrics := resolveWorkflowPromptEditorMetrics(spec)
	border := spec.Colors.border2
	if gtx.Focused(&a.promptInput) {
		border = spec.Colors.focusRing
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorSectionLabel(gtx, "提示词", fmt.Sprintf("%d 字符", len([]rune(a.promptInput.Text()))), spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, promptMetrics.Height, func(gtx layout.Context) layout.Dimensions {
				return a.borderedSurface(gtx, spec.Colors.surface, promptMetrics.Radius, border, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 10, Bottom: 10, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.editorText(gtx, &a.promptInput, "描述主体、场景、镜头、光线与风格", workflowTextSize(spec, 13, 12))
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
			return a.label(gtx, "当前没有参考图；文生图工作流会直接跳过该输入。", workflowTextSize(spec, 11, 10), spec.Colors.textDim, font.Normal)
		}))
	} else {
		limit := min(5, len(sources))
		for index := 0; index < limit; index++ {
			path := sources[index]
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.borderedSurface(gtx, spec.Colors.surface, spec.Metrics.ControlRadius, spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 8, Bottom: 8, Left: 9, Right: 9}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, path, workflowTextSize(spec, 11, 9), spec.Colors.textMuted, font.Normal)
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
			return a.technicalField(gtx, "图像模型", &a.imageModelInput, "gpt-image-2", unit.Dp(42))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowChoiceSection(gtx, "生成张数", batchCountChoices, a.workflowBatchCountButtons, strconv.Itoa(a.batchCount), nil, spec)
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
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowChoiceSection(gtx, "输出格式", formatChoices, a.workflowFormatButtons, a.format, nil, spec)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return workflowDivider(gtx, spec.Colors.border)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.technicalField(gtx, "输出目录", &a.outputDirInput, "选择输出目录", unit.Dp(42))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.compactIconTextButton(gtx, &a.workflowOpenOutputButton, uiIconFolder, "打开输出目录", false)
		}),
	}
	if strings.TrimSpace(snap.Result.SavedPath) != "" {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.workflowInspectorKeyValue(gtx, "最近结果", snap.Result.SavedPath, spec)
		}))
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx, children...)
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
					return a.workflowChoiceButton(gtx, spec, &buttons[index], choices[index].Label, isSelected)
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
			return a.label(gtx, label, workflowTextSize(spec, 11, 9), spec.Colors.textMuted, font.SemiBold)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.monoLabel(gtx, value, workflowTextSize(spec, 11, 9), spec.Colors.textDim, font.Normal)
		}),
	)
}

func (a *App) workflowInspectorKeyValue(gtx layout.Context, label string, value string, spec desktopThemeTokens) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(72), func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, label, workflowTextSize(spec, 11, 9), spec.Colors.textDim, font.Normal)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, chooseNonEmpty(value, "-"), workflowTextSize(spec, 11, 9), spec.Colors.textMuted, font.Medium)
		}),
	)
}

func (a *App) workflowChoiceButton(gtx layout.Context, spec desktopThemeTokens, button *widget.Clickable, label string, selected bool) layout.Dimensions {
	if spec.Style != desktopStyleMacOS {
		return a.compactButton(gtx, button, label, selected, selected)
	}
	background := spec.Colors.surface
	hoverBackground := spec.Colors.surface2
	foreground := spec.Colors.textMuted
	border := spec.Colors.border
	if selected {
		background = spec.Colors.accentSoft
		hoverBackground = withAlpha(spec.Colors.accent, 0x28)
		foreground = spec.Colors.accentText
		border = withAlpha(spec.Colors.accent, 0x30)
	}
	textSize := unit.Sp(12)
	height := minimumTextControlHeight(gtx, spec.Metrics.ControlHeight, a.scaledSp(textSize), unit.Dp(8))
	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		return a.surfaceButton(gtx, button, background, hoverBackground, border, unit.Dp(14), layout.Inset{Left: 8, Right: 8}, func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.singleLineLabel(gtx, label, textSize, foreground, chooseFontWeight(selected))
			})
		}, selected)
	})
}

type workflowPromptEditorMetrics struct {
	Height unit.Dp
	Radius unit.Dp
}

func resolveWorkflowPromptEditorMetrics(spec desktopThemeTokens) workflowPromptEditorMetrics {
	if spec.Style == desktopStyleMacOS {
		return workflowPromptEditorMetrics{Height: unit.Dp(176), Radius: unit.Dp(18)}
	}
	return workflowPromptEditorMetrics{Height: unit.Dp(166), Radius: spec.Metrics.InputRadius}
}

func partialPreviewCount(value string) int {
	count, _ := strconv.Atoi(strings.TrimSpace(value))
	return count
}
