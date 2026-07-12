package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

var (
	fluentControlRadius = unit.Dp(4)
	fluentCardRadius    = unit.Dp(8)
	fluentBadgeRadius   = unit.Dp(4)
	fluentModalRadius   = unit.Dp(8)
	fluentInputRadius   = unit.Dp(4)
)

func installDesktopThemeMetrics(metrics desktopThemeMetrics) {
	fluentControlRadius = metrics.ControlRadius
	fluentCardRadius = metrics.CardRadius
	fluentBadgeRadius = metrics.BadgeRadius
	fluentModalRadius = metrics.ModalRadius
	fluentInputRadius = metrics.InputRadius
}

func (a *App) sectionTitle(gtx layout.Context, text string) layout.Dimensions {
	style := material.Label(a.th, a.scaledSp(unit.Sp(15)), text)
	style.Color = fluent.text
	style.Font.Weight = font.SemiBold
	style.Font.Typeface = desktopTitleTypeface(a.desktopStyle)
	style.WrapPolicy = textWrapWords
	return style.Layout(gtx)
}

func (a *App) sectionEyebrow(gtx layout.Context, text string) layout.Dimensions {
	return a.label(gtx, text, unit.Sp(11), fluent.textMuted, font.SemiBold)
}

func (a *App) titleLabel(gtx layout.Context, text string, size unit.Sp) layout.Dimensions {
	style := material.Label(a.th, a.scaledSp(size), text)
	style.Color = fluent.text
	style.Font.Weight = font.SemiBold
	style.Font.Typeface = desktopTitleTypeface(a.desktopStyle)
	style.WrapPolicy = textWrapWords
	return style.Layout(gtx)
}

func (a *App) button(gtx layout.Context, btn *widget.Clickable, text string, bg color.NRGBA, fg color.NRGBA) layout.Dimensions {
	style := material.Button(a.th, btn, text)
	style.Background = bg
	style.Color = fg
	style.CornerRadius = fluentControlRadius
	style.TextSize = a.scaledSp(unit.Sp(12))
	style.Font.Weight = font.Medium
	style.Inset = layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}
	return style.Layout(gtx)
}

func (a *App) badge(gtx layout.Context, text string, bg color.NRGBA, fg color.NRGBA) layout.Dimensions {
	return a.borderedSurface(gtx, bg, fluentBadgeRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 6, Bottom: 6, Left: 9, Right: 9}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
		})
	})
}

func (a *App) toolPill(gtx layout.Context, text string, active bool) layout.Dimensions {
	bg := fluent.surface
	fg := fluent.textMuted
	if active {
		bg = fluent.accentSoft
		fg = fluent.accentText
	}
	return a.badge(gtx, text, bg, fg)
}

func (a *App) surfaceButton(
	gtx layout.Context,
	btn *widget.Clickable,
	bg color.NRGBA,
	hoverBg color.NRGBA,
	border color.NRGBA,
	radius unit.Dp,
	inset layout.Inset,
	w layout.Widget,
	selected ...bool,
) layout.Dimensions {
	visual := resolveButtonInteractionColors(
		bg,
		hoverBg,
		border,
		fluent.focusRing,
		fluent.text,
		buttonInteractionState{
			Hovered: btn.Hovered(),
			Focused: gtx.Focused(btn),
			Pressed: btn.Pressed(),
		},
	)
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		addOptionalSelectedSemantic(gtx, selected)
		return a.borderedSurface(gtx, visual.Fill, radius, visual.Border, func(gtx layout.Context) layout.Dimensions {
			return inset.Layout(gtx, w)
		})
	})
}

func (a *App) elevatedSurfaceButton(
	gtx layout.Context,
	btn *widget.Clickable,
	bg color.NRGBA,
	hoverBg color.NRGBA,
	border color.NRGBA,
	radius unit.Dp,
	shadowOffset image.Point,
	inset layout.Inset,
	w layout.Widget,
) layout.Dimensions {
	visual := resolveButtonInteractionColors(
		bg,
		hoverBg,
		border,
		fluent.focusRing,
		fluent.text,
		buttonInteractionState{
			Hovered: btn.Hovered(),
			Focused: gtx.Focused(btn),
			Pressed: btn.Pressed(),
		},
	)
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		return a.elevatedBorderedSurface(gtx, visual.Fill, radius, visual.Border, shadowOffset, func(gtx layout.Context) layout.Dimensions {
			return inset.Layout(gtx, w)
		})
	})
}

type buttonInteractionState struct {
	Hovered bool
	Focused bool
	Pressed bool
}

