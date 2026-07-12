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
	"gioui.org/unit"
	"gioui.org/widget"
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
	resultDisplay        historyItemDisplay
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
	for index := range a.macCanvasToolbarLists {
		a.macCanvasToolbarLists[index].List.Axis = layout.Horizontal
		a.macCanvasToolbarLists[index].List.ScrollAnyAxis = true
	}
	spec := desktopThemeSpec(desktopStyleMacOS, a.resolvedThemeMode)
	return a.borderedSurface(gtx, spec.Colors.toolbar, 0, spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutMacCanvasPrimaryTools(gtx, spec, state)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutMacCanvasResultTools(gtx, spec, snap, state)
				}),
			)
		})
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

func (a *App) layoutMacCanvasPrimaryTools(gtx layout.Context, spec desktopThemeTokens, state macCanvasToolbarState) layout.Dimensions {
	if !state.hasCanvasResult {
		return layout.Dimensions{}
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarGroup(gtx, spec, &a.macCanvasToolbarLists[0], "工具", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.panToolButton, uiIconPanTool, "移动", state.currentTool == canvasToolPan, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.maskToolButton, uiIconBrush, "蒙版", state.currentTool == canvasToolMask, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.annotateToolButton, uiIconAnnotate, "标注", state.currentTool == canvasToolAnnotate, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.undoCanvasButton, uiIconUndo, "撤销", false, !state.canUndo)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.redoCanvasButton, uiIconRedo, "重做", false, !state.canRedo)
					}),
				)
			})
		}),
	}

	if state.currentTool == canvasToolMask {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarGroup(gtx, spec, &a.macCanvasToolbarLists[1], "蒙版", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.maskPaintButton, uiIconBrush, "画笔", state.currentBrushMode == canvasBrushPaint, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.maskEraseButton, uiIconDelete, "橡皮", state.currentBrushMode == canvasBrushErase, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.importMaskButton, uiIconSource, "导入", state.hasImportedMask, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.maskBrushSizeDownButton, uiIconChevronLeft, "减小", false, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarNote(gtx, spec, fmt.Sprintf("%d px", state.currentBrushSize))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.maskBrushSizeUpButton, uiIconChevronRight, "增大", false, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.clearMaskButton, uiIconDelete, "清空", false, false)
					}),
				)
			})
		}))
	} else if state.currentTool == canvasToolAnnotate {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.macCanvasToolbarGroup(gtx, spec, &a.macCanvasToolbarLists[1], "标注", func(gtx layout.Context) layout.Dimensions {
				items := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.annotateRectButton, uiIconAnnotate, "矩形", state.annotationKind == canvasAnnotationKindRect, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.annotateArrowButton, uiIconCompare, "箭头", state.annotationKind == canvasAnnotationKindArrow, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.annotateFreehandButton, uiIconEdit, "自由画", state.annotationKind == canvasAnnotationKindFreehand, false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.macCanvasToolbarButton(gtx, spec, &a.annotateTextButton, uiIconList, "文字", state.annotationKind == canvasAnnotationKindText, false)
					}),
				}
				for index, value := range canvasAnnotationColors {
					index := index
					value := value
					items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.toolbarColorButton(gtx, &a.annotateColorButtons[index], value, state.annotationColor == value)
					}))
				}
				items = append(items, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.clearAnnotationsButton, uiIconDelete, "清空", false, false)
				}))
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, items...)
			})
		}))
	}

	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return a.macCanvasToolbarGroup(gtx, spec, &a.macCanvasToolbarLists[2], "视图与变换", func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.resetViewButton, uiIconPanTool, "适合画布", false, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.rotateLeftButton, uiIconRotateLeft, "左转", false, !state.canTransform)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.rotateRightButton, uiIconRotateRight, "右转", false, !state.canTransform)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.flipHorizontalButton, uiIconFlip, "水平", false, !state.canTransform)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.flipVerticalButton, uiIconFlip, "垂直", false, !state.canTransform)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.cropSelectionButton, uiIconCrop, "裁出", false, !state.canTransform || !state.hasSelectedCrop)
				}),
			)
		})
	}))

	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, children...)
}

