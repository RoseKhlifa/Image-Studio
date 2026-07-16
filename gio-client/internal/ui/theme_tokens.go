package ui

import (
	"runtime"
	"strings"

	"gioui.org/unit"
)

const (
	desktopStyleMacOS   = "macos"
	desktopStyleWindows = "windows"

	desktopColorModeLight = "light"
	desktopColorModeDark  = "dark"

	desktopSurfaceSolid       = "solid"
	desktopSurfaceLiquidGlass = "liquid-glass"
)

type desktopThemeMetrics struct {
	HeaderHeight       unit.Dp
	WorkspaceBarHeight unit.Dp
	CommandBarHeight   unit.Dp
	StatusBarHeight    unit.Dp
	ControlHeight      unit.Dp
	InputHeight        unit.Dp
	IconTargetSize     unit.Dp
	RowHeight          unit.Dp
	LeftPaneWidth      unit.Dp
	RightPaneWidth     unit.Dp
	ConsoleHeight      unit.Dp
	NodeWidth          unit.Dp
	ControlRadius      unit.Dp
	CardRadius         unit.Dp
	BadgeRadius        unit.Dp
	ModalRadius        unit.Dp
	InputRadius        unit.Dp
	NodeRadius         unit.Dp
}

// desktopThemeTokens is a value-only theme description. Callers receive a copy,
// so inspecting or modifying a result cannot mutate the canonical theme tables.
type desktopThemeTokens struct {
	Style            string
	ColorMode        string
	SurfaceTreatment string
	Colors           fluentColors
	Metrics          desktopThemeMetrics
}

var installedDesktopTheme desktopThemeTokens

func init() {
	installDesktopThemeSpec("", desktopColorModeLight)
}

func normalizeDesktopStyle(style string) string {
	return normalizeDesktopStyleForGOOS(style, runtime.GOOS)
}

func normalizeDesktopStyleForGOOS(style string, goos string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case desktopStyleMacOS, "mac", "apple":
		return desktopStyleMacOS
	case desktopStyleWindows, "win", "fluent":
		return desktopStyleWindows
	}
	if strings.EqualFold(strings.TrimSpace(goos), "darwin") {
		return desktopStyleMacOS
	}
	return desktopStyleWindows
}

func normalizeDesktopColorMode(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), desktopColorModeDark) {
		return desktopColorModeDark
	}
	return desktopColorModeLight
}

// desktopThemeSpec resolves a complete, immutable-by-copy desktop theme.
func desktopThemeSpec(style string, colorMode string) desktopThemeTokens {
	style = normalizeDesktopStyle(style)
	colorMode = normalizeDesktopColorMode(colorMode)
	if style == desktopStyleMacOS {
		return macOSDesktopThemeSpec(colorMode)
	}
	return windowsDesktopThemeSpec(colorMode)
}

// installDesktopThemeSpec keeps the existing fluent compatibility variables in
// sync while layouts migrate incrementally to semantic desktop theme tokens.
func installDesktopThemeSpec(style string, colorMode string) desktopThemeTokens {
	spec := desktopThemeSpec(style, colorMode)
	installedDesktopTheme = spec
	fluent = spec.Colors
	installDesktopThemeMetrics(spec.Metrics)
	return spec
}

func currentDesktopStyle() string {
	if installedDesktopTheme.Style == "" {
		return normalizeDesktopStyle("")
	}
	return installedDesktopTheme.Style
}

func currentDesktopThemeMetrics() desktopThemeMetrics {
	if installedDesktopTheme.Style == "" {
		return desktopThemeSpec("", desktopColorModeLight).Metrics
	}
	return installedDesktopTheme.Metrics
}
