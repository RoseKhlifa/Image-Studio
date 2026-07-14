package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"github.com/yuanhua/image-gptcodex/pkg/client"
)

const repoURL = "https://github.com/RoseKhlifa/Image-Studio"
const issuesURL = "https://github.com/RoseKhlifa/Image-Studio/issues"
const licenseURL = "https://www.gnu.org/licenses/agpl-3.0.html"

type simplePaneContract struct {
	preferredLeft  unit.Dp
	preferredRight unit.Dp
	minimumLeft    unit.Dp
	minimumRight   unit.Dp
	minimumCenter  unit.Dp
}

type simplePaneWidths struct {
	left   int
	right  int
	center int
}

type appleHeaderBrandContract struct {
	titleSize unit.Sp
}

func headerInsetsForStyle(style string) layout.Inset {
	if normalizeDesktopStyle(style) == desktopStyleMacOS {
		return layout.Inset{Top: 6, Bottom: 6, Left: 12, Right: 12}
	}
	return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}
}

func appleHeaderBrandMetrics() appleHeaderBrandContract {
	return appleHeaderBrandContract{
		titleSize: unit.Sp(13),
	}
}

func shellShowsGlobalFooter(style string) bool {
	return normalizeDesktopStyle(style) != desktopStyleMacOS
}

func shellShowsCommunityActions(style string) bool {
	return normalizeDesktopStyle(style) != desktopStyleMacOS
}

func simplePaneContractForStyle(style string, metrics desktopThemeMetrics, compact bool) simplePaneContract {
	if normalizeDesktopStyle(style) == desktopStyleMacOS {
		return simplePaneContract{
			preferredLeft:  metrics.LeftPaneWidth,
			preferredRight: metrics.RightPaneWidth,
			minimumLeft:    unit.Dp(280),
			minimumRight:   unit.Dp(280),
			minimumCenter:  unit.Dp(440),
		}
	}
	contract := simplePaneContract{
		preferredLeft:  unit.Dp(372),
		preferredRight: unit.Dp(320),
		minimumLeft:    unit.Dp(320),
		minimumRight:   unit.Dp(280),
		minimumCenter:  unit.Dp(360),
	}
	if compact {
		contract.preferredLeft = unit.Dp(336)
		contract.preferredRight = unit.Dp(300)
	}
	return contract
}

func fitSimplePaneWidths(available int, preferredLeft int, preferredRight int, minimumLeft int, minimumRight int, minimumCenter int) simplePaneWidths {
	left := preferredLeft
	right := preferredRight
	overflow := left + right + minimumCenter - available
	if overflow > 0 {
		reduceRight := min(overflow, max(right-minimumRight, 0))
		right -= reduceRight
		overflow -= reduceRight
		if overflow > 0 {
			reduceLeft := min(overflow, max(left-minimumLeft, 0))
			left -= reduceLeft
		}
	}
	return simplePaneWidths{
		left:   left,
		right:  right,
		center: max(available-left-right, 0),
	}
}

func maximumWidth(gtx layout.Context, width unit.Dp, w layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = 0
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(width))
	return w(gtx)
}

