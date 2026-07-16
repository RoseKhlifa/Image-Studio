package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeShutdownCloser struct {
	err     error
	calls   int
	onClose func()
}

func (fake *fakeShutdownCloser) Close() error {
	fake.calls++
	if fake.onClose != nil {
		fake.onClose()
	}
	return fake.err
}

type orderedShutdownController struct {
	onCloseAll func()
}

func (controller *orderedShutdownController) CloseAll() int {
	if controller.onCloseAll != nil {
		controller.onCloseAll()
	}
	return 0
}

func (*orderedShutdownController) Count() int { return 0 }

func TestShutdownDesktopResourcesClosesIPCBeforeWindows(t *testing.T) {
	var order []string
	resource := &fakeShutdownCloser{onClose: func() { order = append(order, "ipc") }}
	controller := &orderedShutdownController{onCloseAll: func() { order = append(order, "windows") }}
	if !shutdownDesktopResources(resource, controller, time.Second, time.Millisecond, nil) {
		t.Fatal("shutdownDesktopResources reported a timeout")
	}
	if got := strings.Join(order, ","); got != "ipc,windows" {
		t.Fatalf("shutdown order=%q want ipc,windows", got)
	}
}

func TestCloseShutdownResourceBeforeProcessExit(t *testing.T) {
	resource := &fakeShutdownCloser{}
	closeShutdownResource(resource, "prompt IPC server", nil)
	if resource.calls != 1 {
		t.Fatalf("Close calls=%d want 1", resource.calls)
	}
}

func TestCloseShutdownResourceLogsFailure(t *testing.T) {
	resource := &fakeShutdownCloser{err: errors.New("close failed")}
	var logs []string
	closeShutdownResource(resource, "prompt IPC server", func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
	})
	if resource.calls != 1 {
		t.Fatalf("Close calls=%d want 1", resource.calls)
	}
	if got := strings.Join(logs, "\n"); !strings.Contains(got, "failed to close prompt IPC server: close failed") {
		t.Fatalf("logs=%q want close failure", got)
	}
}

func TestCloseShutdownResourceAllowsNil(t *testing.T) {
	closeShutdownResource(nil, "prompt IPC server", nil)
}

type fakeShutdownController struct {
	mu            sync.Mutex
	counts        []int
	countCalls    int
	closeAllCalls int
}

func (fake *fakeShutdownController) CloseAll() int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.closeAllCalls++
	if len(fake.counts) == 0 {
		return 0
	}
	return fake.counts[0]
}

func (fake *fakeShutdownController) Count() int {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.countCalls++
	if len(fake.counts) == 0 {
		return 0
	}
	count := fake.counts[0]
	if len(fake.counts) > 1 {
		fake.counts = fake.counts[1:]
	}
	return count
}

func (fake *fakeShutdownController) calls() (closeAll int, count int) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return fake.closeAllCalls, fake.countCalls
}

func TestCloseDesktopWindowsAndWaitReturnsImmediatelyWithoutWindows(t *testing.T) {
	controller := &fakeShutdownController{counts: []int{0}}
	if !closeDesktopWindowsAndWait(controller, time.Hour, time.Hour, nil) {
		t.Fatal("closeDesktopWindowsAndWait reported a timeout with no windows")
	}
	closeAllCalls, countCalls := controller.calls()
	if closeAllCalls != 1 {
		t.Fatalf("CloseAll calls=%d want 1", closeAllCalls)
	}
	if countCalls != 1 {
		t.Fatalf("Count calls=%d want 1", countCalls)
	}
}

func TestCloseDesktopWindowsAndWaitObservesGradualUnregister(t *testing.T) {
	controller := &fakeShutdownController{counts: []int{3, 2, 1, 0}}
	var logs []string
	logf := func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	if !closeDesktopWindowsAndWait(controller, time.Second, time.Millisecond, logf) {
		t.Fatal("closeDesktopWindowsAndWait timed out before all windows unregistered")
	}
	closeAllCalls, countCalls := controller.calls()
	if closeAllCalls != 1 {
		t.Fatalf("CloseAll calls=%d want 1", closeAllCalls)
	}
	if countCalls != 4 {
		t.Fatalf("Count calls=%d want 4", countCalls)
	}
	if got := strings.Join(logs, "\n"); !strings.Contains(got, "all detached windows closed") {
		t.Fatalf("logs=%q want successful shutdown message", got)
	}
}

func TestCloseDesktopWindowsAndWaitTimesOut(t *testing.T) {
	controller := &fakeShutdownController{counts: []int{2}}
	var logs []string
	logf := func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	if closeDesktopWindowsAndWait(controller, 5*time.Millisecond, time.Millisecond, logf) {
		t.Fatal("closeDesktopWindowsAndWait reported success with registered windows")
	}
	closeAllCalls, countCalls := controller.calls()
	if closeAllCalls != 1 {
		t.Fatalf("CloseAll calls=%d want 1", closeAllCalls)
	}
	if countCalls < 2 {
		t.Fatalf("Count calls=%d want at least 2", countCalls)
	}
	if got := strings.Join(logs, "\n"); !strings.Contains(got, "timed out") || !strings.Contains(got, "2 window(s) remain") {
		t.Fatalf("logs=%q want timeout with remaining count", got)
	}
}
