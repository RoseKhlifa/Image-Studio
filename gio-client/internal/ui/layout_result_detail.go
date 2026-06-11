package ui

import (
	"fmt"
	"image"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	sharedCompat "image-studio/shared/compat"

	"gioui.org/font"
	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/unit"
)

func (a *App) layoutResultDetailModal(gtx layout.Context, snap snapshot) layout.Dimensions {
	for a.closeResultDetailButton.Clicked(gtx) {
		a.closeResultDetail()
	}
	item := snap.ActiveResultDetail
	if item.ID == "" && strings.TrimSpace(item.SavedPath) == "" {
		return layout.Dimensions{}
	}
	for a.resultDetailUsePromptButton.Clicked(gtx) {
		a.useResultPrompt(item.Prompt)
	}
	for a.resultDetailUseRevisedButton.Clicked(gtx) {
		a.useResultPrompt(item.RevisedPrompt)
	}
	for a.resultDetailSaveAsButton.Clicked(gtx) {
		a.openSavePromptForItem(item)
	}
	for a.resultDetailUseSourceButton.Clicked(gtx) {
		a.reuseHistoryItemAsSource(item)
		a.appendLog("已将历史结果加入源图: " + shortPrompt(item.Prompt))
	}
	for a.resultDetailApplyParamsButton.Clicked(gtx) {
		a.applyHistoryParams(item)
		a.closeResultDetail()
	}
	for a.resultDetailRegenerateButton.Clicked(gtx) {
		a.closeResultDetail()
		a.regenerateFromHistoryItem(item)
	}
	for a.resultDetailCompareButton.Clicked(gtx) {
		if err := a.toggleCompareItem(item); err != nil && !isMissingPreview(err) {
			a.appendLog("载入对比图失败: " + err.Error())
		} else if compareItemActive(item.ID, snap.Compare.Item.ID) {
			a.appendLog("已退出对比图")
		} else {
			a.appendLog("已设为对比图")
		}
	}
	for a.resultDetailOpenRawButton.Clicked(gtx) {
		raw := strings.TrimSpace(item.RawPath)
		if raw == "" {
			continue
		}
		a.openRawResponseModal(raw)
	}
	for a.resultDetailCopyPromptButton.Clicked(gtx) {
		copyResultDetailText(gtx, item.Prompt)
		a.appendLog("已复制原始提示词")
	}
	for a.resultDetailCopyRevisedButton.Clicked(gtx) {
		copyResultDetailText(gtx, item.RevisedPrompt)
		a.appendLog("已复制优化后提示词")
	}
	for a.resultDetailOpenPathButton.Clicked(gtx) {
		path := strings.TrimSpace(item.SavedPath)
		if path == "" || isVirtualImagePath(path) {
			path = strings.TrimSpace(a.outputDirInput.Text())
		}
		if path == "" {
			continue
		}
		target := path
		if strings.TrimSpace(item.SavedPath) != "" && !isVirtualImagePath(item.SavedPath) {
			target = filepath.Dir(path)
		}
		if err := openPath(target); err != nil {
			a.appendLog("打开文件夹失败: " + err.Error())
		}
	}
	for a.resultDetailCopyPathButton.Clicked(gtx) {
		copyResultDetailText(gtx, item.SavedPath)
		a.appendLog("已复制文件路径")
	}
	for a.resultDetailDeleteButton.Clicked(gtx) {
		a.deleteHistoryItem(item.ID)
	}
	return a.layoutStandardModal(
		gtx,
		unit.Dp(720),
		unit.Dp(620),
		"生成详情",
		"",
		&a.closeResultDetailButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(14))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(280), func(gtx layout.Context) layout.Dimensions {
						return a.layoutResultDetailPreview(gtx, item)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.layoutResultDetailSections(gtx, item)
				}),
			)
		},
	)
}

func (a *App) layoutResultDetailPreview(gtx layout.Context, item sharedCompat.HistoryItem) layout.Dimensions {
	img, imgOp := a.displayHistoryThumb(item, gtx.Dp(unit.Dp(248)))
	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface2, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(8)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.imageThumbWithOp(gtx, img, imgOp, unit.Dp(248), unit.Dp(248), unit.Dp(8))
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !canSaveHistoryItem(item) {
						return layout.Dimensions{}
					}
					children := []layout.FlexChild{
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.compactIconTextButton(gtx, &a.resultDetailSaveAsButton, uiIconSave, "另存为", false)
						}),
					}
					openDirLabel := "打开输出目录"
					if strings.TrimSpace(item.SavedPath) != "" && !isVirtualImagePath(item.SavedPath) {
						openDirLabel = "打开文件夹"
					}
					if strings.TrimSpace(item.SavedPath) != "" || strings.TrimSpace(a.outputDirInput.Text()) != "" {
						children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.compactIconTextButton(gtx, &a.resultDetailOpenPathButton, uiIconFolder, openDirLabel, false)
						}))
					}
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, children...)
				}),
			)
		})
	})
}