func (a *App) layout(gtx layout.Context) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingShell, time.Now())
	snap := a.readSnapshot()
	for a.runButton.Clicked(gtx) {
		a.startRun()
	}
	for a.cancelButton.Clicked(gtx) {
		a.cancelRun()
	}
	for a.clearLogButton.Clicked(gtx) {
		a.clearLogs()
	}
	a.trackGlobalPointer(gtx)
	a.handleCanvasKeyboardShortcuts(gtx, snap)

	paint.FillShape(gtx.Ops, fluent.bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
	if a.shellEffectsEnabled() && gtx.Constraints.Max.X > 0 && gtx.Constraints.Max.Y > 0 {
		bodyStart := withAlpha(fluent.white, 0x08)
		bodyEnd := withAlpha(fluent.bg2, 0x18)
		topGlow := withAlpha(fluent.white, 0x70)
		if a.isDarkTheme() {
			bodyStart = rgba(0xffffff, 0x00)
			bodyEnd = withAlpha(fluent.bg2, 0x22)
			topGlow = withAlpha(fluent.white, 0x09)
		}
		paintLinearGradient(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, gtx.Constraints.Max.Y), 0, bodyStart, bodyEnd)

		glowHeight := min(gtx.Dp(unit.Dp(190)), gtx.Constraints.Max.Y)
		if glowHeight > 0 {
			paintLinearGradient(gtx, image.Rect(0, 0, gtx.Constraints.Max.X, glowHeight), 0, topGlow, rgba(0xffffff, 0x00))
		}
	}
	children := []layout.FlexChild{}
	spec := desktopThemeSpec(a.desktopStyle, a.resolvedThemeMode)
	metrics := spec.Metrics
	if !snap.Fullscreen {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				height := minimumTextControlHeight(gtx, metrics.HeaderHeight, a.scaledSp(unit.Sp(14)), unit.Dp(16))
				return fixedHeight(gtx, height, a.layoutHeader)
			}),
		)
		if len(a.workspaces) > 1 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				height := minimumTextControlHeight(gtx, metrics.WorkspaceBarHeight, a.scaledSp(unit.Sp(12)), unit.Dp(15))
				return fixedHeight(gtx, height, a.layoutWorkspaceBar)
			}))
		}
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return a.layoutBody(gtx, snap)
	}))
	if !snap.Fullscreen && shellShowsGlobalFooter(spec.Style) {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			height := minimumTextControlHeight(gtx, metrics.StatusBarHeight, a.scaledSp(unit.Sp(10)), unit.Dp(18))
			return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
				return a.layoutFooter(gtx, snap)
			})
		}))
	}
	dims := layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	if snap.SavePromptVisible {
		a.layoutSavePrompt(gtx, snap)
	}
	if snap.PromptImportRegisterOpen {
		a.layoutPromptImportRegistrationPrompt(gtx, snap)
	}
	if snap.PromptImportVisible {
		a.layoutPromptImportModal(gtx, snap)
	}
	if a.generalSettingsOpen {
		a.layoutGeneralSettingsModal(gtx, snap)
	}
	if a.aboutModalOpen {
		a.layoutAboutModal(gtx)
	}
	if _, open, _, _ := a.readAppUpdateState(); open {
		a.layoutAppUpdateModal(gtx)
	}
	if a.settingsModalOpen {
		a.layoutSettingsModal(gtx, snap)
		if a.upstreamQuickImportOpen {
			a.layoutUpstreamQuickImportModal(gtx)
		}
		if a.settingsHelpOpen {
			a.layoutSettingsHelpModal(gtx)
		}
	}
	if a.promptTemplateManagerOpen {
		a.layoutPromptTemplateManagerModal(gtx, snap)
	}
	if a.promptHelperOpen {
		a.layoutPromptHelperPopover(gtx)
	}
	if a.presetPickerOpen {
		a.layoutPresetPickerPopover(gtx)
	}
	if a.presetManagerOpen {
		a.layoutPresetManagerModal(gtx, snap)
	}
	if a.customAspectRatioManagerOpen {
		a.layoutCustomAspectRatioManagerModal(gtx)
	}
	if a.customSizeModalOpen {
		a.layoutCustomSizeModal(gtx)
	}
	if a.loopModalOpen {
		a.layoutLoopModal(gtx)
	}
	if a.advancedOpen {
		a.layoutAdvancedModal(gtx)
	}
	if a.canvasAnnotationTextPromptOpen {
		a.layoutCanvasAnnotationTextPrompt(gtx)
	}
	if snap.ActiveResultDetail.ID != "" || snap.ActiveResultDetail.SavedPath != "" {
		a.layoutResultDetailModal(gtx, snap)
	}
	if snap.HistoryActionMenuItem.ID != "" || snap.HistoryActionMenuItem.SavedPath != "" {
		a.layoutHistoryActionMenuModal(gtx, snap)
	}
	if strings.TrimSpace(snap.RawResponseModalPath) != "" || strings.TrimSpace(snap.RawResponseModalError) != "" || strings.TrimSpace(snap.RawResponseModalText) != "" {
		a.layoutRawResponseModal(gtx, snap)
	}
	if snap.ActivePromptGroup.Key != "" {
		a.layoutPromptGroupModal(gtx, snap)
	}
	if snap.HistoryTimelineOpen {
		a.layoutHistoryTimelineModal(gtx, snap)
	}
	return dims
}

func (a *App) shellEffectsEnabled() bool {
	return a != nil && !a.reducedEffects && normalizeDesktopStyle(a.desktopStyle) != desktopStyleMacOS && normalizeExperienceMode(a.experienceMode) == experienceModeSimple
}

