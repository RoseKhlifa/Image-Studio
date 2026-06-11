package ui

import (
	"fmt"
	"image"
	"math"
	"strconv"
	"strings"
	"time"

	sharedCompat "image-studio/shared/compat"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

func (a *App) layoutHistoryTimelineModal(gtx layout.Context, snap snapshot) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingTimelineModal, time.Now())
	clearCompareBtn := a.historyActionButton("timeline-clear-compare")
	for a.closeHistoryTimelineButton.Clicked(gtx) {
		a.closeHistoryTimeline()
	}
	for clearCompareBtn.Clicked(gtx) {
		a.clearCompare()
	}
	for a.historyTimelineModePickerButton.Clicked(gtx) {
		a.historyTimelineModePickerOpen = !a.historyTimelineModePickerOpen
		if a.historyTimelineModePickerOpen {
			a.historyTimelineDatePickerOpen = false
		}
	}
	for a.historyTimelineDatePickerButton.Clicked(gtx) {
		a.historyTimelineDatePickerOpen = !a.historyTimelineDatePickerOpen
		if a.historyTimelineDatePickerOpen {
			a.historyTimelineModePickerOpen = false
		}
	}
	for idx, value := range []string{"all", "generate", "edit"} {
		for a.historyTimelineModeButtons[idx].Clicked(gtx) {
			a.historyTimelineModeFilter = value
			a.historyTimelineModePickerOpen = false
		}
	}
	for idx, value := range []string{"all", "today", "week"} {
		for a.historyTimelineDateButtons[idx].Clicked(gtx) {
			a.historyTimelineDateFilter = value
			a.historyTimelineDatePickerOpen = false
		}
	}

	if !snap.HistoryTimelineOpen {
		return layout.Dimensions{}
	}

	data := a.historyTimelineData(snap.History)
	dayGroups := data.dayGroups
	selectedGroupKey := data.selectedGroupKey
	compareItemID := snap.Compare.Item.ID
	countText := strconv.Itoa(data.filteredCount)
	if data.filteredCount != len(snap.History) {
		countText += " / " + strconv.Itoa(len(snap.History))
	}
	return a.layoutStandardModal(
		gtx,
		unit.Dp(920),
		unit.Dp(660),
		"更多历史",
		countText+" 项",
		&a.closeHistoryTimelineButton,
		func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutTimelineFilterRow(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutTimelineFilterMenus(gtx)
				}),
			}
			if snap.Compare.HasItem {
				children = append(children,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.compactIconTextButton(gtx, clearCompareBtn, uiIconCompare, "退出对比", true)
					}),
				)
			}
			children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if len(dayGroups) == 0 {
					return a.emptyPanel(gtx, "没有匹配的历史记录")
				}
				return a.historyTimelineList.Layout(gtx, len(dayGroups), func(gtx layout.Context, index int) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.layoutHistoryTimelineDayGroup(gtx, dayGroups[index], snap.SelectedHistoryID, selectedGroupKey, compareItemID)
					})
				})
			}))
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx, children...)
		},
	)
}

func timelineModeFilterLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "generate":
		return "文生图"
	case "edit":
		return "图生图"
	default:
		return "全部模式"
	}
}

func timelineDateFilterLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "today":
		return "今天"
	case "week":
		return "近 7 天"
	default:
		return "全部日期"
	}
}

func (a *App) timelineFilterButton(gtx layout.Context, btn *widget.Clickable, label string, open bool) layout.Dimensions {
	return a.surfaceButton(
		gtx,
		btn,
		chooseColor(open, fluent.surface2, fluent.surface),
		fluent.surface2,
		fluent.border,
		fluentControlRadius,
		layout.Inset{Top: 9, Bottom: 9, Left: 10, Right: 10},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.singleLineLabel(gtx, label, unit.Sp(12), fluent.text, font.Normal)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					icon := uiIconExpand
					if open {
						icon = uiIconCollapse
					}
					return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
							return icon.Layout(gtx, fluent.textDim)
						})
					})
				}),
			)
		},
	)
}

