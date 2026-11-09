package backend

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "image/jpeg" // register JPEG decoder for image.Decode
)

// ImageTransformResult is the shape returned by the rotation / flip / crop
// bindings. Path is an absolute path under UserConfigDir/imports.
type ImageTransformResult struct {
	Path         string `json:"path"`
	Acceleration string `json:"acceleration,omitempty"`
}

// RotateImage rotates the image at `path` by `degrees` (multiples of 90 only)
// clockwise and writes the result to imports/ as a new file. Original is left
// untouched.
func (s *Service) RotateImage(path string, degrees int) (ImageTransformResult, error) {
	deg := ((degrees % 360) + 360) % 360
	if deg != 0 && deg != 90 && deg != 180 && deg != 270 {
		return ImageTransformResult{}, errors.New("rotation must be a multiple of 90 degrees")
	}
	allowed, err := s.ensureManagedReadablePath(path, managedImageFile)
	if err != nil {
		return ImageTransformResult{}, err
	}
	src, err := loadImage(allowed)
	if err != nil {
		return ImageTransformResult{}, err
	}
	out, err := prepareTransformOutput(allowed, fmt.Sprintf("rot%d", deg))
	if err != nil {
		return ImageTransformResult{}, err
	}
	if result, err := transformWithGPU(allowed, out, gpuTransformRequest{Kind: gpuTransformRotate, Degrees: deg}); err == nil {
		return result, nil
	}
	_ = os.Remove(out.Path)
	rotated := rotate(src, deg)
	return saveTransform(rotated, out, "cpu")
}

// FlipImage flips horizontally (true) or vertically (false).
func (s *Service) FlipImage(path string, horizontal bool) (ImageTransformResult, error) {
	allowed, err := s.ensureManagedReadablePath(path, managedImageFile)
	if err != nil {
		return ImageTransformResult{}, err
	}
	src, err := loadImage(allowed)
	if err != nil {
		return ImageTransformResult{}, err
	}
	suffix := "fliph"
	if !horizontal {
		suffix = "flipv"
	}
	out, err := prepareTransformOutput(allowed, suffix)
	if err != nil {
		return ImageTransformResult{}, err
	}
	if result, err := transformWithGPU(allowed, out, gpuTransformRequest{Kind: gpuTransformFlip, Horizontal: horizontal}); err == nil {
		return result, nil
	}
	_ = os.Remove(out.Path)
	flipped := flip(src, horizontal)
	return saveTransform(flipped, out, "cpu")
}

// CropImage crops a rectangle (x,y,w,h in source pixels) and writes a new file.
func (s *Service) CropImage(path string, x, y, w, h int) (ImageTransformResult, error) {
	if w <= 0 || h <= 0 {
		return ImageTransformResult{}, errors.New("crop rect must have positive size")
	}
	allowed, err := s.ensureManagedReadablePath(path, managedImageFile)
	if err != nil {
		return ImageTransformResult{}, err
	}
	src, err := loadImage(allowed)
	if err != nil {
		return ImageTransformResult{}, err
	}
	b := src.Bounds()
	rect := image.Rect(b.Min.X+x, b.Min.Y+y, b.Min.X+x+w, b.Min.Y+y+h).Intersect(b)
	if rect.Empty() {
		return ImageTransformResult{}, errors.New("crop rect lies outside the image")
	}
	out, err := prepareTransformOutput(allowed, "crop")
	if err != nil {
		return ImageTransformResult{}, err
	}
	if result, err := transformWithGPU(allowed, out, gpuTransformRequest{
		Kind:  gpuTransformCrop,
		CropX: x,
		CropY: y,
		CropW: rect.Dx(),
		CropH: rect.Dy(),
	}); err == nil {
		return result, nil
	}
	_ = os.Remove(out.Path)
	dst := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	draw.Draw(dst, dst.Bounds(), src, rect.Min, draw.Src)
	return saveTransform(dst, out, "cpu")
}

// --- internal helpers ------------------------------------------------------

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return img, nil
}

// rotate rotates clockwise by deg (0/90/180/270).
func rotate(src image.Image, deg int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if deg == 0 {
		return src
	}
	var dst *image.RGBA
	if deg == 180 {
		dst = image.NewRGBA(image.Rect(0, 0, w, h))
	} else {
		dst = image.NewRGBA(image.Rect(0, 0, h, w)) // 90 / 270 swap
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.At(b.Min.X+x, b.Min.Y+y)
			switch deg {
			case 90:
				dst.Set(h-1-y, x, c)
			case 180:
				dst.Set(w-1-x, h-1-y, c)
			case 270:
				dst.Set(y, w-1-x, c)
			}
		}
	}
	return dst
}

func flip(src image.Image, horizontal bool) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := src.At(b.Min.X+x, b.Min.Y+y)
			if horizontal {
				dst.Set(w-1-x, y, c)
			} else {
				dst.Set(x, h-1-y, c)
			}
		}
	}
	return dst
}

type transformOutput struct {
	Path   string
	Format string
}

func prepareTransformOutput(originalPath, suffix string) (transformOutput, error) {
	dir, err := importsDir()
	if err != nil {
		return transformOutput{}, err
	}
	if err := os.MkdirAll(dir, secureDirMode); err != nil {
		return transformOutput{}, err
	}
	base := filepath.Base(originalPath)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	ext, format := transformEncodingForPath(originalPath)
	name := fmt.Sprintf("%s-%s-%s%s", time.Now().Format("20060102-150405"), sanitiseName(stem), suffix, ext)
	out, err := filepath.Abs(filepath.Join(dir, name))
	if err != nil {
		return transformOutput{}, err
	}
	return transformOutput{Path: out, Format: format}, nil
}

func transformEncodingForPath(originalPath string) (ext string, format string) {
	switch strings.ToLower(filepath.Ext(originalPath)) {
	case ".jpg", ".jpeg":
		return ".jpg", "jpeg"
	default:
		return ".png", "png"
	}
}

func saveTransform(img image.Image, out transformOutput, acceleration string) (ImageTransformResult, error) {
	f, err := os.OpenFile(out.Path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, secureFileMode)
	if err != nil {
		return ImageTransformResult{}, err
	}
	defer f.Close()
	switch out.Format {
	case "jpeg":
		if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 92}); err != nil {
			return ImageTransformResult{}, err
		}
	default:
		if err := png.Encode(f, img); err != nil {
			return ImageTransformResult{}, err
		}
	}
	return ImageTransformResult{Path: out.Path, Acceleration: acceleration}, nil
}
