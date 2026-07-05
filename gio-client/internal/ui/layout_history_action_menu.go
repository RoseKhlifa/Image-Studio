package ui

import (
	"image"
	"io"
	"strings"

	sharedCompat "image-studio/shared/compat"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"
	"gioui.org/widget"
)

const historyActionMenuWidth = 236
const historyActionMenuMargin = 8

type historyActionMenuEntry struct {
	ID              string
	Label           string
	Icon            *widget.Icon
	Accent          bool
	Danger          bool
	Disabled        bool
	SeparatorBefore bool
}

type historyPointerTarget struct{}

func (a *App) historyPointerTarget(key string) *historyPointerTarget {
	if a.historyPointerTargets == nil {
		a.historyPointerTargets = map[string]*historyPointerTarget{}
	}
	if target, ok := a.historyPointerTargets[key]; ok {
		return target
	}
	target := new(historyPointerTarget)
	a.historyPointerTargets[key] = target
	return target
}

func (a *App) historySecondaryMenuSurface(
	gtx layout.Context,
	targetKey string,
	item sharedCompat.HistoryItem,
	context string,
	body layout.Widget,
) layout.Dimensions {
	tag := a.historyPointerTarget(targetKey)
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: tag, Kinds: pointer.Press})
		if !ok {
			break
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		secondary := pe.Buttons == pointer.ButtonSecondary || pe.Modifiers.Contain(key.ModCtrl)
		if !secondary {
			continue
		}
		a.openHistoryActionMenu(item, context)
	}
	macro := op.Record(gtx.Ops)
	dims := body(gtx)
	call := macro.Stop()
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, tag)
	call.Add(gtx.Ops)
	return dims
}

func (a *App) historyActionMenuEntries(item sharedCompat.HistoryItem, context string, compareItemID string) []historyActionMenuEntry {
	compareActive := compareItemActive(item.ID, compareItemID)
	canReuse := strings.TrimSpace(item.SavedPath) != "" || strings.TrimSpace(item.ImageB64) != ""
	canDragOut := canDragOutHistoryItem(item)
	canOpenRaw := strings.TrimSpace(item.RawPath) != ""
	canCopyPath := strings.TrimSpace(item.SavedPath) != ""
	entries := []historyActionMenuEntry{
		{ID: "detail", Label: "详情", Icon: uiIconInfo},
		{ID: "copy-prompt", Label: "复制 prompt", Icon: uiIconCopy, SeparatorBefore: true, Disabled: strings.TrimSpace(item.Prompt) == ""},
		{ID: "copy-path", Label: "复制本地路径", Icon: uiIconFolder, Disabled: !canCopyPath},
		{ID: "drag-out", Label: "拖出复制", Icon: uiIconLaunch, Disabled: !canDragOut},
		{ID: "open-raw", Label: "查看 Raw 响应", Icon: uiIconList, Disabled: !canOpenRaw},
		{ID: "apply", Label: "应用参数(不生成)", Icon: uiIconCheck, SeparatorBefore: true},
		{ID: "rerun", Label: "以此参数重新生成", Icon: uiIconRefresh},
		{ID: "reuse", Label: "设为源图", Icon: uiIconSource, SeparatorBefore: true, Disabled: !canReuse},
		{ID: "compare", Label: chooseCompareMenuLabel(compareActive), Icon: uiIconCompare, Accent: compareActive},
		{ID: "delete", Label: "删除", Icon: uiIconDelete, Danger: true, SeparatorBefore: true},
	}
	if strings.TrimSpace(context) == "latest" {
		for idx := range entries {
			if entries[idx].ID == "detail" {
				entries[idx].Label = "更多"
				break
			}
		}
	}
	return entries
}

func chooseCompareMenuLabel(active bool) string {
	if active {
		return "取消对比"
	}
	return "用作对比图 (B)"
}

func (a *App) triggerHistoryActionMenu(gtx layout.Context, action string, item sharedCompat.HistoryItem, context string) {
	action = strings.TrimSpace(action)
	switch action {
	case "detail":
		a.openResultDetail(item)
		a.closeHistoryActionMenu()
	case "copy-prompt":
		text := strings.TrimSpace(item.Prompt)
		if text == "" {
			return
		}
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
		a.appendLog("已复制 prompt")
		a.closeHistoryActionMenu()
	case "copy-path":
		text := strings.TrimSpace(item.SavedPath)
		if text == "" {
			return
		}
		gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
		a.appendLog("已复制文件路径")
		a.closeHistoryActionMenu()
	case "drag-out":
		if _, err := a.dragOutHistoryItem(item); err != nil {
			a.appendLog("拖出复制失败: " + err.Error())
			return
		}
		a.closeHistoryActionMenu()
	case "open-raw":
		raw := strings.TrimSpace(item.RawPath)
		if raw == "" {
			return
		}
		a.openRawResponseModal(raw)
		a.closeHistoryActionMenu()
	case "apply":
		a.applyHistoryParams(item)
		a.closeHistoryActionMenu()
	case "rerun":
		a.closeHistoryActionMenu()
		a.regenerateFromHistoryItem(item)
	case "reuse":
		a.reuseHistoryItemAsSource(item)
		a.appendLog("已将历史结果加入源图: " + shortPrompt(item.Prompt))
		a.closeHistoryActionMenu()
	case "compare":
		if err := a.toggleCompareItem(item); err != nil && !isMissingPreview(err) {
			a.appendLog("载入对比图失败: " + err.Error())
			return
		}
		a.closeHistoryActionMenu()
	case "delete":
		a.deleteHistoryItem(item.ID)
	default:
		a.closeHistoryActionMenu()
	}
}

