//go:build darwin && !ios

package windowing

import "gioui.org/app"

func platformWindowOptions() []app.Option {
	return []app.Option{app.FullscreenAuxiliary(true)}
}