func (a *App) layoutTimelineFilterRow(gtx layout.Context) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.searchField(gtx, &a.historyTimelineQueryInput, "搜索 prompt / revised prompt...")
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(148), func(gtx layout.Context) layout.Dimensions {
				return a.timelineFilterButton(gtx, &a.historyTimelineModePickerButton, timelineModeFilterLabel(a.historyTimelineModeFilter), a.historyTimelineModePickerOpen)
			})
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(148), func(gtx layout.Context) layout.Dimensions {
				return a.timelineFilterButton(gtx, &a.historyTimelineDatePickerButton, timelineDateFilterLabel(a.historyTimelineDateFilter), a.historyTimelineDatePickerOpen)
			})
		}),
	)
}

func (a *App) layoutTimelineFilterMenus(gtx layout.Context) layout.Dimensions {
	if !a.historyTimelineModePickerOpen && !a.historyTimelineDatePickerOpen {
		return layout.Dimensions{}
	}
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !a.historyTimelineModePickerOpen {
				return layout.Dimensions{}
			}
			return a.timelineFilterMenu(gtx, []timelineFilterOption{
				{Label: "全部模式", Button: &a.historyTimelineModeButtons[0], Active: a.historyTimelineModeFilter == "all"},
				{Label: "文生图", Button: &a.historyTimelineModeButtons[1], Active: a.historyTimelineModeFilter == "generate"},
				{Label: "图生图", Button: &a.historyTimelineModeButtons[2], Active: a.historyTimelineModeFilter == "edit"},
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if !a.historyTimelineDatePickerOpen {
				return layout.Dimensions{}
			}
			return a.timelineFilterMenu(gtx, []timelineFilterOption{
				{Label: "全部日期", Button: &a.historyTimelineDateButtons[0], Active: a.historyTimelineDateFilter == "all"},
				{Label: "今天", Button: &a.historyTimelineDateButtons[1], Active: a.historyTimelineDateFilter == "today"},
				{Label: "近 7 天", Button: &a.historyTimelineDateButtons[2], Active: a.historyTimelineDateFilter == "week"},
			})
		}),
	)
}

type timelineFilterOption struct {
	Label  string
	Button *widget.Clickable
	Active bool
}

func (a *App) timelineFilterMenu(gtx layout.Context, options []timelineFilterOption) layout.Dimensions {
	return a.borderedSurface(gtx, fluent.surface, fluentControlRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(options)*2)
			for idx := range options {
				opt := options[idx]
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.surfaceButton(
						gtx,
						opt.Button,
						chooseColor(opt.Active, fluent.accentSoft, rgba(0xffffff, 0x00)),
						chooseColor(opt.Active, accentAlpha(0x28), fluent.surface2),
						rgba(0xffffff, 0x00),
						fluentControlRadius,
						layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
						func(gtx layout.Context) layout.Dimensions {
							return a.singleLineLabel(gtx, opt.Label, unit.Sp(11), chooseColor(opt.Active, fluent.accent, fluent.text), font.Medium)
						},
					)
				}))
				if idx != len(options)-1 {
					children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout))
				}
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (a *App) layoutHistoryTimelineDayGroup(gtx layout.Context, dayGroup historyDayGroup, selectedHistoryID string, selectedGroupKey string, compareItemID string) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return uiIconCalendar.Layout(gtx, fluent.accent)
						})
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, dayGroup.Label, unit.Sp(13), fluent.text, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.singleLineLabel(gtx, strconv.Itoa(len(dayGroup.Entries))+" 组", unit.Sp(11), fluent.textMuted, font.Medium)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
	}
	for idx, entry := range dayGroup.Entries {
		entry := entry
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.layoutHistoryTimelineEntry(gtx, entry, selectedHistoryID, selectedGroupKey, compareItemID)
		}))
		if idx != len(dayGroup.Entries)-1 {
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
		}
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) layoutHistoryTimelineEntry(gtx layout.Context, entry historyPromptEntry, selectedHistoryID string, selectedGroupKey string, compareItemID string) layout.Dimensions {
	if entry.Kind == "group" {
		return a.layoutTimelineTrackRow(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.layoutHistoryTimelineGroupRow(gtx, *entry.Group, selectedHistoryID, selectedGroupKey, compareItemID, a.expandedPromptGroups[entry.Group.Key])
		})
	}
	return a.layoutTimelineTrackRow(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.layoutHistoryTimelineRow(gtx, *entry.Item, entry.Item.ID == selectedHistoryID, compareItemID)
	})
}

