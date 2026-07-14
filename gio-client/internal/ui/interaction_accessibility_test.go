package ui

import (
	"image"
	"image/color"
	"testing"

	"image-studio/gio-client/internal/windowing"

	"gioui.org/io/input"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func TestSurfaceAndElevatedButtonInteractionColors(t *testing.T) {
	background := color.NRGBA{R: 0x20, G: 0x24, B: 0x2a, A: 0xff}
	hover := color.NRGBA{R: 0x34, G: 0x3a, B: 0x42, A: 0xff}
	border := color.NRGBA{R: 0x50, G: 0x55, B: 0x5d, A: 0xff}
	focus := color.NRGBA{R: 0x00, G: 0x78, B: 0xd4, A: 0xff}
	text := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}

	defaultState := resolveButtonInteractionColors(background, hover, border, focus, text, buttonInteractionState{})
	if defaultState.Fill != background || defaultState.Border != border {
		t.Fatalf("default colors=%+v", defaultState)
	}
	focused := resolveButtonInteractionColors(background, hover, border, focus, text, buttonInteractionState{Focused: true})
	if focused.Fill != hover || focused.Border != focus {
		t.Fatalf("focused colors=%+v want hover fill and focus ring", focused)
	}
	pressed := resolveButtonInteractionColors(background, hover, border, focus, text, buttonInteractionState{Hovered: true, Pressed: true})
	if pressed.Fill == hover || pressed.Fill == background {
		t.Fatalf("pressed fill=%+v must be visibly distinct", pressed.Fill)
	}
	if pressed.Border != border {
		t.Fatalf("pressed border=%+v want %+v", pressed.Border, border)
	}
}

func TestDetachedButtonInteractionColors(t *testing.T) {
	spec := desktopThemeSpec(desktopStyleWindows, desktopColorModeLight)
	base := resolveDesktopButtonVisual(spec, desktopButtonNeutral, buttonInteractionState{})
	focused := resolveDesktopButtonVisual(spec, desktopButtonNeutral, buttonInteractionState{Focused: true})
	if focused.Border != spec.Colors.focusRing {
		t.Fatalf("focused border=%+v want %+v", focused.Border, spec.Colors.focusRing)
	}
	if focused.Fill == base.Fill {
		t.Fatalf("focused fill=%+v should differ from default %+v", focused.Fill, base.Fill)
	}
	pressed := resolveDesktopButtonVisual(spec, desktopButtonPrimary, buttonInteractionState{Pressed: true})
	if pressed.Fill == spec.Colors.accent {
		t.Fatalf("pressed primary fill=%+v should differ from accent", pressed.Fill)
	}
	if pressed.Text != desktopReadableText(pressed.Fill) {
		t.Fatalf("pressed primary text=%+v is not readable over %+v", pressed.Text, pressed.Fill)
	}
}

func TestDetachedStatusChipsSeparateTextAndFillTokens(t *testing.T) {
	spec := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	tests := []struct {
		name string
		text color.NRGBA
		fill color.NRGBA
	}{
		{name: "running", text: detachedStatusColor(spec, desktopPublication{Running: true}), fill: spec.Colors.accent},
		{name: "completed", text: detachedStatusColor(spec, desktopPublication{Completed: 1}), fill: spec.Colors.success},
		{name: "error", text: detachedStatusColor(spec, desktopPublication{LastError: "failed"}), fill: spec.Colors.danger},
		{name: "queued", text: detachedWorkspaceStatusColor(spec, desktopPublication{Workspaces: []desktopWorkspacePublication{{ID: "ws", Queued: true}}}, "ws"), fill: spec.Colors.warning},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.text == testCase.fill {
				t.Fatalf("status text reused raw system color %+v", testCase.text)
			}
			if got := detachedStatusChipFill(spec, testCase.text); got != testCase.fill {
				t.Fatalf("chip fill=%+v want %+v", got, testCase.fill)
			}
		})
	}
}

