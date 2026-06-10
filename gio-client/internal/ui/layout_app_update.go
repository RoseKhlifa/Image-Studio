package ui

import (
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

func (a *App) layoutAppUpdateModal(gtx layout.Context) layout.Dimensions {
	info, open, _, _ := a.readAppUpdateState()
	if !open || info == nil {
		return layout.Dimensions{}
	}
	for a.dismissAppUpdateButton.Clicked(gtx) {
		a.dismissAppUpdateModal()
	}
	for a.ignoreAppUpdateButton.Clicked(gtx) {
		a.ignoreAppUpdate(info.ReleaseTag)
	}
	for a.openAppUpdateButton.Clicked(gtx) {
		a.openAppUpdateRelease()
	}
	summary := strings.TrimSpace(info.Body)
	if summary == "" {
		summary = "GitHub Releases 已发布新版本。"
	}
	if len(summary) > 140 {
		summary = summary[:140]
	}
	return a.layoutStandardModal(
		gtx,
		unit.Dp(460),
		0,
		"发现新版本",
		"",
		&a.dismissAppUpdateButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.accentSoft, fluentCardRadius, accentAlpha(0x22), func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "v"+info.LatestVersion+" 已发布", unit.Sp(13), fluent.text, font.SemiBold)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									current := "当前版本 v" + info.CurrentVersion
									if strings.TrimSpace(info.ReleaseName) != "" {
										current += " · " + strings.TrimSpace(info.ReleaseName)
									}
									return a.label(gtx, current, unit.Sp(11), fluent.textDim, font.Normal)
								}),
							)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface2, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, summary, unit.Sp(11), fluent.text, font.Normal)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.primaryIconTextButton(gtx, &a.openAppUpdateButton, uiIconLaunch, "立即查看更新", fluent.accent, fluent.white)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(96), func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.dismissAppUpdateButton, "稍后再说", false)
							})
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.textActionButton(gtx, &a.ignoreAppUpdateButton, "不再提示这个版本", true)
				}),
			)
		},
	)
}
