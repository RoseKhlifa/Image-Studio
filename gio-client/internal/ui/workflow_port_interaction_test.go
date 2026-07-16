package ui

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestWorkflowCanvasDragConnectsOutputToCompatibleInput(t *testing.T) {
	graph := defaultWorkflowGraph()
	var (
		ops       op.Ops
		router    input.Router
		view      workflowCanvasViewState
		toggled   []workflowEdgeModel
		moveCount int
		moveStart int
		moveEnd   int
	)
	metric := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	viewport := image.Pt(1280, 760)
	spec := desktopThemeSpec(desktopStyleWindows, desktopColorModeDark)
	view.zoom = 0.7
	view.offset = image.Pt(34, 21)
	data := workflowCanvasData{Graph: graph, Selected: "prompt", Workspace: "ws-one"}
	callbacks := workflowCanvasCallbacks{
		MoveStart: func(string) { moveStart++ },
		Move:      func(string, image.Point) { moveCount++ },
		MoveEnd:   func(string) { moveEnd++ },
		RewireConnection: func(previous *workflowEdgeModel, replacement *workflowEdgeModel) {
			if previous != nil || replacement == nil {
				t.Fatalf("unexpected rewire previous=%+v replacement=%+v", previous, replacement)
			}
			edge := *replacement
			toggled = append(toggled, edge)
		},
	}
	render := func() {
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      metric,
			Constraints: layout.Exact(viewport),
		}
		view.Layout(gtx, material.NewTheme(), spec, data, callbacks)
		router.Frame(&ops)
	}

	render()
	gtx := layout.Context{Ops: new(op.Ops), Metric: metric, Constraints: layout.Exact(viewport)}
	prompt, _ := graph.node("prompt")
	generate, _ := graph.node("generate")
	from, ok := workflowPortCenter(gtx, prompt, "text", true, spec, &view)
	if !ok {
		t.Fatal("prompt output center missing")
	}
	to, ok := workflowPortCenter(gtx, generate, "prompt", false, spec, &view)
	if !ok {
		t.Fatal("generate input center missing")
	}

	router.Queue(
		pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Position: f32.Pt(float32(from.X), float32(from.Y)), PointerID: 1},
		pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(from.X), float32(from.Y)), PointerID: 1},
	)
	render()
	if !view.connection.active {
		t.Fatal("port press did not start a connection drag")
	}

	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(to.X), float32(to.Y)), PointerID: 1})
	render()
	if view.connection.position != to {
		t.Fatalf("drag position=%v want %v", view.connection.position, to)
	}

	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(float32(to.X), float32(to.Y)), PointerID: 1})
	render()
	if view.connection.active {
		t.Fatal("connection drag remained active after release")
	}
	if len(toggled) != 1 {
		t.Fatalf("toggle callbacks=%d want 1", len(toggled))
	}
	want := workflowEdgeModel{FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"}
	if workflowEdgeID(toggled[0]) != workflowEdgeID(want) {
		t.Fatalf("toggled edge=%+v want %+v", toggled[0], want)
	}
	if moveCount != 0 || moveStart != 0 || moveEnd != 0 {
		t.Fatalf("port drag also started node movement: start=%d move=%d end=%d", moveStart, moveCount, moveEnd)
	}
}

func TestWorkflowCanvasNodeDragEmitsOneMoveTransaction(t *testing.T) {
	graph := defaultWorkflowGraph()
	var (
		ops       op.Ops
		router    input.Router
		view      workflowCanvasViewState
		moveStart int
		moveEnd   int
		positions []image.Point
	)
	metric := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	viewport := image.Pt(900, 600)
	spec := desktopThemeSpec(desktopStyleWindows, desktopColorModeDark)
	render := func() {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: metric, Constraints: layout.Exact(viewport)}
		view.Layout(gtx, material.NewTheme(), spec, workflowCanvasData{Graph: graph, Workspace: "ws-one"}, workflowCanvasCallbacks{
			MoveStart: func(nodeID string) {
				if nodeID != "prompt" {
					t.Fatalf("move start node=%q", nodeID)
				}
				moveStart++
			},
			Move: func(nodeID string, position image.Point) {
				if nodeID != "prompt" {
					t.Fatalf("move node=%q", nodeID)
				}
				positions = append(positions, position)
			},
			MoveEnd: func(nodeID string) {
				if nodeID != "prompt" {
					t.Fatalf("move end node=%q", nodeID)
				}
				moveEnd++
			},
		})
		router.Frame(&ops)
	}

	render()
	start := image.Pt(160, 112)
	first := image.Pt(190, 132)
	second := image.Pt(230, 162)
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(start.X), float32(start.Y)), PointerID: 4})
	render()
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(first.X), float32(first.Y)), PointerID: 4})
	render()
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(second.X), float32(second.Y)), PointerID: 4})
	render()
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(float32(second.X), float32(second.Y)), PointerID: 4})
	render()

	if moveStart != 1 || moveEnd != 1 || len(positions) != 2 {
		t.Fatalf("node drag start=%d moves=%d end=%d", moveStart, len(positions), moveEnd)
	}
	if positions[len(positions)-1] == graph.Nodes[0].Position {
		t.Fatalf("node drag final position did not change: %v", positions[len(positions)-1])
	}
}

