package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (view *desktopWindowView) syncDetachedWorkspaceDraft(workspace desktopWorkspacePublication) {
	incoming := desktopDraftUpdateFromPublication(workspace)
	if view.draftWorkspaceID == workspace.ID {
		view.refreshDetachedDraftDirty()
		if view.draftDirty && incoming != view.detachedDraftUpdate() {
			return
		}
		if !view.draftDirty && view.draftRevision == workspace.DraftRevision && view.draftBaseline == incoming {
			return
		}
	}
	view.loadDetachedDraft(workspace.ID, workspace.DraftRevision, incoming)
}

func (view *desktopWindowView) loadDetachedDraft(workspaceID string, revision uint64, draft desktopDraftUpdate) {
	view.draftWorkspaceID = workspaceID
	view.draftRevision = revision
	view.draftBaseline = draft
	view.draftDirty = false
	view.promptEditor.SetText(draft.Prompt)
	view.negativeEditor.SetText(draft.NegativePrompt)
	view.draftMode = draft.Mode
	view.draftSize = draft.Size
	view.draftQuality = draft.Quality
	view.draftFormat = draft.OutputFormat
}

func (view *desktopWindowView) detachedDraftUpdate() desktopDraftUpdate {
	return normalizeDesktopDraftUpdate(desktopDraftUpdate{
		Prompt:         view.promptEditor.Text(),
		NegativePrompt: view.negativeEditor.Text(),
		Mode:           view.draftMode,
		Size:           view.draftSize,
		Quality:        view.draftQuality,
		OutputFormat:   view.draftFormat,
	})
}

func (view *desktopWindowView) refreshDetachedDraftDirty() {
	if view.draftWorkspaceID == "" {
		return
	}
	view.draftDirty = view.detachedDraftUpdate() != view.draftBaseline
}

func (view *desktopWindowView) handleDetachedWorkspaceDraftButtons(gtx layout.Context) {
	for index, option := range modeChoices {
		for view.draftModeButtons[index].Clicked(gtx) {
			view.draftMode = option.Value
			view.refreshDetachedDraftDirty()
		}
	}
	for index, option := range sizeChoices {
		for view.draftSizeButtons[index].Clicked(gtx) {
			view.draftSize = option.Value
			view.refreshDetachedDraftDirty()
		}
	}
	for index, option := range qualityChoices {
		for view.draftQualityButtons[index].Clicked(gtx) {
			view.draftQuality = option.Value
			view.refreshDetachedDraftDirty()
		}
	}
	for index, option := range formatChoices {
		for view.draftFormatButtons[index].Clicked(gtx) {
			view.draftFormat = option.Value
			view.refreshDetachedDraftDirty()
		}
	}
}

func (view *desktopWindowView) enqueueDetachedDraft(workspaceID string) bool {
	return view.enqueueDetachedDraftCommand(workspaceID, desktopCommandUpdateDraft)
}

func (view *desktopWindowView) enqueueDetachedDraftAndRun(workspaceID string) bool {
	return view.enqueueDetachedDraftCommand(workspaceID, desktopCommandUpdateDraftAndRun)
}

func (view *desktopWindowView) enqueueDetachedDraftCommand(workspaceID string, kind desktopCommandKind) bool {
	view.refreshDetachedDraftDirty()
	return view.enqueue(desktopCommand{
		Kind:          kind,
		WorkspaceID:   workspaceID,
		Draft:         view.detachedDraftUpdate(),
		DraftRevision: view.draftRevision,
	})
}

func (view *desktopWindowView) layoutDetachedWorkspaceInspector(gtx layout.Context, spec desktopThemeTokens, workspace desktopWorkspacePublication) layout.Dimensions {
	paint.FillShape(gtx.Ops, spec.Colors.inspector, clip.Rect{Max: gtx.Constraints.Max}.Op())
	paint.FillShape(gtx.Ops, spec.Colors.border, clip.Rect(image.Rect(0, 0, 1, gtx.Constraints.Max.Y)).Op())
	return view.draftList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return view.layoutDetachedWorkspaceInspectorContent(gtx, spec, workspace)
	})
}

