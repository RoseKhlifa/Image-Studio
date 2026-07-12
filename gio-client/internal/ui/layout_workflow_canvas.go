package ui

import (
	"image"
	"image/color"
	"math"
	"strings"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const (
	workflowNodeHeight         = unit.Dp(156)
	workflowCanvasMinZoom      = float32(0.65)
	workflowCanvasMaxZoom      = float32(1.55)
	workflowNodeMinVisualScale = float32(0.78)
)

type workflowNodePhase string

const (
	workflowNodePhaseIdle    workflowNodePhase = "idle"
	workflowNodePhaseRunning workflowNodePhase = "running"
	workflowNodePhaseSuccess workflowNodePhase = "success"
	workflowNodePhaseWarning workflowNodePhase = "warning"
	workflowNodePhaseError   workflowNodePhase = "error"
)

type workflowNodeRuntime struct {
	Phase    workflowNodePhase
	Detail   string
	Progress float32
}

type workflowCanvasData struct {
	Graph     workflowGraphModel
	Selected  string
	Runtime   map[string]workflowNodeRuntime
	Workspace string
}

type workflowCanvasCallbacks struct {
	Select func(nodeID string)
	Move   func(nodeID string, position image.Point)
}

type workflowNodeInteraction struct {
	drag     gesture.Drag
	hover    gesture.Hover
	lastDrag image.Point
	active   bool
}

type workflowCanvasViewState struct {
	interactions      map[string]*workflowNodeInteraction
	overrides         map[string]image.Point
	overrideRevisions map[string]int
	workspaceID       string
	graphRevision     int
	canvasTag         struct{}
	panActive         bool
	panPointerID      pointer.ID
	panLast           image.Point
	offset            image.Point
	zoom              float32
}

func (view *workflowCanvasViewState) ensure() {
	if view.interactions == nil {
		view.interactions = map[string]*workflowNodeInteraction{}
	}
	if view.overrides == nil {
		view.overrides = map[string]image.Point{}
	}
	if view.overrideRevisions == nil {
		view.overrideRevisions = map[string]int{}
	}
	if view.zoom <= 0 {
		view.zoom = 1
	}
}

func (view *workflowCanvasViewState) syncModel(data workflowCanvasData) {
	view.ensure()
	workspaceID := strings.TrimSpace(data.Workspace)
	if view.workspaceID != workspaceID {
		view.workspaceID = workspaceID
		view.graphRevision = data.Graph.Revision
		view.interactions = map[string]*workflowNodeInteraction{}
		view.clearOverrides()
		view.panActive = false
		view.panPointerID = 0
		view.panLast = image.Point{}
		return
	}

	if data.Graph.Revision < view.graphRevision {
		view.clearOverrides()
	}
	nodes := make(map[string]workflowNodeModel, len(data.Graph.Nodes))
	for _, node := range data.Graph.Nodes {
		nodes[node.ID] = node
	}
	for nodeID, override := range view.overrides {
		node, exists := nodes[nodeID]
		if !exists || node.Position == override {
			view.clearOverride(nodeID)
			continue
		}
		baseRevision, tracked := view.overrideRevisions[nodeID]
		if tracked && data.Graph.Revision > baseRevision {
			view.clearOverride(nodeID)
		}
	}
	view.graphRevision = data.Graph.Revision
}

func (view *workflowCanvasViewState) setOverride(nodeID string, position image.Point, graphRevision int) {
	view.ensure()
	if _, exists := view.overrides[nodeID]; !exists {
		view.overrideRevisions[nodeID] = graphRevision
	}
	view.overrides[nodeID] = position
}

func (view *workflowCanvasViewState) clearOverride(nodeID string) {
	delete(view.overrides, nodeID)
	delete(view.overrideRevisions, nodeID)
}

func (view *workflowCanvasViewState) clearOverrides() {
	clear(view.overrides)
	clear(view.overrideRevisions)
}

func (view *workflowCanvasViewState) resetViewport() {
	view.ensure()
	view.offset = image.Pt(24, 24)
	view.zoom = 1
	view.panActive = false
	view.panPointerID = 0
	view.panLast = image.Point{}
}

func (view *workflowCanvasViewState) zoomBy(delta float32) {
	view.ensure()
	view.zoom = float32(math.Max(float64(workflowCanvasMinZoom), math.Min(float64(workflowCanvasMaxZoom), float64(view.zoom+delta))))
}

func (view *workflowCanvasViewState) interaction(nodeID string) *workflowNodeInteraction {
	view.ensure()
	if interaction, ok := view.interactions[nodeID]; ok {
		return interaction
	}
	interaction := new(workflowNodeInteraction)
	view.interactions[nodeID] = interaction
	return interaction
}

func workflowCanvasZoom(zoom float32) float32 {
	if zoom < workflowCanvasMinZoom {
		return workflowCanvasMinZoom
	}
	if zoom > workflowCanvasMaxZoom {
		return workflowCanvasMaxZoom
	}
	return zoom
}

func workflowNodeVisualScale(zoom float32) float32 {
	return max(workflowCanvasZoom(zoom), workflowNodeMinVisualScale)
}

func workflowNodeScaledMetric(metric unit.Metric, scale float32) unit.Metric {
	if metric.PxPerDp == 0 {
		metric.PxPerDp = 1
	}
	if metric.PxPerSp == 0 {
		metric.PxPerSp = 1
	}
	metric.PxPerDp *= scale
	metric.PxPerSp *= scale
	return metric
}

func workflowNodeSizeAtScale(gtx layout.Context, spec desktopThemeTokens, scale float32) image.Point {
	metric := workflowNodeScaledMetric(gtx.Metric, scale)
	width := metric.Dp(spec.Metrics.NodeWidth)
	height := metric.Dp(workflowNodeHeight)
	return image.Pt(max(width, metric.Dp(unit.Dp(210))), max(height, metric.Dp(unit.Dp(132))))
}

func workflowNodePixelPosition(gtx layout.Context, position image.Point, view *workflowCanvasViewState) image.Point {
	zoom := workflowCanvasZoom(view.zoom)
	return image.Pt(
		int(float32(gtx.Dp(unit.Dp(position.X)))*zoom)+view.offset.X,
		int(float32(gtx.Dp(unit.Dp(position.Y)))*zoom)+view.offset.Y,
	)
}

func workflowNodeRect(gtx layout.Context, node workflowNodeModel, spec desktopThemeTokens, view *workflowCanvasViewState) image.Rectangle {
	position := node.Position
	if override, ok := view.overrides[node.ID]; ok {
		position = override
	}
	minPoint := workflowNodePixelPosition(gtx, position, view)
	size := workflowNodeSizeAtScale(gtx, spec, workflowNodeVisualScale(view.zoom))
	return image.Rectangle{Min: minPoint, Max: minPoint.Add(size)}
}

func (view *workflowCanvasViewState) handleCanvasEvents(gtx layout.Context, graph workflowGraphModel, spec desktopThemeTokens) {
	area := clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops)
	event.Op(gtx.Ops, &view.canvasTag)
	if view.panActive {
		pointer.CursorGrabbing.Add(gtx.Ops)
	} else {
		pointer.CursorGrab.Add(gtx.Ops)
	}
	area.Pop()

	insideNode := func(point image.Point) bool {
		for _, node := range graph.Nodes {
			if point.In(workflowNodeRect(gtx, node, spec, view)) {
				return true
			}
		}
		return false
	}
	for {
		evt, ok := gtx.Event(pointer.Filter{
			Target:  &view.canvasTag,
			Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -1_000_000, Max: 1_000_000},
			ScrollY: pointer.ScrollRange{Min: -1_000_000, Max: 1_000_000},
		})
		if !ok {
			break
		}
		pe, ok := evt.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Scroll:
			view.offset.X -= int(pe.Scroll.X)
			view.offset.Y -= int(pe.Scroll.Y)
		case pointer.Press:
			point := pe.Position.Round()
			if insideNode(point) {
				continue
			}
			view.panActive = true
			view.panPointerID = pe.PointerID
			view.panLast = point
			gtx.Execute(pointer.GrabCmd{Tag: &view.canvasTag, ID: pe.PointerID})
		case pointer.Drag:
			if !view.panActive || pe.PointerID != view.panPointerID {
				continue
			}
			point := pe.Position.Round()
			delta := point.Sub(view.panLast)
			view.offset = view.offset.Add(delta)
			view.panLast = point
		case pointer.Release, pointer.Cancel:
			if pe.PointerID == view.panPointerID {
				view.panActive = false
				view.panPointerID = 0
				view.panLast = image.Point{}
			}
		}
	}
}

