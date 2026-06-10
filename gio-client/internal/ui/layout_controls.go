package ui

import (
	"fmt"
	"image"
	"image/color"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"image-studio/gio-client/internal/kernel"
	sharedCompat "image-studio/shared/compat"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/yuanhua/image-gptcodex/pkg/client"
)

type promptHelperItem struct {
	ID     string
	Title  string
	Detail string
	Kind   string
}

var builtInPromptTemplates = []promptHelperItem{
	{ID: "photoreal", Title: "写实摄影", Detail: "photorealistic, professional photography, 35mm, natural lighting, sharp focus, high detail"},
	{ID: "cinematic", Title: "电影感", Detail: "cinematic, dramatic lighting, shallow depth of field, film grain, anamorphic, 2.39:1"},
	{ID: "anime", Title: "二次元", Detail: "anime style, vibrant colors, cel shading, detailed illustration"},
	{ID: "oil", Title: "油画", Detail: "oil painting, thick brush strokes, classical art style, warm tones"},
	{ID: "watercolor", Title: "水彩", Detail: "watercolor painting, soft edges, pastel colors, paper texture"},
	{ID: "flat", Title: "扁平插画", Detail: "flat illustration, minimalist, geometric shapes, vector style"},
	{ID: "render3d", Title: "3D 渲染", Detail: "3D render, octane render, ray tracing, glossy, studio lighting"},
	{ID: "pixel", Title: "像素风", Detail: "pixel art, 16-bit, retro game style, limited palette"},
}

type settingsOptionChoice struct {
	Title  string
	Detail string
	Value  string
}

func (a *App) layoutControls(gtx layout.Context, snap snapshot) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingControls, time.Now())
	for a.composeToggleButton.Clicked(gtx) {
		a.composeOpen = !a.composeOpen
	}
	for a.advancedToggleButton.Clicked(gtx) {
		a.advancedOpen = !a.advancedOpen
	}
	for a.manageUpstreamButton.Clicked(gtx) {
		a.openSettingsModal()
	}

	return a.borderedSurface(gtx, fluent.sidebar, unit.Dp(0), fluent.border, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = gtx.Constraints.Max
		return layout.Inset{Top: 12, Bottom: 12, Left: 16, Right: 16}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			trimmedPrompt, promptLen := a.promptInputMetrics()
			ready := strings.TrimSpace(a.apiKeyInput.Text()) != "" && strings.TrimSpace(a.baseURLInput.Text()) != ""
			hasPrompt := trimmedPrompt != ""
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.controlsList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
						children := []layout.FlexChild{
							layout.Rigid(a.layoutWorkbenchCard),
						}
						if strings.TrimSpace(snap.LastErrorMessage) != "" {
							children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.layoutErrorNoticeCard(gtx, snap)
							}))
						}
						children = append(children,
							layout.Rigid(a.layoutModeCard),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.layoutPromptCard(gtx, snap, promptLen)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.layoutComposeCard(gtx, snap)
							}),
							layout.Rigid(a.layoutAdvancedCard),
						)
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, children...)
					})
				}),
				layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutSubmitDock(gtx, snap, ready, hasPrompt)
				}),
			)
		})
	})
}

func (a *App) promptInputMetrics() (string, int) {
	text := a.promptInput.Text()
	a.mu.Lock()
	if a.promptTextMetricsKey == text {
		trimmed := a.promptTextMetricsTrimmed
		length := a.promptTextMetricsLen
		a.mu.Unlock()
		return trimmed, length
	}
	a.mu.Unlock()

	trimmed := strings.TrimSpace(text)
	length := utf8.RuneCountInString(trimmed)

	a.mu.Lock()
	if a.promptTextMetricsKey != text {
		a.promptTextMetricsKey = text
		a.promptTextMetricsTrimmed = trimmed
		a.promptTextMetricsLen = length
	}
	a.mu.Unlock()
	return trimmed, length
}

func (a *App) layoutSubmitDock(gtx layout.Context, snap snapshot, ready bool, hasPrompt bool) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingSubmitDock, time.Now())
	return layout.Stack{}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			if size.X == 0 {
				size.X = gtx.Constraints.Max.X
			}
			if size.Y == 0 {
				size.Y = gtx.Dp(unit.Dp(120))
			}
			paintLinearGradient(gtx, image.Rect(0, 0, size.X, size.Y), 0, rgba(0xffffff, 0x00), withAlpha(fluent.sidebar, 0xf4))
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 8, Bottom: 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.layoutActions(gtx, snap, ready, hasPrompt)
			})
		}),
	)
}

func (a *App) layoutErrorNoticeCard(gtx layout.Context, snap snapshot) layout.Dimensions {
	for a.retryLastRunButton.Clicked(gtx) {
		a.retryLastRun()
	}
	for a.openRawResponseButton.Clicked(gtx) {
		raw := strings.TrimSpace(snap.Result.RawPath)
		if raw == "" {
			continue
		}
		a.openRawResponseModal(raw)
	}
	for a.dismissErrorButton.Clicked(gtx) {
		a.dismissFailureState()
	}

	return a.elevatedBorderedSurface(gtx, dangerAlpha(0x16), fluentCardRadius, dangerAlpha(0x2f), image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			rows := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, snap.LastErrorMessage, unit.Sp(11), fluent.danger, font.Normal)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.ghostIconButton(gtx, &a.dismissErrorButton, uiIconClose, false)
						}),
					)
				}),
			}
			if snap.LastRunAvailable || strings.TrimSpace(snap.Result.RawPath) != "" {
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					buttons := []layout.FlexChild{}
					if snap.LastRunAvailable {
						buttons = append(buttons, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.compactButton(gtx, &a.retryLastRunButton, "重试上次请求", false)
						}))
					}
					if strings.TrimSpace(snap.Result.RawPath) != "" {
						if len(buttons) > 0 {
							buttons = append(buttons, layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout))
						}
						buttons = append(buttons, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.compactButton(gtx, &a.openRawResponseButton, "查看日志", false)
						}))
					}
					return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, buttons...)
					})
				}))
			}
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, rows...)
		})
	})
}

func (a *App) submitActionButton(
	gtx layout.Context,
	btn *widget.Clickable,
	text string,
	bg color.NRGBA,
	hoverBg color.NRGBA,
	border color.NRGBA,
	fg color.NRGBA,
) layout.Dimensions {
	return fixedHeight(gtx, unit.Dp(44), func(gtx layout.Context) layout.Dimensions {
		return a.surfaceButton(
			gtx,
			btn,
			bg,
			hoverBg,
			border,
			unit.Dp(10),
			layout.Inset{Top: 0, Bottom: 0, Left: 12, Right: 12},
			func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, text, unit.Sp(12), fg, font.SemiBold)
				})
			},
		)
	})
}

func (a *App) layoutWorkbenchCard(gtx layout.Context) layout.Dimensions {
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		modeSummary := a.modeLabel()
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(2))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.titleLabel(gtx, "图像工作台", unit.Sp(18))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.workbenchModeBadge(gtx, modeSummary)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, "保持界面简洁，把注意力留给 prompt、参考图和结果。", unit.Sp(10), fluent.textDim, font.Normal)
			}),
		)
	})
}

func (a *App) layoutModeCard(gtx layout.Context) layout.Dimensions {
	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, "模式", unit.Sp(11), fluent.textMuted, font.SemiBold)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.borderedSurface(gtx, fluent.surface2, unit.Dp(10), fluent.border, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(2)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						children := make([]layout.FlexChild, 0, len(modeChoices))
						for idx := range modeChoices {
							idx := idx
							for a.modeButtons[idx].Clicked(gtx) {
								a.mode = modeChoices[idx].Value
							}
							children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								active := a.mode == modeChoices[idx].Value
								icon := uiIconPlay
								if modeChoices[idx].Value == string(client.ModeEdit) {
									icon = uiIconEdit
								}
								return a.surfaceButton(
									gtx,
									&a.modeButtons[idx],
									chooseColor(active, fluent.accentSoft, rgba(0xffffff, 0x00)),
									chooseColor(active, accentAlpha(0x18), fluent.surface),
									chooseColor(active, accentAlpha(0x32), rgba(0xffffff, 0x00)),
									unit.Dp(8),
									layout.Inset{Top: 9, Bottom: 9, Left: 10, Right: 10},
									func(gtx layout.Context) layout.Dimensions {
										fg := chooseColor(active, fluent.accent, fluent.textMuted)
										return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
														return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
															return icon.Layout(gtx, fg)
														})
													})
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.label(gtx, modeChoices[idx].Label, unit.Sp(11), fg, chooseFontWeight(active))
												}),
											)
										})
									},
								)
							}))
						}
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(2))}.Layout(gtx, children...)
					})
				})
			}),
		)
	})
}

func (a *App) layoutPromptCard(gtx layout.Context, snap snapshot, promptLen int) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingPromptCard, time.Now())
	for a.promptHelperButton.Clicked(gtx) {
		if !a.promptHelperOpen {
			a.promptHelperTab = "templates"
		}
		a.promptHelperOpen = !a.promptHelperOpen
	}
	for a.openPromptTemplateManagerButton.Clicked(gtx) {
		a.openPromptTemplateManager()
	}
	for a.openPresetManagerFromPromptButton.Clicked(gtx) {
		a.openPresetManager()
	}
	for a.optimizePromptButton.Clicked(gtx) {
		a.startPromptOptimize()
	}

	title := "提示词"
	hint := "主体 / 场景 / 光照 / 镜头 / 风格\n例如：一只橘猫坐在雨夜窗边，电影级侧逆光，50mm，浅景深，写实摄影"
	if a.mode == string(client.ModeEdit) {
		title = "修改要求"
		hint = "主体保持不变\n把背景换成夜空，补一圈冷色边缘光，保留原有构图"
	}

	return a.card(gtx, func(gtx layout.Context) layout.Dimensions {
		base := func(gtx layout.Context) layout.Dimensions {
			promptBorder := fluent.border2
			if gtx.Focused(&a.promptInput) {
				promptBorder = accentAlpha(0xb8)
			}
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, title, unit.Sp(10), fluent.textMuted, font.SemiBold)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.staticPill(gtx, fmt.Sprintf("%d", promptLen), false, true)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, unit.Dp(124), func(gtx layout.Context) layout.Dimensions {
						return a.borderedSurface(gtx, fluent.surface, fluentInputRadius, promptBorder, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.editorText(gtx, &a.promptInput, hint, unit.Sp(13))
							})
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(32), func(gtx layout.Context) layout.Dimensions {
								return a.ghostIconTextButton(gtx, &a.promptHelperButton, uiIconHistory, "模板 / 历史", a.promptHelperOpen)
							})
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(32), func(gtx layout.Context) layout.Dimensions {
								return a.ghostIconTextButton(gtx, &a.openPromptTemplateManagerButton, uiIconEdit, "管理模板", a.promptTemplateManagerOpen)
							})
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(32), func(gtx layout.Context) layout.Dimensions {
								return a.ghostIconTextButton(gtx, &a.openPresetManagerFromPromptButton, uiIconSave, "参数预设", a.presetManagerOpen)
							})
						}),
						layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := "AI 优化"
							if snap.OptimizingPrompt {
								label = "优化中..."
							}
							icon := uiIconSpark
							if snap.OptimizingPrompt {
								icon = uiIconRefresh
							}
							return fixedHeight(gtx, unit.Dp(32), func(gtx layout.Context) layout.Dimensions {
								return a.ghostIconTextButton(gtx, &a.optimizePromptButton, icon, label, snap.OptimizingPrompt)
							})
						}),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.staticPill(gtx, "Ctrl+Enter", false, true)
						}),
					)
				}),
			)
		}
		if !a.promptHelperOpen {
			return base(gtx)
		}
		promptSuggestions := a.promptSuggestions(snap.History)
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(base),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				macro := op.Record(gtx.Ops)
				overlayDims := layout.Inset{Left: unit.Dp(8), Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.layoutPromptHelperInline(gtx, promptSuggestions)
				})
				call := macro.Stop()
				offsetY := gtx.Dp(unit.Dp(152))
				if offsetY+overlayDims.Size.Y > gtx.Constraints.Max.Y {
					offsetY = gtx.Constraints.Max.Y - overlayDims.Size.Y
				}
				if offsetY < 0 {
					offsetY = 0
				}
				trans := op.Offset(image.Pt(0, offsetY)).Push(gtx.Ops)
				call.Add(gtx.Ops)
				trans.Pop()
				return layout.Dimensions{}
			}),
		)
	})
}

func (a *App) layoutPromptHelperPanel(gtx layout.Context, suggestions []string) layout.Dimensions {
	items := a.promptTemplateItems()
	prefix := "prompt-template:"
	emptyText := "还没有提交过 prompt"
	if a.promptHelperTab == "history" {
		items = a.promptLabelsCached(suggestions)
		prefix = "prompt-history:"
		emptyText = "还没有提交过提示词。"
	} else if a.promptHelperTab == "presets" {
		items = a.presetLabelsCached(a.readSnapshot().Presets)
		prefix = "prompt-preset:"
		emptyText = "还没有保存参数预设。"
	}
	if len(items) == 0 {
		return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(20)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, emptyText, unit.Sp(11), fluent.textDim, font.Normal)
				})
			})
		})
	}
	return fixedHeight(gtx, unit.Dp(308), func(gtx layout.Context) layout.Dimensions {
		return a.promptHelperList.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
			item := items[index]
			return layout.Inset{Bottom: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return a.layoutPromptHelperItem(gtx, prefix+item.ID, item)
			})
		})
	})
}

func (a *App) layoutPromptHelperInline(gtx layout.Context, suggestions []string) layout.Dimensions {
	for a.closePromptHelperButton.Clicked(gtx) {
		a.promptHelperOpen = false
	}
	for a.promptHelperTemplatesButton.Clicked(gtx) {
		a.promptHelperTab = "templates"
	}
	for a.promptHelperPresetsButton.Clicked(gtx) {
		a.promptHelperTab = "presets"
	}
	for a.promptHelperHistoryButton.Clicked(gtx) {
		a.promptHelperTab = "history"
	}
	return fixedWidth(gtx, unit.Dp(360), func(gtx layout.Context) layout.Dimensions {
		return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.layoutPromptHelperTabs(gtx, len(a.promptTemplateItems()), len(a.readSnapshot().Presets), len(suggestions))
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(6)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.ghostIconButton(gtx, &a.closePromptHelperButton, uiIconClose, false)
							}),
						)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedHeight(gtx, unit.Dp(1), func(gtx layout.Context) layout.Dimensions {
						return a.surface(gtx, fluent.border, 0, layout.Spacer{}.Layout)
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.layoutPromptHelperPanel(gtx, suggestions)
					})
				}),
			)
		})
	})
}

func (a *App) layoutPromptHelperModal(gtx layout.Context) layout.Dimensions {
	for a.closePromptHelperButton.Clicked(gtx) {
		a.promptHelperOpen = false
	}
	for a.promptHelperTemplatesButton.Clicked(gtx) {
		a.promptHelperTab = "templates"
	}
	for a.promptHelperPresetsButton.Clicked(gtx) {
		a.promptHelperTab = "presets"
	}
	for a.promptHelperHistoryButton.Clicked(gtx) {
		a.promptHelperTab = "history"
	}
	snap := a.readSnapshot()
	suggestions := a.promptSuggestions(snap.History)
	return a.layoutStandardModal(
		gtx,
		unit.Dp(560),
		0,
		"模板 / 历史",
		"",
		&a.closePromptHelperButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutPromptHelperTabs(gtx, len(a.promptTemplateItems()), len(snap.Presets), len(suggestions))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutPromptHelperPanel(gtx, suggestions)
				}),
			)
		},
	)
}

func (a *App) workbenchModeBadge(gtx layout.Context, text string) layout.Dimensions {
	lines := []string{text}
	if strings.TrimSpace(text) == "文生图" {
		lines = []string{"文生", "图"}
	} else if strings.TrimSpace(text) == "图生图" {
		lines = []string{"图生", "图"}
	}
	return fixedWidth(gtx, unit.Dp(56), func(gtx layout.Context) layout.Dimensions {
		return fixedHeight(gtx, unit.Dp(40), func(gtx layout.Context) layout.Dimensions {
			return a.elevatedBorderedSurface(gtx, fluent.accentSoft, unit.Dp(8), accentAlpha(0x22), image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					rows := make([]layout.FlexChild, 0, len(lines)*2)
					for idx, line := range lines {
						line := line
						if idx > 0 {
							rows = append(rows, layout.Rigid(layout.Spacer{Height: unit.Dp(1)}.Layout))
						}
						rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, line, unit.Sp(10), fluent.accent, font.SemiBold)
							})
						}))
					}
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx, rows...)
				})
			})
		})
	})
}

func chooseFontWeight(active bool) font.Weight {
	if active {
		return font.SemiBold
	}
	return font.Medium
}