func TestWorkflowCanvasDragToBlankCancelsConnection(t *testing.T) {
	graph := defaultWorkflowGraph()
	var (
		ops    op.Ops
		router input.Router
		view   workflowCanvasViewState
		count  int
	)
	metric := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	viewport := image.Pt(900, 600)
	spec := desktopThemeSpec(desktopStyleWindows, desktopColorModeDark)
	render := func() {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: metric, Constraints: layout.Exact(viewport)}
		view.Layout(gtx, material.NewTheme(), spec, workflowCanvasData{Graph: graph, Workspace: "ws-one"}, workflowCanvasCallbacks{
			RewireConnection: func(*workflowEdgeModel, *workflowEdgeModel) { count++ },
		})
		router.Frame(&ops)
	}

	render()
	gtx := layout.Context{Ops: new(op.Ops), Metric: metric, Constraints: layout.Exact(viewport)}
	prompt, _ := graph.node("prompt")
	from, _ := workflowPortCenter(gtx, prompt, "text", true, spec, &view)
	blank := image.Pt(12, 12)
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(from.X), float32(from.Y)), PointerID: 2})
	render()
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(blank.X), float32(blank.Y)), PointerID: 2})
	render()
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(float32(blank.X), float32(blank.Y)), PointerID: 2})
	render()

	if view.connection.active || count != 0 {
		t.Fatalf("blank drop active=%t callbacks=%d", view.connection.active, count)
	}
}

func TestWorkflowCanvasDragConnectedInputToBlankDisconnects(t *testing.T) {
	graph := defaultWorkflowGraph()
	var (
		ops         op.Ops
		router      input.Router
		view        workflowCanvasViewState
		previous    *workflowEdgeModel
		replacement *workflowEdgeModel
	)
	metric := unit.Metric{PxPerDp: 1, PxPerSp: 1}
	viewport := image.Pt(900, 600)
	spec := desktopThemeSpec(desktopStyleWindows, desktopColorModeDark)
	render := func() {
		ops.Reset()
		gtx := layout.Context{Ops: &ops, Source: router.Source(), Metric: metric, Constraints: layout.Exact(viewport)}
		view.Layout(gtx, material.NewTheme(), spec, workflowCanvasData{Graph: graph, Workspace: "ws-one"}, workflowCanvasCallbacks{
			RewireConnection: func(old *workflowEdgeModel, next *workflowEdgeModel) {
				previous = old
				replacement = next
			},
		})
		router.Frame(&ops)
	}

	render()
	gtx := layout.Context{Ops: new(op.Ops), Metric: metric, Constraints: layout.Exact(viewport)}
	generate, _ := graph.node("generate")
	inputCenter, _ := workflowPortCenter(gtx, generate, "prompt", false, spec, &view)
	blank := image.Pt(20, 20)
	router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(inputCenter.X), float32(inputCenter.Y)), PointerID: 3})
	render()
	if !view.connection.active || !view.connection.hasOriginal {
		t.Fatalf("input press connection=%+v want existing edge drag", view.connection)
	}
	router.Queue(pointer.Event{Kind: pointer.Move, Source: pointer.Mouse, Buttons: pointer.ButtonPrimary, Position: f32.Pt(float32(blank.X), float32(blank.Y)), PointerID: 3})
	render()
	router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, Position: f32.Pt(float32(blank.X), float32(blank.Y)), PointerID: 3})
	render()

	want := workflowEdgeModel{FromNode: "prompt", FromPort: "text", ToNode: "generate", ToPort: "prompt"}
	if previous == nil || workflowEdgeID(*previous) != workflowEdgeID(want) || replacement != nil {
		t.Fatalf("disconnect previous=%+v replacement=%+v want previous=%+v", previous, replacement, want)
	}
}
