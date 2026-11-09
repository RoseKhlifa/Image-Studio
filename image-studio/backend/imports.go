package backend

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

// ImportImageFromB64 persists a base64-encoded image (PNG/JPEG/WebP/GIF/SVG) into the
// imports directory and returns its absolute path. The legacy imageB64 field is
// intentionally left empty on Wails so imported files do not stay duplicated in
// frontend state.
//
// Used by:
//   - drag-and-drop / paste flows (App.tsx) — the file becomes a real edit source
//   - rotate / flip / crop derivatives (imageops.go) — they share the same destination
//
// suggestedName is sanitised; if empty, a timestamped name is generated.
func (s *Service) ImportImageFromB64(imageB64, suggestedName string) (ImportedImage, error) {
	if strings.TrimSpace(imageB64) == "" {
		return ImportedImage{}, errors.New("imageB64 is empty")
	}
	data, err := base64.StdEncoding.DecodeString(imageB64)
	if err != nil {
		return ImportedImage{}, fmt.Errorf("decode base64: %w", err)
	}
	if len(data) > client.MaxInputImageBytes {
		return ImportedImage{}, fmt.Errorf("图片超过 50MB,请换一张更小的图片")
	}

	dir, err := importsDir()
	if err != nil {
		return ImportedImage{}, err
	}
	if err := os.MkdirAll(dir, secureDirMode); err != nil {
		return ImportedImage{}, err
	}

	ext := guessExt(suggestedName, data)
	name := time.Now().Format("20060102-150405") + "-" + sanitiseName(suggestedName) + ext
	full := filepath.Join(dir, name)
	if err := os.WriteFile(full, data, secureFileMode); err != nil {
		return ImportedImage{}, fmt.Errorf("write import file: %w", err)
	}
	return ImportedImage{Path: full}, nil
}

// sanitiseName produces a filename-safe stem from a user-supplied filename.
// Keeps ASCII alnum, dash, underscore, and CJK; collapses everything else to "-".
// Truncates by rune count (not byte count) so CJK isn't split mid-character.
func sanitiseName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "import"
	}
	if dot := strings.LastIndex(s, "."); dot > 0 {
		s = s[:dot] // drop extension; guessExt will pick a real one
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z'), (r >= 'A' && r <= 'Z'), (r >= '0' && r <= '9'), r == '-', r == '_':
			b.WriteRune(r)
		case r >= 0x4e00 && r <= 0x9fff: // CJK Unified Ideographs
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := b.String()
	if len(out) > 40 {
		runes := []rune(out)
		if len(runes) > 40 {
			out = string(runes[:40])
		}
	}
	if out == "" {
		return "import"
	}
	return strings.Trim(out, "-")
}

// guessExt sniffs magic bytes first, then falls back to the suggested
// filename's extension if it's one of the supported image types.
func guessExt(name string, data []byte) string {
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return ".png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return ".jpg"
	}
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return ".webp"
	}
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return ".gif"
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 {
		lower := strings.ToLower(string(trimmed))
		if strings.Contains(lower, "<svg") && strings.Contains(lower, ">") {
			return ".svg"
		}
	}
	if dot := strings.LastIndex(name, "."); dot > 0 {
		ext := strings.ToLower(name[dot:])
		if _, ok := client.SupportedImageMime[ext]; ok {
			return ext
		}
	}
	return ".png"
}