func (a *App) layoutPromptHelperTabs(gtx layout.Context, templateCount int, presetCount int, historyCount int) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.surfaceButton(
				gtx,
				&a.promptHelperTemplatesButton,
				chooseColor(a.promptHelperTab == "templates", fluent.accentSoft, rgba(0xffffff, 0x00)),
				fluent.surface2,
				rgba(0xffffff, 0x00),
				fluentControlRadius,
				layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, fmt.Sprintf("模板 (%d)", templateCount), unit.Sp(11), chooseColor(a.promptHelperTab == "templates", fluent.accent, fluent.textMuted), chooseFontWeight(a.promptHelperTab == "templates"))
					})
				},
			)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.surfaceButton(
				gtx,
				&a.promptHelperPresetsButton,
				chooseColor(a.promptHelperTab == "presets", fluent.accentSoft, rgba(0xffffff, 0x00)),
				fluent.surface2,
				rgba(0xffffff, 0x00),
				fluentControlRadius,
				layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, fmt.Sprintf("预设 (%d)", presetCount), unit.Sp(11), chooseColor(a.promptHelperTab == "presets", fluent.accent, fluent.textMuted), chooseFontWeight(a.promptHelperTab == "presets"))
					})
				},
			)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := fmt.Sprintf("历史 (%d)", historyCount)
			return a.surfaceButton(
				gtx,
				&a.promptHelperHistoryButton,
				chooseColor(a.promptHelperTab == "history", fluent.accentSoft, rgba(0xffffff, 0x00)),
				fluent.surface2,
				rgba(0xffffff, 0x00),
				fluentControlRadius,
				layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, label, unit.Sp(11), chooseColor(a.promptHelperTab == "history", fluent.accent, fluent.textMuted), chooseFontWeight(a.promptHelperTab == "history"))
					})
				},
			)
		}),
	)
}

func (a *App) layoutPromptHelperItem(gtx layout.Context, buttonID string, item promptHelperItem) layout.Dimensions {
	btn := a.promptButton(buttonID)
	for btn.Clicked(gtx) {
		a.applyPromptHelperItem(item)
	}
	return a.surfaceButton(
		gtx,
		btn,
		rgba(0xffffff, 0x00),
		fluent.accentSoft,
		rgba(0xffffff, 0x00),
		unit.Dp(10),
		layout.Inset{Top: 9, Bottom: 9, Left: 12, Right: 12},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.clampedLabel(gtx, item.Title, unit.Sp(12), fluent.text, font.SemiBold, 2)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if strings.TrimSpace(item.Detail) == "" || strings.TrimSpace(item.Detail) == strings.TrimSpace(item.Title) {
						return layout.Dimensions{}
					}
					return a.clampedLabel(gtx, item.Detail, unit.Sp(11), fluent.textDim, font.Normal, 3)
				}),
			)
		},
	)
}

func promptHelperApplyText(item promptHelperItem) string {
	if strings.TrimSpace(item.Detail) != "" {
		return item.Detail
	}
	return item.Title
}

func (a *App) promptTemplateItems() []promptHelperItem {
	a.mu.Lock()
	defer a.mu.Unlock()
	items := make([]promptHelperItem, 0, len(a.promptTemplates)+len(builtInPromptTemplates))
	for _, item := range a.promptTemplates {
		title := strings.TrimSpace(item.Label)
		if title == "" {
			title = shortPrompt(item.Text)
		}
		items = append(items, promptHelperItem{
			ID:     strings.TrimSpace(item.ID),
			Title:  title,
			Detail: strings.TrimSpace(item.Text),
			Kind:   "template",
		})
	}
	for _, item := range builtInPromptTemplates {
		item.Kind = "template"
		items = append(items, item)
	}
	return items
}

func (a *App) applyPromptHelperItem(item promptHelperItem) {
	switch strings.TrimSpace(item.Kind) {
	case "preset":
		if a.applyPresetByID(item.ID) {
			return
		}
	default:
		a.applyPromptSuggestion(promptHelperApplyText(item))
	}
}

func (a *App) layoutSettingsModal(gtx layout.Context, snap snapshot) layout.Dimensions {
	for a.closeSettingsButton.Clicked(gtx) {
		a.closeSettingsModal()
	}
	for a.settingsHelpButton.Clicked(gtx) {
		a.settingsHelpOpen = true
	}
	for a.closeSettingsHelpButton.Clicked(gtx) {
		a.settingsHelpOpen = false
	}
	for a.saveSettingsButton.Clicked(gtx) {
		if !a.settingsDraftReady() {
			continue
		}
		if err := a.saveSettingsSelection(); err != nil {
			a.appendLog("保存配置失败: " + err.Error())
			continue
		}
		a.closeSettingsModal()
	}
	for a.toggleAPIKeyMaskButton.Clicked(gtx) {
		a.apiKeyVisible = !a.apiKeyVisible
	}
	for a.settingsImagesCompatButton.Clicked(gtx) {
		a.imagesNewAPICompat = !a.imagesNewAPICompat
	}
	for a.loadUpstreamModelsButton.Clicked(gtx) {
		if !a.settingsDraftReady() {
			continue
		}
		if err := a.testSettingsSelection(); err != nil {
			a.appendLog("拉取模型列表失败: " + err.Error())
			continue
		}
	}
	for a.settingsTestUpstreamButton.Clicked(gtx) {
		if !a.settingsDraftReady() {
			continue
		}
		if err := a.testSettingsSelection(); err != nil {
			a.appendLog("保存配置失败: " + err.Error())
			continue
		}
	}
	for a.syncCodexConfigButton.Clicked(gtx) {
		a.startCodexConfigSync()
	}
	for a.exportUpstreamConfigsButton.Clicked(gtx) {
		a.exportUpstreamConfigs()
	}
	for a.importUpstreamConfigsButton.Clicked(gtx) {
		a.importUpstreamConfigsFromFile()
	}
	for a.openQuickImportUpstreamConfigsButton.Clicked(gtx) {
		a.upstreamQuickImportOpen = true
	}
	for a.closeQuickImportUpstreamConfigsButton.Clicked(gtx) {
		a.upstreamQuickImportOpen = false
	}
	for a.confirmQuickImportUpstreamConfigsButton.Clicked(gtx) {
		if err := a.importUpstreamConfigsFromRaw(a.upstreamQuickImportInput.Text()); err != nil {
			a.appendLog("快捷导入失败: " + err.Error())
			continue
		}
		a.upstreamQuickImportInput.SetText("")
		a.upstreamQuickImportOpen = false
	}
	for a.createProfileButton.Clicked(gtx) {
		if err := a.createSettingsProfile(string(client.APIModeResponses)); err != nil {
			a.appendLog("创建配置失败: " + err.Error())
		}
	}
	for a.createImagesProfileButton.Clicked(gtx) {
		if err := a.createSettingsProfile(string(client.APIModeImages)); err != nil {
			a.appendLog("创建配置失败: " + err.Error())
		}
	}
	activeName := strings.TrimSpace(activeProfileName(snap.Profiles, snap.ActiveProfileID))
	if activeName == "" {
		activeName = "未命名配置"
	}
	activeMode := activeProfileAPIMode(snap.Profiles, snap.ActiveProfileID)
	if activeMode == "" {
		activeMode = a.api
	}
	if a.apiKeyVisible {
		a.apiKeyInput.Mask = 0
	} else {
		a.apiKeyInput.Mask = '*'
	}
	if len(snap.Profiles) == 0 {
		return a.layoutStandardModal(
			gtx,
			unit.Dp(760),
			0,
			"上游配置",
			"",
			&a.closeSettingsButton,
			func(gtx layout.Context) layout.Dimensions {
				return a.layoutSettingsEmptyState(gtx, snap)
			},
		)
	}
	return a.layoutStandardModal(
		gtx,
		unit.Dp(760),
		unit.Dp(620),
		"上游配置",
		"",
		&a.closeSettingsButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(240), func(gtx layout.Context) layout.Dimensions {
						return a.layoutSettingsProfileRail(gtx, snap)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.layoutSettingsEditorPane(gtx, snap)
				}),
			)
		},
	)
}

func (a *App) layoutSettingsHelpModal(gtx layout.Context) layout.Dimensions {
	return a.layoutStandardModal(
		gtx,
		unit.Dp(620),
		unit.Dp(640),
		"接口说明",
		"上游配置 / 常见问题",
		&a.closeSettingsHelpButton,
		func(gtx layout.Context) layout.Dimensions {
			sections := []layout.Widget{
				func(gtx layout.Context) layout.Dimensions {
					return a.helpInfoCard(gtx, "Responses API 与 Images API 怎么选?", "最关键的一条。先看你的 key 绑在哪个分组。", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "Responses API 走 /v1/responses + image_generation 工具，SSE 保活，长推理更稳，Cloudflare 524 风险更低。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "Images API 走标准 /v1/images/generations 与 /v1/images/edits，兼容性最广，但没有 SSE 保活。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
						)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.helpInfoCard(gtx, "支持哪些上游中转站?", "", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "不内置任何默认上游。首次打开时填写你自己的 BASE_URL、API Key，再选择 API 形态。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "只提供 /v1/chat/completions 的中转站不兼容；本应用不发 chat 请求。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
						)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.helpInfoCard(gtx, "模型 ID 怎么填?", "", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "Responses API 会同时用到文本模型 ID 与图像模型 ID；Images API 只读取图像模型 ID。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "留空时默认文本模型是 gpt-5.5，默认图像模型是 gpt-image-2。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
						)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.helpInfoCard(gtx, "BASE_URL 与参数策略", "", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "BASE_URL 只填中转站根地址。应用会按当前 API 形态自动拼接 /v1/...，不要手动把完整路径贴进来。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "OpenAI 标准只发官方公开字段；兼容中转扩展会额外附带 relay 常见扩展字段，例如 seed / negative_prompt。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
						)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.helpInfoCard(gtx, "生成失败 / 524 怎么办?", "", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "优先检查 key 是否过期、是否绑对分组、余额是否足够。Responses API 通常比 Images API 更抗超时。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "排查时可从历史结果详情或运行日志里打开 Raw response 文件，看上游实际返回。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
						)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.helpInfoCard(gtx, "数据存在哪里?", "", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "API Key 只保存在系统凭据存储中；历史记录与配置元数据在本地兼容状态文件里；生成图片默认保存在输出目录。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "除了向你配置的上游发请求，本应用不会把这些数据上传到其他服务器。", unit.Sp(11), fluent.textMuted, font.Normal)
							}),
						)
					})
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.helpInfoCard(gtx, "快捷键", "", func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.monoLabel(gtx, "Ctrl+Enter 提交生成  ·  Ctrl+T 新建标签  ·  Ctrl+W 关闭标签", unit.Sp(10), fluent.textDim, font.Normal)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.monoLabel(gtx, "Ctrl+C / Ctrl+V 复制粘贴图片  ·  Delete 删除标注  ·  Esc 关闭弹层", unit.Sp(10), fluent.textDim, font.Normal)
							}),
						)
					})
				},
			}
			return a.settingsList.Layout(gtx, len(sections), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, sections[index])
			})
		},
	)
}

