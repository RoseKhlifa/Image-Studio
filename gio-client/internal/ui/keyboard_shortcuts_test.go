package ui

import (
	"testing"

	"gioui.org/io/key"
)

func TestPerformGlobalShortcutCreatesAndClosesWorkspace(t *testing.T) {
	isolateGioStableDataRoot(t)
	app := New()
	if len(app.workspaces) != 1 {
		t.Fatalf("initial workspaces=%d want 1", len(app.workspaces))
	}

	if !app.performGlobalShortcut("N", key.ModCtrl, false) {
		t.Fatal("Ctrl+N should be handled")
	}
	if len(app.workspaces) != 2 {
		t.Fatalf("after Ctrl+N workspaces=%d want 2", len(app.workspaces))
	}

	if !app.performGlobalShortcut("T", key.ModCtrl, false) {
		t.Fatal("Ctrl+T should be handled")
	}
	if len(app.workspaces) != 3 {
		t.Fatalf("after Ctrl+T workspaces=%d want 3", len(app.workspaces))
	}

	active := app.activeWorkspaceID
	if !app.performGlobalShortcut("W", key.ModCtrl, false) {
		t.Fatal("Ctrl+W should be handled")
	}
	if len(app.workspaces) != 2 {
		t.Fatalf("after Ctrl+W workspaces=%d want 2", len(app.workspaces))
	}
	if app.activeWorkspaceID == active {
		t.Fatalf("active workspace=%q should change after closing active tab", app.activeWorkspaceID)
	}
}

func TestPerformGlobalShortcutIgnoresWorkspaceActionsWhileTyping(t *testing.T) {
	isolateGioStableDataRoot(t)
	app := New()
	if app.performGlobalShortcut("N", key.ModCtrl, true) {
		t.Fatal("Ctrl+N should not be handled while typing")
	}
	if len(app.workspaces) != 1 {
		t.Fatalf("workspaces=%d want 1", len(app.workspaces))
	}
}

func TestPerformGlobalShortcutTogglesFullscreenOnMacStyleCombo(t *testing.T) {
	app := &App{}
	if !app.performGlobalShortcut("F", key.ModCtrl|key.ModCommand, false) {
		t.Fatal("Ctrl+Cmd+F should be handled")
	}
	if !app.fullscreen {
		t.Fatal("fullscreen should toggle on")
	}
}

func TestPerformGlobalShortcutTogglesFullscreenOnF11(t *testing.T) {
	app := &App{}
	if !app.performGlobalShortcut(key.NameF11, 0, false) {
		t.Fatal("F11 should be handled")
	}
	if !app.fullscreen {
		t.Fatal("fullscreen should toggle on")
	}
}
