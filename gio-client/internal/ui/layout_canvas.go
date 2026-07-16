package ui

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sharedCompat "image-studio/shared/compat"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/yuanhua/image-gptcodex/pkg/client"
)

type checkerboardCache struct {
	size   image.Point
	tile   int
	first  color.NRGBA
	second color.NRGBA
	img    *image.RGBA
	op     paint.ImageOp
}

func canTransformCurrentResult(snap snapshot) bool {
	if !snap.Result.HasItem {
		return false
	}
	return strings.TrimSpace(snap.Result.SavedPath) != ""
}

func (a *App) layoutCanvas(gtx layout.Context, snap snapshot) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingCanvas, time.Now())
	sourcePaths := a.sourcePaths()
	showSourceStrip := a.mode == string(client.ModeEdit) && len(sourcePaths) > 0

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.canvasToolbar(gtx, snap)
		}),
	}
	if showSourceStrip {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, unit.Dp(64), func(gtx layout.Context) layout.Dimensions {
				return a.sourceStrip(gtx, snap, sourcePaths)
			})
		}))
	}
	children = append(children,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.resultSurface(gtx, snap)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.canvasStatusBar(gtx, snap)
		}),
	)

	return a.borderedSurface(gtx, fluent.panel2, unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func (a *App) canvasToolbar(gtx layout.Context, snap snapshot) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingCanvasToolbar, time.Now())
	latestItem, hasLatest := newestHistoryItem(snap.History)
	canTransformCurrent := canTransformCurrentResult(snap)
	for a.closeCompareButton.Clicked(gtx) {
		a.clearCompare()
	}
	if a.saveAsButton.Clicked(gtx) {
		a.openSavePromptForCurrent()
	}
	if a.dragOutButton.Clicked(gtx) {
		if _, err := a.dragOutHistoryItem(snap.Result.Item); err != nil {
			a.appendLog("拖出复制失败: " + err.Error())
		}
	}
	if a.latestResultButton.Clicked(gtx) {
		if hasLatest {
			if err := a.loadHistoryPreview(latestItem, true); err != nil && !isMissingPreview(err) {
				a.appendLog("载入当前图失败: " + err.Error())
			}
		}
	}
	for a.closeResultGridButton.Clicked(gtx) {
		a.closeResultGrid()
	}
	for a.toggleResultGridButton.Clicked(gtx) {
		if snap.ResultGridOpen {
			a.closeResultGrid()
		} else {
			a.openResultGrid()
		}
	}
	for a.panToolButton.Clicked(gtx) {
		a.setCanvasTool(canvasToolPan)
	}
	for a.maskToolButton.Clicked(gtx) {
		a.setCanvasTool(canvasToolMask)
	}
	for a.annotateToolButton.Clicked(gtx) {
		a.setCanvasTool(canvasToolAnnotate)
	}
	for a.annotateRectButton.Clicked(gtx) {
		a.setCanvasAnnotationKind(canvasAnnotationKindRect)
	}
	for a.annotateArrowButton.Clicked(gtx) {
		a.setCanvasAnnotationKind(canvasAnnotationKindArrow)
	}
	for a.annotateFreehandButton.Clicked(gtx) {
		a.setCanvasAnnotationKind(canvasAnnotationKindFreehand)
	}
	for a.annotateTextButton.Clicked(gtx) {
		a.setCanvasAnnotationKind(canvasAnnotationKindText)
	}
	for a.clearAnnotationsButton.Clicked(gtx) {
		a.clearCanvasAnnotations()
	}
	for idx := range canvasAnnotationColors {
		btn := &a.annotateColorButtons[idx]
		for btn.Clicked(gtx) {
			a.setCanvasAnnotationColor(canvasAnnotationColors[idx])
		}
	}
	for a.undoCanvasButton.Clicked(gtx) {
		a.undoLatestCanvasAction()
	}
	for a.redoCanvasButton.Clicked(gtx) {
		a.redoLatestCanvasAction()
	}
	for a.maskPaintButton.Clicked(gtx) {
		a.setCanvasBrushMode(canvasBrushPaint)
	}
	for a.maskEraseButton.Clicked(gtx) {
		a.setCanvasBrushMode(canvasBrushErase)
	}
	for a.maskBrushSizeDownButton.Clicked(gtx) {
		a.adjustCanvasBrushSize(-5)
	}
	for a.maskBrushSizeUpButton.Clicked(gtx) {
		a.adjustCanvasBrushSize(5)
	}
	for a.importMaskButton.Clicked(gtx) {
		paths, err := chooseImageFiles()
		if err != nil {
			a.appendLog("选择蒙版失败: " + err.Error())
		} else if len(paths) > 0 {
			if err := a.importCanvasMask(paths[0]); err != nil {
				a.appendLog("导入蒙版失败: " + err.Error())
			} else {
				a.appendLog("已导入蒙版图片: " + filepath.Base(paths[0]))
			}
		}
	}
	for a.clearMaskButton.Clicked(gtx) {
		a.clearCanvasMask()
	}
	for a.resetViewButton.Clicked(gtx) {
		a.resetCanvasView()
	}
	canNavigateBatchResults := canStepBatchResultSnapshot(snap)
	showResultGridToggle := len(snap.BatchResults) > 1
	currentTool := a.currentCanvasInteractionTool()
	currentBrushMode := a.currentCanvasBrushMode()
	currentBrushSize := a.currentCanvasBrushSize()
	hasImportedMask := a.hasImportedCanvasMask()
	currentAnnotationKind := a.currentCanvasAnnotationKind()
	currentAnnotationColor := a.currentCanvasAnnotationColor()
	canUndoCanvas := a.canUndoCanvasAction()
	canRedoCanvas := a.canRedoCanvasAction()
	for a.previousBatchResultButton.Clicked(gtx) {
		if err := a.stepBatchResult(-1); err != nil && !isMissingPreview(err) {
			a.appendLog("切换上一张失败: " + err.Error())
		}
	}
	for a.nextBatchResultButton.Clicked(gtx) {
		if err := a.stepBatchResult(1); err != nil && !isMissingPreview(err) {
			a.appendLog("切换下一张失败: " + err.Error())
		}
	}
	selectedCropRect, hasSelectedCropRect := a.currentSelectedCanvasCropRect()
	if canTransformCurrent && a.rotateLeftButton.Clicked(gtx) {
		a.startCurrentImageTransform("左转", "rotate", func(path string) (string, error) {
			return rotateImageFile(path, -90)
		})
	}
	if canTransformCurrent && a.rotateRightButton.Clicked(gtx) {
		a.startCurrentImageTransform("右转", "rotate", func(path string) (string, error) {
			return rotateImageFile(path, 90)
		})
	}
	if canTransformCurrent && a.flipHorizontalButton.Clicked(gtx) {
		a.startCurrentImageTransform("水平翻转", "flip", func(path string) (string, error) {
			return flipImageFile(path, true)
		})
	}
	if canTransformCurrent && a.flipVerticalButton.Clicked(gtx) {
		a.startCurrentImageTransform("竖直翻转", "flip", func(path string) (string, error) {
			return flipImageFile(path, false)
		})
	}
	if canTransformCurrent && hasSelectedCropRect && a.cropSelectionButton.Clicked(gtx) {
		rect := selectedCropRect
		a.startCurrentImageTransform("裁出", "crop", func(path string) (string, error) {
			return cropImageFile(path, rect)
		})
	}
	if a.clearCurrentButton.Clicked(gtx) {
		a.clearCurrentResult()
	}
	if a.fullscreenButton.Clicked(gtx) {
		a.toggleFullscreen()
	}
	if a.resultDetailButton.Clicked(gtx) {
		if snap.Result.HasItem {
			a.openResultDetail(snap.Result.Item)
		}
	}
	for a.macCanvasMoreButton.Clicked(gtx) {
		if snap.Result.HasItem {
			a.openHistoryActionMenu(snap.Result.Item, "canvas")
		}
	}
	currentSavedPath := strings.TrimSpace(snap.Result.SavedPath)
	hasCanvasResult := snap.Result.HasItem || currentSavedPath != ""
	compareActive := snap.Compare.HasItem && snap.Compare.Image != nil && !snap.ResultGridOpen
	showLatestJump := hasLatest && latestItem.ID != "" && latestItem.ID != snap.SelectedHistoryID
	if normalizeDesktopStyle(a.desktopStyle) == desktopStyleMacOS {
		return a.layoutMacCanvasToolbar(gtx, snap, macCanvasToolbarState{
			hasCanvasResult:      hasCanvasResult,
			showResultGridToggle: showResultGridToggle,
			canNavigateResults:   canNavigateBatchResults,
			canUndo:              canUndoCanvas,
			canRedo:              canRedoCanvas,
			canTransform:         canTransformCurrent,
			hasSelectedCrop:      hasSelectedCropRect,
			hasImportedMask:      hasImportedMask,
			compareActive:        compareActive,
			showLatestJump:       showLatestJump,
			currentTool:          currentTool,
			currentBrushMode:     currentBrushMode,
			currentBrushSize:     currentBrushSize,
			annotationKind:       currentAnnotationKind,
			annotationColor:      currentAnnotationColor,
		})
	}
	var resultDisplay historyItemDisplay
	if snap.Result.HasItem {
		resultDisplay = a.historyItemDisplay(snap.Result.Item)
	}

	return a.borderedSurface(gtx, fluent.panel2, unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !hasCanvasResult {
						return layout.Dimensions{}
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.toolbarCluster(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.toolbarIconButton(gtx, &a.panToolButton, uiIconPanTool, currentTool == canvasToolPan)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.toolbarIconButton(gtx, &a.maskToolButton, uiIconBrush, currentTool == canvasToolMask)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.toolbarIconButton(gtx, &a.annotateToolButton, uiIconAnnotate, currentTool == canvasToolAnnotate)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !canUndoCanvas {
											return a.toolbarStaticIcon(gtx, uiIconUndo, false, true)
										}
										return a.toolbarIconButton(gtx, &a.undoCanvasButton, uiIconUndo, false)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !canRedoCanvas {
											return a.toolbarStaticIcon(gtx, uiIconRedo, false, true)
										}
										return a.toolbarIconButton(gtx, &a.redoCanvasButton, uiIconRedo, false)
									}),
								)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.toolbarSeparator(gtx)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if currentTool == canvasToolMask {
								return a.toolbarCluster(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarTextButton(gtx, &a.maskPaintButton, uiIconBrush, "画笔", currentBrushMode == canvasBrushPaint)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarTextButton(gtx, &a.maskEraseButton, uiIconDelete, "橡皮", currentBrushMode == canvasBrushErase)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarTextButton(gtx, &a.importMaskButton, uiIconSource, "导入蒙版", hasImportedMask)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarIconButton(gtx, &a.maskBrushSizeDownButton, uiIconChevronLeft, false)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarStaticTextButton(gtx, fmt.Sprintf("%dpx", currentBrushSize), false)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarIconButton(gtx, &a.maskBrushSizeUpButton, uiIconChevronRight, false)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarTextButton(gtx, &a.clearMaskButton, uiIconDelete, "清空蒙版", false)
										}),
									)
								})
							}
							if currentTool == canvasToolAnnotate {
								return a.toolbarCluster(gtx, func(gtx layout.Context) layout.Dimensions {
									children := make([]layout.FlexChild, 0, len(canvasAnnotationColors)+5)
									children = append(children,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarTextButton(gtx, &a.annotateRectButton, uiIconAnnotate, "矩形", currentAnnotationKind == canvasAnnotationKindRect)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarTextButton(gtx, &a.annotateArrowButton, uiIconCompare, "箭头", currentAnnotationKind == canvasAnnotationKindArrow)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarTextButton(gtx, &a.annotateFreehandButton, uiIconEdit, "自由画", currentAnnotationKind == canvasAnnotationKindFreehand)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarTextButton(gtx, &a.annotateTextButton, uiIconList, "文字", currentAnnotationKind == canvasAnnotationKindText)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Inset{Left: unit.Dp(2), Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return a.toolbarSeparator(gtx)
											})
										}),
									)
									for idx, value := range canvasAnnotationColors {
										idx := idx
										value := value
										children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.toolbarColorButton(gtx, &a.annotateColorButtons[idx], value, currentAnnotationColor == value)
										}))
									}
									children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.toolbarTextButton(gtx, &a.clearAnnotationsButton, uiIconDelete, "清空标注", false)
									}))
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, children...)
								})
							}
							return a.toolbarTextButton(gtx, &a.resetViewButton, uiIconPanTool, "重置视图", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.toolbarSeparator(gtx)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.toolbarCluster(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !canTransformCurrent {
											return a.toolbarStaticIcon(gtx, uiIconRotateLeft, false, true)
										}
										return a.toolbarIconButton(gtx, &a.rotateLeftButton, uiIconRotateLeft, false)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !canTransformCurrent {
											return a.toolbarStaticIcon(gtx, uiIconRotateRight, false, true)
										}
										return a.toolbarIconButton(gtx, &a.rotateRightButton, uiIconRotateRight, false)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !canTransformCurrent {
											return a.toolbarStaticIcon(gtx, uiIconFlip, false, true)
										}
										return a.toolbarIconButton(gtx, &a.flipHorizontalButton, uiIconFlip, false)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !canTransformCurrent {
											return a.toolbarStaticIcon(gtx, uiIconFlip, false, true)
										}
										return a.toolbarIconButton(gtx, &a.flipVerticalButton, uiIconFlip, false)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										if !canTransformCurrent || !hasSelectedCropRect {
											return a.toolbarStaticIcon(gtx, uiIconCrop, false, true)
										}
										return a.toolbarTextButton(gtx, &a.cropSelectionButton, uiIconCrop, "裁出", false)
									}),
								)
							})
						}),
					)
				}),
				layout.Flexed(1, layout.Spacer{}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					rightChildren := make([]layout.FlexChild, 0, 8)
					if showLatestJump {
						rightChildren = append(rightChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.toolbarTextButton(gtx, &a.latestResultButton, uiIconHistory, "最近结果", false)
						}))
					}
					if showResultGridToggle {
						label := fmt.Sprintf("网格 %d", len(snap.BatchResults))
						if snap.ResultGridOpen {
							label = "单图"
						}
						rightChildren = append(rightChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.toolbarTextButton(gtx, &a.toggleResultGridButton, uiIconGrid, label, snap.ResultGridOpen)
						}))
					}
					if canNavigateBatchResults {
						rightChildren = append(rightChildren,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.toolbarIconButton(gtx, &a.previousBatchResultButton, uiIconChevronLeft, false)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.toolbarIconButton(gtx, &a.nextBatchResultButton, uiIconChevronRight, false)
							}),
						)
					}
					if compareActive {
						rightChildren = append(rightChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.toolbarTextButton(gtx, &a.closeCompareButton, uiIconCompare, "退出对比", true)
						}))
					}
					if snap.Result.HasItem {
						rightChildren = append(rightChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.metaBadgeRow(gtx, resultDisplay.MetaBadges, true)
						}))
					}
					rightChildren = append(rightChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						icon := uiIconFullscreen
						if snap.Fullscreen {
							icon = uiIconFullscreenExit
						}
						return a.toolbarIconButton(gtx, &a.fullscreenButton, icon, snap.Fullscreen)
					}))
					if snap.Result.HasItem {
						rightChildren = append(rightChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.toolbarIconButton(gtx, &a.resultDetailButton, uiIconInfo, false)
						}))
						rightChildren = append(rightChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.toolbarIconButton(gtx, &a.clearCurrentButton, uiIconDelete, false)
						}))
					}
					if snap.Result.HasItem && canSaveHistoryItem(snap.Result.Item) {
						if canDragOutHistoryItem(snap.Result.Item) {
							rightChildren = append(rightChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.toolbarTextButton(gtx, &a.dragOutButton, uiIconLaunch, "拖出复制", false)
							}))
						}
						rightChildren = append(rightChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.toolbarPrimaryTextButton(gtx, &a.saveAsButton, uiIconDownload, "另存为")
						}))
					}
					return a.toolbarCluster(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx, rightChildren...)
					})
				}),
			)
		})
	})
}

