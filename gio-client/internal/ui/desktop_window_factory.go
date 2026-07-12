package ui

import (
	"errors"
	"fmt"

	"image-studio/gio-client/internal/windowing"
)

var errDesktopWindowRootRequired = errors.New("ui: desktop window root is required")

type desktopWindowFactory struct {
	root *App
}

// NewDesktopWindowFactory creates independent Gio views for detached desktop
// workspaces. The returned factory is safe to attach to windowing.Manager.
func NewDesktopWindowFactory(root *App) windowing.Factory {
	return &desktopWindowFactory{root: root}
}

func (factory *desktopWindowFactory) NewView(request windowing.Request) (windowing.View, error) {
	if factory == nil || factory.root == nil {
		return nil, errDesktopWindowRootRequired
	}
	normalized, err := request.Normalized()
	if err != nil {
		return nil, fmt.Errorf("ui: create desktop window view: %w", err)
	}
	return newDesktopWindowView(factory.root, normalized), nil
}

var _ windowing.Factory = (*desktopWindowFactory)(nil)
