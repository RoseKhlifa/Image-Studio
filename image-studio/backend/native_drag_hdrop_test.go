package backend

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

func TestBuildHDropPayloadUsesWideDoubleNullTerminatedPath(t *testing.T) {
	path := `C:\Users\tester\Pictures\结果.png`
	payload, err := buildHDropPayload(path)
	if err != nil {
		t.Fatalf("buildHDropPayload: %v", err)
	}
	if got := binary.LittleEndian.Uint32(payload[0:4]); got != dropFilesHeaderSize {
		t.Fatalf("DROPFILES.pFiles=%d want %d", got, dropFilesHeaderSize)
	}
	if got := binary.LittleEndian.Uint32(payload[16:20]); got != 1 {
		t.Fatalf("DROPFILES.fWide=%d want 1", got)
	}

	encoded := payload[dropFilesHeaderSize:]
	units := make([]uint16, len(encoded)/2)
	for index := range units {
		units[index] = binary.LittleEndian.Uint16(encoded[index*2:])
	}
	if len(units) < 2 || units[len(units)-2] != 0 || units[len(units)-1] != 0 {
		t.Fatalf("file list is not double-NUL terminated: %v", units)
	}
	if got := string(utf16.Decode(units[:len(units)-2])); got != path {
		t.Fatalf("decoded path=%q want %q", got, path)
	}
}

func TestBuildHDropPayloadRejectsInvalidPaths(t *testing.T) {
	for _, path := range []string{"", "C:\\bad\x00path.png"} {
		if _, err := buildHDropPayload(path); err == nil {
			t.Fatalf("buildHDropPayload(%q) unexpectedly succeeded", path)
		}
	}
}