type buttonInteractionColors struct {
	Fill   color.NRGBA
	Border color.NRGBA
}

func resolveButtonInteractionColors(
	background color.NRGBA,
	hoverBackground color.NRGBA,
	border color.NRGBA,
	focusRing color.NRGBA,
	pressedTint color.NRGBA,
	state buttonInteractionState,
) buttonInteractionColors {
	fill := background
	if state.Hovered || state.Focused {
		fill = hoverBackground
	}
	if state.Pressed {
		fill = pressedSurfaceColor(fill, pressedTint)
	}
	if state.Focused {
		border = focusRing
	}
	return buttonInteractionColors{Fill: fill, Border: border}
}

func pressedSurfaceColor(background color.NRGBA, tint color.NRGBA) color.NRGBA {
	const tintWeight = uint32(36)
	const backgroundWeight = uint32(255) - tintWeight
	mix := func(base uint8, overlay uint8) uint8 {
		return uint8((uint32(base)*backgroundWeight + uint32(overlay)*tintWeight + 127) / 255)
	}
	return color.NRGBA{
		R: mix(background.R, tint.R),
		G: mix(background.G, tint.G),
		B: mix(background.B, tint.B),
		A: mix(background.A, tint.A),
	}
}

func (a *App) pillButton(gtx layout.Context, btn *widget.Clickable, text string, active bool) layout.Dimensions {
	bg := fluent.surface
	hoverBg := fluent.surface2
	border := fluent.border
	fg := fluent.textMuted
	if active {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
	}
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		hoverBg,
		border,
		fluentControlRadius,
		layout.Inset{Top: 7, Bottom: 7, Left: 10, Right: 10},
		func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
		},
	)
}

func (a *App) compactButton(gtx layout.Context, btn *widget.Clickable, text string, accent bool, selected ...bool) layout.Dimensions {
	bg := fluent.surface
	hoverBg := fluent.surface2
	fg := fluent.textMuted
	border := fluent.border
	if accent {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
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
					return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
				})
			},
			selected...,
		)
	})
}

func (a *App) textActionButton(gtx layout.Context, btn *widget.Clickable, text string, accent bool) layout.Dimensions {
	bg := rgba(0xffffff, 0x00)
	hoverBg := fluent.toolHoverBg
	fg := fluent.textMuted
	border := rgba(0xffffff, 0x00)
	if btn.Hovered() {
		fg = fluent.toolHoverText
	}
	if accent {
		bg = rgba(0xffffff, 0x00)
		hoverBg = accentAlpha(0x16)
		fg = fluent.accentText
	}
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		hoverBg,
		border,
		fluentControlRadius,
		layout.Inset{Top: 6, Bottom: 6, Left: 6, Right: 6},
		func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
		},
	)
}

func (a *App) headerIconButton(gtx layout.Context, btn *widget.Clickable, text string, active bool) layout.Dimensions {
	bg := rgba(0xffffff, 0x00)
	hoverBg := fluent.toolHoverBg
	fg := fluent.textDim
	border := rgba(0xffffff, 0x00)
	if btn.Hovered() {
		fg = fluent.toolHoverText
	}
	if active {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
		border = fluent.border
	}
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		hoverBg,
		border,
		fluentControlRadius,
		layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8},
		func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, text, unit.Sp(12), fg, font.Medium)
				})
			})
		},
	)
}

func (a *App) headerIconButtonIcon(gtx layout.Context, btn *widget.Clickable, icon *widget.Icon, active bool, semanticName ...string) layout.Dimensions {
	bg := rgba(0xffffff, 0x00)
	hoverBg := fluent.toolHoverBg
	fg := fluent.textDim
	border := rgba(0xffffff, 0x00)
	if btn.Hovered() {
		fg = fluent.toolHoverText
	}
	if active {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
		border = fluent.border
	}
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		hoverBg,
		border,
		fluentControlRadius,
		layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8},
		func(gtx layout.Context) layout.Dimensions {
			if len(semanticName) > 0 {
				if name := strings.TrimSpace(semanticName[0]); name != "" {
					semantic.LabelOp(name).Add(gtx.Ops)
				}
			}
			return fixedWidth(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return icon.Layout(gtx, fg)
					})
				})
			})
		},
	)
}

func (a *App) compactIconTextButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	text string,
	accent bool,
	selected ...bool,
) layout.Dimensions {
	bg := fluent.surface
	hoverBg := fluent.surface2
	fg := fluent.textMuted
	border := fluent.border
	if accent {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
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
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
									return icon.Layout(gtx, fg)
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
						}),
					)
				})
			},
			selected...,
		)
	})
}

