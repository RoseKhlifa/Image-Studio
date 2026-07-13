package ui

import (
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/widget"
)

func (a *App) anyTextEditorFocused(gtx layout.Context) bool {
	editors := []*widget.Editor{
		&a.apiKeyInput,
		&a.baseURLInput,
		&a.textModelInput,
		&a.imageModelInput,
		&a.profileNameInput,
		&a.concurrencyLimitInput,
		&a.promptInput,
		&a.sourcePathsInput,
		&a.outputDirInput,
		&a.seedInput,
		&a.negativePromptInput,
		&a.partialImagesInput,
		&a.outputCompressionInput,
		&a.proxyURLInput,
		&a.userIdentifierInput,
		&a.savePromptPathInput,
		&a.promptTemplateLabelInput,
		&a.promptTemplateTextInput,
		&a.presetNameInput,
		&a.presetSizeInput,
		&a.presetQualityInput,
		&a.presetOutputFormatInput,
		&a.presetBatchCountInput,
		&a.presetStyleTagInput,
		&a.loopTotalCountInput,
		&a.loopConcurrencyInput,
		&a.loopAutoSaveDirInput,
		&a.batchInputDirInput,
		&a.batchOutputDirInput,
		&a.batchOutputPrefixInput,
		&a.batchConcurrencyInput,
		&a.upstreamQuickImportInput,
		&a.rawResponseViewerInput,
		&a.historyQueryInput,
		&a.historyTimelineQueryInput,
		&a.historyTimelinePickedDateInput,
		&a.workspaceNameInput,
		&a.canvasAnnotationTextInput,
		&a.customSizeWidthInput,
		&a.customSizeHeightInput,
		&a.customAspectWidthInput,
		&a.customAspectHeightInput,
	}
	for _, editor := range editors {
		if editor != nil && gtx.Focused(editor) {
			return true
		}
	}
	return false
}

func matchPrimaryShortcut(modifiers key.Modifiers) bool {
	return modifiers.Contain(key.ModCtrl) || modifiers.Contain(key.ModCommand)
}

func (a *App) performGlobalShortcut(keyName key.Name, modifiers key.Modifiers, textInputFocused bool) bool {
	if keyName == key.NameF11 {
		if textInputFocused {
			return false
		}
		a.toggleFullscreen()
		return true
	}
	if !matchPrimaryShortcut(modifiers) {
		return false
	}
	switch keyName {
	case key.NameReturn, key.NameEnter:
		a.startRun()
		return true
	case "N", "T":
		if textInputFocused {
			return false
		}
		a.createWorkspace()
		return true
	case "W":
		if textInputFocused {
			return false
		}
		if len(a.workspaces) > 1 {
			a.closeWorkspace(a.activeWorkspaceID)
		}
		return true
	case "F":
		if textInputFocused || !modifiers.Contain(key.ModCtrl) || !modifiers.Contain(key.ModCommand) {
			return false
		}
		a.toggleFullscreen()
		return true
	}
	return false
}

func (a *App) canvasShortcutModalsOpen(snap snapshot) bool {
	if snap.SavePromptVisible || snap.PromptImportVisible || snap.PromptImportRegisterOpen {
		return true
	}
	if snap.HistoryTimelineOpen || snap.ActivePromptGroup.Key != "" {
		return true
	}
	if snap.ActiveResultDetail.ID != "" || snap.ActiveResultDetail.SavedPath != "" {
		return true
	}
	if snap.HistoryActionMenuItem.ID != "" || snap.HistoryActionMenuItem.SavedPath != "" {
		return true
	}
	if snap.RawResponseModalPath != "" || snap.RawResponseModalText != "" || snap.RawResponseModalError != "" {
		return true
	}
	if a.generalSettingsOpen || a.aboutModalOpen || a.settingsModalOpen || a.promptHelperOpen || a.presetPickerOpen || a.promptTemplateManagerOpen || a.presetManagerOpen || a.customAspectRatioManagerOpen || a.customSizeModalOpen || a.loopModalOpen || a.advancedOpen {
		return true
	}
	if a.canvasAnnotationTextPromptOpen {
		return true
	}
	return false
}