func (a *App) layoutHeader(gtx layout.Context) layout.Dimensions {
	showCommunityActions := shellShowsCommunityActions(a.desktopStyle)
	for idx, mode := range []string{"system", "light", "dark"} {
		for a.themeButtons[idx].Clicked(gtx) {
			a.persistThemeMode(mode)
		}
	}
	for a.headerAddWorkspaceButton.Clicked(gtx) {
		a.createWorkspace()
		a.scrollWorkspaceListToEnd()
	}
	for a.headerQuoteButton.Clicked(gtx) {
		if showCommunityActions {
			a.headerQuoteIndex = nextHeaderQuoteIndex(a.headerQuoteIndex)
		}
	}
	for a.githubButton.Clicked(gtx) {
		if showCommunityActions {
			if err := openExternalURL(repoURL); err != nil {
				a.appendLog("打开 GitHub 失败: " + err.Error())
			}
		}
	}
	for a.headerStarButton.Clicked(gtx) {
		if showCommunityActions {
			if err := openExternalURL(repoURL); err != nil {
				a.appendLog("打开 GitHub 失败: " + err.Error())
			}
		}
	}
	for a.settingsButton.Clicked(gtx) {
		a.openGeneralSettingsModal()
	}
	for a.macToggleSidebarButton.Clicked(gtx) {
		a.macSidebarHidden = !a.macSidebarHidden
		a.invalidateNow()
	}
	for a.macToggleInspectorButton.Clicked(gtx) {
		a.macInspectorHidden = !a.macInspectorHidden
		a.invalidateNow()
	}

	if normalizeDesktopStyle(a.desktopStyle) == desktopStyleMacOS {
		return a.layoutAppleHeader(gtx)
	}
	return a.layoutWindowsHeader(gtx)
}

func (a *App) layoutWindowsHeader(gtx layout.Context) layout.Dimensions {
	return a.borderedSurface(gtx, fluent.toolbar, unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return headerInsetsForStyle(desktopStyleWindows).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.layoutHeaderBrand(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutExperienceSwitch(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutHeaderAddWorkspaceButton(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutHeaderThemeSelector(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.headerIconButtonIcon(gtx, &a.githubButton, uiIconLaunch, false, "打开 GitHub 仓库")
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.headerIconButtonIcon(gtx, &a.headerStarButton, uiIconStar, false, "在 GitHub 收藏项目")
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.headerIconButtonIcon(gtx, &a.settingsButton, uiIconSettings, a.generalSettingsOpen, "打开设置")
				}),
			)
		})
	})
}

func (a *App) layoutAppleHeader(gtx layout.Context) layout.Dimensions {
	return a.borderedSurface(gtx, fluent.toolbar, unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return headerInsetsForStyle(desktopStyleMacOS).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := "隐藏生成设置"
							if a.macSidebarHidden {
								label = "显示生成设置"
							}
							return a.headerIconButtonIcon(gtx, &a.macToggleSidebarButton, uiIconList, !a.macSidebarHidden, label)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
						layout.Flexed(1, a.layoutHeaderBrand),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutExperienceSwitch(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutHeaderAddWorkspaceButton(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := "隐藏历史记录"
					if a.macInspectorHidden {
						label = "显示历史记录"
					}
					return a.headerIconButtonIcon(gtx, &a.macToggleInspectorButton, uiIconHistory, !a.macInspectorHidden, label)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.headerIconButtonIcon(gtx, &a.settingsButton, uiIconSettings, a.generalSettingsOpen, "打开设置")
				}),
			)
		})
	})
}

func (a *App) layoutHeaderAddWorkspaceButton(gtx layout.Context) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return a.headerIconButtonIcon(gtx, &a.headerAddWorkspaceButton, uiIconAdd, false, "新建工作区")
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			if len(a.workspaces) <= 1 {
				return layout.Dimensions{}
			}
			return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: unit.Dp(-2), Right: unit.Dp(-2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.badge(gtx, fmt.Sprintf("%d", len(a.workspaces)), fluent.accent, desktopReadableText(fluent.accent))
				})
			})
		}),
	)
}

func (a *App) layoutHeaderThemeSelector(gtx layout.Context) layout.Dimensions {
	return a.borderedSurface(gtx, fluent.surface, fluentControlRadius, accentAlpha(0x12), func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: 2, Bottom: 2, Left: 2, Right: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(2))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.headerIconButtonIcon(gtx, &a.themeButtons[0], uiIconSystem, a.themeMode == "system", "使用系统主题")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.headerIconButtonIcon(gtx, &a.themeButtons[1], uiIconLight, a.themeMode == "light", "切换为浅色主题")
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.headerIconButtonIcon(gtx, &a.themeButtons[2], uiIconDark, a.themeMode == "dark", "切换为深色主题")
				}),
			)
		})
	})
}

