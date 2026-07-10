package ui

import (
	"image"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sharedCompat "image-studio/shared/compat"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/yuanhua/image-gptcodex/pkg/client"
)

func (a *App) layoutHistoryAndLogs(gtx layout.Context, snap snapshot) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingHistoryRail, time.Now())
	clearCompareBtn := a.historyActionButton("clear-compare-rail")
	for a.historyCollapseButton.Clicked(gtx) {
		a.historyRailCollapsed = !a.historyRailCollapsed
	}
	for a.openHistoryTimelineButton.Clicked(gtx) {
		a.openHistoryTimeline()
	}
	for a.openHistoryTimelineMoreButton.Clicked(gtx) {
		a.openHistoryTimeline()
	}
	for clearCompareBtn.Clicked(gtx) {
		a.clearCompare()
	}
	for idx, value := range []string{"all", "generate", "edit"} {
		for a.historyModeButtons[idx].Clicked(gtx) {
			a.historyModeFilter = value
		}
	}
	for idx, value := range []string{"all", "today", "week"} {
		for a.historyDateButtons[idx].Clicked(gtx) {
			a.historyDateFilter = value
		}
	}

	data := a.historyPanelData(snap.History)
	entries := data.entries
	filteredCount := data.filteredCount
	generateCount, editCount := data.generateCount, data.editCount
	latest, hasLatest := data.latest, data.hasLatest
	compareItemID := snap.Compare.Item.ID
	visible := entries
	if len(visible) > 18 {
		visible = visible[:18]
	}

	for _, profile := range snap.Profiles {
		button := a.profileButton("profile:" + profile.ID)
		for button.Clicked(gtx) {
			a.switchActiveProfile(profile.ID)
		}
	}
	for _, entry := range visible {
		if entry.Kind != "item" {
			continue
		}
		button := a.historyButton("row:" + entry.Item.ID)
		for {
			click, ok := button.Update(gtx)
			if !ok {
				break
			}
			a.handleHistoryItemClick(click, *entry.Item, false)
		}
	}
	for _, entry := range visible {
		if entry.Kind != "item" {
			continue
		}
		item := *entry.Item
		detailBtn := a.historyActionButton("row-detail:" + item.ID)
		applyBtn := a.historyActionButton("row-apply:" + item.ID)
		rerunBtn := a.historyActionButton("row-rerun:" + item.ID)
		reuseBtn := a.historyActionButton("row-reuse:" + item.ID)
		for detailBtn.Clicked(gtx) {
			a.openHistoryActionMenu(item, "history")
		}
		for applyBtn.Clicked(gtx) {
			a.applyHistoryParams(item)
		}
		for rerunBtn.Clicked(gtx) {
			a.regenerateFromHistoryItem(item)
		}
		for reuseBtn.Clicked(gtx) {
			a.reuseHistoryItemAsSource(item)
			a.appendLog("已将历史结果加入源图: " + shortPrompt(item.Prompt))
		}
		compareBtn := a.historyActionButton("row-compare:" + item.ID)
		for compareBtn.Clicked(gtx) {
			if err := a.toggleCompareItem(item); err != nil && !isMissingPreview(err) {
				a.appendLog("载入对比图失败: " + err.Error())
			}
		}
		deleteBtn := a.historyActionButton("row-delete:" + item.ID)
		for deleteBtn.Clicked(gtx) {
			a.deleteHistoryItem(item.ID)
		}
	}

	return a.borderedSurface(gtx, fluent.inspector, unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Inset{Top: 12, Bottom: 12, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutUpstreamCard(gtx, snap)
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutHistorySummaryCard(gtx, snap, filteredCount, generateCount, editCount)
				}),
			}

			if snap.Compare.HasItem {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.compactIconTextButton(gtx, clearCompareBtn, uiIconCompare, "退出对比", true)
					}),
				)
			}

			if !a.historyRailCollapsed && hasLatest {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.layoutLatestHistoryCard(gtx, latest, snap.SelectedHistoryID == latest.ID, compareItemID)
					}),
				)
			}

			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout))
			if !a.historyRailCollapsed && len(visible) > 0 {
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.layoutHistoryResultsCard(gtx, snap, filteredCount, entries, visible, compareItemID)
				}))
			} else {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if a.historyRailCollapsed {
						return layout.Dimensions{}
					}
					return a.layoutHistoryResultsCard(gtx, snap, filteredCount, entries, visible, compareItemID)
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (a *App) handleHistoryItemClick(click widget.Click, item sharedCompat.HistoryItem, closeTimeline bool) {
	if click.NumClicks >= 2 {
		a.reuseHistoryItemAsSource(item)
		a.appendLog("已将历史结果加入源图: " + shortPrompt(item.Prompt))
		if closeTimeline {
			a.closeHistoryTimeline()
		}
		return
	}
	if click.Modifiers.Contain(key.ModShift) {
		if err := a.toggleCompareItem(item); err != nil && !isMissingPreview(err) {
			a.appendLog("载入对比图失败: " + err.Error())
			return
		}
		if closeTimeline {
			a.closeHistoryTimeline()
		}
		return
	}
	if err := a.loadHistoryPreview(item, true); err != nil && !isMissingPreview(err) {
		a.appendLog("载入历史结果失败: " + err.Error())
		return
	}
	if closeTimeline {
		a.closeHistoryTimeline()
	}
}

func (a *App) layoutUpstreamCard(gtx layout.Context, snap snapshot) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingUpstreamCard, time.Now())
	for a.upstreamConfigButton.Clicked(gtx) {
		a.openSettingsModal()
	}
	for a.profilePickerButton.Clicked(gtx) {
		a.profilePickerOpen = !a.profilePickerOpen
	}
	for a.testUpstreamButton.Clicked(gtx) {
		a.startUpstreamProbe()
	}

	activeName := strings.TrimSpace(activeProfileName(snap.Profiles, snap.ActiveProfileID))
	if activeName == "" {
		activeName = "还没有上游配置"
	}
	activeMode := activeProfileAPIMode(snap.Profiles, snap.ActiveProfileID)
	if activeMode == "" {
		activeMode = a.api
	}
	apiModeLabel := "Responses API"
	if activeMode == string(client.APIModeImages) {
		apiModeLabel = "Images API"
	}
	ready := strings.TrimSpace(a.apiKeyInput.Text()) != "" && strings.TrimSpace(a.baseURLInput.Text()) != ""
	statusLabel := "未配置"
	statusColor := fluent.danger
	dotColor := fluent.danger
	if ready {
		statusLabel = "已配置"
		statusColor = fluent.accent
		dotColor = fluent.accent
	}

	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			base := func(gtx layout.Context) layout.Dimensions {
				children := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.sectionEyebrow(gtx, "上游")
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return fixedWidth(gtx, unit.Dp(7), func(gtx layout.Context) layout.Dimensions {
											return fixedHeight(gtx, unit.Dp(7), func(gtx layout.Context) layout.Dimensions {
												return a.surface(gtx, dotColor, unit.Dp(4), layout.Spacer{}.Layout)
											})
										})
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, statusLabel, unit.Sp(11), statusColor, font.Medium)
									}),
								)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, "当前连接", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
						)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
				}

				if len(snap.Profiles) == 0 {
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "还没有上游配置，先建一条再开始生成。", unit.Sp(11), fluent.textMuted, font.Normal)
					}))
				} else {
					children = append(children,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.surfaceButton(
								gtx,
								&a.profilePickerButton,
								chooseColor(a.profilePickerOpen, fluent.surface2, fluent.surface),
								fluent.surface2,
								fluent.border,
								fluentBadgeRadius,
								layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
								func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.singleLineLabel(gtx, activeName+" · "+apiModeLabel, unit.Sp(12), fluent.text, font.Medium)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											icon := uiIconExpand
											if a.profilePickerOpen {
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
						}),
					)
				}

				children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout))
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.compactButton(gtx, &a.upstreamConfigButton, "上游配置", false)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(104), func(gtx layout.Context) layout.Dimensions {
								label := "测试"
								if snap.TestingUpstream {
									label = "检查中..."
								}
								return a.compactButton(gtx, &a.testUpstreamButton, label, !snap.TestingUpstream && strings.TrimSpace(snap.LastProbeSummary) != "")
							})
						}),
					)
				}))
				children = append(children,
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, apiModeLabel, unit.Sp(11), fluent.textDim, font.Normal)
					}),
				)
				if strings.TrimSpace(snap.LastProbeSummary) != "" {
					children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, snap.LastProbeSummary, unit.Sp(10), fluent.textDim, font.Normal)
					}))
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			}

			if !a.profilePickerOpen || len(snap.Profiles) == 0 {
				return base(gtx)
			}
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(base),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					macro := op.Record(gtx.Ops)
					overlayDims := layout.Inset{Top: unit.Dp(46)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.layoutProfilePickerPopup(gtx, snap)
					})
					call := macro.Stop()
					call.Add(gtx.Ops)
					return overlayDims
				}),
			)
		})
	})
}

