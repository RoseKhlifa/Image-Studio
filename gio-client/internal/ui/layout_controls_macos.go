package ui

import (
	"strings"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

func (a *App) layoutMacControls(gtx layout.Context, snap snapshot) layout.Dimensions {
	trimmedPrompt, promptLen := a.promptInputMetrics()
	ready := strings.TrimSpace(a.apiKeyInput.Text()) != "" && strings.TrimSpace(a.baseURLInput.Text()) != ""

	return a.borderedSurface(gtx, fluent.sidebar, 0, fluent.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Inset{Top: 10, Bottom: 10, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Bottom: unit.Dp(8), Left: unit.Dp(2), Right: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "生成设置", unit.Sp(13), fluent.text, font.SemiBold)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, a.modeLabel(), unit.Sp(11), fluent.textMuted, font.Normal)
							}),
						)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.controlsList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
						children := []layout.FlexChild{
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.layoutUpstreamCard(gtx, snap)
							}),
						}
						if strings.TrimSpace(snap.LastErrorMessage) != "" {
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.layoutErrorNoticeCard(gtx, snap)
							}))
						}
						children = append(children,
							layout.Rigid(a.layoutModeCard),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.layoutPromptCard(gtx, snap, promptLen)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.layoutComposeCard(gtx, snap)
							}),
							layout.Rigid(a.layoutAdvancedLauncherCard),
							layout.Rigid(a.layoutLoopLauncherCard),
						)
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, children...)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.layoutActions(gtx, snap, ready, trimmedPrompt != "")
					})
				}),
			)
		})
	})
}

func (a *App) layoutMacAdvancedLauncherRow(gtx layout.Context, summary string) layout.Dimensions {
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		return a.surfaceButton(
			gtx,
			&a.advancedToggleButton,
			rgba(0xffffff, 0x00),
			fluent.toolHoverBg,
			rgba(0xffffff, 0x00),
			fluentControlRadius,
			layout.Inset{Top: 4, Bottom: 4, Left: 2, Right: 2},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(15), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(15), func(gtx layout.Context) layout.Dimensions {
								return uiIconSettings.Layout(gtx, fluent.textMuted)
							})
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, "高级参数", unit.Sp(12), fluent.text, font.Medium)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, summary, unit.Sp(11), fluent.textMuted, font.Normal)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
								return uiIconChevronRight.Layout(gtx, fluent.textDim)
							})
						})
					}),
				)
			},
		)
	})
}

func (a *App) layoutMacLoopLauncherRow(gtx layout.Context, summary string) layout.Dimensions {
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.surfaceButton(
					gtx,
					&a.openLoopModalButton,
					rgba(0xffffff, 0x00),
					fluent.toolHoverBg,
					rgba(0xffffff, 0x00),
					fluentControlRadius,
					layout.Inset{Top: 4, Bottom: 4, Left: 2, Right: 2},
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(15), func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, unit.Dp(15), func(gtx layout.Context) layout.Dimensions {
										return uiIconRefresh.Layout(gtx, chooseColor(a.loopEnabled, fluent.accent, fluent.textMuted))
									})
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, "循环出图", unit.Sp(12), fluent.text, font.Medium)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, summary, unit.Sp(11), fluent.textMuted, font.Normal)
							}),
						)
					},
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutMacSwitch(gtx, &a.toggleLoopEnabledButton, a.loopEnabled, "循环出图")
			}),
		)
	})
}

func (a *App) layoutMacSwitch(gtx layout.Context, button *widget.Clickable, checked bool, label string) layout.Dimensions {
	track := fluent.border2
	thumbAlignment := layout.W
	if checked {
		track = fluent.accent
		thumbAlignment = layout.E
	}
	return button.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp(label).Add(gtx.Ops)
		semantic.SelectedOp(checked).Add(gtx.Ops)
		return fixedWidth(gtx, unit.Dp(34), func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, unit.Dp(20), func(gtx layout.Context) layout.Dimensions {
				return layout.Stack{}.Layout(gtx,
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return a.surface(gtx, track, unit.Dp(10), layout.Spacer{}.Layout)
					}),
					layout.Stacked(func(gtx layout.Context) layout.Dimensions {
						return thumbAlignment.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(2)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
										return a.surface(gtx, fluent.white, unit.Dp(8), layout.Spacer{}.Layout)
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
