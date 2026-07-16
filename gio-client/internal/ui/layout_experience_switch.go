package ui

import (
	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

type experienceSwitchContract struct {
	width  unit.Dp
	height unit.Dp
	radius unit.Dp
}

func experienceSwitchContractForStyle(style string, spec desktopThemeTokens) experienceSwitchContract {
	if normalizeDesktopStyle(style) == desktopStyleMacOS {
		return experienceSwitchContract{width: unit.Dp(176), height: unit.Dp(28), radius: unit.Dp(7)}
	}
	return experienceSwitchContract{width: unit.Dp(224), radius: spec.Metrics.ControlRadius}
}

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
	contract := experienceSwitchContractForStyle(a.desktopStyle, spec)
	if spec.Style == desktopStyleMacOS {
		return a.layoutAppleExperienceSwitch(gtx, spec, contract)
	}
	return fixedWidth(gtx, contract.width, func(gtx layout.Context) layout.Dimensions {
		return a.borderedSurface(gtx, spec.Colors.surface2, contract.radius, spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
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

func (a *App) layoutAppleExperienceSwitch(gtx layout.Context, spec desktopThemeTokens, contract experienceSwitchContract) layout.Dimensions {
	return fixedWidth(gtx, contract.width, func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, contract.height, func(gtx layout.Context) layout.Dimensions {
			return a.borderedSurface(gtx, spec.Colors.surface2, contract.radius, spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 2, Bottom: 2, Left: 2, Right: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(2))}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							selected := a.experienceMode == experienceModeSimple
							return a.layoutAppleExperienceSwitchButton(gtx, &a.experienceModeButtons[0], uiIconPhoto, "简易", selected, spec)
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							selected := a.experienceMode == experienceModeWorkflow
							return a.layoutAppleExperienceSwitchButton(gtx, &a.experienceModeButtons[1], uiIconWorkflow, "工作流", selected, spec)
						}),
					)
				})
			})
		})
	})
}

func (a *App) layoutAppleExperienceSwitchButton(
	gtx layout.Context,
	button *widget.Clickable,
	icon *widget.Icon,
	label string,
	selected bool,
	spec desktopThemeTokens,
) layout.Dimensions {
	fill := rgba(0xffffff, 0x00)
	hover := spec.Colors.surface
	border := rgba(0xffffff, 0x00)
	foreground := spec.Colors.textMuted
	if selected {
		fill = spec.Colors.surfaceElevated
		hover = spec.Colors.surfaceElevated
		border = spec.Colors.border
		foreground = spec.Colors.text
	}
	return a.surfaceButton(
		gtx,
		button,
		fill,
		hover,
		border,
		spec.Metrics.ControlRadius,
		layout.Inset{Top: 4, Bottom: 4, Left: 10, Right: 10},
		func(gtx layout.Context) layout.Dimensions {
			semantic.LabelOp(label).Add(gtx.Ops)
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return icon.Layout(gtx, foreground)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					weight := font.Medium
					if selected {
						weight = font.SemiBold
					}
					return a.singleLineLabel(gtx, label, unit.Sp(11), foreground, weight)
				}),
			)
		},
		selected,
	)
}