func (a *App) toolbarSeparator(gtx layout.Context) layout.Dimensions {
	return fixedWidth(gtx, unit.Dp(1), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
			return a.surface(gtx, withAlpha(fluent.border, 0xd0), unit.Dp(0), layout.Spacer{}.Layout)
		})
	})
}

func (a *App) toolbarCluster(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return body(gtx)
}

func (a *App) sourceStrip(gtx layout.Context, snap snapshot, sourcePaths []string) layout.Dimensions {
	for a.addSourceStripButton.Clicked(gtx) {
		paths, err := chooseImageFiles()
		if err != nil {
			a.appendLog("选择源图失败: " + err.Error())
		} else {
			for _, path := range paths {
				a.appendSourcePath(path)
			}
		}
	}
	for a.clearSourcesButton.Clicked(gtx) {
		a.setSourcePaths(nil)
	}
	compareActive := false
	if len(sourcePaths) > 0 {
		compareBtn := a.sourceButton("compare:" + sourcePaths[0])
		for compareBtn.Clicked(gtx) {
			if err := a.compareSourcePathOnCanvas(sourcePaths[0]); err != nil {
				a.appendLog("对比主参考图失败: " + err.Error())
			}
		}
		compareActive = compareItemActive("source-preview:"+strings.TrimSpace(sourcePaths[0]), snap.Compare.Item.ID)
	}
	for _, path := range sourcePaths {
		path := path
		removeBtn := a.sourceButton("remove:" + path)
		for removeBtn.Clicked(gtx) {
			a.removeSourcePath(path)
		}
		previewBtn := a.sourceButton("preview:" + path)
		for previewBtn.Clicked(gtx) {
			if err := a.viewSourcePathOnCanvas(path); err != nil {
				a.appendLog("预览参考图失败: " + err.Error())
			}
		}
		moveLeftBtn := a.sourceButton("move-left:" + path)
		for moveLeftBtn.Clicked(gtx) {
			a.moveSourcePath(path, -1)
		}
		moveRightBtn := a.sourceButton("move-right:" + path)
		for moveRightBtn.Clicked(gtx) {
			a.moveSourcePath(path, 1)
		}
	}

	label := "参考图 " + strconv.Itoa(len(sourcePaths)) + " 张"

	return a.borderedSurface(gtx, fluent.panel2, unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(80), func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, label, unit.Sp(11), fluent.textMuted, font.SemiBold)
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					tiles := make([]layout.FlexChild, 0, len(sourcePaths)+2)
					if len(sourcePaths) > 0 {
						compareBtn := a.sourceButton("compare:" + sourcePaths[0])
						tiles = append(tiles, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.compactIconTextButton(gtx, compareBtn, uiIconCompare, "对比主参考图", compareActive)
						}))
					}
					for idx, path := range sourcePaths {
						path := path
						indexLabel := strconv.Itoa(idx + 1)
						tiles = append(tiles, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							active := strings.TrimSpace(snap.Result.SavedPath) == strings.TrimSpace(path) || strings.TrimSpace(snap.Result.Item.SavedPath) == strings.TrimSpace(path)
							return a.layoutSourceStripTile(gtx, path, indexLabel, active, idx, len(sourcePaths))
						}))
					}
					tiles = append(tiles, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.layoutSourceStripAddTile(gtx)
					}))
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, tiles...)
				}),
			)
		})
	})
}

