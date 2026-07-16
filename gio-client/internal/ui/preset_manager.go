package ui

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"time"

	gioCompat "image-studio/gio-client/internal/compat"
	sharedCompat "image-studio/shared/compat"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

type presetSummaryState struct {
	SelectedPreset *sharedCompat.Preset
	MatchedPreset  *sharedCompat.Preset
	Title          string
	Detail         string
}

func nextPresetName(items []sharedCompat.Preset) string {
	used := map[int]struct{}{}
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if !strings.HasPrefix(name, "配置") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(name, "配置"))
		value, err := strconv.Atoi(raw)
		if err == nil && value > 0 {
			used[value] = struct{}{}
		}
	}
	for i := 1; ; i++ {
		if _, ok := used[i]; !ok {
			return "配置" + strconv.Itoa(i)
		}
	}
}

func presetMatchesSnapshot(preset sharedCompat.Preset, snapshot sharedCompat.Preset) bool {
	if strings.TrimSpace(preset.Size) != strings.TrimSpace(snapshot.Size) {
		return false
	}
	if strings.TrimSpace(preset.Quality) != strings.TrimSpace(snapshot.Quality) {
		return false
	}
	if strings.TrimSpace(preset.OutputFormat) != strings.TrimSpace(snapshot.OutputFormat) {
		return false
	}
	if strings.TrimSpace(preset.NegativePrompt) != strings.TrimSpace(snapshot.NegativePrompt) {
		return false
	}
	if strings.TrimSpace(preset.Background) != strings.TrimSpace(snapshot.Background) {
		return false
	}
	presetCompression := 0
	if preset.OutputCompression != nil {
		presetCompression = *preset.OutputCompression
	}
	snapshotCompression := 0
	if snapshot.OutputCompression != nil {
		snapshotCompression = *snapshot.OutputCompression
	}
	if presetCompression != snapshotCompression {
		return false
	}
	if strings.TrimSpace(preset.InputFidelity) != strings.TrimSpace(snapshot.InputFidelity) {
		return false
	}
	if strings.TrimSpace(preset.ImageStyle) != strings.TrimSpace(snapshot.ImageStyle) {
		return false
	}
	if strings.TrimSpace(preset.Moderation) != strings.TrimSpace(snapshot.Moderation) {
		return false
	}
	if strings.TrimSpace(preset.StyleTag) != strings.TrimSpace(snapshot.StyleTag) {
		return false
	}
	if strings.TrimSpace(preset.EditAutoAspectRes) != strings.TrimSpace(snapshot.EditAutoAspectRes) {
		return false
	}
	if strings.TrimSpace(preset.KernelRuntimeMode) != strings.TrimSpace(snapshot.KernelRuntimeMode) {
		return false
	}
	return normalizeBatchCount(preset.BatchCount) == normalizeBatchCount(snapshot.BatchCount)
}

func describePreset(preset sharedCompat.Preset) string {
	sizeLabel := strings.TrimSpace(preset.Size)
	if autoAspect := strings.TrimSpace(preset.EditAutoAspectRes); autoAspect != "" {
		sizeLabel = "按源图比例 + " + strings.ToUpper(autoAspect)
	} else if sizeLabel == "auto" {
		sizeLabel = "Auto"
	}
	parts := compactNonEmpty([]string{
		sizeLabel,
		qualityChoiceLabel(strings.TrimSpace(preset.Quality)),
		strings.ToUpper(strings.TrimSpace(preset.OutputFormat)),
		fmt.Sprintf("%d 张", normalizeBatchCount(preset.BatchCount)),
	})
	if styleTag := strings.TrimSpace(preset.StyleTag); styleTag != "" {
		parts = append(parts, "#"+styleChoiceLabel(styleTag))
	}
	return strings.Join(parts, " · ")
}