func (a *App) layoutProfilePickerPopup(gtx layout.Context, snap snapshot) layout.Dimensions {
	return a.elevatedBorderedSurface(gtx, fluent.surface, fluentControlRadius, fluent.border, image.Pt(0, 2), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			rows := make([]layout.FlexChild, 0, len(snap.Profiles)+2)
			for idx, profile := range snap.Profiles {
				profile := profile
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutProfileOption(gtx, profile, profile.ID == snap.ActiveProfileID)
				}))
				if idx != len(snap.Profiles)-1 {
					rows = append(rows, layout.Rigid(layout.Spacer{Height: unit.Dp(2)}.Layout))
				}
			}
			if len(snap.Profiles) > 0 {
				rows = append(rows,
					layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.surfaceButton(
							gtx,
							&a.upstreamConfigButton,
							rgba(0xffffff, 0x00),
							fluent.surface2,
							rgba(0xffffff, 0x00),
							unit.Dp(4),
							layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
							func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, "管理配置...", unit.Sp(12), fluent.textMuted, font.Medium)
							},
						)
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		})
	})
}

func (a *App) layoutProfileOption(gtx layout.Context, profile sharedCompat.UpstreamProfile, active bool) layout.Dimensions {
	btn := a.profileButton("profile:" + profile.ID)
	modeLabel := "Responses"
	if strings.TrimSpace(profile.APIMode) == string(client.APIModeImages) {
		modeLabel = "Images"
	}
	return a.surfaceButton(
		gtx,
		btn,
		chooseColor(active, fluent.accentSoft, rgba(0xffffff, 0x00)),
		chooseColor(active, accentAlpha(0x18), fluent.surface2),
		rgba(0xffffff, 0x00),
		unit.Dp(4),
		layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
		func(gtx layout.Context) layout.Dimensions {
			return a.singleLineLabel(gtx, strings.TrimSpace(profile.Name)+" · "+modeLabel, unit.Sp(12), chooseColor(active, fluent.accent, fluent.text), font.Medium)
		},
	)
}