func (a *App) layoutSourceStripTile(gtx layout.Context, path string, indexLabel string, active bool, index int, total int) layout.Dimensions {
	removeBtn := a.sourceButton("remove:" + path)
	previewBtn := a.sourceButton("preview:" + path)
	moveLeftBtn := a.sourceButton("move-left:" + path)
	moveRightBtn := a.sourceButton("move-right:" + path)
	img, imgOp := a.displayPathThumb(path, gtx.Dp(unit.Dp(48)))
	bg := fluent.surface
	border := fluent.border
	if previewBtn.Hovered() {
		bg = fluent.surface2
		border = accentAlpha(0x38)
	}
	if active {
		bg = fluent.surface2
		border = fluent.accent
	}
	return previewBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.borderedSurface(gtx, bg, unit.Dp(6), border, func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(48), func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, unit.Dp(48), func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return a.imageThumbCoverWithOp(gtx, img, imgOp, unit.Dp(48), unit.Dp(48), unit.Dp(6))
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return layout.NW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(3), Top: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.surface(gtx, rgba(0x111111, 0xb8), unit.Dp(3), func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Top: 1, Bottom: 1, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, indexLabel, unit.Sp(8), fluent.white, font.Medium)
										})
									})
								})
							})
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							label := sourceStripFormatLabel(path)
							if label == "" {
								return layout.Dimensions{}
							}
							return layout.SW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(3), Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.surface(gtx, rgba(0x111111, 0xb8), unit.Dp(3), func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Top: 1, Bottom: 1, Left: 4, Right: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, label, unit.Sp(7), fluent.white, font.Medium)
										})
									})
								})
							})
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(3), Right: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if index <= 0 {
												return layout.Dimensions{}
											}
											return a.surfaceButton(
												gtx,
												moveLeftBtn,
												rgba(0x111111, 0xc0),
												rgba(0x111111, 0xdb),
												rgba(0xffffff, 0x00),
												unit.Dp(3),
												layout.Inset{Top: 2, Bottom: 2, Left: 2, Right: 2},
												func(gtx layout.Context) layout.Dimensions {
													return fixedWidth(gtx, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
														return fixedHeight(gtx, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
															return uiIconChevronLeft.Layout(gtx, fluent.white)
														})
													})
												},
											)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if index >= total-1 {
												return layout.Dimensions{}
											}
											return a.surfaceButton(
												gtx,
												moveRightBtn,
												rgba(0x111111, 0xc0),
												rgba(0x111111, 0xdb),
												rgba(0xffffff, 0x00),
												unit.Dp(3),
												layout.Inset{Top: 2, Bottom: 2, Left: 2, Right: 2},
												func(gtx layout.Context) layout.Dimensions {
													return fixedWidth(gtx, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
														return fixedHeight(gtx, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
															return uiIconChevronRight.Layout(gtx, fluent.white)
														})
													})
												},
											)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.surfaceButton(
												gtx,
												removeBtn,
												rgba(0x111111, 0xc0),
												dangerAlpha(0xd8),
												rgba(0xffffff, 0x00),
												unit.Dp(3),
												layout.Inset{Top: 2, Bottom: 2, Left: 2, Right: 2},
												func(gtx layout.Context) layout.Dimensions {
													return fixedWidth(gtx, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
														return fixedHeight(gtx, unit.Dp(10), func(gtx layout.Context) layout.Dimensions {
															return uiIconClose.Layout(gtx, fluent.white)
														})
													})
												},
											)
										}),
									)
								})
							})
						}),
					)
				})
			})
		})
	})
}

func (a *App) layoutSourceStripAddTile(gtx layout.Context) layout.Dimensions {
	return a.borderedSurface(gtx, fluent.surface, unit.Dp(6), fluent.border, func(gtx layout.Context) layout.Dimensions {
		return a.surfaceButton(
			gtx,
			&a.addSourceStripButton,
			fluent.surface,
			fluent.toolHoverBg,
			rgba(0xffffff, 0x00),
			unit.Dp(6),
			layout.Inset{},
			func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, unit.Dp(48), func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, unit.Dp(48), func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
									return uiIconAdd.Layout(gtx, fluent.textDim)
								})
							})
						})
					})
				})
			},
		)
	})
}

func sourceStripFormatLabel(path string) string {
	ext := strings.TrimPrefix(strings.ToUpper(filepath.Ext(strings.TrimSpace(path))), ".")
	if ext == "" {
		return ""
	}
	return ext
}

func (a *App) resultSurface(gtx layout.Context, snap snapshot) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingResultSurface, time.Now())
	for a.emptyStateImportButton.Clicked(gtx) {
		paths, err := chooseImageFiles()
		if err != nil {
			a.appendLog("选择图片失败: " + err.Error())
		} else if len(paths) > 0 {
			if err := a.importImagePathAsEditSource(paths[0]); err != nil {
				a.appendLog("载入本地图片失败: " + err.Error())
			}
		}
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	return a.surface(gtx, fluent.canvasBg, unit.Dp(0), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		if normalizeDesktopStyle(a.desktopStyle) != desktopStyleMacOS || snap.Result.Image != nil {
			a.paintCheckerboard(gtx, clip.Rect{Max: gtx.Constraints.Max}.Op(), gtx.Dp(unit.Dp(22)), fluent.canvasBg, fluent.canvasTile)
		}
		liveBatchTotal := 0
		if snap.Running {
			liveBatchTotal = snap.BatchLiveSlotCount
		}
		if snap.ResultGridOpen && batchGridTotalSlots(snap.BatchResults, snap.BatchPreviewItems, liveBatchTotal) > 1 {
			a.canvasDisplayScale = 0
			return a.layoutBatchResultGrid(gtx, snap)
		}
		if snap.Result.Image == nil {
			a.canvasDisplayScale = 0
			return layout.Center.Layout(gtx, a.layoutCanvasEmptyState)
		}
		if snap.Result.Rev != a.imageOpRev {
			a.imageOp = paint.NewImageOp(snap.Result.Image)
			a.imageOpRev = snap.Result.Rev
		}
		if snap.Compare.Image != nil && snap.Compare.Rev != a.compareImageOpRev {
			a.compareImageOp = paint.NewImageOp(snap.Compare.Image)
			a.compareImageOpRev = snap.Compare.Rev
		}
		if snap.Compare.Image != nil {
			return a.layoutCompareSurface(gtx, snap)
		}
		return layout.UniformInset(unit.Dp(28)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.layoutCanvasViewport(gtx, snap)
		})
	})
}

func (a *App) layoutCanvasImageContain(gtx layout.Context, src image.Image, op paint.ImageOp) layout.Dimensions {
	if src == nil {
		a.canvasDisplayScale = 0
		return layout.Dimensions{}
	}
	size := containNoUpscaleSize(src.Bounds().Dx(), src.Bounds().Dy(), gtx.Constraints.Max.X, gtx.Constraints.Max.Y)
	if size.X <= 0 || size.Y <= 0 {
		a.canvasDisplayScale = 0
		return layout.Dimensions{}
	}
	a.canvasDisplayScale = float32(size.X) / float32(src.Bounds().Dx())
	view := widget.Image{
		Src:      op,
		Fit:      widget.Contain,
		Position: layout.Center,
	}
	return fixedPixelWidth(gtx, size.X, func(gtx layout.Context) layout.Dimensions {
		return fixedPixelHeight(gtx, size.Y, view.Layout)
	})
}

func canvasViewStateKey(state resultState) string {
	return strings.TrimSpace(state.Item.ID) + "|" + strings.TrimSpace(state.SavedPath) + "|" + strings.TrimSpace(state.SourceEvent)
}

func (a *App) syncCanvasViewState(state resultState) {
	key := canvasViewStateKey(state)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.canvasViewKey == key {
		return
	}
	a.canvasViewKey = key
	a.canvasViewScale = 1
	a.canvasViewOffset = image.Point{}
	a.canvasViewDragging = false
	a.canvasViewLastDragPos = image.Point{}
	a.resetCanvasMaskLocked()
	a.resetCanvasAnnotationsLocked()
}

func (a *App) resetCanvasView() {
	a.mu.Lock()
	a.canvasViewScale = 1
	a.canvasViewOffset = image.Point{}
	a.canvasViewDragging = false
	a.canvasViewLastDragPos = image.Point{}
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) canvasViewState() (float32, image.Point, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	scale := a.canvasViewScale
	if scale <= 0 {
		scale = 1
	}
	return scale, a.canvasViewOffset, a.canvasViewDragging
}