func addOptionalSelectedSemantic(gtx layout.Context, selected []bool) {
	if value, present := resolveOptionalSelectedState(selected); present {
		semantic.SelectedOp(value).Add(gtx.Ops)
	}
}

func resolveOptionalSelectedState(selected []bool) (value bool, present bool) {
	if len(selected) == 0 {
		return false, false
	}
	return selected[0], true
}

func (a *App) ghostIconTextButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	text string,
	accent bool,
) layout.Dimensions {
	bg := rgba(0xffffff, 0x00)
	hoverBg := fluent.toolHoverBg
	fg := fluent.textMuted
	border := rgba(0xffffff, 0x00)
	if btn.Hovered() {
		fg = fluent.toolHoverText
	}
	if accent {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
	}
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		hoverBg,
		border,
		fluentControlRadius,
		layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(13), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(13), func(gtx layout.Context) layout.Dimensions {
							return icon.Layout(gtx, fg)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
				}),
			)
		},
	)
}

func (a *App) ghostIconButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	accent bool,
) layout.Dimensions {
	bg := rgba(0xffffff, 0x00)
	hoverBg := fluent.toolHoverBg
	fg := fluent.textDim
	border := rgba(0xffffff, 0x00)
	if btn.Hovered() {
		fg = fluent.toolHoverText
	}
	if accent {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
	}
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		hoverBg,
		border,
		fluentControlRadius,
		layout.Inset{Top: 5, Bottom: 5, Left: 5, Right: 5},
		func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
					return icon.Layout(gtx, fg)
				})
			})
		},
	)
}

func (a *App) toolbarIconButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	active bool,
) layout.Dimensions {
	bg := rgba(0xffffff, 0x00)
	hoverBg := fluent.toolHoverBg
	fg := fluent.textMuted
	border := rgba(0xffffff, 0x00)
	if btn.Hovered() {
		fg = fluent.toolHoverText
	}
	if active {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
		border = accentAlpha(0x24)
	}
	return fixedWidth(gtx, unit.Dp(32), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
			return a.surfaceButton(
				gtx,
				btn,
				bg,
				hoverBg,
				border,
				unit.Dp(4),
				layout.Inset{},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return icon.Layout(gtx, fg)
							})
						})
					})
				},
			)
		})
	})
}

func (a *App) toolbarStaticIcon(
	gtx layout.Context,
	icon *widget.Icon,
	active bool,
	disabled bool,
) layout.Dimensions {
	bg := rgba(0xffffff, 0x00)
	border := rgba(0xffffff, 0x00)
	fg := fluent.textMuted
	if disabled {
		fg = withAlpha(fluent.textDim, 0x8a)
	}
	if active {
		bg = fluent.accentSoft
		border = accentAlpha(0x24)
		fg = fluent.accentText
	}
	return fixedWidth(gtx, unit.Dp(32), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
			return a.borderedSurface(gtx, bg, fluentControlRadius, border, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return icon.Layout(gtx, fg)
						})
					})
				})
			})
		})
	})
}

func (a *App) historyMiniIconButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	active bool,
) layout.Dimensions {
	bg := fluent.surface
	hoverBg := fluent.surface2
	fg := fluent.textMuted
	border := rgba(0xffffff, 0x00)
	if active {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
		border = accentAlpha(0x38)
	}
	return fixedWidth(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
			return a.surfaceButton(
				gtx,
				btn,
				bg,
				hoverBg,
				border,
				unit.Dp(4),
				layout.Inset{},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return icon.Layout(gtx, fg)
							})
						})
					})
				},
			)
		})
	})
}

func (a *App) historyRailIconButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	active bool,
) layout.Dimensions {
	bg := fluent.surface
	hoverBg := fluent.surface2
	fg := fluent.textMuted
	border := fluent.border
	if active {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
		border = accentAlpha(0x38)
	}
	return fixedWidth(gtx, unit.Dp(28), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(28), func(gtx layout.Context) layout.Dimensions {
			return a.surfaceButton(
				gtx,
				btn,
				bg,
				hoverBg,
				border,
				unit.Dp(4),
				layout.Inset{},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return icon.Layout(gtx, fg)
							})
						})
					})
				},
			)
		})
	})
}

