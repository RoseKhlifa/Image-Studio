package ui

import (
	"fmt"
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func TestAppleShellContractMatchesHIGWorkspace(t *testing.T) {
	mac := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	if mac.Metrics.HeaderHeight != unit.Dp(52) {
		t.Fatalf("macOS header height=%v want 52", mac.Metrics.HeaderHeight)
	}
	if shellShowsGlobalFooter(mac.Style) {
		t.Fatal("macOS shell must not render the global footer")
	}
	if shellShowsCommunityActions(mac.Style) {
		t.Fatal("macOS header must not render GitHub, Star, or quote actions")
	}
	headerInsets := headerInsetsForStyle(mac.Style)
	if headerInsets.Left != unit.Dp(12) || headerInsets.Right != unit.Dp(12) {
		t.Fatalf("macOS header horizontal insets=%v/%v want 12/12", headerInsets.Left, headerInsets.Right)
	}
	brand := appleHeaderBrandMetrics()
	if brand.titleSize != unit.Sp(13) {
		t.Fatalf("macOS brand contract=%+v", brand)
	}
	if appleWorkspaceTabHeight(false) != unit.Dp(28) || appleWorkspaceTabHeight(true) != unit.Dp(28) {
		t.Fatalf("macOS tab heights inactive=%v active=%v", appleWorkspaceTabHeight(false), appleWorkspaceTabHeight(true))
	}

	wideContract := simplePaneContractForStyle(mac.Style, mac.Metrics, false)
	wide := fitSimplePaneWidths(1440, int(wideContract.preferredLeft), int(wideContract.preferredRight), int(wideContract.minimumLeft), int(wideContract.minimumRight), int(wideContract.minimumCenter))
	if wide != (simplePaneWidths{left: 312, right: 320, center: 808}) {
		t.Fatalf("wide macOS panes=%+v", wide)
	}
	minimum := fitSimplePaneWidths(1040, int(wideContract.preferredLeft), int(wideContract.preferredRight), int(wideContract.minimumLeft), int(wideContract.minimumRight), int(wideContract.minimumCenter))
	if minimum != (simplePaneWidths{left: 312, right: 288, center: 440}) {
		t.Fatalf("minimum macOS panes=%+v", minimum)
	}

	windows := desktopThemeSpec(desktopStyleWindows, desktopColorModeLight)
	if !shellShowsGlobalFooter(windows.Style) || !shellShowsCommunityActions(windows.Style) {
		t.Fatal("Windows shell must retain its footer and community actions")
	}
	compactWindows := simplePaneContractForStyle(windows.Style, windows.Metrics, true)
	compact := fitSimplePaneWidths(1100, int(compactWindows.preferredLeft), int(compactWindows.preferredRight), int(compactWindows.minimumLeft), int(compactWindows.minimumRight), int(compactWindows.minimumCenter))
	if compact != (simplePaneWidths{left: 336, right: 300, center: 464}) {
		t.Fatalf("compact Windows panes=%+v", compact)
	}
}

func TestAppleHeaderSemanticsHideCommunityActions(t *testing.T) {
	previous := installedDesktopTheme
	defer installDesktopThemeSpec(previous.Style, previous.ColorMode)

	app := newShellParityTestApp(desktopStyleMacOS)
	installDesktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	var ops op.Ops
	var router input.Router
	gtx := shellParityContext(&ops, router.Source(), image.Pt(1200, 52))
	dims := app.layoutHeader(gtx)
	router.Frame(&ops)
	if dims.Size != image.Pt(1200, 52) {
		t.Fatalf("Apple header size=%v", dims.Size)
	}
	nodes := router.AppendSemantics(nil)
	for _, label := range []string{"主工作区", "隐藏生成设置", "新建工作区", "隐藏历史记录", "打开设置"} {
		if !shellSemanticTreeContains(nodes, label) {
			t.Errorf("Apple header missing semantic label %q", label)
		}
	}
	for _, label := range []string{"打开 GitHub 仓库", "在 GitHub 收藏项目"} {
		if shellSemanticTreeContains(nodes, label) {
			t.Errorf("Apple header unexpectedly exposes %q", label)
		}
	}
}

func TestAppleHeaderLabelsMatchActiveExperience(t *testing.T) {
	for _, test := range []struct {
		mode                    string
		hidden                  bool
		wantSidebar, wantDetail string
	}{
		{mode: experienceModeSimple, wantSidebar: "隐藏生成设置", wantDetail: "隐藏历史记录"},
		{mode: experienceModeWorkflow, wantSidebar: "隐藏工作流资源", wantDetail: "隐藏节点检查器"},
		{mode: experienceModeWorkflow, hidden: true, wantSidebar: "显示工作流资源", wantDetail: "显示节点检查器"},
	} {
		if got := macSidebarToggleLabel(test.mode, test.hidden); got != test.wantSidebar {
			t.Errorf("sidebar label mode=%q hidden=%t got %q want %q", test.mode, test.hidden, got, test.wantSidebar)
		}
		if got := macInspectorToggleLabel(test.mode, test.hidden); got != test.wantDetail {
			t.Errorf("inspector label mode=%q hidden=%t got %q want %q", test.mode, test.hidden, got, test.wantDetail)
		}
	}
}

func TestWindowsHeaderSemanticsKeepCommunityActions(t *testing.T) {
	previous := installedDesktopTheme
	defer installDesktopThemeSpec(previous.Style, previous.ColorMode)

	app := newShellParityTestApp(desktopStyleWindows)
	installDesktopThemeSpec(desktopStyleWindows, desktopColorModeLight)
	var ops op.Ops
	var router input.Router
	gtx := shellParityContext(&ops, router.Source(), image.Pt(1440, 48))
	app.layoutHeader(gtx)
	router.Frame(&ops)
	nodes := router.AppendSemantics(nil)
	for _, label := range []string{"打开 GitHub 仓库", "在 GitHub 收藏项目", "打开设置"} {
		if !shellSemanticTreeContains(nodes, label) {
			t.Errorf("Windows header missing semantic label %q", label)
		}
	}
}

func TestAppleWorkspaceBarUsesScrollableHorizontalList(t *testing.T) {
	previous := installedDesktopTheme
	defer installDesktopThemeSpec(previous.Style, previous.ColorMode)
	installDesktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)

	app := newShellParityTestApp(desktopStyleMacOS)
	app.workspaces = make([]workspaceState, 12)
	for index := range app.workspaces {
		app.workspaces[index] = workspaceState{ID: fmt.Sprintf("ws-%02d", index), Name: fmt.Sprintf("工作区 %02d", index+1)}
	}
	app.activeWorkspaceID = app.workspaces[0].ID

	var ops op.Ops
	var router input.Router
	gtx := shellParityContext(&ops, router.Source(), image.Pt(520, 40))
	dims := app.layoutWorkspaceBar(gtx)
	router.Frame(&ops)
	if dims.Size != image.Pt(520, 40) {
		t.Fatalf("workspace bar size=%v", dims.Size)
	}
	if app.workspaceList.Position.Count >= len(app.workspaces) {
		t.Fatalf("workspace list rendered every tab; count=%d length=%d", app.workspaceList.Position.Count, len(app.workspaces))
	}
	if !app.workspaceList.List.ScrollAnyAxis {
		t.Fatal("horizontal workspace list must accept a vertical mouse wheel")
	}
	if app.workspaceList.Position.Length <= 520 {
		t.Fatalf("workspace list length=%d does not exceed viewport", app.workspaceList.Position.Length)
	}
	selected, ok := shellSemanticButton(router.AppendSemantics(nil), "工作区 工作区 01")
	if !ok || !selected.Desc.Selected {
		t.Fatalf("active workspace semantics=%+v present=%t", selected.Desc, ok)
	}

	app.workspaceList.ScrollBy(5)
	ops.Reset()
	gtx = shellParityContext(&ops, router.Source(), image.Pt(520, 40))
	gtx.Now = gtx.Now.Add(time.Second / 60)
	app.layoutWorkspaceBar(gtx)
	router.Frame(&ops)
	if app.workspaceList.Position.First == 0 {
		t.Fatalf("horizontal workspace list did not scroll: %+v", app.workspaceList.Position)
	}
}