func (a *App) layoutHeaderBrand(gtx layout.Context) layout.Dimensions {
	if normalizeDesktopStyle(a.desktopStyle) == desktopStyleMacOS {
		return a.layoutAppleHeaderBrand(gtx)
	}
	quote := currentHeaderQuote(a.headerQuoteIndex)
	quoteText := strings.TrimSpace(quote.Text)
	if quoteText == "" {
		quoteText = "山有顶峰，湖有彼岸；在人生漫漫长途中，万物皆有回转。"
	}
	quoteFrom := strings.TrimSpace(quote.From)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.titleLabel(gtx, "Image Studio", unit.Sp(14))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.surfaceButton(
						gtx,
						&a.headerQuoteButton,
						rgba(0xffffff, 0x00),
						rgba(0xffffff, 0x00),
						rgba(0xffffff, 0x00),
						fluentControlRadius,
						layout.Inset{Top: 0, Bottom: 0, Left: 0, Right: 0},
						func(gtx layout.Context) layout.Dimensions {
							text := quoteText
							if quoteFrom != "" {
								text += " — " + quoteFrom
							}
							return a.singleLineLabel(gtx, text, unit.Sp(9), fluent.textMuted, font.Normal)
						},
					)
				}),
			)
		}),
	)
}

func (a *App) layoutAppleHeaderBrand(gtx layout.Context) layout.Dimensions {
	spec := desktopThemeSpec(a.desktopStyle, a.resolvedThemeMode)
	contract := appleHeaderBrandMetrics()
	title := strings.TrimSpace(a.currentWorkspaceDisplayName())
	if title == "" {
		title = "图像生成"
	}
	return a.singleLineLabel(gtx, title, contract.titleSize, spec.Colors.text, font.SemiBold)
}

func (a *App) layoutFooter(gtx layout.Context, snap snapshot) layout.Dimensions {
	for a.footerOutputButton.Clicked(gtx) {
		if err := openPath(a.outputDirInput.Text()); err != nil {
			a.appendLog("打开输出目录失败: " + err.Error())
		}
	}
	for a.footerGithubButton.Clicked(gtx) {
		if err := openExternalURL(repoURL); err != nil {
			a.appendLog("打开 GitHub 失败: " + err.Error())
		}
	}
	for a.footerFeedbackButton.Clicked(gtx) {
		if err := openExternalURL(issuesURL); err != nil {
			a.appendLog("打开反馈页失败: " + err.Error())
		}
	}

	state := "就绪"
	dot := fluent.textDim
	if snap.Running {
		state = "运行中"
		dot = fluent.accent
	}
	todayCount := snap.TodayHistoryCount
	totalCount := len(snap.History)
	activeRunningCount := max(snap.BatchTotal, 1)
	return a.borderedSurface(gtx, withAlpha(fluent.toolbar, 0xf2), unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Inset{Top: 9, Bottom: 9, Left: 18, Right: 18}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.footerIconTextButton(gtx, &a.footerOutputButton, uiIconFolder, "输出目录")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.footerIconTextButton(gtx, &a.footerGithubButton, uiIconLaunch, "GitHub")
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.footerIconTextButton(gtx, &a.footerFeedbackButton, uiIconFeedback, "反馈")
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					children := []layout.FlexChild{
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.footerMetric(gtx, "今日已生图:", fmt.Sprintf("%d", todayCount), fluent.text)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, "·", unit.Sp(11), withAlpha(fluent.textDim, 0x88), font.Normal)
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.footerMetric(gtx, "总生图:", fmt.Sprintf("%d", totalCount), fluent.text)
						}),
					}
					if snap.Running {
						children = append(children,
							layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "·", unit.Sp(11), withAlpha(fluent.textDim, 0x88), font.Normal)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(12)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.footerMetric(gtx, "当前标签:", fmt.Sprintf("%d", activeRunningCount), fluent.accent)
							}),
						)
					}
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, state, unit.Sp(11), fluent.textMuted, font.Medium)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(7), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(7), func(gtx layout.Context) layout.Dimensions {
									return a.surface(gtx, dot, unit.Dp(4), layout.Spacer{}.Layout)
								})
							})
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "v"+client.Version, unit.Sp(11), fluent.textDim, font.Normal)
				}),
			)
		})
	})
}