func (a *App) timelineActionButton(gtx layout.Context, btn *widget.Clickable, text string, active bool) layout.Dimensions {
	bg := fluent.surface
	hoverBg := fluent.surface2
	border := fluent.border
	fg := fluent.textMuted
	if active {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
		border = accentAlpha(0x38)
	}
	return fixedHeight(gtx, unit.Dp(34), func(gtx layout.Context) layout.Dimensions {
		return a.surfaceButton(
			gtx,
			btn,
			bg,
			hoverBg,
			border,
			fluentControlRadius,
			layout.Inset{Left: 12, Right: 12},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, text, unit.Sp(11), fg, font.SemiBold)
				})
			},
		)
	})
}

func (a *App) timelineActionIconButton(gtx layout.Context, btn *widget.Clickable, icon *widget.Icon) layout.Dimensions {
	return fixedWidth(gtx, unit.Dp(34), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(34), func(gtx layout.Context) layout.Dimensions {
			return a.surfaceButton(
				gtx,
				btn,
				fluent.surface,
				fluent.surface2,
				fluent.border,
				fluentControlRadius,
				layout.Inset{},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return icon.Layout(gtx, fluent.textMuted)
							})
						})
					})
				},
			)
		})
	})
}

func (a *App) pillIconTextButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	text string,
	active bool,
) layout.Dimensions {
	bg := fluent.surface
	hoverBg := fluent.surface2
	border := fluent.border
	fg := fluent.textMuted
	if active {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
	}
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		hoverBg,
		border,
		fluentControlRadius,
		layout.Inset{Top: 7, Bottom: 7, Left: 10, Right: 10},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return icon.Layout(gtx, fg)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
				}),
			)
		},
	)
}

func (a *App) toolbarTextButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	text string,
	selected bool,
) layout.Dimensions {
	bg := rgba(0xffffff, 0x00)
	hoverBg := fluent.toolHoverBg
	border := rgba(0xffffff, 0x00)
	fg := fluent.textMuted
	if btn.Hovered() {
		fg = fluent.toolHoverText
	}
	if selected {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		border = accentAlpha(0x24)
		fg = fluent.accentText
	}
	return fixedHeight(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
		return a.surfaceButton(
			gtx,
			btn,
			bg,
			hoverBg,
			border,
			fluentControlRadius,
			layout.Inset{Left: 8, Right: 8},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
									return icon.Layout(gtx, fg)
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
						}),
					)
				})
			},
		)
	})
}

func (a *App) toolbarStaticTextButton(
	gtx layout.Context,
	text string,
	accent bool,
) layout.Dimensions {
	bg := rgba(0xffffff, 0x00)
	border := rgba(0xffffff, 0x00)
	fg := fluent.textMuted
	if accent {
		bg = fluent.accentSoft
		border = accentAlpha(0x24)
		fg = fluent.accentText
	}
	return fixedHeight(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
		return a.borderedSurface(gtx, bg, fluentControlRadius, border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
				})
			})
		})
	})
}

func (a *App) toolbarColorButton(
	gtx layout.Context,
	btn *widget.Clickable,
	swatch color.NRGBA,
	selected bool,
) layout.Dimensions {
	bg := rgba(0xffffff, 0x00)
	hoverBg := fluent.toolHoverBg
	border := rgba(0xffffff, 0x00)
	if selected {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		border = accentAlpha(0x24)
	}
	return fixedWidth(gtx, unit.Dp(28), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(28), func(gtx layout.Context) layout.Dimensions {
			return a.surfaceButton(
				gtx,
				btn,
				bg,
				hoverBg,
				border,
				fluentControlRadius,
				layout.Inset{},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return a.surface(gtx, swatch, unit.Dp(7), layout.Spacer{}.Layout)
							})
						})
					})
				},
			)
		})
	})
}

func (a *App) compactIconButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	active bool,
) layout.Dimensions {
	bg := fluent.surface
	hoverBg := fluent.surface2
	fg := fluent.textMuted
	border := fluent.border
	if active {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x28)
		fg = fluent.accentText
	}
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		hoverBg,
		border,
		fluentControlRadius,
		layout.Inset{Top: 6, Bottom: 6, Left: 6, Right: 6},
		func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
					return icon.Layout(gtx, fg)
				})
			})
		},
	)
}

func (a *App) primaryIconTextButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	text string,
	bg color.NRGBA,
	fg color.NRGBA,
) layout.Dimensions {
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		chooseColor(bg == fluent.accent, fluent.accent2, fluent.surface2),
		chooseColor(bg == fluent.accent, accentAlpha(0x58), fluent.border),
		fluentControlRadius,
		layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
							return icon.Layout(gtx, fg)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, text, unit.Sp(12), fg, font.Medium)
				}),
			)
		},
	)
}

