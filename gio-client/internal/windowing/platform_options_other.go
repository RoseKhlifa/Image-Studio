//go:build !darwin || ios

package windowing

import "gioui.org/app"

func platformWindowOptions() []app.Option {
	return nil
}