func (a *App) layoutResultDetailSections(gtx layout.Context, item sharedCompat.HistoryItem) layout.Dimensions {
	sections := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return a.layoutResultDetailMeta(gtx, item) },
		func(gtx layout.Context) layout.Dimensions {
			return a.layoutResultDetailTextSection(gtx, "原始提示词", item.Prompt)
		},
	}
	if strings.TrimSpace(item.RevisedPrompt) != "" {
		sections = append(sections, func(gtx layout.Context) layout.Dimensions {
			return a.layoutResultDetailTextSection(gtx, "优化后提示词", item.RevisedPrompt)
		})
	}
	if strings.TrimSpace(item.NegativePrompt) != "" {
		sections = append(sections, func(gtx layout.Context) layout.Dimensions {
			return a.layoutResultDetailTextSection(gtx, "负向提示词", item.NegativePrompt)
		})
	}
	sections = append(sections, func(gtx layout.Context) layout.Dimensions {
		return a.layoutResultDetailFileSection(gtx, item)
	})
	return a.settingsList.Layout(gtx, len(sections), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(12)}.Layout(gtx, sections[index])
	})
}

func (a *App) layoutResultDetailMeta(gtx layout.Context, item sharedCompat.HistoryItem) layout.Dimensions {
	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			rows := []layout.Widget{
				func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "参数", unit.Sp(11), fluent.textMuted, font.SemiBold)
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.detailKVRow(gtx, "模式", chooseModeLabel(item.Mode), false, false)
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.detailKVRow(gtx, "尺寸", sizeDisplayLabel(item.Size), false, false)
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.detailKVRow(gtx, "质量", qualityDisplayLabel(item.Quality), false, false)
				},
				func(gtx layout.Context) layout.Dimensions {
					return a.detailKVRow(gtx, "格式", strings.ToUpper(strings.TrimSpace(item.OutputFormat)), false, false)
				},
			}
			if item.Seed != 0 {
				rows = append(rows, func(gtx layout.Context) layout.Dimensions {
					return a.detailKVRow(gtx, "Seed", detailValue(item.Seed), true, false)
				})
			}
			if strings.TrimSpace(item.StyleTag) != "" {
				rows = append(rows, func(gtx layout.Context) layout.Dimensions {
					return a.detailKVRow(gtx, "风格", "#"+styleChoiceLabel(item.StyleTag), false, false)
				})
			}
			rows = append(rows,
				func(gtx layout.Context) layout.Dimensions {
					return a.detailKVRow(gtx, "创建时间", formatHistoryDateTime(item.CreatedAt), false, false)
				},
				func(gtx layout.Context) layout.Dimensions {
					if item.ElapsedSec <= 0 {
						return layout.Dimensions{}
					}
					return a.detailKVRow(gtx, "耗时", detailValue(item.ElapsedSec)+"s", false, true)
				},
				func(gtx layout.Context) layout.Dimensions {
					return layout.Inset{Top: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.resultDetailApplyParamsButton, uiIconCheck, "应用参数", false)
							}),
							layout.Rigid(func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, &a.resultDetailRegenerateButton, uiIconRefresh, "重新生成", true)
							}),
						)
					})
				},
			)
			return a.settingsList.Layout(gtx, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
				return rows[index](gtx)
			})
		})
	})
}

func (a *App) layoutResultDetailTextSection(gtx layout.Context, title string, text string) layout.Dimensions {
	actionBtn := &a.resultDetailUsePromptButton
	copyBtn := &a.resultDetailCopyPromptButton
	actionLabel := "用作下次提示词"
	actionAccent := false
	if strings.Contains(title, "优化后") {
		actionBtn = &a.resultDetailUseRevisedButton
		copyBtn = &a.resultDetailCopyRevisedButton
		actionAccent = true
	}
	muted := strings.Contains(title, "负向")
	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, title, unit.Sp(11), fluent.textMuted, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					content := strings.TrimSpace(text)
					if content == "" {
						content = "(空)"
					}
					return a.resultDetailPromptBlock(gtx, content, muted, actionAccent)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if strings.TrimSpace(text) == "" {
						return layout.Dimensions{}
					}
					children := []layout.FlexChild{
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.compactIconTextButton(gtx, copyBtn, uiIconCopy, "复制", false)
						}),
					}
					if !muted {
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return layout.Inset{Left: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								return a.compactIconTextButton(gtx, actionBtn, uiIconRefresh, actionLabel, actionAccent)
							})
						}))
					}
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(0))}.Layout(gtx, children...)
				}),
			)
		})
	})
}

