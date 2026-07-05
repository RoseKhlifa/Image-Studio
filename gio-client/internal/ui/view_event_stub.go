//go:build !darwin

package ui

import "gioui.org/app"

func (a *App) handlePlatformViewEvent(_ app.ViewEvent) {}
