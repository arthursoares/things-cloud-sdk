package thingscloud

import "testing"

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