func (a *App) setCanvasView(scale float32, offset image.Point) {
	if scale <= 0 {
		scale = 1
	}
	a.mu.Lock()
	a.canvasViewScale = scale
	a.canvasViewOffset = offset
	a.mu.Unlock()
	a.invalidateSoon(16 * time.Millisecond)
}

func applyCanvasZoom(current float32, delta float32) float32 {
	if current <= 0 {
		current = 1
	}
	if delta == 0 {
		return current
	}
	factor := float32(1.12)
	if delta > 0 {
		current /= factor
	} else {
		current *= factor
	}
	if current < 0.05 {
		current = 0.05
	}
	if current > 8 {
		current = 8
	}
	return current
}

func (a *App) layoutCanvasViewport(gtx layout.Context, snap snapshot) layout.Dimensions {
	src := snap.Result.Image
	if src == nil {
		a.canvasDisplayScale = 0
		return layout.Center.Layout(gtx, a.layoutCanvasEmptyState)
	}
	a.syncCanvasViewState(snap.Result)
	if snap.Result.Rev != a.imageOpRev {
		a.imageOp = paint.NewImageOp(snap.Result.Image)
		a.imageOpRev = snap.Result.Rev
	}
	currentTool := a.currentCanvasTool()
	scale, offset, dragging := a.canvasViewState()
	viewportMax := gtx.Constraints.Max
	if viewportMax.X <= 0 || viewportMax.Y <= 0 {
		a.canvasDisplayScale = 0
		return layout.Dimensions{}
	}
	baseSize := containNoUpscaleSize(src.Bounds().Dx(), src.Bounds().Dy(), viewportMax.X, viewportMax.Y)
	if baseSize.X <= 0 || baseSize.Y <= 0 {
		a.canvasDisplayScale = 0
		return layout.Dimensions{}
	}
	displaySize := image.Pt(
		max(1, int(float32(baseSize.X)*scale)),
		max(1, int(float32(baseSize.Y)*scale)),
	)
	a.canvasDisplayScale = float32(displaySize.X) / float32(src.Bounds().Dx())

	fullArea := clip.Rect{Max: viewportMax}.Push(gtx.Ops)
	event.Op(gtx.Ops, &a.canvasPointerTag)
	switch {
	case currentTool == canvasToolMask:
		pointer.CursorCrosshair.Add(gtx.Ops)
	case currentTool == canvasToolAnnotate:
		pointer.CursorCrosshair.Add(gtx.Ops)
	case dragging:
		pointer.CursorGrabbing.Add(gtx.Ops)
	default:
		pointer.CursorGrab.Add(gtx.Ops)
	}
	fullArea.Pop()

	origin := image.Pt((viewportMax.X-displaySize.X)/2+offset.X, (viewportMax.Y-displaySize.Y)/2+offset.Y)

	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  &a.canvasPointerTag,
			Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -1_000_000, Max: 1_000_000},
			ScrollY: pointer.ScrollRange{Min: -1_000_000, Max: 1_000_000},
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			if currentTool == canvasToolMask {
				point, inside := canvasPointFromPointer(pe.Position, origin, a.canvasDisplayScale, src.Bounds())
				if !inside {
					continue
				}
				a.startCanvasMaskStroke(point, src.Bounds())
				continue
			}
			if currentTool == canvasToolAnnotate {
				point, inside := canvasPointFromPointer(pe.Position, origin, a.canvasDisplayScale, src.Bounds())
				if !inside {
					a.clearCanvasAnnotationSelection()
					continue
				}
				kind := a.currentCanvasAnnotationKind()
				color := a.currentCanvasAnnotationColor()
				annotations, _, _, _, _ := a.canvasAnnotationState()
				if selectedID, ok := hitCanvasAnnotation(annotations, point); ok {
					a.selectCanvasAnnotation(selectedID)
					a.clearCanvasAnnotationDraft()
					continue
				}
				a.clearCanvasAnnotationSelection()
				if kind == canvasAnnotationKindText {
					a.openCanvasAnnotationTextPrompt(point)
				} else if kind == canvasAnnotationKindFreehand {
					a.startCanvasFreehandDraft(color, point)
				} else {
					a.updateCanvasAnnotationDraft(kind, color, point, point)
				}
				continue
			}
			a.mu.Lock()
			a.canvasViewDragging = true
			a.canvasViewLastDragPos = image.Pt(int(pe.Position.X), int(pe.Position.Y))
			a.mu.Unlock()
		case pointer.Drag:
			if currentTool == canvasToolMask {
				if point, inside := canvasPointFromPointer(pe.Position, origin, a.canvasDisplayScale, src.Bounds()); inside {
					a.appendCanvasMaskStrokePoint(point, src.Bounds())
				}
				continue
			}
			if currentTool == canvasToolAnnotate {
				if point, inside := canvasPointFromPointer(pe.Position, origin, a.canvasDisplayScale, src.Bounds()); inside {
					_, _, draft, _, _ := a.canvasAnnotationState()
					if draft != nil {
						if draft.Kind == canvasAnnotationKindFreehand {
							a.appendCanvasFreehandDraftPoint(point)
						} else {
							a.updateCanvasAnnotationDraft(draft.Kind, draft.Color, draft.Start, point)
						}
					}
				}
				continue
			}
			a.mu.Lock()
			last := a.canvasViewLastDragPos
			current := image.Pt(int(pe.Position.X), int(pe.Position.Y))
			a.canvasViewOffset = a.canvasViewOffset.Add(current.Sub(last))
			a.canvasViewLastDragPos = current
			a.mu.Unlock()
			a.invalidateSoon(16 * time.Millisecond)
		case pointer.Release, pointer.Cancel:
			if currentTool == canvasToolMask {
				a.commitCanvasMaskStroke()
				continue
			}
			if currentTool == canvasToolAnnotate {
				_, _, draft, _, _ := a.canvasAnnotationState()
				if draft != nil {
					a.clearCanvasAnnotationDraft()
					switch draft.Kind {
					case canvasAnnotationKindRect:
						rect := normalizeCanvasAnnotationRect(draft.Start, draft.Current)
						if validCanvasAnnotationRect(rect) {
							a.addCanvasAnnotationItem(canvasAnnotation{
								ID:    "ann-" + strconv.FormatInt(time.Now().UnixNano(), 36),
								Kind:  draft.Kind,
								Color: draft.Color,
								Rect:  rect,
							})
						}
					case canvasAnnotationKindArrow:
						rect := image.Rectangle{Min: draft.Start, Max: draft.Current}
						if draft.Start != draft.Current {
							a.addCanvasAnnotationItem(canvasAnnotation{
								ID:    "ann-" + strconv.FormatInt(time.Now().UnixNano(), 36),
								Kind:  draft.Kind,
								Color: draft.Color,
								Rect:  rect,
							})
						}
					case canvasAnnotationKindFreehand:
						if validCanvasFreehandPoints(draft.Points) {
							a.addCanvasAnnotationItem(canvasAnnotation{
								ID:     "ann-" + strconv.FormatInt(time.Now().UnixNano(), 36),
								Kind:   draft.Kind,
								Color:  draft.Color,
								Points: append([]image.Point(nil), draft.Points...),
							})
						}
					}
				}
				continue
			}
			a.mu.Lock()
			a.canvasViewDragging = false
			a.mu.Unlock()
		case pointer.Scroll:
			scrollDelta := pe.Scroll.Y
			if scrollDelta == 0 {
				scrollDelta = pe.Scroll.X
			}
			if scrollDelta != 0 {
				nextScale := applyCanvasZoom(scale, scrollDelta)
				if nextScale != scale {
					ratio := nextScale / scale
					nextOffset := image.Pt(
						int(float32(offset.X)*ratio),
						int(float32(offset.Y)*ratio),
					)
					scale = nextScale
					offset = nextOffset
					a.setCanvasView(nextScale, nextOffset)
				}
			}
		}
	}
	scale, offset, _ = a.canvasViewState()
	displaySize = image.Pt(
		max(1, int(float32(baseSize.X)*scale)),
		max(1, int(float32(baseSize.Y)*scale)),
	)
	origin = image.Pt((viewportMax.X-displaySize.X)/2+offset.X, (viewportMax.Y-displaySize.Y)/2+offset.Y)

	paintArea := clip.Rect{Max: viewportMax}.Push(gtx.Ops)
	defer paintArea.Pop()
	imageOffset := op.Offset(origin).Push(gtx.Ops)
	dims := fixedPixelWidth(gtx, displaySize.X, func(gtx layout.Context) layout.Dimensions {
		return fixedPixelHeight(gtx, displaySize.Y, func(gtx layout.Context) layout.Dimensions {
			view := widget.Image{
				Src:      a.imageOp,
				Fit:      widget.Fill,
				Position: layout.Center,
			}
			return view.Layout(gtx)
		})
	})
	imageOffset.Pop()
	maskStrokes, maskDraft, _, _ := a.canvasMaskState()
	a.paintCanvasMaskStrokes(gtx, maskStrokes, maskDraft, origin, displaySize)
	annotations, selectedAnnotationID, draft, _, _ := a.canvasAnnotationState()
	a.paintCanvasAnnotations(gtx, annotations, selectedAnnotationID, draft, origin, a.canvasDisplayScale)
	return layout.Dimensions{Size: viewportMax, Baseline: dims.Baseline}
}

