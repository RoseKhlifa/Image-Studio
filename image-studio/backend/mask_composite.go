package backend

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// normalizeEditMaskB64 converts imported black/white masks and painted masks
// to OpenAI's alpha convention: transparent pixels are editable and opaque
// pixels protect the source image. The returned PNG always matches the first
// edit source dimensions.
func normalizeEditMaskB64(maskB64, sourcePath string) (string, error) {
	maskBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(maskB64))
	if err != nil {
		return "", fmt.Errorf("蒙版图片 base64 无效: %w", err)
	}
	if len(maskBytes) == 0 {
		return "", errors.New("蒙版图片为空")
	}
	mask, _, err := image.Decode(bytes.NewReader(maskBytes))
	if err != nil {
		return "", fmt.Errorf("解析蒙版图片失败: %w", err)
	}
	source, err := loadImage(sourcePath)
	if err != nil {
		return "", fmt.Errorf("读取蒙版源图失败: %w", err)
	}

	dims := source.Bounds().Size()
	if dims.X <= 0 || dims.Y <= 0 {
		return "", errors.New("蒙版源图尺寸无效")
	}
	scaled := image.NewNRGBA(image.Rect(0, 0, dims.X, dims.Y))
	copyOrScaleMask(scaled, mask)

	hasTransparency := maskHasTransparency(scaled)

	normalized := image.NewNRGBA(scaled.Bounds())
	for y := 0; y < dims.Y; y++ {
		for x := 0; x < dims.X; x++ {
			pixel := scaled.NRGBAAt(x, y)
			alpha := pixel.A
			if !hasTransparency {
				// Opaque black/white masks are accepted as a convenience. Match the
				// canvas UI: white is the painted/editable selection, black is kept.
				luma := (uint32(pixel.R)*299 + uint32(pixel.G)*587 + uint32(pixel.B)*114 + 500) / 1000
				alpha = uint8(0xff - luma)
			}
			normalized.SetNRGBA(x, y, color.NRGBA{A: alpha})
		}
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, normalized); err != nil {
		return "", fmt.Errorf("编码蒙版 PNG 失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encoded.Bytes()), nil
}

func maskHasTransparency(mask *image.NRGBA) bool {
	for y := mask.Bounds().Min.Y; y < mask.Bounds().Max.Y; y++ {
		for x := mask.Bounds().Min.X; x < mask.Bounds().Max.X; x++ {
			if mask.NRGBAAt(x, y).A < 0xff {
				return true
			}
		}
	}
	return false
}

// compositeMaskedEditB64 enforces the mask locally after generation. Pixels
// protected by the mask come from the original source, so model drift outside
// the editable selection cannot leak into the saved result.
func compositeMaskedEditB64(sourcePath, normalizedMaskB64, generatedB64 string) (string, error) {
	source, err := loadImage(sourcePath)
	if err != nil {
		return "", fmt.Errorf("读取蒙版源图失败: %w", err)
	}
	mask, err := decodeBase64Image(normalizedMaskB64, "蒙版")
	if err != nil {
		return "", err
	}
	generated, err := decodeBase64Image(generatedB64, "生成结果")
	if err != nil {
		return "", err
	}

	bounds := image.Rect(0, 0, source.Bounds().Dx(), source.Bounds().Dy())
	if bounds.Empty() {
		return "", errors.New("蒙版源图尺寸无效")
	}
	sourceRGBA := image.NewNRGBA(bounds)
	draw.Draw(sourceRGBA, bounds, source, source.Bounds().Min, draw.Src)
	maskRGBA := image.NewNRGBA(bounds)
	copyOrScaleMask(maskRGBA, mask)
	generatedRGBA := image.NewNRGBA(bounds)
	xdraw.CatmullRom.Scale(generatedRGBA, bounds, generated, generated.Bounds(), draw.Src, nil)

	result := image.NewNRGBA(bounds)
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			keep := uint32(maskRGBA.NRGBAAt(x, y).A)
			edit := uint32(0xff) - keep
			srcPixel := sourceRGBA.NRGBAAt(x, y)
			generatedPixel := generatedRGBA.NRGBAAt(x, y)
			result.SetNRGBA(x, y, color.NRGBA{
				R: blendMaskChannel(srcPixel.R, generatedPixel.R, keep, edit),
				G: blendMaskChannel(srcPixel.G, generatedPixel.G, keep, edit),
				B: blendMaskChannel(srcPixel.B, generatedPixel.B, keep, edit),
				A: blendMaskChannel(srcPixel.A, generatedPixel.A, keep, edit),
			})
		}
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, result); err != nil {
		return "", fmt.Errorf("编码蒙版合成结果失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(encoded.Bytes()), nil
}

func copyOrScaleMask(dst *image.NRGBA, src image.Image) {
	if dst.Bounds().Dx() == src.Bounds().Dx() && dst.Bounds().Dy() == src.Bounds().Dy() {
		draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
		return
	}
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
}

func decodeBase64Image(value, label string) (image.Image, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("%s base64 无效: %w", label, err)
	}
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("解析%s失败: %w", label, err)
	}
	return decoded, nil
}

func blendMaskChannel(source, generated uint8, keep, edit uint32) uint8 {
	return uint8((uint32(source)*keep + uint32(generated)*edit + 127) / 255)
}