func (view *workflowCanvasViewState) Layout(
	gtx layout.Context,
	th *material.Theme,
	spec desktopThemeTokens,
	data workflowCanvasData,
	callbacks workflowCanvasCallbacks,
) layout.Dimensions {
	view.syncModel(data)
	gtx.Constraints.Min = gtx.Constraints.Max
	paint.FillShape(gtx.Ops, spec.Colors.canvasBg, clip.Rect{Max: gtx.Constraints.Max}.Op())
	view.paintGrid(gtx, spec)
	view.handleCanvasEvents(gtx, data.Graph, spec)
	view.paintEdges(gtx, data, spec)
	for _, node := range data.Graph.Nodes {
		view.layoutNode(gtx, th, spec, data, node, callbacks)
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}

func (view *workflowCanvasViewState) paintGrid(gtx layout.Context, spec desktopThemeTokens) {
	maxSize := gtx.Constraints.Max
	minor := max(12, gtx.Dp(unit.Dp(20)))
	major := minor * 5
	minorColor := withAlpha(spec.Colors.canvasTile, 0x62)
	majorColor := withAlpha(spec.Colors.border2, 0x72)
	startX := view.offset.X % minor
	if startX < 0 {
		startX += minor
	}
	startY := view.offset.Y % minor
	if startY < 0 {
		startY += minor
	}
	for x := startX; x < maxSize.X; x += minor {
		width := 1
		colorValue := minorColor
		if ((x-view.offset.X)%major+major)%major == 0 {
			colorValue = majorColor
		}
		paint.FillShape(gtx.Ops, colorValue, clip.Rect(image.Rect(x, 0, x+width, maxSize.Y)).Op())
	}
	for y := startY; y < maxSize.Y; y += minor {
		height := 1
		colorValue := minorColor
		if ((y-view.offset.Y)%major+major)%major == 0 {
			colorValue = majorColor
		}
		paint.FillShape(gtx.Ops, colorValue, clip.Rect(image.Rect(0, y, maxSize.X, y+height)).Op())
	}
}

func (view *workflowCanvasViewState) paintEdges(gtx layout.Context, data workflowCanvasData, spec desktopThemeTokens) {
	nodes := make(map[string]workflowNodeModel, len(data.Graph.Nodes))
	for _, node := range data.Graph.Nodes {
		nodes[node.ID] = node
	}
	for _, edge := range data.Graph.Edges {
		from, fromOK := nodes[edge.FromNode]
		to, toOK := nodes[edge.ToNode]
		if !fromOK || !toOK || !from.Enabled || !to.Enabled {
			continue
		}
		fromRect := workflowNodeRect(gtx, from, spec, view)
		toRect := workflowNodeRect(gtx, to, spec, view)
		fromIndex := workflowPortIndex(from.Outputs, edge.FromPort)
		toIndex := workflowPortIndex(to.Inputs, edge.ToPort)
		visualScale := workflowNodeVisualScale(view.zoom)
		fromPoint := image.Pt(fromRect.Max.X, workflowPortPixelY(gtx, fromRect, fromIndex, visualScale))
		toPoint := image.Pt(toRect.Min.X, workflowPortPixelY(gtx, toRect, toIndex, visualScale))
		metric := workflowNodeScaledMetric(gtx.Metric, visualScale)
		curve := max(metric.Dp(unit.Dp(48)), absInt(toPoint.X-fromPoint.X)/2)
		var path clip.Path
		path.Begin(gtx.Ops)
		path.MoveTo(f32.Pt(float32(fromPoint.X), float32(fromPoint.Y)))
		path.CubeTo(
			f32.Pt(float32(fromPoint.X+curve), float32(fromPoint.Y)),
			f32.Pt(float32(toPoint.X-curve), float32(toPoint.Y)),
			f32.Pt(float32(toPoint.X), float32(toPoint.Y)),
		)
		lineColor := withAlpha(spec.Colors.textDim, 0x8c)
		if runtime, ok := data.Runtime[edge.FromNode]; ok && runtime.Phase == workflowNodePhaseRunning {
			lineColor = spec.Colors.accent
		}
		paint.FillShape(gtx.Ops, lineColor, clip.Stroke{Path: path.End(), Width: float32(max(1, metric.Dp(unit.Dp(2))))}.Op())
	}
}

func (view *workflowCanvasViewState) layoutNode(
	gtx layout.Context,
	th *material.Theme,
	spec desktopThemeTokens,
	data workflowCanvasData,
	node workflowNodeModel,
	callbacks workflowCanvasCallbacks,
) {
	rect := workflowNodeRect(gtx, node, spec, view)
	if rect.Max.X < 0 || rect.Max.Y < 0 || rect.Min.X > gtx.Constraints.Max.X || rect.Min.Y > gtx.Constraints.Max.Y {
		return
	}
	interaction := view.interaction(node.ID)
	for {
		evt, ok := interaction.drag.Update(gtx.Metric, gtx.Source, gesture.Both)
		if !ok {
			break
		}
		switch evt.Kind {
		case pointer.Press:
			interaction.active = true
			interaction.lastDrag = evt.Position.Round()
			if evt.Source == pointer.Mouse {
				gtx.Execute(key.FocusCmd{Tag: interaction})
			}
			if callbacks.Select != nil {
				callbacks.Select(node.ID)
			}
		case pointer.Drag:
			if !interaction.active {
				continue
			}
			point := evt.Position.Round()
			deltaPx := point.Sub(interaction.lastDrag)
			interaction.lastDrag = point
			position := node.Position
			if override, ok := view.overrides[node.ID]; ok {
				position = override
			}
			canvasZoom := workflowCanvasZoom(view.zoom)
			position.X += int(float32(gtx.Metric.PxToDp(deltaPx.X)) / canvasZoom)
			position.Y += int(float32(gtx.Metric.PxToDp(deltaPx.Y)) / canvasZoom)
			position.X = clampInt(position.X, -4096, 4096)
			position.Y = clampInt(position.Y, -4096, 4096)
			view.setOverride(node.ID, position, data.Graph.Revision)
			if callbacks.Move != nil {
				callbacks.Move(node.ID, position)
			}
		case pointer.Release, pointer.Cancel:
			interaction.active = false
			interaction.lastDrag = image.Point{}
		}
	}
	for {
		evt, ok := gtx.Event(
			key.FocusFilter{Target: interaction},
			key.Filter{Focus: interaction, Name: key.NameReturn},
			key.Filter{Focus: interaction, Name: key.NameSpace},
		)
		if !ok {
			break
		}
		keyEvent, ok := evt.(key.Event)
		if !ok || keyEvent.State != key.Release {
			continue
		}
		if callbacks.Select != nil {
			callbacks.Select(node.ID)
		}
	}

	offset := op.Offset(rect.Min).Push(gtx.Ops)
	defer offset.Pop()
	localRect := image.Rectangle{Max: rect.Size()}
	visualScale := workflowNodeVisualScale(view.zoom)
	nodeMetric := workflowNodeScaledMetric(gtx.Metric, visualScale)
	nodeRadius := nodeMetric.Dp(spec.Metrics.NodeRadius)
	area := clip.RRect{Rect: localRect, NE: nodeRadius, NW: nodeRadius, SE: nodeRadius, SW: nodeRadius}.Push(gtx.Ops)
	interaction.drag.Add(gtx.Ops)
	interaction.hover.Add(gtx.Ops)
	event.Op(gtx.Ops, interaction)
	semantic.Button.Add(gtx.Ops)
	semantic.LabelOp(node.Title).Add(gtx.Ops)
	semantic.DescriptionOp(node.Subtitle).Add(gtx.Ops)
	semantic.SelectedOp(data.Selected == node.ID).Add(gtx.Ops)
	semantic.EnabledOp(node.Enabled).Add(gtx.Ops)
	area.Pop()

	hovered := interaction.hover.Update(gtx.Source)
	selected := data.Selected == node.ID
	focused := gtx.Focused(interaction)
	runtimeState := data.Runtime[node.ID]
	fill := spec.Colors.surface
	if hovered {
		fill = spec.Colors.surface2
	}
	if !node.Enabled {
		fill = withAlpha(spec.Colors.surface2, 0xd8)
	}
	paint.FillShape(gtx.Ops, fill, clip.RRect{Rect: localRect, NE: nodeRadius, NW: nodeRadius, SE: nodeRadius, SW: nodeRadius}.Op(gtx.Ops))
	borderColor := spec.Colors.border2
	borderWidth := max(1, nodeMetric.Dp(unit.Dp(1)))
	if selected || focused {
		borderColor = spec.Colors.focusRing
		borderWidth = max(2, nodeMetric.Dp(unit.Dp(2)))
	}
	paintWorkflowRectOutline(gtx, localRect, nodeRadius, borderWidth, borderColor)

	headerHeight := nodeMetric.Dp(unit.Dp(36))
	headerColor := workflowNodePhaseColor(spec, runtimeState.Phase)
	paint.FillShape(gtx.Ops, withAlpha(headerColor, 0x24), clip.RRect{Rect: image.Rect(0, 0, localRect.Max.X, headerHeight), NE: nodeRadius, NW: nodeRadius}.Op(gtx.Ops))
	headerRuleHeight := max(1, nodeMetric.Dp(unit.Dp(2)))
	paint.FillShape(gtx.Ops, withAlpha(headerColor, 0xaa), clip.Rect(image.Rect(0, headerHeight-headerRuleHeight, localRect.Max.X, headerHeight)).Op())

	content := layout.Context{
		Ops:    gtx.Ops,
		Now:    gtx.Now,
		Source: gtx.Source,
		Metric: nodeMetric,
		Constraints: layout.Constraints{
			Min: localRect.Size(),
			Max: localRect.Size(),
		},
	}
	layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(14), Right: unit.Dp(14)}.Layout(content, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return workflowNodeTitle(gtx, th, spec, node, runtimeState)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(22)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				detail := strings.TrimSpace(runtimeState.Detail)
				if detail == "" {
					detail = node.Subtitle
				}
				return workflowMaterialLabel(gtx, th, detail, unit.Sp(11), spec.Colors.textMuted, font.Normal, 2)
			}),
			layout.Flexed(1, layout.Spacer{}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return workflowNodeFooter(gtx, th, spec, runtimeState)
			}),
		)
	})
	view.paintPorts(gtx, spec, localRect, node, visualScale)
}

