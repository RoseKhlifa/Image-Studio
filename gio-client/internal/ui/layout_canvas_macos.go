package ui

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

type macCanvasToolbarState struct {
	hasCanvasResult      bool
	showResultGridToggle bool
	canNavigateResults   bool
	canUndo              bool
	canRedo              bool
	canTransform         bool
	hasSelectedCrop      bool
	hasImportedMask      bool
	compareActive        bool
	showLatestJump       bool
	currentTool          canvasToolMode
	currentBrushMode     canvasBrushMode
	currentBrushSize     int
	annotationKind       canvasAnnotationKind
	annotationColor      color.NRGBA
}

func showMacCanvasToolbar(hasCanvasResult bool, batchResultCount int) bool {
	return hasCanvasResult || batchResultCount > 1
}

func showMacCanvasStatus(running bool, processingTransform bool, hasResult bool) bool {
	return running || processingTransform || hasResult
}

type macCanvasStatusVisibility struct {
	show         bool
	showMetadata bool
	showZoom     bool
}

func macCanvasStatusVisibilityFor(running bool, processingTransform bool, hasResult bool) macCanvasStatusVisibility {
	return macCanvasStatusVisibility{
		show:         showMacCanvasStatus(running, processingTransform, hasResult),
		showMetadata: hasResult && !running && !processingTransform,
		showZoom:     hasResult && !running && !processingTransform,
	}
}

func (a *App) layoutMacCanvasToolbar(gtx layout.Context, snap snapshot, state macCanvasToolbarState) layout.Dimensions {
	if !showMacCanvasToolbar(state.hasCanvasResult, len(snap.BatchResults)) {
		return layout.Dimensions{}
	}
	a.macCanvasToolbarList.List.Axis = layout.Horizontal
	a.macCanvasToolbarList.List.ScrollAnyAxis = true
	spec := desktopThemeSpec(desktopStyleMacOS, a.resolvedThemeMode)
	return a.borderedSurface(gtx, spec.Colors.toolbar, 0, spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				return a.layoutMacCanvasToolbarRow(gtx, spec, snap, state)
			})
		})
	})
}

func (a *App) layoutMacCanvasToolbarRow(gtx layout.Context, spec desktopThemeTokens, snap snapshot, state macCanvasToolbarState) layout.Dimensions {
	groups := make([]layout.FlexChild, 0, 5)
	if state.hasCanvasResult {
		groups = append(groups,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutMacCanvasLeadingGroup(gtx, spec, state)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.macCanvasToolbarDivider(gtx, spec)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutMacCanvasContextGroup(gtx, spec, state)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.macCanvasToolbarDivider(gtx, spec)
			}),
		)
	}
	groups = append(groups,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutMacCanvasResultGroup(gtx, spec, snap, state)
		}),
	)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, groups...)
}

func (a *App) layoutMacCanvasLeadingGroup(gtx layout.Context, spec desktopThemeTokens, state macCanvasToolbarState) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarIconButton(gtx, spec, &a.panToolButton, uiIconPanTool, "移动", state.currentTool == canvasToolPan, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarIconButton(gtx, spec, &a.maskToolButton, uiIconBrush, "蒙版", state.currentTool == canvasToolMask, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarIconButton(gtx, spec, &a.annotateToolButton, uiIconAnnotate, "标注", state.currentTool == canvasToolAnnotate, false)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarIconButton(gtx, spec, &a.undoCanvasButton, uiIconUndo, "撤销", false, !state.canUndo)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarIconButton(gtx, spec, &a.redoCanvasButton, uiIconRedo, "重做", false, !state.canRedo)
		}),
	)
}

