package thingscloud

import (
	"fmt"
	"unicode/utf8"
)

// NoteTypeFullText indicates a note with complete text
const NoteTypeFullText = 1

// NoteTypeDelta indicates a note with incremental patches
const NoteTypeDelta = 2

// NotePatch describes a single text replacement operation
type NotePatch struct {
	Replacement string `json:"r"`
	Position    int    `json:"p"`
	Length      int    `json:"l"`
	Checksum    int64  `json:"ch"`
}

// Note describes a structured note as used by the Things API
type Note struct {
	TypeTag  string      `json:"_t"`
	Type     int         `json:"t"`
	Checksum int64       `json:"ch,omitempty"`
	Value    string      `json:"v,omitempty"`
	Patches  []NotePatch `json:"ps,omitempty"`
}

// ApplyPatches applies a series of text patches using UTF-8 byte offsets.
func ApplyPatches(original string, patches []NotePatch) string {
	result, _ := applyPatches(original, patches, false)
	return result
}

// ApplyPatchesChecked applies byte-offset patches and rejects any patch that
// leaves the note in an invalid UTF-8 state. ApplyPatches remains available for
// callers that rely on its historical unchecked behavior.
func ApplyPatchesChecked(original string, patches []NotePatch) (string, error) {
	return applyPatches(original, patches, true)
}

func applyPatches(original string, patches []NotePatch, validateUTF8 bool) (string, error) {
	if validateUTF8 && !utf8.ValidString(original) {
		return "", fmt.Errorf("original note is not valid UTF-8")
	}

	text := []byte(original)
	for i, p := range patches {
		position := p.Position
		if position < 0 {
			position = 0
		}
		if position > len(text) {
			position = len(text)
		}

		length := p.Length
		if length < 0 {
			length = 0
		}
		remaining := len(text) - position
		if length > remaining {
			length = remaining
		}
		end := position + length

		result := append([]byte(nil), text[:position]...)
		result = append(result, p.Replacement...)
		result = append(result, text[end:]...)
		if validateUTF8 && !utf8.Valid(result) {
			return "", fmt.Errorf("note patch %d produced invalid UTF-8", i)
		}
		text = result
	}
	return string(text), nil
}
