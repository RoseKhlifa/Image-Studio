package ui

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

func imageFormatFromMIMEType(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg":
		return "jpeg"
	case "image/webp":
		return "webp"
	default:
		return "png"
	}
}

func imageMIMETypeForPath(path string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(path)))
	if mimeType, ok := client.SupportedImageMime[ext]; ok {
		return mimeType
	}
	return "image/png"
}

func virtualImageMIMEType(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, virtualImagePrefix) {
		return "", false
	}
	virtualImageMu.Lock()
	record, ok := virtualImageStore[path]
	if ok {
		record.lastAccessed = time.Now()
		virtualImageStore[path] = record
	}
	virtualImageMu.Unlock()
	if !ok || strings.TrimSpace(record.mimeType) == "" {
		return "", false
	}
	return record.mimeType, true
}

func clipboardImageDataForResult(snap snapshot) ([]byte, string, error) {
	path := strings.TrimSpace(snap.Result.SavedPath)
	if path == "" {
		path = strings.TrimSpace(snap.Result.Item.SavedPath)
	}
	if path != "" {
		if isVirtualImagePath(path) {
			imageB64, ok := readVirtualImageB64(path)
			if !ok || strings.TrimSpace(imageB64) == "" {
				return nil, "", fmt.Errorf("当前图片没有可复制内容")
			}
			data, err := base64.StdEncoding.DecodeString(imageB64)
			if err != nil {
				return nil, "", fmt.Errorf("decode clipboard image: %w", err)
			}
			mimeType, ok := virtualImageMIMEType(path)
			if !ok {
				mimeType = imageMIMETypeForPath(virtualImageDisplayName(path))
			}
			return data, mimeType, nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", err
		}
		return data, imageMIMETypeForPath(path), nil
	}
	imageB64 := strings.TrimSpace(snap.Result.Item.ImageB64)
	if imageB64 == "" {
		return nil, "", fmt.Errorf("当前没有图片可复制")
	}
	data, err := base64.StdEncoding.DecodeString(imageB64)
	if err != nil {
		return nil, "", fmt.Errorf("decode clipboard image: %w", err)
	}
	mimeType := imageMIMETypeForPath("image." + client.FileExtForFormat(strings.TrimSpace(snap.Result.Item.OutputFormat)))
	return data, mimeType, nil
}

func (a *App) copyCurrentResultImageToClipboard(gtx layout.Context, snap snapshot) error {
	data, mimeType, err := clipboardImageDataForResult(snap)
	if err != nil {
		return err
	}
	gtx.Execute(clipboard.WriteCmd{
		Type: mimeType,
		Data: io.NopCloser(bytes.NewReader(data)),
	})
	return nil
}

func (a *App) importClipboardImageData(data []byte, mimeType string) error {
	if len(data) == 0 {
		return fmt.Errorf("剪贴板里没有可导入图片")
	}
	format := imageFormatFromMIMEType(mimeType)
	name := fmt.Sprintf("clipboard-%s.%s", time.Now().Format("20060102-150405"), client.FileExtForFormat(format))
	virtualPath := registerVirtualImage(base64.StdEncoding.EncodeToString(data), name, format)
	if strings.TrimSpace(virtualPath) == "" {
		return fmt.Errorf("导入剪贴板图片失败: 数据为空")
	}
	if err := a.viewSourcePathOnCanvas(virtualPath); err != nil {
		return err
	}
	a.batchMode = false
	a.appendSourcePath(virtualPath)
	a.clearCompare()
	return nil
}