func (a *App) layoutHistorySummaryCard(
	gtx layout.Context,
	snap snapshot,
	filteredCount int,
	generateCount int,
	editCount int,
) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingHistorySummaryCard, time.Now())
	countText := strconv.Itoa(filteredCount)
	if filteredCount != len(snap.History) {
		countText += " / " + strconv.Itoa(len(snap.History))
	}
	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.sectionEyebrow(gtx, "历史")
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, countText+" 项", unit.Sp(11), fluent.textMuted, font.Normal)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							icon := uiIconCollapse
							if a.historyRailCollapsed {
								icon = uiIconExpand
							}
							return a.compactIconTextButton(gtx, &a.historyCollapseButton, icon, chooseHistoryCollapseLabel(a.historyRailCollapsed), a.historyRailCollapsed)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.historyStatButton(gtx, &a.historyModeButtons[0], uiIconList, "全部", strconv.Itoa(len(snap.History)), a.historyModeFilter == "all")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.historyStatButton(gtx, &a.historyModeButtons[1], uiIconPlay, "文生图", strconv.Itoa(generateCount), a.historyModeFilter == "generate")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.historyStatButton(gtx, &a.historyModeButtons[2], uiIconEdit, "图生图", strconv.Itoa(editCount), a.historyModeFilter == "edit")
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.searchField(gtx, &a.historyQueryInput, "搜索 prompt...")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.compactButton(gtx, &a.historyDateButtons[0], "全部", a.historyDateFilter == "all")
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.compactButton(gtx, &a.historyDateButtons[1], "今天", a.historyDateFilter == "today")
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.compactButton(gtx, &a.historyDateButtons[2], "本周", a.historyDateFilter == "week")
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{}
				}),
			)
		})
	})
}

func (a *App) historyStatButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	label string,
	count string,
	active bool,
) layout.Dimensions {
	bg := fluent.surface
	hoverBg := fluent.surface2
	border := fluent.border
	fg := fluent.textMuted
	if active {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accent
	}
	return fixedHeight(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
		return a.surfaceButton(
			gtx,
			btn,
			bg,
			hoverBg,
			border,
			fluentControlRadius,
			layout.Inset{Left: 9, Right: 9},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
									return icon.Layout(gtx, fg)
								})
							})
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, label, unit.Sp(11), fg, font.Medium)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, count, unit.Sp(11), fg, font.SemiBold)
						}),
					)
				})
			},
		)
	})
}

func (a *App) layoutLatestHistoryCard(gtx layout.Context, item sharedCompat.HistoryItem, active bool, compareItemID string) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingLatestHistoryCard, time.Now())
	btn := a.historyButton("feature:" + item.ID)
	detailBtn := a.historyActionButton("feature-detail:" + item.ID)
	compareActive := compareItemActive(item.ID, compareItemID)
	display := a.historyItemDisplay(item)
	for {
		click, ok := btn.Update(gtx)
		if !ok {
			break
		}
		a.handleHistoryItemClick(click, item, false)
	}
	for detailBtn.Clicked(gtx) {
		a.openHistoryActionMenu(item, "latest")
	}
	return a.historySecondaryMenuSurface(gtx, "feature-secondary:"+item.ID, item, "latest", func(gtx layout.Context) layout.Dimensions {
		return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				img, imgOp := a.displayHistoryThumb(item, gtx.Dp(unit.Dp(88)))
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
											return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
												return uiIconHistory.Layout(gtx, fluent.textMuted)
											})
										})
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.sectionEyebrow(gtx, "最近作品")
									}),
								)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.textActionButton(gtx, &a.openHistoryTimelineButton, "完整历史", true)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.elevatedSurfaceButton(
							gtx,
							btn,
							chooseColor(active || compareActive, fluent.surface2, fluent.surfaceElevated),
							fluent.surface2,
							chooseColor(active || compareActive, accentAlpha(0x48), fluent.border),
							unit.Dp(6),
							image.Pt(0, 1),
							layout.Inset{Top: 8, Bottom: 8, Left: 8, Right: 8},
							func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.layoutHistoryThumbWithCompare(gtx, img, imgOp, item.Mode, unit.Dp(88), unit.Dp(88), compareActive)
									}),
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return a.clampedLabel(gtx, display.ShortPrompt, unit.Sp(12), fluent.text, font.Medium, 2)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
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
												return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
													gtx.Constraints.Min.X = 0
													return fixedHeight(gtx, unit.Dp(28), func(gtx layout.Context) layout.Dimensions {
														return a.surfaceButton(
															gtx,
															detailBtn,
															fluent.surface,
															fluent.surface2,
															fluent.border,
															fluentControlRadius,
															layout.Inset{Left: 8, Right: 8},
															func(gtx layout.Context) layout.Dimensions {
																return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx,
																	layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																		return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
																			return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
																				return uiIconMoreHoriz.Layout(gtx, fluent.textMuted)
																			})
																		})
																	}),
																	layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																		return a.label(gtx, "更多", unit.Sp(11), fluent.textMuted, font.Medium)
																	}),
																)
															},
														)
													})
												})
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return a.singleLineLabel(gtx, "双击设为源图，右键或点更多查看完整操作", unit.Sp(10), fluent.textDim, font.Normal)
											}),
										)
									}),
								)
							},
						)
					}),
				)
			})
		})
	})
}