func (a *App) toolbarPrimaryTextButton(
	gtx layout.Context,
	btn *widget.Clickable,
	icon *widget.Icon,
	text string,
) layout.Dimensions {
	foreground := desktopReadableText(fluent.accent)
	return fixedHeight(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
		return a.surfaceButton(
			gtx,
			btn,
			fluent.accent,
			fluent.accent2,
			accentAlpha(0x58),
			fluentControlRadius,
			layout.Inset{Left: 10, Right: 10},
			func(gtx layout.Context) layout.Dimensions {
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
							return a.label(gtx, text, unit.Sp(11), foreground, font.Medium)
						}),
					)
				})
			},
		)
	})
}

func (a *App) primaryButton(
	gtx layout.Context,
	btn *widget.Clickable,
	text string,
	bg color.NRGBA,
	fg color.NRGBA,
) layout.Dimensions {
	if bg.A == 0xff {
		fg = desktopReadableText(bg)
	}
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		chooseColor(bg == fluent.accent, fluent.accent2, fluent.surface2),
		chooseColor(bg == fluent.accent, accentAlpha(0x58), fluent.border),
		fluentControlRadius,
		layout.Inset{Top: 9, Bottom: 9, Left: 12, Right: 12},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, text, unit.Sp(12), fg, font.SemiBold)
			})
		},
	)
}

func (a *App) staticPill(gtx layout.Context, text string, accent bool, dimmed bool) layout.Dimensions {
	bg := fluent.surface
	fg := fluent.textMuted
	border := fluent.border
	if accent {
		bg = fluent.accentSoft
		fg = fluent.accentText
	}
	if dimmed {
		fg = fluent.textDim
		border = fluent.border2
	}
	return a.borderedSurface(gtx, bg, fluentBadgeRadius, border, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 7, Bottom: 7, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, text, unit.Sp(11), fg, font.Medium)
		})
	})
}

func (a *App) metaBadge(gtx layout.Context, text string, compact bool) layout.Dimensions {
	text = strings.TrimSpace(text)
	if text == "" {
		return layout.Dimensions{}
	}
	bg := rgba(0x000000, 0x06)
	border := rgba(0x000000, 0x0d)
	if a.isDarkTheme() {
		bg = rgba(0xffffff, 0x0d)
		border = rgba(0xffffff, 0x0d)
	}
	size := unit.Sp(11)
	inset := layout.Inset{Top: 3, Bottom: 3, Left: 8, Right: 8}
	if compact {
		size = unit.Sp(10)
		inset = layout.Inset{Top: 2, Bottom: 2, Left: 7, Right: 7}
	}
	return a.borderedSurface(gtx, bg, unit.Dp(999), border, func(gtx layout.Context) layout.Dimensions {
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, text, size, fluent.textMuted, font.Normal)
		})
	})
}

func (a *App) metaBadgeRow(gtx layout.Context, items []string, compact bool) layout.Dimensions {
	visibleCount := 0
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			visibleCount++
		}
	}
	if visibleCount == 0 {
		return layout.Dimensions{}
	}
	children := make([]layout.FlexChild, 0, visibleCount*2)
	gap := unit.Dp(6)
	if compact {
		gap = unit.Dp(4)
	}
	seen := 0
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if seen > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: gap}.Layout))
		}
		item := item
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.metaBadge(gtx, item, compact)
		}))
		seen++
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (a *App) imageThumb(gtx layout.Context, img image.Image, width unit.Dp, height unit.Dp, radius unit.Dp) layout.Dimensions {
	if img == nil {
		return a.imageThumbWithOp(gtx, nil, paint.ImageOp{}, width, height, radius)
	}
	return a.imageThumbWithOp(gtx, img, paint.NewImageOp(img), width, height, radius)
}

func (a *App) imageThumbWithOp(gtx layout.Context, img image.Image, imgOp paint.ImageOp, width unit.Dp, height unit.Dp, radius unit.Dp) layout.Dimensions {
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
			return a.borderedSurface(gtx, fluent.panel2, radius, fluent.border, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				if img == nil {
					return a.previewFallbackGraphic(gtx, radius)
				}
				return layout.UniformInset(unit.Dp(3)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = gtx.Constraints.Max
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						view := widget.Image{
							Src:      imgOp,
							Fit:      widget.Contain,
							Position: layout.Center,
						}
						return view.Layout(gtx)
					})
				})
			})
		})
	})
}

