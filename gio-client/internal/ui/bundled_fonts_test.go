package ui

import (
	"runtime"
	"testing"

	"gioui.org/widget/material"
)

func TestDarwinSystemFontCollectionProvidesAppleFaces(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system SF fonts are a macOS runtime contract")
	}
	faces := platformFontCollection()
	if len(faces) == 0 {
		t.Fatal("macOS system SF font collection is empty")
	}
	want := map[string]bool{
		string(uiMacSansTypeface):  false,
		string(uiMacTitleTypeface): false,
		string(uiMacMonoTypeface):  false,
	}
	for _, face := range faces {
		if _, ok := want[string(face.Font.Typeface)]; ok {
			want[string(face.Font.Typeface)] = true
		}
	}
	for typeface, found := range want {
		if !found {
			t.Fatalf("macOS system collection is missing %q", typeface)
		}
	}
}

func TestBundledFontCollectionReturnsIndependentSlices(t *testing.T) {
	first := bundledFontCollection()
	if len(first) == 0 {
		t.Fatal("bundled font collection is empty")
	}
	original := first[0]
	first[0] = first[len(first)-1]
	second := bundledFontCollection()
	if second[0].Font != original.Font {
		t.Fatal("caller mutation leaked into cached font collection")
	}
}

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
