package ui

import (
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func (a *App) layoutUpstreamQuickImportModal(gtx layout.Context) layout.Dimensions {
	return a.layoutStandardModal(
		gtx,
		unit.Dp(620),
		unit.Dp(520),
		"快捷导入上游配置",
		"",
		&a.closeQuickImportUpstreamConfigsButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "直接粘贴对方提供的 JSON 模板。当前支持本应用导出文件、newapi_channel_conn、OpenCode provider 配置。", unit.Sp(11), fluent.textDim, font.Normal)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface, fluentInputRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							style := material.Editor(a.th, &a.upstreamQuickImportInput, "在这里粘贴 JSON...\n例如 {\"_type\":\"newapi_channel_conn\",...}")
							style.Color = fluent.text
							style.HintColor = fluent.textDim
							style.SelectionColor = accentAlpha(0x3d)
							style.TextSize = a.scaledSp(unit.Sp(12))
							style.Font.Typeface = uiMonoTypeface
							return style.Layout(gtx)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.accentSoft, fluentCardRadius, accentAlpha(0x22), func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, "导入后会直接生成可用 profile，并把 API Key 写入系统凭据存储。若模板里自带 /v1，会自动适配成站点根地址。", unit.Sp(10), fluent.accent, font.Normal)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(96), func(gtx layout.Context) layout.Dimensions {
									return a.compactButton(gtx, &a.closeQuickImportUpstreamConfigsButton, "取消", false)
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(120), func(gtx layout.Context) layout.Dimensions {
									return a.primaryButton(gtx, &a.confirmQuickImportUpstreamConfigsButton, "立即导入", fluent.accent, fluent.white)
								})
							}),
						)
					})
				}),
			)
		},
	)
}