func (a *App) imageThumbCover(gtx layout.Context, img image.Image, width unit.Dp, height unit.Dp, radius unit.Dp) layout.Dimensions {
	if img == nil {
		return a.imageThumbCoverWithOp(gtx, nil, paint.ImageOp{}, width, height, radius)
	}
	return a.imageThumbCoverWithOp(gtx, img, paint.NewImageOp(img), width, height, radius)
}

func (a *App) imageThumbCoverWithOp(gtx layout.Context, img image.Image, imgOp paint.ImageOp, width unit.Dp, height unit.Dp, radius unit.Dp) layout.Dimensions {
	return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
			return a.borderedSurface(gtx, fluent.panel2, radius, fluent.border, func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = gtx.Constraints.Max
				if img == nil {
					return a.previewFallbackGraphic(gtx, radius)
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

func (a *App) previewFallbackGraphic(gtx layout.Context, radius unit.Dp) layout.Dimensions {
	size := gtx.Constraints.Max
	if size.X <= 0 || size.Y <= 0 {
		return layout.Dimensions{Size: size}
	}
	gtx.Constraints.Min = size
	if a.reducedEffects {
		paint.FillShape(gtx.Ops, fluent.canvasTile, clip.RRect{
			Rect: image.Rect(0, 0, size.X, size.Y),
			NW:   gtx.Dp(radius),
			NE:   gtx.Dp(radius),
			SW:   gtx.Dp(radius),
			SE:   gtx.Dp(radius),
		}.Op(gtx.Ops))
		return layout.Dimensions{Size: size}
	}
	paintLinearGradient(gtx, image.Rect(0, 0, size.X, size.Y), radius, rgb(0x49b2e8), rgb(0x4b27d2))
	paint.FillShape(gtx.Ops, rgba(0xffffff, 0x2d), clip.Ellipse(image.Rect(int(float32(size.X)*0.6), int(float32(size.Y)*0.05), int(float32(size.X)*1.02), int(float32(size.Y)*0.48))).Op(gtx.Ops))
	paint.FillShape(gtx.Ops, rgba(0x0c145a, 0x45), clip.Ellipse(image.Rect(int(float32(size.X)*-0.08), int(float32(size.Y)*0.48), int(float32(size.X)*0.52), int(float32(size.Y)*1.08))).Op(gtx.Ops))
	barRect := image.Rect(int(float32(size.X)*0.18), int(float32(size.Y)*0.67), int(float32(size.X)*0.82), int(float32(size.Y)*0.83))
	paint.FillShape(gtx.Ops, rgba(0x1a0a44, 0x94), clip.RRect{
		Rect: barRect,
		NW:   gtx.Dp(unit.Dp(10)),
		NE:   gtx.Dp(unit.Dp(10)),
		SW:   gtx.Dp(unit.Dp(10)),
		SE:   gtx.Dp(unit.Dp(10)),
	}.Op(gtx.Ops))
	return layout.Dimensions{Size: size}
}

func (a *App) label(gtx layout.Context, text string, size unit.Sp, color color.NRGBA, weight font.Weight) layout.Dimensions {
	style := material.Label(a.th, a.scaledSp(size), text)
	style.Color = color
	style.Font.Weight = weight
	style.WrapPolicy = textWrapWords
	return style.Layout(gtx)
}

func (a *App) singleLineLabel(gtx layout.Context, text string, size unit.Sp, color color.NRGBA, weight font.Weight) layout.Dimensions {
	style := material.Label(a.th, a.scaledSp(size), text)
	style.Color = color
	style.Font.Weight = weight
	style.MaxLines = 1
	style.Truncator = "..."
	return style.Layout(gtx)
}

func (a *App) clampedLabel(gtx layout.Context, text string, size unit.Sp, color color.NRGBA, weight font.Weight, lines int) layout.Dimensions {
	if lines <= 0 {
		lines = 1
	}
	style := material.Label(a.th, a.scaledSp(size), text)
	style.Color = color
	style.Font.Weight = weight
	style.MaxLines = lines
	style.Truncator = "..."
	style.WrapPolicy = textWrapWords
	return style.Layout(gtx)
}

func (a *App) monoLabel(gtx layout.Context, text string, size unit.Sp, color color.NRGBA, weight font.Weight) layout.Dimensions {
	style := material.Label(a.th, a.scaledSp(size), text)
	style.Color = color
	style.Font.Weight = weight
	style.Font.Typeface = desktopMonoTypeface(a.desktopStyle)
	return style.Layout(gtx)
}

func (a *App) card(gtx layout.Context, w layout.Widget) layout.Dimensions {
	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, w)
	})
}