func (a *App) layoutGeneralSettingsModal(gtx layout.Context, snap snapshot) layout.Dimensions {
	for a.closeGeneralSettingsButton.Clicked(gtx) {
		a.closeGeneralSettingsModal()
	}
	for a.copyGeneralPerformanceDiagnosticsButton.Clicked(gtx) {
		copyResultDetailText(gtx, a.buildPerformanceDiagnosticsReport())
		a.appendLog("已复制性能诊断")
	}
	for a.openGeneralDiagnosticsDirButton.Clicked(gtx) {
		dir, err := diagnosticsDirPath()
		if err != nil {
			a.appendLog("打开诊断目录失败: " + err.Error())
		} else if err := openPath(dir); err != nil {
			a.appendLog("打开诊断目录失败: " + err.Error())
		}
	}
	for a.openGeneralLastLowFPSSnapshotButton.Clicked(gtx) {
		path := strings.TrimSpace(snap.LastLowFPSSnapshotPath)
		if path == "" {
			continue
		}
		if err := openPath(path); err != nil {
			a.appendLog("打开低帧率快照失败: " + err.Error())
		}
	}
	for a.triggerGeneralHistoryMediaBackfillButton.Clicked(gtx) {
		a.startHistoryThumbBackfill()
		a.appendLog("已开始补齐历史预览/缩略图")
	}
	for a.generalRuntimePickerButton.Clicked(gtx) {
		a.generalRuntimePickerOpen = !a.generalRuntimePickerOpen
	}
	for idx, choice := range generalKernelRuntimeChoices {
		choice := choice
		for a.generalRuntimeButtons[idx].Clicked(gtx) {
			a.kernelRuntimeMode = choice.Value
			a.generalRuntimePickerOpen = false
		}
	}
	for idx, mode := range []string{"system", "dark", "light"} {
		for a.generalThemeButtons[idx].Clicked(gtx) {
			a.persistThemeMode(mode)
		}
	}
	for idx, scale := range []float64{0.85, 1, 1.15} {
		scale := scale
		for a.generalFontScaleButtons[idx].Clicked(gtx) {
			a.applyFontScale(scale)
		}
	}
	for a.generalPerformanceButtons[0].Clicked(gtx) {
		a.applyReducedEffects(false)
	}
	for a.generalPerformanceButtons[1].Clicked(gtx) {
		a.applyReducedEffects(true)
	}
	for a.generalCompletionSoundButtons[0].Clicked(gtx) {
		a.completionSound.Enabled = true
	}
	for a.generalCompletionSoundButtons[1].Clicked(gtx) {
		a.completionSound.Enabled = false
	}
	for a.generalCompletionSoundModeButtons[0].Clicked(gtx) {
		a.completionSound.Mode = "default"
	}
	for a.generalCompletionSoundModeButtons[1].Clicked(gtx) {
		if strings.TrimSpace(a.completionSound.CustomData) == "" {
			a.importCompletionSound()
		} else {
			a.completionSound.Mode = "custom"
		}
	}
	for a.previewCompletionSoundButton.Clicked(gtx) {
		a.previewCompletionSound()
	}
	for a.chooseCompletionSoundButton.Clicked(gtx) {
		a.importCompletionSound()
	}
	for a.resetCompletionSoundButton.Clicked(gtx) {
		a.resetCompletionSoundCustom()
	}
	for a.generalCompletionNotificationButtons[0].Clicked(gtx) {
		a.setCompletionNotificationEnabled(true)
	}
	for a.generalCompletionNotificationButtons[1].Clicked(gtx) {
		a.setCompletionNotificationEnabled(false)
	}
	for a.requestCompletionNotificationButton.Clicked(gtx) {
		a.refreshCompletionNotificationPermission()
	}
	for a.generalCleanupPreviewCacheButtons[0].Clicked(gtx) {
		a.cleanupPreviewCacheOnExit = false
	}
	for a.generalCleanupPreviewCacheButtons[1].Clicked(gtx) {
		a.cleanupPreviewCacheOnExit = true
	}
	for a.generalProtectStreamPreviewButtons[0].Clicked(gtx) {
		a.protectStreamPreview = true
	}
	for a.generalProtectStreamPreviewButtons[1].Clicked(gtx) {
		a.protectStreamPreview = false
	}
	for a.generalAutoRetryButtons[0].Clicked(gtx) {
		a.autoRetryEnabled = true
	}
	for a.generalAutoRetryButtons[1].Clicked(gtx) {
		a.autoRetryEnabled = false
	}
	for a.generalLoopButtons[0].Clicked(gtx) {
		a.loopEnabled = true
	}
	for a.generalLoopButtons[1].Clicked(gtx) {
		a.loopEnabled = false
	}
	for a.generalLoopAutoSaveButtons[0].Clicked(gtx) {
		a.setLoopAutoSaveEnabled(true)
	}
	for a.generalLoopAutoSaveButtons[1].Clicked(gtx) {
		a.setLoopAutoSaveEnabled(false)
	}
	for a.generalLoopPreviewButtons[0].Clicked(gtx) {
		a.loopLivePreview = true
	}
	for a.generalLoopPreviewButtons[1].Clicked(gtx) {
		a.loopLivePreview = false
	}
	for a.generalBatchButtons[0].Clicked(gtx) {
		a.batchMode = true
	}
	for a.generalBatchButtons[1].Clicked(gtx) {
		a.batchMode = false
	}
	for a.generalBatchRetryButtons[0].Clicked(gtx) {
		a.batchRetryOnFail = true
	}
	for a.generalBatchRetryButtons[1].Clicked(gtx) {
		a.batchRetryOnFail = false
	}
	for a.generalBatchAutoAspectButtons[0].Clicked(gtx) {
		a.batchAutoAspect = ""
	}
	for a.generalBatchAutoAspectButtons[1].Clicked(gtx) {
		if strings.TrimSpace(a.batchAutoAspect) == "" {
			a.batchAutoAspect = "1k"
		}
	}
	for idx, value := range []string{"256", "512", "1k", "2k", "4k"} {
		value := value
		for a.generalBatchAutoAspectResolutionButtons[idx].Clicked(gtx) {
			a.batchAutoAspect = value
		}
	}
	for idx, count := range generalAutoRetryCountChoices {
		count := count
		for a.generalAutoRetryCountButtons[idx].Clicked(gtx) {
			a.autoRetryCount = count
		}
	}
	for a.generalSavePromptButtons[0].Clicked(gtx) {
		a.setSavePromptSuppressed(false)
	}
	for a.generalSavePromptButtons[1].Clicked(gtx) {
		a.setSavePromptSuppressed(true)
	}
	for a.generalKeepLogsButtons[0].Clicked(gtx) {
		a.keepLogs = false
	}
	for a.generalKeepLogsButtons[1].Clicked(gtx) {
		a.keepLogs = true
	}
	for a.openGeneralOutputButton.Clicked(gtx) {
		if err := openPath(a.outputDirInput.Text()); err != nil {
			a.appendLog("打开输出目录失败: " + err.Error())
		}
	}
	for a.chooseGeneralOutputButton.Clicked(gtx) {
		dir, err := chooseDirectory()
		if err != nil {
			a.appendLog("选择输出目录失败: " + err.Error())
			continue
		}
		if strings.TrimSpace(dir) != "" {
			a.outputDirInput.SetText(dir)
		}
	}
	for a.resetGeneralOutputButton.Clicked(gtx) {
		a.outputDirInput.SetText(kernel.DefaultOutputDir())
	}
	for a.chooseGeneralLoopAutoSaveDirButton.Clicked(gtx) {
		a.chooseLoopAutoSaveDir("选择循环自动另存为目录失败: ")
	}
	for a.useGeneralLoopOutputDirButton.Clicked(gtx) {
		a.useCurrentOutputDirForLoopAutoSave()
	}
	for a.chooseGeneralBatchInputButton.Clicked(gtx) {
		dir, err := chooseDirectory()
		if err != nil {
			a.appendLog("选择批处理输入目录失败: " + err.Error())
			continue
		}
		if strings.TrimSpace(dir) != "" {
			a.batchInputDirInput.SetText(dir)
		}
	}
	for a.chooseGeneralBatchFilesButton.Clicked(gtx) {
		paths, err := chooseImageFiles()
		if err != nil {
			a.appendLog("选择批处理图片失败: " + err.Error())
			continue
		}
		if len(paths) > 0 {
			a.batchInputDirInput.SetText("")
			a.sourcePathsInput.SetText(strings.Join(paths, "\n"))
			a.sourceButtons = map[string]*widget.Clickable{}
		}
	}
	for a.chooseGeneralBatchOutputButton.Clicked(gtx) {
		dir, err := chooseDirectory()
		if err != nil {
			a.appendLog("选择批处理输出目录失败: " + err.Error())
			continue
		}
		if strings.TrimSpace(dir) != "" {
			a.batchOutputDirInput.SetText(dir)
			a.batchOutputDir = dir
		}
	}
	a.syncLoopSettingsFromInputs()
	for a.openGeneralHistoryTimelineButton.Clicked(gtx) {
		a.closeGeneralSettingsModal()
		a.openHistoryTimeline()
	}
	for a.exportGeneralHistoryButton.Clicked(gtx) {
		a.exportHistoryJSON()
	}
	for a.importGeneralHistoryButton.Clicked(gtx) {
		a.importHistoryJSON()
	}
	for a.clearGeneralAPIKeyButton.Clicked(gtx) {
		a.clearCurrentProfileAPIKey()
	}
	for a.clearGeneralHistoryButton.Clicked(gtx) {
		a.clearAllHistory()
	}
	for a.openGeneralAboutButton.Clicked(gtx) {
		a.aboutModalOpen = true
	}
	for a.openPresetManagerButton.Clicked(gtx) {
		a.openPresetManager()
	}
	for a.openCustomAspectRatioManagerButton.Clicked(gtx) {
		a.openCustomAspectRatioManager()
	}
	for idx, days := range []int{3, 7} {
		days := days
		for a.pruneGeneralHistoryButtons[idx].Clicked(gtx) {
			a.pruneHistoryOlderThanDays(days)
		}
	}
	for a.openGeneralUpstreamButton.Clicked(gtx) {
		a.closeGeneralSettingsModal()
		a.openSettingsModal()
	}
	for idx, choice := range proxyChoices {
		choice := choice
		for a.generalProxyButtons[idx].Clicked(gtx) {
			a.proxy = choice.Value
		}
	}
	for a.openGeneralRepoButton.Clicked(gtx) {
		if err := openExternalURL(repoURL); err != nil {
			a.appendLog("打开 GitHub 失败: " + err.Error())
		}
	}
	for a.openGeneralFeedbackButton.Clicked(gtx) {
		if err := openExternalURL(issuesURL); err != nil {
			a.appendLog("打开反馈页失败: " + err.Error())
		}
	}
	for a.openGeneralUpdateButton.Clicked(gtx) {
		info, _, _, _ := a.readAppUpdateState()
		if info != nil && info.HasUpdate && strings.TrimSpace(info.ReleaseTag) != strings.TrimSpace(a.ignoredReleaseTag) {
			a.mu.Lock()
			a.appUpdateModalOpen = true
			a.mu.Unlock()
			a.invalidateNow()
			continue
		}
		a.openAppUpdateRelease()
	}
	renderDiagnostics := formatRenderDiagnostics(snap)
	partialPreview := strings.TrimSpace(a.partialImagesInput.Text())
	if partialPreview == "" {
		partialPreview = strconv.Itoa(kernel.DefaultConfig().PartialImages)
	}
	partialPreviewSummary := partialPreview + " 帧"
	if partialPreview == "0" {
		partialPreviewSummary = "仅最终图"
	}
	historyThumbPathsPresent := 0
	historyPreviewPathsPresent := 0
	for _, item := range snap.History {
		if strings.TrimSpace(item.ThumbPath) != "" {
			historyThumbPathsPresent++
		}
		if strings.TrimSpace(item.PreviewPath) != "" {
			historyPreviewPathsPresent++
		}
	}
	historyThumbPathsMissing := max(0, len(snap.History)-historyThumbPathsPresent)
	historyPreviewPathsMissing := max(0, len(snap.History)-historyPreviewPathsPresent)
	historyThumbCoverage := 0.0
	historyPreviewCoverage := 0.0
	if len(snap.History) > 0 {
		historyThumbCoverage = float64(historyThumbPathsPresent) * 100 / float64(len(snap.History))
		historyPreviewCoverage = float64(historyPreviewPathsPresent) * 100 / float64(len(snap.History))
	}
	a.mu.Lock()
	historyBackfillInFlight := len(a.historyThumbBackfillInFlight)
	layoutShellEMA := a.layoutShellEMA
	layoutControlsEMA := a.layoutControlsEMA
	layoutSubmitDockEMA := a.layoutSubmitDockEMA
	layoutActionsEMA := a.layoutActionsEMA
	layoutPromptCardEMA := a.layoutPromptCardEMA
	layoutComposeCardEMA := a.layoutComposeCardEMA
	layoutAdvancedCardEMA := a.layoutAdvancedCardEMA
	layoutCanvasEMA := a.layoutCanvasEMA
	layoutCanvasToolbarEMA := a.layoutCanvasToolbarEMA
	layoutResultSurfaceEMA := a.layoutResultSurfaceEMA
	layoutCanvasStatusEMA := a.layoutCanvasStatusEMA
	layoutHistoryRailEMA := a.layoutHistoryRailEMA
	layoutUpstreamCardEMA := a.layoutUpstreamCardEMA
	layoutHistorySummaryEMA := a.layoutHistorySummaryEMA
	layoutLatestHistoryEMA := a.layoutLatestHistoryEMA
	layoutHistoryResultsEMA := a.layoutHistoryResultsEMA
	layoutTimelineModalEMA := a.layoutTimelineModalEMA
	layoutPeaks := a.layoutPeaks
	lastHistoryThumbPrewarmAt := a.lastHistoryThumbPrewarmAt
	lastHistoryThumbPrewarmMs := a.lastHistoryThumbPrewarmMs
	lastHistoryThumbPrewarmLoad := a.lastHistoryThumbPrewarmLoad
	lastHistoryThumbPrewarmFail := a.lastHistoryThumbPrewarmFail
	controlsVisible := !a.fullscreen
	submitDockVisible := !a.fullscreen
	actionsVisible := !a.fullscreen
	controlCardsVisible := !a.fullscreen
	historyRailVisible := !a.fullscreen
	timelineVisible := a.historyTimelineOpen
	a.mu.Unlock()
	thumbDecodeQueueLen := thumbDecodeQueueLen()
	thumbDecodeBusyCount := thumbDecodeBusyCount()
	thumbDecodeQueuePeak := thumbDecodeQueuePeakCount()
	thumbDecodeBusyPeak := thumbDecodeBusyPeakCount()
	thumbRequests := thumbDisplayRequestCount()
	thumbHits := thumbDisplayCacheHitCount()
	thumbLoadsQueued := thumbDisplayLoadQueuedCount()
	historyThumbPreviewHits := historyThumbSourcePreviewCount()
	historyThumbThumbHits := historyThumbSourceThumbCount()
	historyThumbSavedHits := historyThumbSourceSavedCount()
	canvasManagedPreviewHits := canvasDisplaySourceManagedPreviewCount()
	canvasPathThumbHits := canvasDisplaySourcePathThumbCount()
	canvasHistoryScaledHits := canvasDisplaySourceHistoryScaledCount()
	canvasInlineHits := canvasDisplaySourceInlineCount()
	thumbMisses := max(0, int(thumbRequests-thumbHits))
	thumbHitRate := 0.0
	if thumbRequests > 0 {
		thumbHitRate = float64(thumbHits) * 100 / float64(thumbRequests)
	}
	historyThumbPrewarmSummary := "最近预热 尚未运行"
	if !lastHistoryThumbPrewarmAt.IsZero() {
		historyThumbPrewarmSummary = fmt.Sprintf(
			"最近预热 %d 项 · 失败 %d · %.1fms · %s",
			lastHistoryThumbPrewarmLoad,
			lastHistoryThumbPrewarmFail,
			float64(lastHistoryThumbPrewarmMs)/float64(time.Millisecond),
			lastHistoryThumbPrewarmAt.Format("15:04:05"),
		)
	}
	currentResultSavedPresent := false
	currentResultPreviewPresent := false
	currentResultThumbPresent := false
	currentResultManagedPreviewReady := false
	currentResultCanvasTarget := a.effectiveCanvasMaxDimension()
	if item := snap.Result.Item; strings.TrimSpace(item.SavedPath) != "" {
		currentResultSavedPresent = headlessPathReady(item.SavedPath)
		currentResultPreviewPresent = headlessPathReady(item.PreviewPath)
		currentResultThumbPresent = headlessPathReady(item.ThumbPath)
		if previewPath, err := managedSourcePreviewPath(item.SavedPath, currentResultCanvasTarget); err == nil {
			currentResultManagedPreviewReady = headlessPathReady(previewPath)
		}
	}
	currentResultCanvasSummary := fmt.Sprintf(
		"当前结果 原图/预览/缩略图 %t/%t/%t · 受管画布预览 %t · 目标 %dpx",
		currentResultSavedPresent,
		currentResultPreviewPresent,
		currentResultThumbPresent,
		currentResultManagedPreviewReady,
		currentResultCanvasTarget,
	)
	slowestLayout := slowestLayoutSample([]layoutTimingSample{
		{name: "shell", duration: layoutShellEMA, visible: true},
		{name: "controls", duration: layoutControlsEMA, visible: controlsVisible},
		{name: "submit_dock", duration: layoutSubmitDockEMA, visible: submitDockVisible},
		{name: "actions", duration: layoutActionsEMA, visible: actionsVisible},
		{name: "prompt_card", duration: layoutPromptCardEMA, visible: controlCardsVisible},
		{name: "compose_card", duration: layoutComposeCardEMA, visible: controlCardsVisible},
		{name: "advanced_card", duration: layoutAdvancedCardEMA, visible: controlCardsVisible},
		{name: "canvas", duration: layoutCanvasEMA, visible: true},
		{name: "canvas_toolbar", duration: layoutCanvasToolbarEMA, visible: true},
		{name: "result_surface", duration: layoutResultSurfaceEMA, visible: true},
		{name: "canvas_status", duration: layoutCanvasStatusEMA, visible: true},
		{name: "history_rail", duration: layoutHistoryRailEMA, visible: historyRailVisible},
		{name: "upstream_card", duration: layoutUpstreamCardEMA, visible: historyRailVisible},
		{name: "history_summary", duration: layoutHistorySummaryEMA, visible: historyRailVisible},
		{name: "latest_history", duration: layoutLatestHistoryEMA, visible: historyRailVisible},
		{name: "history_results", duration: layoutHistoryResultsEMA, visible: historyRailVisible},
		{name: "history_timeline", duration: layoutTimelineModalEMA, visible: timelineVisible},
	})
	slowestLayoutPeak := slowestLayoutSample([]layoutTimingSample{
		{name: "shell", duration: layoutPeaks[layoutTimingShell], visible: true},
		{name: "controls", duration: layoutPeaks[layoutTimingControls], visible: controlsVisible},
		{name: "submit_dock", duration: layoutPeaks[layoutTimingSubmitDock], visible: submitDockVisible},
		{name: "actions", duration: layoutPeaks[layoutTimingActions], visible: actionsVisible},
		{name: "prompt_card", duration: layoutPeaks[layoutTimingPromptCard], visible: controlCardsVisible},
		{name: "compose_card", duration: layoutPeaks[layoutTimingComposeCard], visible: controlCardsVisible},
		{name: "advanced_card", duration: layoutPeaks[layoutTimingAdvancedCard], visible: controlCardsVisible},
		{name: "canvas", duration: layoutPeaks[layoutTimingCanvas], visible: true},
		{name: "canvas_toolbar", duration: layoutPeaks[layoutTimingCanvasToolbar], visible: true},
		{name: "result_surface", duration: layoutPeaks[layoutTimingResultSurface], visible: true},
		{name: "canvas_status", duration: layoutPeaks[layoutTimingCanvasStatusBar], visible: true},
		{name: "history_rail", duration: layoutPeaks[layoutTimingHistoryRail], visible: historyRailVisible},
		{name: "upstream_card", duration: layoutPeaks[layoutTimingUpstreamCard], visible: historyRailVisible},
		{name: "history_summary", duration: layoutPeaks[layoutTimingHistorySummaryCard], visible: historyRailVisible},
		{name: "latest_history", duration: layoutPeaks[layoutTimingLatestHistoryCard], visible: historyRailVisible},
		{name: "history_results", duration: layoutPeaks[layoutTimingHistoryResultsCard], visible: historyRailVisible},
		{name: "history_timeline", duration: layoutPeaks[layoutTimingTimelineModal], visible: timelineVisible},
	})

	themeRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactIconTextButton(gtx, &a.generalThemeButtons[0], uiIconSystem, "系统", a.themeMode == "system")
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactIconTextButton(gtx, &a.generalThemeButtons[1], uiIconDark, "深色", a.themeMode == "dark")
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactIconTextButton(gtx, &a.generalThemeButtons[2], uiIconLight, "浅色", a.themeMode == "light")
		}),
	}
	fontScaleRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalFontScaleButtons[0], "小", a.fontScale == 0.85)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalFontScaleButtons[1], "中", a.fontScale == 1)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalFontScaleButtons[2], "大", a.fontScale == 1.15)
		}),
	}
	performanceRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalPerformanceButtons[0], "标准", !a.reducedEffects)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalPerformanceButtons[1], "低特效", a.reducedEffects)
		}),
	}
	savePromptRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactIconTextButton(gtx, &a.generalSavePromptButtons[0], uiIconSave, "提示", !a.savePromptSuppressed)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalSavePromptButtons[1], "不提示", a.savePromptSuppressed)
		}),
	}
	completionSoundRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalCompletionSoundButtons[0], "开启", a.completionSound.Enabled)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalCompletionSoundButtons[1], "关闭", !a.completionSound.Enabled)
		}),
	}
	completionSoundModeRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalCompletionSoundModeButtons[0], "默认音", a.completionSound.Mode != "custom")
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalCompletionSoundModeButtons[1], "自定义", a.completionSound.Mode == "custom" && strings.TrimSpace(a.completionSound.CustomData) != "")
		}),
	}
	completionNotificationRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalCompletionNotificationButtons[0], "开启", a.completionNotification.Enabled)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalCompletionNotificationButtons[1], "关闭", !a.completionNotification.Enabled)
		}),
	}
	protectStreamPreviewRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalProtectStreamPreviewButtons[0], "开启", a.protectStreamPreview)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalProtectStreamPreviewButtons[1], "关闭", !a.protectStreamPreview)
		}),
	}
	autoRetryRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalAutoRetryButtons[0], "开启", a.autoRetryEnabled)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalAutoRetryButtons[1], "关闭", !a.autoRetryEnabled)
		}),
	}
	loopRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalLoopButtons[0], "开启", a.loopEnabled)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalLoopButtons[1], "关闭", !a.loopEnabled)
		}),
	}
	loopAutoSaveRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalLoopAutoSaveButtons[0], "自动另存为", a.loopAutoSave)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalLoopAutoSaveButtons[1], "不自动保存", !a.loopAutoSave)
		}),
	}
	loopPreviewRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalLoopPreviewButtons[0], "实时预览开", a.loopLivePreview)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalLoopPreviewButtons[1], "实时预览关", !a.loopLivePreview)
		}),
	}
	batchRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalBatchButtons[0], "开启", a.batchMode)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalBatchButtons[1], "关闭", !a.batchMode)
		}),
	}
	batchRetryRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalBatchRetryButtons[0], "自动重试", a.batchRetryOnFail)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalBatchRetryButtons[1], "失败跳过", !a.batchRetryOnFail)
		}),
	}
	batchAutoAspectRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalBatchAutoAspectButtons[0], "沿用当前比例", strings.TrimSpace(a.batchAutoAspect) == "")
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalBatchAutoAspectButtons[1], "按源图比例", strings.TrimSpace(a.batchAutoAspect) != "")
		}),
	}
	keepLogsRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalKeepLogsButtons[0], "关闭", !a.keepLogs)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalKeepLogsButtons[1], "开启", a.keepLogs)
		}),
	}
	cleanupPreviewCacheRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalCleanupPreviewCacheButtons[0], "关闭", !a.cleanupPreviewCacheOnExit)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.generalCleanupPreviewCacheButtons[1], "开启", a.cleanupPreviewCacheOnExit)
		}),
	}
	presetItems := a.presetLabelsCached(a.presets)
	sections := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "内核执行", func(gtx layout.Context) layout.Dimensions {
				rows := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedHeight(gtx, unit.Dp(40), func(gtx layout.Context) layout.Dimensions {
							return a.timelineFilterButton(gtx, &a.generalRuntimePickerButton, kernelRuntimeModeLabel(a.kernelRuntimeMode), a.generalRuntimePickerOpen)
						})
					}),
				}
				if a.generalRuntimePickerOpen {
					rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								menuRows := make([]layout.FlexChild, 0, len(generalKernelRuntimeChoices)*2)
								for idx, choice := range generalKernelRuntimeChoices {
									idx := idx
									choice := choice
									if idx > 0 {
										menuRows = append(menuRows, layout.Rigid(layout.Spacer{Height: unit.Dp(4)}.Layout))
									}
									menuRows = append(menuRows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.compactButton(gtx, &a.generalRuntimeButtons[idx], choice.Title, a.kernelRuntimeMode == choice.Value)
									}))
								}
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx, menuRows...)
							})
						})
					}))
				}
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "桌面可切到 remote 对照 Android / Worker 是否走同一套共享请求内核。", unit.Sp(10), fluent.textDim, font.Normal)
				}))
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, rows...)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "代理服务器", func(gtx layout.Context) layout.Dimensions {
				rows := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						children := make([]layout.FlexChild, 0, len(proxyChoices))
						for idx, choice := range proxyChoices {
							idx := idx
							choice := choice
							children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.generalProxyButtons[idx], choice.Label, a.proxy == choice.Value)
							}))
						}
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, children...)
					}),
				}
				if a.proxy == client.ProxyModeCustom || strings.TrimSpace(a.proxyURLInput.Text()) != "" {
					rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.technicalField(gtx, "", &a.proxyURLInput, "http://127.0.0.1:7890", unit.Dp(40))
					}))
				}
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "默认使用系统代理；自定义地址支持 http:// 和 https://。", unit.Sp(10), fluent.textDim, font.Normal)
				}))
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, rows...)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "输出目录", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.borderedSurface(gtx, fluent.surface, fluentInputRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, strings.TrimSpace(a.outputDirInput.Text()), unit.Sp(12), fluent.text, font.Normal)
							})
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.openGeneralOutputButton, uiIconFolder, "打开输出目录", false)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(78), func(gtx layout.Context) layout.Dimensions {
									return a.compactButton(gtx, &a.chooseGeneralOutputButton, "修改", false)
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(78), func(gtx layout.Context) layout.Dimensions {
									return a.compactButton(gtx, &a.resetGeneralOutputButton, "默认", false)
								})
							}),
						)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "生成后保存提示", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, savePromptRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "生成完成后询问是否另存到指定位置。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			currentSound := "使用内置默认音"
			if a.completionSound.Mode == "custom" && strings.TrimSpace(a.completionSound.CustomName) != "" && strings.TrimSpace(a.completionSound.CustomData) != "" {
				currentSound = "当前使用 " + strings.TrimSpace(a.completionSound.CustomName)
			}
			return a.generalSettingsCard(gtx, "完成提示音", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, completionSoundRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, completionSoundModeRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.previewCompletionSoundButton, uiIconPlay, "试听", false)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.chooseCompletionSoundButton, uiIconFolder, "导入音频", false)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(78), func(gtx layout.Context) layout.Dimensions {
									return a.compactButton(gtx, &a.resetCompletionSoundButton, "默认", false)
								})
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "整批任务全部完成后只播放一次。"+currentSound+"。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "系统通知", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, completionNotificationRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.compactIconTextButton(gtx, &a.requestCompletionNotificationButton, uiIconInfo, completionNotificationPermissionActionLabel(a.completionNotificationPermission), false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "整批任务全部完成后，仅在窗口不在前台时发送系统通知。当前权限："+systemNotificationPermissionLabel(a.completionNotificationPermission)+"。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "流式预览保护", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, protectStreamPreviewRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "默认开启。桌面端高并发任务时会自动关闭流式预览，优先保证最终图完整；关闭后严格按你设置的预览帧数请求。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "失败自动重试", func(gtx layout.Context) layout.Dimensions {
				rows := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, autoRetryRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						countRows := make([]layout.FlexChild, 0, len(generalAutoRetryCountChoices))
						for idx, count := range generalAutoRetryCountChoices {
							idx := idx
							count := count
							countRows = append(countRows, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.generalAutoRetryCountButtons[idx], fmt.Sprintf("%d 次", count), normalizeAutoRetryCount(a.autoRetryCount) == count)
							}))
						}
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, countRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "开启后会对可重试的上游/网络错误自动补发请求；当前次数用于限制首次请求后的额外重试上限。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				}
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, rows...)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "循环出图", func(gtx layout.Context) layout.Dimensions {
				currentOutputDir := strings.TrimSpace(a.outputDirInput.Text())
				loopAutoSaveSummary := "未开启自动另存为"
				if a.loopAutoSave {
					if dir := strings.TrimSpace(a.loopAutoSaveDirInput.Text()); dir != "" {
						loopAutoSaveSummary = "当前复制目录: " + dir
					} else {
						loopAutoSaveSummary = "已开启自动另存为,目录为空时会沿用当前输出目录"
					}
				}
				rows := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, loopRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.field(gtx, "总张数", &a.loopTotalCountInput, "10", unit.Dp(42))
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.field(gtx, "并发数", &a.loopConcurrencyInput, "2", unit.Dp(42))
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, loopAutoSaveRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.technicalField(gtx, "自动另存为目录", &a.loopAutoSaveDirInput, "留空 = 沿用当前输出目录", unit.Dp(42))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.chooseGeneralLoopAutoSaveDirButton, uiIconFolder, "选择自动另存为目录", false)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								label := "使用当前输出目录"
								if currentOutputDir == "" {
									label = "当前输出目录未设置"
								}
								return a.compactIconTextButton(gtx, &a.useGeneralLoopOutputDirButton, uiIconSave, label, false)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, loopPreviewRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "开启后会按这里的总张数和并发持续补位生成；常规出图张数只在普通模式下生效。"+loopAutoSaveSummary+"。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "关闭实时预览后不会在画布显示过程图，可以明显降低内存占用。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				}
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, rows...)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "批处理图生图", func(gtx layout.Context) layout.Dimensions {
				rows := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, batchRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.technicalField(gtx, "输入目录", &a.batchInputDirInput, "选择一个目录后扫描当前目录图片", unit.Dp(42))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.compactIconTextButton(gtx, &a.chooseGeneralBatchInputButton, uiIconFolder, "选择批处理输入目录", false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.compactIconTextButton(gtx, &a.chooseGeneralBatchFilesButton, uiIconAdd, "直接加入多张图片", false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.technicalField(gtx, "输出目录", &a.batchOutputDirInput, "留空 = 回原图目录", unit.Dp(42))
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.compactIconTextButton(gtx, &a.chooseGeneralBatchOutputButton, uiIconFolder, "选择批处理输出目录", false)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, batchRetryRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, batchAutoAspectRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if strings.TrimSpace(a.batchAutoAspect) == "" {
							return layout.Dimensions{}
						}
						rows := make([]layout.FlexChild, 0, 5)
						for idx, value := range []string{"256", "512", "1k", "2k", "4k"} {
							idx := idx
							value := value
							rows = append(rows, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.generalBatchAutoAspectResolutionButtons[idx], strings.ToUpper(value), a.batchAutoAspect == value)
							}))
						}
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, rows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "当前已支持目录扫描、多图并发执行、输出落盘；开启按源图比例后，会按每张图自身比例和这里的分辨率档位推导尺寸。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				}
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, rows...)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "日志保留", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, keepLogsRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "关闭时当前会话仍可查看原始响应，退出应用后会自动清理输出目录中的日志。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "退出时清理预览缓存", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, cleanupPreviewCacheRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "默认关闭。开启后，退出应用时会删除可重建的预览图和缩略图缓存；源图、结果图和历史记录不会删除。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "主题", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, themeRows...)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, fmt.Sprintf("字号 %d%%", int(a.fontScale*100)), func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, fontScaleRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "即时缩放 Gio 客户端里的标题、正文和输入文本。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "性能模式", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, performanceRows...)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "低特效会减少背景渐变、卡片阴影和高光装饰，适合低端 GPU 或高分辨率窗口。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "参数预设", func(gtx layout.Context) layout.Dimensions {
				rows := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if len(presetItems) == 0 {
							return a.label(gtx, "当前还没有保存参数预设。常用的尺寸、质量、格式和张数会显示在这里。", unit.Sp(10), fluent.textDim, font.Normal)
						}
						return a.label(gtx, fmt.Sprintf("已保存 %d 个参数预设，常用配置会在创作参数里直接复用。", len(presetItems)), unit.Sp(10), fluent.textDim, font.Normal)
					}),
				}
				if len(presetItems) > 0 {
					limit := min(len(presetItems), 3)
					for idx := 0; idx < limit; idx++ {
						item := presetItems[idx]
						rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
								return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.singleLineLabel(gtx, item.Title, unit.Sp(11), fluent.text, font.SemiBold)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.singleLineLabel(gtx, item.Detail, unit.Sp(10), fluent.textDim, font.Normal)
										}),
									)
								})
							})
						}))
					}
					if len(presetItems) > limit {
						rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, fmt.Sprintf("还有 %d 个预设未展开显示。", len(presetItems)-limit), unit.Sp(10), fluent.textDim, font.Normal)
						}))
					}
				}
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.openPresetManagerButton, uiIconSettings, "打开预设管理", false)
				}))
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, rows...)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "自定义比例", func(gtx layout.Context) layout.Dimensions {
				rows := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if len(a.customAspectRatios) == 0 {
							return a.label(gtx, "当前还没有自定义比例。已保存的共享比例可以在这里维护。", unit.Sp(10), fluent.textDim, font.Normal)
						}
						return a.label(gtx, fmt.Sprintf("已保存 %d 个自定义比例，会直接出现在 Gio 的比例按钮区。", len(a.customAspectRatios)), unit.Sp(10), fluent.textDim, font.Normal)
					}),
				}
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.openCustomAspectRatioManagerButton, uiIconSettings, "打开比例管理", false)
				}))
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, rows...)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "历史与数据", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "历史时间线、JSON 导入导出和最近结果管理都集中在这一组里。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.openGeneralHistoryTimelineButton, uiIconHistory, "完整历史", false)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.exportGeneralHistoryButton, uiIconDownload, "导出历史", false)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.importGeneralHistoryButton, uiIconFolder, "导入历史", false)
							}),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.pruneGeneralHistoryButtons[0], "清理 3 天前", false)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.pruneGeneralHistoryButtons[1], "清理 7 天前", false)
							}),
						)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "性能诊断", func(gtx layout.Context) layout.Dimensions {
				rows := []layout.FlexChild{
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "当前 Gio 渲染后端、帧率、内存、历史规模和最近日志可一键复制。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
				}
				if strings.TrimSpace(renderDiagnostics) != "" {
					rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, renderDiagnostics+" · 预览 "+partialPreviewSummary, unit.Sp(11), fluent.text, font.Normal)
							})
						})
					}))
				}
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, fmt.Sprintf("历史媒体 %d 条 · 缩略图 %d/%d(%.1f%%) · 预览 %d/%d(%.1f%%) · 回填 %d", len(snap.History), historyThumbPathsPresent, historyThumbPathsMissing, historyThumbCoverage, historyPreviewPathsPresent, historyPreviewPathsMissing, historyPreviewCoverage, historyBackfillInFlight), unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, fmt.Sprintf("缩略图命中率 %.1f%% · 请求 %d · 未命中 %d · 排队 %d/%d · 解码 %d/%d", thumbHitRate, thumbRequests, thumbMisses, thumbDecodeQueueLen, thumbDecodeQueuePeak, thumbDecodeBusyCount, thumbDecodeBusyPeak), unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, fmt.Sprintf("缩略图来源 预览/缩略图/原图 = %d/%d/%d · 加载排队 %d · 最慢布局 %s", historyThumbPreviewHits, historyThumbThumbHits, historyThumbSavedHits, thumbLoadsQueued, slowestLayout), unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "布局峰值 "+slowestLayoutPeak, unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, historyThumbPrewarmSummary, unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, currentResultCanvasSummary, unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, fmt.Sprintf("画布来源 受管预览/路径缩图/历史缩放/内存 = %d/%d/%d/%d", canvasManagedPreviewHits, canvasPathThumbHits, canvasHistoryScaledHits, canvasInlineHits), unit.Sp(10), fluent.textDim, font.Normal)
								}),
							)
						})
					})
				}))
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.copyGeneralPerformanceDiagnosticsButton, uiIconCopy, "复制性能诊断", false)
				}))
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.openGeneralDiagnosticsDirButton, uiIconFolder, "打开诊断目录", false)
				}))
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.triggerGeneralHistoryMediaBackfillButton, uiIconRefresh, "补齐历史预览/缩略图", false)
				}))
				if strings.TrimSpace(snap.LastLowFPSSnapshotPath) != "" {
					rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.compactIconTextButton(gtx, &a.openGeneralLastLowFPSSnapshotButton, uiIconFolder, "打开最近低帧率快照", false)
					}))
				}
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, rows...)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "上游配置", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "需要修改 BASE_URL、API Key、模型或请求策略时，从这里进入上游配置。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.compactIconTextButton(gtx, &a.openGeneralUpstreamButton, uiIconSettings, "打开上游配置", false)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "关于 Image Studio", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "独立 Gio 客户端 · 当前版本 v"+client.Version, unit.Sp(11), fluent.text, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "保持界面简洁，围绕 prompt、参考图、结果预览和上游配置提供一个更接近 Windows Fluent 的原生工作台。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.compactIconTextButton(gtx, &a.openGeneralAboutButton, uiIconInfo, "打开关于面板", false)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "凭据与清理", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "清理当前活动上游的 API Key，或直接移除本地历史记录。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.clearGeneralAPIKeyButton, "清除 API Key", false)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.clearGeneralHistoryButton, "清空历史", false)
							}),
						)
					}),
				)
			})
		},
		func(gtx layout.Context) layout.Dimensions {
			return a.generalSettingsCard(gtx, "支持与反馈", func(gtx layout.Context) layout.Dimensions {
				info, _, checking, _ := a.readAppUpdateState()
				updateLabel := "更新"
				if checking {
					updateLabel = "检查中..."
				} else if info != nil && info.HasUpdate {
					updateLabel = "更新 v" + info.LatestVersion
				}
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "代码仓库、问题反馈和更新记录都以 GitHub 为准。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.openGeneralUpdateButton, uiIconLaunch, updateLabel, false)
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.openGeneralFeedbackButton, uiIconFeedback, "反馈", false)
							}),
						)
					}),
				)
			})
		},
	}

	return a.layoutStandardModal(
		gtx,
		unit.Dp(540),
		unit.Dp(820),
		"设置",
		"",
		&a.closeGeneralSettingsButton,
		func(gtx layout.Context) layout.Dimensions {
			return a.settingsList.Layout(gtx, len(sections), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(10)}.Layout(gtx, sections[index])
			})
		},
	)
}