func (a *App) handleCanvasKeyboardShortcuts(gtx layout.Context, snap snapshot) {
	if a.canvasShortcutModalsOpen(snap) {
		return
	}
	event.Op(gtx.Ops, &a.keyboardShortcutTag)
	textInputFocused := a.anyTextEditorFocused(gtx)
	globalFilters := []event.Filter{
		key.Filter{Name: key.NameReturn, Required: key.ModCtrl},
		key.Filter{Name: key.NameEnter, Required: key.ModCtrl},
		key.Filter{Name: key.NameReturn, Required: key.ModCommand},
		key.Filter{Name: key.NameEnter, Required: key.ModCommand},
	}
	if !textInputFocused {
		globalFilters = append(globalFilters,
			key.Filter{Name: "N", Required: key.ModCtrl},
			key.Filter{Name: "N", Required: key.ModCommand},
			key.Filter{Name: "T", Required: key.ModCtrl},
			key.Filter{Name: "T", Required: key.ModCommand},
			key.Filter{Name: "W", Required: key.ModCtrl},
			key.Filter{Name: "W", Required: key.ModCommand},
			key.Filter{Name: key.NameF11},
			key.Filter{Name: "F", Required: key.ModCtrl | key.ModCommand},
		)
	}
	for {
		ev, ok := gtx.Event(globalFilters...)
		if !ok {
			break
		}
		keyEvent, ok := ev.(key.Event)
		if !ok || keyEvent.State != key.Press {
			continue
		}
		a.performGlobalShortcut(keyEvent.Name, keyEvent.Modifiers, textInputFocused)
	}
	if textInputFocused {
		return
	}
	workflowMode := normalizeExperienceMode(a.experienceMode) == experienceModeWorkflow
	performCanvasUndo := func(redo bool) {
		if workflowMode {
			if redo {
				a.redoWorkflowGraph(a.activeWorkspaceID)
			} else {
				a.undoWorkflowGraph(a.activeWorkspaceID)
			}
			return
		}
		if redo {
			a.redoLatestCanvasAction()
		} else {
			a.undoLatestCanvasAction()
		}
	}
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: key.NameLeftArrow},
			key.Filter{Name: key.NameRightArrow},
			key.Filter{Name: key.NameDeleteBackward},
			key.Filter{Name: key.NameDeleteForward},
			key.Filter{Name: key.NameEscape},
			key.Filter{Name: key.NameSpace},
			key.Filter{Name: "1"},
			key.Filter{Name: "2"},
			key.Filter{Name: "3"},
			key.Filter{Name: "F"},
			key.Filter{Name: "["},
			key.Filter{Name: "]"},
			key.Filter{Name: "Z", Required: key.ModCtrl, Optional: key.ModShift},
			key.Filter{Name: "Z", Required: key.ModCommand, Optional: key.ModShift},
			key.Filter{Name: "Y", Required: key.ModCtrl},
			key.Filter{Name: "Y", Required: key.ModCommand},
			key.Filter{Name: "M", Required: key.ModCtrl},
			key.Filter{Name: "M", Required: key.ModCommand},
			key.Filter{Name: "S", Required: key.ModCtrl},
			key.Filter{Name: "S", Required: key.ModCommand},
			key.Filter{Name: "O", Required: key.ModCtrl},
			key.Filter{Name: "O", Required: key.ModCommand},
			key.Filter{Name: "C", Required: key.ModCtrl},
			key.Filter{Name: "C", Required: key.ModCommand},
			key.Filter{Name: "V", Required: key.ModCtrl},
			key.Filter{Name: "V", Required: key.ModCommand},
		)
		if !ok {
			break
		}
		keyEvent, ok := ev.(key.Event)
		if !ok {
			continue
		}
		if keyEvent.Name == key.NameSpace {
			a.setCanvasSpacePan(keyEvent.State == key.Press)
			continue
		}
		if keyEvent.State != key.Press {
			continue
		}
		delta := 0
		switch keyEvent.Name {
		case key.NameLeftArrow:
			delta = -1
		case key.NameRightArrow:
			delta = 1
		case key.NameDeleteBackward, key.NameDeleteForward:
			if workflowMode {
				a.deleteSelectedWorkflowNode(a.activeWorkspaceID)
			} else if a.currentCanvasTool() != canvasToolMask {
				a.deleteSelectedCanvasAnnotation()
			}
			continue
		case key.NameEscape:
			switch {
			case workflowMode && a.workflowCanvas.connection.active:
				a.workflowCanvas.connection = workflowConnectionDrag{}
				a.invalidateNow()
			case snap.Running:
				a.cancelRun()
			case snap.Compare.HasItem:
				a.clearCompare()
			case a.currentCanvasTool() != canvasToolMask:
				a.clearCanvasAnnotationSelection()
			case strings.TrimSpace(snap.LastErrorMessage) != "":
				a.dismissFailureState()
			}
			continue
		case "1":
			a.setCanvasTool(canvasToolPan)
			continue
		case "2":
			a.setCanvasTool(canvasToolMask)
			continue
		case "3":
			a.setCanvasTool(canvasToolAnnotate)
			continue
		case "F":
			if workflowMode {
				a.workflowCanvas.fitGraph(a.workflowGraph(a.activeWorkspaceID), desktopThemeSpec(a.desktopStyle, a.resolvedThemeMode))
				a.invalidateNow()
			} else {
				a.resetCanvasView()
			}
			continue
		case "[":
			if a.currentCanvasTool() == canvasToolMask {
				a.adjustCanvasBrushSize(-5)
			}
			continue
		case "]":
			if a.currentCanvasTool() == canvasToolMask {
				a.adjustCanvasBrushSize(5)
			}
			continue
		case "Z":
			performCanvasUndo(keyEvent.Modifiers.Contain(key.ModShift))
			continue
		case "Y":
			performCanvasUndo(true)
			continue
		case "M":
			if workflowMode {
				a.toggleSelectedWorkflowNodeEnabled(a.activeWorkspaceID)
			}
			continue
		case "S":
			if workflowMode {
				a.exportWorkflowJSON()
			}
			continue
		case "O":
			if workflowMode {
				a.importWorkflowJSON()
			}
			continue
		case "C":
			if err := a.copyCurrentResultImageToClipboard(gtx, snap); err != nil {
				a.appendLog("复制图片失败: " + err.Error())
			} else {
				a.appendLog("已复制图片到剪贴板")
			}
			continue
		case "V":
			gtx.Execute(clipboard.ReadCmd{Tag: &a.keyboardShortcutTag})
			continue
		}
		if delta == 0 || !canStepBatchResultSnapshot(snap) {
			continue
		}
		if err := a.stepBatchResult(delta); err != nil && !isMissingPreview(err) {
			a.appendLog("切换批量结果失败: " + err.Error())
		}
	}
	for {
		ev, ok := gtx.Event(
			transfer.TargetFilter{Target: &a.keyboardShortcutTag, Type: "image/png"},
			transfer.TargetFilter{Target: &a.keyboardShortcutTag, Type: "image/jpeg"},
			transfer.TargetFilter{Target: &a.keyboardShortcutTag, Type: "image/webp"},
		)
		if !ok {
			break
		}
		dataEvent, ok := ev.(transfer.DataEvent)
		if !ok {
			continue
		}
		reader := dataEvent.Open()
		data, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			a.appendLog("读取剪贴板图片失败: " + err.Error())
			continue
		}
		if err := a.importClipboardImageData(data, dataEvent.Type); err != nil {
			a.appendLog("导入剪贴板图片失败: " + err.Error())
			continue
		}
		a.appendLog("已从剪贴板导入图片")
	}
}
