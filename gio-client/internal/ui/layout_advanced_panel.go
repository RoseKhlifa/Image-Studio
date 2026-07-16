package ui

import (
	"image"
	"strings"
	"time"

	gioCompat "image-studio/gio-client/internal/compat"
	sharedCompat "image-studio/shared/compat"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
)

const advancedPanelMargin = 16
const advancedPanelHeaderHeight = 84

func defaultAdvancedPanelGroupPrefs() map[string]bool {
	return map[string]bool{
		"core":     true,
		"output":   false,
		"strategy": false,
		"stream":   false,
	}
}

func normalizeAdvancedPanelGroupPrefs(groups map[string]bool) map[string]bool {
	normalized := defaultAdvancedPanelGroupPrefs()
	for key, value := range groups {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		normalized[key] = value
	}
	return normalized
}

func (a *App) applyAdvancedPanelGroupPrefs(groups map[string]bool) {
	groups = normalizeAdvancedPanelGroupPrefs(groups)
	a.advancedCoreGroupOpen = groups["core"]
	a.advancedOutputGroupOpen = groups["output"]
	a.advancedStrategyGroupOpen = groups["strategy"]
	a.advancedStreamGroupOpen = groups["stream"]
}

func (a *App) advancedPanelGroupPrefs() map[string]bool {
	return map[string]bool{
		"core":     a.advancedCoreGroupOpen,
		"output":   a.advancedOutputGroupOpen,
		"strategy": a.advancedStrategyGroupOpen,
		"stream":   a.advancedStreamGroupOpen,
	}
}

func (a *App) persistAdvancedPanelPrefs() error {
	a.mu.Lock()
	x := a.advancedPanelPos.X
	y := a.advancedPanelPos.Y
	groups := a.advancedPanelGroupPrefs()
	a.mu.Unlock()
	return gioCompat.UpdateState(func(state *sharedCompat.State) error {
		*state = sharedCompat.Normalize(*state)
		state.Settings.AdvancedFloatingPanel = &sharedCompat.AdvancedFloatingPanelPrefs{
			X:      &x,
			Y:      &y,
			Groups: groups,
		}
		state.UpdatedAt = time.Now().UnixMilli()
		return nil
	})
}

func (a *App) advancedPanelWidth(gtx layout.Context) int {
	width := gtx.Dp(unit.Dp(360))
	maxWidth := gtx.Constraints.Max.X - advancedPanelMargin*2
	if maxWidth > 0 && width > maxWidth {
		width = maxWidth
	}
	if width < 280 {
		width = max(220, maxWidth)
	}
	return width
}

func (a *App) advancedPanelMaxHeight(gtx layout.Context) int {
	maxHeight := gtx.Constraints.Max.Y - advancedPanelMargin*2
	if maxHeight < gtx.Dp(unit.Dp(320)) {
		return maxHeight
	}
	preferred := gtx.Dp(unit.Dp(720))
	if preferred > maxHeight {
		preferred = maxHeight
	}
	return preferred
}

func (a *App) clampAdvancedPanelPos(pos image.Point, viewport image.Point, panelWidth int, panelHeight int) image.Point {
	maxX := max(advancedPanelMargin, viewport.X-panelWidth-advancedPanelMargin)
	maxY := max(advancedPanelMargin, viewport.Y-panelHeight-advancedPanelMargin)
	return image.Pt(
		clampInt(pos.X, advancedPanelMargin, maxX),
		clampInt(pos.Y, advancedPanelMargin, maxY),
	)
}

func (a *App) ensureAdvancedPanelPos(viewport image.Point, panelWidth int, panelHeight int) image.Point {
	a.mu.Lock()
	pos := a.advancedPanelPos
	a.mu.Unlock()
	if pos == (image.Point{}) {
		pos = image.Pt(
			max(advancedPanelMargin, viewport.X-panelWidth-28),
			advancedPanelMargin+72,
		)
	}
	clamped := a.clampAdvancedPanelPos(pos, viewport, panelWidth, panelHeight)
	if clamped != pos {
		a.mu.Lock()
		a.advancedPanelPos = clamped
		a.mu.Unlock()
	}
	return clamped
}