func (a *App) layoutPromptGroupModal(gtx layout.Context, snap snapshot) layout.Dimensions {
	group := snap.ActivePromptGroup
	if group.Key == "" {
		return layout.Dimensions{}
	}
	latest := group.Representative
	currentGroup := historyPromptGroupContains(group, snap.SelectedHistoryID)
	compareGroup := historyPromptGroupContains(group, snap.Compare.Item.ID)
	for a.closePromptGroupButton.Clicked(gtx) {
		a.closePromptGroup()
	}
	latestBtn := a.historyButton("modal-latest:" + latest.ID)
	for {
		_, ok := latestBtn.Update(gtx)
		if !ok {
			break
		}
		if err := a.loadHistoryPreview(latest, true); err != nil && !isMissingPreview(err) {
			a.appendLog("载入历史结果失败: " + err.Error())
		}
	}
	latestDetailBtn := a.historyActionButton("modal-latest-detail:" + latest.ID)
	for latestDetailBtn.Clicked(gtx) {
		a.openHistoryActionMenu(latest, "prompt-group-latest")
	}

	return a.layoutStandardModal(
		gtx,
		unit.Dp(760),
		unit.Dp(560),
		"同提示词历史",
		"",
		&a.closePromptGroupButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					borderColor := chooseColor(currentGroup || compareGroup, accentAlpha(0x48), fluent.border)
					if compareGroup {
						borderColor = accentAlpha(0x64)
					}
					return a.elevatedBorderedSurface(gtx, chooseColor(currentGroup || compareGroup, fluent.surface2, fluent.surfaceElevated), fluentCardRadius, borderColor, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(14)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(unit.Dp(18))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.layoutHistoryGroupPileSized(gtx, group, unit.Dp(150), unit.Dp(112), unit.Dp(126), unit.Dp(96), unit.Dp(8), unit.Dp(8))
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
														return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
															return uiIconGrid.Layout(gtx, fluent.accent)
														})
													})
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.label(gtx, "同提示词", unit.Sp(12), fluent.accent, font.SemiBold)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.singleLineLabel(gtx, group.CountText, unit.Sp(11), fluent.textDim, font.Medium)
												}),
											)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.clampedLabel(gtx, group.Title, unit.Sp(15), fluent.text, font.SemiBold, 2)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.metaBadgeRow(gtx, []string{
												formatHistoryDateTime(latest.CreatedAt),
												sizeDisplayLabel(latest.Size),
												qualityDisplayLabel(latest.Quality),
											}, true)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.primaryButton(gtx, latestBtn, "查看最新", fluent.accent, fluent.white)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.timelineActionButton(gtx, latestDetailBtn, "更多", false)
												}),
											)
										}),
									)
								}),
							)
						})
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					columns := promptGroupModalColumns(gtx)
					rowCount := (len(group.Items) + columns - 1) / columns
					return a.promptGroupList.Layout(gtx, rowCount, func(gtx layout.Context, row int) layout.Dimensions {
						return layout.Inset{Bottom: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.layoutPromptGroupModalGridRow(gtx, group.Items, row, columns, snap.SelectedHistoryID, snap.Compare.Item.ID)
						})
					})
				}),
			)
		},
	)
}

func historyAutoGridColumns(gtx layout.Context, minTile unit.Dp, gap unit.Dp) int {
	available := max(gtx.Constraints.Max.X, 1)
	columns := 3
	minTilePx := gtx.Dp(minTile)
	gapPx := gtx.Dp(gap)
	if available > 0 && minTilePx > 0 {
		columns = max((available+gapPx)/(minTilePx+gapPx), 1)
	}
	return columns
}

func promptGroupModalColumns(gtx layout.Context) int {
	return historyAutoGridColumns(gtx, unit.Dp(118), unit.Dp(10))
}

func (a *App) layoutPromptGroupModalGridRow(gtx layout.Context, items []*sharedCompat.HistoryItem, row int, columns int, selectedHistoryID string, compareItemID string) layout.Dimensions {
	cells := make([]layout.FlexChild, 0, columns)
	for col := 0; col < columns; col++ {
		idx := row*columns + col
		if idx >= len(items) {
			cells = append(cells, layout.Flexed(1, layout.Spacer{}.Layout))
			continue
		}
		item := items[idx]
		cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Right: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.layoutPromptGroupModalTile(gtx, *item, selectedHistoryID == item.ID, compareItemID)
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, cells...)
}

func (a *App) layoutPromptGroupModalTile(gtx layout.Context, item sharedCompat.HistoryItem, active bool, compareItemID string) layout.Dimensions {
	btn := a.historyButton("modal:" + item.ID)
	detailBtn := a.historyActionButton("modal-detail:" + item.ID)
	compareActive := compareItemActive(item.ID, compareItemID)
	for {
		click, ok := btn.Update(gtx)
		if !ok {
			break
		}
		a.handlePromptGroupItemClick(click, item)
	}
	for detailBtn.Clicked(gtx) {
		a.openHistoryActionMenu(item, "prompt-group")
	}
	return a.historySecondaryMenuSurface(gtx, "prompt-group-secondary:"+item.ID, item, "prompt-group", func(gtx layout.Context) layout.Dimensions {
		return a.elevatedSurfaceButton(
			gtx,
			btn,
			chooseColor(active || compareActive, fluent.surface2, fluent.surfaceElevated),
			fluent.surface2,
			chooseColor(active || compareActive, accentAlpha(0x48), fluent.border),
			unit.Dp(8),
			image.Pt(0, 1),
			layout.Inset{},
			func(gtx layout.Context) layout.Dimensions {
				indexLabel := chooseBatchIndexLabel(item.BatchIndex)
				if strings.HasPrefix(indexLabel, "第 ") {
					indexLabel = "#" + strings.TrimSuffix(strings.TrimPrefix(indexLabel, "第 "), " 张")
				}
				side := max(gtx.Constraints.Max.X, gtx.Dp(unit.Dp(118)))
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
						return layout.SW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.surface(gtx, rgba(0x111111, 0xba), unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 2, Bottom: 2, Left: 6, Right: 6}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, indexLabel, unit.Sp(9), fluent.white, font.Medium)
									})
								})
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
						return layout.SE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Right: unit.Dp(6), Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								if !btn.Hovered() {
									return layout.Dimensions{}
								}
								return a.surfaceButton(
									gtx,
									detailBtn,
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
	})
}