func (a *App) helpInfoCard(gtx layout.Context, title string, hint string, body layout.Widget) layout.Dimensions {
	return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, title, unit.Sp(12), fluent.text, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if strings.TrimSpace(hint) == "" {
						return layout.Dimensions{}
					}
					return a.label(gtx, hint, unit.Sp(10), fluent.textDim, font.Normal)
				}),
				layout.Rigid(body),
			)
		})
	})
}

func (a *App) settingsPaneCard(gtx layout.Context, title string, body layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, title, unit.Sp(11), fluent.textMuted, font.SemiBold)
		}),
		layout.Rigid(body),
	)
}

func (a *App) generalSettingsCard(gtx layout.Context, title string, body layout.Widget) layout.Dimensions {
	bg := withAlpha(fluent.white, 0xb8)
	radius := unit.Dp(12)
	if resolveThemeMode(a.themeMode) == "dark" {
		bg = fluent.surfaceElevated
		radius = unit.Dp(8)
	}
	return a.elevatedBorderedSurface(gtx, bg, radius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(13)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, title, unit.Sp(11), fluent.text, font.SemiBold)
				}),
				layout.Rigid(body),
			)
		})
	})
}

func (a *App) layoutSettingsEmptyState(gtx layout.Context, snap snapshot) layout.Dimensions {
	canSync := canLoadCodexAPIConfig()
	syncLabel := "同步 Codex 配置"
	if snap.SyncingCodexConfig {
		syncLabel = "同步中..."
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(14))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(44), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(44), func(gtx layout.Context) layout.Dimensions {
									return a.borderedSurface(gtx, fluent.accentSoft, unit.Dp(14), accentAlpha(0x22), func(gtx layout.Context) layout.Dimensions {
										return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return fixedWidth(gtx, unit.Dp(20), func(gtx layout.Context) layout.Dimensions {
												return fixedHeight(gtx, unit.Dp(20), func(gtx layout.Context) layout.Dimensions {
													return uiIconSpark.Layout(gtx, fluent.accent)
												})
											})
										})
									})
								})
							})
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "先连上一个可用上游", unit.Sp(16), fluent.text, font.SemiBold)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "先保存一条可用的 API 中转配置，后面所有生成、编辑、提示词优化都会走这里。", unit.Sp(12), fluent.textMuted, font.Normal)
								}),
							)
						}),
					)
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !canSync {
				return layout.Dimensions{}
			}
			return a.elevatedSurfaceButton(
				gtx,
				&a.syncCodexConfigButton,
				fluent.surfaceElevated,
				withAlpha(fluent.accentSoft, 0xe2),
				accentAlpha(0x28),
				fluentControlRadius,
				image.Pt(0, 1),
				layout.Inset{Top: 12, Bottom: 12, Left: 14, Right: 14},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Spacing: layout.SpaceBetween}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, syncLabel, unit.Sp(13), fluent.accent, font.SemiBold)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "自动读取当前电脑里的 Codex base_url 和 OPENAI_API_KEY。", unit.Sp(11), withAlpha(fluent.accent, 0xd8), font.Normal)
								}),
							)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
									return uiIconRefresh.Layout(gtx, fluent.accent)
								})
							})
						}),
					)
				},
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.elevatedSurfaceButton(
						gtx,
						&a.createProfileButton,
						fluent.surfaceElevated,
						withAlpha(fluent.accentSoft, 0xd8),
						fluent.border,
						fluentCardRadius,
						image.Pt(0, 1),
						layout.Inset{Top: 14, Bottom: 14, Left: 14, Right: 14},
						func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.badge(gtx, "R", fluent.accentSoft, fluent.accent)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, "Responses API", unit.Sp(13), fluent.text, font.SemiBold)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "首选。支持 SSE 保活，长任务更稳。", unit.Sp(11), fluent.textMuted, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "适合 GPT 图像链路和提示词优化。", unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
												return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
													return uiIconAdd.Layout(gtx, fluent.accent)
												})
											})
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, "新建这类配置", unit.Sp(11), fluent.accent, font.Medium)
										}),
									)
								}),
							)
						},
					)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.elevatedSurfaceButton(
						gtx,
						&a.createImagesProfileButton,
						fluent.surfaceElevated,
						withAlpha(fluent.accentSoft, 0xd8),
						fluent.border,
						fluentCardRadius,
						image.Pt(0, 1),
						layout.Inset{Top: 14, Bottom: 14, Left: 14, Right: 14},
						func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.badge(gtx, "I", fluent.accentSoft, fluent.accent)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, "Images API", unit.Sp(13), fluent.text, font.SemiBold)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "兼容性更广，接标准 generations / edits。", unit.Sp(11), fluent.textMuted, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "适合只想尽快接上常规生图接口。", unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(5))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
												return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
													return uiIconAdd.Layout(gtx, fluent.accent)
												})
											})
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, "新建这类配置", unit.Sp(11), fluent.accent, font.Medium)
										}),
									)
								}),
							)
						},
					)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.borderedSurface(gtx, fluent.accentSoft, fluentControlRadius, accentAlpha(0x22), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "保存后会写入系统凭据存储。之后你可以继续新增多个上游配置，再按场景切换。", unit.Sp(10), fluent.accent, font.Normal)
				})
			})
		}),
	)
}

