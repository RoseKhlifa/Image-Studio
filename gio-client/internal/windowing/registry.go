package windowing

import (
	"sort"
	"sync"
)

// registry deliberately contains no app.Window state. It owns only normalized
// identities and comparable lifecycle handles, which keeps deduplication and
// teardown independently testable without creating GUI windows.
type registry[T comparable] struct {
	mu      sync.RWMutex
	entries map[Key]registryEntry[T]
}

type registryEntry[T comparable] struct {
	request Request
	value   T
}

func newRegistry[T comparable]() *registry[T] {
	return &registry[T]{entries: make(map[Key]registryEntry[T])}
}

func (r *registry[T]) loadOrStore(request Request, value T) (Request, T, bool, error) {
	normalized, err := request.Normalized()
	if err != nil {
		var zero T
		return Request{}, zero, false, err
	}
	key := normalized.key()
	r.mu.Lock()
	defer r.mu.Unlock()
	if entry, ok := r.entries[key]; ok {
		return entry.request, entry.value, true, nil
	}
	r.entries[key] = registryEntry[T]{request: normalized, value: value}
	return normalized, value, false, nil
}

func (r *registry[T]) load(role Role, workspaceID string) (T, bool) {
	request, err := (Request{Role: role, WorkspaceID: workspaceID}).Normalized()
	if err != nil {
		var zero T
		return zero, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	entry, ok := r.entries[request.key()]
	return entry.value, ok
}

func (r *registry[T]) delete(request Request, expected T) bool {
	normalized, err := request.Normalized()
	if err != nil {
		return false
	}
	key := normalized.key()
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.entries[key]
	if !ok || entry.value != expected {
		return false
	}
	delete(r.entries, key)
	return true
}

func (r *registry[T]) count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.entries)
}

func (r *registry[T]) requests() []Request {
	r.mu.RLock()
	requests := make([]Request, 0, len(r.entries))
	for _, entry := range r.entries {
		requests = append(requests, entry.request)
	}
	r.mu.RUnlock()
	sort.Slice(requests, func(i, j int) bool {
		left, right := requests[i].key(), requests[j].key()
		if left.Role != right.Role {
			return left.Role < right.Role
		}
		return left.WorkspaceID < right.WorkspaceID
	})
	return requests
}

func (r *registry[T]) values() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()
	values := make([]T, 0, len(r.entries))
	for _, entry := range r.entries {
		values = append(values, entry.value)
	}
	return values
}
