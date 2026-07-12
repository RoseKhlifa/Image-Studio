package ui

import (
	"testing"

	"gioui.org/widget/material"
)

func TestDesktopTypefacesTrackDesignLanguage(t *testing.T) {
	tests := []struct {
		style string
		sans  string
		title string
		mono  string
	}{
		{
			style: desktopStyleMacOS,
			sans:  string(uiMacSansTypeface),
			title: string(uiMacTitleTypeface),
			mono:  string(uiMacMonoTypeface),
		},
		{
			style: desktopStyleWindows,
			sans:  string(uiSansTypeface),
			title: string(uiTitleTypeface),
			mono:  string(uiMonoTypeface),
		},
	}
	for _, testCase := range tests {
		t.Run(testCase.style, func(t *testing.T) {
			if got := string(desktopSansTypeface(testCase.style)); got != testCase.sans {
				t.Fatalf("sans typeface=%q want %q", got, testCase.sans)
			}
			if got := string(desktopTitleTypeface(testCase.style)); got != testCase.title {
				t.Fatalf("title typeface=%q want %q", got, testCase.title)
			}
			if got := string(desktopMonoTypeface(testCase.style)); got != testCase.mono {
				t.Fatalf("mono typeface=%q want %q", got, testCase.mono)
			}
		})
	}
}

func TestInstallDesktopStyleUpdatesMainMaterialTypeface(t *testing.T) {
	previous := installedDesktopTheme
	defer installDesktopThemeSpec(previous.Style, previous.ColorMode)

	app := &App{
		th:                material.NewTheme(),
		resolvedThemeMode: desktopColorModeLight,
	}
	app.installDesktopStyle(desktopStyleMacOS)
	if app.th.Face != uiMacSansTypeface {
		t.Fatalf("macOS material face=%q want %q", app.th.Face, uiMacSansTypeface)
	}
	app.installDesktopStyle(desktopStyleWindows)
	if app.th.Face != uiSansTypeface {
		t.Fatalf("Windows material face=%q want %q", app.th.Face, uiSansTypeface)
	}
}

func TestDetachedWindowThemeUsesSelectedDesignLanguage(t *testing.T) {
	view := &desktopWindowView{theme: material.NewTheme()}
	view.applyTheme(desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight))
	if view.desktopStyle != desktopStyleMacOS || view.theme.Face != uiMacSansTypeface {
		t.Fatalf("macOS detached theme style=%q face=%q", view.desktopStyle, view.theme.Face)
	}
	view.applyTheme(desktopThemeSpec(desktopStyleWindows, desktopColorModeDark))
	if view.desktopStyle != desktopStyleWindows || view.theme.Face != uiSansTypeface {
		t.Fatalf("Windows detached theme style=%q face=%q", view.desktopStyle, view.theme.Face)
	}
}
