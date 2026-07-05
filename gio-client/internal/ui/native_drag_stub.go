//go:build !darwin

package ui

import "fmt"

func beginNativeFileDragDarwin(_ uintptr, _ string) error {
	return fmt.Errorf("当前平台不支持原生文件拖出")
}
