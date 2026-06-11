package ui

import (
	"image"
	"strconv"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
)

func canvasTextAnnotationRect(text string, point image.Point) image.Rectangle {
	text = strings.TrimSpace(text)
	if text == "" {
		return image.Rectangle{}
	}
	width := max(24, len([]rune(text))*12)
	height := 24
	return image.Rect(point.X, point.Y, point.X+width, point.Y+height)
}

func (a *App) openCanvasAnnotationTextPrompt(point image.Point) {
	a.mu.Lock()
	a.canvasAnnotationTextPromptOpen = true
	a.canvasAnnotationTextPoint = point
	a.canvasAnnotationTextInput.SetText("")
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closeCanvasAnnotationTextPrompt() {
	a.mu.Lock()
	a.canvasAnnotationTextPromptOpen = false
	a.canvasAnnotationTextInput.SetText("")
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) confirmCanvasAnnotationText() {
	a.mu.Lock()
	text := strings.TrimSpace(a.canvasAnnotationTextInput.Text())
	point := a.canvasAnnotationTextPoint
	color := a.canvasAnnotationColor
	a.canvasAnnotationTextPromptOpen = false
	a.canvasAnnotationTextInput.SetText("")
	a.mu.Unlock()
	if text == "" {
		a.invalidateNow()
		return
	}
	rect := canvasTextAnnotationRect(text, point)
	if rect.Empty() {
		a.invalidateNow()
		return
	}
	a.addCanvasAnnotationItem(canvasAnnotation{
		ID:    "ann-" + strconv.FormatInt(time.Now().UnixNano(), 36),
		Kind:  canvasAnnotationKindText,
		Color: color,
		Rect:  rect,
		Text:  text,
	})
}

func (a *App) layoutCanvasAnnotationTextPrompt(gtx layout.Context) layout.Dimensions {
	for a.canvasAnnotationTextConfirmButton.Clicked(gtx) {
		a.confirmCanvasAnnotationText()
	}
	for a.canvasAnnotationTextCancelButton.Clicked(gtx) {
		a.closeCanvasAnnotationTextPrompt()
	}
	return a.layoutStandardModal(
		gtx,
		unit.Dp(420),
		0,
		"文字标注",
		"",
		nil,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "输入一段文字后，会按当前标注颜色放到你点击的位置。", unit.Sp(11), fluent.textDim, font.Normal)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.field(gtx, "文本", &a.canvasAnnotationTextInput, "输入标注内容", unit.Dp(48))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(110), func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.canvasAnnotationTextCancelButton, "取消", false)
							})
						}),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(132), func(gtx layout.Context) layout.Dimensions {
								return a.primaryIconTextButton(gtx, &a.canvasAnnotationTextConfirmButton, uiIconCheck, "添加文字", fluent.accent, fluent.white)
							})
						}),
					)
				}),
			)
		},
	)
}
