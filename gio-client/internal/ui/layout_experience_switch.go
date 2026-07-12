package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
)

func (a *App) layoutExperienceSwitch(gtx layout.Context) layout.Dimensions {
	for a.experienceModeButtons[0].Clicked(gtx) {
		a.persistExperienceMode(experienceModeSimple)
	}
	for a.experienceModeButtons[1].Clicked(gtx) {
		a.ensureWorkflowGraph(a.activeWorkspaceID)
		a.persistExperienceMode(experienceModeWorkflow)
		a.applyDefaultWindowLayout()
	}
	spec := desktopThemeSpec(a.desktopStyle, a.resolvedThemeMode)
	return fixedWidth(gtx, unit.Dp(224), func(gtx layout.Context) layout.Dimensions {
		return a.borderedSurface(gtx, spec.Colors.surface2, spec.Metrics.ControlRadius, spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 2, Bottom: 2, Left: 2, Right: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(2))}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						selected := a.experienceMode == experienceModeSimple
						return a.compactIconTextButton(gtx, &a.experienceModeButtons[0], uiIconPhoto, "简易", selected, selected)
					}),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						selected := a.experienceMode == experienceModeWorkflow
						return a.compactIconTextButton(gtx, &a.experienceModeButtons[1], uiIconWorkflow, "工作流", selected, selected)
					}),
				)
			})
		})
	})
}
