package client

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"os"
	"strings"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

type openAIImagesMaskPair struct {
	sourcePNG []byte
	maskPNG   []byte
}

func validateResponsesMaskInput(mode Mode, sourceDataURLs []string, maskB64 string) error {
	if strings.TrimSpace(maskB64) == "" {
		return nil
	}
	if mode != ModeEdit {
		return errors.New("蒙版仅支持图生图模式")
	}
	if len(sourceDataURLs) == 0 || strings.TrimSpace(sourceDataURLs[0]) == "" {
		return errors.New("蒙版任务需要至少一张源图")
	}
	if _, _, err := decodeImageDataURLWithinLimit(sourceDataURLs[0], "首张源图"); err != nil {
		return err
	}
	maskRaw, err := decodeBase64WithinLimit(maskB64, "蒙版图片")
	if err != nil {
		return err
	}
	_, _, err = decodeSupportedImage(maskRaw, "蒙版图片")
	return err
}

func prepareOpenAIImagesMaskPair(sourcePath, maskB64 string) (openAIImagesMaskPair, error) {
	sourceRaw, err := readImageFileWithinLimit(sourcePath, "首张源图")
	if err != nil {
		return openAIImagesMaskPair{}, err
	}
	source, _, err := decodeSupportedImage(sourceRaw, "首张源图")
	if err != nil {
		return openAIImagesMaskPair{}, err
	}
	dims := source.Bounds().Size()
	if dims.X <= 0 || dims.Y <= 0 {
		return openAIImagesMaskPair{}, errors.New("首张源图尺寸无效")
	}

	maskRaw, err := decodeBase64WithinLimit(maskB64, "蒙版图片")
	if err != nil {
		return openAIImagesMaskPair{}, err
	}
	mask, _, err := decodeSupportedImage(maskRaw, "蒙版图片")
	if err != nil {
		return openAIImagesMaskPair{}, err
	}
	normalizedMask := normalizeMaskToSize(mask, dims)

	sourcePNG, err := encodePNGWithinLimit(source, "首张源图")
	if err != nil {
		return openAIImagesMaskPair{}, err
	}
	maskPNG, err := encodePNGWithinLimit(normalizedMask, "蒙版图片")
	if err != nil {
		return openAIImagesMaskPair{}, err
	}
	return openAIImagesMaskPair{sourcePNG: sourcePNG, maskPNG: maskPNG}, nil
}

func readImageFileWithinLimit(path, label string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("%s为空", label)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("读取%s失败: %w", label, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s不是图片文件", label)
	}
	if info.Size() <= 0 {
		return nil, fmt.Errorf("%s为空", label)
	}
	if info.Size() > MaxInputImageBytes {
		return nil, fmt.Errorf("%s过大(%dB > %dB 上限)", label, info.Size(), MaxInputImageBytes)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取%s失败: %w", label, err)
	}
	if len(raw) > MaxInputImageBytes {
		return nil, fmt.Errorf("%s过大(%dB > %dB 上限)", label, len(raw), MaxInputImageBytes)
	}
	return raw, nil
}

func decodeBase64WithinLimit(value, label string) ([]byte, error) {
	encoded := strings.TrimSpace(value)
	if encoded == "" {
		return nil, fmt.Errorf("%s为空", label)
	}
	if base64.StdEncoding.DecodedLen(len(encoded)) > MaxInputImageBytes+2 {
		return nil, fmt.Errorf("%s超过 50MB 上限", label)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%s base64 无效: %w", label, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("%s为空", label)
	}
	if len(raw) > MaxInputImageBytes {
		return nil, fmt.Errorf("%s过大(%dB > %dB 上限)", label, len(raw), MaxInputImageBytes)
	}
	return raw, nil
}

func decodeImageDataURLWithinLimit(value, label string) ([]byte, image.Image, error) {
	value = strings.TrimSpace(value)
	comma := strings.IndexByte(value, ',')
	if !strings.HasPrefix(value, "data:image/") || comma < 0 || !strings.Contains(value[:comma], ";base64") {
		return nil, nil, fmt.Errorf("%s不是有效的 base64 图片 data URL", label)
	}
	raw, err := decodeBase64WithinLimit(value[comma+1:], label)
	if err != nil {
		return nil, nil, err
	}
	img, _, err := decodeSupportedImage(raw, label)
	return raw, img, err
}

func decodeSupportedImage(raw []byte, label string) (image.Image, string, error) {
	mimeType := detectImageMimeTypeFromBytes(raw)
	if mimeType == "" {
		return nil, "", fmt.Errorf("%s不是支持的 PNG/JPEG/WebP 图片", label)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("解析%s失败: %w", label, err)
	}
	if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
		return nil, "", fmt.Errorf("%s尺寸无效", label)
	}
	return img, mimeType, nil
}

func normalizeMaskToSize(mask image.Image, dims image.Point) *image.NRGBA {
	bounds := image.Rect(0, 0, dims.X, dims.Y)
	scaled := image.NewNRGBA(bounds)
	if mask.Bounds().Size() == dims {
		draw.Draw(scaled, bounds, mask, mask.Bounds().Min, draw.Src)
	} else {
		xdraw.ApproxBiLinear.Scale(scaled, bounds, mask, mask.Bounds(), draw.Src, nil)
	}

	hasTransparency := false
	for y := bounds.Min.Y; y < bounds.Max.Y && !hasTransparency; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if scaled.NRGBAAt(x, y).A < 0xff {
				hasTransparency = true
				break
			}
		}
	}

	normalized := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := scaled.NRGBAAt(x, y)
			alpha := pixel.A
			if !hasTransparency {
				luma := (uint32(pixel.R)*299 + uint32(pixel.G)*587 + uint32(pixel.B)*114 + 500) / 1000
				alpha = uint8(0xff - luma)
			}
			normalized.SetNRGBA(x, y, color.NRGBA{A: alpha})
		}
	}
	return normalized
}

func encodePNGWithinLimit(img image.Image, label string) ([]byte, error) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, img); err != nil {
		return nil, fmt.Errorf("编码%s PNG 失败: %w", label, err)
	}
	if encoded.Len() > MaxInputImageBytes {
		return nil, fmt.Errorf("%s转换为 PNG 后过大(%dB > %dB 上限)", label, encoded.Len(), MaxInputImageBytes)
	}
	return encoded.Bytes(), nil
}
