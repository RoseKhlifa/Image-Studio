package ui

import (
	_ "embed"

	"gioui.org/font"
	"gioui.org/font/opentype"
)

const (
	uiFallbackSansTypeface = font.Typeface("HarmonyOS Sans SC")
	uiFallbackMonoTypeface = font.Typeface("JetBrains Mono")
	uiSansTypeface         = font.Typeface(`"Segoe UI Variable Text", "Segoe UI", "HarmonyOS Sans SC"`)
	uiTitleTypeface        = font.Typeface(`"Segoe UI Variable Display", "Segoe UI", "HarmonyOS Sans SC"`)
	uiMonoTypeface         = font.Typeface(`"Cascadia Code", "JetBrains Mono", Consolas`)
	uiMacSansTypeface      = font.Typeface(`"SF Pro Text", "Helvetica Neue", "HarmonyOS Sans SC"`)
	uiMacTitleTypeface     = font.Typeface(`"SF Pro Display", "Helvetica Neue", "HarmonyOS Sans SC"`)
	uiMacMonoTypeface      = font.Typeface(`"SF Mono", "JetBrains Mono", Menlo, monospace`)
)

//go:embed assets/HarmonyOS_SansSC_Regular.ttf
var harmonySansSC []byte

//go:embed assets/JetBrainsMono-Regular.ttf
var jetBrainsMono []byte

func bundledFontCollection() []font.FontFace {
	out := make([]font.FontFace, 0, 4)
	out = append(out, parseBundledFont(harmonySansSC, uiFallbackSansTypeface)...)
	out = append(out, parseBundledFont(jetBrainsMono, uiFallbackMonoTypeface)...)
	return out
}

func parseBundledFont(src []byte, typeface font.Typeface) []font.FontFace {
	faces, err := opentype.ParseCollection(src)
	if err != nil {
		return nil
	}
	for idx := range faces {
		faces[idx].Font.Typeface = typeface
	}
	return faces
}

func desktopSansTypeface(style string) font.Typeface {
	if normalizeDesktopStyle(style) == desktopStyleMacOS {
		return uiMacSansTypeface
	}
	return uiSansTypeface
}

func desktopTitleTypeface(style string) font.Typeface {
	if normalizeDesktopStyle(style) == desktopStyleMacOS {
		return uiMacTitleTypeface
	}
	return uiTitleTypeface
}

func desktopMonoTypeface(style string) font.Typeface {
	if normalizeDesktopStyle(style) == desktopStyleMacOS {
		return uiMacMonoTypeface
	}
	return uiMonoTypeface
}
