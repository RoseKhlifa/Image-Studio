package main

import (
	"bufio"
	"go/build/constraint"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopBuildConstraintsExcludeMobileTargets(t *testing.T) {
	targets := []struct {
		name string
		tags map[string]bool
	}{
		{name: "windows", tags: map[string]bool{"windows": true}},
		{name: "linux", tags: map[string]bool{"linux": true}},
		{name: "android", tags: map[string]bool{"linux": true, "android": true}},
		{name: "macos", tags: map[string]bool{"darwin": true}},
		{name: "ios", tags: map[string]bool{"darwin": true, "ios": true}},
		{name: "freebsd", tags: map[string]bool{"freebsd": true}},
	}
	groups := []struct {
		name  string
		files []string
	}{
		{name: "main", files: []string{"main.go", "main_unsupported.go"}},
		{name: "perf", files: []string{"perf_supported.go", "perf_unsupported.go"}},
		{name: "promptipc", files: []string{
			filepath.Join("..", "..", "internal", "promptipc", "endpoint_windows.go"),
			filepath.Join("..", "..", "internal", "promptipc", "endpoint_unix.go"),
			filepath.Join("..", "..", "internal", "promptipc", "endpoint_unsupported.go"),
		}},
		{name: "promptscheme", files: []string{
			filepath.Join("..", "..", "internal", "promptscheme", "windows.go"),
			filepath.Join("..", "..", "internal", "promptscheme", "linux.go"),
			filepath.Join("..", "..", "internal", "promptscheme", "unsupported.go"),
		}},
		{name: "native-drag", files: []string{
			filepath.Join("..", "..", "internal", "ui", "native_drag_darwin.go"),
			filepath.Join("..", "..", "internal", "ui", "native_drag_stub.go"),
		}},
		{name: "view-event", files: []string{
			filepath.Join("..", "..", "internal", "ui", "view_event_darwin.go"),
			filepath.Join("..", "..", "internal", "ui", "view_event_stub.go"),
		}},
	}

	for _, target := range targets {
		target := target
		for _, group := range groups {
			group := group
			t.Run(target.name+"/"+group.name, func(t *testing.T) {
				selected := 0
				for _, path := range group.files {
					if buildConstraintMatches(t, path, target.tags) {
						selected++
					}
				}
				if selected != 1 {
					t.Fatalf("selected %d files from %v; want exactly one", selected, group.files)
				}
			})
		}
	}

	androidTags := map[string]bool{"linux": true, "android": true}
	for _, path := range []string{
		"main.go",
		"perf_supported.go",
		filepath.Join("..", "..", "internal", "promptipc", "endpoint_unix.go"),
		filepath.Join("..", "..", "internal", "promptscheme", "linux.go"),
		filepath.Join("..", "..", "internal", "ui", "native_drag_darwin.go"),
		filepath.Join("..", "..", "internal", "ui", "view_event_darwin.go"),
	} {
		if buildConstraintMatches(t, path, androidTags) {
			t.Fatalf("Android unexpectedly selects desktop implementation %s", path)
		}
	}
}

func buildConstraintMatches(t *testing.T, path string, tags map[string]bool) bool {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatalf("read build constraint from %s: %v", path, scanner.Err())
	}
	expression, err := constraint.Parse(scanner.Text())
	if err != nil {
		t.Fatalf("parse build constraint from %s: %v", path, err)
	}
	return expression.Eval(func(tag string) bool { return tags[tag] })
}
