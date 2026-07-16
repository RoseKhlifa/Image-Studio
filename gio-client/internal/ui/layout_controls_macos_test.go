package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func TestMacSwitchHasStableDimensionsAndStateSemantics(t *testing.T) {
	previous := installedDesktopTheme
	defer installDesktopThemeSpec(previous.Style, previous.ColorMode)
	installDesktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)

	app := &App{
		th:                material.NewTheme(),
		desktopStyle:      desktopStyleMacOS,
		resolvedThemeMode: desktopColorModeLight,
		fontScale:         1,
	}
	var (
		ops    op.Ops
		router input.Router
		button widget.Clickable
	)
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(100, 40)},
	}
	if got := app.layoutMacSwitch(gtx, &button, true, "循环出图").Size; got != image.Pt(34, 20) {
		t.Fatalf("Mac switch size=%v want 34x20", got)
	}
	router.Frame(&ops)
	assertSemanticSelected(t, router.AppendSemantics(nil), "循环出图", true)
}