func canvasPointFromPointer(pos f32.Point, origin image.Point, scale float32, bounds image.Rectangle) (image.Point, bool) {
	if scale <= 0 {
		return image.Point{}, false
	}
	x := int((pos.X - float32(origin.X)) / scale)
	y := int((pos.Y - float32(origin.Y)) / scale)
	inside := x >= bounds.Min.X && x < bounds.Max.X && y >= bounds.Min.Y && y < bounds.Max.Y
	x = clampInt(x, bounds.Min.X, bounds.Max.X-1)
	y = clampInt(y, bounds.Min.Y, bounds.Max.Y-1)
	return image.Pt(x, y), inside
}

func paintCanvasRectOutline(gtx layout.Context, rect image.Rectangle, strokePx int, color color.NRGBA) {
	if rect.Empty() || strokePx <= 0 {
		return
	}
	paint.FillShape(gtx.Ops, color, clip.Rect(image.Rect(rect.Min.X, rect.Min.Y, rect.Max.X, rect.Min.Y+strokePx)).Op())
	paint.FillShape(gtx.Ops, color, clip.Rect(image.Rect(rect.Min.X, rect.Max.Y-strokePx, rect.Max.X, rect.Max.Y)).Op())
	paint.FillShape(gtx.Ops, color, clip.Rect(image.Rect(rect.Min.X, rect.Min.Y, rect.Min.X+strokePx, rect.Max.Y)).Op())
	paint.FillShape(gtx.Ops, color, clip.Rect(image.Rect(rect.Max.X-strokePx, rect.Min.Y, rect.Max.X, rect.Max.Y)).Op())
}

func paintCanvasLineStroke(gtx layout.Context, from image.Point, to image.Point, width float32, color color.NRGBA) {
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(float32(from.X), float32(from.Y)))
	path.LineTo(f32.Pt(float32(to.X), float32(to.Y)))
	paint.FillShape(gtx.Ops, color, clip.Stroke{
		Path:  path.End(),
		Width: width,
	}.Op())
}

func paintCanvasPolyline(gtx layout.Context, points []image.Point, width float32, color color.NRGBA) {
	if len(points) == 0 {
		return
	}
	if len(points) == 1 {
		rect := image.Rect(points[0].X-1, points[0].Y-1, points[0].X+2, points[0].Y+2)
		paint.FillShape(gtx.Ops, color, clip.Rect(rect).Op())
		return
	}
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(f32.Pt(float32(points[0].X), float32(points[0].Y)))
	for _, point := range points[1:] {
		path.LineTo(f32.Pt(float32(point.X), float32(point.Y)))
	}
	paint.FillShape(gtx.Ops, color, clip.Stroke{
		Path:  path.End(),
		Width: width,
	}.Op())
}

func paintCanvasArrow(gtx layout.Context, from image.Point, to image.Point, width float32, color color.NRGBA) {
	paintCanvasLineStroke(gtx, from, to, width, color)
	dx := float64(to.X - from.X)
	dy := float64(to.Y - from.Y)
	length := math.Hypot(dx, dy)
	if length < 1 {
		return
	}
	ux := dx / length
	uy := dy / length
	headLen := math.Max(10, float64(width)*3.5)
	headWidth := math.Max(6, float64(width)*2.2)
	left := image.Pt(
		int(float64(to.X)-ux*headLen-uy*headWidth),
		int(float64(to.Y)-uy*headLen+ux*headWidth),
	)
	right := image.Pt(
		int(float64(to.X)-ux*headLen+uy*headWidth),
		int(float64(to.Y)-uy*headLen-ux*headWidth),
	)
	paintCanvasLineStroke(gtx, to, left, width, color)
	paintCanvasLineStroke(gtx, to, right, width, color)
}

func (a *App) paintCanvasAnnotations(gtx layout.Context, annotations []canvasAnnotation, selectedAnnotationID string, draft *canvasAnnotationDraft, origin image.Point, scale float32) {
	if scale <= 0 {
		return
	}
	strokePx := max(1, gtx.Dp(unit.Dp(2)))
	for _, annotation := range annotations {
		strokeColor := annotation.Color
		selectionColor := rgb(0x7e5cff)
		width := float32(strokePx)
		switch normalizeCanvasAnnotationKind(annotation.Kind) {
		case canvasAnnotationKindRect:
			rect := image.Rect(
				origin.X+int(float32(annotation.Rect.Min.X)*scale),
				origin.Y+int(float32(annotation.Rect.Min.Y)*scale),
				origin.X+int(float32(annotation.Rect.Max.X)*scale),
				origin.Y+int(float32(annotation.Rect.Max.Y)*scale),
			)
			if annotation.ID == selectedAnnotationID {
				paintCanvasRectOutline(gtx, rect.Inset(-strokePx), strokePx, selectionColor)
			}
			paintCanvasRectOutline(gtx, rect, strokePx, strokeColor)
		case canvasAnnotationKindArrow:
			from := image.Pt(
				origin.X+int(float32(annotation.Rect.Min.X)*scale),
				origin.Y+int(float32(annotation.Rect.Min.Y)*scale),
			)
			to := image.Pt(
				origin.X+int(float32(annotation.Rect.Max.X)*scale),
				origin.Y+int(float32(annotation.Rect.Max.Y)*scale),
			)
			if annotation.ID == selectedAnnotationID {
				paintCanvasArrow(gtx, from, to, width+2, selectionColor)
			}
			paintCanvasArrow(gtx, from, to, width, strokeColor)
		case canvasAnnotationKindFreehand:
			points := make([]image.Point, 0, len(annotation.Points))
			for _, point := range annotation.Points {
				points = append(points, image.Pt(
					origin.X+int(float32(point.X)*scale),
					origin.Y+int(float32(point.Y)*scale),
				))
			}
			if annotation.ID == selectedAnnotationID {
				paintCanvasPolyline(gtx, points, width+2, selectionColor)
			}
			paintCanvasPolyline(gtx, points, width, strokeColor)
		case canvasAnnotationKindText:
			offset := op.Offset(image.Pt(
				origin.X+int(float32(annotation.Rect.Min.X)*scale),
				origin.Y+int(float32(annotation.Rect.Min.Y)*scale),
			)).Push(gtx.Ops)
			size := unit.Sp(16)
			if scaled := 16 * scale; scaled > 0 {
				size = unit.Sp(maxFloat32(8, scaled))
			}
			if annotation.ID == selectedAnnotationID {
				highlightRect := image.Rect(0, 0, int(float32(annotation.Rect.Dx())*scale), int(float32(annotation.Rect.Dy())*scale))
				paint.FillShape(gtx.Ops, accentAlpha(0x22), clip.Rect(highlightRect).Op())
			}
			a.label(gtx, annotation.Text, size, strokeColor, font.SemiBold)
			offset.Pop()
		}
	}
	if draft != nil {
		switch draft.Kind {
		case canvasAnnotationKindRect:
			rect := normalizeCanvasAnnotationRect(draft.Start, draft.Current)
			if validCanvasAnnotationRect(rect) {
				displayRect := image.Rect(
					origin.X+int(float32(rect.Min.X)*scale),
					origin.Y+int(float32(rect.Min.Y)*scale),
					origin.X+int(float32(rect.Max.X)*scale),
					origin.Y+int(float32(rect.Max.Y)*scale),
				)
				paintCanvasRectOutline(gtx, displayRect, strokePx, draft.Color)
			}
		case canvasAnnotationKindArrow:
			from := image.Pt(origin.X+int(float32(draft.Start.X)*scale), origin.Y+int(float32(draft.Start.Y)*scale))
			to := image.Pt(origin.X+int(float32(draft.Current.X)*scale), origin.Y+int(float32(draft.Current.Y)*scale))
			paintCanvasArrow(gtx, from, to, float32(strokePx), draft.Color)
		case canvasAnnotationKindFreehand:
			points := make([]image.Point, 0, len(draft.Points))
			for _, point := range draft.Points {
				points = append(points, image.Pt(
					origin.X+int(float32(point.X)*scale),
					origin.Y+int(float32(point.Y)*scale),
				))
			}
			paintCanvasPolyline(gtx, points, float32(strokePx), draft.Color)
		}
	}
}

func containNoUpscaleSize(srcW int, srcH int, maxW int, maxH int) image.Point {
	if srcW <= 0 || srcH <= 0 || maxW <= 0 || maxH <= 0 {
		return image.Point{}
	}
	scaleX := float32(maxW) / float32(srcW)
	scaleY := float32(maxH) / float32(srcH)
	scale := minFloat32(scaleX, scaleY)
	if scale > 1 {
		scale = 1
	}
	return image.Pt(
		max(1, int(float32(srcW)*scale)),
		max(1, int(float32(srcH)*scale)),
	)
}

func formatCanvasScaleLabel(scale float32) string {
	if scale <= 0 {
		return ""
	}
	return strconv.Itoa(int(scale*100+0.5)) + "%"
}

func minFloat32(values ...float32) float32 {
	if len(values) == 0 {
		return 0
	}
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}

