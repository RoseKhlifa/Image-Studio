package ui

import (
	"image/color"
	"math"

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
	accentText      color.NRGBA
	successText     color.NRGBA
	warningText     color.NRGBA
	dangerText      color.NRGBA
	cardShadow      color.NRGBA
	cardGlow        color.NRGBA
	bgGlow          color.NRGBA
	canvasBg        color.NRGBA
	canvasTile      color.NRGBA
	success         color.NRGBA
	warning         color.NRGBA
	warningSoft     color.NRGBA
	danger          color.NRGBA
	dangerSoft      color.NRGBA
	focusRing       color.NRGBA
	toolHoverBg     color.NRGBA
	toolHoverText   color.NRGBA
	windowOutline   color.NRGBA
	white           color.NRGBA
}

var fluentLight = windowsDesktopColors(desktopColorModeLight)
var fluentDark = windowsDesktopColors(desktopColorModeDark)
var fluent = fluentLight
var systemThemeResolver = systemThemeMode

func themePalette(mode string) fluentColors {
	return desktopThemeSpec(currentDesktopStyle(), mode).Colors
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
	spec := installDesktopThemeSpec(currentDesktopStyle(), mode)
	a.resolvedThemeMode = spec.ColorMode
	a.installMaterialThemePalette(spec.Colors)
}

// installDesktopStyle switches the desktop design language without changing
// the selected light/dark appearance. Persistence is owned by the caller.
func (a *App) installDesktopStyle(style string) desktopThemeTokens {
	mode := a.resolvedThemeMode
	if mode == "" {
		mode = resolveThemeMode(a.themeMode)
	}
	spec := installDesktopThemeSpec(style, mode)
	a.resolvedThemeMode = spec.ColorMode
	a.installMaterialThemePalette(spec.Colors)
	if a.th != nil {
		a.th.Face = desktopSansTypeface(spec.Style)
	}
	return spec
}

func (a *App) installMaterialThemePalette(colors fluentColors) {
	if a.th == nil {
		return
	}
	a.th.Palette = material.Palette{
		Bg:         colors.bg,
		Fg:         colors.text,
		ContrastBg: colors.accent,
		ContrastFg: desktopReadableText(colors.accent),
	}
}

// desktopReadableText returns the black or white foreground with the higher
// WCAG contrast against an opaque desktop control fill.
func desktopReadableText(background color.NRGBA) color.NRGBA {
	luminance := desktopRelativeLuminance(background)
	blackContrast := (luminance + 0.05) / 0.05
	whiteContrast := 1.05 / (luminance + 0.05)
	if blackContrast >= whiteContrast {
		return rgb(0x000000)
	}
	return rgb(0xffffff)
}

func desktopRelativeLuminance(value color.NRGBA) float64 {
	linear := func(component uint8) float64 {
		channel := float64(component) / 255
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(value.R) + 0.7152*linear(value.G) + 0.0722*linear(value.B)
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