func (a *App) handlePromptGroupItemClick(click widget.Click, item sharedCompat.HistoryItem) {
	if click.NumClicks >= 2 {
		a.reuseHistoryItemAsSource(item)
		a.appendLog("已将历史结果加入源图: " + shortPrompt(item.Prompt))
		return
	}
	if click.Modifiers.Contain(key.ModShift) {
		if err := a.toggleCompareItem(item); err != nil && !isMissingPreview(err) {
			a.appendLog("载入对比图失败: " + err.Error())
			return
		}
		return
	}
	if err := a.loadHistoryPreview(item, true); err != nil && !isMissingPreview(err) {
		a.appendLog("载入历史结果失败: " + err.Error())
		return
	}
}

func (a *App) layoutHistoryResultsCard(
	gtx layout.Context,
	snap snapshot,
	filteredCount int,
	entries []historyPromptEntry,
	visible []historyPromptEntry,
	compareItemID string,
) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingHistoryResultsCard, time.Now())
	selectedGroupKey := promptGroupKeyForEntries(entries, snap.SelectedHistoryID)
	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
										return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
											return uiIconList.Layout(gtx, fluent.accent)
										})
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.sectionEyebrow(gtx, "结果")
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := strconv.Itoa(len(visible))
							if len(entries) > len(visible) {
								label += " / " + strconv.Itoa(len(entries))
							}
							return a.label(gtx, label, unit.Sp(11), fluent.textMuted, font.Normal)
						}),
					)
				}),
				func() layout.FlexChild {
					if len(visible) == 0 {
						return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							text := "还没有结果"
							if filteredCount == 0 && len(snap.History) > 0 {
								text = "没有匹配项"
							}
							return a.emptyPanel(gtx, text)
						})
					}
					return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.historyList.Layout(gtx, len(visible), func(gtx layout.Context, i int) layout.Dimensions {
							entry := visible[i]
							return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								if entry.Kind == "group" {
									return a.layoutHistoryGroupRow(gtx, *entry.Group, selectedGroupKey, compareItemID)
								}
								return a.layoutHistoryRow(gtx, *entry.Item, entry.Item.ID == snap.SelectedHistoryID, compareItemID)
							})
						})
					})
				}(),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if len(entries) <= len(visible) {
						return layout.Dimensions{}
					}
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return a.compactButton(gtx, &a.openHistoryTimelineMoreButton, "查看更多历史", false)
				}),
			)
		})
	})
}

func (a *App) layoutHistoryGroupRow(gtx layout.Context, group historyPromptGroup, selectedGroupKey string, compareItemID string) layout.Dimensions {
	active := group.Key != "" && group.Key == selectedGroupKey
	summaryBtn := a.historyButton("group:" + group.Key)
	pileBtn := a.historyButton("pile:" + group.Key)
	expandBtn := a.historyButton("expand:" + group.Key)
	compareActive := historyPromptGroupContains(group, compareItemID)
	label := choosePromptGroupTitle(group)
	meta := joinHistoryMetaParts(group.CountText, sizeDisplayLabel(group.Representative.Size), qualityDisplayLabel(group.Representative.Quality))
	for {
		click, ok := summaryBtn.Update(gtx)
		if !ok {
			break
		}
		a.handleHistoryGroupSummaryClick(click, group, false)
	}
	for pileBtn.Clicked(gtx) {
		a.openPromptGroup(group)
	}
	for expandBtn.Clicked(gtx) {
		a.openPromptGroup(group)
	}

	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, unit.Dp(6), chooseColor(active || compareActive, accentAlpha(0x48), fluent.border), image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(7)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.historySecondaryMenuSurface(gtx, "group-summary-secondary:"+group.Key, group.Representative, "history", func(gtx layout.Context) layout.Dimensions {
						return summaryBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.historySecondaryMenuSurface(gtx, "group-pile-secondary:"+group.Key, group.Representative, "history", func(gtx layout.Context) layout.Dimensions {
										return pileBtn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return a.layoutHistoryResultGroupThumb(gtx, group)
										})
									})
								}),
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.singleLineLabel(gtx, label, unit.Sp(12), fluent.text, font.SemiBold)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.singleLineLabel(gtx, meta, unit.Sp(11), fluent.textMuted, font.Normal)
										}),
									)
								}),
							)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.historyMiniIconButton(gtx, expandBtn, uiIconGrid, false)
				}),
			)
		})
	})
}

