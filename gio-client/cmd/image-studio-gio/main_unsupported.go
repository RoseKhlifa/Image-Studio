//go:build !windows && (!linux || android) && (!darwin || ios)

package main

import (
	"fmt"
	"os"
)

func main() {
	if handled, exitCode, err := runCLICommand(os.Args[1:], os.Stdout, os.Stderr); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(exitCode)
	}
	fmt.Fprintln(os.Stderr, "Image Studio Gio client is supported on Windows, macOS, and Linux.")
	os.Exit(1)
}