func (a *App) layoutPromptHelperButtons(gtx layout.Context, prefix string, items []promptHelperItem) layout.Dimensions {
	rows := make([]layout.FlexChild, 0, len(items))
	for _, item := range items {
		item := item
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				btn := a.promptButton(prefix + item.ID)
				return a.surfaceButton(
					gtx,
					btn,
					rgba(0xffffff, 0x00),
					fluent.accentSoft,
					rgba(0xffffff, 0x00),
					fluentControlRadius,
					layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, item.Title, unit.Sp(11), fluent.text, font.Medium)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if item.Detail == "" {
									return layout.Dimensions{}
								}
								return a.singleLineLabel(gtx, item.Detail, unit.Sp(10), fluent.textDim, font.Normal)
							}),
						)
					},
				)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func (a *App) layoutSettingsOptionCards(
	gtx layout.Context,
	title string,
	options []settingsOptionChoice,
	selected string,
	buttons []widget.Clickable,
	columns int,
	set func(string),
) layout.Dimensions {
	if columns <= 0 {
		columns = 2
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, title, unit.Sp(11), fluent.textMuted, font.SemiBold)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			rows := make([]layout.FlexChild, 0, (len(options)+columns-1)/columns)
			for row := 0; row < len(options); row += columns {
				row := row
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					cells := make([]layout.FlexChild, 0, columns)
					for col := 0; col < columns; col++ {
						idx := row + col
						if idx >= len(options) {
							cells = append(cells, layout.Flexed(1, layout.Spacer{}.Layout))
							continue
						}
						for buttons[idx].Clicked(gtx) {
							set(options[idx].Value)
						}
						cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							active := options[idx].Value == selected
							return a.surfaceButton(
								gtx,
								&buttons[idx],
								chooseColor(active, fluent.accentSoft, fluent.surface),
								chooseColor(active, accentAlpha(0x28), fluent.surface2),
								chooseColor(active, accentAlpha(0x24), fluent.border),
								unit.Dp(8),
								layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
								func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, options[idx].Title, unit.Sp(12), chooseColor(active, fluent.accent, fluent.text), font.SemiBold)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, options[idx].Detail, unit.Sp(10), chooseColor(active, withAlpha(fluent.accent, 0xcc), fluent.textDim), font.Normal)
										}),
									)
								},
							)
						}))
					}
					return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, cells...)
					})
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
		}),
	)
}

func (a *App) layoutSettingsAPIKeyField(gtx layout.Context) layout.Dimensions {
	icon := uiIconVisibility
	if a.apiKeyVisible {
		icon = uiIconVisibilityOff
	}
	border := fluent.border2
	if gtx.Focused(&a.apiKeyInput) {
		border = accentAlpha(0xb8)
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, "API Key", unit.Sp(11), fluent.textMuted, font.SemiBold)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, unit.Dp(40), func(gtx layout.Context) layout.Dimensions {
				return a.borderedSurface(gtx, fluent.surface, fluentInputRadius, border, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 8, Bottom: 8, Left: 12, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								style := material.Editor(a.th, &a.apiKeyInput, "sk-...")
								style.Color = fluent.text
								style.HintColor = fluent.textDim
								style.SelectionColor = accentAlpha(0x3d)
								style.TextSize = a.scaledSp(unit.Sp(13))
								style.Font.Weight = font.Medium
								style.Font.Typeface = uiMonoTypeface
								return style.Layout(gtx)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.ghostIconButton(gtx, &a.toggleAPIKeyMaskButton, icon, a.apiKeyVisible)
							}),
						)
					})
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, "API Key 保存在系统凭据存储中，不写入本地状态文件。", unit.Sp(10), fluent.textDim, font.Normal)
		}),
	)
}

func (a *App) layoutSettingsProfileRail(gtx layout.Context, snap snapshot) layout.Dimensions {
	canSync := canLoadCodexAPIConfig()
	for _, profile := range snap.Profiles {
		btn := a.settingsProfileButton("settings-profile:" + profile.ID)
		profile := profile
		for btn.Clicked(gtx) {
			if err := a.loadSettingsProfileDraft(profile.ID); err != nil {
				a.appendLog("读取配置失败: " + err.Error())
			}
		}
	}
	for a.duplicateProfileButton.Clicked(gtx) {
		if err := a.duplicateSettingsProfile(); err != nil {
			a.appendLog("复制配置失败: " + err.Error())
		}
	}
	for a.deleteProfileButton.Clicked(gtx) {
		if err := a.deleteSettingsProfile(); err != nil {
			a.appendLog("删除配置失败: " + err.Error())
		}
	}
	for a.settingsActivateProfileButton.Clicked(gtx) {
		if err := a.activateStoredProfile(snap.SettingsSelectedProfileID); err != nil {
			a.appendLog("切换上游失败: " + err.Error())
		}
	}
	activeName := strings.TrimSpace(activeProfileName(snap.Profiles, snap.ActiveProfileID))
	if activeName == "" {
		activeName = "未命名配置"
	}
	selectedID := strings.TrimSpace(snap.SettingsSelectedProfileID)
	if selectedID == "" {
		selectedID = strings.TrimSpace(snap.ActiveProfileID)
	}

	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedHeight(gtx, unit.Dp(460), func(gtx layout.Context) layout.Dimensions {
				return a.borderedSurface(gtx, fluent.surface, unit.Dp(10), fluent.border, func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						if len(snap.Profiles) == 0 {
							return layout.UniformInset(unit.Dp(6)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "还没有配置,点下方新建开始。", unit.Sp(10), fluent.textDim, font.Normal)
							})
						}
						return a.settingsProfileList.Layout(gtx, len(snap.Profiles), func(gtx layout.Context, index int) layout.Dimensions {
							profile := snap.Profiles[index]
							return layout.Inset{Bottom: unit.Dp(3)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								btn := a.settingsProfileButton("settings-profile:" + profile.ID)
								selected := profile.ID == selectedID
								active := profile.ID == snap.ActiveProfileID
								return a.surfaceButton(
									gtx,
									btn,
									chooseColor(selected, fluent.accentSoft, rgba(0xffffff, 0x00)),
									chooseColor(selected, accentAlpha(0x18), fluent.toolHoverBg),
									chooseColor(selected, accentAlpha(0x38), rgba(0xffffff, 0x00)),
									unit.Dp(8),
									layout.Inset{Top: 5, Bottom: 5, Left: 8, Right: 8},
									func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												dot := fluent.textDim
												if active {
													dot = fluent.accent
												} else if selected {
													dot = withAlpha(fluent.accent, 0xb8)
												}
												return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
													return fixedWidth(gtx, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
														return fixedHeight(gtx, unit.Dp(8), func(gtx layout.Context) layout.Dimensions {
															return a.surface(gtx, dot, unit.Dp(4), layout.Spacer{}.Layout)
														})
													})
												})
											}),
											layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
												return a.singleLineLabel(gtx, strings.TrimSpace(profile.Name), unit.Sp(12), chooseColor(selected, fluent.accent, fluent.text), font.Medium)
											}),
											layout.Rigid(func(gtx layout.Context) layout.Dimensions {
												modeTag := "R"
												if strings.TrimSpace(profile.APIMode) == "images" {
													modeTag = "I"
												}
												return a.metaBadge(gtx, modeTag, true)
											}),
										)
									},
								)
							})
						})
					})
				})
			})
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !canSync {
				return layout.Dimensions{}
			}
			label := "同步 Codex 配置"
			if snap.SyncingCodexConfig {
				label = "同步中..."
			}
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return a.compactIconTextButton(gtx, &a.syncCodexConfigButton, uiIconRefresh, label, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.exportUpstreamConfigsButton, uiIconSave, "导出", false)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.importUpstreamConfigsButton, uiIconFolder, "导入文件", false)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.compactIconTextButton(gtx, &a.openQuickImportUpstreamConfigsButton, uiIconEdit, "粘贴 JSON 快捷导入", false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.createProfileButton, uiIconAdd, "新建", false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.compactIconButton(gtx, &a.duplicateProfileButton, uiIconCopy, false)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.compactIconButton(gtx, &a.deleteProfileButton, uiIconDelete, false)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if selectedID == "" || selectedID == snap.ActiveProfileID {
				return layout.Dimensions{}
			}
			return a.compactButton(gtx, &a.settingsActivateProfileButton, "设为当前激活", true)
		}),
	)
}