func maxFloat32(values ...float32) float32 {
	if len(values) == 0 {
		return 0
	}
	maximum := values[0]
	for _, value := range values[1:] {
		if value > maximum {
			maximum = value
		}
	}
	return maximum
}

func (a *App) layoutCompareSurface(gtx layout.Context, snap snapshot) layout.Dimensions {
	split := snap.CompareSplit
	if split < 0 {
		split = 0
	}
	if split > 1 {
		split = 1
	}
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return a.layoutCompareViewport(gtx, snap.Result.Image, a.imageOp, snap.Compare.Image, a.compareImageOp, split)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.NW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.layoutCompareBadge(gtx, "A · 当前图", rgba(0x111111, 0x8c), rgba(0x9ec5ff, 0xff))
				})
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.layoutCompareBadge(gtx, "B · 对比图", rgba(0x111111, 0x8c), rgba(0xcdb8ff, 0xff))
				})
			})
		}),
	)
}

func (a *App) layoutCompareViewport(gtx layout.Context, currentImg image.Image, currentOp paint.ImageOp, compareImg image.Image, compareOp paint.ImageOp, split float32) layout.Dimensions {
	max := gtx.Constraints.Max
	gtx.Constraints.Min = max
	for {
		ev, ok := a.compareSplitDrag.Update(gtx.Metric, gtx.Source, gesture.Horizontal)
		if !ok {
			break
		}
		if max.X <= 0 {
			continue
		}
		a.noteRenderActivity()
		next := ev.Position.X / float32(max.X)
		if next < 0 {
			next = 0
		}
		if next > 1 {
			next = 1
		}
		a.compareSplitSlider.Value = next
		split = next
	}
	splitPx := clampInt(int(float32(max.X)*split), 0, max.X)
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			stack := clip.Rect(image.Rect(0, 0, splitPx, max.Y)).Push(gtx.Ops)
			defer stack.Pop()
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.layoutCanvasImageContain(gtx, currentImg, currentOp)
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			stack := clip.Rect(image.Rect(splitPx, 0, max.X, max.Y)).Push(gtx.Ops)
			defer stack.Pop()
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.layoutCanvasImageContain(gtx, compareImg, compareOp)
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if max.X <= 0 || max.Y <= 0 {
				return layout.Dimensions{Size: max}
			}
			lineLeft := clampInt(splitPx-1, 0, max.X)
			lineRight := clampInt(splitPx+1, 0, max.X)
			compareAccent := rgb(0x7e5cff)
			if lineRight > lineLeft {
				paint.FillShape(gtx.Ops, compareAccent, clip.Rect(image.Rect(lineLeft, 0, lineRight, max.Y)).Op())
			}
			centerX := clampInt(splitPx, 12, max.X-12)
			handleRect := image.Rect(centerX-12, max.Y/2-12, centerX+12, max.Y/2+12)
			paint.FillShape(gtx.Ops, compareAccent, clip.Ellipse(handleRect).Op(gtx.Ops))
			labelOffset := op.Offset(image.Pt(centerX-12, max.Y/2-12)).Push(gtx.Ops)
			fixedPixelWidth(gtx, 24, func(gtx layout.Context) layout.Dimensions {
				return fixedPixelHeight(gtx, 24, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "⇆", unit.Sp(11), fluent.white, font.Medium)
					})
				})
			})
			labelOffset.Pop()
			return layout.Dimensions{Size: max}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if max.X <= 0 || max.Y <= 0 {
				return layout.Dimensions{Size: max}
			}
			centerX := clampInt(splitPx, 12, max.X-12)
			dragRect := image.Rect(centerX-18, 0, centerX+18, max.Y)
			stack := clip.Rect(dragRect).Push(gtx.Ops)
			pointer.CursorColResize.Add(gtx.Ops)
			a.compareSplitDrag.Add(gtx.Ops)
			stack.Pop()
			return layout.Dimensions{Size: max}
		}),
	)
}

func (a *App) layoutCompareBadge(gtx layout.Context, text string, bg color.NRGBA, fg color.NRGBA) layout.Dimensions {
	return a.surface(gtx, bg, unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 2, Bottom: 2, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
		})
	})
}

func (a *App) layoutBatchResultGrid(gtx layout.Context, snap snapshot) layout.Dimensions {
	items := snap.BatchResults
	previewItems := snap.BatchPreviewItems
	liveBatchTotal := 0
	preferPreviewSlots := false
	if snap.Running {
		liveBatchTotal = snap.BatchLiveSlotCount
		preferPreviewSlots = liveBatchTotal > 0
	}
	slots := buildBatchGridSlots(items, previewItems, liveBatchTotal, preferPreviewSlots)
	totalSlots := len(slots)
	livePreview := snap.Running && totalSlots > 1
	columns := 3
	if totalSlots <= 2 {
		columns = 2
	} else if totalSlots <= 4 {
		columns = 2
	}
	rows := (totalSlots + columns - 1) / columns
	return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						title := fmt.Sprintf("本批结果 · %d 张", len(items))
						if livePreview {
							title = fmt.Sprintf("本批预览 · %d/%d", len(items), totalSlots)
						}
						subtitle := ""
						if snap.Running && snap.BatchTotal > 1 {
							subtitle = fmt.Sprintf("进行中 · 已完成 %d/%d", len(items), snap.BatchTotal)
						}
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(2))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, title, unit.Sp(12), fluent.text, font.SemiBold)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if subtitle == "" {
									return layout.Dimensions{}
								}
								return a.singleLineLabel(gtx, subtitle, unit.Sp(10), fluent.textDim, font.Normal)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if livePreview {
							return layout.Dimensions{}
						}
						return a.compactButton(gtx, &a.closeResultGridButton, "返回当前图", false)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
		}
		for row := 0; row < rows; row++ {
			row := row
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				cells := make([]layout.FlexChild, 0, columns)
				for col := 0; col < columns; col++ {
					idx := row*columns + col
					if idx >= totalSlots {
						cells = append(cells, layout.Flexed(1, layout.Spacer{}.Layout))
						continue
					}
					slot := slots[idx]
					cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: chooseBatchGridInset(col, columns), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							switch slot.Kind {
							case batchGridSlotResult:
								return a.layoutBatchGridTile(gtx, slot.Item, idx, snap.SelectedHistoryID == slot.Item.ID, false)
							case batchGridSlotPreview:
								return a.layoutBatchGridTile(gtx, slot.Item, idx, false, true)
							default:
								return a.layoutBatchGridPendingTile(gtx, idx)
							}
						})
					}))
				}
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
			}))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

func chooseBatchGridInset(col int, columns int) unit.Dp {
	if col == columns-1 {
		return 0
	}
	return unit.Dp(10)
}

func (a *App) layoutBatchGridTile(gtx layout.Context, item sharedCompat.HistoryItem, index int, active bool, preview bool) layout.Dimensions {
	var btn *widget.Clickable
	var dragBtn *widget.Clickable
	if !preview && strings.TrimSpace(item.ID) != "" {
		btn = a.historyButton("batch-grid:" + item.ID)
		for btn.Clicked(gtx) {
			if err := a.loadHistoryPreview(item, true); err != nil && !isMissingPreview(err) {
				a.appendLog("载入批量结果失败: " + err.Error())
			} else {
				a.closeResultGrid()
			}
		}
		dragBtn = a.historyActionButton("batch-grid-drag:" + item.ID)
		for dragBtn.Clicked(gtx) {
			next, err := a.dragOutHistoryItem(item)
			if err != nil {
				a.appendLog("拖出复制失败: " + err.Error())
				continue
			}
			item = next
		}
	}
	img, imgOp := a.displayHistoryThumb(item, gtx.Dp(unit.Dp(208)))
	tile := func(gtx layout.Context) layout.Dimensions {
		hovered := btn != nil && btn.Hovered()
		bg := fluent.surface
		hoverBg := fluent.surface
		border := fluent.border
		if hovered {
			border = accentAlpha(0x38)
			hoverBg = fluent.surface2
		}
		if preview {
			bg = fluent.surface2
			border = accentAlpha(0x42)
		}
		if active {
			bg = fluent.surface2
			hoverBg = fluent.surface2
			border = fluent.accent
		}
		return fixedHeight(gtx, unit.Dp(208), func(gtx layout.Context) layout.Dimensions {
			return a.elevatedBorderedSurface(gtx, chooseColor(hovered, hoverBg, bg), fluentCardRadius, border, image.Pt(0, 2), func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return a.surface(gtx, fluent.canvasBg, fluentCardRadius, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min = gtx.Constraints.Max
							if img == nil {
								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "预览", unit.Sp(10), fluent.textDim, font.Medium)
								})
							}
							view := widget.Image{
								Src:      imgOp,
								Fit:      widget.Contain,
								Position: layout.Center,
							}
							return view.Layout(gtx)
						})
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.NW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.surface(gtx, rgba(0x111111, 0xba), unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 2, Bottom: 2, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, fmt.Sprintf("#%d", index+1), unit.Sp(9), fluent.white, font.Medium)
									})
								})
							})
						})
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						if preview {
							return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Right: unit.Dp(8), Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.surface(gtx, accentAlpha(0xe8), unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Top: 2, Bottom: 2, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, "流式预览", unit.Sp(9), fluent.white, font.Medium)
										})
									})
								})
							})
						}
						if item.ElapsedSec <= 0 {
							return layout.Dimensions{}
						}
						return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Right: unit.Dp(8), Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.surface(gtx, rgba(0x111111, 0xba), unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 2, Bottom: 2, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, fmt.Sprintf("%.0fs", item.ElapsedSec), unit.Sp(9), fluent.white, font.Medium)
									})
								})
							})
						})
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						if !canDragOutHistoryItem(item) || preview || dragBtn == nil {
							return layout.Dimensions{}
						}
						return layout.SE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Right: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.surfaceButton(
									gtx,
									dragBtn,
									rgba(0x111111, 0xb2),
									rgba(0x111111, 0xdb),
									rgba(0xffffff, 0x00),
									unit.Dp(999),
									layout.Inset{Top: 4, Bottom: 4, Left: 8, Right: 8},
									func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
													return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
														return uiIconLaunch.Layout(gtx, fluent.white)
													})
												})
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return a.label(gtx, "拖出复制", unit.Sp(9), fluent.white, font.Medium)
											}),
										)
									},
								)
							})
						})
					}),
				)
			})
		})
	}
	if btn == nil {
		return tile(gtx)
	}
	return btn.Layout(gtx, tile)
}

