package ui

import (
	"image/color"

	"gioui.org/unit"
	"gioui.org/widget/material"
)

type fluentColors struct {
	accent          color.NRGBA
	accent2         color.NRGBA
	accentSoft      color.NRGBA
	bg              color.NRGBA
	bg2             color.NRGBA
	panel           color.NRGBA
	panel2          color.NRGBA
	surface         color.NRGBA
	surface2        color.NRGBA
	surfaceElevated color.NRGBA
	sidebar         color.NRGBA
	inspector       color.NRGBA
	toolbar         color.NRGBA
	border          color.NRGBA
	border2         color.NRGBA
	text            color.NRGBA
	textMuted       color.NRGBA
	textDim         color.NRGBA
	cardShadow      color.NRGBA
	cardGlow        color.NRGBA
	bgGlow          color.NRGBA
	canvasBg        color.NRGBA
	canvasTile      color.NRGBA
	success         color.NRGBA
	danger          color.NRGBA
	dangerSoft      color.NRGBA
	toolHoverBg     color.NRGBA
	toolHoverText   color.NRGBA
	windowOutline   color.NRGBA
	white           color.NRGBA
}

var fluentLight = fluentColors{
	accent:          rgb(0x005fb8),
	accent2:         rgb(0x0a6fcb),
	accentSoft:      rgba(0x005fb8, 0x1f),
	bg:              rgb(0xf3f3f3),
	bg2:             rgb(0xe9e9e9),
	panel:           rgb(0xfbfbfb),
	panel2:          rgb(0xf6f6f6),
	surface:         rgb(0xffffff),
	surface2:        rgb(0xf2f2f2),
	surfaceElevated: rgb(0xfcfcfc),
	sidebar:         rgb(0xfbfbfb),
	inspector:       rgb(0xf8f8f8),
	toolbar:         rgb(0xf7f7f7),
	border:          rgba(0x000000, 0x14),
	border2:         rgba(0x000000, 0x24),
	text:            rgb(0x1f1f1f),
	textMuted:       rgba(0x1f1f1f, 0xb8),
	textDim:         rgba(0x1f1f1f, 0x8a),
	cardShadow:      rgba(0x000000, 0x12),
	cardGlow:        rgba(0xffffff, 0x05),
	bgGlow:          rgba(0xffffff, 0x28),
	canvasBg:        rgb(0xeeeeee),
	canvasTile:      rgb(0xdedede),
	success:         rgb(0x0f7b0f),
	danger:          rgb(0xc42b1c),
	dangerSoft:      rgba(0xc42b1c, 0x1f),
	toolHoverBg:     rgba(0x000000, 0x0a),
	toolHoverText:   rgb(0x1f1f1f),
	windowOutline:   rgba(0xffffff, 0x38),
	white:           rgb(0xffffff),
}

var fluentDark = fluentColors{
	accent:          rgb(0x60cdff),
	accent2:         rgb(0x8bdcff),
	accentSoft:      rgba(0x60cdff, 0x29),
	bg:              rgb(0x202020),
	bg2:             rgb(0x1c1c1c),
	panel:           rgb(0x2b2b2b),
	panel2:          rgb(0x282828),
	surface:         rgb(0x333333),
	surface2:        rgb(0x3b3b3b),
	surfaceElevated: rgb(0x333333),
	sidebar:         rgb(0x2b2b2b),
	inspector:       rgb(0x292929),
	toolbar:         rgb(0x2d2d2d),
	border:          rgba(0xffffff, 0x14),
	border2:         rgba(0xffffff, 0x24),
	text:            rgb(0xf5f5f5),
	textMuted:       rgba(0xf3f3f3, 0xb8),
	textDim:         rgba(0xf3f3f3, 0x80),
	cardShadow:      rgba(0x000000, 0x00),
	cardGlow:        rgba(0xffffff, 0x00),
	bgGlow:          rgba(0xffffff, 0x04),
	canvasBg:        rgb(0x242424),
	canvasTile:      rgb(0x343434),
	success:         rgb(0x6ccb5f),
	danger:          rgb(0xff99a4),
	dangerSoft:      rgba(0xff99a4, 0x1f),
	toolHoverBg:     rgba(0xffffff, 0x0f),
	toolHoverText:   rgb(0xf3f3f3),
	windowOutline:   rgba(0xffffff, 0x00),
	white:           rgb(0xffffff),
}

var fluent = fluentLight
var systemThemeResolver = systemThemeMode

func themePalette(mode string) fluentColors {
	if mode == "dark" {
		return fluentDark
	}
	return fluentLight
}

func normalizeThemeMode(mode string) string {
	switch mode {
	case "dark", "light", "system":
		return mode
	default:
		return "system"
	}
}

func resolveThemeMode(mode string) string {
	switch normalizeThemeMode(mode) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	}
	if systemThemeResolver() == "dark" {
		return "dark"
	}
	return "light"
}

func (a *App) isDarkTheme() bool {
	if a == nil {
		return false
	}
	if a.resolvedThemeMode != "" {
		return a.resolvedThemeMode == "dark"
	}
	return normalizeThemeMode(a.themeMode) == "dark"
}

func normalizeFontScale(scale float64) float64 {
	switch {
	case scale <= 0:
		return 1
	case scale < 0.9:
		return 0.85
	case scale > 1.08:
		return 1.15
	default:
		return 1
	}
}

func (a *App) scaledSp(size unit.Sp) unit.Sp {
	scale := float32(1)
	if a != nil && a.fontScale > 0 {
		scale = float32(a.fontScale)
	}
	return unit.Sp(float32(size) * scale)
}

func (a *App) applyFontScale(scale float64) {
	a.fontScale = normalizeFontScale(scale)
	a.th.TextSize = a.scaledSp(unit.Sp(14))
	a.invalidateNow()
}

func (a *App) applyThemeMode(mode string) {
	a.themeMode = normalizeThemeMode(mode)
	a.installResolvedTheme(resolveThemeMode(a.themeMode))
	a.invalidateNow()
}

func (a *App) refreshSystemTheme() {
	if a == nil || normalizeThemeMode(a.themeMode) != "system" {
		return
	}
	resolved := resolveThemeMode("system")
	if resolved == a.resolvedThemeMode {
		return
	}
	a.installResolvedTheme(resolved)
	a.invalidateNow()
}

func (a *App) installResolvedTheme(mode string) {
	if mode != "dark" {
		mode = "light"
	}
	a.resolvedThemeMode = mode
	fluent = themePalette(mode)
	if a.th == nil {
		return
	}
	a.th.Palette = material.Palette{
		Bg:         fluent.bg,
		Fg:         fluent.text,
		ContrastBg: fluent.accent,
		ContrastFg: fluent.white,
	}
}

func rgb(v uint32) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff}
}

func rgba(v uint32, alpha uint8) color.NRGBA {
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: alpha}
}

func withAlpha(base color.NRGBA, alpha uint8) color.NRGBA {
	base.A = alpha
	return base
}

func accentAlpha(alpha uint8) color.NRGBA {
	return withAlpha(fluent.accent, alpha)
}

func dangerAlpha(alpha uint8) color.NRGBA {
	return withAlpha(fluent.danger, alpha)
}