func (a *App) layoutSettingsEditorPane(gtx layout.Context, snap snapshot) layout.Dimensions {
	selectedID := strings.TrimSpace(snap.SettingsSelectedProfileID)
	if selectedID == "" {
		selectedID = strings.TrimSpace(snap.ActiveProfileID)
	}
	selectedName := strings.TrimSpace(activeProfileName(snap.Profiles, selectedID))
	if selectedName == "" {
		selectedName = "未命名配置"
	}
	selectedMode := activeProfileAPIMode(snap.Profiles, selectedID)
	if selectedMode == "" {
		selectedMode = a.api
	}
	activeName := strings.TrimSpace(activeProfileName(snap.Profiles, snap.ActiveProfileID))
	if activeName == "" {
		activeName = "未命名配置"
	}
	activeMode := activeProfileAPIMode(snap.Profiles, snap.ActiveProfileID)
	if activeMode == "" {
		activeMode = a.api
	}
	fallbackCandidates := make([]sharedCompat.UpstreamProfile, 0, len(snap.Profiles))
	for _, profile := range snap.Profiles {
		if profile.ID == selectedID {
			continue
		}
		if strings.TrimSpace(profile.BaseURL) == "" {
			continue
		}
		fallbackCandidates = append(fallbackCandidates, profile)
	}
	connectionSection := func(gtx layout.Context) layout.Dimensions {
		rows := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.field(gtx, "名称", &a.profileNameInput, "配置1", unit.Dp(40))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutSettingsOptionCards(gtx, "API 形态", []settingsOptionChoice{
					{Title: "Responses API", Detail: "SSE 保活，更适合长推理", Value: string(client.APIModeResponses)},
					{Title: "Images API", Detail: "标准 generations / edits", Value: string(client.APIModeImages)},
				}, a.api, a.apiButtons, 2, func(value string) { a.api = value })
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				hint := "需要 key 绑定到拥有 gpt-5.5 模型的分组。SSE 保活可防 Cloudflare 524。"
				if a.api == string(client.APIModeImages) {
					hint = "使用标准 Images API，key 走 image-2 / image API 分组，兼容性最广。"
				}
				return a.label(gtx, hint, unit.Sp(10), fluent.textDim, font.Normal)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.api != string(client.APIModeResponses) {
					return layout.Dimensions{}
				}
				return a.layoutSettingsOptionCards(gtx, "Responses 传输", responsesTransportChoices, a.responsesTransport, a.responsesTransportButtons, 2, func(value string) {
					a.responsesTransport = normalizeProfileResponsesTransport(value)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if a.api != string(client.APIModeResponses) {
					return layout.Dimensions{}
				}
				return a.label(gtx, "WebSocket mode 需要上游 / 代理正确转发 Upgrade: websocket；若握手失败，请求层会自动回退到 HTTP SSE，并把原因写进 Raw response。", unit.Sp(10), fluent.textDim, font.Normal)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutSettingsOptionCards(gtx, "参数策略", []settingsOptionChoice{
					{Title: "OpenAI 标准", Detail: "只发送官方公开字段", Value: string(client.RequestPolicyOpenAI)},
					{Title: "兼容中转扩展", Detail: "附带 seed / negative_prompt", Value: string(client.RequestPolicyCompat)},
				}, a.policy, a.policyButtons, 1, func(value string) { a.policy = value })
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, "OpenAI 标准更适合直连 OpenAI；兼容中转扩展会额外发送 relay 常见扩展字段。", unit.Sp(10), fluent.textDim, font.Normal)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.technicalField(gtx, "上游 BASE_URL", &a.baseURLInput, "https://example.com", unit.Dp(40))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, "只填中转站根地址。应用会按当前 API 形态自动拼接 /v1/...，不要把完整接口路径手动贴进来。", unit.Sp(10), fluent.textDim, font.Normal)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutSettingsAPIKeyField(gtx)
			}),
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, rows...)
	}
	runtimeSection := func(gtx layout.Context) layout.Dimensions {
		rows := []layout.FlexChild{}
		canSave := strings.TrimSpace(a.baseURLInput.Text()) != "" && strings.TrimSpace(a.apiKeyInput.Text()) != ""
		probeTextModels, probeImageModels := preferredProbeModels(a.api, snap.LastProbeModels)
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			countLabel := "尚未加载"
			if len(snap.LastProbeModels) > 0 {
				countLabel = fmt.Sprintf("已识别 %d 个模型", len(snap.LastProbeModels))
			}
			if snap.TestingUpstream {
				countLabel = "拉取中..."
			}
			return a.helpInfoCard(gtx, "上游模型列表", "", func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, countLabel, unit.Sp(11), fluent.text, font.Medium)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "通过宿主侧请求 /v1/models 获取模型列表，避免浏览器跨域或 WebView 差异影响结果。", unit.Sp(10), fluent.textDim, font.Normal)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := "拉取并解析上游模型"
						if snap.TestingUpstream {
							label = "拉取中..."
						}
						return a.compactIconTextButton(gtx, &a.loadUpstreamModelsButton, uiIconRefresh, label, false)
					}),
				)
			})
		}))
		if a.api == string(client.APIModeResponses) {
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.technicalField(gtx, "文本模型 ID", &a.textModelInput, client.TextModel, unit.Dp(40))
			}))
			if len(probeTextModels) > 0 {
				rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutProbeModelSuggestions(gtx, "推荐文本模型", "text", probeTextModels, strings.TrimSpace(a.textModelInput.Text()), func(id string) {
						a.textModelInput.SetText(strings.TrimSpace(id))
					})
				}))
			}
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutSettingsOptionCards(gtx, "推理强度", reasoningEffortChoices, a.reasoningEffort, a.reasoningEffortButtons, 2, func(value string) {
					a.reasoningEffort = normalizeReasoningEffort(value)
				})
			}))
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, "仅 Responses API 生效，会写入 reasoning.effort。推理越高通常越稳，但延迟也会更长。", unit.Sp(10), fluent.textDim, font.Normal)
			}))
		}
		rows = append(rows,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.technicalField(gtx, "图像模型 ID", &a.imageModelInput, client.ImageModel, unit.Dp(40))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.technicalField(gtx, "并发数量限制", &a.concurrencyLimitInput, "留空 = 不限制", unit.Dp(40))
			}),
		)
		if len(probeImageModels) > 0 {
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutProbeModelSuggestions(gtx, "推荐图像模型", "image", probeImageModels, strings.TrimSpace(a.imageModelInput.Text()), func(id string) {
					a.imageModelInput.SetText(strings.TrimSpace(id))
				})
			}))
		}
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "失败重试路由到", unit.Sp(11), fluent.textMuted, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := a.settingsProfileButton("fallback:none")
					for btn.Clicked(gtx) {
						a.fallbackProfileID = ""
					}
					active := strings.TrimSpace(a.fallbackProfileID) == ""
					return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.surfaceButton(
							gtx,
							btn,
							chooseColor(active, fluent.accentSoft, fluent.surface),
							chooseColor(active, accentAlpha(0x18), fluent.surface2),
							chooseColor(active, accentAlpha(0x24), fluent.border),
							unit.Dp(8),
							layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
							func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, "不自动切备用上游", unit.Sp(12), chooseColor(active, fluent.accent, fluent.text), font.SemiBold)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, "主上游自动重试仍失败后，不再额外切备用配置。", unit.Sp(10), chooseColor(active, withAlpha(fluent.accent, 0xd0), fluent.textDim), font.Normal)
									}),
								)
							},
						)
					})
				}),
			}
			for _, profile := range fallbackCandidates {
				profile := profile
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := a.settingsProfileButton("fallback:" + profile.ID)
					for btn.Clicked(gtx) {
						a.fallbackProfileID = profile.ID
					}
					active := strings.TrimSpace(a.fallbackProfileID) == profile.ID
					return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.surfaceButton(
							gtx,
							btn,
							chooseColor(active, fluent.accentSoft, fluent.surface),
							chooseColor(active, accentAlpha(0x18), fluent.surface2),
							chooseColor(active, accentAlpha(0x24), fluent.border),
							unit.Dp(8),
							layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
							func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, strings.TrimSpace(profile.Name)+" · "+apiChoiceLabel(strings.TrimSpace(profile.APIMode)), unit.Sp(12), chooseColor(active, fluent.accent, fluent.text), font.SemiBold)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, strings.TrimSpace(profile.BaseURL), unit.Sp(10), chooseColor(active, withAlpha(fluent.accent, 0xd0), fluent.textDim), font.Normal)
									}),
								)
							},
						)
					})
				}))
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}))
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, "当前上游自动重试仍失败后，可额外切到这里选定的备用 profile 再尝试一次。默认关闭。", unit.Sp(10), fluent.textDim, font.Normal)
		}))
		if a.api == string(client.APIModeImages) {
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				active := a.imagesNewAPICompat
				return a.elevatedSurfaceButton(
					gtx,
					&a.settingsImagesCompatButton,
					chooseColor(active, fluent.accentSoft, fluent.surfaceElevated),
					chooseColor(active, accentAlpha(0x18), fluent.surface2),
					chooseColor(active, accentAlpha(0x24), fluent.border),
					unit.Dp(8),
					image.Pt(0, 1),
					layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12},
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, "Images API 中转兼容", unit.Sp(12), chooseColor(active, fluent.accent, fluent.text), font.SemiBold)
									}),
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, "开启后会强制发送 response_format=b64_json，并关闭 stream / partial_images，更适合部分 NewAPI 风格中转站。", unit.Sp(10), chooseColor(active, withAlpha(fluent.accent, 0xd0), fluent.textDim), font.Normal)
									}),
								)
							}),
							layout.Rigid(layout.Spacer{Width: unit.Dp(10)}.Layout),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								text := "已关闭"
								bg := withAlpha(fluent.textDim, 0x24)
								fg := fluent.textDim
								if active {
									text = "已开启"
									bg = fluent.accentSoft
									fg = fluent.accent
								}
								return a.badge(gtx, text, bg, fg)
							}),
						)
					},
				)
			}))
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, "默认关闭，保持 OpenAI 标准 Images API 请求。只有默认标准参数用不了时，再尝试开启。", unit.Sp(10), fluent.textDim, font.Normal)
			}))
		}
		rows = append(rows,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.segmentedWithTitle(gtx, "代理", proxyChoices, a.proxy, a.proxyButtons, func(value string) { a.proxy = value })
			}),
		)
		if a.proxy == "custom" || strings.TrimSpace(a.proxyURLInput.Text()) != "" {
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.technicalField(gtx, "自定义代理 URL", &a.proxyURLInput, "http://127.0.0.1:7890", unit.Dp(40))
			}))
		}
		rows = append(rows,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.technicalField(gtx, "输出目录", &a.outputDirInput, "生成图片保存目录", unit.Dp(40))
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := "保存并测试连接"
				if snap.TestingUpstream {
					label = "测试中..."
				}
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				if !canSave {
					return a.surfaceButton(
						gtx,
						&a.settingsTestUpstreamButton,
						fluent.surface2,
						fluent.surface2,
						rgba(0xffffff, 0x00),
						fluentControlRadius,
						layout.Inset{Top: 9, Bottom: 9, Left: 12, Right: 12},
						func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, label, unit.Sp(12), withAlpha(fluent.textDim, 0x88), font.SemiBold)
							})
						},
					)
				}
				return a.primaryButton(gtx, &a.settingsTestUpstreamButton, label, fluent.accent, fluent.white)
			}),
		)
		if strings.TrimSpace(snap.LastProbeSummary) != "" {
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, snap.LastProbeSummary, unit.Sp(10), fluent.textDim, font.Normal)
			}))
		}
		if a.api == string(client.APIModeImages) {
			rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.borderedSurface(gtx, fluent.accentSoft, unit.Dp(10), accentAlpha(0x22), func(gtx layout.Context) layout.Dimensions {
					return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
										return uiIconInfo.Layout(gtx, fluent.accent)
									})
								})
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "Images API 路径走标准 /v1/images/generations 与 /v1/images/edits，没有 SSE 保活，长推理 CF 524 风险更高，但兼容性最广。", unit.Sp(10), fluent.accent, font.Normal)
							}),
						)
					})
				})
			}))
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, rows...)
	}
	header := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				title := selectedName + " · " + apiChoiceLabel(selectedMode)
				if selectedID != snap.ActiveProfileID {
					title += " · 当前生效: " + activeName
				}
				return a.singleLineLabel(gtx, title, unit.Sp(11), fluent.textMuted, font.Medium)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.compactIconTextButton(gtx, &a.settingsHelpButton, uiIconInfo, "接口说明", true)
			}),
		)
	}
	sections := []layout.Widget{
		connectionSection,
		runtimeSection,
	}
	footer := func(gtx layout.Context) layout.Dimensions {
		canSave := a.settingsDraftReady()
		saveBtn := func(gtx layout.Context) layout.Dimensions {
			if !canSave {
				return a.surfaceButton(
					gtx,
					&a.saveSettingsButton,
					fluent.surface2,
					fluent.surface2,
					rgba(0xffffff, 0x00),
					fluentControlRadius,
					layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
					func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, unit.Dp(16), func(gtx layout.Context) layout.Dimensions {
										return uiIconSave.Layout(gtx, withAlpha(fluent.textDim, 0x88))
									})
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "保存", unit.Sp(12), withAlpha(fluent.textDim, 0x88), font.Medium)
							}),
						)
					},
				)
			}
			return a.primaryButton(gtx, &a.saveSettingsButton, "保存", fluent.accent, fluent.white)
		}
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(96), func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.closeSettingsButton, "关闭", false)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(110), saveBtn)
						}),
					)
				})
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if canSave {
					return layout.Dimensions{}
				}
				return a.label(gtx, "BASE_URL 和 API Key 必须填齐才能保存。", unit.Sp(10), fluent.textDim, font.Normal)
			}),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(header),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.settingsList.Layout(gtx, len(sections), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, sections[index])
			})
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(footer),
	)
}

func (a *App) composeSummary(snap snapshot) string {
	activeAspect := deriveAspectPreset(a.size, a.customAspectRatios)
	activeResolution := normalizeResolutionChoice(deriveResolutionPreset(a.size), a.api, a.policy, a.imageModelInput.Text())
	currentSaved := strings.TrimSpace(snap.Result.SavedPath)
	sourcePaths := a.sourcePaths()
	sourceLabel := "文生图"
	if a.mode == string(client.ModeEdit) {
		count := len(sourcePaths)
		if count > 0 {
			sourceLabel = fmt.Sprintf("%d 张源图", count)
		} else if currentSaved != "" {
			sourceLabel = "画板图作源图"
		} else {
			sourceLabel = "未添加源图"
		}
	}
	runModeLabel := ""
	if a.batchMode {
		batchSources := a.batchSourcePaths()
		runModeLabel = fmt.Sprintf("批处理 %d 张", len(batchSources))
	} else if a.loopEnabled {
		runModeLabel = fmt.Sprintf("循环 %d 张 / 并发 %d", normalizeLoopGenerationCount(a.loopTotalCount), normalizeLoopGenerationConcurrency(a.loopConcurrency))
	}
	key := strings.Join([]string{
		a.styleTag,
		activeAspect,
		activeResolution,
		a.quality,
		strconv.Itoa(normalizeBatchCount(a.batchCount)),
		a.mode,
		runModeLabel,
		strconv.Itoa(len(sourcePaths)),
		currentSaved,
	}, "\x00")
	a.mu.Lock()
	if a.composeSummaryCacheKey == key {
		summary := a.composeSummaryCache
		a.mu.Unlock()
		return summary
	}
	a.mu.Unlock()

	summary := strings.Join(compactNonEmpty([]string{
		chooseStyleSummary(a.styleTag),
		aspectChoiceLabel(activeAspect),
		choiceLabel(resolutionChoices, activeResolution),
		qualityChoiceLabel(a.quality),
		fmt.Sprintf("%d 张", normalizeBatchCount(a.batchCount)),
		runModeLabel,
		sourceLabel,
	}), " · ")
	a.mu.Lock()
	a.composeSummaryCacheKey = key
	a.composeSummaryCache = summary
	a.mu.Unlock()
	return summary
}

func (a *App) layoutComposeCard(gtx layout.Context, snap snapshot) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingComposeCard, time.Now())
	activeAspect := deriveAspectPreset(a.size, a.customAspectRatios)
	activeResolution := normalizeResolutionChoice(deriveResolutionPreset(a.size), a.api, a.policy, a.imageModelInput.Text())
	currentSaved := strings.TrimSpace(snap.Result.SavedPath)
	sourcePaths := a.sourcePaths()
	summary := a.composeSummary(snap)

	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutDisclosureHeader(gtx, &a.composeToggleButton, "创作参数", summary, a.composeOpen)
				}),
			}
			if a.composeOpen {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.composeSectionCard(gtx, a.layoutStyleSection)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.composeSectionCard(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.layoutAspectSection(gtx, activeAspect, activeResolution)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.composeSectionCard(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.layoutResolutionSection(gtx, activeAspect, activeResolution)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.composeSectionCard(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "质量", unit.Sp(11), fluent.textMuted, font.SemiBold)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.segmented(gtx, qualityChoices, a.quality, a.qualityButtons, func(value string) { a.quality = value })
								}),
							)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.composeSectionCard(gtx, a.layoutBatchCountSection)
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.composeSectionCard(gtx, a.layoutLoopSection)
					}),
				)
				if a.mode == string(client.ModeEdit) {
					children = append(children,
						layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.composeSectionCard(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.layoutSourceInputSection(gtx, sourcePaths, currentSaved)
							})
						}),
					)
				}
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (a *App) composeSectionCard(gtx layout.Context, body layout.Widget) layout.Dimensions {
	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, unit.Dp(8), fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(10)).Layout(gtx, body)
	})
}