func TestAppleWorkspaceTabAndExperienceSwitchDimensions(t *testing.T) {
	previous := installedDesktopTheme
	defer installDesktopThemeSpec(previous.Style, previous.ColorMode)
	installDesktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)

	app := newShellParityTestApp(desktopStyleMacOS)
	app.workspaces = []workspaceState{{ID: "ws-1", Name: "主工作区"}, {ID: "ws-2", Name: "辅助工作区"}}
	app.activeWorkspaceID = "ws-1"
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Constraints{Max: image.Pt(260, 40)},
		Now:         time.Now(),
	}
	if got := app.layoutWorkspaceTab(gtx, app.workspaces[0], true).Size.Y; got != 28 {
		t.Fatalf("active Apple tab height=%d want 28", got)
	}
	ops.Reset()
	if got := app.layoutWorkspaceTab(gtx, app.workspaces[1], false).Size.Y; got != 28 {
		t.Fatalf("inactive Apple tab height=%d want 28", got)
	}

	ops.Reset()
	gtx.Constraints = layout.Constraints{Max: image.Pt(300, 60)}
	if got := app.layoutExperienceSwitch(gtx).Size; got != image.Pt(176, 28) {
		t.Fatalf("Apple experience switch size=%v want 176x28", got)
	}
}

