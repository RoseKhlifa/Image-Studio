package ui

import (
	"image"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
)

const promptHelperPopoverMargin = 12

func (a *App) trackGlobalPointer(gtx layout.Context) {
	if gtx.Constraints.Max.X <= 0 || gtx.Constraints.Max.Y <= 0 {
		return
	}
	clipArea := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &a.globalPointerTag)
	pass.Pop()
	clipArea.Pop()
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  &a.globalPointerTag,
			Kinds:   pointer.Press | pointer.Move | pointer.Drag | pointer.Release,
			ScrollX: pointer.ScrollRange{Min: -1_000_000, Max: 1_000_000},
			ScrollY: pointer.ScrollRange{Min: -1_000_000, Max: 1_000_000},
		})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		pos := image.Pt(int(pe.Position.X), int(pe.Position.Y))
		a.mu.Lock()
		a.lastGlobalPointer = pos
		if pe.Kind == pointer.Press {
			a.lastGlobalPressPos = pos
		}
		a.mu.Unlock()
	}
}

func (a *App) promptHelperDefaultAnchorRect() image.Rectangle {
	return image.Rect(28, 240, 160, 272)
}

func (a *App) promptHelperAnchorRectFromButton(btn *widget.Clickable, size image.Point) (image.Rectangle, bool) {
	if btn == nil || size.X <= 0 || size.Y <= 0 {
		return image.Rectangle{}, false
	}
	history := btn.History()
	if len(history) == 0 {
		return image.Rectangle{}, false
	}
	press := history[len(history)-1]
	a.mu.Lock()
	global := a.lastGlobalPressPos
	if global == (image.Point{}) {
		global = a.lastGlobalPointer
	}
	a.mu.Unlock()
	minPt := global.Sub(press.Position)
	return image.Rectangle{Min: minPt, Max: minPt.Add(size)}, true
}

func normalizePromptHelperTab(value string) string {
	switch strings.TrimSpace(value) {
	case "history":
		return strings.TrimSpace(value)
	default:
		return "templates"
	}
}

func (a *App) openPromptHelperPopover(tab string, btn *widget.Clickable, size image.Point) {
	rect, ok := a.promptHelperAnchorRectFromButton(btn, size)
	if !ok {
		a.mu.Lock()
		rect = a.promptHelperAnchorRect
		a.mu.Unlock()
		if rect == (image.Rectangle{}) {
			rect = a.promptHelperDefaultAnchorRect()
		}
	}
	a.mu.Lock()
	a.promptHelperTab = normalizePromptHelperTab(tab)
	a.promptHelperAnchorRect = rect
	a.promptHelperOpen = true
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closePromptHelperPopover() {
	a.mu.Lock()
	a.promptHelperOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) layoutPromptHelperToggleButton(gtx layout.Context) layout.Dimensions {
	return fixedHeight(gtx, unit.Dp(32), func(gtx layout.Context) layout.Dimensions {
		dims := a.ghostIconTextButton(gtx, &a.promptHelperButton, uiIconHistory, "模板 / 历史", a.promptHelperOpen)
		a.promptHelperButtonSize = dims.Size
		return dims
	})
}

func (a *App) promptHelperPopoverWidth(gtx layout.Context) int {
	width := gtx.Dp(unit.Dp(360))
	maxWidth := gtx.Constraints.Max.X - promptHelperPopoverMargin*2
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
	}
	if width < 280 {
		width = max(220, maxWidth)
	}
	return width
}

func (a *App) promptHelperPopoverPos(viewport image.Point, anchor image.Rectangle, panelWidth int, panelHeight int) image.Point {
	left := anchor.Min.X
	maxLeft := max(promptHelperPopoverMargin, viewport.X-panelWidth-promptHelperPopoverMargin)
	left = clampInt(left, promptHelperPopoverMargin, maxLeft)
	top := anchor.Max.Y + promptHelperPopoverMargin
	maxTop := max(promptHelperPopoverMargin, viewport.Y-panelHeight-promptHelperPopoverMargin)
	top = clampInt(top, promptHelperPopoverMargin, maxTop)
	return image.Pt(left, top)
}

func (a *App) promptHelperHandleOverlayEvents(gtx layout.Context, panelRect image.Rectangle) {
	clipArea := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &a.promptHelperEventTag)
	pass.Pop()
	clipArea.Pop()
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			pointer.Filter{
				Target:  &a.promptHelperEventTag,
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
				a.closePromptHelperPopover()
			}
		case pointer.Event:
			point := image.Pt(int(evt.Position.X), int(evt.Position.Y))
			a.mu.Lock()
			anchorRect := a.promptHelperAnchorRect
			a.mu.Unlock()
			if point.In(panelRect) || point.In(anchorRect) {
				continue
			}
			a.closePromptHelperPopover()
		}
	}
}

func (a *App) layoutPromptHelperPopover(gtx layout.Context) layout.Dimensions {
	snap := a.readSnapshot()
	suggestions := a.promptSuggestions(snap.History)
	for a.closePromptHelperButton.Clicked(gtx) {
		a.closePromptHelperPopover()
	}
	for a.promptHelperTemplatesButton.Clicked(gtx) {
		a.promptHelperTab = "templates"
	}
	for a.promptHelperHistoryButton.Clicked(gtx) {
		a.promptHelperTab = "history"
	}
	viewport := gtx.Constraints.Max
	if viewport.X <= 0 || viewport.Y <= 0 {
		return layout.Dimensions{Size: viewport}
	}
	panelWidth := a.promptHelperPopoverWidth(gtx)
	panelHeight := min(gtx.Dp(unit.Dp(360)), max(200, viewport.Y-promptHelperPopoverMargin*2))
	a.mu.Lock()
	anchorRect := a.promptHelperAnchorRect
	a.mu.Unlock()
	if anchorRect == (image.Rectangle{}) {
		anchorRect = a.promptHelperDefaultAnchorRect()
	}
	position := a.promptHelperPopoverPos(viewport, anchorRect, panelWidth, panelHeight)
	panelRect := image.Rectangle{Min: position, Max: position.Add(image.Pt(panelWidth, panelHeight))}
	a.promptHelperHandleOverlayEvents(gtx, panelRect)

	trans := op.Offset(position).Push(gtx.Ops)
	_ = fixedWidth(gtx, unit.Dp(float32(panelWidth)), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(float32(panelHeight)), func(gtx layout.Context) layout.Dimensions {
			return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, unit.Dp(14), fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
								layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
									return a.layoutPromptHelperTabs(gtx, len(a.promptTemplateItems()), len(suggestions))
								}),
								layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.ghostIconButton(gtx, &a.closePromptHelperButton, uiIconClose, false)
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
							return a.layoutPromptHelperPanel(gtx, suggestions)
						})
					}),
				)
			})
		})
	})
	trans.Pop()
	return layout.Dimensions{Size: viewport}
}
