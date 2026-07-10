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

func nextPromptTemplateLabel(items []sharedCompat.PromptTemplate) string {
	used := map[int]struct{}{}
	for _, item := range items {
		label := strings.TrimSpace(item.Label)
		if !strings.HasPrefix(label, "模板") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(label, "模板"))
		value, err := strconv.Atoi(raw)
		if err == nil && value > 0 {
			used[value] = struct{}{}
		}
	}
	for i := 1; ; i++ {
		if _, ok := used[i]; !ok {
			return "模板" + strconv.Itoa(i)
		}
	}
}

func (a *App) promptTemplateListButton(id string) *widget.Clickable {
	if a.promptTemplateListButtons == nil {
		a.promptTemplateListButtons = map[string]*widget.Clickable{}
	}
	if btn, ok := a.promptTemplateListButtons[id]; ok {
		return btn
	}
	btn := new(widget.Clickable)
	a.promptTemplateListButtons[id] = btn
	return btn
}

func (a *App) openPromptTemplateManager() {
	a.mu.Lock()
	a.promptHelperOpen = false
	if a.selectedPromptTemplateID == "" && len(a.promptTemplates) > 0 {
		a.selectedPromptTemplateID = strings.TrimSpace(a.promptTemplates[0].ID)
	}
	if a.selectedPromptTemplateID != "" {
		a.loadPromptTemplateDraftLocked(a.selectedPromptTemplateID)
	} else {
		a.promptTemplateLabelInput.SetText(nextPromptTemplateLabel(a.promptTemplates))
		a.promptTemplateTextInput.SetText(strings.TrimSpace(a.promptInput.Text()))
	}
	a.promptTemplateManagerOpen = true
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closePromptTemplateManager() {
	a.mu.Lock()
	a.promptTemplateManagerOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) loadPromptTemplateDraftLocked(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, item := range a.promptTemplates {
		if strings.TrimSpace(item.ID) != id {
			continue
		}
		a.selectedPromptTemplateID = id
		a.promptTemplateLabelInput.SetText(strings.TrimSpace(item.Label))
		a.promptTemplateTextInput.SetText(strings.TrimSpace(item.Text))
		return true
	}
	return false
}

func (a *App) startNewPromptTemplateDraft() {
	a.mu.Lock()
	a.selectedPromptTemplateID = ""
	a.promptTemplateLabelInput.SetText(nextPromptTemplateLabel(a.promptTemplates))
	a.promptTemplateTextInput.SetText(strings.TrimSpace(a.promptInput.Text()))
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) savePromptTemplateDraft() {
	label := strings.TrimSpace(a.promptTemplateLabelInput.Text())
	text := strings.TrimSpace(a.promptTemplateTextInput.Text())
	if label == "" || text == "" {
		a.appendLog("模板标题和内容不能为空")
		return
	}
	now := time.Now().UnixMilli()
	targetID := ""
	a.mu.Lock()
	targetID = strings.TrimSpace(a.selectedPromptTemplateID)
	a.mu.Unlock()
	updated := false
	var templates []sharedCompat.PromptTemplate
	err := gioCompat.UpdateState(func(state *sharedCompat.State) error {
		*state = sharedCompat.Normalize(*state)
		for i := range state.Settings.PromptTemplates {
			if strings.TrimSpace(state.Settings.PromptTemplates[i].ID) != targetID || targetID == "" {
				continue
			}
			state.Settings.PromptTemplates[i].Label = label
			state.Settings.PromptTemplates[i].Text = text
			state.Settings.PromptTemplates[i].UpdatedAt = now
			updated = true
			break
		}
		if !updated {
			targetID = fmt.Sprintf("tpl-%d", now)
			state.Settings.PromptTemplates = append([]sharedCompat.PromptTemplate{{
				ID:        targetID,
				Label:     label,
				Text:      text,
				CreatedAt: now,
				UpdatedAt: now,
			}}, state.Settings.PromptTemplates...)
		}
		state.UpdatedAt = now
		templates = append([]sharedCompat.PromptTemplate(nil), state.Settings.PromptTemplates...)
		return nil
	})
	if err != nil {
		a.appendLog("保存模板失败: " + err.Error())
		return
	}
	a.mu.Lock()
	a.setPromptTemplatesLocked(templates)
	a.selectedPromptTemplateID = targetID
	a.promptTemplateManagerOpen = true
	a.loadPromptTemplateDraftLocked(targetID)
	a.mu.Unlock()
	if updated {
		a.appendLog("已更新模板: " + label)
	} else {
		a.appendLog("已保存模板: " + label)
	}
	a.invalidateNow()
}

func (a *App) deleteSelectedPromptTemplate() {
	targetID := ""
	a.mu.Lock()
	targetID = strings.TrimSpace(a.selectedPromptTemplateID)
	a.mu.Unlock()
	if targetID == "" {
		a.appendLog("当前没有可删除的模板")
		return
	}
	var next []sharedCompat.PromptTemplate
	removedLabel := ""
	removed := false
	err := gioCompat.UpdateState(func(state *sharedCompat.State) error {
		*state = sharedCompat.Normalize(*state)
		next = make([]sharedCompat.PromptTemplate, 0, len(state.Settings.PromptTemplates))
		for _, item := range state.Settings.PromptTemplates {
			if strings.TrimSpace(item.ID) == targetID {
				removedLabel = strings.TrimSpace(item.Label)
				removed = true
				continue
			}
			next = append(next, item)
		}
		if removed {
			state.Settings.PromptTemplates = next
			state.UpdatedAt = time.Now().UnixMilli()
		}
		return nil
	})
	if err != nil {
		a.appendLog("删除模板失败: " + err.Error())
		return
	}
	if !removed {
		a.appendLog("当前模板不存在")
		return
	}
	a.mu.Lock()
	a.setPromptTemplatesLocked(next)
	if len(next) > 0 {
		a.selectedPromptTemplateID = strings.TrimSpace(next[0].ID)
		a.loadPromptTemplateDraftLocked(a.selectedPromptTemplateID)
	} else {
		a.selectedPromptTemplateID = ""
		a.promptTemplateLabelInput.SetText(nextPromptTemplateLabel(nil))
		a.promptTemplateTextInput.SetText("")
	}
	a.mu.Unlock()
	if removedLabel == "" {
		removedLabel = targetID
	}
	a.appendLog("已删除模板: " + removedLabel)
	a.invalidateNow()
}

func (a *App) layoutPromptTemplateManagerModal(gtx layout.Context, snap snapshot) layout.Dimensions {
	for a.closePromptHelperButton.Clicked(gtx) {
		a.closePromptTemplateManager()
	}
	for a.newPromptTemplateButton.Clicked(gtx) {
		a.startNewPromptTemplateDraft()
	}
	for a.savePromptTemplateButton.Clicked(gtx) {
		a.savePromptTemplateDraft()
	}
	for a.deletePromptTemplateButton.Clicked(gtx) {
		a.deleteSelectedPromptTemplate()
	}
	for _, item := range snap.PromptTemplates {
		btn := a.promptTemplateListButton("prompt-template-list:" + item.ID)
		for btn.Clicked(gtx) {
			a.mu.Lock()
			a.loadPromptTemplateDraftLocked(item.ID)
			a.mu.Unlock()
			a.invalidateNow()
		}
	}

	return a.layoutStandardModal(
		gtx,
		unit.Dp(860),
		unit.Dp(620),
		"自定义提示词模板",
		"",
		&a.closePromptHelperButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(260), func(gtx layout.Context) layout.Dimensions {
						return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								children := []layout.FlexChild{
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
											layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
												return a.compactIconTextButton(gtx, &a.newPromptTemplateButton, uiIconAdd, "新建", false)
											}),
											layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
												return a.compactIconTextButton(gtx, &a.savePromptTemplateButton, uiIconSave, "保存", true)
											}),
										)
									}),
									layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
								}
								if len(snap.PromptTemplates) == 0 {
									children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, "还没有自定义模板。可从当前 prompt 新建一条。", unit.Sp(11), fluent.textDim, font.Normal)
									}))
								} else {
									for _, item := range snap.PromptTemplates {
										item := item
										children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											selected := strings.TrimSpace(item.ID) == strings.TrimSpace(a.selectedPromptTemplateID)
											return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return a.surfaceButton(
													gtx,
													a.promptTemplateListButton("prompt-template-list:"+item.ID),
													chooseColor(selected, fluent.accentSoft, rgba(0xffffff, 0x00)),
													chooseColor(selected, accentAlpha(0x18), fluent.surface2),
													chooseColor(selected, accentAlpha(0x38), fluent.border),
													fluentCardRadius,
													layout.Inset{Top: 9, Bottom: 9, Left: 10, Right: 10},
													func(gtx layout.Context) layout.Dimensions {
														return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return a.singleLineLabel(gtx, strings.TrimSpace(item.Label), unit.Sp(11), chooseColor(selected, fluent.accent, fluent.text), font.SemiBold)
															}),
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return a.clampedLabel(gtx, strings.TrimSpace(item.Text), unit.Sp(10), fluent.textDim, font.Normal, 2)
															}),
														)
													},
												)
											})
										}))
									}
								}
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
							})
						})
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									title := "新建模板"
									if strings.TrimSpace(a.selectedPromptTemplateID) != "" {
										title = "编辑模板"
									}
									return a.label(gtx, title, unit.Sp(12), fluent.text, font.SemiBold)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.field(gtx, "模板标题", &a.promptTemplateLabelInput, "模板1", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, "模板内容", unit.Sp(11), fluent.textMuted, font.SemiBold)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return fixedHeight(gtx, unit.Dp(320), func(gtx layout.Context) layout.Dimensions {
												return a.borderedSurface(gtx, fluent.surface, fluentInputRadius, fluent.border2, func(gtx layout.Context) layout.Dimensions {
													return layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
														return a.editorText(gtx, &a.promptTemplateTextInput, "输入模板内容", unit.Sp(13))
													})
												})
											})
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.compactIconTextButton(gtx, &a.savePromptTemplateButton, uiIconSave, "保存模板", true)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return fixedWidth(gtx, unit.Dp(112), func(gtx layout.Context) layout.Dimensions {
												return a.compactIconTextButton(gtx, &a.deletePromptTemplateButton, uiIconDelete, "删除", false)
											})
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, fmt.Sprintf("当前已保存 %d 条自定义模板；内置模板仍会保留在「模板 / 历史」里。", len(snap.PromptTemplates)), unit.Sp(10), fluent.textDim, font.Normal)
								}),
							)
						})
					})
				}),
			)
		},
	)
}
