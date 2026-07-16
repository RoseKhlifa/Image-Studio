//go:build !darwin && !windows

package backend

import "errors"

func beginNativeFileDrag(_ string) error {
	return errors.New("当前平台不支持原生文件拖出")
}
