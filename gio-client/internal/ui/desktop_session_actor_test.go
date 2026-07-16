package ui

import (
	"image"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"image-studio/gio-client/internal/windowing"
)

type actorTestDesktopWindows struct {
	mu            sync.Mutex
	opened        []windowing.Request
	invalidations int
}

func (windows *actorTestDesktopWindows) Open(request windowing.Request) (bool, error) {
	windows.mu.Lock()
	windows.opened = append(windows.opened, request)
	windows.mu.Unlock()
	return true, nil
}

func (*actorTestDesktopWindows) CloseAll() int { return 0 }

func (windows *actorTestDesktopWindows) InvalidateAll() int {
	windows.mu.Lock()
	windows.invalidations++
	count := len(windows.opened)
	windows.mu.Unlock()
	return count
}

func (windows *actorTestDesktopWindows) Count() int {
	windows.mu.Lock()
	defer windows.mu.Unlock()
	return len(windows.opened)
}

func (windows *actorTestDesktopWindows) Requests() []windowing.Request {
	windows.mu.Lock()
	defer windows.mu.Unlock()
	return append([]windowing.Request(nil), windows.opened...)
}

func TestDesktopSessionActorProcessesCommandsWithoutMainFrameEvent(t *testing.T) {
	windows := new(actorTestDesktopWindows)
	root := &App{
		desktopWindows:             windows,
		desktopCommands:            make(chan desktopCommand, 16),
		desktopPendingMoves:        map[string]desktopCommand{},
		activeWorkspaceID:          "ws-1",
		workspaces:                 []workspaceState{{ID: "ws-1", Name: "Actor", Mode: "generate", Size: "auto", Quality: "auto", OutputFormat: "png"}},
		workflowGraphs:             map[string]workflowGraphModel{"ws-1": defaultWorkflowGraph()},
		workflowSelectedNodes:      map[string]string{"ws-1": "generate"},
		mode:                       "generate",
		batchCount:                 1,
		loopTotalCount:             1,
		batchConcurrency:           1,
		loopConcurrency:            1,
		desktopQueuedWorkspaceRuns: nil,
	}
	root.apiKeyInput.SetText("test-key")
	root.baseURLInput.SetText("https://example.test")

	cancelled := make(chan struct{}, 1)
	root.running = true
	root.cancel = func() { cancelled <- struct{}{} }
	var wakes atomic.Int32
	var raises atomic.Int32
	root.raiseMainWindow = func() { raises.Add(1) }
	session := root.startDesktopSessionActor(func() { wakes.Add(1) })
	defer root.stopDesktopSessionActor(session)

	commands := []desktopCommand{
		{Kind: desktopCommandCancel},
		{Kind: desktopCommandRun, WorkspaceID: "ws-1"},
		{Kind: desktopCommandMoveNode, WorkspaceID: "ws-1", NodeID: "prompt", Position: image.Pt(410, 230)},
		{Kind: desktopCommandOpenWindow, WorkspaceID: "ws-1", WindowRole: windowing.RoleConsole},
		{Kind: desktopCommandRaiseMain},
	}
	for _, command := range commands {
		if !root.enqueueDesktopCommand(command) {
			t.Fatalf("enqueue command kind %d", command.Kind)
		}
	}
	if wakes.Load() == 0 {
		t.Fatal("detached commands did not request an event-loop wakeup")
	}

	// This directly models Gio's inactive-window wakeup event. No layout call or
	// FrameEvent is involved in draining commands or rebuilding publication.
	if !root.handleDesktopSessionEvent(session) {
		t.Fatal("session actor rejected its wakeup event")
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("cancel command was not processed")
	}
	if root.isRunning() {
		t.Fatal("cancel command left the run active")
	}
	node, _ := root.workflowGraph("ws-1").node("prompt")
	if node.Position != image.Pt(410, 230) {
		t.Fatalf("move command position=%v", node.Position)
	}
	if windows.Count() != 1 || windows.Requests()[0].Role != windowing.RoleConsole {
		t.Fatalf("open command requests=%+v", windows.Requests())
	}
	if raises.Load() != 1 {
		t.Fatalf("raise command count=%d", raises.Load())
	}
	publication := root.desktopSnapshot()
	if publication.Revision == 0 || publication.Status != "已取消" {
		t.Fatalf("publication was not refreshed: %+v", publication)
	}
	if !desktopPublicationHasLog(publication, "请先填写提示词") {
		t.Fatalf("run command did not reach validation: logs=%q", publication.Logs)
	}
}

func TestDesktopSessionActorStopRejectsLateWork(t *testing.T) {
	root := &App{
		desktopCommands:     make(chan desktopCommand, 1),
		desktopPendingMoves: map[string]desktopCommand{},
	}
	var wakes atomic.Int32
	session := root.startDesktopSessionActor(func() { wakes.Add(1) })
	root.requestWakeup()
	if wakes.Load() != 1 {
		t.Fatalf("active wake count=%d", wakes.Load())
	}
	root.stopDesktopSessionActor(session)
	root.requestWakeup()
	if wakes.Load() != 1 {
		t.Fatalf("stopped actor woke again: %d", wakes.Load())
	}
	if root.enqueueDesktopCommand(desktopCommand{Kind: desktopCommandCancel}) {
		t.Fatal("stopped actor accepted a late command")
	}
	if root.handleDesktopSessionEvent(session) {
		t.Fatal("stopped actor handled a late event")
	}
}

func desktopPublicationHasLog(publication desktopPublication, part string) bool {
	for _, line := range publication.Logs {
		if strings.Contains(line, part) {
			return true
		}
	}
	return false
}
