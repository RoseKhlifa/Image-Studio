package ui

import (
	"context"
	"strings"

	"gioui.org/font"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/unit"
	"github.com/yuanhua/image-gptcodex/pkg/promptimport"
	"image-studio/gio-client/internal/promptscheme"
	sharedCompat "image-studio/shared/compat"
)

func normalizeImportedPromptSize(size string, customRatios []sharedCompat.CustomAspectRatio) string {
	size = strings.TrimSpace(size)
	if size == "" || size == "auto" {
		return "auto"
	}
	if _, ok := sizeToAspect[size]; ok {
		return size
	}
	for _, ratio := range customRatios {
		for _, resolution := range []string{"1k", "2k", "4k"} {
			if buildCustomSizeSelection(ratio, resolution) == size {
				return size
			}
		}
	}
	return "auto"
}

func normalizeImportedPromptSizeForApp(a *App, size string) string {
	candidate := strings.TrimSpace(size)
	if candidate == "" {
		candidate = "auto"
	}
	if a == nil {
		return normalizeImportedPromptSize(candidate, nil)
	}
	customRatios := append([]sharedCompat.CustomAspectRatio(nil), a.customAspectRatios...)
	return normalizeSizeSelection(candidate, a.api, a.policy, a.imageModelInput.Text(), customRatios)
}

func promptImportErrorMessage(err error) string {
	switch promptimport.ErrorCode(err) {
	case promptimport.TokenUsed:
		return "这个提示词已经导入过了"
	case promptimport.TokenExpired:
		return "导入链接已过期，请回网页重新发送"
	case promptimport.TokenNotFound, promptimport.TokenInvalid:
		return "提示词链接无效或已被清理，请回网页重新发送"
	default:
		return "导入服务暂时不可用"
	}
}

func promptImportPreferredText(text *promptimport.BilingualText) string {
	return promptimport.PreferChinese(text)
}

func (a *App) HandlePromptImportInvalid() {
	a.RaiseWindow()
	a.mu.Lock()
	a.status = "导入链接无效"
	a.lastErrorMessage = "提示词链接无效或已被清理，请回网页重新发送"
	a.appendLogLocked("外部导入失败: " + a.lastErrorMessage)
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) HandlePromptImportToken(token string) {
	token = strings.TrimSpace(token)
	if !promptimport.IsValidToken(token) {
		a.HandlePromptImportInvalid()
		return
	}
	a.RaiseWindow()
	a.mu.Lock()
	a.promptImportQueue = append(a.promptImportQueue, token)
	shouldStart := !a.promptImportLoading && !a.promptImportOpen && len(a.promptImportQueue) == 1
	a.mu.Unlock()
	a.invalidateNow()
	if shouldStart {
		go a.runNextPromptImport()
	}
}

func (a *App) runNextPromptImport() {
	a.mu.Lock()
	if a.promptImportLoading || a.promptImportOpen || len(a.promptImportQueue) == 0 {
		a.mu.Unlock()
		return
	}
	token := a.promptImportQueue[0]
	a.promptImportLoading = true
	a.promptImportToken = token
	a.promptImportPayload = nil
	a.promptImportResolvedSize = "auto"
	a.mu.Unlock()
	a.invalidateNow()

	payload, err := promptimport.Fetch(context.Background(), token, promptimport.FetchOptions{})
	if err != nil {
		message := promptImportErrorMessage(err)
		a.mu.Lock()
		if len(a.promptImportQueue) > 0 && a.promptImportQueue[0] == token {
			a.promptImportQueue = a.promptImportQueue[1:]
		}
		a.promptImportLoading = false
		a.promptImportToken = ""
		a.status = "导入失败"
		a.lastErrorMessage = message
		a.appendLogLocked("外部导入失败: " + message)
		a.mu.Unlock()
		a.invalidateNow()
		go a.runNextPromptImport()
		return
	}

	a.mu.Lock()
	if len(a.promptImportQueue) > 0 && a.promptImportQueue[0] == token {
		a.promptImportQueue = a.promptImportQueue[1:]
	}
	a.promptImportLoading = false
	a.promptImportOpen = true
	a.promptImportToken = token
	a.promptImportPayload = payload
	a.promptImportResolvedSize = normalizeImportedPromptSizeForApp(a, payload.ResolvedSize)
	a.lastErrorMessage = ""
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) confirmPromptImport() {
	a.mu.Lock()
	payload := a.promptImportPayload
	resolvedSize := a.promptImportResolvedSize
	a.mu.Unlock()
	if payload == nil {
		a.closePromptImportModal()
		return
	}
	a.promptInput.SetText(promptImportPreferredText(&payload.Prompt))
	if payload.NegativePrompt != nil {
		a.negativePromptInput.SetText(promptImportPreferredText(payload.NegativePrompt))
	} else {
		a.negativePromptInput.SetText("")
	}
	a.size = normalizeImportedPromptSizeForApp(a, resolvedSize)
	a.mu.Lock()
	a.promptImportOpen = false
	a.promptImportPayload = nil
	a.promptImportToken = ""
	a.status = "已从 Image-Prompts 导入提示词"
	a.saveActiveWorkspaceSnapshot()
	a.appendLogLocked("已从 Image-Prompts 导入提示词")
	a.mu.Unlock()
	a.invalidateNow()
	go a.runNextPromptImport()
}