func (a *App) layoutMacCanvasContextGroup(gtx layout.Context, spec desktopThemeTokens, state macCanvasToolbarState) layout.Dimensions {
	items := make([]layout.FlexChild, 0, 10)
	add := func(button *widget.Clickable, icon *widget.Icon, label string, selected bool, disabled bool) {
		items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarIconButton(gtx, spec, button, icon, label, selected, disabled)
		}))
	}
	switch state.currentTool {
	case canvasToolMask:
		add(&a.maskPaintButton, uiIconBrush, "画笔", state.currentBrushMode == canvasBrushPaint, false)
		add(&a.maskEraseButton, uiIconDelete, "橡皮", state.currentBrushMode == canvasBrushErase, false)
		add(&a.importMaskButton, uiIconSource, "导入蒙版", state.hasImportedMask, false)
		add(&a.maskBrushSizeDownButton, uiIconChevronLeft, "减小笔刷", false, false)
		items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarNote(gtx, spec, fmt.Sprintf("%d", state.currentBrushSize))
		}))
		add(&a.maskBrushSizeUpButton, uiIconChevronRight, "增大笔刷", false, false)
		add(&a.clearMaskButton, uiIconClear, "清空蒙版", false, false)
	case canvasToolAnnotate:
		add(&a.annotateRectButton, uiIconAnnotate, "矩形", state.annotationKind == canvasAnnotationKindRect, false)
		add(&a.annotateArrowButton, uiIconCompare, "箭头", state.annotationKind == canvasAnnotationKindArrow, false)
		add(&a.annotateFreehandButton, uiIconEdit, "自由画", state.annotationKind == canvasAnnotationKindFreehand, false)
		add(&a.annotateTextButton, uiIconList, "文字", state.annotationKind == canvasAnnotationKindText, false)
		for index, value := range canvasAnnotationColors {
			index := index
			value := value
			items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.toolbarColorButton(gtx, &a.annotateColorButtons[index], value, state.annotationColor == value)
			}))
		}
		add(&a.clearAnnotationsButton, uiIconClear, "清空标注", false, false)
	default:
		add(&a.resetViewButton, uiIconFit, "适合画布", false, false)
		add(&a.rotateLeftButton, uiIconRotateLeft, "向左旋转", false, !state.canTransform)
		add(&a.rotateRightButton, uiIconRotateRight, "向右旋转", false, !state.canTransform)
		add(&a.flipHorizontalButton, uiIconFlip, "水平翻转", false, !state.canTransform)
		add(&a.flipVerticalButton, uiIconFlip, "垂直翻转", false, !state.canTransform)
		add(&a.cropSelectionButton, uiIconCrop, "裁剪", false, !state.canTransform || !state.hasSelectedCrop)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx, items...)
}

func (a *App) layoutMacCanvasResultGroup(gtx layout.Context, spec desktopThemeTokens, snap snapshot, state macCanvasToolbarState) layout.Dimensions {
	items := make([]layout.FlexChild, 0, 10)
	add := func(button *widget.Clickable, icon *widget.Icon, label string, selected bool) {
		items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarIconButton(gtx, spec, button, icon, label, selected, false)
		}))
	}
	if state.showLatestJump {
		add(&a.latestResultButton, uiIconHistory, "最近结果", false)
	}
	if state.showResultGridToggle {
		add(&a.toggleResultGridButton, uiIconGrid, "切换结果视图", snap.ResultGridOpen)
	}
	if state.canNavigateResults {
		add(&a.previousBatchResultButton, uiIconChevronLeft, "上一张", false)
		add(&a.nextBatchResultButton, uiIconChevronRight, "下一张", false)
	}
	if state.compareActive {
		add(&a.closeCompareButton, uiIconCompare, "退出对比", true)
	}
	fullscreenIcon := uiIconFullscreen
	fullscreenLabel := "全屏"
	if snap.Fullscreen {
		fullscreenIcon = uiIconFullscreenExit
		fullscreenLabel = "退出全屏"
	}
	add(&a.fullscreenButton, fullscreenIcon, fullscreenLabel, snap.Fullscreen)
	if snap.Result.HasItem {
		add(&a.macCanvasMoreButton, uiIconMoreHoriz, "更多操作", false)
		if canSaveHistoryItem(snap.Result.Item) {
			items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.macCanvasPrimaryButton(gtx, spec, &a.saveAsButton, uiIconDownload, "另存为")
			}))
		}
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx, items...)
}

func (a *App) macCanvasToolbarDivider(gtx layout.Context, spec desktopThemeTokens) layout.Dimensions {
	return fixedWidth(gtx, unit.Dp(1), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(20), func(gtx layout.Context) layout.Dimensions {
			return a.surface(gtx, spec.Colors.border, 0, layout.Spacer{}.Layout)
		})
	})
}

func (a *App) macCanvasToolbarIconButton(gtx layout.Context, spec desktopThemeTokens, button *widget.Clickable, icon *widget.Icon, label string, selected bool, disabled bool) layout.Dimensions {
	background := rgba(0xffffff, 0x00)
	hover := spec.Colors.toolHoverBg
	foreground := spec.Colors.textMuted
	if selected {
		background = spec.Colors.accentSoft
		hover = spec.Colors.accentSoft
		foreground = spec.Colors.accentText
	}
	if disabled {
		foreground = withAlpha(spec.Colors.textDim, 0x80)
	}
	content := func(gtx layout.Context) layout.Dimensions {
		semantic.LabelOp(label).Add(gtx.Ops)
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
					return icon.Layout(gtx, foreground)
				})
			})
		})
	}
	return fixedWidth(gtx, spec.Metrics.IconTargetSize, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, spec.Metrics.IconTargetSize, func(gtx layout.Context) layout.Dimensions {
			if !disabled {
				return a.surfaceButton(gtx, button, background, hover, rgba(0xffffff, 0x00), spec.Metrics.ControlRadius, layout.Inset{}, content, selected)
			}
			macro := op.Record(gtx.Ops)
			dims := a.surface(gtx, background, spec.Metrics.ControlRadius, content)
			call := macro.Stop()
			semanticArea := clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops)
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp(label).Add(gtx.Ops)
			semantic.SelectedOp(selected).Add(gtx.Ops)
			semantic.EnabledOp(false).Add(gtx.Ops)
			call.Add(gtx.Ops)
			semanticArea.Pop()
			return dims
		})
	})
}

