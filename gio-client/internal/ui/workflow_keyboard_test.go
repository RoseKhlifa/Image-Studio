package ui

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

func TestWorkflowNodeCanBeSelectedFromKeyboard(t *testing.T) {
	var (
		ops      op.Ops
		router   input.Router
		view     workflowCanvasViewState
		selected string
	)
	data := workflowCanvasData{
		Graph:     defaultWorkflowGraph(),
		Selected:  "prompt",
		Workspace: "workspace-1",
	}
	render := func() {
		ops.Reset()
		gtx := layout.Context{
			Ops:         &ops,
			Source:      router.Source(),
			Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
			Constraints: layout.Exact(image.Pt(1280, 760)),
		}
		view.Layout(gtx, material.NewTheme(), desktopThemeSpec(desktopStyleWindows, desktopColorModeDark), data, workflowCanvasCallbacks{
			Select: func(nodeID string) { selected = nodeID },
		})
		router.Frame(&ops)
	}

	render()
	interaction := view.interaction("generate")
	router.Source().Execute(key.FocusCmd{Tag: interaction})
	render()
	router.Queue(
		key.Event{Name: key.NameReturn, State: key.Press},
		key.Event{Name: key.NameReturn, State: key.Release},
	)
	render()

	if selected != "generate" {
		t.Fatalf("keyboard selected %q want generate", selected)
	}
}
