package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	gioCompat "image-studio/gio-client/internal/compat"
	sharedCompat "image-studio/shared/compat"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

func reduceCustomAspectRatio(width int, height int) (int, int) {
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	a, b := width, height
	for b != 0 {
		a, b = b, a%b
	}
	if a <= 0 {
		return 0, 0
	}
	return width / a, height / a
}

func buildCustomAspectRatioID(width int, height int) string {
	w, h := reduceCustomAspectRatio(width, height)
	if w <= 0 || h <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", w, h)
}

func isBuiltInAspectRatioID(id string) bool {
	switch strings.TrimSpace(id) {
	case "1:1", "3:2", "2:3", "16:9", "9:16", "7:4", "4:7":
		return true
	default:
		return false
	}
}

func (a *App) customAspectRatioListButton(id string) *widget.Clickable {
	if a.customAspectRatioListButtons == nil {
		a.customAspectRatioListButtons = map[string]*widget.Clickable{}
	}
	if btn, ok := a.customAspectRatioListButtons[id]; ok {
		return btn
	}
	btn := new(widget.Clickable)
	a.customAspectRatioListButtons[id] = btn
	return btn
}

func (a *App) openCustomAspectRatioManager() {
	a.mu.Lock()
	if a.selectedCustomAspectRatioID == "" && len(a.customAspectRatios) > 0 {
		a.selectedCustomAspectRatioID = strings.TrimSpace(a.customAspectRatios[0].ID)
	}
	a.customAspectRatioManagerOpen = true
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closeCustomAspectRatioManager() {
	a.mu.Lock()
	a.customAspectRatioManagerOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) addCustomAspectRatio() {
	width, _ := strconv.Atoi(strings.TrimSpace(a.customAspectWidthInput.Text()))
	height, _ := strconv.Atoi(strings.TrimSpace(a.customAspectHeightInput.Text()))
	if width <= 0 || height <= 0 {
		a.appendLog("请输入有效的自定义比例宽高")
		return
	}
	id := buildCustomAspectRatioID(width, height)
	if id == "" {
		a.appendLog("无法生成自定义比例")
		return
	}
	if isBuiltInAspectRatioID(id) {
		a.appendLog("这个比例已经内置了")
		return
	}
	state, _, err := gioCompat.LoadState()
	if err != nil {
		a.appendLog("保存自定义比例失败: " + err.Error())
		return
	}
	state = sharedCompat.Normalize(state)
	for _, ratio := range state.Settings.CustomAspectRatios {
		if strings.TrimSpace(ratio.ID) == id {
			a.appendLog("比例已存在: " + id)
			return
		}
	}
	w, h := reduceCustomAspectRatio(width, height)
	ratio := sharedCompat.CustomAspectRatio{
		ID:        id,
		Label:     id,
		Width:     w,
		Height:    h,
		CreatedAt: time.Now().UnixMilli(),
	}
	state.Settings.CustomAspectRatios = append(state.Settings.CustomAspectRatios, ratio)
	state.UpdatedAt = time.Now().UnixMilli()
	if err := gioCompat.SaveState(state); err != nil {
		a.appendLog("保存自定义比例失败: " + err.Error())
		return
	}
	a.mu.Lock()
	a.customAspectRatios = append([]sharedCompat.CustomAspectRatio(nil), state.Settings.CustomAspectRatios...)
	a.selectedCustomAspectRatioID = ratio.ID
	a.customAspectWidthInput.SetText("")
	a.customAspectHeightInput.SetText("")
	a.mu.Unlock()
	a.appendLog("已添加自定义比例: " + id)
	a.invalidateNow()
}

func (a *App) deleteSelectedCustomAspectRatio() {
	targetID := ""
	a.mu.Lock()
	targetID = strings.TrimSpace(a.selectedCustomAspectRatioID)
	a.mu.Unlock()
	if targetID == "" {
		a.appendLog("当前没有可删除的自定义比例")
		return
	}
	state, _, err := gioCompat.LoadState()
	if err != nil {
		a.appendLog("删除自定义比例失败: " + err.Error())
		return
	}
	state = sharedCompat.Normalize(state)
	next := make([]sharedCompat.CustomAspectRatio, 0, len(state.Settings.CustomAspectRatios))
	removed := false
	for _, ratio := range state.Settings.CustomAspectRatios {
		if strings.TrimSpace(ratio.ID) == targetID {
			removed = true
			continue
		}
		next = append(next, ratio)
	}
	if !removed {
		a.appendLog("当前自定义比例不存在")
		return
	}
	state.Settings.CustomAspectRatios = next
	state.UpdatedAt = time.Now().UnixMilli()
	if err := gioCompat.SaveState(state); err != nil {
		a.appendLog("删除自定义比例失败: " + err.Error())
		return
	}
	a.mu.Lock()
	a.customAspectRatios = append([]sharedCompat.CustomAspectRatio(nil), next...)
	if len(next) > 0 {
		a.selectedCustomAspectRatioID = strings.TrimSpace(next[0].ID)
	} else {
		a.selectedCustomAspectRatioID = ""
	}
	a.mu.Unlock()
	a.appendLog("已删除自定义比例: " + targetID)
	a.invalidateNow()
}

func (a *App) layoutCustomAspectRatioManagerModal(gtx layout.Context) layout.Dimensions {
	for a.closePromptHelperButton.Clicked(gtx) {
		a.closeCustomAspectRatioManager()
	}
	for a.addCustomAspectRatioButton.Clicked(gtx) {
		a.addCustomAspectRatio()
	}
	for a.deleteCustomAspectRatioButton.Clicked(gtx) {
		a.deleteSelectedCustomAspectRatio()
	}
	for _, ratio := range a.customAspectRatios {
		btn := a.customAspectRatioListButton("custom-aspect:" + ratio.ID)
		for btn.Clicked(gtx) {
			a.mu.Lock()
			a.selectedCustomAspectRatioID = ratio.ID
			a.mu.Unlock()
			a.invalidateNow()
		}
	}

	return a.layoutStandardModal(
		gtx,
		unit.Dp(760),
		unit.Dp(560),
		"自定义比例",
		"",
		&a.closePromptHelperButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "新增后会直接出现在 Gio 的比例按钮区，并和 WebView 共用这份状态。", unit.Sp(11), fluent.textDim, font.Normal)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.field(gtx, "宽", &a.customAspectWidthInput, "21", unit.Dp(42))
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.field(gtx, "高", &a.customAspectHeightInput, "9", unit.Dp(42))
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(110), func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.addCustomAspectRatioButton, uiIconAdd, "添加", true)
							})
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if len(a.customAspectRatios) == 0 {
						return a.label(gtx, "当前还没有自定义比例。", unit.Sp(11), fluent.textDim, font.Normal)
					}
					rows := make([]layout.FlexChild, 0, len(a.customAspectRatios))
					for _, ratio := range a.customAspectRatios {
						ratio := ratio
						rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							selected := strings.TrimSpace(ratio.ID) == strings.TrimSpace(a.selectedCustomAspectRatioID)
							return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.surfaceButton(
									gtx,
									a.customAspectRatioListButton("custom-aspect:"+ratio.ID),
									chooseColor(selected, fluent.accentSoft, rgba(0xffffff, 0x00)),
									chooseColor(selected, accentAlpha(0x18), fluent.surface2),
									chooseColor(selected, accentAlpha(0x38), fluent.border),
									fluentCardRadius,
									layout.Inset{Top: 9, Bottom: 9, Left: 10, Right: 10},
									func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
											layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
												return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
													layout.Rigid(func(gtx layout.Context) layout.Dimensions {
														return a.singleLineLabel(gtx, ratio.Label, unit.Sp(11), chooseColor(selected, fluent.accent, fluent.text), font.SemiBold)
													}),
													layout.Rigid(func(gtx layout.Context) layout.Dimensions {
														return a.singleLineLabel(gtx, "归一化比例 "+ratio.ID, unit.Sp(10), fluent.textDim, font.Normal)
													}),
												)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												w, h := aspectShapeFromRatio(ratio.Width, ratio.Height)
												return a.surface(gtx, fluent.accentSoft, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
													return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
														return a.borderedSurface(gtx, fluent.surface, unit.Dp(3), fluent.accent, func(gtx layout.Context) layout.Dimensions {
															return fixedPixelWidth(gtx, w, func(gtx layout.Context) layout.Dimensions {
																return fixedPixelHeight(gtx, h, func(gtx layout.Context) layout.Dimensions {
																	return layout.Dimensions{Size: gtx.Constraints.Min}
																})
															})
														})
													})
												})
											}),
										)
									},
								)
							})
						}))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(128), func(gtx layout.Context) layout.Dimensions {
						return a.compactIconTextButton(gtx, &a.deleteCustomAspectRatioButton, uiIconDelete, "删除所选", false)
					})
				}),
			)
		},
	)
}