func workflowNodeTitle(gtx layout.Context, th *material.Theme, spec desktopThemeTokens, node workflowNodeModel, runtimeState workflowNodeRuntime) layout.Dimensions {
	icon := workflowNodeKindIcon(node.Kind)
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Dp(unit.Dp(16))
			gtx.Constraints = layout.Exact(image.Pt(size, size))
			return icon.Layout(gtx, workflowNodePhaseColor(spec, runtimeState.Phase))
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(7)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return workflowMaterialLabel(gtx, th, node.Title, unit.Sp(13), spec.Colors.text, font.SemiBold, 1)
		}),
	)
}

func workflowNodeKindIcon(kind workflowNodeKind) *widget.Icon {
	switch kind {
	case workflowNodePrompt:
		return uiIconEdit
	case workflowNodeSource:
		return uiIconSource
	case workflowNodeGenerate:
		return uiIconSpark
	case workflowNodePreview:
		return uiIconVisibility
	case workflowNodeExport:
		return uiIconSave
	default:
		return uiIconWorkflow
	}
}

func (view *workflowCanvasViewState) paintPorts(gtx layout.Context, spec desktopThemeTokens, rect image.Rectangle, node workflowNodeModel, visualScale float32) {
	metric := workflowNodeScaledMetric(gtx.Metric, visualScale)
	radius := max(3, metric.Dp(unit.Dp(5)))
	for idx, port := range node.Inputs {
		y := workflowPortPixelY(gtx, rect, idx, visualScale)
		center := image.Pt(0, y)
		paint.FillShape(gtx.Ops, workflowPortColor(spec, port.Kind), clip.Ellipse(image.Rect(center.X-radius, center.Y-radius, center.X+radius, center.Y+radius)).Op(gtx.Ops))
	}
	for idx, port := range node.Outputs {
		y := workflowPortPixelY(gtx, rect, idx, visualScale)
		center := image.Pt(rect.Max.X, y)
		paint.FillShape(gtx.Ops, workflowPortColor(spec, port.Kind), clip.Ellipse(image.Rect(center.X-radius, center.Y-radius, center.X+radius, center.Y+radius)).Op(gtx.Ops))
	}
}