func (a *App) layoutBody(gtx layout.Context, snap snapshot) layout.Dimensions {
	if snap.Fullscreen {
		return a.layoutCanvas(gtx, snap)
	}
	if a.experienceMode == experienceModeWorkflow {
		return a.layoutWorkflowShell(gtx, snap)
	}
	if normalizeDesktopStyle(a.desktopStyle) == desktopStyleMacOS {
		return a.layoutMacSimpleBody(gtx, snap)
	}
	width := gtx.Constraints.Max.X
	spec := desktopThemeSpec(a.desktopStyle, a.resolvedThemeMode)
	contract := simplePaneContractForStyle(spec.Style, spec.Metrics, width <= gtx.Dp(unit.Dp(1180)))
	widths := fitSimplePaneWidths(
		width,
		gtx.Dp(contract.preferredLeft),
		gtx.Dp(contract.preferredRight),
		gtx.Dp(contract.minimumLeft),
		gtx.Dp(contract.minimumRight),
		gtx.Dp(contract.minimumCenter),
	)
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedPixelWidth(gtx, widths.left, func(gtx layout.Context) layout.Dimensions {
				return a.layoutControls(gtx, snap)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.layoutCanvas(gtx, snap)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedPixelWidth(gtx, widths.right, func(gtx layout.Context) layout.Dimensions {
				return a.layoutHistoryAndLogs(gtx, snap)
			})
		}),
	)
}

func (a *App) layoutWorkspaceBar(gtx layout.Context) layout.Dimensions {
	for {
		event, ok := a.workspaceNameInput.Update(gtx)
		if !ok {
			break
		}
		switch event.(type) {
		case widget.SubmitEvent:
			a.commitWorkspaceRename()
		}
	}
	for a.addWorkspaceButton.Clicked(gtx) {
		a.createWorkspace()
		a.scrollWorkspaceListToEnd()
	}
	for _, ws := range a.workspaces {
		ws := ws
		btn := a.workspaceButton("workspace:" + ws.ID)
		for btn.Clicked(gtx) {
			a.handleWorkspacePrimaryClick(ws.ID, gtx.Now)
		}
		closeBtn := a.closeWorkspaceButton("workspace-close:" + ws.ID)
		for closeBtn.Clicked(gtx) {
			a.closeWorkspace(ws.ID)
		}
	}

	spec := desktopThemeSpec(a.desktopStyle, a.resolvedThemeMode)
	barFill := withAlpha(fluent.toolbar, 0xf2)
	inset := layout.Inset{Top: 6, Bottom: 7, Left: 10, Right: 10}
	if spec.Style == desktopStyleMacOS {
		barFill = spec.Colors.toolbar
		inset = layout.Inset{Top: 3, Bottom: 3, Left: 20, Right: 20}
	}
	return a.borderedSurface(gtx, barFill, unit.Dp(0), spec.Colors.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.workspaceList.Layout(gtx, len(a.workspaces), func(gtx layout.Context, index int) layout.Dimensions {
						ws := a.workspaces[index]
						return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.layoutWorkspaceTab(gtx, ws, ws.ID == a.activeWorkspaceID)
						})
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutAddWorkspaceButton(gtx, spec)
				}),
			)
		})
	})
}

func (a *App) scrollWorkspaceListToEnd() {
	a.workspaceList.ScrollToEnd = true
	a.workspaceList.Position.BeforeEnd = false
}

func (a *App) layoutAddWorkspaceButton(gtx layout.Context, spec desktopThemeTokens) layout.Dimensions {
	height := unit.Dp(30)
	radius := unit.Dp(4)
	hover := spec.Colors.panel
	if spec.Style == desktopStyleMacOS {
		height = unit.Dp(32)
		radius = unit.Dp(16)
		hover = spec.Colors.surface2
	}
	return fixedWidth(gtx, unit.Dp(32), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
			return a.surfaceButton(
				gtx,
				&a.addWorkspaceButton,
				rgba(0xffffff, 0x00),
				hover,
				rgba(0xffffff, 0x00),
				radius,
				layout.Inset{},
				func(gtx layout.Context) layout.Dimensions {
					semantic.LabelOp("新建工作区").Add(gtx.Ops)
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
								return uiIconAdd.Layout(gtx, spec.Colors.textMuted)
							})
						})
					})
				},
			)
		})
	})
}

