package ui

import (
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/yuanhua/image-gptcodex/pkg/client"
)

func (a *App) openCustomSizeModal() {
	if !supportsPreciseSizeControl(a.api, a.policy, a.imageModelInput.Text()) {
		a.appendLog("当前模型链路不支持精确尺寸自定义")
		return
	}
	width := 1024
	height := 1024
	if parsedWidth, parsedHeight, ok := parseSizeSelectionValue(strings.TrimSpace(a.size)); ok {
		width = parsedWidth
		height = parsedHeight
	}
	a.customSizeWidthInput.SetText(strconv.Itoa(width))
	a.customSizeHeightInput.SetText(strconv.Itoa(height))
	a.customSizeModalOpen = true
	a.invalidateNow()
}

func (a *App) closeCustomSizeModal() {
	a.customSizeModalOpen = false
	a.invalidateNow()
}

func (a *App) applyCustomSize() {
	if !supportsPreciseSizeControl(a.api, a.policy, a.imageModelInput.Text()) {
		a.appendLog("当前模型链路不支持精确尺寸自定义")
		return
	}
	width, _ := strconv.Atoi(strings.TrimSpace(a.customSizeWidthInput.Text()))
	height, _ := strconv.Atoi(strings.TrimSpace(a.customSizeHeightInput.Text()))
	nextSize, ok := buildExactSizeValue(width, height)
	if !ok {
		a.appendLog("请输入有效的精确尺寸")
		return
	}
	a.size = nextSize
	a.customSizeModalOpen = false
	a.appendLog("已应用精确尺寸: " + nextSize)
	a.invalidateNow()
}

func (a *App) layoutCustomSizeModal(gtx layout.Context) layout.Dimensions {
	for a.closeCustomSizeModalButton.Clicked(gtx) {
		a.closeCustomSizeModal()
	}
	for a.applyCustomSizeButton.Clicked(gtx) {
		a.applyCustomSize()
	}

	currentSize := strings.TrimSpace(a.size)
	if currentSize == "" || currentSize == "auto" {
		currentSize = client.DefaultSize
	}

	return a.layoutStandardModal(
		gtx,
		unit.Dp(680),
		0,
		"精确尺寸",
		"",
		&a.closeCustomSizeModalButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "直接指定要传给上游的 size。点击比例或分辨率预设后，会自动退出精确尺寸模式。", unit.Sp(11), fluent.textDim, font.Normal)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface2, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "当前工作区尺寸", unit.Sp(12), fluent.text, font.SemiBold)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, currentSize, unit.Sp(11), fluent.textDim, font.Normal)
								}),
							)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface2, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							children := []layout.FlexChild{
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "设置精确尺寸", unit.Sp(12), fluent.text, font.SemiBold)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "应用会自动收口到最接近的合法尺寸；最长边不超过 3840px，宽高都能被 16 整除。", unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.field(gtx, "宽度(px)", &a.customSizeWidthInput, "1536", unit.Dp(42))
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return a.label(gtx, "×", unit.Sp(18), fluent.textDim, font.SemiBold)
											})
										}),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.field(gtx, "高度(px)", &a.customSizeHeightInput, "1024", unit.Dp(42))
										}),
									)
								}),
								layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return fixedWidth(gtx, unit.Dp(120), func(gtx layout.Context) layout.Dimensions {
											return a.primaryButton(gtx, &a.applyCustomSizeButton, "应用尺寸", fluent.accent, fluent.white)
										})
									})
								}),
							}
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, children...)
						})
					})
				}),
			)
		},
	)
}