func (a *App) currentPresetSummaryState() presetSummaryState {
	snapshot := a.currentPresetSnapshot()
	var selected *sharedCompat.Preset
	selectedID := strings.TrimSpace(a.selectedPresetID)
	for idx := range a.presets {
		if strings.TrimSpace(a.presets[idx].ID) == selectedID {
			selected = &a.presets[idx]
			break
		}
	}
	var matched *sharedCompat.Preset
	for idx := range a.presets {
		if presetMatchesSnapshot(a.presets[idx], snapshot) {
			matched = &a.presets[idx]
			break
		}
	}
	state := presetSummaryState{
		SelectedPreset: selected,
		MatchedPreset:  matched,
	}
	switch {
	case selected != nil && matched != nil && strings.TrimSpace(selected.ID) == strings.TrimSpace(matched.ID):
		state.Title = "当前使用「" + strings.TrimSpace(selected.Name) + "」"
		state.Detail = "当前参数与已选预设完全一致，可直接覆盖保存。"
	case selected != nil:
		state.Title = "已选「" + strings.TrimSpace(selected.Name) + "」"
		state.Detail = "当前选中方案：" + describePreset(*selected)
	case matched != nil:
		state.Title = "当前匹配「" + strings.TrimSpace(matched.Name) + "」"
		state.Detail = "当前参数正好匹配一条已有预设，可直接覆盖保存。"
	default:
		state.Title = "还没有选中预设"
		state.Detail = "可以把当前参数直接保存成新预设，或打开预设管理器切换。"
	}
	return state
}

func (a *App) presetListButton(id string) *widget.Clickable {
	if a.presetListButtons == nil {
		a.presetListButtons = map[string]*widget.Clickable{}
	}
	if btn, ok := a.presetListButtons[id]; ok {
		return btn
	}
	btn := new(widget.Clickable)
	a.presetListButtons[id] = btn
	return btn
}

func (a *App) presetQuickButton(id string) *widget.Clickable {
	if a.presetQuickButtons == nil {
		a.presetQuickButtons = map[string]*widget.Clickable{}
	}
	if btn, ok := a.presetQuickButtons[id]; ok {
		return btn
	}
	btn := new(widget.Clickable)
	a.presetQuickButtons[id] = btn
	return btn
}

func (a *App) currentPresetSnapshot() sharedCompat.Preset {
	compression := 100
	if raw := strings.TrimSpace(a.outputCompressionInput.Text()); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			compression = value
		}
	}
	return sharedCompat.Preset{
		Name:              strings.TrimSpace(a.presetNameInput.Text()),
		Size:              strings.TrimSpace(a.size),
		Quality:           strings.TrimSpace(a.quality),
		OutputFormat:      strings.TrimSpace(a.format),
		NegativePrompt:    strings.TrimSpace(a.negativePromptInput.Text()),
		Background:        strings.TrimSpace(a.background),
		OutputCompression: &compression,
		InputFidelity:     strings.TrimSpace(a.inputFidelity),
		ImageStyle:        strings.TrimSpace(a.imageStyle),
		Moderation:        strings.TrimSpace(a.moderation),
		StyleTag:          strings.TrimSpace(a.styleTag),
		EditAutoAspectRes: strings.TrimSpace(a.editAutoAspectResolution),
		KernelRuntimeMode: normalizeKernelRuntimeMode(a.kernelRuntimeMode),
		BatchCount:        normalizeBatchCount(a.batchCount),
	}
}

func (a *App) selectedPresetEditableSummary() []string {
	if strings.TrimSpace(a.selectedPresetID) == "" {
		return nil
	}
	sizeLabel := strings.TrimSpace(a.size)
	if autoAspect := strings.TrimSpace(a.editAutoAspectResolution); autoAspect != "" {
		sizeLabel = "按源图比例 + " + strings.ToUpper(autoAspect)
	} else if label := sizeDisplayLabel(a.size); strings.TrimSpace(label) != "" {
		sizeLabel = label
	}
	items := compactNonEmpty([]string{
		sizeLabel,
		qualityDisplayLabel(a.quality),
		strings.ToUpper(strings.TrimSpace(a.format)),
		fmt.Sprintf("%d 张", normalizeBatchCount(a.batchCount)),
	})
	if styleTag := strings.TrimSpace(a.styleTag); styleTag != "" {
		items = append(items, "#"+styleChoiceLabel(styleTag))
	}
	return items
}

