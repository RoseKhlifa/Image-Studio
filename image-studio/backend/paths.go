package backend

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

const (
	secureDirMode  = 0o700
	secureFileMode = 0o600

	appCompanyName     = "YuanHua"
	appProductName     = "Image Studio"
	appConfigDirName   = "image-studio"
	appDocumentDirName = "Image Studio"
)

// imagesSubdir / logSubdir 把根目录拆为「生成的 PNG」和「原始响应/排错日志」两个子文件夹。
// 用户在 SettingsPanel 里可以「打开输出目录」=> 落到根,所以两类内容在同一个文件夹下并列。
func imagesSubdir(root string) string { return filepath.Join(root, "images") }
func logSubdir(root string) string    { return filepath.Join(root, "log") }

// writeBase64PNG decodes a base64 image and writes it atomically; returns the
// absolute path of the written file.
func writeBase64PNG(b64, path string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, secureFileMode); err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(path)
	return abs, nil
}

// buildImageName composes the canonical filename for a generated image, e.g.
// `image-generate-cyberpunk-cat-20260518-210500.png`.
// outputFormat 来自 GenerateOptions.OutputFormat (png/jpeg/webp/gif/svg),空时回退到 client.OutputFormat。
// 扩展名走 client.FileExtForFormat 标准化(jpeg→jpg)。
func buildImageName(mode client.Mode, prompt, timestamp, outputFormat string) string {
	prefix := "generate"
	if mode == client.ModeEdit {
		prefix = "edit"
	}
	slug := client.Slugify(prompt, "image")
	ext := client.FileExtForFormat(outputFormat)
	return fmt.Sprintf("image-%s-%s-%s.%s", prefix, slug, timestamp, ext)
}