func TestWorkflowShellDisablesSimpleModeBackgroundEffects(t *testing.T) {
	if !(&App{desktopStyle: desktopStyleWindows}).shellEffectsEnabled() {
		t.Fatal("Windows simple mode should retain shell effects")
	}
	if (&App{desktopStyle: desktopStyleMacOS}).shellEffectsEnabled() {
		t.Fatal("macOS must use a solid shell background")
	}
	if (&App{experienceMode: experienceModeWorkflow}).shellEffectsEnabled() {
		t.Fatal("workflow mode must use a solid shell background")
	}
	if (&App{experienceMode: experienceModeSimple, reducedEffects: true}).shellEffectsEnabled() {
		t.Fatal("reduced effects must disable shell effects")
	}
}

func TestSurfaceEffectsFollowReducedEffectsPreference(t *testing.T) {
	if !(&App{desktopStyle: desktopStyleWindows}).surfaceEffectsEnabled() {
		t.Fatal("Windows surfaces should retain static highlights and shadows")
	}
	if (&App{desktopStyle: desktopStyleMacOS}).surfaceEffectsEnabled() {
		t.Fatal("macOS surfaces must not add custom highlights or shadows")
	}
	if (&App{reducedEffects: true}).surfaceEffectsEnabled() {
		t.Fatal("reduced effects must disable static glass highlights and shadows")
	}
	var app *App
	if app.surfaceEffectsEnabled() {
		t.Fatal("nil app must not enable surface effects")
	}
}

func TestHeaderIconButtonHasStableSemanticName(t *testing.T) {
	var (
		ops    op.Ops
		router input.Router
		button widget.Clickable
	)
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(40, 40)),
	}
	app := &App{}
	app.headerIconButtonIcon(gtx, &button, uiIconAdd, false, "新建工作区")
	router.Frame(&ops)
	if !semanticTreeContainsLabel(router.AppendSemantics(nil), "新建工作区") {
		t.Fatal("header icon button semantic label is missing")
	}
}

func TestCompactButtonsExposeExplicitSelectedSemantics(t *testing.T) {
	installDesktopThemeSpec(desktopStyleWindows, desktopColorModeLight)
	var (
		ops               op.Ops
		router            input.Router
		selectedText      widget.Clickable
		unselectedText    widget.Clickable
		selectedIcon      widget.Clickable
		emphasizedCommand widget.Clickable
	)
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(320, 120)),
	}
	app := &App{th: material.NewTheme()}
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.compactButton(gtx, &selectedText, "文本已选", true, true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.compactButton(gtx, &unselectedText, "文本未选", false, false)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.compactIconTextButton(gtx, &selectedIcon, uiIconWorkflow, "图标已选", true, true)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return app.compactIconTextButton(gtx, &emphasizedCommand, uiIconSave, "强调操作", true)
		}),
	)
	router.Frame(&ops)

	nodes := router.AppendSemantics(nil)
	assertSemanticSelected(t, nodes, "文本已选", true)
	assertSemanticSelected(t, nodes, "文本未选", false)
	assertSemanticSelected(t, nodes, "图标已选", true)
	assertSemanticSelected(t, nodes, "强调操作", false)
}

