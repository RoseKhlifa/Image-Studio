//go:build !darwin

package ui

import "gioui.org/font"

func platformFontCollection() []font.FontFace { return nil }