func (a *App) layoutBatchGridPendingTile(gtx layout.Context, index int) layout.Dimensions {
	return fixedHeight(gtx, unit.Dp(208), func(gtx layout.Context) layout.Dimensions {
		return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return a.surface(gtx, fluent.surface2, fluentCardRadius, func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Min = gtx.Constraints.Max
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return fixedWidth(gtx, unit.Dp(34), func(gtx layout.Context) layout.Dimensions {
										return fixedHeight(gtx, unit.Dp(34), func(gtx layout.Context) layout.Dimensions {
											return a.borderedSurface(gtx, rgba(0xffffff, 0x00), unit.Dp(17), accentAlpha(0x38), func(gtx layout.Context) layout.Dimensions {
												return layout.Dimensions{Size: gtx.Constraints.Min}
											})
										})
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "等待预览", unit.Sp(11), fluent.textDim, font.Medium)
								}),
							)
						})
					})
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.NW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.surface(gtx, rgba(0x111111, 0xba), unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: 2, Bottom: 2, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, fmt.Sprintf("#%d", index+1), unit.Sp(9), fluent.white, font.Medium)
								})
							})
						})
					})
				}),
			)
		})
	})
}

func (a *App) layoutCanvasEmptyState(gtx layout.Context) layout.Dimensions {
	if normalizeDesktopStyle(a.desktopStyle) == desktopStyleMacOS {
		return a.layoutMacCanvasEmptyState(gtx)
	}
	copy := "先在左侧写提示词，再开始生成第一张图。"
	if a.mode == string(client.ModeEdit) {
		copy = "图生图时可直接导入一张本地图片，或从历史结果里挑一张继续编辑。"
	}
	return fixedWidth(gtx, unit.Dp(384), func(gtx layout.Context) layout.Dimensions {
		return a.elevatedBorderedSurface(gtx, withAlpha(fluent.white, 0xb8), unit.Dp(16), fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 32, Bottom: 32, Left: 28, Right: 28}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(64), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(64), func(gtx layout.Context) layout.Dimensions {
									return a.borderedSurface(gtx, fluent.accentSoft, unit.Dp(14), accentAlpha(0x22), func(gtx layout.Context) layout.Dimensions {
										return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return fixedWidth(gtx, unit.Dp(24), func(gtx layout.Context) layout.Dimensions {
												return fixedHeight(gtx, unit.Dp(24), func(gtx layout.Context) layout.Dimensions {
													return uiIconPhoto.Layout(gtx, fluent.accent)
												})
											})
										})
									})
								})
							})
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(14)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, "还没有图片", unit.Sp(18), fluent.text, font.SemiBold)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(292), func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, copy, unit.Sp(12), fluent.textMuted, font.Normal)
							})
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(16)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.surfaceButton(
								gtx,
								&a.emptyStateImportButton,
								withAlpha(fluent.white, 0xb3),
								fluent.surface2,
								fluent.border,
								unit.Dp(10),
								layout.Inset{Top: 10, Bottom: 10, Left: 16, Right: 16},
								func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
												return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
													return uiIconSource.Layout(gtx, fluent.text)
												})
											})
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, "选择本地图片", unit.Sp(12), fluent.text, font.Medium)
										}),
									)
								},
							)
						})
					}),
				)
			})
		})
	})
}

func (a *App) paintCheckerboard(gtx layout.Context, area clip.Op, tile int, first color.NRGBA, second color.NRGBA) {
	if a.reducedEffects {
		paint.FillShape(gtx.Ops, first, area)
		return
	}
	if tile <= 0 {
		tile = 16
	}
	max := gtx.Constraints.Max
	if max.X <= 0 || max.Y <= 0 {
		return
	}
	cache := &a.checkerboard
	if cache.img == nil || cache.size != max || cache.tile != tile || cache.first != first || cache.second != second {
		img := image.NewRGBA(image.Rect(0, 0, max.X, max.Y))
		draw.Draw(img, img.Bounds(), image.NewUniform(first), image.Point{}, draw.Src)
		for y := 0; y < max.Y; y += tile {
			for x := 0; x < max.X; x += tile {
				if ((x/tile)+(y/tile))%2 == 0 {
					continue
				}
				rect := image.Rect(x, y, min(x+tile, max.X), min(y+tile, max.Y))
				draw.Draw(img, rect, image.NewUniform(second), image.Point{}, draw.Src)
			}
		}
		*cache = checkerboardCache{
			size:   max,
			tile:   tile,
			first:  first,
			second: second,
			img:    img,
			op:     paint.NewImageOp(img),
		}
	}
	stack := area.Push(gtx.Ops)
	defer stack.Pop()
	cache.op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func (a *App) canvasStatusBar(gtx layout.Context, snap snapshot) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingCanvasStatusBar, time.Now())
	if normalizeDesktopStyle(a.desktopStyle) == desktopStyleMacOS {
		return a.layoutMacCanvasStatusBar(gtx, snap)
	}
	lastLog := ""
	if len(snap.Logs) > 0 {
		lastLog = snap.Logs[len(snap.Logs)-1]
	}
	hasLastLog := strings.TrimSpace(lastLog) != ""
	zoomLabel := formatCanvasScaleLabel(a.canvasDisplayScale)
	renderDiagnostics := formatRenderDiagnostics(snap)
	hasRenderDiagnostics := strings.TrimSpace(renderDiagnostics) != ""
	hasRevisedPrompt := strings.TrimSpace(snap.Result.RevisedPrompt) != ""

	return a.borderedSurface(gtx, fluent.panel2, unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 9, Bottom: 9, Left: 14, Right: 14}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			if snap.Running || snap.ProcessingImageTransform {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
										return uiIconRefresh.Layout(gtx, fluent.accent)
									})
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, chooseStatusText(snap.Status), unit.Sp(11), fluent.text, font.Medium)
								})
							}),
							layout.Flexed(1, layout.Spacer{}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !hasLastLog {
									return layout.Dimensions{}
								}
								return fixedWidth(gtx, unit.Dp(220), func(gtx layout.Context) layout.Dimensions {
									return a.singleLineLabel(gtx, lastLog, unit.Sp(11), fluent.textDim, font.Normal)
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if snap.BatchTotal <= 1 {
									return layout.Dimensions{}
								}
								completed := len(snap.BatchResults)
								return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.singleLineLabel(gtx, fmt.Sprintf("已完成 %d/%d", completed, snap.BatchTotal), unit.Sp(11), fluent.textDim, font.Normal)
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !hasRenderDiagnostics {
									return layout.Dimensions{}
								}
								return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.singleLineLabel(gtx, renderDiagnostics, unit.Sp(11), fluent.textDim, font.Normal)
								})
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

			if snap.Result.HasItem {
				display := a.historyItemDisplay(snap.Result.Item)
				headline := "生成结果"
				if snap.Result.Item.Mode == "edit" {
					headline = "编辑结果"
				}
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return uiIconCheck.Layout(gtx, fluent.accent)
							})
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, headline, unit.Sp(11), fluent.accent, font.Medium)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.metaBadgeRow(gtx, display.StatusMetaBadges, true)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if display.ClockPrecise == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.singleLineLabel(gtx, display.ClockPrecise, unit.Sp(11), fluent.textDim, font.Normal)
						})
					}),
					func() layout.FlexChild {
						if !hasRevisedPrompt {
							return layout.Flexed(1, layout.Spacer{}.Layout)
						}
						return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, "✨ "+snap.Result.RevisedPrompt, unit.Sp(11), fluent.textDim, font.Normal)
							})
						})
					}(),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if zoomLabel == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.singleLineLabel(gtx, zoomLabel, unit.Sp(11), fluent.textDim, font.Normal)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !hasRenderDiagnostics {
							return layout.Dimensions{}
						}
						return layout.Inset{Left: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.singleLineLabel(gtx, renderDiagnostics, unit.Sp(11), fluent.textDim, font.Normal)
						})
					}),
				)
			}

			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return uiIconCheck.Layout(gtx, fluent.textDim)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "准备就绪", unit.Sp(11), fluent.textMuted, font.Normal)
					})
				}),
				layout.Flexed(1, layout.Spacer{}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !hasRenderDiagnostics {
						return layout.Dimensions{}
					}
					return a.singleLineLabel(gtx, renderDiagnostics, unit.Sp(11), fluent.textDim, font.Normal)
				}),
			)
		})
	})
}