func TestSharedControlsUseDesktopThemeDimensions(t *testing.T) {
	previous := installedDesktopTheme
	t.Cleanup(func() {
		installDesktopThemeSpec(previous.Style, previous.ColorMode)
	})

	themes := []struct {
		name          string
		style         string
		controlHeight int
		iconTarget    int
	}{
		{name: "macos", style: desktopStyleMacOS, controlHeight: 30, iconTarget: 30},
		{name: "windows", style: desktopStyleWindows, controlHeight: 32, iconTarget: 32},
	}
	for _, theme := range themes {
		t.Run(theme.name, func(t *testing.T) {
			installDesktopThemeSpec(theme.style, desktopColorModeLight)
			app := &App{th: material.NewTheme(), desktopStyle: theme.style}
			controls := []struct {
				name      string
				wantWidth int
				layout    func(layout.Context) layout.Dimensions
			}{
				{
					name: "compact text",
					layout: func(gtx layout.Context) layout.Dimensions {
						var button widget.Clickable
						return app.compactButton(gtx, &button, "操作", false)
					},
				},
				{
					name: "compact icon text",
					layout: func(gtx layout.Context) layout.Dimensions {
						var button widget.Clickable
						return app.compactIconTextButton(gtx, &button, uiIconWorkflow, "工作流", false)
					},
				},
				{
					name:      "compact icon",
					wantWidth: theme.iconTarget,
					layout: func(gtx layout.Context) layout.Dimensions {
						var button widget.Clickable
						return app.compactIconButton(gtx, &button, uiIconWorkflow, false)
					},
				},
				{
					name: "pill text",
					layout: func(gtx layout.Context) layout.Dimensions {
						var button widget.Clickable
						return app.pillButton(gtx, &button, "状态", false)
					},
				},
				{
					name: "pill icon text",
					layout: func(gtx layout.Context) layout.Dimensions {
						var button widget.Clickable
						return app.pillIconTextButton(gtx, &button, uiIconWorkflow, "状态", false)
					},
				},
				{
					name:      "toolbar icon",
					wantWidth: theme.iconTarget,
					layout: func(gtx layout.Context) layout.Dimensions {
						var button widget.Clickable
						return app.toolbarIconButton(gtx, &button, uiIconWorkflow, false)
					},
				},
				{
					name: "toolbar text",
					layout: func(gtx layout.Context) layout.Dimensions {
						var button widget.Clickable
						return app.toolbarTextButton(gtx, &button, uiIconWorkflow, "工作流", false)
					},
				},
				{
					name:      "header icon target",
					wantWidth: theme.iconTarget,
					layout: func(gtx layout.Context) layout.Dimensions {
						var button widget.Clickable
						return app.headerIconButtonIcon(gtx, &button, uiIconAdd, false, "新建工作区")
					},
				},
			}
			for _, control := range controls {
				t.Run(control.name, func(t *testing.T) {
					var (
						ops    op.Ops
						router input.Router
					)
					gtx := layout.Context{
						Ops:         &ops,
						Source:      router.Source(),
						Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
						Constraints: layout.Constraints{Max: image.Pt(400, 100)},
					}
					dims := control.layout(gtx)
					wantHeight := theme.controlHeight
					if control.name == "header icon target" {
						wantHeight = theme.iconTarget
					}
					if dims.Size.Y != wantHeight {
						t.Fatalf("height=%d want %d", dims.Size.Y, wantHeight)
					}
					if control.wantWidth > 0 && dims.Size.X != control.wantWidth {
						t.Fatalf("width=%d want %d", dims.Size.X, control.wantWidth)
					}
				})
			}
		})
	}
}

func TestOptionalSelectedStateRequiresExplicitValue(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		selected    []bool
		wantValue   bool
		wantPresent bool
	}{
		{name: "omitted", wantValue: false, wantPresent: false},
		{name: "explicit false", selected: []bool{false}, wantValue: false, wantPresent: true},
		{name: "explicit true", selected: []bool{true}, wantValue: true, wantPresent: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			value, present := resolveOptionalSelectedState(testCase.selected)
			if value != testCase.wantValue || present != testCase.wantPresent {
				t.Fatalf("resolveOptionalSelectedState(%v)=(%v, %v) want (%v, %v)", testCase.selected, value, present, testCase.wantValue, testCase.wantPresent)
			}
		})
	}
}