func (a *App) layoutAspectSection(gtx layout.Context, activeAspect string, currentResolution string) layout.Dimensions {
	children := make([]layout.FlexChild, 0, 2)
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return a.label(gtx, "比例", unit.Sp(11), fluent.textMuted, font.SemiBold)
	}))
	children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout))

	choices := aspectChoicesWithCustom(a.customAspectRatios)
	rows := (len(choices) + 2) / 3
	cols := 3
	for row := 0; row < rows; row++ {
		row := row
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			cells := make([]layout.FlexChild, 0, cols)
			for col := 0; col < cols; col++ {
				idx := row*cols + col
				if idx >= len(choices) {
					cells = append(cells, layout.Flexed(1, layout.Spacer{}.Layout))
					continue
				}
				choice := choices[idx]
				for a.aspectButtons[idx].Clicked(gtx) {
					next := buildAspectSizeSelection(choice.Value, currentResolution, a.api, a.policy, a.imageModelInput.Text())
					if strings.HasPrefix(choice.Value, "custom:") {
						for _, ratio := range a.customAspectRatios {
							if "custom:"+ratio.ID == choice.Value {
								next = buildCustomSizeSelection(ratio, currentResolution)
								break
							}
						}
					}
					a.size = next
				}
				cells = append(cells, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.layoutAspectButton(gtx, &a.aspectButtons[idx], choice, activeAspect == choice.Value)
				}))
			}
			return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, cells...)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) layoutStyleSection(gtx layout.Context) layout.Dimensions {
	for a.clearStyleButton.Clicked(gtx) {
		a.styleTag = ""
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "风格", unit.Sp(11), fluent.textMuted, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if a.styleTag == "" {
						return layout.Dimensions{}
					}
					return a.textActionButton(gtx, &a.clearStyleButton, "清除", true)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
	}
	rows := [][]choice{
		styleChoices[:3],
		styleChoices[3:],
	}
	base := 0
	for _, rowChoices := range rows {
		rowChoices := rowChoices
		rowStart := base
		base += len(rowChoices)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := make([]layout.FlexChild, 0, len(rowChoices))
			for idx := range rowChoices {
				choice := rowChoices[idx]
				btnIdx := rowStart + idx
				row = append(row, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					btn := &a.styleButtons[btnIdx]
					for btn.Clicked(gtx) {
						if a.styleTag == choice.Value {
							a.styleTag = ""
						} else {
							a.styleTag = choice.Value
						}
					}
					return layout.Inset{Right: unit.Dp(8), Bottom: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return a.surfaceButton(
							gtx,
							btn,
							chooseColor(a.styleTag == choice.Value, fluent.accentSoft, rgba(0xffffff, 0x00)),
							chooseColor(a.styleTag == choice.Value, accentAlpha(0x28), fluent.surface2),
							chooseColor(a.styleTag == choice.Value, accentAlpha(0x28), fluent.border),
							fluentControlRadius,
							layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10},
							func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, choice.Label, unit.Sp(11), chooseColor(a.styleTag == choice.Value, fluent.accent, fluent.textMuted), font.Medium)
							},
						)
					})
				}))
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, row...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) layoutSourceInputSection(gtx layout.Context, sourcePaths []string, currentSaved string) layout.Dimensions {
	for a.composeSourceModeButtons[0].Clicked(gtx) {
		a.batchMode = false
	}
	for a.composeSourceModeButtons[1].Clicked(gtx) {
		a.batchMode = true
	}
	modeRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.composeSourceModeButtons[0], "普通图生图", !a.batchMode)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.composeSourceModeButtons[1], "批处理", a.batchMode)
		}),
	}
	if a.batchMode {
		return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, modeRows...)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.layoutBatchSourceQueueSection(gtx, sourcePaths)
			}),
		)
	}
	for a.addSourceFilesButton.Clicked(gtx) {
		paths, err := chooseImageFiles()
		if err != nil {
			a.appendLog("选择源图失败: " + err.Error())
		} else {
			for _, path := range paths {
				a.appendSourcePath(path)
			}
		}
	}
	for a.clearSourcesButton.Clicked(gtx) {
		a.setSourcePaths(nil)
	}
	for _, path := range sourcePaths {
		path := path
		btn := a.sourceButton("panel-remove:" + path)
		for btn.Clicked(gtx) {
			a.removeSourcePath(path)
		}
	}

	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, modeRows...)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "源图片 / 参考图", unit.Sp(11), fluent.textMuted, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if len(sourcePaths) == 0 {
						return layout.Dimensions{}
					}
					return a.metaBadge(gtx, fmt.Sprintf("%d 张", len(sourcePaths)), true)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
	}

	if len(sourcePaths) == 0 && currentSaved != "" {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.borderedSurface(gtx, fluent.surface2, fluentControlRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.singleLineLabel(gtx, "(画板当前图 · 隐式源图)", unit.Sp(10), fluent.textDim, font.Normal)
				})
			})
		}))
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
	}

	for _, path := range sourcePaths {
		path := path
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			btn := a.sourceButton("panel-remove:" + path)
			return a.borderedSurface(gtx, fluent.surface, fluentControlRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							idx := indexOfSourcePath(path, sourcePaths) + 1
							if idx <= 0 {
								return layout.Dimensions{}
							}
							return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, strconv.Itoa(idx)+".", unit.Sp(10), fluent.textDim, font.Medium)
							})
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.singleLineLabel(gtx, filepath.Base(path), unit.Sp(11), fluent.text, font.Medium)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.ghostIconButton(gtx, btn, uiIconClose, false)
						}),
					)
				})
			})
		}))
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
	}

	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		row := []layout.FlexChild{}
		row = append(row, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactIconTextButton(gtx, &a.addSourceFilesButton, uiIconAdd, "添加图片", false)
		}))
		if len(sourcePaths) > 0 {
			row = append(row, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.compactIconButton(gtx, &a.clearSourcesButton, uiIconDelete, false)
				})
			}))
		}
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, row...)
	}))

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) layoutBatchSourceQueueSection(gtx layout.Context, sourcePaths []string) layout.Dimensions {
	scannedPaths, scanErr := a.batchSourcePathsForRun()
	displayPaths := sourcePaths
	if len(displayPaths) == 0 && strings.TrimSpace(a.batchInputDirInput.Text()) != "" {
		displayPaths = scannedPaths
	}
	for a.chooseBatchInputDirButton.Clicked(gtx) {
		dir, err := chooseDirectory()
		if err != nil {
			a.appendLog("选择批处理输入目录失败: " + err.Error())
		} else if strings.TrimSpace(dir) != "" {
			a.batchInputDirInput.SetText(dir)
			a.sourcePathsInput.SetText("")
			a.sourceButtons = map[string]*widget.Clickable{}
		}
	}
	for a.chooseBatchFilesButton.Clicked(gtx) {
		paths, err := chooseImageFiles()
		if err != nil {
			a.appendLog("选择批处理图片失败: " + err.Error())
		} else if len(paths) > 0 {
			a.batchInputDirInput.SetText("")
			a.sourcePathsInput.SetText(strings.Join(paths, "\n"))
			a.sourceButtons = map[string]*widget.Clickable{}
		}
	}
	for a.chooseBatchOutputDirButton.Clicked(gtx) {
		dir, err := chooseDirectory()
		if err != nil {
			a.appendLog("选择批处理输出目录失败: " + err.Error())
		} else if strings.TrimSpace(dir) != "" {
			a.batchOutputDirInput.SetText(dir)
			a.batchOutputDir = dir
		}
	}
	for a.addSourceFilesButton.Clicked(gtx) {
		paths, err := chooseImageFiles()
		if err != nil {
			a.appendLog("选择批处理图片失败: " + err.Error())
		} else {
			for _, path := range paths {
				a.appendSourcePath(path)
			}
		}
	}
	for a.clearSourcesButton.Clicked(gtx) {
		a.setSourcePaths(nil)
	}
	for _, path := range sourcePaths {
		path := path
		btn := a.sourceButton("batch-remove:" + path)
		for btn.Clicked(gtx) {
			a.removeSourcePath(path)
		}
	}

	queueLabel := fmt.Sprintf("当前队列 %d 张", len(displayPaths))
	if strings.TrimSpace(a.batchInputDirInput.Text()) != "" && len(sourcePaths) == 0 {
		queueLabel = fmt.Sprintf("%s · 目录模式", queueLabel)
	}
	if len(sourcePaths) > 0 {
		queueLabel = fmt.Sprintf("%s · 手动队列", fmt.Sprintf("当前队列 %d 张", len(sourcePaths)))
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "批处理队列", unit.Sp(11), fluent.textMuted, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.metaBadge(gtx, queueLabel, true)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.chooseBatchInputDirButton, uiIconFolder, "选目录", false)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.chooseBatchFilesButton, uiIconAdd, "选多图", false)
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.compactIconTextButton(gtx, &a.chooseBatchOutputDirButton, uiIconFolder, "输出目录", false)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
	}
	sourceModeLabel := "直接多图"
	if strings.TrimSpace(a.batchInputDirInput.Text()) != "" && len(sourcePaths) == 0 {
		sourceModeLabel = "目录扫描"
	}
	if len(displayPaths) > 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, "当前来源: "+sourceModeLabel, unit.Sp(10), fluent.textDim, font.Normal)
		}))
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout))
	}

	if scanErr != nil {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.borderedSurface(gtx, dangerAlpha(0x12), fluentControlRadius, dangerAlpha(0x2a), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "批处理目录读取失败: "+scanErr.Error(), unit.Sp(10), fluent.danger, font.Normal)
				})
			})
		}))
	} else if len(displayPaths) == 0 {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.borderedSurface(gtx, fluent.surface2, fluentControlRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "可直接加入多张图片，或在设置里选择输入目录后按目录扫描。", unit.Sp(10), fluent.textDim, font.Normal)
				})
			})
		}))
	} else {
		limit := len(displayPaths)
		if limit > 6 {
			limit = 6
		}
		for idx := 0; idx < limit; idx++ {
			path := displayPaths[idx]
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				btn := a.sourceButton("batch-remove:" + path)
				removeEnabled := len(sourcePaths) > 0
				return a.borderedSurface(gtx, fluent.surface, fluentControlRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Right: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, strconv.Itoa(idx+1)+".", unit.Sp(10), fluent.textDim, font.Medium)
								})
							}),
							layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
								return a.singleLineLabel(gtx, filepath.Base(path), unit.Sp(11), fluent.text, font.Medium)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								if !removeEnabled {
									return layout.Dimensions{}
								}
								return a.ghostIconButton(gtx, btn, uiIconClose, false)
							}),
						)
					})
				})
			}))
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
		}
		if len(displayPaths) > limit {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, fmt.Sprintf("还有 %d 张未展开显示。", len(displayPaths)-limit), unit.Sp(10), fluent.textDim, font.Normal)
			}))
			children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
		}
	}

	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		row := []layout.FlexChild{
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.compactIconTextButton(gtx, &a.addSourceFilesButton, uiIconAdd, "继续加入图片", false)
			}),
		}
		if len(sourcePaths) > 0 {
			row = append(row, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return a.compactIconButton(gtx, &a.clearSourcesButton, uiIconDelete, false)
				})
			}))
		}
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, row...)
	}))

	children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		retryRows := []layout.FlexChild{
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.compactButton(gtx, &a.composeBatchRetryButtons[0], "自动重试", a.batchRetryOnFail)
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.compactButton(gtx, &a.composeBatchRetryButtons[1], "失败跳过", !a.batchRetryOnFail)
			}),
		}
		for a.composeBatchRetryButtons[0].Clicked(gtx) {
			a.batchRetryOnFail = true
		}
		for a.composeBatchRetryButtons[1].Clicked(gtx) {
			a.batchRetryOnFail = false
		}
		return a.borderedSurface(gtx, fluent.surface2, fluentControlRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "失败处理", unit.Sp(11), fluent.textMuted, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, retryRows...)
					}),
				)
			})
		})
	}))

	children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		autoAspectRows := []layout.FlexChild{
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.compactButton(gtx, &a.composeBatchAutoAspectButtons[0], "沿用当前比例", strings.TrimSpace(a.batchAutoAspect) == "")
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return a.compactButton(gtx, &a.composeBatchAutoAspectButtons[1], "按源图比例", strings.TrimSpace(a.batchAutoAspect) != "")
			}),
		}
		for a.composeBatchAutoAspectButtons[0].Clicked(gtx) {
			a.batchAutoAspect = ""
		}
		for a.composeBatchAutoAspectButtons[1].Clicked(gtx) {
			if strings.TrimSpace(a.batchAutoAspect) == "" {
				a.batchAutoAspect = "1k"
			}
		}
		cardChildren := []layout.FlexChild{
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, "尺寸策略", unit.Sp(11), fluent.textMuted, font.SemiBold)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, autoAspectRows...)
			}),
		}
		if strings.TrimSpace(a.batchAutoAspect) != "" {
			cardChildren = append(cardChildren, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				rows := make([]layout.FlexChild, 0, 5)
				for idx, value := range []string{"256", "512", "1k", "2k", "4k"} {
					idx := idx
					value := value
					for a.composeBatchAutoAspectResolutionButtons[idx].Clicked(gtx) {
						a.batchAutoAspect = value
					}
					rows = append(rows, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return a.compactButton(gtx, &a.composeBatchAutoAspectResolutionButtons[idx], strings.ToUpper(value), a.batchAutoAspect == value)
					}))
				}
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, rows...)
			}))
		}
		return a.borderedSurface(gtx, fluent.surface2, fluentControlRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, cardChildren...)
			})
		})
	}))

	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func indexOfSourcePath(path string, sourcePaths []string) int {
	for idx, value := range sourcePaths {
		if value == path {
			return idx
		}
	}
	return -1
}

func (a *App) layoutResolutionSection(gtx layout.Context, activeAspect string, activeResolution string) layout.Dimensions {
	choices := visibleResolutionChoices(a.api, a.policy, a.imageModelInput.Text())
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, "分辨率", unit.Sp(11), fluent.textMuted, font.SemiBold)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return a.segmented(gtx, choices, activeResolution, a.resolutionButtons, func(value string) {
			a.size = buildResolutionSizeSelection(activeAspect, value, a.api, a.policy, a.imageModelInput.Text())
		})
	}))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		hint := sizeCapabilityHint(a.api, a.policy, a.imageModelInput.Text())
		if hint == "" {
			return layout.Dimensions{}
		}
		return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, hint, unit.Sp(10), fluent.textDim, font.Normal)
		})
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) layoutBatchCountSection(gtx layout.Context) layout.Dimensions {
	batchCount := normalizeBatchCount(a.batchCount)
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "出图张数", unit.Sp(11), fluent.textMuted, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.metaBadge(gtx, fmt.Sprintf("%dx", batchCount), true)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: unit.Dp(6)}.Layout),
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return a.segmentedGrid(gtx, batchCountChoices, strconv.Itoa(batchCount), a.batchCountButtons, 3, func(value string) {
			n, _ := strconv.Atoi(value)
			a.batchCount = normalizeBatchCount(n)
		})
	}))
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: unit.Dp(2)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, "多张会并行请求, 完成后在画板按网格挑图; 受上游并发限制约束。", unit.Sp(10), fluent.textDim, font.Normal)
		})
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) layoutLoopSection(gtx layout.Context) layout.Dimensions {
	for a.composeLoopButtons[0].Clicked(gtx) {
		a.loopEnabled = true
	}
	for a.composeLoopButtons[1].Clicked(gtx) {
		a.loopEnabled = false
	}
	for a.composeLoopAutoSaveButtons[0].Clicked(gtx) {
		a.setLoopAutoSaveEnabled(true)
	}
	for a.composeLoopAutoSaveButtons[1].Clicked(gtx) {
		a.setLoopAutoSaveEnabled(false)
	}
	for a.composeLoopPreviewButtons[0].Clicked(gtx) {
		a.loopLivePreview = true
	}
	for a.composeLoopPreviewButtons[1].Clicked(gtx) {
		a.loopLivePreview = false
	}
	for a.chooseLoopAutoSaveDirButton.Clicked(gtx) {
		a.chooseLoopAutoSaveDir("选择循环自动另存为目录失败: ")
	}
	for idx, value := range []int{4, 8, 10, 20, 50} {
		value := value
		for a.composeLoopCountButtons[idx].Clicked(gtx) {
			a.setLoopTotalCount(value)
		}
	}
	for idx, value := range []int{1, 2, 4, 8} {
		value := value
		for a.composeLoopConcurrencyButtons[idx].Clicked(gtx) {
			a.setLoopConcurrency(value)
		}
	}
	a.syncLoopSettingsFromInputs()

	loopRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.composeLoopButtons[0], "开启", a.loopEnabled)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.composeLoopButtons[1], "关闭", !a.loopEnabled)
		}),
	}
	autoSaveRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.composeLoopAutoSaveButtons[0], "自动另存为", a.loopAutoSave)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.composeLoopAutoSaveButtons[1], "不自动保存", !a.loopAutoSave)
		}),
	}
	previewRows := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.composeLoopPreviewButtons[0], "实时预览开", a.loopLivePreview)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.composeLoopPreviewButtons[1], "实时预览关", !a.loopLivePreview)
		}),
	}
	countRows := make([]layout.FlexChild, 0, 5)
	for idx, value := range []int{4, 8, 10, 20, 50} {
		idx := idx
		value := value
		countRows = append(countRows, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.composeLoopCountButtons[idx], fmt.Sprintf("%d 张", value), normalizeLoopGenerationCount(a.loopTotalCount) == value)
		}))
	}
	concurrencyRows := make([]layout.FlexChild, 0, 4)
	for idx, value := range []int{1, 2, 4, 8} {
		idx := idx
		value := value
		concurrencyRows = append(concurrencyRows, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.compactButton(gtx, &a.composeLoopConcurrencyButtons[idx], fmt.Sprintf("%d 并发", value), normalizeLoopGenerationConcurrency(a.loopConcurrency) == value)
		}))
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "循环出图", unit.Sp(11), fluent.textMuted, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					stateLabel := "关闭"
					if a.loopEnabled {
						stateLabel = fmt.Sprintf("%d 张 / %d 并发", normalizeLoopGenerationCount(a.loopTotalCount), normalizeLoopGenerationConcurrency(a.loopConcurrency))
					}
					return a.metaBadge(gtx, stateLabel, true)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, loopRows...)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, countRows...)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, concurrencyRows...)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, autoSaveRows...)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, previewRows...)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.technicalField(gtx, "自动另存为目录", &a.loopAutoSaveDirInput, "留空 = 关闭自动另存为", unit.Dp(42))
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.compactIconTextButton(gtx, &a.chooseLoopAutoSaveDirButton, uiIconFolder, "选择自动另存为目录", false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, "开启后提交按钮会按这里的总张数和并发持续补位生成。自动另存为与实时预览开关都已接到执行路径。", unit.Sp(10), fluent.textDim, font.Normal)
		}),
	)
}

func (a *App) advancedSummary() string {
	partialPreview := strings.TrimSpace(a.partialImagesInput.Text())
	if partialPreview == "" {
		partialPreview = strconv.Itoa(kernel.DefaultConfig().PartialImages)
	}
	partialPreviewSummary := partialPreview + " 帧"
	if partialPreview == "0" {
		partialPreviewSummary = "仅最终图"
	}
	key := strings.Join([]string{
		a.negativePromptInput.Text(),
		a.format,
		a.background,
		a.outputCompressionInput.Text(),
		a.inputFidelity,
		a.imageStyle,
		a.moderation,
		partialPreview,
		strconv.FormatBool(a.protectStreamPreview),
		a.userIdentifierInput.Text(),
		a.seedInput.Text(),
	}, "\x00")
	a.mu.Lock()
	if a.advancedSummaryCacheKey == key {
		summary := a.advancedSummaryCache
		a.mu.Unlock()
		return summary
	}
	a.mu.Unlock()

	summary := strings.Join(compactNonEmpty([]string{
		negativePromptSummary(a.negativePromptInput.Text()),
		strings.ToUpper(strings.TrimSpace(a.format)),
		"背景 " + backgroundChoiceLabel(a.background),
		chooseOptionalCompressionSummary(a.outputCompressionInput.Text(), a.format),
		chooseOptionalFidelitySummary(a.inputFidelity),
		chooseOptionalImageStyleSummary(a.imageStyle),
		"审核 " + moderationChoiceLabel(a.moderation),
		"预览 " + partialPreviewSummary,
		protectStreamPreviewSummary(a.protectStreamPreview),
		chooseOptionalUserIdentifierSummary(a.userIdentifierInput.Text()),
		seedSummary(a.seedInput.Text()),
	}), " · ")
	a.mu.Lock()
	a.advancedSummaryCacheKey = key
	a.advancedSummaryCache = summary
	a.mu.Unlock()
	return summary
}

