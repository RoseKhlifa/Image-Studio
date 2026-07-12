//go:build !darwin || ios

package ui

import "gioui.org/app"

func (a *App) handlePlatformViewEvent(_ app.ViewEvent) {}
