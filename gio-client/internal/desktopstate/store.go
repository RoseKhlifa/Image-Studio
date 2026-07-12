package desktopstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CorruptStateError struct {
	Path string
	Err  error
}

func (e *CorruptStateError) Error() string {
	return fmt.Sprintf("desktopstate: %s is corrupt; using defaults: %v", e.Path, e.Err)
}

func (e *CorruptStateError) Unwrap() error {
	return e.Err
}

type UnsupportedVersionError struct {
	Path    string
	Version int
}

func (e *UnsupportedVersionError) Error() string {
	return fmt.Sprintf("desktopstate: %s uses unsupported schema version %d", e.Path, e.Version)
}

func IsCorrupt(err error) bool {
	var target *CorruptStateError
	return errors.As(err, &target)
}

type Store struct {
	path string
	now  func() time.Time
	mu   sync.Mutex
}

func NewStore(path string) *Store {
	return &Store{
		path: filepath.Clean(path),
		now:  time.Now,
	}
}

func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func Load(path string) (State, error) {
	return NewStore(path).Load()
}

func Save(path string, state State) (State, error) {
	return NewStore(path).Save(state)
}

func (s *Store) Load() (State, error) {
	if err := s.validate(); err != nil {
		return Default(), err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return loadFile(s.path)
}

func (s *Store) Save(state State) (State, error) {
	if err := s.validate(); err != nil {
		return Normalize(state), err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	diskRevision, diskUpdatedAt, err := readMetadata(s.path)
	if err != nil {
		var corrupt *CorruptStateError
		if !errors.Is(err, os.ErrNotExist) && !errors.As(err, &corrupt) {
			return Normalize(state), err
		}
	}

	next := Normalize(state)
	next.Revision = max(next.Revision, diskRevision) + 1
	now := s.now().UnixMilli()
	if now <= diskUpdatedAt {
		now = diskUpdatedAt + 1
	}
	if now <= next.UpdatedAt {
		now = next.UpdatedAt + 1
	}
	next.UpdatedAt = now
	if err := saveFileAtomic(s.path, next); err != nil {
		return Normalize(state), err
	}
	return next, nil
}

func (s *Store) validate() error {
	if s == nil {
		return errors.New("desktopstate: nil store")
	}
	if s.path == "" || s.path == "." {
		return errors.New("desktopstate: state path is empty")
	}
	if s.now == nil {
		s.now = time.Now
	}
	return nil
}

func loadFile(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Default(), err
	}
	state, err := decodeState(path, data)
	if err != nil {
		return Default(), err
	}
	return Normalize(state), nil
}

func decodeState(path string, data []byte) (State, error) {
	if len(bytes.TrimSpace(data)) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return State{}, &CorruptStateError{Path: path, Err: errors.New("empty state document")}
	}
	state := Default()
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&state); err != nil {
		return State{}, &CorruptStateError{Path: path, Err: err}
	}
	if err := ensureDecoderEOF(decoder); err != nil {
		return State{}, &CorruptStateError{Path: path, Err: err}
	}
	if state.SchemaVersion > SchemaVersion {
		return State{}, &UnsupportedVersionError{Path: path, Version: state.SchemaVersion}
	}
	if state.SchemaVersion < 0 {
		return State{}, &CorruptStateError{Path: path, Err: fmt.Errorf("invalid schema version %d", state.SchemaVersion)}
	}
	return state, nil
}

func ensureDecoderEOF(decoder *json.Decoder) error {
	var extra json.RawMessage
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("multiple JSON documents")
}

func readMetadata(path string) (uint64, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	state, err := decodeState(path, data)
	if err != nil {
		return 0, 0, err
	}
	return state.Revision, state.UpdatedAt, nil
}

func saveFileAtomic(path string, state State) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("desktopstate: create state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("desktopstate: encode state: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".desktop-state-*.tmp")
	if err != nil {
		return fmt.Errorf("desktopstate: create temporary state file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("desktopstate: protect temporary state file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("desktopstate: write temporary state file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("desktopstate: sync temporary state file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("desktopstate: close temporary state file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("desktopstate: replace state file: %w", err)
	}
	committed = true
	return nil
}