func (a *App) layoutAdvancedCard(gtx layout.Context) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingAdvancedCard, time.Now())
	for a.copyPerformanceDiagnosticsButton.Clicked(gtx) {
		copyResultDetailText(gtx, a.buildPerformanceDiagnosticsReport())
		a.appendLog("已复制性能诊断")
	}
	for a.protectStreamPreviewButtons[0].Clicked(gtx) {
		a.protectStreamPreview = true
	}
	for a.protectStreamPreviewButtons[1].Clicked(gtx) {
		a.protectStreamPreview = false
	}
	summary := a.advancedSummary()

	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			children := []layout.FlexChild{
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.layoutDisclosureHeader(gtx, &a.advancedToggleButton, "高级参数", summary, a.advancedOpen)
				}),
			}
			if a.advancedOpen {
				children = append(children,
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.advancedSectionCard(gtx, "负向提示词", "", func(gtx layout.Context) layout.Dimensions {
							border := fluent.border2
							if gtx.Focused(&a.negativePromptInput) {
								border = accentAlpha(0xb8)
							}
							return fixedHeight(gtx, unit.Dp(96), func(gtx layout.Context) layout.Dimensions {
								return a.borderedSurface(gtx, fluent.surface, fluentControlRadius, border, func(gtx layout.Context) layout.Dimensions {
									return layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
										return a.editorText(gtx, &a.negativePromptInput, "例如：不要文字、不要水印、不要多余肢体、不要过曝", unit.Sp(13))
									})
								})
							})
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.advancedSectionCard(gtx, "输出格式", "", func(gtx layout.Context) layout.Dimensions {
							return a.segmented(gtx, formatChoices, a.format, a.formatButtons, func(value string) { a.format = value })
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.advancedSectionCard(gtx, "背景", "", func(gtx layout.Context) layout.Dimensions {
							return a.segmented(gtx, backgroundChoices, a.background, a.backgroundButtons, func(value string) { a.background = value })
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.advancedSectionCard(gtx, "输出压缩", "", func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.field(gtx, "0-100", &a.outputCompressionInput, strconv.Itoa(client.DefaultOutputCompression), unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "JPEG / WebP 会读取该值；PNG 会忽略它。", unit.Sp(10), fluent.textDim, font.Normal)
								}),
							)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.advancedSectionCard(gtx, "输入保真", "", func(gtx layout.Context) layout.Dimensions {
							return a.segmented(gtx, inputFidelityChoices, a.inputFidelity, a.inputFidelityButtons, func(value string) { a.inputFidelity = value })
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.advancedSectionCard(gtx, "图像风格", "", func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.segmented(gtx, imageStyleChoices, a.imageStyle, a.imageStyleButtons, func(value string) { a.imageStyle = value })
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "仅 dall-e-3 文生图支持；默认值会省略该字段。", unit.Sp(10), fluent.textDim, font.Normal)
								}),
							)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.advancedSectionCard(gtx, "内容审核", "", func(gtx layout.Context) layout.Dimensions {
							return a.segmented(gtx, moderationChoices, a.moderation, a.moderationButtons, func(value string) { a.moderation = value })
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.advancedSectionCard(gtx, "稳定用户标识", "", func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.technicalField(gtx, "User", &a.userIdentifierInput, "留空 = 不发送", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "Responses API 会映射到 safety_identifier；Images API 会映射到 user。", unit.Sp(10), fluent.textDim, font.Normal)
								}),
							)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.advancedSectionCard(gtx, "流式预览帧数", "", func(gtx layout.Context) layout.Dimensions {
							selected := strings.TrimSpace(a.partialImagesInput.Text())
							if selected == "" {
								selected = strconv.Itoa(kernel.DefaultConfig().PartialImages)
							}
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.segmented(gtx, partialPreviewChoices, selected, a.partialPreviewButtons, func(value string) { a.partialImagesInput.SetText(value) })
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "Gio 默认仅请求最终图；打开预览帧会增加上游响应体积和界面刷新开销。", unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.compactButton(gtx, &a.protectStreamPreviewButtons[0], "保护开启", a.protectStreamPreview)
										}),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.compactButton(gtx, &a.protectStreamPreviewButtons[1], "保护关闭", !a.protectStreamPreview)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "默认开启。桌面端高并发任务时会自动关闭流式预览；关闭后严格按上面的预览帧数请求。", unit.Sp(10), fluent.textDim, font.Normal)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.compactIconTextButton(gtx, &a.copyPerformanceDiagnosticsButton, uiIconCopy, "复制性能诊断", false)
								}),
							)
						})
					}),
					layout.Rigid(layout.Spacer{Height: unit.Dp(12)}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.advancedSectionCard(gtx, "随机种子", "", func(gtx layout.Context) layout.Dimensions {
							for a.randomSeedButton.Clicked(gtx) {
								a.seedInput.SetText(strconv.FormatInt(time.Now().UnixNano()%1000000007, 10))
							}
							for a.clearSeedButton.Clicked(gtx) {
								a.seedInput.SetText("")
							}
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.field(gtx, "Seed", &a.seedInput, "0", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.compactIconTextButton(gtx, &a.randomSeedButton, uiIconRefresh, "随机", false)
										}),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.compactIconTextButton(gtx, &a.clearSeedButton, uiIconClear, "清空", false)
										}),
									)
								}),
							)
						})
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		})
	})
}

func (a *App) layoutAspectButton(gtx layout.Context, btn *widget.Clickable, choice aspectChoice, active bool) layout.Dimensions {
	return a.surfaceButton(
		gtx,
		btn,
		chooseColor(active, fluent.accentSoft, fluent.surface),
		chooseColor(active, accentAlpha(0x28), fluent.surface2),
		fluent.border,
		unit.Dp(6),
		layout.Inset{Top: 8, Bottom: 8, Left: 8, Right: 8},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return fixedWidth(gtx, unit.Dp(float32(choice.W)+10), func(gtx layout.Context) layout.Dimensions {
							return fixedHeight(gtx, unit.Dp(float32(choice.H)+10), func(gtx layout.Context) layout.Dimensions {
								return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.borderedSurface(gtx, chooseColor(active, fluent.surface, fluent.panel2), unit.Dp(3), chooseColor(active, fluent.accent, fluent.textDim), func(gtx layout.Context) layout.Dimensions {
										return fixedWidth(gtx, unit.Dp(float32(choice.W)), func(gtx layout.Context) layout.Dimensions {
											return fixedHeight(gtx, unit.Dp(float32(choice.H)), func(gtx layout.Context) layout.Dimensions {
												return layout.Dimensions{Size: gtx.Constraints.Min}
											})
										})
									})
								})
							})
						})
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, choice.Label, unit.Sp(10), chooseColor(active, fluent.accent, fluent.textMuted), font.Medium)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !choice.Auto {
							return layout.Dimensions{}
						}
						return a.singleLineLabel(gtx, "让上游决定尺寸", unit.Sp(9), chooseColor(active, fluent.accent, fluent.textDim), font.Normal)
					}),
				)
			})
		},
	)
}

func (a *App) layoutActions(gtx layout.Context, snap snapshot, ready bool, hasPrompt bool) layout.Dimensions {
	defer a.recordLayoutTiming(layoutTimingActions, time.Now())
	children := make([]layout.FlexChild, 0, 4)
	if !ready {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.borderedSurface(gtx, fluent.accentSoft, unit.Dp(10), accentAlpha(0x22), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, "还没有可用上游配置", unit.Sp(11), fluent.accent, font.SemiBold)
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(248), func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, "先配置 BASE_URL 和 API Key，才能测试连接或开始生成。", unit.Sp(10), fluent.accent, font.Normal)
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return a.surfaceButton(
										gtx,
										&a.manageUpstreamButton,
										withAlpha(fluent.white, 0xb3),
										fluent.surface,
										accentAlpha(0x1c),
										unit.Dp(8),
										layout.Inset{Top: 6, Bottom: 6, Left: 10, Right: 10},
										func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return fixedWidth(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
														return fixedHeight(gtx, unit.Dp(14), func(gtx layout.Context) layout.Dimensions {
															return uiIconSettings.Layout(gtx, fluent.accent)
														})
													})
												}),
												layout.Rigid(func(gtx layout.Context) layout.Dimensions {
													return a.label(gtx, "配置上游", unit.Sp(10), fluent.accent, font.Medium)
												}),
											)
										},
									)
								})
							})
						}),
					)
				})
			})
		}))
		children = append(children, layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout))
	}

	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		if !ready {
			return a.submitActionButton(gtx, &a.manageUpstreamButton, "配置上游", fluent.accent, fluent.accent2, accentAlpha(0x58), fluent.white)
		}
		if snap.Running {
			return a.submitActionButton(gtx, &a.cancelButton, "取消生成", fluent.dangerSoft, dangerAlpha(0x2a), dangerAlpha(0x30), fluent.danger)
		}
		label := "生成"
		if a.mode == string(client.ModeEdit) {
			label = "编辑"
		}
		if !hasPrompt {
			return a.submitActionButton(gtx, &a.runButton, label, fluent.surface2, fluent.surface2, fluent.border, fluent.textDim)
		}
		return a.submitActionButton(gtx, &a.runButton, label, fluent.accent, fluent.accent2, accentAlpha(0x58), fluent.white)
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *App) layoutDisclosureHeader(gtx layout.Context, btn *widget.Clickable, title string, summary string, open bool) layout.Dimensions {
	stateText := "展开"
	stateIcon := uiIconExpand
	if open {
		stateText = "收起"
		stateIcon = uiIconCollapse
	}
	return a.surfaceButton(
		gtx,
		btn,
		chooseColor(open, fluent.surface2, fluent.surface),
		fluent.surface2,
		fluent.border,
		fluentCardRadius,
		layout.Inset{Top: 10, Bottom: 10, Left: 12, Right: 12},
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.sectionEyebrow(gtx, title)
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.singleLineLabel(gtx, summary, unit.Sp(11), fluent.textMuted, font.Normal)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
								return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
									return stateIcon.Layout(gtx, fluent.textDim)
								})
							})
						}),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, stateText, unit.Sp(11), fluent.textDim, font.Medium)
						}),
					)
				}),
			)
		},
	)
}

func (a *App) layoutComposeAccordionHeader(gtx layout.Context, summary string, open bool) layout.Dimensions {
	stateText := "展开"
	stateIcon := uiIconExpand
	if open {
		stateText = "收起"
		stateIcon = uiIconCollapse
	}
	return a.composeToggleButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fg := fluent.textMuted
		if a.composeToggleButton.Hovered() {
			fg = fluent.text
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "创作参数", unit.Sp(11), fluent.textMuted, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, summary, unit.Sp(12), fluent.textMuted, font.Normal)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.surface(gtx, chooseColor(a.composeToggleButton.Hovered(), fluent.toolHoverBg, rgba(0xffffff, 0x00)), fluentControlRadius, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
										return stateIcon.Layout(gtx, fg)
									})
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, stateText, unit.Sp(12), fg, font.Normal)
							}),
						)
					})
				})
			}),
		)
	})
}

func (a *App) layoutAdvancedAccordionHeader(gtx layout.Context, summary string, open bool) layout.Dimensions {
	stateText := "展开"
	stateIcon := uiIconExpand
	if open {
		stateText = "收起"
		stateIcon = uiIconCollapse
	}
	return a.advancedToggleButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		fg := fluent.textMuted
		if a.advancedToggleButton.Hovered() {
			fg = fluent.text
		}
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "高级参数", unit.Sp(11), fluent.textMuted, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.singleLineLabel(gtx, summary, unit.Sp(12), fluent.textMuted, font.Normal)
					}),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.surface(gtx, chooseColor(a.advancedToggleButton.Hovered(), fluent.toolHoverBg, rgba(0xffffff, 0x00)), fluentControlRadius, func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: 6, Bottom: 6, Left: 8, Right: 8}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return fixedWidth(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
									return fixedHeight(gtx, unit.Dp(12), func(gtx layout.Context) layout.Dimensions {
										return stateIcon.Layout(gtx, fg)
									})
								})
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.label(gtx, stateText, unit.Sp(12), fg, font.Normal)
							}),
						)
					})
				})
			}),
		)
	})
}

func (a *App) advancedSectionCard(gtx layout.Context, title string, hint string, body layout.Widget) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, title, unit.Sp(11), fluent.textMuted, font.SemiBold)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, unit.Dp(8), fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
						layout.Rigid(body),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							if strings.TrimSpace(hint) == "" {
								return layout.Dimensions{}
							}
							return a.label(gtx, hint, unit.Sp(10), fluent.textDim, font.Normal)
						}),
					)
				})
			})
		}),
	)
}

func (a *App) editorText(gtx layout.Context, editor *widget.Editor, hint string, size unit.Sp) layout.Dimensions {
	style := material.Editor(a.th, editor, hint)
	style.Color = fluent.text
	style.HintColor = fluent.textDim
	style.SelectionColor = accentAlpha(0x3d)
	style.TextSize = a.scaledSp(size)
	return style.Layout(gtx)
}

func apiChoiceLabel(value string) string {
	return choiceLabel(apiChoices, value)
}

func (a *App) layoutProbeModelSuggestions(
	gtx layout.Context,
	title string,
	prefix string,
	models []kernel.UpstreamModelDescriptor,
	selectedID string,
	onSelect func(string),
) layout.Dimensions {
	if len(models) == 0 {
		return layout.Dimensions{}
	}
	limit := len(models)
	if limit > 8 {
		limit = 8
	}
	rows := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, title, unit.Sp(11), fluent.textMuted, font.SemiBold)
		}),
	}
	for idx := 0; idx < limit; idx += 2 {
		idx := idx
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			children := make([]layout.FlexChild, 0, 2)
			for col := 0; col < 2; col++ {
				modelIndex := idx + col
				if modelIndex >= limit {
					children = append(children, layout.Flexed(1, layout.Spacer{}.Layout))
					continue
				}
				model := models[modelIndex]
				btn := a.settingsProfileButton("probe-model:" + prefix + ":" + model.ID)
				for btn.Clicked(gtx) {
					onSelect(model.ID)
				}
				active := strings.TrimSpace(selectedID) == strings.TrimSpace(model.ID)
				children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.surfaceButton(
						gtx,
						btn,
						chooseColor(active, fluent.accentSoft, fluent.surface),
						chooseColor(active, accentAlpha(0x18), fluent.surface2),
						chooseColor(active, accentAlpha(0x24), fluent.border),
						unit.Dp(8),
						layout.Inset{Top: 8, Bottom: 8, Left: 10, Right: 10},
						func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, probeModelLabel(model), unit.Sp(10), chooseColor(active, fluent.accent, fluent.text), font.Medium)
						},
					)
				}))
			}
			return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx, children...)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, rows...)
}

func policyChoiceLabel(value string) string {
	return choiceLabel(policyChoices, value)
}

func proxyChoiceLabel(value string) string {
	return choiceLabel(proxyChoices, value)
}

func negativePromptSummary(value string) string {
	if strings.TrimSpace(value) == "" {
		return "无负向限制"
	}
	return "已填负向提示词"
}

func chooseOptionalCompressionSummary(value string, format string) string {
	value = strings.TrimSpace(value)
	format = strings.TrimSpace(strings.ToLower(format))
	if format == "png" || value == "" {
		return ""
	}
	return "压缩 " + value
}

func chooseOptionalFidelitySummary(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == client.DefaultInputFidelity {
		return ""
	}
	return "保真 " + inputFidelityChoiceLabel(value)
}

func chooseOptionalImageStyleSummary(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == client.DefaultImageStyle {
		return ""
	}
	return "图风 " + imageStyleChoiceLabel(value)
}

func chooseOptionalUserIdentifierSummary(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "用户标识 已填"
}

func protectStreamPreviewSummary(enabled bool) string {
	if enabled {
		return "预览保护 开"
	}
	return "预览保护 关"
}

func seedSummary(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return "随机 Seed"
	}
	return "Seed " + value
}

func presetLabels(presets []sharedCompat.Preset) []promptHelperItem {
	items := make([]promptHelperItem, 0, len(presets))
	for _, preset := range presets {
		detailItems := compactNonEmpty([]string{
			preset.Size,
			preset.Quality,
			strings.ToUpper(strings.TrimSpace(preset.OutputFormat)),
			fmt.Sprintf("%d 张", normalizeBatchCount(preset.BatchCount)),
		})
		if styleTag := strings.TrimSpace(preset.StyleTag); styleTag != "" {
			detailItems = append(detailItems, "#"+styleChoiceLabel(styleTag))
		}
		detail := strings.Join(detailItems, " · ")
		items = append(items, promptHelperItem{
			ID:     preset.ID,
			Title:  strings.TrimSpace(preset.Name),
			Detail: detail,
			Kind:   "preset",
		})
	}
	return items
}

func promptLabels(values []string) []promptHelperItem {
	items := make([]promptHelperItem, 0, len(values))
	for idx, value := range values {
		items = append(items, promptHelperItem{
			ID:     fmt.Sprintf("%d", idx),
			Title:  shortPrompt(value),
			Detail: value,
			Kind:   "history",
		})
	}
	return items
}

func promptLabelCacheKey(values []string) string {
	return strings.Join(values, "\x00")
}

func presetLabelCacheKey(presets []sharedCompat.Preset) string {
	parts := make([]string, 0, len(presets)*6)
	for _, preset := range presets {
		parts = append(parts,
			preset.ID,
			strings.TrimSpace(preset.Name),
			strings.TrimSpace(preset.Size),
			strings.TrimSpace(preset.Quality),
			strings.TrimSpace(preset.OutputFormat),
			strings.TrimSpace(preset.NegativePrompt),
			strings.TrimSpace(preset.Background),
			strings.TrimSpace(preset.InputFidelity),
			strings.TrimSpace(preset.ImageStyle),
			strings.TrimSpace(preset.Moderation),
			strings.TrimSpace(preset.StyleTag),
			strings.TrimSpace(preset.KernelRuntimeMode),
			strconv.Itoa(normalizeBatchCount(preset.BatchCount)),
		)
	}
	return strings.Join(parts, "\x00")
}

func (a *App) promptLabelsCached(values []string) []promptHelperItem {
	key := promptLabelCacheKey(values)
	a.mu.Lock()
	if a.promptLabelCacheKey == key {
		items := a.promptLabelCacheItems
		a.mu.Unlock()
		return items
	}
	a.mu.Unlock()

	items := promptLabels(values)

	a.mu.Lock()
	if a.promptLabelCacheKey == key {
		items = a.promptLabelCacheItems
		a.mu.Unlock()
		return items
	}
	a.promptLabelCacheKey = key
	a.promptLabelCacheItems = items
	a.mu.Unlock()
	return items
}

func (a *App) presetLabelsCached(presets []sharedCompat.Preset) []promptHelperItem {
	key := presetLabelCacheKey(presets)
	a.mu.Lock()
	if a.presetLabelCacheKey == key {
		items := a.presetLabelCacheItems
		a.mu.Unlock()
		return items
	}
	a.mu.Unlock()

	items := presetLabels(presets)

	a.mu.Lock()
	if a.presetLabelCacheKey == key {
		items = a.presetLabelCacheItems
		a.mu.Unlock()
		return items
	}
	a.presetLabelCacheKey = key
	a.presetLabelCacheItems = items
	a.mu.Unlock()
	return items
}