func (a *App) closePromptImportModal() {
	a.mu.Lock()
	a.promptImportOpen = false
	a.promptImportPayload = nil
	a.promptImportToken = ""
	a.mu.Unlock()
	a.invalidateNow()
	go a.runNextPromptImport()
}

func (a *App) schedulePromptImportRegistrationCheck() {
	go func() {
		status, err := promptscheme.StatusForCurrentExecutable()
		if err != nil {
			a.appendLog("检测网页导入协议失败: " + err.Error())
			return
		}
		a.mu.Lock()
		a.promptImportRegisterOpen = !status.Registered
		if strings.TrimSpace(status.Handler) != "" {
			a.promptImportRegisterNote = status.Handler
		} else if strings.TrimSpace(status.Detail) != "" {
			a.promptImportRegisterNote = status.Detail
		}
		a.mu.Unlock()
		a.invalidateNow()
	}()
}

func (a *App) registerPromptImportProtocol() {
	a.mu.Lock()
	if a.promptImportRegisterBusy {
		a.mu.Unlock()
		return
	}
	a.promptImportRegisterBusy = true
	a.promptImportRegisterNote = "正在注册 image-studio:// 协议..."
	a.mu.Unlock()
	a.invalidateNow()
	go func() {
		err := promptscheme.RegisterCurrentExecutable()
		if err != nil {
			a.mu.Lock()
			a.promptImportRegisterBusy = false
			a.promptImportRegisterNote = "注册失败: " + err.Error()
			a.lastErrorMessage = "网页导入协议注册失败"
			a.appendLogLocked("网页导入协议注册失败: " + err.Error())
			a.mu.Unlock()
			a.invalidateNow()
			return
		}
		a.mu.Lock()
		a.promptImportRegisterBusy = false
		a.promptImportRegisterOpen = false
		a.promptImportRegisterNote = "已注册 image-studio:// 到当前 Gio 客户端"
		a.appendLogLocked("已注册网页导入协议到当前 Gio 客户端")
		a.mu.Unlock()
		a.invalidateNow()
	}()
}

func (a *App) dismissPromptImportRegistrationPrompt() {
	a.mu.Lock()
	a.promptImportRegisterOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) RaiseWindow() {
	if a.window != nil {
		a.window.Perform(system.ActionRaise)
		a.window.Invalidate()
	}
}