func historyActionMenuEstimatedHeight(entries []historyActionMenuEntry) int {
	height := 8
	for _, entry := range entries {
		if entry.SeparatorBefore {
			height += 10
		}
		height += 36
	}
	return height + 8
}

func clampHistoryActionMenuPos(pos image.Point, viewport image.Point, width int, height int) image.Point {
	maxX := max(historyActionMenuMargin, viewport.X-width-historyActionMenuMargin)
	maxY := max(historyActionMenuMargin, viewport.Y-height-historyActionMenuMargin)
	return image.Pt(
		clampInt(pos.X, historyActionMenuMargin, maxX),
		clampInt(pos.Y, historyActionMenuMargin, maxY),
	)
}

func (a *App) layoutHistoryActionMenuModal(gtx layout.Context, snap snapshot) layout.Dimensions {
	item := snap.HistoryActionMenuItem
	if strings.TrimSpace(item.ID) == "" && strings.TrimSpace(item.SavedPath) == "" {
		return layout.Dimensions{}
	}
	entries := a.historyActionMenuEntries(item, snap.HistoryActionMenuContext, snap.Compare.Item.ID)
	for _, entry := range entries {
		if entry.Disabled {
			continue
		}
		btn := a.historyActionButton("menu:" + entry.ID + ":" + item.ID + ":" + snap.HistoryActionMenuContext)
		for btn.Clicked(gtx) {
			a.triggerHistoryActionMenu(gtx, entry.ID, item, snap.HistoryActionMenuContext)
		}
	}
	viewport := gtx.Constraints.Max
	if viewport.X <= 0 || viewport.Y <= 0 {
		return layout.Dimensions{Size: viewport}
	}
	width := min(gtx.Dp(unit.Dp(historyActionMenuWidth)), viewport.X-historyActionMenuMargin*2)
	height := min(historyActionMenuEstimatedHeight(entries), viewport.Y-historyActionMenuMargin*2)
	a.mu.Lock()
	anchor := a.historyActionMenuPos
	a.mu.Unlock()
	if anchor == (image.Point{}) {
		anchor = image.Pt(240, 180)
	}
	position := clampHistoryActionMenuPos(anchor, viewport, width, height)
	panelRect := image.Rectangle{Min: position, Max: position.Add(image.Pt(width, height))}

	clipArea := clip.Rect{Max: viewport}.Push(gtx.Ops)
	pass := pointer.PassOp{}.Push(gtx.Ops)
	event.Op(gtx.Ops, &a.historyActionMenuEventTag)
	pass.Pop()
	clipArea.Pop()
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameEscape},
			pointer.Filter{
				Target:  &a.historyActionMenuEventTag,
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
				a.closeHistoryActionMenu()
			}
		case pointer.Event:
			point := image.Pt(int(evt.Position.X), int(evt.Position.Y))
			if point.In(panelRect) {
				continue
			}
			a.closeHistoryActionMenu()
		}
	}

	trans := op.Offset(position).Push(gtx.Ops)
	_ = fixedWidth(gtx, unit.Dp(float32(width)), func(gtx layout.Context) layout.Dimensions {
		return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, unit.Dp(12), fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(4)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				children := make([]layout.FlexChild, 0, len(entries)*2)
				for idx := range entries {
					entry := entries[idx]
					if entry.SeparatorBefore && len(children) > 0 {
						children = append(children,
							layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(1), func(gtx layout.Context) layout.Dimensions {
									return a.surface(gtx, fluent.border, 0, layout.Spacer{}.Layout)
								})
							}),
							layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
						)
					}
					btn := a.historyActionButton("menu:" + entry.ID + ":" + item.ID + ":" + snap.HistoryActionMenuContext)
					children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.layoutHistoryActionMenuEntry(gtx, btn, entry)
					}))
					if idx != len(entries)-1 {
						children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout))
					}
				}
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		})
	})
	trans.Pop()
	return layout.Dimensions{Size: viewport}
}

func (a *App) layoutHistoryActionMenuEntry(gtx layout.Context, btn *widget.Clickable, entry historyActionMenuEntry) layout.Dimensions {
	bg := fluent.surface
	hoverBg := fluent.surface2
	border := fluent.border
	fg := fluent.text
	if entry.Accent {
		bg = fluent.accentSoft
		hoverBg = accentAlpha(0x24)
		border = accentAlpha(0x38)
		fg = fluent.accent
	}
	if entry.Danger {
		fg = fluent.danger
	}
	if entry.Disabled {
		fg = fluent.textDim
	}
	return a.surfaceButton(
		gtx,
		btn,
		bg,
		hoverBg,
		border,
		fluentControlRadius,
		layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if entry.Icon == nil {
						return layout.Dimensions{}
					}
					return fixedWidth(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
							return entry.Icon.Layout(gtx, fg)
						})
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					weight := font.Medium
					if entry.Accent {
						weight = font.SemiBold
					}
					return a.label(gtx, entry.Label, unit.Sp(12), fg, weight)
				}),
			)
		},
	)
}