func (a *App) openPresetManager() {
	a.mu.Lock()
	a.promptHelperOpen = false
	a.presetPickerOpen = false
	if a.selectedPresetID == "" && len(a.presets) > 0 {
		a.selectedPresetID = strings.TrimSpace(a.presets[0].ID)
	}
	if a.selectedPresetID != "" {
		a.loadPresetDraftLocked(a.selectedPresetID)
	} else {
		a.presetNameInput.SetText(nextPresetName(a.presets))
	}
	a.presetManagerOpen = true
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closePresetManager() {
	a.mu.Lock()
	a.presetManagerOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) presetPickerAnchorRectFromButton(btn *widget.Clickable, size image.Point) (image.Rectangle, bool) {
	if btn == nil || size.X <= 0 || size.Y <= 0 {
		return image.Rectangle{}, false
	}
	history := btn.History()
	if len(history) == 0 {
		return image.Rectangle{}, false
	}
	press := history[len(history)-1]
	a.mu.Lock()
	global := a.lastGlobalPressPos
	if global == (image.Point{}) {
		global = a.lastGlobalPointer
	}
	a.mu.Unlock()
	minPt := global.Sub(press.Position)
	return image.Rectangle{Min: minPt, Max: minPt.Add(size)}, true
}

func (a *App) openPresetPicker(btn *widget.Clickable, size image.Point) {
	rect, ok := a.presetPickerAnchorRectFromButton(btn, size)
	if !ok {
		a.mu.Lock()
		rect = a.presetPickerAnchorRect
		a.mu.Unlock()
		if rect == (image.Rectangle{}) {
			rect = image.Rect(28, 168, 320, 214)
		}
	}
	a.mu.Lock()
	a.presetPickerAnchorRect = rect
	a.presetPickerOpen = true
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) closePresetPicker() {
	a.mu.Lock()
	a.presetPickerOpen = false
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) loadPresetDraftLocked(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, item := range a.presets {
		if strings.TrimSpace(item.ID) != id {
			continue
		}
		a.selectedPresetID = id
		a.presetNameInput.SetText(strings.TrimSpace(item.Name))
		a.presetSizeInput.SetText(strings.TrimSpace(item.Size))
		a.presetQualityInput.SetText(strings.TrimSpace(item.Quality))
		a.presetOutputFormatInput.SetText(strings.TrimSpace(item.OutputFormat))
		a.presetBatchCountInput.SetText(strconv.Itoa(normalizeBatchCount(item.BatchCount)))
		a.presetStyleTagInput.SetText(strings.TrimSpace(item.StyleTag))
		a.editAutoAspectResolution = strings.TrimSpace(item.EditAutoAspectRes)
		return true
	}
	return false
}

func (a *App) currentPresetDraftValues() (string, string, string, int, string) {
	size := strings.TrimSpace(a.presetSizeInput.Text())
	if size == "" {
		size = strings.TrimSpace(a.size)
	}
	quality := strings.TrimSpace(a.presetQualityInput.Text())
	if quality == "" {
		quality = strings.TrimSpace(a.quality)
	}
	outputFormat := strings.TrimSpace(a.presetOutputFormatInput.Text())
	if outputFormat == "" {
		outputFormat = strings.TrimSpace(a.format)
	}
	batchCount := normalizeBatchCount(a.batchCount)
	if raw := strings.TrimSpace(a.presetBatchCountInput.Text()); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			batchCount = normalizeBatchCount(value)
		}
	}
	styleTag := strings.TrimSpace(a.presetStyleTagInput.Text())
	if styleTag == "" {
		styleTag = strings.TrimSpace(a.styleTag)
	}
	return size, quality, outputFormat, batchCount, styleTag
}

func buildUpdatedPresetFromDraft(current sharedCompat.Preset, name string, size string, quality string, outputFormat string, batchCount int, styleTag string) sharedCompat.Preset {
	updated := current
	updated.Name = strings.TrimSpace(name)
	if strings.TrimSpace(size) != "" {
		updated.Size = strings.TrimSpace(size)
	}
	if strings.TrimSpace(quality) != "" {
		updated.Quality = strings.TrimSpace(quality)
	}
	if strings.TrimSpace(outputFormat) != "" {
		updated.OutputFormat = strings.TrimSpace(outputFormat)
	}
	updated.BatchCount = normalizeBatchCount(batchCount)
	updated.StyleTag = strings.TrimSpace(styleTag)
	return updated
}

func (a *App) startNewPresetDraft() {
	a.mu.Lock()
	a.selectedPresetID = ""
	a.presetNameInput.SetText(nextPresetName(a.presets))
	a.presetSizeInput.SetText(strings.TrimSpace(a.size))
	a.presetQualityInput.SetText(strings.TrimSpace(a.quality))
	a.presetOutputFormatInput.SetText(strings.TrimSpace(a.format))
	a.presetBatchCountInput.SetText(strconv.Itoa(normalizeBatchCount(a.batchCount)))
	a.presetStyleTagInput.SetText(strings.TrimSpace(a.styleTag))
	a.mu.Unlock()
	a.invalidateNow()
}