func (a *App) layoutHistoryTimelineGroupRow(gtx layout.Context, group historyPromptGroup, selectedHistoryID string, selectedGroupKey string, compareItemID string, expanded bool) layout.Dimensions {
	active := group.Key != "" && group.Key == selectedGroupKey
	summaryBtn := a.historyButton("timeline-group:" + group.Key)
	latestBtn := a.historyActionButton("timeline-group-latest:" + group.Key)
	expandBtn := a.historyActionButton("timeline-group-expand:" + group.Key)
	moreBtn := a.historyActionButton("timeline-group-more:" + group.Key)
	compareActive := compareItemActive(group.Representative.ID, compareItemID)
	display := group.RepresentativeDisplay
	for summaryBtn.Clicked(gtx) {
		a.expandedPromptGroups[group.Key] = !a.expandedPromptGroups[group.Key]
	}
	for latestBtn.Clicked(gtx) {
		if err := a.loadHistoryPreview(group.Representative, true); err != nil && !isMissingPreview(err) {
			a.appendLog("载入历史结果失败: " + err.Error())
		} else {
			a.closeHistoryTimeline()
		}
	}
	for expandBtn.Clicked(gtx) {
		a.expandedPromptGroups[group.Key] = !a.expandedPromptGroups[group.Key]
	}
	for moreBtn.Clicked(gtx) {
		a.openPromptGroup(group)
		a.closeHistoryTimeline()
	}

	return a.elevatedBorderedSurface(
		gtx,
		chooseColor(active || compareActive, fluent.surface2, fluent.surfaceElevated),
		fluentCardRadius,
		chooseColor(active || compareActive, accentAlpha(0x48), fluent.border),
		image.Pt(0, 1),
		func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 8, Bottom: 8, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return summaryBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.layoutTimelineGroupPile(gtx, group)
								})
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return summaryBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
														return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
															return uiIconGrid.Layout(gtx, fluent.textMuted)
														})
													})
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.label(gtx, "同提示词", unit.Sp(10), fluent.textMuted, font.Medium)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.metaBadgeRow(gtx, []string{group.CountText}, true)
												}),
											)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.clampedLabel(gtx, group.PromptPreview, unit.Sp(12), fluent.text, font.Medium, 2)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
														return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
															return uiIconHistory.Layout(gtx, fluent.textDim)
														})
													})
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.singleLineLabel(gtx, display.Clock, unit.Sp(10), fluent.textDim, font.Normal)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.metaBadgeRow(gtx, display.MetaBadges, true)
												}),
											)
										}),
									)
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.timelineActionButton(gtx, latestBtn, "查看最新", false)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								icon := uiIconExpand
								if expanded {
									icon = uiIconCollapse
								}
								return a.timelineActionIconButton(gtx, expandBtn, icon)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.timelineActionIconButton(gtx, moreBtn, uiIconMoreHoriz)
							}),
						)
					}),
				}
				if expanded {
					children = append(children,
						layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(1), func(gtx layout.Context) layout.Dimensions {
								return a.surface(gtx, withAlpha(fluent.border, 0xc0), 0, layout.Spacer{}.Layout)
							})
						}),
						layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.layoutTimelinePromptThumbGrid(gtx, group.Items, selectedHistoryID, compareItemID)
						}),
					)
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		},
	)
}