func workflowNodeFooter(gtx layout.Context, th *material.Theme, spec desktopThemeTokens, runtimeState workflowNodeRuntime) layout.Dimensions {
	progress := runtimeState.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	label := "就绪"
	switch runtimeState.Phase {
	case workflowNodePhaseRunning:
		label = "运行中"
	case workflowNodePhaseSuccess:
		label = "已完成"
	case workflowNodePhaseWarning:
		label = "待配置"
	case workflowNodePhaseError:
		label = "失败"
	}
	return layout.Flex{Axis: layout.Vertical, Gap: gtx.Dp(unit.Dp(6))}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return workflowMaterialLabel(gtx, th, label, unit.Sp(10), workflowNodePhaseTextColor(spec, runtimeState.Phase), font.Medium, 1)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			height := max(2, gtx.Dp(unit.Dp(3)))
			width := gtx.Constraints.Max.X
			paint.FillShape(gtx.Ops, spec.Colors.surface2, clip.RRect{Rect: image.Rect(0, 0, width, height), NE: height / 2, NW: height / 2, SE: height / 2, SW: height / 2}.Op(gtx.Ops))
			if runtimeState.Phase == workflowNodePhaseRunning && progress <= 0 {
				progress = 0.15
			}
			filled := int(float32(width) * progress)
			if filled > 0 {
				paint.FillShape(gtx.Ops, workflowNodePhaseColor(spec, runtimeState.Phase), clip.RRect{Rect: image.Rect(0, 0, filled, height), NE: height / 2, NW: height / 2, SE: height / 2, SW: height / 2}.Op(gtx.Ops))
			}
			return layout.Dimensions{Size: image.Pt(width, height)}
		}),
	)
}

