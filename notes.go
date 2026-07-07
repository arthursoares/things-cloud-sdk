package thingscloud

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

// ApplyPatches applies a series of text patches to an original string
func ApplyPatches(original string, patches []NotePatch) string {
	runes := []rune(original)
	for _, p := range patches {
		if p.Position < 0 {
			p.Position = 0
		}
		if p.Position > len(runes) {
			p.Position = len(runes)
		}
		end := p.Position + p.Length
		if end > len(runes) {
			end = len(runes)
		}
		if end < p.Position {
			// Negative length in a corrupt/hostile patch: treat as a
			// pure insertion instead of slicing out of bounds.
			end = p.Position
		}
		actualLength := end - p.Position
		replacementRunes := []rune(p.Replacement)
		newCap := len(runes) - actualLength + len(replacementRunes)
		if newCap < 0 {
			newCap = len(replacementRunes)
		}
		result := make([]rune, 0, newCap)
		result = append(result, runes[:p.Position]...)
		result = append(result, replacementRunes...)
		result = append(result, runes[end:]...)
		runes = result
	}
	return string(runes)
}