func (a *App) handleHistoryGroupSummaryClick(click widget.Click, group historyPromptGroup, closeTimeline bool) {
	if click.Modifiers.Contain(key.ModShift) {
		if historyPromptGroupContains(group, a.readSnapshot().Compare.Item.ID) {
			a.clearCompare()
		} else if err := a.toggleCompareItem(group.Representative); err != nil && !isMissingPreview(err) {
			a.appendLog("载入对比图失败: " + err.Error())
			return
		}
		if closeTimeline {
			a.closeHistoryTimeline()
		}
		return
	}
	if err := a.loadHistoryPreview(group.Representative, true); err != nil && !isMissingPreview(err) {
		a.appendLog("载入历史结果失败: " + err.Error())
		return
	}
	if closeTimeline {
		a.closeHistoryTimeline()
	}
}

func (a *App) layoutHistoryGroupPile(gtx layout.Context, group historyPromptGroup) layout.Dimensions {
	return a.layoutHistoryGroupPileSized(gtx, group, unit.Dp(58), unit.Dp(44), unit.Dp(45), unit.Dp(36), unit.Dp(4), unit.Dp(4))
}

func (a *App) layoutHistoryResultGroupThumb(gtx layout.Context, group historyPromptGroup) layout.Dimensions {
	return a.layoutHistoryGroupPile(gtx, group)
}

func (a *App) layoutHistoryPileLayer(gtx layout.Context, img image.Image, imgOp paint.ImageOp, width unit.Dp, height unit.Dp, radius unit.Dp) layout.Dimensions {
	border := withAlpha(fluent.white, 0xdc)
	bg := rgb(0xf4f4f5)
	if a.isDarkTheme() {
		border = withAlpha(fluent.white, 0x29)
		bg = rgb(0x27272a)
	}
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
			return a.elevatedBorderedSurface(gtx, bg, radius, border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
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

func (a *App) layoutHistoryGroupPileSized(
	gtx layout.Context,
	group historyPromptGroup,
	frameWidth unit.Dp,
	frameHeight unit.Dp,
	thumbWidth unit.Dp,
	thumbHeight unit.Dp,
	offsetX unit.Dp,
	offsetY unit.Dp,
) layout.Dimensions {
	return fixedWidth(gtx, frameWidth, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, frameHeight, func(gtx layout.Context) layout.Dimensions {
			maxThumbs := min(3, len(group.Items))
			compact := frameWidth <= unit.Dp(60)
			baseInset := image.Pt(gtx.Dp(offsetX), gtx.Dp(offsetY))
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
			layerRadius := unit.Dp(14)
			modeInset := image.Pt(7, 7)
			countInset := image.Pt(2, 2)
			countMinWidth := unit.Dp(30)
			countHeight := unit.Dp(24)
			countSize := unit.Sp(11)
			countPadding := layout.Inset{Top: 5, Bottom: 5, Left: 8, Right: 8}
			if compact {
				offsets = []image.Point{
					image.Pt(0, 0),
					image.Pt(5, 0),
					image.Pt(9, 1),
				}
				angles = []float32{
					float32(-1 * math.Pi / 180),
					float32(3 * math.Pi / 180),
					float32(6 * math.Pi / 180),
				}
				layerRadius = unit.Dp(5)
				modeInset = image.Pt(4, 4)
				countInset = image.Pt(0, 0)
				countMinWidth = unit.Dp(18)
				countHeight = unit.Dp(16)
				countSize = unit.Sp(9)
				countPadding = layout.Inset{Top: 2, Bottom: 2, Left: 5, Right: 5}
			}
			stackClip := clip.RRect{
				Rect: image.Rectangle{Max: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)},
				NW:   gtx.Dp(unit.Dp(5)),
				NE:   gtx.Dp(unit.Dp(5)),
				SW:   gtx.Dp(unit.Dp(5)),
				SE:   gtx.Dp(unit.Dp(5)),
			}.Push(gtx.Ops)
			dims := layout.Stack{}.Layout(gtx,
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, gtx.Constraints.Min.Y)}
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					for idx := maxThumbs - 1; idx >= 0; idx-- {
						item := group.Items[idx]
						img, imgOp := a.displayHistoryThumb(*item, max(gtx.Dp(thumbWidth), gtx.Dp(thumbHeight)))
						offset := offsets[min(idx, len(offsets)-1)]
						offset = image.Pt(baseInset.X+offset.X, baseInset.Y+offset.Y)
						layout.Inset{
							Left: unit.Dp(float32(offset.X)),
							Top:  unit.Dp(float32(offset.Y)),
						}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							opacity := []float32{1, 0.86, 0.72}[min(idx, 2)]
							opacityStack := paint.PushOpacity(gtx.Ops, opacity)
							origin := f32.Pt(float32(gtx.Dp(thumbWidth)/2), float32(gtx.Dp(thumbHeight)/2))
							transform := f32.AffineId().
								Scale(origin, f32.Pt(scales[min(idx, len(scales)-1)], scales[min(idx, len(scales)-1)])).
								Rotate(origin, angles[min(idx, len(angles)-1)])
							stack := op.Affine(transform).Push(gtx.Ops)
							dims := a.layoutHistoryPileLayer(gtx, img, imgOp, thumbWidth, thumbHeight, layerRadius)
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
						return layout.Inset{Left: unit.Dp(float32(modeInset.X)), Top: unit.Dp(float32(modeInset.Y))}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.historyModeBadge(gtx, group.Representative.Mode)
						})
					})
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					return layout.SE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Right: unit.Dp(float32(countInset.X)), Bottom: unit.Dp(float32(countInset.Y))}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, countMinWidth, func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, countHeight, func(gtx layout.Context) layout.Dimensions {
									return a.elevatedBorderedSurface(gtx, fluent.accent, unit.Dp(999), withAlpha(fluent.white, 0xd8), image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
										return countPadding.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return a.label(gtx, group.CountValue, countSize, fluent.white, font.SemiBold)
											})
										})
									})
								})
							})
						})
					})
				}),
			)
			stackClip.Pop()
			return dims
		})
	})
}

