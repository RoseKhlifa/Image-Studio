//go:build windows

package compat

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	windowsRegistryPath          = `Software\YuanHua\Image Studio`
	windowsRegistryDataRootValue = "DataRoot"
	windowsRegistrySchemaValue   = "DataRootSchema"
	windowsRegistrySchemaVersion = uint32(1)
)

var stableDataRootFunc = stableDataRootImpl

func StableDataRoot() (string, error) {
	stableDataRootFuncMu.RLock()
	fn := stableDataRootFunc
	stableDataRootFuncMu.RUnlock()
	return fn()
}

func stableDataRootImpl() (string, error) {
	if root, err := readWindowsRegistryDataRoot(); err == nil && strings.TrimSpace(root) != "" {
		if err := os.MkdirAll(root, 0o700); err != nil {
			return "", err
		}
		return root, nil
	}
	root, err := documentsDataRoot()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	if err := writeWindowsRegistryDataRoot(root); err != nil {
		return root, nil
	}
	return root, nil
}

func DefaultOutputDir() string {
	root, err := StableDataRoot()
	if err != nil {
		return filepath.Join(".", "image-studio-output")
	}
	return root
}

func documentsDataRoot() (string, error) {
	docs, err := windows.KnownFolderPath(windows.FOLDERID_Documents, windows.KF_FLAG_DEFAULT)
	if err != nil || strings.TrimSpace(docs) == "" {
		home, homeErr := os.UserHomeDir()
		if homeErr != nil {
			return filepath.Join(".", "Image Studio"), nil
		}
		docs = filepath.Join(home, "Documents")
	}
	return filepath.Join(docs, "Image Studio"), nil
}

func readWindowsRegistryDataRoot() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, windowsRegistryPath, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	value, _, err := key.GetStringValue(windowsRegistryDataRootValue)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func writeWindowsRegistryDataRoot(root string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, windowsRegistryPath, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetStringValue(windowsRegistryDataRootValue, root); err != nil {
		return err
	}
	return key.SetDWordValue(windowsRegistrySchemaValue, windowsRegistrySchemaVersion)
}
