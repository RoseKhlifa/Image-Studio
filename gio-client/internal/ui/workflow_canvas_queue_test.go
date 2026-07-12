package ui

import (
	"fmt"
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestWorkflowNodeVisualScaleTracksZoomWithReadableFloor(t *testing.T) {
	tests := []struct {
		name string
		zoom float32
		want float32
	}{
		{name: "below-canvas-minimum", zoom: 0.1, want: workflowNodeMinVisualScale},
		{name: "canvas-minimum", zoom: workflowCanvasMinZoom, want: workflowNodeMinVisualScale},
		{name: "readable-floor", zoom: workflowNodeMinVisualScale, want: workflowNodeMinVisualScale},
		{name: "identity", zoom: 1, want: 1},
		{name: "zoomed", zoom: 1.4, want: 1.4},
		{name: "above-maximum", zoom: 4, want: workflowCanvasMaxZoom},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := workflowNodeVisualScale(test.zoom); got != test.want {
				t.Fatalf("visual scale=%v want %v", got, test.want)
			}
		})
	}
}

func TestWorkflowNodeInternalsUseSameVisualScale(t *testing.T) {
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(900, 600)),
	}
	spec := desktopThemeSpec(desktopStyleWindows, desktopColorModeDark)
	node := defaultWorkflowGraph().Nodes[2]
	node.Position = image.Point{}

	normalView := workflowCanvasViewState{zoom: 1}
	zoomedView := workflowCanvasViewState{zoom: 1.4}
	minimumView := workflowCanvasViewState{zoom: workflowCanvasMinZoom}
	normalRect := workflowNodeRect(gtx, node, spec, &normalView)
	zoomedRect := workflowNodeRect(gtx, node, spec, &zoomedView)
	minimumRect := workflowNodeRect(gtx, node, spec, &minimumView)

	normalMetric := workflowNodeScaledMetric(gtx.Metric, workflowNodeVisualScale(normalView.zoom))
	zoomedMetric := workflowNodeScaledMetric(gtx.Metric, workflowNodeVisualScale(zoomedView.zoom))
	minimumMetric := workflowNodeScaledMetric(gtx.Metric, workflowNodeVisualScale(minimumView.zoom))
	if normalRect.Size() != workflowNodeSizeAtScale(gtx, spec, 1) {
		t.Fatalf("normal node size=%v", normalRect.Size())
	}
	if zoomedRect.Dx() != zoomedMetric.Dp(spec.Metrics.NodeWidth) || zoomedRect.Dy() != zoomedMetric.Dp(workflowNodeHeight) {
		t.Fatalf("zoomed node size=%v metric=%+v", zoomedRect.Size(), zoomedMetric)
	}
	if minimumRect.Dx() != minimumMetric.Dp(spec.Metrics.NodeWidth) || minimumRect.Dy() != minimumMetric.Dp(workflowNodeHeight) {
		t.Fatalf("minimum node size=%v metric=%+v", minimumRect.Size(), minimumMetric)
	}

	normalPortY := workflowPortPixelY(gtx, normalRect, 1, workflowNodeVisualScale(normalView.zoom))
	zoomedPortY := workflowPortPixelY(gtx, zoomedRect, 1, workflowNodeVisualScale(zoomedView.zoom))
	minimumPortY := workflowPortPixelY(gtx, minimumRect, 1, workflowNodeVisualScale(minimumView.zoom))
	if normalPortY != normalMetric.Dp(unit.Dp(83)) || zoomedPortY != zoomedMetric.Dp(unit.Dp(83)) || minimumPortY != minimumMetric.Dp(unit.Dp(83)) {
		t.Fatalf("port y normal=%d zoomed=%d minimum=%d", normalPortY, zoomedPortY, minimumPortY)
	}
	if zoomedMetric.Sp(unit.Sp(13)) <= normalMetric.Sp(unit.Sp(13)) || zoomedMetric.Dp(unit.Dp(16)) <= normalMetric.Dp(unit.Dp(16)) {
		t.Fatalf("text/icon did not scale: normal=%d/%d zoomed=%d/%d", normalMetric.Sp(13), normalMetric.Dp(16), zoomedMetric.Sp(13), zoomedMetric.Dp(16))
	}
	if minimumMetric.Sp(unit.Sp(13)) < 10 || minimumMetric.Dp(unit.Dp(16)) < 12 {
		t.Fatalf("readable minimum too small: text=%d icon=%d", minimumMetric.Sp(13), minimumMetric.Dp(16))
	}

	for _, kind := range []workflowNodeKind{workflowNodePrompt, workflowNodeSource, workflowNodeGenerate, workflowNodePreview, workflowNodeExport, "unknown"} {
		if workflowNodeKindIcon(kind) == nil {
			t.Fatalf("node kind %q has no icon", kind)
		}
	}
}

func TestWorkflowQueueListScrollsWithinShortViewport(t *testing.T) {
	app := &App{
		th:                material.NewTheme(),
		activeWorkspaceID: "active",
		workspaces:        []workspaceState{{ID: "active", Name: "当前任务"}},
	}
	for index := 0; index < 24; index++ {
		workspaceID := fmt.Sprintf("queued-%02d", index)
		app.desktopQueuedWorkspaceRuns = append(app.desktopQueuedWorkspaceRuns, workspaceID)
		app.workspaces = append(app.workspaces, workspaceState{ID: workspaceID, Name: fmt.Sprintf("排队任务 %02d", index+1)})
	}
	app.workflowConsoleList.List.Axis = layout.Vertical
	snap := snapshot{Running: true, Status: "正在生成", BatchTotal: 8}
	spec := desktopThemeSpec(desktopStyleWindows, desktopColorModeDark)
	viewport := image.Pt(420, 92)

	render := func() layout.Dimensions {
		var ops op.Ops
		gtx := layout.Context{
			Ops:         &ops,
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(viewport),
		}
		return app.layoutWorkflowQueue(gtx, snap, spec)
	}

	dims := render()
	itemCount := 1 + len(app.desktopQueuedWorkspaceRuns)
	if dims.Size != viewport {
		t.Fatalf("queue dimensions=%v want bounded viewport %v", dims.Size, viewport)
	}
	if app.workflowConsoleList.Position.Count >= itemCount {
		t.Fatalf("queue laid out every item in short viewport: count=%d total=%d", app.workflowConsoleList.Position.Count, itemCount)
	}
	if app.workflowConsoleList.Position.Length <= viewport.Y {
		t.Fatalf("queue content length=%d want > viewport %d", app.workflowConsoleList.Position.Length, viewport.Y)
	}

	app.workflowConsoleList.ScrollBy(8)
	render()
	if app.workflowConsoleList.Position.First == 0 {
		t.Fatalf("queue did not scroll: %+v", app.workflowConsoleList.Position)
	}
}

func TestWorkflowQueueShowsWaitingItemsBetweenRuns(t *testing.T) {
	app := &App{
		th:                         material.NewTheme(),
		desktopQueuedWorkspaceRuns: []string{"queued"},
		workspaces:                 []workspaceState{{ID: "queued", Name: "待运行"}},
	}
	var ops op.Ops
	gtx := layout.Context{
		Ops:         &ops,
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(320, 48)),
	}
	app.layoutWorkflowQueue(gtx, snapshot{}, desktopThemeSpec(desktopStyleWindows, desktopColorModeLight))
	if app.workflowConsoleList.Position.Count != 1 {
		t.Fatalf("waiting-only queue count=%d want 1", app.workflowConsoleList.Position.Count)
	}
}