func (a *App) layoutHistoryModeThumb(gtx layout.Context, img image.Image, imgOp paint.ImageOp, mode string, width unit.Dp, height unit.Dp) layout.Dimensions {
	return a.layoutHistoryThumbWithCompare(gtx, img, imgOp, mode, width, height, false)
}

func (a *App) layoutHistoryThumbWithCompare(gtx layout.Context, img image.Image, imgOp paint.ImageOp, mode string, width unit.Dp, height unit.Dp, compareActive bool) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return a.imageThumbCoverWithOp(gtx, img, imgOp, width, height, unit.Dp(4))
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.NW.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(4), Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.historyModeBadge(gtx, mode)
				})
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if !compareActive {
				return layout.Dimensions{}
			}
			return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(4), Right: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.historyCompareBadge(gtx)
				})
			})
		}),
	)
}

func (a *App) historyModeBadge(gtx layout.Context, mode string) layout.Dimensions {
	label := "文生图"
	if mode == "edit" {
		label = "图生图"
	}
	return a.surface(gtx, rgba(0x000000, 0x75), fluentControlRadius, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 2, Bottom: 2, Left: 5, Right: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, label, unit.Sp(9), fluent.white, font.Medium)
		})
	})
}

func (a *App) historyCompareBadge(gtx layout.Context) layout.Dimensions {
	return a.surface(gtx, rgb(0x2563eb), unit.Dp(4), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 2, Bottom: 2, Left: 5, Right: 5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, "B", unit.Sp(9), fluent.white, font.SemiBold)
		})
	})
}

func (a *App) layoutHistoryRow(gtx layout.Context, item sharedCompat.HistoryItem, active bool, compareItemID string) layout.Dimensions {
	btn := a.historyButton("row:" + item.ID)
	detailBtn := a.historyActionButton("row-detail:" + item.ID)
	compareActive := compareItemActive(item.ID, compareItemID)
	display := a.historyItemDisplay(item)
	return a.historySecondaryMenuSurface(gtx, "history-row-secondary:"+item.ID, item, "history", func(gtx layout.Context) layout.Dimensions {
		return a.elevatedSurfaceButton(
			gtx,
			btn,
			fluent.surfaceElevated,
			fluent.surface2,
			chooseColor(active || compareActive, accentAlpha(0x48), fluent.border),
			unit.Dp(6),
			image.Pt(0, 1),
			layout.Inset{Top: 7, Bottom: 7, Left: 7, Right: 7},
			func(gtx layout.Context) layout.Dimensions {
				img, imgOp := a.displayHistoryThumb(item, gtx.Dp(unit.Dp(48)))
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.imageThumbCoverWithOp(gtx, img, imgOp, unit.Dp(48), unit.Dp(48), unit.Dp(4))
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.clampedLabel(gtx, display.ShortPrompt, unit.Sp(12), fluent.text, font.Medium, 2)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.historyModeBadge(gtx, item.Mode)
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
								return a.singleLineLabel(gtx, "双击设为源图，右键或点更多查看完整操作", unit.Sp(10), fluent.textDim, font.Normal)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.historyRailIconButton(gtx, detailBtn, uiIconMoreHoriz, false)
					}),
				)
			},
		)
	})
}

func chooseCompareButtonLabel(active bool) string {
	if active {
		return "退出对比"
	}
	return "对比"
}

func (a *App) layoutLogsCard(gtx layout.Context, snap snapshot) layout.Dimensions {
	for a.openLogsRawResponseButton.Clicked(gtx) {
		raw := strings.TrimSpace(snap.Result.RawPath)
		if raw == "" {
			continue
		}
		a.openRawResponseModal(raw)
	}
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.sectionEyebrow(gtx, "运行日志")
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if strings.TrimSpace(snap.Result.RawPath) == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.textActionButton(gtx, &a.openLogsRawResponseButton, "查看日志", true)
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.compactButton(gtx, &a.clearLogButton, "清空", false)
					}),
				)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if len(snap.Logs) == 0 {
					return a.emptyPanel(gtx, "暂无日志")
				}
				return a.logList.Layout(gtx, len(snap.Logs), func(gtx layout.Context, i int) layout.Dimensions {
					idx := len(snap.Logs) - 1 - i
					line := snap.Logs[idx]
					return layout.Inset{Bottom: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.borderedSurface(gtx, fluent.surface, unit.Dp(8), fluent.border, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, line, unit.Sp(10), fluent.textMuted, font.Normal)
							})
						})
					})
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				raw := strings.TrimSpace(snap.Result.RawPath)
				if raw == "" {
					return a.label(gtx, "Raw response: 暂无", unit.Sp(10), fluent.textDim, font.Normal)
				}
				return a.borderedSurface(gtx, fluent.surface2, unit.Dp(8), fluent.border, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, raw, unit.Sp(10), fluent.textDim, font.Normal)
					})
				})
			}),
		)
	})
}

func (a *App) emptyPanel(gtx layout.Context, text string) layout.Dimensions {
	return fixedHeight(gtx, unit.Dp(64), func(gtx layout.Context) layout.Dimensions {
		return a.borderedSurface(gtx, withAlpha(fluent.surface2, 0xd8), unit.Dp(6), withAlpha(fluent.border, 0xd8), func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, text, unit.Sp(11), fluent.textMuted, font.Normal)
				})
			})
		})
	})
}

