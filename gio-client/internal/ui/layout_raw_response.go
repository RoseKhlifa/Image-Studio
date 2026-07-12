package ui

import (
	"image"
	"io"
	"strings"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (a *App) layoutRawResponseModal(gtx layout.Context, snap snapshot) layout.Dimensions {
	path := strings.TrimSpace(snap.RawResponseModalPath)
	if path == "" && strings.TrimSpace(snap.RawResponseModalError) == "" && strings.TrimSpace(snap.RawResponseModalText) == "" {
		return layout.Dimensions{}
	}
	for a.closeRawResponseButton.Clicked(gtx) {
		a.closeRawResponseModal()
	}
	for a.copyRawResponseButton.Clicked(gtx) {
		text := strings.TrimSpace(snap.RawResponseModalText)
		if text == "" {
			continue
		}
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
		a.appendLog("已复制 Raw response")
	}

	return a.layoutStandardModal(
		gtx,
		unit.Dp(760),
		unit.Dp(620),
		"原始上游响应",
		"",
		&a.closeRawResponseButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.monoLabel(gtx, path, unit.Sp(10), fluent.textDim, font.Normal)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.compactIconTextButton(gtx, &a.copyRawResponseButton, uiIconCopy, "复制全文", false)
						}),
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					if strings.TrimSpace(snap.RawResponseModalError) != "" {
						return a.elevatedBorderedSurface(gtx, dangerAlpha(0x12), fluentCardRadius, dangerAlpha(0x2f), image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, snap.RawResponseModalError, unit.Sp(11), fluent.danger, font.Normal)
							})
						})
					}
					return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							style := material.Editor(a.th, &a.rawResponseViewerInput, "")
							style.Color = fluent.textMuted
							style.HintColor = fluent.textDim
							style.TextSize = a.scaledSp(unit.Sp(11))
							style.Font.Typeface = desktopMonoTypeface(a.desktopStyle)
							style.Font.Weight = font.Normal
							return style.Layout(gtx)
						})
					})
				}),
			)
		},
	)
}