func (a *App) savePresetAsNew() {
	name := strings.TrimSpace(a.presetNameInput.Text())
	if name == "" {
		a.appendLog("预设名称不能为空")
		return
	}
	now := time.Now().UnixMilli()
	preset := a.currentPresetSnapshot()
	preset.ID = fmt.Sprintf("preset-%d", now)
	preset.Name = name
	var presets []sharedCompat.Preset
	err := gioCompat.UpdateState(func(state *sharedCompat.State) error {
		*state = sharedCompat.Normalize(*state)
		state.Settings.Presets = append([]sharedCompat.Preset{preset}, state.Settings.Presets...)
		state.UpdatedAt = now
		presets = append([]sharedCompat.Preset(nil), state.Settings.Presets...)
		return nil
	})
	if err != nil {
		a.appendLog("保存预设失败: " + err.Error())
		return
	}
	a.mu.Lock()
	a.setPresetsLocked(presets)
	a.selectedPresetID = preset.ID
	a.presetNameInput.SetText(preset.Name)
	a.mu.Unlock()
	a.appendLog("已保存预设: " + preset.Name)
	a.invalidateNow()
}

func (a *App) saveCurrentPresetQuick() {
	a.mu.Lock()
	a.selectedPresetID = ""
	a.mu.Unlock()
	a.presetNameInput.SetText(nextPresetName(a.presets))
	a.savePresetAsNew()
}

func (a *App) overwritePresetByID(targetID string) {
	a.mu.Lock()
	a.selectedPresetID = strings.TrimSpace(targetID)
	a.loadPresetDraftLocked(a.selectedPresetID)
	a.mu.Unlock()
	a.overwriteSelectedPreset()
}

func (a *App) overwriteSelectedPreset() {
	targetID := ""
	a.mu.Lock()
	targetID = strings.TrimSpace(a.selectedPresetID)
	a.mu.Unlock()
	if targetID == "" {
		a.appendLog("当前没有可覆盖的预设")
		return
	}
	nameDraft := strings.TrimSpace(a.presetNameInput.Text())
	size, quality, outputFormat, batchCount, styleTag := a.currentPresetDraftValues()
	updated := false
	name := ""
	var presets []sharedCompat.Preset
	err := gioCompat.UpdateState(func(state *sharedCompat.State) error {
		*state = sharedCompat.Normalize(*state)
		for i := range state.Settings.Presets {
			if strings.TrimSpace(state.Settings.Presets[i].ID) != targetID {
				continue
			}
			name = strings.TrimSpace(state.Settings.Presets[i].Name)
			if nameDraft != "" {
				name = nameDraft
			}
			state.Settings.Presets[i] = buildUpdatedPresetFromDraft(state.Settings.Presets[i], name, size, quality, outputFormat, batchCount, styleTag)
			updated = true
			break
		}
		if updated {
			state.UpdatedAt = time.Now().UnixMilli()
		}
		presets = append([]sharedCompat.Preset(nil), state.Settings.Presets...)
		return nil
	})
	if err != nil {
		a.appendLog("更新预设失败: " + err.Error())
		return
	}
	if !updated {
		a.appendLog("当前预设不存在")
		return
	}
	a.mu.Lock()
	a.setPresetsLocked(presets)
	a.selectedPresetID = targetID
	a.presetNameInput.SetText(name)
	a.mu.Unlock()
	a.appendLog("已更新预设: " + name)
	a.invalidateNow()
}

