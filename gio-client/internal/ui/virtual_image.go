package ui

import (
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

const virtualImagePrefix = "memory://image/"

type virtualImageRecord struct {
	name         string
	mimeType     string
	imageB64     string
	lastAccessed time.Time
}

var (
	virtualImageMu    sync.Mutex
	virtualImageStore = map[string]virtualImageRecord{}
)

func registerVirtualImage(imageB64 string, suggestedName string, outputFormat string) string {
	imageB64 = strings.TrimSpace(imageB64)
	if imageB64 == "" {
		return ""
	}
	suggestedName = strings.TrimSpace(suggestedName)
	if suggestedName == "" {
		suggestedName = "image." + client.FileExtForFormat(strings.TrimSpace(outputFormat))
	}
	ext := filepath.Ext(suggestedName)
	if ext == "" {
		suggestedName += "." + client.FileExtForFormat(strings.TrimSpace(outputFormat))
		ext = filepath.Ext(suggestedName)
	}
	mimeType := client.SupportedImageMime[strings.ToLower(ext)]
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/png"
	}
	key := virtualImagePrefix + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + sanitizeVirtualTextName(suggestedName)
	virtualImageMu.Lock()
	virtualImageStore[key] = virtualImageRecord{
		name:         suggestedName,
		mimeType:     mimeType,
		imageB64:     imageB64,
		lastAccessed: time.Now(),
	}
	virtualImageMu.Unlock()
	return key
}

func isVirtualImagePath(path string) bool {
	path = strings.TrimSpace(path)
	return strings.HasPrefix(path, virtualImagePrefix)
}

func virtualImageDataURL(path string) (string, bool) {
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
	if !ok {
		return "", false
	}
	return "data:" + record.mimeType + ";base64," + record.imageB64, true
}

func readVirtualImageB64(path string) (string, bool) {
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
	if !ok {
		return "", false
	}
	return record.imageB64, true
}

func virtualImageDisplayName(path string) string {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, virtualImagePrefix) {
		return filepath.Base(path)
	}
	virtualImageMu.Lock()
	record, ok := virtualImageStore[path]
	if ok {
		record.lastAccessed = time.Now()
		virtualImageStore[path] = record
	}
	virtualImageMu.Unlock()
	if ok && strings.TrimSpace(record.name) != "" {
		return record.name
	}
	return filepath.Base(path)
}

func sourcePathDisplayName(path string) string {
	return virtualImageDisplayName(path)
}
