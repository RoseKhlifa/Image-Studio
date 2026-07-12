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

func TestMacCanvasChromeVisibility(t *testing.T) {
	if showMacCanvasToolbar(false, 0) {
		t.Fatal("empty Apple canvas must not reserve toolbar space")
	}
	if !showMacCanvasToolbar(true, 0) || !showMacCanvasToolbar(false, 2) {
		t.Fatal("Apple canvas toolbar must appear for a result or batch navigation")
	}

	idle := macCanvasStatusVisibilityFor(false, false, false)
	if idle.show || idle.showMetadata || idle.showZoom {
		t.Fatalf("idle Apple status visibility=%+v", idle)
	}
	running := macCanvasStatusVisibilityFor(true, false, true)
	if !running.show || running.showMetadata || running.showZoom {
		t.Fatalf("running Apple status visibility=%+v", running)
	}
	result := macCanvasStatusVisibilityFor(false, false, true)
	if !result.show || !result.showMetadata || !result.showZoom {
		t.Fatalf("result Apple status visibility=%+v", result)
	}
}

func TestMacCanvasChromeDimensions(t *testing.T) {
	previousTheme := installedDesktopTheme
	defer installDesktopThemeSpec(previousTheme.Style, previousTheme.ColorMode)
	spec := installDesktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	app := &App{
		th:                 material.NewTheme(),
		desktopStyle:       desktopStyleMacOS,
		resolvedThemeMode:  desktopColorModeLight,
		fontScale:          1,
		canvasDisplayScale: 1,
		canvasTool:         canvasToolPan,
	}

	newContext := func(width, height int) layout.Context {
		return layout.Context{
			Ops:         new(op.Ops),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Constraints{Max: image.Pt(width, height)},
		}
	}
	if dims := app.layoutMacCanvasToolbar(newContext(1400, 300), snapshot{}, macCanvasToolbarState{}); dims.Size != (image.Point{}) {
		t.Fatalf("hidden Apple toolbar size=%v", dims.Size)
	}
	toolbar := app.layoutMacCanvasToolbar(newContext(1400, 300), snapshot{}, macCanvasToolbarState{
		hasCanvasResult: true,
		currentTool:     canvasToolPan,
	})
	if toolbar.Size.X != 1400 || toolbar.Size.Y < int(spec.Metrics.ControlHeight*2) {
		t.Fatalf("Apple toolbar size=%v want full width and two control rows", toolbar.Size)
	}
	compactToolbar := app.layoutMacCanvasToolbar(newContext(360, 300), snapshot{}, macCanvasToolbarState{
		hasCanvasResult: true,
		currentTool:     canvasToolPan,
		canTransform:    true,
	})
	if compactToolbar.Size.X != 360 {
		t.Fatalf("compact Apple toolbar width=%d want 360", compactToolbar.Size.X)
	}
	for index := range app.macCanvasToolbarLists {
		if app.macCanvasToolbarLists[index].List.Axis != layout.Horizontal || !app.macCanvasToolbarLists[index].List.ScrollAnyAxis {
			t.Fatalf("Apple toolbar list %d is not horizontally scrollable", index)
		}
	}
	if dims := app.layoutMacCanvasStatusBar(newContext(800, 100), snapshot{}); dims.Size != (image.Point{}) {
		t.Fatalf("idle Apple status size=%v", dims.Size)
	}
	running := app.layoutMacCanvasStatusBar(newContext(800, 100), snapshot{Running: true, Status: "正在生成"})
	if running.Size.X != 800 || running.Size.Y <= 0 || running.Size.Y >= 60 {
		t.Fatalf("running Apple status size=%v", running.Size)
	}
}

func TestMacCanvasToolbarControlSemantics(t *testing.T) {
	previousTheme := installedDesktopTheme
	defer installDesktopThemeSpec(previousTheme.Style, previousTheme.ColorMode)
	spec := installDesktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	var (
		ops        op.Ops
		router     input.Router
		selected   widget.Clickable
		unselected widget.Clickable
		disabled   widget.Clickable
	)
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(420, 120)),
	}
	app := &App{th: material.NewTheme(), desktopStyle: desktopStyleMacOS, resolvedThemeMode: desktopColorModeLight, fontScale: 1}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.macCanvasToolbarButton(gtx, spec, &selected, uiIconPanTool, "移动", true, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.macCanvasToolbarButton(gtx, spec, &unselected, uiIconBrush, "蒙版", false, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.macCanvasToolbarButton(gtx, spec, &disabled, uiIconUndo, "撤销", false, true)
		}),
	)
	router.Frame(&ops)
	nodes := router.AppendSemantics(nil)
	assertSemanticSelected(t, nodes, "移动", true)
	assertSemanticSelected(t, nodes, "蒙版", false)
	undo, ok := semanticTreeButtonByLabel(nodes, "撤销")
	if !ok {
		t.Fatalf("disabled Apple canvas control is missing from semantics: %#v", nodes)
	}
	if !undo.Desc.Disabled {
		t.Fatal("disabled Apple canvas control is not exposed as disabled")
	}
}
