package thingscloud

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNote_FullText(t *testing.T) {
	raw := `{"_t":"tx","t":1,"ch":0,"v":"Hello world"}`
	var n Note
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		t.Fatal(err)
	}
	if n.Type != NoteTypeFullText {
		t.Errorf("expected type %d, got %d", NoteTypeFullText, n.Type)
	}
	if n.Value != "Hello world" {
		t.Errorf("expected 'Hello world', got '%s'", n.Value)
	}
}

func TestNote_Delta(t *testing.T) {
	raw := `{"_t":"tx","t":2,"ps":[{"r":"inserted text","p":0,"l":0,"ch":12345}]}`
	var n Note
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		t.Fatal(err)
	}
	if n.Type != NoteTypeDelta {
		t.Errorf("expected type %d, got %d", NoteTypeDelta, n.Type)
	}
	if len(n.Patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(n.Patches))
	}
	if n.Patches[0].Replacement != "inserted text" {
		t.Errorf("unexpected replacement: %s", n.Patches[0].Replacement)
	}
}

func TestNote_ApplyPatch(t *testing.T) {
	original := "Hello world"
	patch := NotePatch{Position: 5, Length: 6, Replacement: " Go"}
	result := ApplyPatches(original, []NotePatch{patch})
	if result != "Hello Go" {
		t.Errorf("expected 'Hello Go', got '%s'", result)
	}
}

func TestNote_ApplyPatch_UsesUTF8ByteOffsets(t *testing.T) {
	original := "Native Task7 note α 🚀\nSecond line."
	patch := NotePatch{
		Position:    26,
		Length:      5,
		Replacement: "Update",
		Checksum:    3672733299,
	}

	result := ApplyPatches(original, []NotePatch{patch})
	if result != "Native Task7 note α 🚀\nUpdated line." {
		t.Errorf("expected native Task7 note replay, got %q", result)
	}
}

func TestNote_ApplyPatch_PositionBeyondLength(t *testing.T) {
	// Patch position is beyond the string — should not panic
	result := ApplyPatches("", []NotePatch{{Position: 10, Length: 0, Replacement: "hello"}})
	if result != "hello" {
		t.Errorf("expected 'hello', got '%s'", result)
	}
}

func TestNote_ApplyPatch_LengthBeyondEnd(t *testing.T) {
	result := ApplyPatches("AB", []NotePatch{{Position: 1, Length: 100, Replacement: "X"}})
	if result != "AX" {
		t.Errorf("expected 'AX', got '%s'", result)
	}
}

func TestNote_ApplyMultiplePatches(t *testing.T) {
	original := "ABCDEF"
	patches := []NotePatch{
		{Position: 0, Length: 1, Replacement: "X"},
	}
	result := ApplyPatches(original, patches)
	if result != "XBCDEF" {
		t.Errorf("expected 'XBCDEF', got '%s'", result)
	}
}

func TestApplyPatchesCheckedRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()

	_, err := ApplyPatchesChecked("α", []NotePatch{{Position: 1, Length: 1}})
	if err == nil {
		t.Fatal("ApplyPatchesChecked accepted a patch that split a UTF-8 encoding")
	}
	if !strings.Contains(err.Error(), "patch 0") {
		t.Fatalf("error %q does not identify the invalid patch", err)
	}
}

func TestApplyPatchesCheckedRejectsInvalidOriginal(t *testing.T) {
	t.Parallel()

	invalid := string([]byte{0xff})
	for _, patches := range [][]NotePatch{
		nil,
		{{Position: 0, Length: 1, Replacement: "valid"}},
	} {
		if _, err := ApplyPatchesChecked(invalid, patches); err == nil {
			t.Fatalf("ApplyPatchesChecked accepted invalid original with patches %+v", patches)
		}
	}
}

func TestApplyPatchesCheckedAllowsMidCodePointNoOpWhenResultIsValid(t *testing.T) {
	t.Parallel()

	got, err := ApplyPatchesChecked("α", []NotePatch{{Position: 1}})
	if err != nil {
		t.Fatalf("ApplyPatchesChecked rejected a valid result: %v", err)
	}
	if got != "α" {
		t.Fatalf("ApplyPatchesChecked = %q, want α", got)
	}
}

func TestApplyPatchesCheckedRejectsInvalidIntermediateResult(t *testing.T) {
	t.Parallel()

	patches := []NotePatch{
		{Position: 1, Length: 1}, // Leaves only the first byte of α.
		{Position: 0, Length: 1}, // Would make the final result valid again.
	}
	if got := ApplyPatches("α", patches); got != "" || !utf8.ValidString(got) {
		t.Fatalf("test setup produced %q, want a valid empty final result", got)
	}
	if _, err := ApplyPatchesChecked("α", patches); err == nil {
		t.Fatal("ApplyPatchesChecked accepted an invalid intermediate result")
	}
}

func TestApplyPatchesCheckedRejectsInvalidLaterPatch(t *testing.T) {
	t.Parallel()

	patches := []NotePatch{
		{Position: 0, Length: 0, Replacement: "A"},
		{Position: 2, Length: 1}, // Splits α after the valid insertion.
	}
	if _, err := ApplyPatchesChecked("α", patches); err == nil || !strings.Contains(err.Error(), "patch 1") {
		t.Fatalf("ApplyPatchesChecked error = %v, want invalid patch 1", err)
	}
}

func TestApplyPatchesCheckedValidNativeUnicodeDelta(t *testing.T) {
	t.Parallel()

	got, err := ApplyPatchesChecked("Native Task7 note α 🚀\nSecond line.", []NotePatch{{
		Position:    26,
		Length:      5,
		Replacement: "Update",
		Checksum:    3672733299,
	}})
	if err != nil {
		t.Fatalf("ApplyPatchesChecked: %v", err)
	}
	if got != "Native Task7 note α 🚀\nUpdated line." {
		t.Fatalf("ApplyPatchesChecked = %q", got)
	}
}
