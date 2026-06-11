package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"image-studio/gio-client/internal/kernel"
	sharedCompat "image-studio/shared/compat"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

func canSaveHistoryItem(item sharedCompat.HistoryItem) bool {
	return strings.TrimSpace(item.SavedPath) != "" || strings.TrimSpace(item.ImageB64) != ""
}

func suggestedSaveNameForHistoryItem(item sharedCompat.HistoryItem) string {
	mode := client.ModeGenerate
	if strings.TrimSpace(item.Mode) == string(client.ModeEdit) {
		mode = client.ModeEdit
	}
	timestamp := time.Now().Format("20060102-150405")
	if item.CreatedAt > 0 {
		timestamp = time.UnixMilli(item.CreatedAt).Format("20060102-150405")
	}
	prompt := strings.TrimSpace(item.Prompt)
	if prompt == "" {
		prompt = "image"
	}
	format := strings.TrimSpace(item.OutputFormat)
	if format == "" {
		format = client.OutputFormat
	}
	prefix := "generate"
	if mode == client.ModeEdit {
		prefix = "edit"
	}
	slug := client.Slugify(prompt, "image")
	ext := client.FileExtForFormat(format)
	return fmt.Sprintf("image-%s-%s-%s.%s", prefix, slug, timestamp, ext)
}

func defaultSavePromptTargetForHistoryItem(item sharedCompat.HistoryItem, outputDir string) string {
	if path := strings.TrimSpace(item.SavedPath); path != "" {
		if !isVirtualImagePath(path) {
			return path
		}
		outputDir = strings.TrimSpace(outputDir)
		if outputDir == "" {
			outputDir = kernel.DefaultOutputDir()
		}
		name := strings.TrimSpace(virtualImageDisplayName(path))
		if name == "" {
			name = suggestedSaveNameForHistoryItem(item)
		}
		return filepath.Join(outputDir, name)
	}
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		outputDir = kernel.DefaultOutputDir()
	}
	return filepath.Join(outputDir, suggestedSaveNameForHistoryItem(item))
}

func saveImageB64ToPath(imageB64 string, suggestedName string, dst string) (string, error) {
	imageB64 = strings.TrimSpace(imageB64)
	dst = strings.TrimSpace(strings.Trim(dst, `"'`))
	if imageB64 == "" {
		return "", fmt.Errorf("图片数据为空")
	}
	if dst == "" {
		return "", fmt.Errorf("目标路径为空")
	}
	suggestedName = strings.TrimSpace(suggestedName)
	if suggestedName == "" {
		suggestedName = "image.png"
	}
	if strings.HasSuffix(dst, string(os.PathSeparator)) || strings.HasSuffix(dst, "/") || strings.HasSuffix(dst, `\`) {
		dst = filepath.Join(dst, suggestedName)
	}
	if info, err := os.Stat(dst); err == nil && info.IsDir() {
		dst = filepath.Join(dst, suggestedName)
	}
	if filepath.Ext(dst) == "" {
		dst += filepath.Ext(suggestedName)
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absDst), 0o700); err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(imageB64)
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}
	if err := os.WriteFile(absDst, data, 0o600); err != nil {
		return "", err
	}
	return absDst, nil
}

func saveHistoryItemToDirectory(item sharedCompat.HistoryItem, directory string) (string, error) {
	directory = strings.TrimSpace(strings.Trim(directory, `"'`))
	if directory == "" {
		return "", fmt.Errorf("目标目录为空")
	}
	suggested := suggestedSaveNameForHistoryItem(item)
	savedPath := strings.TrimSpace(item.SavedPath)
	if savedPath != "" && !isVirtualImagePath(savedPath) {
		return copyImageFile(savedPath, filepath.Join(directory, suggested))
	}
	imageB64 := strings.TrimSpace(item.ImageB64)
	if imageB64 == "" && savedPath != "" {
		if b64, ok := readVirtualImageB64(savedPath); ok {
			imageB64 = strings.TrimSpace(b64)
		}
	}
	if imageB64 == "" {
		return "", fmt.Errorf("当前图片没有可保存内容")
	}
	return saveImageB64ToPath(imageB64, suggested, directory)
}

func saveHistoryItemsToDirectory(items []sharedCompat.HistoryItem, directory string) ([]string, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("请先勾选要另存的图片")
	}
	saved := make([]string, 0, len(items))
	for _, item := range items {
		path, err := saveHistoryItemToDirectory(item, directory)
		if err != nil {
			return nil, err
		}
		saved = append(saved, path)
	}
	return saved, nil
}
