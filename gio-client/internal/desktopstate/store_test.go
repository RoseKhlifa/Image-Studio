package desktopstate

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStoreLoadMissingReturnsDefault(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	state, err := NewStore(path).Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !reflect.DeepEqual(state, Default()) {
		t.Fatalf("missing file state=%#v want default=%#v", state, Default())
	}
}

func TestStoreSaveLoadRoundTripAndMonotonicMetadata(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", FileName)
	store := NewStore(path)
	store.now = func() time.Time { return time.UnixMilli(1_700_000_000_000) }

	input := Default()
	input.Preferences.InterfaceStyle = InterfaceStyleMacOS
	input.Preferences.ExperienceMode = ExperienceModeWorkflow
	input.Preferences.DefaultWindowLayout = WindowLayoutDual
	input.Preferences.AutoShowProgress = false
	input.Windows = []Window{{
		ID:          "canvas-1",
		Role:        WindowRoleCanvas,
		WorkspaceID: "workspace-1",
		WidthDp:     1200,
		HeightDp:    800,
		Mode:        WindowModeMaximized,
		Visible:     true,
	}}
	input.Workspaces = []Workspace{{
		ID:   "workspace-1",
		Name: "Campaign",
		Workflow: WorkflowGraph{
			Nodes:    []WorkflowNode{{ID: "generate", Kind: "image.generate", Properties: map[string]string{"prompt": "cover"}}},
			Edges:    []WorkflowEdge{},
			Viewport: Viewport{OffsetX: 12, OffsetY: -8, Zoom: 1.25},
		},
	}}

	first, err := store.Save(input)
	if err != nil {
		t.Fatalf("first Save() error: %v", err)
	}
	if first.Revision != 1 || first.UpdatedAt != 1_700_000_000_000 {
		t.Fatalf("first metadata revision=%d updatedAt=%d", first.Revision, first.UpdatedAt)
	}

	second, err := store.Save(input)
	if err != nil {
		t.Fatalf("second Save() error: %v", err)
	}
	if second.Revision != 2 || second.UpdatedAt != first.UpdatedAt+1 {
		t.Fatalf("second metadata revision=%d updatedAt=%d", second.Revision, second.UpdatedAt)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if !reflect.DeepEqual(loaded, second) {
		t.Fatalf("round trip mismatch\nloaded=%#v\nsaved=%#v", loaded, second)
	}
	if loaded.Preferences.AutoShowProgress {
		t.Fatal("explicit false preference was not preserved")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat() error: %v", err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("file mode=%o want 600", got)
		}
	}
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".desktop-state-*.tmp"))
	if err != nil {
		t.Fatalf("Glob() error: %v", err)
	}
	if len(temps) != 0 {
		t.Fatalf("temporary files were not cleaned: %v", temps)
	}
}

func TestStoreCorruptFileReturnsDefaultAndTypedError(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	state, err := NewStore(path).Load()
	if err == nil {
		t.Fatal("Load() error=nil want corrupt-state error")
	}
	if !IsCorrupt(err) {
		t.Fatalf("Load() error type=%T want *CorruptStateError", err)
	}
	var corrupt *CorruptStateError
	if !errors.As(err, &corrupt) || corrupt.Path != path {
		t.Fatalf("corrupt error=%#v", corrupt)
	}
	if !reflect.DeepEqual(state, Default()) {
		t.Fatalf("corrupt fallback=%#v want default", state)
	}
}

func TestStoreSaveRepairsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("null"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	store := NewStore(path)
	store.now = func() time.Time { return time.UnixMilli(1234) }
	saved, err := store.Save(Default())
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if saved.Revision != 1 || saved.UpdatedAt != 1234 {
		t.Fatalf("saved metadata=%#v", saved)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after repair error: %v", err)
	}
	if !reflect.DeepEqual(loaded, saved) {
		t.Fatalf("loaded=%#v saved=%#v", loaded, saved)
	}
}

func TestStoreUnsupportedVersionFallsBackWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	original := `{"schemaVersion":99,"revision":7,"updatedAt":10,"preferences":{},"windows":[],"workspaces":[]}`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	store := NewStore(path)
	state, err := store.Load()
	if err == nil {
		t.Fatal("Load() error=nil want unsupported-version error")
	}
	var unsupported *UnsupportedVersionError
	if !errors.As(err, &unsupported) || unsupported.Version != 99 {
		t.Fatalf("Load() error=%#v", err)
	}
	if !reflect.DeepEqual(state, Default()) {
		t.Fatalf("unsupported fallback=%#v want default", state)
	}
	if _, err := store.Save(Default()); err == nil {
		t.Fatal("Save() should not overwrite a newer schema")
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile() error: %v", readErr)
	}
	if string(data) != original {
		t.Fatalf("newer schema was overwritten: %s", data)
	}
}

func TestStoreRejectsEmptyPath(t *testing.T) {
	store := NewStore("")
	if _, err := store.Load(); err == nil || !strings.Contains(err.Error(), "path is empty") {
		t.Fatalf("Load() error=%v", err)
	}
	if _, err := store.Save(Default()); err == nil || !strings.Contains(err.Error(), "path is empty") {
		t.Fatalf("Save() error=%v", err)
	}
}

func TestLoadRejectsMultipleJSONDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), FileName)
	if err := os.WriteFile(path, []byte("{}\n{}"), 0o600); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	_, err := Load(path)
	if !IsCorrupt(err) {
		t.Fatalf("Load() error=%v want corrupt-state error", err)
	}
}

func TestStoreConcurrentSavesSerializeRevisions(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), FileName))
	store.now = func() time.Time { return time.UnixMilli(5000) }
	const saveCount = 12
	revisions := make(chan uint64, saveCount)
	errorsFound := make(chan error, saveCount)
	var wg sync.WaitGroup
	for index := 0; index < saveCount; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			saved, err := store.Save(Default())
			if err != nil {
				errorsFound <- err
				return
			}
			revisions <- saved.Revision
		}()
	}
	wg.Wait()
	close(errorsFound)
	close(revisions)
	for err := range errorsFound {
		t.Fatalf("concurrent Save() error: %v", err)
	}
	got := make([]int, 0, saveCount)
	for revision := range revisions {
		got = append(got, int(revision))
	}
	sort.Ints(got)
	for index, revision := range got {
		if want := index + 1; revision != want {
			t.Fatalf("revisions=%v; entry %d=%d want %d", got, index, revision, want)
		}
	}
}
