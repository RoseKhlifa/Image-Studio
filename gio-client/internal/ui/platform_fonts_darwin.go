//go:build darwin

package ui

import (
	"os"

	"gioui.org/font"
)

func platformFontCollection() []font.FontFace {
	out := make([]font.FontFace, 0, 10)
	if source := firstReadableFont(
		"/System/Library/Fonts/SFNS.ttf",
		"/System/Library/Fonts/SFNS.otf",
	); len(source) > 0 {
		out = append(out, parseBundledFont(source, uiMacSansTypeface)...)
		out = append(out, parseBundledFont(source, uiMacTitleTypeface)...)
	}
	if source := firstReadableFont(
		"/System/Library/Fonts/SFNSMono.ttf",
		"/System/Library/Fonts/SFNSMono.otf",
	); len(source) > 0 {
		out = append(out, parseBundledFont(source, uiMacMonoTypeface)...)
	}
	return out
}

func firstReadableFont(paths ...string) []byte {
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err == nil && len(source) > 0 {
			return source
		}
	}
	return nil
}
