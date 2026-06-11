//go:build !windows

package compat

import (
	"os"
	"path/filepath"
)

var stableDataRootFunc = stableDataRootImpl

func StableDataRoot() (string, error) {
	return stableDataRootFunc()
}

func stableDataRootImpl() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "image-studio-output"), nil
	}
	root := filepath.Join(cfg, "image-studio")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return root, nil
}

func DefaultOutputDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "image-studio-output")
	}
	return filepath.Join(home, "Pictures", "Image Studio")
}