func (a *App) controlCard(gtx layout.Context, w layout.Widget) layout.Dimensions {
	bg := withAlpha(fluent.white, 0xb3)
	if a.isDarkTheme() {
		bg = fluent.surfaceElevated
	}
	return a.elevatedBorderedSurface(gtx, bg, unit.Dp(12), fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, w)
	})
}

func (a *App) layoutStandardModal(
	gtx layout.Context,
	width unit.Dp,
	height unit.Dp,
	title string,
	subtitle string,
	closeBtn *widget.Clickable,
	body layout.Widget,
) layout.Dimensions {
	paint.FillShape(gtx.Ops, rgba(0x000000, 0x32), clip.Rect{Max: gtx.Constraints.Max}.Op())
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return fixedWidth(gtx, width, func(gtx layout.Context) layout.Dimensions {
			frame := func(gtx layout.Context) layout.Dimensions {
				return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentModalRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(0)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						children := []layout.FlexChild{
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(12), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.label(gtx, title, unit.Sp(15), fluent.text, font.SemiBold)
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													if strings.TrimSpace(subtitle) == "" {
														return layout.Dimensions{}
													}
													return a.singleLineLabel(gtx, subtitle, unit.Sp(11), fluent.textMuted, font.Normal)
												}),
											)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											if closeBtn == nil {
												return layout.Dimensions{}
											}
											return a.ghostIconButton(gtx, closeBtn, uiIconClose, false)
										}),
									)
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(1), func(gtx layout.Context) layout.Dimensions {
									return a.surface(gtx, fluent.border, 0, layout.Spacer{}.Layout)
								})
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(16), Bottom: unit.Dp(16), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, body)
							}),
						}
						return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
					})
				})
			}
			if height > 0 {
				return fixedHeight(gtx, height, frame)
			}
			return frame(gtx)
		})
	})
}

func (a *App) elevatedBorderedSurface(
	gtx layout.Context,
	bg color.NRGBA,
	radius unit.Dp,
	border color.NRGBA,
	shadowOffset image.Point,
	w layout.Widget,
) layout.Dimensions {
	if a.reducedEffects {
		return a.borderedSurface(gtx, bg, radius, border, w)
	}
	macro := op.Record(gtx.Ops)
	dims := a.borderedSurface(gtx, bg, radius, border, w)
	call := macro.Stop()
	if dims.Size.X > 0 && dims.Size.Y > 0 && fluent.cardShadow.A > 0 {
		shadow := op.Offset(shadowOffset).Push(gtx.Ops)
		paint.FillShape(gtx.Ops, fluent.cardShadow, clip.RRect{
			Rect: image.Rectangle{Max: dims.Size},
			NW:   gtx.Dp(radius),
			NE:   gtx.Dp(radius),
			SW:   gtx.Dp(radius),
			SE:   gtx.Dp(radius),
		}.Op(gtx.Ops))
		shadow.Pop()
	}
	call.Add(gtx.Ops)
	if dims.Size.X > 2 && dims.Size.Y > 2 && fluent.cardGlow.A > 0 {
		glowHeight := min(dims.Size.Y/3, gtx.Dp(unit.Dp(28)))
		if glowHeight < 8 {
			glowHeight = min(dims.Size.Y, gtx.Dp(unit.Dp(8)))
		}
		if glowHeight > 0 {
			paint.FillShape(gtx.Ops, fluent.cardGlow, clip.RRect{
				Rect: image.Rect(1, 1, dims.Size.X-1, glowHeight),
				NW:   gtx.Dp(radius),
				NE:   gtx.Dp(radius),
			}.Op(gtx.Ops))
		}
	}
	return dims
}

func (a *App) borderedSurface(gtx layout.Context, bg color.NRGBA, radius unit.Dp, border color.NRGBA, w layout.Widget) layout.Dimensions {
	return a.surface(gtx, border, radius, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			macro := op.Record(gtx.Ops)
			dims := a.surface(gtx, bg, radius, w)
			call := macro.Stop()
			call.Add(gtx.Ops)
			if !a.reducedEffects && dims.Size.X > 2 && dims.Size.Y > 2 && fluent.windowOutline.A > 0 {
				highlightHeight := min(dims.Size.Y/3, gtx.Dp(unit.Dp(22)))
				if highlightHeight < 4 {
					highlightHeight = min(dims.Size.Y, gtx.Dp(unit.Dp(4)))
				}
				if highlightHeight > 0 {
					paintLinearGradient(gtx, image.Rect(1, 1, dims.Size.X-1, highlightHeight), radius, fluent.windowOutline, rgba(0xffffff, 0x00))
				}
			}
			return dims
		})
	})
}