func (view *desktopWindowView) layoutDetachedWorkspaceInspectorContent(gtx layout.Context, spec desktopThemeTokens, workspace desktopWorkspacePublication) layout.Dimensions {
	return layout.Inset{Top: unit.Dp(12), Bottom: unit.Dp(12), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(9))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.label(gtx, "工作区参数", unit.Sp(13), spec.Colors.text, font.SemiBold, 1)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.label(gtx, workspace.Name, unit.Sp(10), spec.Colors.textMuted, font.Normal, 1)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return detachedInspectorDivider(gtx, spec)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.detachedInspectorLabel(gtx, spec, "提示词")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.detachedDraftEditor(gtx, spec, &view.promptEditor, "描述主体、场景、镜头与风格", unit.Dp(132))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.detachedInspectorLabel(gtx, spec, "负面提示")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.detachedDraftEditor(gtx, spec, &view.negativeEditor, "可选", unit.Dp(64))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return detachedInspectorDivider(gtx, spec)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.detachedDraftChoices(gtx, spec, "模式", modeChoices, view.draftModeButtons, view.draftMode, nil)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.detachedDraftChoices(gtx, spec, "质量", qualityChoices, view.draftQualityButtons, view.draftQuality, nil)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.detachedDraftChoices(gtx, spec, "尺寸", sizeChoices, view.draftSizeButtons, view.draftSize, []int{0, 3, 4, 5})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.detachedDraftChoices(gtx, spec, "格式", formatChoices, view.draftFormatButtons, view.draftFormat, nil)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return view.button(gtx, spec, &view.applyDraftButton, view.icons.save, "应用到工作区", desktopButtonPrimary)
			}),
		)
	})
}

func (view *desktopWindowView) detachedInspectorLabel(gtx layout.Context, spec desktopThemeTokens, label string) layout.Dimensions {
	return view.label(gtx, label, unit.Sp(9), spec.Colors.textMuted, font.SemiBold, 1)
}

func (view *desktopWindowView) detachedDraftEditor(gtx layout.Context, spec desktopThemeTokens, editor *widget.Editor, hint string, height unit.Dp) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Dp(height)
	gtx.Constraints.Max.Y = gtx.Dp(height)
	paint.FillShape(gtx.Ops, spec.Colors.surface, clip.RRect{Rect: image.Rectangle{Max: gtx.Constraints.Max}, NE: gtx.Dp(spec.Metrics.InputRadius), NW: gtx.Dp(spec.Metrics.InputRadius), SE: gtx.Dp(spec.Metrics.InputRadius), SW: gtx.Dp(spec.Metrics.InputRadius)}.Op(gtx.Ops))
	paintWorkflowRectOutline(gtx, image.Rectangle{Max: gtx.Constraints.Max}, gtx.Dp(spec.Metrics.InputRadius), 1, spec.Colors.border2)
	return layout.Inset{Top: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(9), Right: unit.Dp(9)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		style := material.Editor(view.theme, editor, hint)
		style.Color = spec.Colors.text
		style.HintColor = spec.Colors.textDim
		style.TextSize = view.scaledTextSize(unit.Sp(11))
		return style.Layout(gtx)
	})
}

func (view *desktopWindowView) detachedDraftChoices(gtx layout.Context, spec desktopThemeTokens, label string, choices []choice, buttons []widget.Clickable, selected string, indexes []int) layout.Dimensions {
	if len(indexes) == 0 {
		indexes = make([]int, len(choices))
		for index := range choices {
			indexes[index] = index
		}
	}
	rows := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return view.detachedInspectorLabel(gtx, spec, label)
		}),
	}
	for start := 0; start < len(indexes); start += 2 {
		end := min(start+2, len(indexes))
		row := append([]int(nil), indexes[start:end]...)
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(row))
			for _, index := range row {
				index := index
				tone := desktopButtonNeutral
				if choices[index].Value == selected {
					tone = desktopButtonSelected
				}
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return view.button(gtx, spec, &buttons[index], nil, choices[index].Label, tone)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx, children...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx, rows...)
}

func detachedInspectorDivider(gtx layout.Context, spec desktopThemeTokens) layout.Dimensions {
	height := max(1, gtx.Dp(unit.Dp(1)))
	paint.FillShape(gtx.Ops, spec.Colors.border, clip.Rect{Max: image.Pt(gtx.Constraints.Max.X, height)}.Op())
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, height)}
}
