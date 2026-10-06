package kernel

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"

	"github.com/yuanhua/image-gptcodex/pkg/client"
	xdraw "golang.org/x/image/draw"
)

type preparedEditMask struct {
	source *image.NRGBA
	mask   *image.NRGBA
}

func prepareEditMask(cfg *Config) (*preparedEditMask, error) {
	if cfg == nil || cfg.Mode != client.ModeEdit || strings.TrimSpace(cfg.MaskB64) == "" {
		return nil, nil
	}
	source, err := firstEditSourceImage(*cfg)
	if err != nil {
		return nil, fmt.Errorf("读取蒙版源图失败: %w", err)
	}
	sourceNRGBA := copyImageToNRGBA(source)
	if sourceNRGBA.Bounds().Empty() {
		return nil, errors.New("蒙版源图尺寸无效")
	}
	normalizedB64, normalizedMask, err := normalizeEditMaskB64(cfg.MaskB64, sourceNRGBA.Bounds().Size())
	if err != nil {
		return nil, err
	}
	cfg.MaskB64 = normalizedB64
	return &preparedEditMask{source: sourceNRGBA, mask: normalizedMask}, nil
}

func firstEditSourceImage(cfg Config) (image.Image, error) {
	if paths := normalizeSourcePaths(cfg.SourcePaths); len(paths) > 0 {
		file, err := os.Open(paths[0])
		if err != nil {
			return nil, err
		}
		defer file.Close()
		img, _, err := image.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("解析首张源图失败: %w", err)
		}
		return img, nil
	}
	if dataURLs := normalizeSourceImageDataURLs(cfg.SourceImageDataURLs); len(dataURLs) > 0 {
		return decodeImageDataURL(dataURLs[0])
	}
	return nil, errors.New("图生图模式需要至少一张源图")
}

func decodeImageDataURL(value string) (image.Image, error) {
	value = strings.TrimSpace(value)
	comma := strings.IndexByte(value, ',')
	if !strings.HasPrefix(value, "data:") || comma < 0 || !strings.Contains(value[:comma], ";base64") {
		return nil, errors.New("首张源图不是有效的 base64 data URL")
	}
	raw, err := base64.StdEncoding.DecodeString(value[comma+1:])
	if err != nil {
		return nil, fmt.Errorf("首张源图 base64 无效: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("解析首张源图失败: %w", err)
	}
	return img, nil
}

// normalizeEditMaskB64 converts legacy opaque black/white masks and alpha
// masks to the OpenAI convention: transparent pixels are editable and opaque
// pixels protect the source. The returned PNG matches the source dimensions.
func normalizeEditMaskB64(maskB64 string, dims image.Point) (string, *image.NRGBA, error) {
	if dims.X <= 0 || dims.Y <= 0 {
		return "", nil, errors.New("蒙版源图尺寸无效")
	}
	mask, err := decodeBase64Image(maskB64, "蒙版")
	if err != nil {
		return "", nil, err
	}

	bounds := image.Rect(0, 0, dims.X, dims.Y)
	scaled := image.NewNRGBA(bounds)
	if mask.Bounds().Size() == dims {
		draw.Draw(scaled, bounds, mask, mask.Bounds().Min, draw.Src)
	} else {
		xdraw.ApproxBiLinear.Scale(scaled, bounds, mask, mask.Bounds(), draw.Src, nil)
	}

	hasTransparency := maskHasTransparency(scaled)
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

	encoded, err := encodePNGBase64(normalized)
	if err != nil {
		return "", nil, fmt.Errorf("编码蒙版 PNG 失败: %w", err)
	}
	return encoded, normalized, nil
}

func compositeMaskedEditB64(prepared *preparedEditMask, generatedB64 string) (string, error) {
	if prepared == nil || prepared.source == nil || prepared.mask == nil {
		return "", errors.New("蒙版合成状态无效")
	}
	generated, err := decodeBase64Image(generatedB64, "生成结果")
	if err != nil {
		return "", err
	}
	bounds := prepared.source.Bounds()
	generatedNRGBA := image.NewNRGBA(bounds)
	xdraw.CatmullRom.Scale(generatedNRGBA, bounds, generated, generated.Bounds(), draw.Src, nil)

	result := image.NewNRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			keep := uint32(prepared.mask.NRGBAAt(x, y).A)
			edit := uint32(0xff) - keep
			sourcePixel := prepared.source.NRGBAAt(x, y)
			generatedPixel := generatedNRGBA.NRGBAAt(x, y)
			result.SetNRGBA(x, y, color.NRGBA{
				R: blendMaskChannel(sourcePixel.R, generatedPixel.R, keep, edit),
				G: blendMaskChannel(sourcePixel.G, generatedPixel.G, keep, edit),
				B: blendMaskChannel(sourcePixel.B, generatedPixel.B, keep, edit),
				A: blendMaskChannel(sourcePixel.A, generatedPixel.A, keep, edit),
			})
		}
	}
	encoded, err := encodePNGBase64(result)
	if err != nil {
		return "", fmt.Errorf("编码蒙版合成结果失败: %w", err)
	}
	return encoded, nil
}

func decodeBase64Image(value, label string) (image.Image, error) {
	value = strings.TrimSpace(value)
	if comma := strings.IndexByte(value, ','); strings.HasPrefix(value, "data:") && comma >= 0 {
		value = value[comma+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%s base64 无效: %w", label, err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("%s图片为空", label)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("解析%s失败: %w", label, err)
	}
	return img, nil
}

func encodePNGBase64(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func copyImageToNRGBA(src image.Image) *image.NRGBA {
	if src == nil || src.Bounds().Empty() {
		return image.NewNRGBA(image.Rectangle{})
	}
	bounds := image.Rect(0, 0, src.Bounds().Dx(), src.Bounds().Dy())
	dst := image.NewNRGBA(bounds)
	draw.Draw(dst, bounds, src, src.Bounds().Min, draw.Src)
	return dst
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

func blendMaskChannel(source, generated uint8, keep, edit uint32) uint8 {
	return uint8((uint32(source)*keep + uint32(generated)*edit + 127) / 255)
}
