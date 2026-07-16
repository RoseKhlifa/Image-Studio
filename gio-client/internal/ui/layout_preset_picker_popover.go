package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

const presetPickerPopoverMargin = 12
const presetPickerPopoverWidth = 360

func (a *App) presetPickerPopoverPos(viewport image.Point, anchor image.Rectangle, panelWidth int, panelHeight int) image.Point {
	left := anchor.Min.X
	maxLeft := max(presetPickerPopoverMargin, viewport.X-panelWidth-presetPickerPopoverMargin)
	left = clampInt(left, presetPickerPopoverMargin, maxLeft)
	top := anchor.Max.Y + presetPickerPopoverMargin
	maxTop := max(presetPickerPopoverMargin, viewport.Y-panelHeight-presetPickerPopoverMargin)
	top = clampInt(top, presetPickerPopoverMargin, maxTop)
	return image.Pt(left, top)
}

func (a *App) presetPickerHandleOverlayEvents(gtx layout.Context, panelRect image.Rectangle) {
	clipArea := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &a.presetPickerEventTag)
	pass.Pop()
	clipArea.Pop()
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			pointer.Filter{
				Target:  &a.presetPickerEventTag,
				Kinds:   pointer.Press,
				ScrollX: pointer.ScrollRange{Min: -1_000_000, Max: 1_000_000},
				ScrollY: pointer.ScrollRange{Min: -1_000_000, Max: 1_000_000},
			},
		)
		if !ok {
			break
		}
		switch evt := ev.(type) {
		case key.Event:
			if evt.State == key.Press && evt.Name == key.NameEscape {
				a.closePresetPicker()
			}
		case pointer.Event:
			point := image.Pt(int(evt.Position.X), int(evt.Position.Y))
			a.mu.Lock()
			anchorRect := a.presetPickerAnchorRect
			a.mu.Unlock()
			if point.In(panelRect) || point.In(anchorRect) {
				continue
			}
			a.closePresetPicker()
		}
	}
}

func (a *App) layoutPresetPickerPopover(gtx layout.Context) layout.Dimensions {
	if !a.presetPickerOpen || len(a.presets) == 0 {
		return layout.Dimensions{Size: gtx.Constraints.Max}
	}
	presetSummary := a.currentPresetSummaryState()
	for a.closePresetPickerButton.Clicked(gtx) {
		a.closePresetPicker()
	}
	viewport := gtx.Constraints.Max
	if viewport.X <= 0 || viewport.Y <= 0 {
		return layout.Dimensions{Size: viewport}
	}
	panelWidth := min(gtx.Dp(unit.Dp(presetPickerPopoverWidth)), viewport.X-presetPickerPopoverMargin*2)
	if panelWidth <= 0 {
		panelWidth = max(220, viewport.X-presetPickerPopoverMargin*2)
	}
	panelHeight := min(gtx.Dp(unit.Dp(360)), max(200, viewport.Y-presetPickerPopoverMargin*2))
	a.mu.Lock()
	anchorRect := a.presetPickerAnchorRect
	a.mu.Unlock()
	if anchorRect == (image.Rectangle{}) {
		anchorRect = image.Rect(28, 168, 320, 214)
	}
	position := a.presetPickerPopoverPos(viewport, anchorRect, panelWidth, panelHeight)
	panelRect := image.Rectangle{Min: position, Max: position.Add(image.Pt(panelWidth, panelHeight))}
	a.presetPickerHandleOverlayEvents(gtx, panelRect)

	trans := op.Offset(position).Push(gtx.Ops)
	_ = fixedWidth(gtx, unit.Dp(float32(panelWidth)), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(float32(panelHeight)), func(gtx layout.Context) layout.Dimensions {
			return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, unit.Dp(14), fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "选择预设", unit.Sp(11), fluent.textMuted, font.SemiBold)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.ghostIconButton(gtx, &a.closePresetPickerButton, uiIconClose, false)
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
						return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.layoutPresetQuickPickerPopup(gtx, presetSummary)
						})
					}),
				)
			})
		})
	})
	trans.Pop()
	return layout.Dimensions{Size: viewport}
}