func (a *App) layoutTimelineTrackRow(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(120), func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
									return layout.Stack{}.Layout(gtx,
										layout.Stacked(func(gtx layout.Context) layout.Dimensions {
											return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
												return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
													return a.surface(gtx, accentAlpha(0x38), unit.Dp(6), layout.Spacer{}.Layout)
												})
											})
										}),
										layout.Stacked(func(gtx layout.Context) layout.Dimensions {
											return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return fixedWidth(gtx, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
													return fixedHeight(gtx, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
														return a.surface(gtx, fluent.accent, unit.Dp(4), layout.Spacer{}.Layout)
													})
												})
											})
										}),
									)
								})
							})
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(1), func(gtx layout.Context) layout.Dimensions {
								return a.surface(gtx, withAlpha(fluent.textDim, 0x22), 0, layout.Spacer{}.Layout)
							})
						}),
					)
				})
			})
		}),
		layout.Flexed(1, body),
	)
}

func (a *App) layoutTimelinePileLayer(gtx layout.Context, img image.Image, imgOp paint.ImageOp, width unit.Dp, height unit.Dp) layout.Dimensions {
	border := withAlpha(fluent.white, 0xdc)
	bg := rgb(0xf4f4f5)
	if resolveThemeMode(a.themeMode) == "dark" {
		border = withAlpha(fluent.white, 0x29)
		bg = rgb(0x27272a)
	}
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
			return a.elevatedBorderedSurface(gtx, bg, unit.Dp(14), border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				if img == nil {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "预览", unit.Sp(10), fluent.textDim, font.Medium)
					})
				}
				view := widget.Image{
					Src:      imgOp,
					Fit:      widget.Cover,
					Position: layout.Center,
				}
				return view.Layout(gtx)
			})
		})
	})
}

func (a *App) layoutTimelineGroupPile(gtx layout.Context, group historyPromptGroup) layout.Dimensions {
	return fixedWidth(gtx, unit.Dp(118), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(88), func(gtx layout.Context) layout.Dimensions {
			maxThumbs := min(3, len(group.Items))
			offsets := []image.Point{
				image.Pt(0, 0),
				image.Pt(9, 0),
				image.Pt(16, 1),
			}
			angles := []float32{
				float32(-1 * math.Pi / 180),
				float32(4 * math.Pi / 180),
				float32(8 * math.Pi / 180),
			}
			scales := []float32{1, 0.96, 0.92}
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					for idx := maxThumbs - 1; idx >= 0; idx-- {
						item := group.Items[idx]
						img, imgOp := a.displayHistoryThumb(*item, max(gtx.Dp(unit.Dp(98)), gtx.Dp(unit.Dp(74))))
						offset := offsets[min(idx, len(offsets)-1)]
						layout.Inset{
							Left: unit.Dp(float32(offset.X)),
							Top:  unit.Dp(float32(offset.Y)),
						}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							opacity := []float32{1, 0.86, 0.72}[min(idx, 2)]
							opacityStack := paint.PushOpacity(gtx.Ops, opacity)
							origin := f32.Pt(float32(gtx.Dp(unit.Dp(49))), float32(gtx.Dp(unit.Dp(37))))
							transform := f32.AffineId().
								Scale(origin, f32.Pt(scales[min(idx, len(scales)-1)], scales[min(idx, len(scales)-1)])).
								Rotate(origin, angles[min(idx, len(angles)-1)])
							stack := op.Affine(transform).Push(gtx.Ops)
							dims := a.layoutTimelinePileLayer(gtx, img, imgOp, unit.Dp(98), unit.Dp(74))
							stack.Pop()
							opacityStack.Pop()
							return dims
						})
					}
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					if len(group.Items) == 0 {
						return layout.Dimensions{}
					}
					return layout.NW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(7), Top: unit.Dp(7)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.historyModeBadge(gtx, group.Representative.Mode)
						})
					})
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.SE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(0), Bottom: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.elevatedBorderedSurface(gtx, fluent.accent, unit.Dp(999), withAlpha(fluent.white, 0xd8), image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: 5, Bottom: 5, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, group.CountValue, unit.Sp(10), fluent.white, font.SemiBold)
								})
							})
						})
					})
				}),
			)
		})
	})
}