func (a *App) layoutPromptImportModal(gtx layout.Context, snap snapshot) layout.Dimensions {
	for a.promptImportCloseButton.Clicked(gtx) {
		a.closePromptImportModal()
	}
	for a.promptImportConfirmButton.Clicked(gtx) {
		a.confirmPromptImport()
	}
	payload := snap.PromptImportPayload
	if payload == nil {
		return layout.Dimensions{}
	}
	aspectRatio := strings.TrimSpace(payload.AspectRatio)
	if aspectRatio == "" {
		aspectRatio = "auto"
	}
	renderTextBlock := func(gtx layout.Context, title string, text *promptimport.BilingualText) layout.Dimensions {
		return a.borderedSurface(gtx, fluent.surface2, unit.Dp(14), fluent.border, func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, title, unit.Sp(12), fluent.text, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "中文", unit.Sp(10), fluent.textMuted, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, strings.TrimSpace(text.Zh), unit.Sp(11), fluent.text, font.Normal)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, "English", unit.Sp(10), fluent.textMuted, font.SemiBold)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return a.label(gtx, strings.TrimSpace(text.En), unit.Sp(11), fluent.text, font.Normal)
					}),
				)
			})
		})
	}
	return a.layoutStandardModal(
		gtx,
		unit.Dp(640),
		0,
		"从 Image-Prompts 导入提示词",
		"来源站点: prompts.sorry.ink",
		&a.promptImportCloseButton,
		func(gtx layout.Context) layout.Dimensions {
			negative := payload.NegativePrompt
			if negative == nil {
				negative = &promptimport.BilingualText{}
			}
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "确认后将覆盖当前表单中的提示词、反向提示词与尺寸，不会改动参考图、模式、批处理或上游配置。", unit.Sp(11), fluent.textMuted, font.Normal)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return renderTextBlock(gtx, "正向提示词", &payload.Prompt)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return renderTextBlock(gtx, "反向提示词", negative)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.borderedSurface(gtx, fluent.surface2, unit.Dp(12), fluent.border, func(gtx layout.Context) layout.Dimensions {
								return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, "站点比例", unit.Sp(10), fluent.textMuted, font.SemiBold)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, aspectRatio, unit.Sp(12), fluent.text, font.SemiBold)
										}),
									)
								})
							})
						}),
						layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
							return a.borderedSurface(gtx, fluent.surface2, unit.Dp(12), fluent.border, func(gtx layout.Context) layout.Dimensions {
								return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx,
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, "应用后尺寸", unit.Sp(10), fluent.textMuted, font.SemiBold)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, snap.PromptImportResolvedSize, unit.Sp(12), fluent.text, font.SemiBold)
										}),
									)
								})
							})
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(110), func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.promptImportCloseButton, "取消", false)
							})
						}),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(148), func(gtx layout.Context) layout.Dimensions {
								return a.primaryButton(gtx, &a.promptImportConfirmButton, "导入到表单", fluent.accent, fluent.white)
							})
						}),
					)
				}),
			)
		},
	)
}

func (a *App) layoutPromptImportRegistrationPrompt(gtx layout.Context, snap snapshot) layout.Dimensions {
	for a.promptImportRegisterNowButton.Clicked(gtx) {
		a.registerPromptImportProtocol()
	}
	for a.promptImportRegisterLaterButton.Clicked(gtx) {
		a.dismissPromptImportRegistrationPrompt()
	}
	subtitle := "当前 Gio 桌面端可接收 image-studio://import?... 深链。"
	return a.layoutStandardModal(
		gtx,
		unit.Dp(560),
		0,
		"注册网页导入协议",
		subtitle,
		nil,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return a.label(gtx, "若你希望从 prompts.sorry.ink 点击 “Send to Image-Studio” 后直接打开当前 Gio 客户端，请先把 image-studio:// 协议注册到当前程序。", unit.Sp(11), fluent.textMuted, font.Normal)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if strings.TrimSpace(snap.PromptImportRegisterNote) == "" {
						return layout.Dimensions{}
					}
					return a.borderedSurface(gtx, fluent.surface2, unit.Dp(12), fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return a.label(gtx, snap.PromptImportRegisterNote, unit.Sp(10), fluent.textDim, font.Normal)
						})
					})
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return fixedWidth(gtx, unit.Dp(110), func(gtx layout.Context) layout.Dimensions {
								return a.compactButton(gtx, &a.promptImportRegisterLaterButton, "稍后", false)
							})
						}),
						layout.Flexed(1, layout.Spacer{}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							label := "注册为默认处理器"
							if snap.PromptImportRegisterBusy {
								label = "注册中..."
							}
							return fixedWidth(gtx, unit.Dp(172), func(gtx layout.Context) layout.Dimensions {
								return a.primaryButton(gtx, &a.promptImportRegisterNowButton, label, fluent.accent, fluent.white)
							})
						}),
					)
				}),
			)
		},
	)
}
