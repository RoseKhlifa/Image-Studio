package backend

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
)

const dropFilesHeaderSize = 20

func buildHDropPayload(path string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("drag file path is empty")
	}
	if strings.ContainsRune(path, '\x00') {
		return nil, fmt.Errorf("drag file path contains NUL")
	}

	pathUTF16 := utf16.Encode([]rune(path))
	payload := make([]byte, dropFilesHeaderSize+(len(pathUTF16)+2)*2)
	binary.LittleEndian.PutUint32(payload[0:4], dropFilesHeaderSize)
	binary.LittleEndian.PutUint32(payload[16:20], 1) // DROPFILES.fWide
	for index, value := range pathUTF16 {
		binary.LittleEndian.PutUint16(payload[dropFilesHeaderSize+index*2:], value)
	}
	return payload, nil
}