func (a *App) layoutWorkspaceTab(gtx layout.Context, ws workspaceState, active bool) layout.Dimensions {
	if normalizeDesktopStyle(a.desktopStyle) == desktopStyleMacOS {
		return a.layoutAppleWorkspaceTab(gtx, ws, active)
	}
	return a.layoutWindowsWorkspaceTab(gtx, ws, active)
}

func (a *App) layoutWindowsWorkspaceTab(gtx layout.Context, ws workspaceState, active bool) layout.Dimensions {
	btn := a.workspaceButton("workspace:" + ws.ID)
	closeBtn := a.closeWorkspaceButton("workspace-close:" + ws.ID)
	editing := a.workspaceRenameID == ws.ID
	running := a.isRunning() && ws.ID == a.activeWorkspaceID
	bg := chooseColor(active, fluent.surface, rgba(0xffffff, 0x00))
	hoverBg := chooseColor(active, fluent.surface, withAlpha(fluent.surface, 0xc6))
	border := chooseColor(active, fluent.border, rgba(0xffffff, 0x00))
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.Button.Add(gtx.Ops)
		semantic.LabelOp("工作区 " + a.displayedWorkspaceName(ws)).Add(gtx.Ops)
		semantic.SelectedOp(active).Add(gtx.Ops)
		fill := bg
		if btn.Hovered() {
			fill = hoverBg
		}
		body := func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 7, Bottom: 6, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if editing {
							return fixedWidth(gtx, unit.Dp(96), func(gtx layout.Context) layout.Dimensions {
								border := fluent.border2
								if gtx.Focused(&a.workspaceNameInput) {
									border = accentAlpha(0xb8)
								}
								return a.borderedSurface(gtx, fluent.surface, fluentControlRadius, border, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 7, Bottom: 7, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return a.editorText(gtx, &a.workspaceNameInput, "未命名", unit.Sp(11))
									})
								})
							})
						}
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(120), func(gtx layout.Context) layout.Dimensions {
									weight := font.Medium
									if active {
										weight = font.SemiBold
									}
									return a.singleLineLabel(gtx, a.displayedWorkspaceName(ws), unit.Sp(12), chooseColor(active, fluent.text, fluent.textMuted), weight)
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								modeLabel := ""
								if ws.BatchMode {
									modeLabel = "批"
								} else if ws.LoopEnabled {
									modeLabel = "循"
								}
								if modeLabel == "" {
									return layout.Dimensions{}
								}
								return a.metaBadge(gtx, modeLabel, true)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !running {
									return layout.Dimensions{}
								}
								return fixedWidth(gtx, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
										return a.surface(gtx, fluent.accent, unit.Dp(3), layout.Spacer{}.Layout)
									})
								})
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if len(a.workspaces) <= 1 || editing {
							return layout.Dimensions{}
						}
						if !btn.Hovered() {
							return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
									return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
								})
							})
						}
						return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.surfaceButton(
								gtx,
								closeBtn,
								rgba(0x000000, 0x00),
								chooseColor(active, dangerAlpha(0x10), fluent.surface2),
								rgba(0xffffff, 0x00),
								unit.Dp(3),
								layout.Inset{Top: 2, Bottom: 2, Left: 3, Right: 3},
								func(gtx layout.Context) layout.Dimensions {
									return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
										return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
											return uiIconClose.Layout(gtx, fluent.textDim)
										})
									})
								},
							)
						})
					}),
				)
			})
		}
		if active {
			return a.borderedTopTabSurface(gtx, fill, border, unit.Dp(6), body)
		}
		return a.borderedTopTabSurface(gtx, fill, border, unit.Dp(6), body)
	})
}

func appleWorkspaceTabHeight(_ bool) unit.Dp {
	return unit.Dp(28)
}

