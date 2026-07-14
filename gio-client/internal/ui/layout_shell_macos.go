package ui

import (
	"gioui.org/layout"
	"gioui.org/unit"
)

func fitVisiblePaneWidths(available int, contract simplePaneContract, metric func(unit.Dp) int, showLeft bool, showRight bool) simplePaneWidths {
	preferredLeft := 0
	preferredRight := 0
	minimumLeft := 0
	minimumRight := 0
	if showLeft {
		preferredLeft = metric(contract.preferredLeft)
		minimumLeft = metric(contract.minimumLeft)
	}
	if showRight {
		preferredRight = metric(contract.preferredRight)
		minimumRight = metric(contract.minimumRight)
	}
	minimumCenter := metric(contract.minimumCenter)
	return fitSimplePaneWidths(available, preferredLeft, preferredRight, minimumLeft, minimumRight, minimumCenter)
}

func (a *App) layoutMacSimpleBody(gtx layout.Context, snap snapshot) layout.Dimensions {
	spec := desktopThemeSpec(desktopStyleMacOS, a.resolvedThemeMode)
	showLeft := !a.macSidebarHidden
	showRight := !a.macInspectorHidden
	contract := simplePaneContractForStyle(spec.Style, spec.Metrics, false)
	widths := fitVisiblePaneWidths(gtx.Constraints.Max.X, contract, func(value unit.Dp) int {
		return gtx.Dp(value)
	}, showLeft, showRight)

	children := make([]layout.FlexChild, 0, 3)
	if showLeft {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedPixelWidth(gtx, widths.left, func(gtx layout.Context) layout.Dimensions {
				return a.layoutControls(gtx, snap)
			})
		}))
	}
	children = append(children, layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
		return a.layoutCanvas(gtx, snap)
	}))
	if showRight {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return fixedPixelWidth(gtx, widths.right, func(gtx layout.Context) layout.Dimensions {
				return a.layoutHistoryAndLogs(gtx, snap)
			})
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}