func (a *App) borderedTopTabSurface(
	gtx layout.Context,
	bg color.NRGBA,
	border color.NRGBA,
	radius unit.Dp,
	w layout.Widget,
) layout.Dimensions {
	return a.surfaceCorners(gtx, border, radius, radius, 0, 0, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(1)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.surfaceCorners(gtx, bg, radius, radius, 0, 0, w)
		})
	})
}

func (a *App) surface(gtx layout.Context, bg color.NRGBA, radius unit.Dp, w layout.Widget) layout.Dimensions {
	return a.surfaceCorners(gtx, bg, radius, radius, radius, radius, w)
}

func paintLinearGradient(gtx layout.Context, rect image.Rectangle, radius unit.Dp, from color.NRGBA, to color.NRGBA) {
	if rect.Empty() {
		return
	}
	paint.LinearGradientOp{
		Stop1:  f32.Pt(float32(rect.Min.X), float32(rect.Min.Y)),
		Color1: from,
		Stop2:  f32.Pt(float32(rect.Min.X), float32(rect.Max.Y)),
		Color2: to,
	}.Add(gtx.Ops)
	stack := clip.RRect{
		Rect: rect,
		NW:   gtx.Dp(radius),
		NE:   gtx.Dp(radius),
		SW:   gtx.Dp(radius),
		SE:   gtx.Dp(radius),
	}.Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Pop()
}

func (a *App) surfaceCorners(
	gtx layout.Context,
	bg color.NRGBA,
	nw unit.Dp,
	ne unit.Dp,
	sw unit.Dp,
	se unit.Dp,
	w layout.Widget,
) layout.Dimensions {
	macro := op.Record(gtx.Ops)
	dims := w(gtx)
	call := macro.Stop()
	shape := clip.RRect{
		Rect: image.Rectangle{Max: dims.Size},
		NW:   gtx.Dp(nw),
		NE:   gtx.Dp(ne),
		SW:   gtx.Dp(sw),
		SE:   gtx.Dp(se),
	}
	shapeOp := shape.Op(gtx.Ops)
	paint.FillShape(gtx.Ops, bg, shapeOp)
	stack := shape.Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
	return dims
}

func fixedWidth(gtx layout.Context, width unit.Dp, w layout.Widget) layout.Dimensions {
	px := gtx.Dp(width)
	if px > gtx.Constraints.Max.X {
		px = gtx.Constraints.Max.X
	}
	gtx.Constraints.Min.X = px
	gtx.Constraints.Max.X = px
	return w(gtx)
}

func fixedPixelWidth(gtx layout.Context, px int, w layout.Widget) layout.Dimensions {
	if px > gtx.Constraints.Max.X {
		px = gtx.Constraints.Max.X
	}
	if px < 0 {
		px = 0
	}
	gtx.Constraints.Min.X = px
	gtx.Constraints.Max.X = px
	return w(gtx)
}

func fixedPixelHeight(gtx layout.Context, px int, w layout.Widget) layout.Dimensions {
	if px > gtx.Constraints.Max.Y {
		px = gtx.Constraints.Max.Y
	}
	if px < 0 {
		px = 0
	}
	gtx.Constraints.Min.Y = px
	gtx.Constraints.Max.Y = px
	return w(gtx)
}

func minimumTextControlHeight(gtx layout.Context, token unit.Dp, scaledTextSize unit.Sp, verticalPadding unit.Dp) unit.Dp {
	tokenPixels := gtx.Dp(token)
	textPixels := gtx.Sp(scaledTextSize)
	linePixels := (textPixels*5 + 3) / 4
	requiredPixels := linePixels + gtx.Dp(verticalPadding)
	if requiredPixels <= tokenPixels {
		return token
	}
	return gtx.Metric.PxToDp(requiredPixels)
}

func fixedHeight(gtx layout.Context, height unit.Dp, w layout.Widget) layout.Dimensions {
	px := gtx.Dp(height)
	if px > gtx.Constraints.Max.Y {
		px = gtx.Constraints.Max.Y
	}
	gtx.Constraints.Min.Y = px
	gtx.Constraints.Max.Y = px
	return w(gtx)
}

func chooseColor(ok bool, whenTrue color.NRGBA, whenFalse color.NRGBA) color.NRGBA {
	if ok {
		return whenTrue
	}
	return whenFalse
}

var textWrapWords = text.WrapWords