func (a *App) layoutAppleWorkspaceTab(gtx layout.Context, ws workspaceState, active bool) layout.Dimensions {
	spec := desktopThemeSpec(a.desktopStyle, a.resolvedThemeMode)
	btn := a.workspaceButton("workspace:" + ws.ID)
	closeBtn := a.closeWorkspaceButton("workspace-close:" + ws.ID)
	editing := a.workspaceRenameID == ws.ID
	running := a.isRunning() && ws.ID == a.activeWorkspaceID
	height := appleWorkspaceTabHeight(active)
	radius := height / 2
	fill := rgba(0xffffff, 0x00)
	border := rgba(0xffffff, 0x00)
	if active {
		fill = spec.Colors.surfaceElevated
		border = spec.Colors.border
	} else if btn.Hovered() {
		fill = spec.Colors.surface
	}

	return fixedHeight(gtx, height, func(gtx layout.Context) layout.Dimensions {
		return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			semantic.Button.Add(gtx.Ops)
			semantic.LabelOp("工作区 " + a.displayedWorkspaceName(ws)).Add(gtx.Ops)
			semantic.SelectedOp(active).Add(gtx.Ops)
			body := func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 4, Bottom: 4, Left: 14, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if editing {
								return fixedWidth(gtx, unit.Dp(96), func(gtx layout.Context) layout.Dimensions {
									inputBorder := spec.Colors.border2
									if gtx.Focused(&a.workspaceNameInput) {
										inputBorder = spec.Colors.focusRing
									}
									return a.borderedSurface(gtx, spec.Colors.surface, unit.Dp(10), inputBorder, func(gtx layout.Context) layout.Dimensions {
										return layout.Inset{Top: 2, Bottom: 2, Left: 7, Right: 7}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return a.editorText(gtx, &a.workspaceNameInput, "未命名", unit.Sp(11))
										})
									})
								})
							}
							return maximumWidth(gtx, unit.Dp(132), func(gtx layout.Context) layout.Dimensions {
								weight := font.Medium
								textColor := spec.Colors.textMuted
								if active {
									weight = font.SemiBold
									textColor = spec.Colors.text
								}
								return a.singleLineLabel(gtx, a.displayedWorkspaceName(ws), unit.Sp(12), textColor, weight)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							modeLabel := ""
							if ws.BatchMode {
								modeLabel = "批"
							} else if ws.LoopEnabled {
								modeLabel = "循"
							}
							return a.metaBadge(gtx, modeLabel, true)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if !running {
								return layout.Dimensions{}
							}
							return fixedWidth(gtx, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(6), func(gtx layout.Context) layout.Dimensions {
									return a.surface(gtx, spec.Colors.accent, unit.Dp(3), layout.Spacer{}.Layout)
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if len(a.workspaces) <= 1 || editing {
								return layout.Dimensions{}
							}
							if !btn.Hovered() {
								return fixedWidth(gtx, unit.Dp(20), layout.Spacer{}.Layout)
							}
							return fixedWidth(gtx, unit.Dp(20), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(20), func(gtx layout.Context) layout.Dimensions {
									return a.surfaceButton(
										gtx,
										closeBtn,
										rgba(0xffffff, 0x00),
										spec.Colors.surface2,
										rgba(0xffffff, 0x00),
										unit.Dp(10),
										layout.Inset{Top: 4, Bottom: 4, Left: 4, Right: 4},
										func(gtx layout.Context) layout.Dimensions {
											semantic.LabelOp("关闭工作区 " + a.displayedWorkspaceName(ws)).Add(gtx.Ops)
											return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
												return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
													return uiIconClose.Layout(gtx, spec.Colors.textDim)
												})
											})
										},
									)
								})
							})
						}),
					)
				})
			}
			if active {
				return a.elevatedBorderedSurface(gtx, fill, radius, border, image.Pt(0, 1), body)
			}
			return a.borderedSurface(gtx, fill, radius, border, body)
		})
	})
}

func (a *App) footerIconTextButton(gtx layout.Context, btn *widget.Clickable, icon *widget.Icon, text string) layout.Dimensions {
	fg := fluent.textMuted
	if btn.Hovered() {
		fg = fluent.toolHoverText
	}
	return a.surfaceButton(
		gtx,
		btn,
		rgba(0xffffff, 0x00),
		fluent.toolHoverBg,
		rgba(0xffffff, 0x00),
		unit.Dp(4),
		layout.Inset{Top: 3, Bottom: 3, Left: 6, Right: 6},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
							return icon.Layout(gtx, fg)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, text, unit.Sp(10), fg, font.Normal)
				}),
			)
		},
	)
}

func (a *App) footerMetric(gtx layout.Context, label string, value string, valueColor color.NRGBA) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, label, unit.Sp(10), withAlpha(fluent.textMuted, 0xb4), font.Normal)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(3)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.monoLabel(gtx, value, unit.Sp(10), valueColor, font.Medium)
		}),
	)
}
