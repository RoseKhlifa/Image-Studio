package ui

import (
	"image-studio/gio-client/internal/desktopstate"
	"image-studio/gio-client/internal/windowing"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

func (a *App) handleDesktopPreferenceEvents(gtx layout.Context) {
	for a.desktopStyleButtons[0].Clicked(gtx) {
		a.persistDesktopStyle(desktopStyleMacOS)
	}
	for a.desktopStyleButtons[1].Clicked(gtx) {
		a.persistDesktopStyle(desktopStyleWindows)
	}
	for a.generalStartupModeButtons[0].Clicked(gtx) {
		a.persistExperienceMode(experienceModeSimple)
	}
	for a.generalStartupModeButtons[1].Clicked(gtx) {
		a.persistExperienceMode(experienceModeWorkflow)
		a.applyDefaultWindowLayout()
	}
	layouts := []desktopstate.WindowLayout{desktopstate.WindowLayoutSingle, desktopstate.WindowLayoutDual, desktopstate.WindowLayoutMulti}
	for index, value := range layouts {
		for a.generalWindowLayoutButtons[index].Clicked(gtx) {
			a.desktopState.Preferences.DefaultWindowLayout = value
			a.persistDesktopPreferences("默认窗口布局")
			a.applyDefaultWindowLayout()
		}
	}
	for a.generalAutoProgressToggle.Clicked(gtx) {
		a.desktopState.Preferences.AutoShowProgress = !a.desktopState.Preferences.AutoShowProgress
		a.persistDesktopPreferences("自动进度窗口")
	}
	for a.generalReopenWindowsToggle.Clicked(gtx) {
		a.desktopState.Preferences.ReopenDetachedWindows = !a.desktopState.Preferences.ReopenDetachedWindows
		a.persistDesktopPreferences("重开独立窗口")
	}
	for a.generalRestoreSessionToggle.Clicked(gtx) {
		a.desktopState.Preferences.RestoreSession = !a.desktopState.Preferences.RestoreSession
		a.persistDesktopPreferences("恢复工作区")
	}
}

func (a *App) persistDesktopPreferences(label string) {
	if err := a.saveGioDesktopState(); err != nil {
		a.appendLog("保存" + label + "失败: " + err.Error())
	}
	a.invalidateNow()
}

func (a *App) applyDefaultWindowLayout() {
	if a.desktopWindows == nil || a.experienceMode != experienceModeWorkflow {
		return
	}
	switch a.desktopState.Preferences.DefaultWindowLayout {
	case desktopstate.WindowLayoutDual:
		a.openDesktopWindow(windowing.RoleCanvas, a.activeWorkspaceID)
		a.openDesktopWindow(windowing.RoleConsole, a.activeWorkspaceID)
	case desktopstate.WindowLayoutMulti:
		a.openDesktopWindow(windowing.RoleCanvas, a.activeWorkspaceID)
		a.openDesktopWindow(windowing.RoleConsole, a.activeWorkspaceID)
		a.openDesktopWindow(windowing.RoleProgress, a.activeWorkspaceID)
	}
}

func (a *App) layoutDesktopExperienceSettingsCard(gtx layout.Context) layout.Dimensions {
	styleRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			selected := a.desktopStyle == desktopStyleMacOS
			return a.compactIconTextButton(gtx, &a.desktopStyleButtons[0], uiIconMacOS, "macOS", selected, selected)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			selected := a.desktopStyle == desktopStyleWindows
			return a.compactIconTextButton(gtx, &a.desktopStyleButtons[1], uiIconWindows, "Windows", selected, selected)
		}),
	}
	modeRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			selected := a.experienceMode == experienceModeSimple
			return a.compactIconTextButton(gtx, &a.generalStartupModeButtons[0], uiIconPhoto, "简易模式", selected, selected)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			selected := a.experienceMode == experienceModeWorkflow
			return a.compactIconTextButton(gtx, &a.generalStartupModeButtons[1], uiIconWorkflow, "工作流模式", selected, selected)
		}),
	}
	layoutValues := []desktopstate.WindowLayout{desktopstate.WindowLayoutSingle, desktopstate.WindowLayoutDual, desktopstate.WindowLayoutMulti}
	layoutLabels := []string{"单窗口", "画布 + 控制台", "多窗口"}
	layoutRows := make([]layout.FlexChild, len(layoutValues))
	for index, value := range layoutValues {
		index := index
		value := value
		layoutRows[index] = layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			selected := a.desktopState.Preferences.DefaultWindowLayout == value
			return a.compactButton(gtx, &a.generalWindowLayoutButtons[index], layoutLabels[index], selected, selected)
		})
	}
	return a.generalSettingsCard(gtx, "桌面体验", func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.desktopSettingsLabel(gtx, "界面风格")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, styleRows...)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.desktopSettingsLabel(gtx, "启动体验")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, modeRows...)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.desktopSettingsLabel(gtx, "默认窗口布局")
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, layoutRows...)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.desktopSettingsToggleRow(gtx, "任务进度窗", &a.generalAutoProgressToggle, a.desktopState.Preferences.AutoShowProgress)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.desktopSettingsToggleRow(gtx, "重开独立窗口", &a.generalReopenWindowsToggle, a.desktopState.Preferences.ReopenDetachedWindows)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.desktopSettingsToggleRow(gtx, "恢复工作区", &a.generalRestoreSessionToggle, a.desktopState.Preferences.RestoreSession)
			}),
		)
	})
}

func (a *App) desktopSettingsLabel(gtx layout.Context, label string) layout.Dimensions {
	return a.label(gtx, label, unit.Sp(9), fluent.textMuted, font.SemiBold)
}

func (a *App) desktopSettingsToggleRow(gtx layout.Context, label string, toggle *widget.Clickable, value bool) layout.Dimensions {
	spec := desktopThemeSpec(a.desktopStyle, a.resolvedThemeMode)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, label, unit.Sp(10), fluent.text, font.Normal)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			track := spec.Colors.surface2
			hover := spec.Colors.border2
			if value {
				track = spec.Colors.accent
				hover = spec.Colors.accent2
			}
			return fixedWidth(gtx, unit.Dp(40), func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, unit.Dp(22), func(gtx layout.Context) layout.Dimensions {
					return a.surfaceButton(gtx, toggle, track, hover, spec.Colors.border, unit.Dp(11), layout.Inset{Top: 2, Bottom: 2, Left: 2, Right: 2}, func(gtx layout.Context) layout.Dimensions {
						semantic.Switch.Add(gtx.Ops)
						semantic.LabelOp(label).Add(gtx.Ops)
						semantic.SelectedOp(value).Add(gtx.Ops)
						gtx.Constraints.Min = gtx.Constraints.Max
						align := layout.W
						if value {
							align = layout.E
						}
						return align.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(18), func(gtx layout.Context) layout.Dimensions {
									return a.surface(gtx, desktopReadableText(track), unit.Dp(9), layout.Spacer{}.Layout)
								})
							})
						})
					})
				})
			})
		}),
	)
}