func (a *App) deleteSelectedPreset() {
	targetID := ""
	a.mu.Lock()
	targetID = strings.TrimSpace(a.selectedPresetID)
	a.mu.Unlock()
	if targetID == "" {
		a.appendLog("当前没有可删除的预设")
		return
	}
	var next []sharedCompat.Preset
	removedName := ""
	removed := false
	err := gioCompat.UpdateState(func(state *sharedCompat.State) error {
		*state = sharedCompat.Normalize(*state)
		next = make([]sharedCompat.Preset, 0, len(state.Settings.Presets))
		for _, item := range state.Settings.Presets {
			if strings.TrimSpace(item.ID) == targetID {
				removedName = strings.TrimSpace(item.Name)
				removed = true
				continue
			}
			next = append(next, item)
		}
		if removed {
			state.Settings.Presets = next
			state.UpdatedAt = time.Now().UnixMilli()
		}
		return nil
	})
	if err != nil {
		a.appendLog("删除预设失败: " + err.Error())
		return
	}
	if !removed {
		a.appendLog("当前预设不存在")
		return
	}
	a.mu.Lock()
	a.setPresetsLocked(next)
	if len(next) > 0 {
		a.selectedPresetID = strings.TrimSpace(next[0].ID)
		a.presetNameInput.SetText(strings.TrimSpace(next[0].Name))
	} else {
		a.selectedPresetID = ""
		a.presetNameInput.SetText(nextPresetName(nil))
	}
	a.mu.Unlock()
	if removedName == "" {
		removedName = targetID
	}
	a.appendLog("已删除预设: " + removedName)
	a.invalidateNow()
}

func (a *App) applySelectedPreset() {
	targetID := ""
	a.mu.Lock()
	targetID = strings.TrimSpace(a.selectedPresetID)
	a.mu.Unlock()
	if !a.applyPresetByID(targetID) {
		a.appendLog("当前预设不存在")
	}
}