func workflowMaterialLabel(gtx layout.Context, th *material.Theme, textValue string, size unit.Sp, colorValue color.NRGBA, weight font.Weight, maxLines int) layout.Dimensions {
	style := material.Label(th, size, textValue)
	style.Color = colorValue
	style.Font.Weight = weight
	style.MaxLines = maxLines
	return style.Layout(gtx)
}

func workflowNodePhaseColor(spec desktopThemeTokens, phase workflowNodePhase) color.NRGBA {
	switch phase {
	case workflowNodePhaseRunning:
		return spec.Colors.accent
	case workflowNodePhaseSuccess:
		return spec.Colors.success
	case workflowNodePhaseWarning:
		return spec.Colors.warning
	case workflowNodePhaseError:
		return spec.Colors.danger
	default:
		return spec.Colors.textDim
	}
}

func workflowNodePhaseTextColor(spec desktopThemeTokens, phase workflowNodePhase) color.NRGBA {
	switch phase {
	case workflowNodePhaseRunning:
		return spec.Colors.accentText
	case workflowNodePhaseSuccess:
		return spec.Colors.successText
	case workflowNodePhaseWarning:
		return spec.Colors.warningText
	case workflowNodePhaseError:
		return spec.Colors.dangerText
	default:
		return spec.Colors.textDim
	}
}

func workflowPortColor(spec desktopThemeTokens, kind workflowPortKind) color.NRGBA {
	switch kind {
	case workflowPortText:
		return spec.Colors.warning
	case workflowPortImage:
		return spec.Colors.accent
	case workflowPortJob:
		return spec.Colors.success
	default:
		return spec.Colors.textDim
	}
}

func workflowPortIndex(ports []workflowPortModel, id string) int {
	for idx, port := range ports {
		if port.ID == id {
			return idx
		}
	}
	return 0
}

func workflowPortPixelY(gtx layout.Context, rect image.Rectangle, index int, visualScale float32) int {
	metric := workflowNodeScaledMetric(gtx.Metric, visualScale)
	return rect.Min.Y + metric.Dp(unit.Dp(58+index*25))
}

func paintWorkflowRectOutline(gtx layout.Context, rect image.Rectangle, radius int, width int, colorValue color.NRGBA) {
	if width <= 0 || rect.Empty() {
		return
	}
	outer := clip.RRect{Rect: rect, NE: radius, NW: radius, SE: radius, SW: radius}
	paint.FillShape(gtx.Ops, colorValue, clip.Stroke{Path: outer.Path(gtx.Ops), Width: float32(width)}.Op())
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