func (a *App) layoutTimelinePromptThumbGrid(gtx layout.Context, items []*sharedCompat.HistoryItem, selectedHistoryID string, compareItemID string) layout.Dimensions {
	columns := historyAutoGridColumns(gtx, unit.Dp(104), unit.Dp(10))
	rows := (len(items) + columns - 1) / columns
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
				cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: chooseBatchGridInset(col, columns), Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.layoutTimelinePromptThumb(gtx, *item, idx, selectedHistoryID == item.ID, compareItemID)
					})
				}))
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) layoutTimelinePromptThumb(gtx layout.Context, item sharedCompat.HistoryItem, index int, active bool, compareItemID string) layout.Dimensions {
	btn := a.historyButton("timeline-group-thumb:" + item.ID)
	moreBtn := a.historyActionButton("timeline-group-thumb-more:" + item.ID)
	compareActive := compareItemActive(item.ID, compareItemID)
	for btn.Clicked(gtx) {
		if err := a.loadHistoryPreview(item, true); err != nil && !isMissingPreview(err) {
			a.appendLog("载入历史结果失败: " + err.Error())
		} else {
			a.closeHistoryTimeline()
		}
	}
	for moreBtn.Clicked(gtx) {
		a.openResultDetail(item)
		a.closeHistoryTimeline()
	}
	return a.elevatedSurfaceButton(
		gtx,
		btn,
		chooseColor(active || compareActive, fluent.surface2, fluent.surfaceElevated),
		fluent.surface2,
		chooseColor(active || compareActive, accentAlpha(0x48), fluent.border),
		fluentCardRadius,
		image.Pt(0, 1),
		layout.Inset{},
		func(gtx layout.Context) layout.Dimensions {
			displayIndex := index + 1
			if item.BatchIndex >= 0 {
				displayIndex = item.BatchIndex + 1
			}
			side := max(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(104)))
			img, imgOp := a.displayHistoryThumb(item, side)
			sideDp := unit.Dp(float32(side) / gtx.Metric.PxPerDp)
			return layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return fixedPixelWidth(gtx, side, func(gtx layout.Context) layout.Dimensions {
						return fixedPixelHeight(gtx, side, func(gtx layout.Context) layout.Dimensions {
							return a.imageThumbCoverWithOp(gtx, img, imgOp, sideDp, sideDp, unit.Dp(8))
						})
					})
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.NW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(6), Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.historyModeBadge(gtx, item.Mode)
						})
					})
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(6), Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							if compareActive {
								return a.historyCompareBadge(gtx)
							}
							return layout.Dimensions{}
						})
					})
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.SW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Left: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.surface(gtx, rgba(0x111111, 0xba), unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: 2, Bottom: 2, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, fmt.Sprintf("#%d", displayIndex), unit.Sp(9), fluent.white, font.Medium)
								})
							})
						})
					})
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.SE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							if !btn.Hovered() {
								return layout.Dimensions{}
							}
							return a.surfaceButton(
								gtx,
								moreBtn,
								rgba(0x111111, 0xb2),
								rgba(0x111111, 0xdb),
								rgba(0xffffff, 0x00),
								unit.Dp(999),
								layout.Inset{Top: 4, Bottom: 4, Left: 4, Right: 4},
								func(gtx layout.Context) layout.Dimensions {
									return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
										return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
											return uiIconMoreHoriz.Layout(gtx, fluent.white)
										})
									})
								},
							)
						})
					})
				}),
			)
		},
	)
}