func (a *App) layoutRunningStatusProgressBar(gtx layout.Context) layout.Dimensions {
	return fixedHeight(gtx, unit.Dp(2), func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Min
		if size.X == 0 {
			size.X = gtx.Constraints.Max.X
		}
		if size.Y == 0 {
			size.Y = gtx.Dp(unit.Dp(2))
		}
		paint.FillShape(gtx.Ops, withAlpha(fluent.accent, 0x18), clip.Rect(image.Rect(0, 0, size.X, size.Y)).Op())
		if size.X > 0 {
			if a.reducedEffects {
				paint.FillShape(gtx.Ops, fluent.accent, clip.Rect(image.Rect(0, 0, size.X, size.Y)).Op())
				return layout.Dimensions{Size: size}
			}
			paintLinearGradient(gtx, image.Rect(0, 0, size.X, size.Y), 0, fluent.accent, fluent.accent2)
		}
		return layout.Dimensions{Size: size}
	})
}

func chooseStatusText(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "正在请求..."
	}
	return status
}

func (a *App) layoutSavePrompt(gtx layout.Context, snap snapshot) layout.Dimensions {
	if a.savePromptNeverAsk.Update(gtx) {
		a.setSavePromptSuppressed(a.savePromptNeverAsk.Value)
	}
	for a.savePromptSelectAllButton.Clicked(gtx) {
		a.setAllSavePromptBatchSelections(true)
	}
	for a.savePromptClearSelectionButton.Clicked(gtx) {
		a.setAllSavePromptBatchSelections(false)
	}
	for a.savePromptChooseDirButton.Clicked(gtx) {
		dir, err := chooseDirectory()
		if err != nil {
			a.appendLog("选择批量另存为目录失败: " + err.Error())
		} else if strings.TrimSpace(dir) != "" {
			a.savePromptPathInput.SetText(dir)
		}
	}
	for a.savePromptSkipButton.Clicked(gtx) {
		a.closeSavePrompt()
	}
	for a.savePromptSaveButton.Clicked(gtx) {
		a.savePromptCopy()
	}
	if len(snap.SavePromptBatchItems) > 0 {
		for _, item := range snap.SavePromptBatchItems {
			btn := a.savePromptSelectionButton("batch:" + item.ID)
			for btn.Clicked(gtx) {
				a.toggleSavePromptBatchSelection(item.ID)
			}
		}
		return a.layoutBatchSavePrompt(gtx, snap)
	}
	item := snap.Result.Item
	img := snap.Result.Image
	imgOp := a.imageOp
	return a.layoutStandardModal(
		gtx,
		unit.Dp(520),
		0,
		"是否另存这张图片?",
		"",
		nil,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
								return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.imageThumbWithOp(gtx, img, imgOp, unit.Dp(116), unit.Dp(116), unit.Dp(8))
								})
							})
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							sourcePath := strings.TrimSpace(item.SavedPath)
							if sourcePath == "" {
								sourcePath = strings.TrimSpace(a.savePromptSourcePath)
							}
							generatedLocally := sourcePath != ""
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if generatedLocally {
										return a.label(gtx, "图片已生成并保存在默认输出目录。", unit.Sp(13), fluent.text, font.Medium)
									}
									return a.label(gtx, "图片已生成，当前先保留在预览态。", unit.Sp(13), fluent.text, font.Medium)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if generatedLocally {
										return a.label(gtx, "需要放到项目、相册或其他目录时，可以现在填写目标位置另存一份。", unit.Sp(11), fluent.textMuted, font.Normal)
									}
									return a.label(gtx, "当前结果还没有额外保存到本地文件，继续操作前可以先写到你指定的位置。", unit.Sp(11), fluent.textMuted, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									path := sourcePath
									if path == "" {
										return layout.Dimensions{}
									}
									return a.borderedSurface(gtx, fluent.surface2, unit.Dp(8), fluent.border, func(gtx layout.Context) layout.Dimensions {
										return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, path, unit.Sp(10), fluent.textDim, font.Normal)
										})
									})
								}),
							)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.technicalField(gtx, "保存到", &a.savePromptPathInput, "输入完整文件路径或目录", unit.Dp(48))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					style := material.CheckBox(a.th, &a.savePromptNeverAsk, "以后不再提示")
					style.Color = fluent.text
					style.IconColor = fluent.accent
					return style.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(110), func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.savePromptSkipButton, "稍后", false)
							})
						}),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(152), func(gtx layout.Context) layout.Dimensions {
								return a.primaryIconTextButton(gtx, &a.savePromptSaveButton, uiIconFolder, "保存到指定位置", fluent.accent, fluent.white)
							})
						}),
					)
				}),
			)
		},
	)
}

func (a *App) layoutBatchSavePrompt(gtx layout.Context, snap snapshot) layout.Dimensions {
	items := snap.SavePromptBatchItems
	selectedCount := a.savePromptBatchSelectedCount()
	columns := 3
	if len(items) <= 4 {
		columns = 2
	}
	rows := (len(items) + columns - 1) / columns
	return a.layoutStandardModal(
		gtx,
		unit.Dp(860),
		unit.Dp(720),
		fmt.Sprintf("本次结果 · %d 张", len(items)),
		"",
		nil,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "单次任务只弹这一次。勾选需要的图片后，再统一另存为到目标目录。", unit.Sp(12), fluent.text, font.Medium)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, fmt.Sprintf("当前已选 %d / %d 张。", selectedCount, len(items)), unit.Sp(11), fluent.textMuted, font.Normal)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.compactButton(gtx, &a.savePromptSelectAllButton, "全选", false)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.compactButton(gtx, &a.savePromptClearSelectionButton, "清空", false)
								}),
							)
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface2, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							children := make([]layout.FlexChild, 0, rows)
							for row := 0; row < rows; row++ {
								row := row
								children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									cells := make([]layout.FlexChild, 0, columns)
									for col := 0; col < columns; col++ {
										idx := row*columns + col
										if idx >= len(items) {
											cells = append(cells, layout.Flexed(1, layout.Spacer{}.Layout))
											continue
										}
										item := items[idx]
										selected := a.savePromptBatchSelected(item.ID)
										cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return layout.Inset{Right: chooseBatchGridInset(col, columns), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return a.layoutSavePromptBatchTile(gtx, item, idx, selected)
											})
										}))
									}
									return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
								}))
							}
							return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.technicalField(gtx, "目录", &a.savePromptPathInput, "输入或选择目录", unit.Dp(48))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(104), func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.savePromptChooseDirButton, uiIconFolder, "选择目录", false)
							})
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					style := material.CheckBox(a.th, &a.savePromptNeverAsk, "以后不再提示")
					style.Color = fluent.text
					style.IconColor = fluent.accent
					return style.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(110), func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.savePromptSkipButton, "稍后", false)
							})
						}),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(176), func(gtx layout.Context) layout.Dimensions {
								return a.primaryIconTextButton(gtx, &a.savePromptSaveButton, uiIconFolder, fmt.Sprintf("另存选中项 (%d)", selectedCount), fluent.accent, fluent.white)
							})
						}),
					)
				}),
			)
		},
	)
}

func (a *App) layoutSavePromptBatchTile(gtx layout.Context, item sharedCompat.HistoryItem, index int, selected bool) layout.Dimensions {
	btn := a.savePromptSelectionButton("batch:" + item.ID)
	img, imgOp := a.displayHistoryThumb(item, gtx.Dp(unit.Dp(208)))
	bg := fluent.surface
	hoverBg := fluent.surface2
	border := fluent.border
	if btn.Hovered() {
		border = accentAlpha(0x38)
	}
	if selected {
		bg = fluent.surface2
		hoverBg = fluent.surface2
		border = fluent.accent
	}
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(208), func(gtx layout.Context) layout.Dimensions {
			return a.elevatedBorderedSurface(gtx, chooseColor(btn.Hovered(), hoverBg, bg), fluentCardRadius, border, image.Pt(0, 2), func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return a.surface(gtx, fluent.canvasBg, fluentCardRadius, func(gtx layout.Context) layout.Dimensions {
							gtx.Constraints.Min = gtx.Constraints.Max
							if img == nil {
								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "预览", unit.Sp(10), fluent.textDim, font.Medium)
								})
							}
							view := widget.Image{
								Src:      imgOp,
								Fit:      widget.Contain,
								Position: layout.Center,
							}
							return view.Layout(gtx)
						})
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.NW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(8), Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.surface(gtx, rgba(0x111111, 0xba), unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 2, Bottom: 2, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, fmt.Sprintf("#%d", index+1), unit.Sp(9), fluent.white, font.Medium)
									})
								})
							})
						})
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Right: unit.Dp(8), Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								label := "未选"
								tagBg := rgba(0x111111, 0xba)
								if selected {
									label = "已选"
									tagBg = accentAlpha(0xe8)
								}
								return a.surface(gtx, tagBg, unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 2, Bottom: 2, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, label, unit.Sp(9), fluent.white, font.Medium)
									})
								})
							})
						})
					}),
				)
			})
		})
	})
}