func (a *App) layoutResultDetailFileSection(gtx layout.Context, item sharedCompat.HistoryItem) layout.Dimensions {
	compareActive := compareItemActive(item.ID, a.readSnapshot().Compare.Item.ID)
	return a.elevatedBorderedSurface(gtx, fluent.surfaceElevated, fluentCardRadius, fluent.border, image.Pt(0, 1), func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "文件", unit.Sp(11), fluent.textMuted, font.SemiBold)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface2, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							path := strings.TrimSpace(item.SavedPath)
							if path == "" {
								return a.label(gtx, "(本次未落盘 / 路径丢失)", unit.Sp(11), fluent.textDim, font.Normal)
							}
							return a.label(gtx, path, unit.Sp(11), fluent.textMuted, font.Normal)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if strings.TrimSpace(item.SavedPath) == "" && strings.TrimSpace(item.RawPath) == "" && !canSaveHistoryItem(item) {
						return layout.Dimensions{}
					}
					children := make([]layout.FlexChild, 0, 5)
					if strings.TrimSpace(item.SavedPath) != "" {
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.compactIconTextButton(gtx, &a.resultDetailCopyPathButton, uiIconCopy, "复制路径", false)
						}))
					}
					if canSaveHistoryItem(item) {
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.compactIconTextButton(gtx, &a.resultDetailUseSourceButton, uiIconSource, "设为源图", false)
						}))
					}
					if strings.TrimSpace(item.ID) != "" || strings.TrimSpace(item.SavedPath) != "" {
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.compactIconTextButton(gtx, &a.resultDetailCompareButton, uiIconCompare, chooseCompareButtonLabel(compareActive), compareActive)
						}))
					}
					if strings.TrimSpace(item.RawPath) != "" {
						children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return a.compactIconTextButton(gtx, &a.resultDetailOpenRawButton, uiIconList, "查看 Raw", false)
						}))
					}
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx, children...)
				}),
			)
		})
	})
}

func copyResultDetailText(gtx layout.Context, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
}

func (a *App) detailKV(gtx layout.Context, label string, value string) layout.Dimensions {
	value = strings.TrimSpace(value)
	if value == "" {
		return layout.Dimensions{}
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedWidth(gtx, unit.Dp(68), func(gtx layout.Context) layout.Dimensions {
				return a.label(gtx, label, unit.Sp(10), fluent.textDim, font.Medium)
			})
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, value, unit.Sp(11), fluent.text, font.Normal)
		}),
	)
}

func (a *App) detailKVRow(gtx layout.Context, label string, value string, mono bool, last bool) layout.Dimensions {
	value = strings.TrimSpace(value)
	if value == "" {
		return layout.Dimensions{}
	}
	return layout.Inset{Bottom: chooseInset(last)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(72), func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, label, unit.Sp(10), fluent.textDim, font.Normal)
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					weight := font.Medium
					if mono {
						weight = font.SemiBold
					}
					return a.label(gtx, value, unit.Sp(11), fluent.text, weight)
				}),
			)
		})
	})
}

func chooseInset(last bool) unit.Dp {
	if last {
		return 0
	}
	return unit.Dp(4)
}

func (a *App) resultDetailPromptBlock(gtx layout.Context, text string, muted bool, highlight bool) layout.Dimensions {
	bg := fluent.surface2
	fg := fluent.textMuted
	border := fluent.border
	if muted {
		bg = fluent.surface2
		fg = fluent.textDim
	}
	if highlight {
		bg = fluent.accentSoft
		fg = fluent.accent
		border = accentAlpha(0x24)
	}
	return a.borderedSurface(gtx, bg, fluentCardRadius, border, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return a.label(gtx, text, unit.Sp(11), fg, font.Normal)
		})
	})
}

func detailHeadline(item sharedCompat.HistoryItem) string {
	return chooseModeLabel(item.Mode) + " · " + historyMetaText(item)
}

func chooseModeLabel(mode string) string {
	if mode == "edit" {
		return "图生图"
	}
	return "文生图"
}

func detailValue[T any](value T) string {
	switch v := any(value).(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