func (a *App) layoutPresetManagerModal(gtx layout.Context, snap snapshot) layout.Dimensions {
	for a.closePresetManagerButton.Clicked(gtx) {
		a.closePresetManager()
	}
	for a.newPresetButton.Clicked(gtx) {
		a.startNewPresetDraft()
	}
	for a.savePresetButton.Clicked(gtx) {
		a.savePresetAsNew()
	}
	for a.overwritePresetButton.Clicked(gtx) {
		a.overwriteSelectedPreset()
	}
	for a.applyPresetButton.Clicked(gtx) {
		a.applySelectedPreset()
	}
	for a.deletePresetButton.Clicked(gtx) {
		a.deleteSelectedPreset()
	}
	for _, item := range snap.Presets {
		btn := a.presetListButton("preset-list:" + item.ID)
		for btn.Clicked(gtx) {
			a.mu.Lock()
			a.loadPresetDraftLocked(item.ID)
			a.mu.Unlock()
			a.invalidateNow()
		}
	}

	return a.layoutStandardModal(
		gtx,
		unit.Dp(900),
		unit.Dp(620),
		"参数预设",
		"",
		&a.closePresetManagerButton,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(12))}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return fixedWidth(gtx, unit.Dp(280), func(gtx layout.Context) layout.Dimensions {
						return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
							return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
								children := []layout.FlexChild{
									layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
											layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
												return a.compactIconTextButton(gtx, &a.newPresetButton, uiIconAdd, "新建", false)
											}),
											layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
												return a.compactIconTextButton(gtx, &a.applyPresetButton, uiIconCheck, "应用", true)
											}),
										)
									}),
									layout.Rigid(layout.Spacer{Height: unit.Dp(10)}.Layout),
								}
								if len(snap.Presets) == 0 {
									children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
										return a.label(gtx, "还没有保存参数预设。先调好当前参数，再保存一条。", unit.Sp(11), fluent.textDim, font.Normal)
									}))
								} else {
									items := a.presetLabelsCached(snap.Presets)
									for idx, item := range snap.Presets {
										item := item
										summary := items[idx]
										children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											selected := strings.TrimSpace(item.ID) == strings.TrimSpace(a.selectedPresetID)
											return layout.Inset{Bottom: unit.Dp(6)}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
												return a.surfaceButton(
													gtx,
													a.presetListButton("preset-list:"+item.ID),
													chooseColor(selected, fluent.accentSoft, rgba(0xffffff, 0x00)),
													chooseColor(selected, accentAlpha(0x18), fluent.surface2),
													chooseColor(selected, accentAlpha(0x38), fluent.border),
													fluentCardRadius,
													layout.Inset{Top: 9, Bottom: 9, Left: 10, Right: 10},
													func(gtx layout.Context) layout.Dimensions {
														return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(3))}.Layout(gtx,
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return a.singleLineLabel(gtx, strings.TrimSpace(summary.Title), unit.Sp(11), chooseColor(selected, fluent.accent, fluent.text), font.SemiBold)
															}),
															layout.Rigid(func(gtx layout.Context) layout.Dimensions {
																return a.singleLineLabel(gtx, strings.TrimSpace(summary.Detail), unit.Sp(10), fluent.textDim, font.Normal)
															}),
														)
													},
												)
											})
										}))
									}
								}
								return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
							})
						})
					})
				}),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return a.borderedSurface(gtx, fluent.surface, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
						return layout.UniformInset(unit.Dp(12)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(10))}.Layout(gtx,
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									title := "保存当前参数"
									if strings.TrimSpace(a.selectedPresetID) != "" {
										title = "当前选中预设"
									}
									return a.label(gtx, title, unit.Sp(12), fluent.text, font.SemiBold)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.field(gtx, "预设名称", &a.presetNameInput, "配置1", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(a.selectedPresetID) == "" {
										return layout.Dimensions{}
									}
									return a.field(gtx, "尺寸", &a.presetSizeInput, "1024x1024 / auto", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(a.selectedPresetID) == "" {
										return layout.Dimensions{}
									}
									return a.field(gtx, "质量", &a.presetQualityInput, "auto / low / medium / high", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(a.selectedPresetID) == "" {
										return layout.Dimensions{}
									}
									return a.field(gtx, "输出格式", &a.presetOutputFormatInput, "png / jpeg / webp", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(a.selectedPresetID) == "" {
										return layout.Dimensions{}
									}
									return a.field(gtx, "出图张数", &a.presetBatchCountInput, "1-9", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(a.selectedPresetID) == "" {
										return layout.Dimensions{}
									}
									return a.field(gtx, "风格", &a.presetStyleTagInput, "默认 / anime / cyberpunk...", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									lines := a.selectedPresetEditableSummary()
									if len(lines) == 0 {
										return a.label(gtx, "选中左侧预设后，可直接修改名称、尺寸、质量、输出格式、风格和张数。", unit.Sp(10), fluent.textDim, font.Normal)
									}
									rows := make([]layout.FlexChild, 0, len(lines))
									for _, line := range lines {
										line := line
										rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return a.label(gtx, line, unit.Sp(10), fluent.textDim, font.Normal)
										}))
									}
									return a.borderedSurface(gtx, fluent.surface2, fluentCardRadius, fluent.border, func(gtx layout.Context) layout.Dimensions {
										return layout.UniformInset(unit.Dp(10)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
											return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(4))}.Layout(gtx, rows...)
										})
									})
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(a.selectedPresetID) == "" {
										return layout.Dimensions{}
									}
									return a.field(gtx, "尺寸", &a.presetSizeInput, "1024x1024 / auto", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(a.selectedPresetID) == "" {
										return layout.Dimensions{}
									}
									return a.field(gtx, "质量", &a.presetQualityInput, "auto / low / medium / high", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(a.selectedPresetID) == "" {
										return layout.Dimensions{}
									}
									return a.field(gtx, "输出格式", &a.presetOutputFormatInput, "png / jpeg / webp", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(a.selectedPresetID) == "" {
										return layout.Dimensions{}
									}
									return a.field(gtx, "出图张数", &a.presetBatchCountInput, "1-9", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									if strings.TrimSpace(a.selectedPresetID) == "" {
										return layout.Dimensions{}
									}
									return a.field(gtx, "风格", &a.presetStyleTagInput, "默认 / anime / cyberpunk...", unit.Dp(42))
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.compactIconTextButton(gtx, &a.savePresetButton, uiIconSave, "另存新预设", false)
										}),
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.compactIconTextButton(gtx, &a.overwritePresetButton, uiIconRefresh, "覆盖当前预设", true)
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return layout.Flex{Axis: layout.Horizontal, Gap: gtx.Dp(unit.Dp(8))}.Layout(gtx,
										layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
											return a.compactIconTextButton(gtx, &a.applyPresetButton, uiIconCheck, "应用到当前参数", true)
										}),
										layout.Rigid(func(gtx layout.Context) layout.Dimensions {
											return fixedWidth(gtx, unit.Dp(124), func(gtx layout.Context) layout.Dimensions {
												return a.compactIconTextButton(gtx, &a.deletePresetButton, uiIconDelete, "删除", false)
											})
										}),
									)
								}),
								layout.Rigid(func(gtx layout.Context) layout.Dimensions {
									return a.label(gtx, fmt.Sprintf("当前已保存 %d 条共享预设，WebView 与 Gio 会共用这份状态。", len(snap.Presets)), unit.Sp(10), fluent.textDim, font.Normal)
								}),
							)
						})
					})
				}),
			)
		},
	)
}

func chooseValueOrFallback(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
