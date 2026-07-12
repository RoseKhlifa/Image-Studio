//go:build darwin && !ios

package ui

import "gioui.org/app"

func (a *App) handlePlatformViewEvent(ev app.ViewEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if view, ok := any(ev).(app.AppKitViewEvent); ok {
		if view.Valid() {
			a.darwinAppKitView = view.View
		} else {
			a.darwinAppKitView = 0
		}
	}
}