func chooseHistoryCollapseLabel(collapsed bool) string {
	if collapsed {
		return "展开"
	}
	return "折叠"
}

func choosePromptGroupTitle(group historyPromptGroup) string {
	if strings.TrimSpace(group.Title) != "" {
		return group.Title
	}
	if strings.TrimSpace(group.Prompt) == "" {
		return "(无 prompt)"
	}
	return group.Prompt
}

func chooseBatchIndexLabel(batchIndex int) string {
	if batchIndex < 0 {
		return "历史结果"
	}
	return "第 " + strconv.Itoa(batchIndex+1) + " 张"
}

func historyMetaText(item sharedCompat.HistoryItem) string {
	mode := "文生图"
	if item.Mode == "edit" {
		mode = "图生图"
	}
	format := strings.ToUpper(strings.TrimSpace(item.OutputFormat))
	style := ""
	if strings.TrimSpace(item.StyleTag) != "" {
		style = "#" + styleChoiceLabel(item.StyleTag)
	}
	return joinHistoryMetaParts(
		mode,
		sizeDisplayLabel(item.Size),
		qualityDisplayLabel(item.Quality),
		style,
		format,
	)
}

func historyRailMetaText(item sharedCompat.HistoryItem) string {
	style := ""
	if strings.TrimSpace(item.StyleTag) != "" {
		style = "#" + styleChoiceLabel(item.StyleTag)
	}
	return joinHistoryMetaParts(
		sizeDisplayLabel(item.Size),
		qualityDisplayLabel(item.Quality),
		style,
	)
}

func historyMetaBadgeItems(item sharedCompat.HistoryItem) []string {
	items := make([]string, 0, 3)
	if size := strings.TrimSpace(sizeDisplayLabel(item.Size)); size != "" {
		items = append(items, size)
	}
	if quality := strings.TrimSpace(qualityDisplayLabel(item.Quality)); quality != "" {
		items = append(items, quality)
	}
	style := ""
	if strings.TrimSpace(item.StyleTag) != "" {
		style = "#" + styleChoiceLabel(item.StyleTag)
	}
	if style != "" {
		items = append(items, style)
	}
	return items
}

func statusBarMetaBadgeItems(item sharedCompat.HistoryItem) []string {
	items := make([]string, 0, 5)
	if size := strings.TrimSpace(sizeDisplayLabel(item.Size)); size != "" {
		items = append(items, size)
	}
	if quality := strings.TrimSpace(qualityDisplayLabel(item.Quality)); quality != "" {
		items = append(items, quality)
	}
	if item.ElapsedSec > 0 {
		items = append(items, detailValue(item.ElapsedSec)+"s")
	}
	if item.Seed != 0 {
		items = append(items, "seed "+detailValue(item.Seed))
	}
	if strings.TrimSpace(item.StyleTag) != "" {
		items = append(items, "#"+styleChoiceLabel(item.StyleTag))
	}
	return items
}

func joinHistoryMetaParts(parts ...string) string {
	total := 0
	count := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		total += len(part)
		count++
	}
	if count == 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(total + (count-1)*len(" · "))
	written := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if written > 0 {
			b.WriteString(" · ")
		}
		b.WriteString(part)
		written++
	}
	return b.String()
}

func historyPathText(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "未登记保存路径"
	}
	return filepath.Base(path)
}

func formatHistoryClock(createdAt int64) string {
	if createdAt <= 0 {
		return ""
	}
	t := time.UnixMilli(createdAt)
	hour, min, _ := t.Clock()
	var buf [5]byte
	buf[0] = byte('0' + hour/10)
	buf[1] = byte('0' + hour%10)
	buf[2] = ':'
	buf[3] = byte('0' + min/10)
	buf[4] = byte('0' + min%10)
	return string(buf[:])
}

func formatHistoryClockPrecise(createdAt int64) string {
	if createdAt <= 0 {
		return ""
	}
	t := time.UnixMilli(createdAt)
	hour, min, sec := t.Clock()
	var buf [8]byte
	buf[0] = byte('0' + hour/10)
	buf[1] = byte('0' + hour%10)
	buf[2] = ':'
	buf[3] = byte('0' + min/10)
	buf[4] = byte('0' + min%10)
	buf[5] = ':'
	buf[6] = byte('0' + sec/10)
	buf[7] = byte('0' + sec%10)
	return string(buf[:])
}

func formatHistoryDateTime(createdAt int64) string {
	if createdAt <= 0 {
		return ""
	}
	t := time.UnixMilli(createdAt)
	year, month, day := t.Date()
	hour, min, _ := t.Clock()
	var buf [16]byte
	buf[0] = byte('0' + (year/1000)%10)
	buf[1] = byte('0' + (year/100)%10)
	buf[2] = byte('0' + (year/10)%10)
	buf[3] = byte('0' + year%10)
	buf[4] = '-'
	mon := int(month)
	buf[5] = byte('0' + mon/10)
	buf[6] = byte('0' + mon%10)
	buf[7] = '-'
	buf[8] = byte('0' + day/10)
	buf[9] = byte('0' + day%10)
	buf[10] = ' '
	buf[11] = byte('0' + hour/10)
	buf[12] = byte('0' + hour%10)
	buf[13] = ':'
	buf[14] = byte('0' + min/10)
	buf[15] = byte('0' + min%10)
	return string(buf[:])
}