func (a *App) layoutMacCanvasResultTools(gtx layout.Context, spec desktopThemeTokens, snap snapshot, state macCanvasToolbarState) layout.Dimensions {
	return a.macCanvasToolbarGroup(gtx, spec, &a.macCanvasToolbarLists[3], "结果与操作", func(gtx layout.Context) layout.Dimensions {
		children := make([]layout.FlexChild, 0, 12)
		if state.showLatestJump {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.macCanvasToolbarButton(gtx, spec, &a.latestResultButton, uiIconHistory, "最近结果", false, false)
			}))
		}
		if state.showResultGridToggle {
			label := fmt.Sprintf("网格 %d", len(snap.BatchResults))
			if snap.ResultGridOpen {
				label = "单图"
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.macCanvasToolbarButton(gtx, spec, &a.toggleResultGridButton, uiIconGrid, label, snap.ResultGridOpen, false)
			}))
		}
		if state.canNavigateResults {
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.previousBatchResultButton, uiIconChevronLeft, "上一张", false, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.nextBatchResultButton, uiIconChevronRight, "下一张", false, false)
				}),
			)
		}
		if state.compareActive {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.macCanvasToolbarButton(gtx, spec, &a.closeCompareButton, uiIconCompare, "退出对比", true, false)
			}))
		}
		if snap.Result.HasItem && len(state.resultDisplay.MetaBadges) > 0 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.metaBadgeRow(gtx, state.resultDisplay.MetaBadges, true)
			}))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			icon := uiIconFullscreen
			label := "全屏"
			if snap.Fullscreen {
				icon = uiIconFullscreenExit
				label = "退出全屏"
			}
			return a.macCanvasToolbarButton(gtx, spec, &a.fullscreenButton, icon, label, snap.Fullscreen, false)
		}))
		if snap.Result.HasItem {
			children = append(children,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.resultDetailButton, uiIconInfo, "详情", false, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.clearCurrentButton, uiIconDelete, "移除", false, false)
				}),
			)
			if canDragOutHistoryItem(snap.Result.Item) {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasToolbarButton(gtx, spec, &a.dragOutButton, uiIconLaunch, "拖出", false, false)
				}))
			}
			if canSaveHistoryItem(snap.Result.Item) {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.macCanvasPrimaryButton(gtx, spec, &a.saveAsButton, uiIconDownload, "另存为")
				}))
			}
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, children...)
	})
}

func (a *App) macCanvasToolbarGroup(gtx layout.Context, spec desktopThemeTokens, list *widget.List, caption string, body layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return a.elevatedBorderedSurface(gtx, spec.Colors.panel2, unit.Dp(18), spec.Colors.border, imagePoint(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 7, Bottom: 8, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, caption, unit.Sp(9), spec.Colors.textDim, font.Medium)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					list.List.Axis = layout.Horizontal
					list.List.ScrollAnyAxis = true
					return list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(2)}.Layout(gtx, body)
					})
				}),
			)
		})
	})
}

func (a *App) macCanvasToolbarButton(gtx layout.Context, spec desktopThemeTokens, button *widget.Clickable, icon *widget.Icon, label string, selected bool, disabled bool) layout.Dimensions {
	background := rgba(0xffffff, 0x00)
	hover := spec.Colors.toolHoverBg
	border := rgba(0xffffff, 0x00)
	foreground := spec.Colors.textMuted
	if selected {
		background = spec.Colors.accentSoft
		hover = withAlpha(spec.Colors.accent, 0x28)
		border = withAlpha(spec.Colors.accent, 0x34)
		foreground = spec.Colors.accentText
	}
	if disabled {
		foreground = withAlpha(spec.Colors.textDim, 0x80)
	}
	content := func(gtx layout.Context) layout.Dimensions {
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
					return a.label(gtx, label, unit.Sp(11), foreground, font.Medium)
				}),
			)
		})
	}
	return fixedHeight(gtx, spec.Metrics.ControlHeight, func(gtx layout.Context) layout.Dimensions {
		if !disabled {
			return a.surfaceButton(gtx, button, background, hover, border, spec.Metrics.ControlRadius, layout.Inset{Left: 10, Right: 10}, content, selected)
		}
		macro := op.Record(gtx.Ops)
		dims := a.borderedSurface(gtx, background, spec.Metrics.ControlRadius, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 10, Right: 10}.Layout(gtx, content)
		})
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

// imagePoint keeps the visual helper call sites declarative without exposing
// shadow geometry to the toolbar state model.
func imagePoint(x, y int) image.Point { return image.Pt(x, y) }
