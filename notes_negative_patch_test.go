package thingscloud

import "testing"

func TestApplyPatches_NegativePositionClampsToStart(t *testing.T) {
	t.Parallel()

	got := ApplyPatches("hello", []NotePatch{{Replacement: "X", Position: -1, Length: 1}})
	if got != "Xello" {
		t.Errorf("ApplyPatches negative position = %q, want %q", got, "Xello")
	}
}

func TestApplyPatches_NegativeLengthDoesNotPanic(t *testing.T) {
	t.Parallel()

	// A hostile or corrupted server payload with a negative length must not
	// crash the SDK. A negative length is treated as zero (pure insertion).
	got := ApplyPatches("hello", []NotePatch{{Replacement: "X", Position: 0, Length: -5}})
	if got != "Xhello" {
		t.Errorf("ApplyPatches negative length = %q, want %q", got, "Xhello")
	}

	got = ApplyPatches("hello", []NotePatch{{Replacement: "", Position: 3, Length: -1}})
	if got != "hello" {
		t.Errorf("ApplyPatches negative length no-op = %q, want %q", got, "hello")
	}
}

func TestApplyPatches_LengthOverflowClampsToEnd(t *testing.T) {
	t.Parallel()

	maxInt := int(^uint(0) >> 1)
	got := ApplyPatches("AB", []NotePatch{{Replacement: "X", Position: 1, Length: maxInt}})
	if got != "AX" {
		t.Errorf("ApplyPatches overflowing length = %q, want %q", got, "AX")
	}
}
