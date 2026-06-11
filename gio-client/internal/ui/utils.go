package ui

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	sharedCompat "image-studio/shared/compat"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	_ "github.com/gen2brain/avif"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const historyThumbFallbackMaxDimension = 256
const fastThumbScaleMaxDimension = 64

func decodeImageB64(imageB64 string) (image.Image, error) {
	data, err := base64.StdEncoding.DecodeString(imageB64)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image bytes: %w", err)
	}
	return img, nil
}

func decodeImageFile(path string) (image.Image, error) {
	if imageB64, ok := readVirtualImageB64(path); ok {
		return decodeImageB64(imageB64)
	}
	file, err := os.Open(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode image file: %w", err)
	}
	return img, nil
}

func downscaleToMaxDimension(img image.Image, maxDimension int) image.Image {
	if img == nil {
		return nil
	}
	if maxDimension <= 0 {
		return img
	}
	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= maxDimension && srcH <= maxDimension {
		return img
	}
	scale := float64(maxDimension) / float64(max(srcW, srcH))
	dstW := max(1, int(float64(srcW)*scale+0.5))
	dstH := max(1, int(float64(srcH)*scale+0.5))
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	if maxDimension <= fastThumbScaleMaxDimension {
		xdraw.NearestNeighbor.Scale(dst, dst.Bounds(), img, bounds, draw.Src, nil)
	} else {
		xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Src, nil)
	}
	return dst
}

func downscaleForHistoryThumb(img image.Image) image.Image {
	return downscaleToMaxDimension(img, historyThumbFallbackMaxDimension)
}

func appendBounded(logs []string, line string) []string {
	const maxLogs = 240
	logs = append(logs, time.Now().Format("15:04:05")+"  "+line)
	if len(logs) > maxLogs {
		copy(logs, logs[len(logs)-maxLogs:])
		logs = logs[:maxLogs]
	}
	return logs
}

func shortPrompt(prompt string) string {
	const limit = 40
	var b strings.Builder
	runeCount := 0
	pendingSpace := false
	sawContent := false
	truncated := false

	for _, r := range prompt {
		if unicode.IsSpace(r) {
			if sawContent {
				pendingSpace = true
			}
			continue
		}
		if pendingSpace {
			if runeCount >= limit {
				truncated = true
				break
			}
			b.WriteByte(' ')
			runeCount++
			pendingSpace = false
		}
		if runeCount >= limit {
			truncated = true
			break
		}
		b.WriteRune(r)
		runeCount++
		sawContent = true
	}

	if truncated {
		return b.String() + "..."
	}
	return b.String()
}

func compactNonEmpty(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func localDayStart(now time.Time) time.Time {
	year, month, day := now.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, now.Location())
}

func formatHistoryDay(createdAt int64) string {
	if createdAt <= 0 {
		return "更早"
	}
	return time.UnixMilli(createdAt).Format("2006-01-02")
}

type historyDateFilterKind uint8

const (
	historyDateFilterAll historyDateFilterKind = iota
	historyDateFilterToday
	historyDateFilterWeek
)

func normalizeHistorySearchQuery(query string) string {
	return strings.TrimSpace(strings.ToLower(query))
}

func prepareHistoryDateFilter(filter string, now time.Time) (historyDateFilterKind, int64) {
	switch strings.TrimSpace(filter) {
	case "today":
		return historyDateFilterToday, localDayStart(now).UnixMilli()
	case "week":
		return historyDateFilterWeek, now.AddDate(0, 0, -7).UnixMilli()
	default:
		return historyDateFilterAll, 0
	}
}

func matchHistoryDatePrepared(createdAt int64, kind historyDateFilterKind, cutoff int64) bool {
	switch kind {
	case historyDateFilterToday, historyDateFilterWeek:
		return createdAt >= cutoff
	default:
		return true
	}
}

func matchHistoryDate(createdAt int64, filter string, now time.Time) bool {
	kind, cutoff := prepareHistoryDateFilter(filter, now)
	return matchHistoryDatePrepared(createdAt, kind, cutoff)
}

func matchHistoryQueryNormalized(item sharedCompat.HistoryItem, query string) bool {
	if query == "" {
		return true
	}
	if item.Prompt != "" && strings.Contains(strings.ToLower(item.Prompt), query) {
		return true
	}
	if item.RevisedPrompt != "" && strings.Contains(strings.ToLower(item.RevisedPrompt), query) {
		return true
	}
	if item.SavedPath != "" && strings.Contains(strings.ToLower(item.SavedPath), query) {
		return true
	}
	if item.Size != "" && strings.Contains(strings.ToLower(item.Size), query) {
		return true
	}
	if item.Quality != "" && strings.Contains(strings.ToLower(item.Quality), query) {
		return true
	}
	return false
}

func matchHistoryQuery(item sharedCompat.HistoryItem, query string) bool {
	return matchHistoryQueryNormalized(item, normalizeHistorySearchQuery(query))
}

func copyImageFile(src, dst string) (string, error) {
	src = strings.TrimSpace(src)
	dst = strings.TrimSpace(strings.Trim(dst, `"'`))
	if src == "" {
		return "", errors.New("源图片路径为空")
	}
	if dst == "" {
		return "", errors.New("目标路径为空")
	}
	if strings.HasSuffix(dst, string(os.PathSeparator)) || strings.HasSuffix(dst, "/") || strings.HasSuffix(dst, `\`) {
		dst = filepath.Join(dst, filepath.Base(src))
	}
	if info, err := os.Stat(dst); err == nil && info.IsDir() {
		dst = filepath.Join(dst, filepath.Base(src))
	}
	if filepath.Ext(dst) == "" {
		dst += filepath.Ext(src)
	}
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return "", err
	}
	absDst, err := filepath.Abs(dst)
	if err != nil {
		return "", err
	}
	if filepath.Clean(absSrc) == filepath.Clean(absDst) {
		return absDst, nil
	}
	if err := os.MkdirAll(filepath.Dir(absDst), 0o700); err != nil {
		return "", err
	}
	in, err := os.Open(absSrc)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.OpenFile(absDst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		_ = out.Close()
		if !ok {
			_ = os.Remove(absDst)
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return "", err
	}
	if err := out.Close(); err != nil {
		return "", err
	}
	ok = true
	return absDst, nil
}