func (a *App) advancedFloatingPanelHandleEvents(gtx layout.Context, panelRect image.Rectangle) {
	event.Op(gtx.Ops, &a.advancedPanelEventTag)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &a.advancedPanelEventTag)
	pass.Pop()

	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			pointer.Filter{
				Target:  &a.advancedPanelEventTag,
				Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel,
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
				a.closeAdvancedPanel()
			}
		case pointer.Event:
			headerRect := image.Rect(panelRect.Min.X, panelRect.Min.Y, panelRect.Max.X, panelRect.Min.Y+advancedPanelHeaderHeight)
			closeRect := image.Rect(panelRect.Max.X-56, panelRect.Min.Y, panelRect.Max.X, panelRect.Min.Y+56)
			point := image.Pt(int(evt.Position.X), int(evt.Position.Y))
			switch evt.Kind {
			case pointer.Press:
				if !point.In(headerRect) || point.In(closeRect) {
					continue
				}
				a.mu.Lock()
				a.advancedPanelDragActive = true
				a.advancedPanelDragPointerID = evt.PointerID
				a.advancedPanelDragOffset = point.Sub(panelRect.Min)
				a.mu.Unlock()
				gtx.Execute(pointer.GrabCmd{Tag: &a.advancedPanelEventTag, ID: evt.PointerID})
			case pointer.Drag:
				a.mu.Lock()
				active := a.advancedPanelDragActive && a.advancedPanelDragPointerID == evt.PointerID
				offset := a.advancedPanelDragOffset
				a.mu.Unlock()
				if !active {
					continue
				}
				width := panelRect.Dx()
				height := panelRect.Dy()
				viewport := gtx.Constraints.Max
				next := image.Pt(point.X-offset.X, point.Y-offset.Y)
				next = a.clampAdvancedPanelPos(next, viewport, width, height)
				a.mu.Lock()
				a.advancedPanelPos = next
				a.mu.Unlock()
				a.invalidateSoon(16 * time.Millisecond)
			case pointer.Release, pointer.Cancel:
				shouldPersist := false
				a.mu.Lock()
				if a.advancedPanelDragPointerID == evt.PointerID {
					a.advancedPanelDragActive = false
					a.advancedPanelDragPointerID = 0
					a.advancedPanelDragOffset = image.Point{}
					shouldPersist = true
				}
				a.mu.Unlock()
				if shouldPersist {
					if err := a.persistAdvancedPanelPrefs(); err != nil {
						a.appendLog("保存高级参数面板偏好失败: " + err.Error())
					}
				}
			}
		}
	}
}

func (a *App) layoutAdvancedFloatingPanel(gtx layout.Context) layout.Dimensions {
	viewport := gtx.Constraints.Max
	if viewport.X <= 0 || viewport.Y <= 0 {
		return layout.Dimensions{Size: viewport}
	}
	panelWidth := a.advancedPanelWidth(gtx)
	panelHeight := a.advancedPanelMaxHeight(gtx)
	if panelWidth <= 0 || panelHeight <= 0 {
		return layout.Dimensions{Size: viewport}
	}
	position := a.ensureAdvancedPanelPos(viewport, panelWidth, panelHeight)
	panelRect := image.Rectangle{Min: position, Max: position.Add(image.Pt(panelWidth, panelHeight))}
	a.advancedFloatingPanelHandleEvents(gtx, panelRect)

	cover := clip.Rect{Max: viewport}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	pointer.CursorDefault.Add(gtx.Ops)
	pass.Pop()
	cover.Pop()

	trans := op.Offset(position).Push(gtx.Ops)
	panelDims := fixedWidth(gtx, unit.Dp(float32(panelWidth)), func(gtx layout.Context) layout.Dimensions {
		maxHeight := panelHeight
		if maxHeight > viewport.Y {
			maxHeight = viewport.Y
		}
		return fixedHeight(gtx, unit.Dp(float32(maxHeight)), func(gtx layout.Context) layout.Dimensions {
			return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, unit.Dp(14), fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						bg := chooseColor(a.advancedPanelDragActive, accentAlpha(0x12), withAlpha(fluent.surface, 0xf2))
						return a.borderedSurface(gtx, bg, unit.Dp(13), rgba(0xffffff, 0x00), func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(12), Left: unit.Dp(16), Right: unit.Dp(16)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.borderedSurface(gtx, fluent.accentSoft, unit.Dp(10), accentAlpha(0x1c), func(gtx layout.Context) layout.Dimensions {
											return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return fixedWidth(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
													return fixedHeight(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
														return uiIconSettings.Layout(gtx, fluent.accent)
													})
												})
											})
										})
									}),
									layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
									layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
													layout.Rigid(func(gtx layout.Context) layout.Dimensions {
														return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
															return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
																return uiIconMoreHoriz.Layout(gtx, fluent.textDim)
															})
														})
													}),
													layout.Rigid(func(gtx layout.Context) layout.Dimensions {
														return a.sectionEyebrow(gtx, "高级参数")
													}),
												)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												return a.label(gtx, a.currentWorkspaceDisplayName(), unit.Sp(12), fluent.text, font.Medium)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												summary := strings.TrimSpace(a.advancedSummary())
												if summary == "" {
													summary = "按分组展开常用与进阶参数"
												}
												return a.label(gtx, summary, unit.Sp(11), fluent.textDim, font.Normal)
											}),
										)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.ghostIconButton(gtx, &a.advancedCloseButton, uiIconClose, false)
									}),
								)
							})
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(1), func(gtx layout.Context) layout.Dimensions {
							return a.surface(gtx, fluent.border, 0, layout.Spacer{}.Layout)
						})
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: unit.Dp(14), Bottom: unit.Dp(14), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.layoutAdvancedContent(gtx)
						})
					}),
				)
			})
		})
	})
	trans.Pop()

	clamped := a.clampAdvancedPanelPos(position, viewport, panelDims.Size.X, panelDims.Size.Y)
	if clamped != position {
		a.mu.Lock()
		a.advancedPanelPos = clamped
		a.mu.Unlock()
	}
	return layout.Dimensions{Size: viewport}
}