func (a *App) layoutMacCanvasEmptyState(gtx layout.Context) layout.Dimensions {
	spec := desktopThemeSpec(desktopStyleMacOS, a.resolvedThemeMode)
	return fixedWidth(gtx, unit.Dp(280), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, unit.Dp(52), func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, unit.Dp(52), func(gtx layout.Context) layout.Dimensions {
						return a.surface(gtx, spec.Colors.surface2, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(26), func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, unit.Dp(26), func(gtx layout.Context) layout.Dimensions {
										return uiIconPhoto.Layout(gtx, spec.Colors.textDim)
									})
								})
							})
						})
					})
				})
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, "开始创作", unit.Sp(16), spec.Colors.text, font.SemiBold)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				style := material.Label(a.th, a.scaledSp(unit.Sp(11)), "选择现有图像继续编辑，或在生成设置中直接创作。")
				style.Alignment = text.Middle
				style.Color = spec.Colors.textDim
				style.WrapPolicy = textWrapWords
				return style.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.macCanvasPrimaryButton(gtx, spec, &a.emptyStateImportButton, uiIconSource, "选择图像")
			}),
		)
	})
}

func (a *App) layoutMacCanvasStatusBar(gtx layout.Context, snap snapshot) layout.Dimensions {
	visibility := macCanvasStatusVisibilityFor(snap.Running, snap.ProcessingImageTransform, snap.Result.HasItem)
	if !visibility.show {
		return layout.Dimensions{}
	}
	spec := desktopThemeSpec(desktopStyleMacOS, a.resolvedThemeMode)
	return a.borderedSurface(gtx, spec.Colors.toolbar, 0, spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 7, Bottom: 7, Left: 14, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if snap.Running || snap.ProcessingImageTransform {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
										return uiIconRefresh.Layout(gtx, spec.Colors.accent)
									})
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, chooseStatusText(snap.Status), unit.Sp(11), spec.Colors.text, font.Medium)
								})
							}),
							layout.Flexed(1, layout.Spacer{}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if snap.BatchTotal <= 1 {
									return layout.Dimensions{}
								}
								return a.singleLineLabel(gtx, fmt.Sprintf("已完成 %d/%d", len(snap.BatchResults), snap.BatchTotal), unit.Sp(11), spec.Colors.textDim, font.Normal)
							}),
						)
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.S.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.layoutRunningStatusProgressBar(gtx)
						})
					}),
				)
			}

			display := a.historyItemDisplay(snap.Result.Item)
			headline := "生成结果"
			if snap.Result.Item.Mode == "edit" {
				headline = "编辑结果"
			}
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return uiIconCheck.Layout(gtx, spec.Colors.accent)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, headline, unit.Sp(11), spec.Colors.accentText, font.Medium)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !visibility.showMetadata || len(display.StatusMetaBadges) == 0 {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.metaBadgeRow(gtx, display.StatusMetaBadges, true)
					})
				}),
				layout.Flexed(1, layout.Spacer{}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					zoomLabel := formatCanvasScaleLabel(a.canvasDisplayScale)
					if !visibility.showZoom || zoomLabel == "" {
						return layout.Dimensions{}
					}
					return a.singleLineLabel(gtx, zoomLabel, unit.Sp(11), spec.Colors.textDim, font.Normal)
				}),
			)
		})
	})
}

func (a *App) macCanvasPrimaryButton(gtx layout.Context, spec desktopThemeTokens, button *widget.Clickable, icon *widget.Icon, label string) layout.Dimensions {
	foreground := desktopReadableText(spec.Colors.accent)
	return fixedHeight(gtx, spec.Metrics.ControlHeight, func(gtx layout.Context) layout.Dimensions {
		return a.surfaceButton(gtx, button, spec.Colors.accent, spec.Colors.accent2, withAlpha(spec.Colors.accent, 0x70), unit.Dp(14), layout.Inset{Left: 12, Right: 12}, func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(label).Add(gtx.Ops)
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
						return a.label(gtx, label, unit.Sp(11), foreground, font.SemiBold)
					}),
				)
			})
		})
	})
}

func (a *App) macCanvasToolbarNote(gtx layout.Context, spec desktopThemeTokens, text string) layout.Dimensions {
	return fixedHeight(gtx, spec.Metrics.ControlHeight, func(gtx layout.Context) layout.Dimensions {
		return a.borderedSurface(gtx, withAlpha(spec.Colors.surface2, 0xb0), spec.Metrics.ControlRadius, spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, text, unit.Sp(11), spec.Colors.textMuted, font.Medium)
				})
			})
		})
	})
}
