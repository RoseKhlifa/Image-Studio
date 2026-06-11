package ui

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

const virtualTextPrefix = "memory://text/"

type virtualTextRecord struct {
	text         string
	lastAccessed time.Time
}

var (
	virtualTextMu    sync.Mutex
	virtualTextStore = map[string]virtualTextRecord{}
)

func registerVirtualText(text string, suggestedName string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	name := strings.TrimSpace(suggestedName)
	if name == "" {
		name = "raw-response.txt"
	}
	key := virtualTextPrefix + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + sanitizeVirtualTextName(name)
	virtualTextMu.Lock()
	virtualTextStore[key] = virtualTextRecord{
		text:         text,
		lastAccessed: time.Now(),
	}
	virtualTextMu.Unlock()
	return key
}

func readVirtualText(path string) (string, bool) {
	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, virtualTextPrefix) {
		return "", false
	}
	virtualTextMu.Lock()
	record, ok := virtualTextStore[path]
	if ok {
		record.lastAccessed = time.Now()
		virtualTextStore[path] = record
	}
	virtualTextMu.Unlock()
	if !ok {
		return "", false
	}
	return record.text, true
}

func sanitizeVirtualTextName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "raw-response.txt"
	}
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, ":", "-")
	return name
}