func (a *App) layoutHistoryTimelineRow(gtx layout.Context, item sharedCompat.HistoryItem, active bool, compareItemID string) layout.Dimensions {
	rowBtn := a.historyButton("timeline-row:" + item.ID)
	detailBtn := a.historyActionButton("timeline-detail:" + item.ID)
	applyBtn := a.historyActionButton("timeline-apply:" + item.ID)
	rerunBtn := a.historyActionButton("timeline-rerun:" + item.ID)
	compareBtn := a.historyActionButton("timeline-compare:" + item.ID)
	reuseBtn := a.historyActionButton("timeline-reuse:" + item.ID)
	deleteBtn := a.historyActionButton("timeline-delete:" + item.ID)
	compareActive := compareItemActive(item.ID, compareItemID)
	display := a.historyItemDisplay(item)
	for rowBtn.Clicked(gtx) {
		if err := a.loadHistoryPreview(item, true); err != nil && !isMissingPreview(err) {
			a.appendLog("载入历史结果失败: " + err.Error())
		} else {
			a.closeHistoryTimeline()
		}
	}
	for detailBtn.Clicked(gtx) {
		a.openResultDetail(item)
		a.closeHistoryTimeline()
	}
	for applyBtn.Clicked(gtx) {
		a.applyHistoryParams(item)
		a.closeHistoryTimeline()
	}
	for rerunBtn.Clicked(gtx) {
		a.closeHistoryTimeline()
		a.regenerateFromHistoryItem(item)
	}
	for compareBtn.Clicked(gtx) {
		if err := a.toggleCompareItem(item); err != nil && !isMissingPreview(err) {
			a.appendLog("载入对比图失败: " + err.Error())
		}
	}
	for reuseBtn.Clicked(gtx) {
		a.reuseHistoryItemAsSource(item)
		a.closeHistoryTimeline()
	}
	for deleteBtn.Clicked(gtx) {
		a.deleteHistoryItem(item.ID)
	}
	return a.elevatedSurfaceButton(
		gtx,
		rowBtn,
		chooseColor(active || compareActive, fluent.surface2, fluent.surfaceElevated),
		fluent.surface2,
		chooseColor(active || compareActive, accentAlpha(0x48), fluent.border),
		fluentCardRadius,
		image.Pt(0, 1),
		layout.Inset{Top: 10, Bottom: 10, Left: 10, Right: 10},
		func(gtx layout.Context) layout.Dimensions {
			img, imgOp := a.displayHistoryThumb(item, max(gtx.Dp(unit.Dp(152)), gtx.Dp(unit.Dp(114))))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutHistoryThumbWithCompare(gtx, img, imgOp, item.Mode, unit.Dp(152), unit.Dp(114), compareActive)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return fixedWidth(gtx, unit.Dp(13), func(gtx layout.Context) layout.Dimensions {
										return fixedHeight(gtx, unit.Dp(13), func(gtx layout.Context) layout.Dimensions {
											return uiIconHistory.Layout(gtx, fluent.textDim)
										})
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.singleLineLabel(gtx, display.Clock, unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return a.metaBadgeRow(gtx, display.MetaBadges, true)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if !compareActive {
										return layout.Dimensions{}
									}
									return a.historyCompareBadge(gtx)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.clampedLabel(gtx, display.ShortPrompt, unit.Sp(13), fluent.text, font.Medium, 2)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if strings.TrimSpace(item.RevisedPrompt) == "" {
								return layout.Dimensions{}
							}
							return a.borderedSurface(gtx, fluent.surface2, unit.Dp(6), fluent.border, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, "优化后", unit.Sp(10), fluent.textMuted, font.SemiBold)
										}),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.clampedLabel(gtx, strings.TrimSpace(item.RevisedPrompt), unit.Sp(10), fluent.textDim, font.Normal, 2)
										}),
									)
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return fixedWidth(gtx, unit.Dp(13), func(gtx layout.Context) layout.Dimensions {
												return fixedHeight(gtx, unit.Dp(13), func(gtx layout.Context) layout.Dimensions {
													return uiIconMoreHoriz.Layout(gtx, fluent.textDim)
												})
											})
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.singleLineLabel(gtx, "更多操作", unit.Sp(10), fluent.textDim, font.Normal)
										}),
									)
								}),
								layout.Flexed(1, layout.Spacer{}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.singleLineLabel(gtx, "双击设为源图", unit.Sp(10), fluent.textDim, font.Normal)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.timelineActionButton(gtx, detailBtn, "查看", false)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.timelineActionButton(gtx, applyBtn, "应用参数", false)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.timelineActionButton(gtx, rerunBtn, "重新生成", false)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.timelineActionButton(gtx, reuseBtn, "设为源图", false)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.compactButton(gtx, compareBtn, "对比", compareActive)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.compactButton(gtx, deleteBtn, "删除", false)
										}),
									)
								}),
							)
						}),
					)
				}),
			)
		},
	)
}