func TestExperienceSwitchSemanticsDistinguishCurrentMode(t *testing.T) {
	installDesktopThemeSpec(desktopStyleMacOS, desktopColorModeDark)
	var (
		ops    op.Ops
		router input.Router
	)
	gtx := layout.Context{
		Ops:         &ops,
		Source:      router.Source(),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(224, 36)),
	}
	app := &App{
		th:                    material.NewTheme(),
		desktopStyle:          desktopStyleMacOS,
		resolvedThemeMode:     desktopColorModeDark,
		experienceMode:        experienceModeWorkflow,
		experienceModeButtons: make([]widget.Clickable, 2),
	}
	app.layoutExperienceSwitch(gtx)
	router.Frame(&ops)

	nodes := router.AppendSemantics(nil)
	assertSemanticSelected(t, nodes, "简易", false)
	assertSemanticSelected(t, nodes, "工作流", true)
}

func assertSemanticSelected(t *testing.T, nodes []input.SemanticNode, label string, want bool) {
	t.Helper()
	node, ok := semanticTreeButtonByLabel(nodes, label)
	if !ok {
		t.Fatalf("semantic button %q is missing", label)
	}
	if node.Desc.Selected != want {
		t.Fatalf("semantic node %q selected=%v want %v", label, node.Desc.Selected, want)
	}
}

func semanticTreeButtonByLabel(nodes []input.SemanticNode, label string) (input.SemanticNode, bool) {
	for _, node := range nodes {
		if node.Desc.Class == semantic.Button && (node.Desc.Label == label || semanticTreeContainsLabel(node.Children, label)) {
			return node, true
		}
		if match, ok := semanticTreeButtonByLabel(node.Children, label); ok {
			return match, true
		}
	}
	return input.SemanticNode{}, false
}

func semanticTreeContainsLabel(nodes []input.SemanticNode, label string) bool {
	for _, node := range nodes {
		if node.Desc.Label == label || semanticTreeContainsLabel(node.Children, label) {
			return true
		}
	}
	return false
}

func TestDesktopPublicationCarriesNormalizedFontScale(t *testing.T) {
	app := &App{}
	for _, testCase := range []struct {
		name  string
		scale float64
		want  float64
	}{
		{name: "default", scale: 0, want: 1},
		{name: "large", scale: 1.12, want: 1.15},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			app.fontScale = testCase.scale
			app.publishDesktopState(snapshot{})
			if got := app.desktopSnapshot().FontScale; got != testCase.want {
				t.Fatalf("publication font scale=%v want %v", got, testCase.want)
			}
		})
	}
}

func TestDetachedWindowTextScaleDefaultsAndSupports115Percent(t *testing.T) {
	view := newDesktopWindowView(&App{}, windowing.Request{})
	spec := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)

	view.applyTheme(spec, 0)
	if got := view.scaledTextSize(unit.Sp(10)); got != unit.Sp(10) {
		t.Fatalf("default text size=%v want 10sp", got)
	}
	if got := view.theme.TextSize; got != unit.Sp(13) {
		t.Fatalf("default theme text size=%v want 13sp", got)
	}

	view.applyTheme(spec, 1.15)
	if got := view.scaledTextSize(unit.Sp(10)); got != unit.Sp(11.5) {
		t.Fatalf("115%% text size=%v want 11.5sp", got)
	}
	if got := view.theme.TextSize; got != unit.Sp(float32(13)*1.15) {
		t.Fatalf("115%% theme text size=%v", got)
	}

	gtx := layout.Context{Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}}
	if got := minimumTextControlHeight(gtx, unit.Dp(28), view.scaledTextSize(unit.Sp(11)), unit.Dp(8)); got != unit.Dp(28) {
		t.Fatalf("normal control height=%v want token height", got)
	}
	if got := minimumTextControlHeight(gtx, unit.Dp(20), view.scaledTextSize(unit.Sp(11)), unit.Dp(8)); got <= unit.Dp(20) {
		t.Fatalf("constrained control height=%v must fit 115%% text", got)
	}
}
