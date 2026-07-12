package windowing

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"gioui.org/unit"
)

func TestRequestNormalizationAndKey(t *testing.T) {
	raw := Request{
		Role:        Role("  CANVAS "),
		WorkspaceID: "  WS-42  ",
		Title:       "  Detached Canvas  ",
		Size:        DpSize{Width: unit.Dp(900), Height: unit.Dp(700)},
	}
	normalized, err := raw.Normalized()
	if err != nil {
		t.Fatalf("normalize request: %v", err)
	}
	if normalized.Role != RoleCanvas {
		t.Fatalf("role=%q want %q", normalized.Role, RoleCanvas)
	}
	if normalized.WorkspaceID != "WS-42" {
		t.Fatalf("workspace=%q want WS-42", normalized.WorkspaceID)
	}
	if normalized.Title != "Detached Canvas" {
		t.Fatalf("title=%q want Detached Canvas", normalized.Title)
	}
	key, err := raw.Key()
	if err != nil {
		t.Fatalf("request key: %v", err)
	}
	if key != (Key{Role: RoleCanvas, WorkspaceID: "WS-42"}) {
		t.Fatalf("key=%+v", key)
	}
	if key.String() != "canvas:WS-42" {
		t.Fatalf("key string=%q", key.String())
	}
}

func TestRequestNormalizationRejectsInvalidValues(t *testing.T) {
	tests := []Request{
		{Role: "unknown"},
		{Role: RoleCanvas, Size: DpSize{Width: 640}},
		{Role: RoleCanvas, MinSize: DpSize{Width: 640, Height: -1}},
	}
	for _, request := range tests {
		if _, err := request.Normalized(); err == nil {
			t.Fatalf("Normalized(%+v) succeeded, want error", request)
		}
	}
}

func TestRegistryDeduplicatesNormalizedKey(t *testing.T) {
	registry := newRegistry[*int]()
	first, second := new(int), new(int)
	stored, actual, loaded, err := registry.loadOrStore(Request{
		Role:        Role(" CANVAS "),
		WorkspaceID: " workspace-1 ",
		Title:       "First title",
	}, first)
	if err != nil || loaded {
		t.Fatalf("first loadOrStore loaded=%v err=%v", loaded, err)
	}
	if actual != first || stored.Role != RoleCanvas {
		t.Fatalf("first registration actual=%p stored=%+v", actual, stored)
	}

	_, actual, loaded, err = registry.loadOrStore(Request{
		Role:        RoleCanvas,
		WorkspaceID: "workspace-1",
		Title:       "Replacement title",
		TopMost:     true,
	}, second)
	if err != nil || !loaded {
		t.Fatalf("duplicate loadOrStore loaded=%v err=%v", loaded, err)
	}
	if actual != first {
		t.Fatalf("duplicate returned %p want first %p", actual, first)
	}
	if registry.count() != 1 {
		t.Fatalf("count=%d want 1", registry.count())
	}
	requests := registry.requests()
	if len(requests) != 1 || requests[0].Title != "First title" || requests[0].TopMost {
		t.Fatalf("requests=%+v", requests)
	}
}

func TestRegistryDeleteRequiresRegisteredValue(t *testing.T) {
	registry := newRegistry[*int]()
	registered, other := new(int), new(int)
	request := Request{Role: RoleConsole, WorkspaceID: " workspace-2 "}
	if _, _, loaded, err := registry.loadOrStore(request, registered); err != nil || loaded {
		t.Fatalf("loadOrStore loaded=%v err=%v", loaded, err)
	}
	if registry.delete(request, other) {
		t.Fatal("delete accepted a different registration value")
	}
	if registry.count() != 1 {
		t.Fatalf("count after rejected delete=%d want 1", registry.count())
	}
	if !registry.delete(request, registered) {
		t.Fatal("delete rejected the registered value")
	}
	if registry.count() != 0 {
		t.Fatalf("count after delete=%d want 0", registry.count())
	}
}

func TestRegistryConcurrentSafety(t *testing.T) {
	registry := newRegistry[*int]()
	const callers = 128
	values := make([]int, callers)
	actuals := make(chan *int, callers)
	var loadedCount atomic.Int64
	var wait sync.WaitGroup
	for idx := 0; idx < callers; idx++ {
		idx := idx
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, actual, loaded, err := registry.loadOrStore(Request{
				Role:        Role(" WORKSPACE "),
				WorkspaceID: " shared ",
			}, &values[idx])
			if err != nil {
				t.Errorf("loadOrStore: %v", err)
				return
			}
			if loaded {
				loadedCount.Add(1)
			}
			actuals <- actual
		}()
	}
	wait.Wait()
	close(actuals)
	if got := loadedCount.Load(); got != callers-1 {
		t.Fatalf("loaded count=%d want %d", got, callers-1)
	}
	var registered *int
	for actual := range actuals {
		if registered == nil {
			registered = actual
			continue
		}
		if actual != registered {
			t.Fatalf("actual=%p want registered %p", actual, registered)
		}
	}
	if registry.count() != 1 {
		t.Fatalf("count=%d want 1", registry.count())
	}

	for idx := 0; idx < 32; idx++ {
		idx := idx
		wait.Add(1)
		go func() {
			defer wait.Done()
			request := Request{Role: RoleProgress, WorkspaceID: fmt.Sprintf("ws-%02d", idx)}
			_, _, _, err := registry.loadOrStore(request, &values[idx])
			if err != nil {
				t.Errorf("store %s: %v", request.WorkspaceID, err)
			}
			_ = registry.requests()
			_ = registry.count()
		}()
	}
	wait.Wait()
	if registry.count() != 33 {
		t.Fatalf("count after distinct registrations=%d want 33", registry.count())
	}
	if !registry.delete(Request{Role: RoleWorkspace, WorkspaceID: "shared"}, registered) {
		t.Fatal("concurrent registration could not be deleted")
	}
}