func TestAppleSimplePaneVisibilityPreservesCanvasPriority(t *testing.T) {
	mac := desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight)
	contract := simplePaneContractForStyle(mac.Style, mac.Metrics, false)
	metric := func(value unit.Dp) int { return int(value) }

	if got := fitVisiblePaneWidths(1440, contract, metric, true, true); got != (simplePaneWidths{left: 312, right: 320, center: 808}) {
		t.Fatalf("visible Apple panes=%+v", got)
	}
	if got := fitVisiblePaneWidths(1440, contract, metric, true, false); got != (simplePaneWidths{left: 312, center: 1128}) {
		t.Fatalf("sidebar-only Apple panes=%+v", got)
	}
	if got := fitVisiblePaneWidths(1440, contract, metric, false, true); got != (simplePaneWidths{right: 320, center: 1120}) {
		t.Fatalf("inspector-only Apple panes=%+v", got)
	}
	if got := fitVisiblePaneWidths(1440, contract, metric, false, false); got != (simplePaneWidths{center: 1440}) {
		t.Fatalf("canvas-only Apple panes=%+v", got)
	}
}

func newShellParityTestApp(style string) *App {
	spec := desktopThemeSpec(style, desktopColorModeLight)
	theme := material.NewTheme()
	theme.Palette = material.Palette{
		Bg:         spec.Colors.bg,
		Fg:         spec.Colors.text,
		ContrastBg: spec.Colors.accent,
		ContrastFg: desktopReadableText(spec.Colors.accent),
	}
	app := &App{
		th:                    theme,
		desktopStyle:          spec.Style,
		resolvedThemeMode:     spec.ColorMode,
		themeMode:             desktopColorModeLight,
		experienceMode:        experienceModeSimple,
		themeButtons:          make([]widget.Clickable, 3),
		experienceModeButtons: make([]widget.Clickable, 2),
		workspaces:            []workspaceState{{ID: "ws-1", Name: "主工作区"}},
		activeWorkspaceID:     "ws-1",
		workspaceButtons:      map[string]*widget.Clickable{},
		closeWorkspaceButtons: map[string]*widget.Clickable{},
	}
	app.workspaceList.List.Axis = layout.Horizontal
	app.workspaceList.List.Alignment = layout.Middle
	app.workspaceList.List.ScrollAnyAxis = true
	return app
}

func shellParityContext(ops *op.Ops, source input.Source, size image.Point) layout.Context {
	return layout.Context{
		Ops:         ops,
		Source:      source,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(size),
		Now:         time.Now(),
	}
}

func shellSemanticTreeContains(nodes []input.SemanticNode, label string) bool {
	for _, node := range nodes {
		if node.Desc.Label == label || shellSemanticTreeContains(node.Children, label) {
			return true
		}
	}
	return false
}

func shellSemanticButton(nodes []input.SemanticNode, label string) (input.SemanticNode, bool) {
	for _, node := range nodes {
		if node.Desc.Label == label {
			return node, true
		}
		if match, ok := shellSemanticButton(node.Children, label); ok {
			return match, true
		}
	}
	return input.SemanticNode{}, false
}
