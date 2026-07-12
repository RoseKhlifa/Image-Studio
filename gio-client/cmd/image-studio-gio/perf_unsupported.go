//go:build !windows && (!linux || android) && (!darwin || ios)

package main

import (
	"fmt"
	"io"
)

func runPerfCommand(_ []string, _ io.Writer, _ io.Writer) (int, error) {
	return 1, fmt.Errorf("perf commands are only supported on Windows, macOS, and Linux")
}
