package ui

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"time"

	"gioui.org/font"
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

	paint.FillShape(gtx.Ops, fluent.bg, clip.Rect{Max: gtx.Constraints.Max}.Op())
	if !a.reducedEffects && gtx.Constraints.Max.X > 0 && gtx.Constraints.Max.Y > 0 {
		bodyStart := withAlpha(fluent.white, 0x08)
		bodyEnd := withAlpha(fluent.bg2, 0x18)
		topGlow := withAlpha(fluent.white, 0x70)
		if resolveThemeMode(a.themeMode) == "dark" {
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
	if !snap.Fullscreen {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, unit.Dp(48), a.layoutHeader)
			}),
		)
		if len(a.workspaces) > 1 {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedHeight(gtx, unit.Dp(38), a.layoutWorkspaceBar)
			}))
		}
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return a.layoutBody(gtx, snap)
	}))
	if !snap.Fullscreen {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, unit.Dp(42), func(gtx layout.Context) layout.Dimensions {
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
	if a.presetManagerOpen {
		a.layoutPresetManagerModal(gtx, snap)
	}
	if a.customAspectRatioManagerOpen {
		a.layoutCustomAspectRatioManagerModal(gtx)
	}
	if snap.ActiveResultDetail.ID != "" || snap.ActiveResultDetail.SavedPath != "" {
		a.layoutResultDetailModal(gtx, snap)
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

func (a *App) layoutHeader(gtx layout.Context) layout.Dimensions {
	for idx, mode := range []string{"system", "light", "dark"} {
		for a.themeButtons[idx].Clicked(gtx) {
			a.persistThemeMode(mode)
		}
	}
	for a.headerAddWorkspaceButton.Clicked(gtx) {
		a.createWorkspace()
	}
	for a.headerQuoteButton.Clicked(gtx) {
		a.headerQuoteIndex = nextHeaderQuoteIndex(a.headerQuoteIndex)
	}
	for a.githubButton.Clicked(gtx) {
		if err := openExternalURL(repoURL); err != nil {
			a.appendLog("打开 GitHub 失败: " + err.Error())
		}
	}
	for a.headerStarButton.Clicked(gtx) {
		if err := openExternalURL(repoURL); err != nil {
			a.appendLog("打开 GitHub 失败: " + err.Error())
		}
	}
	for a.settingsButton.Clicked(gtx) {
		a.openGeneralSettingsModal()
	}

	return a.borderedSurface(gtx, fluent.toolbar, unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.layoutHeaderBrand(gtx)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Stack{}.Layout(gtx,
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							return a.headerIconButtonIcon(gtx, &a.headerAddWorkspaceButton, uiIconAdd, false)
						}),
						layout.Stacked(func(gtx layout.Context) layout.Dimensions {
							if len(a.workspaces) <= 1 {
								return layout.Dimensions{}
							}
							return layout.NE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(-2), Right: unit.Dp(-2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.badge(gtx, fmt.Sprintf("%d", len(a.workspaces)), fluent.accent, fluent.white)
								})
							})
						}),
					)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface, fluentControlRadius, accentAlpha(0x12), func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: 2, Bottom: 2, Left: 2, Right: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(2))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.headerIconButtonIcon(gtx, &a.themeButtons[0], uiIconSystem, a.themeMode == "system")
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.headerIconButtonIcon(gtx, &a.themeButtons[1], uiIconLight, a.themeMode == "light")
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.headerIconButtonIcon(gtx, &a.themeButtons[2], uiIconDark, a.themeMode == "dark")
								}),
							)
						})
					})
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.headerIconButtonIcon(gtx, &a.githubButton, uiIconLaunch, false)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.headerIconButtonIcon(gtx, &a.headerStarButton, uiIconStar, false)
				}),
				layout.Rigid(layout.Spacer{Width: unit.Dp(4)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.headerIconButtonIcon(gtx, &a.settingsButton, uiIconSettings, a.generalSettingsOpen)
				}),
			)
		})
	})
}

func (a *App) layoutHeaderBrand(gtx layout.Context) layout.Dimensions {
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
	width := gtx.Constraints.Max.X
	centerMin := gtx.Dp(unit.Dp(360))
	leftWidth := gtx.Dp(unit.Dp(372))
	rightWidth := gtx.Dp(unit.Dp(320))
	if width <= gtx.Dp(unit.Dp(1180)) {
		leftWidth = gtx.Dp(unit.Dp(336))
		rightWidth = gtx.Dp(unit.Dp(300))
	}
	if overflow := leftWidth + rightWidth + centerMin - width; overflow > 0 {
		rightMin := gtx.Dp(unit.Dp(280))
		leftMin := gtx.Dp(unit.Dp(320))
		reduceRight := min(overflow, max(rightWidth-rightMin, 0))
		rightWidth -= reduceRight
		overflow -= reduceRight
		if overflow > 0 {
			reduceLeft := min(overflow, max(leftWidth-leftMin, 0))
			leftWidth -= reduceLeft
		}
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedPixelWidth(gtx, leftWidth, func(gtx layout.Context) layout.Dimensions {
				return a.layoutControls(gtx, snap)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.layoutCanvas(gtx, snap)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedPixelWidth(gtx, rightWidth, func(gtx layout.Context) layout.Dimensions {
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

	return a.borderedSurface(gtx, withAlpha(fluent.toolbar, 0xf2), unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Inset{Top: 6, Bottom: 7, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, len(a.workspaces)+1)
			for _, ws := range a.workspaces {
				ws := ws
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Right: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.layoutWorkspaceTab(gtx, ws, ws.ID == a.activeWorkspaceID)
					})
				}))
			}
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return fixedWidth(gtx, unit.Dp(32), func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, unit.Dp(30), func(gtx layout.Context) layout.Dimensions {
						return a.surfaceButton(
							gtx,
							&a.addWorkspaceButton,
							rgba(0xffffff, 0x00),
							fluent.panel,
							rgba(0xffffff, 0x00),
							unit.Dp(4),
							layout.Inset{},
							func(gtx layout.Context) layout.Dimensions {
								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
										return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
											return uiIconAdd.Layout(gtx, fluent.textMuted)
										})
									})
								})
							},
						)
					})
				})
			}))
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}.Layout(gtx, children...)
		})
	})
}

func (a *App) layoutWorkspaceTab(gtx layout.Context, ws workspaceState, active bool) layout.Dimensions {
	btn := a.workspaceButton("workspace:" + ws.ID)
	closeBtn := a.closeWorkspaceButton("workspace-close:" + ws.ID)
	editing := a.workspaceRenameID == ws.ID
	running := a.isRunning() && ws.ID == a.activeWorkspaceID
	bg := chooseColor(active, fluent.surface, rgba(0xffffff, 0x00))
	hoverBg := chooseColor(active, fluent.surface, withAlpha(fluent.surface, 0xc6))
	border := chooseColor(active, fluent.border, rgba(0xffffff, 0x00))
	return btn.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fill := bg
		if btn.Hovered() {
			fill = hoverBg
		}
		body := func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 7, Bottom: 6, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
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
