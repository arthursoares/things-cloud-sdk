package thingscloud

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestEncodeUUID_CanonicalLeadingZeros(t *testing.T) {
	t.Parallel()

	u := uuid.UUID{0x00, 0x7f, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee}
	s := EncodeUUID(u)
	if !strings.HasPrefix(s, "1") {
		t.Errorf("EncodeUUID(%v) = %q, want leading %q for leading zero byte", u, s, "1")
	}

	u2 := uuid.UUID{0x00, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee}
	s2 := EncodeUUID(u2)
	if !strings.HasPrefix(s2, "11") {
		t.Errorf("EncodeUUID(%v) = %q, want two leading 1s for two leading zero bytes", u2, s2)
	}
}

func TestEncodeUUID_AllZero(t *testing.T) {
	t.Parallel()

	got := EncodeUUID(uuid.UUID{})
	want := "1111111111111111" // one '1' per zero byte
	if got != want {
		t.Errorf("EncodeUUID(zero) = %q, want %q", got, want)
	}
}

func TestEncodeDecodeUUID_RoundTrip(t *testing.T) {
	t.Parallel()

	cases := []uuid.UUID{
		{},
		{0x00, 0x7f, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee},
		{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	}
	for i := 0; i < 64; i++ {
		cases = append(cases, uuid.New())
	}
	// Force leading-zero-byte cases, the historical corruption source.
	for i := 0; i < 8; i++ {
		u := uuid.New()
		u[0] = 0x00
		cases = append(cases, u)
	}

	for _, u := range cases {
		s := EncodeUUID(u)
		back, err := DecodeUUID(s)
		if err != nil {
			t.Fatalf("DecodeUUID(EncodeUUID(%v) = %q) failed: %v", u, s, err)
		}
		if back != u {
			t.Errorf("round trip: %v -> %q -> %v", u, s, back)
		}
	}
}

func TestDecodeUUID_RealThingsIdentifiers(t *testing.T) {
	t.Parallel()

	// Captured from real Things.app sync traffic (HAR).
	for _, s := range []string{"VJ1edXTP9q3PmFDUuy8EQh", "FQxaqvLBkbR5q2Q5oRoknc", "BVU8qZ9dNjrdxLvDHPvfDS"} {
		u, err := DecodeUUID(s)
		if err != nil {
			t.Fatalf("DecodeUUID(%q) failed: %v", s, err)
		}
		if got := EncodeUUID(u); got != s {
			t.Errorf("re-encode of %q = %q, not canonical round trip", s, got)
		}
	}
}

func TestValidateUUID(t *testing.T) {
	t.Parallel()

	valid := []string{
		"VJ1edXTP9q3PmFDUuy8EQh",
		"1111111111111111",
		EncodeUUID(uuid.UUID{0x00, 0x7f, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee}),
	}
	for _, s := range valid {
		if err := ValidateUUID(s); err != nil {
			t.Errorf("ValidateUUID(%q) = %v, want nil", s, err)
		}
	}

	invalid := map[string]string{
		"standard hyphenated UUID":        "6f9b2c1e-8a4d-4e5f-9c3b-2a1d0e9f8b7c",
		"contains 0":                      "VJ0edXTP9q3PmFDUuy8EQh",
		"contains O":                      "VJOedXTP9q3PmFDUuy8EQh",
		"contains I":                      "VJIedXTP9q3PmFDUuy8EQh",
		"contains l":                      "VJledXTP9q3PmFDUuy8EQh",
		"empty":                           "",
		"value overflows 16 bytes":        "zzzzzzzzzzzzzzzzzzzzzz",
		"non-canonical missing leading 1": strings.TrimPrefix(EncodeUUID(uuid.UUID{0x00, 0x7f, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee}), "1"),
	}
	for name, s := range invalid {
		if err := ValidateUUID(s); err == nil {
			t.Errorf("ValidateUUID(%q) [%s] = nil, want error", s, name)
		}
	}
}

func TestNewUUID_AlwaysCanonical(t *testing.T) {
	t.Parallel()

	for i := 0; i < 2000; i++ {
		s := NewUUID()
		if err := ValidateUUID(s); err != nil {
			t.Fatalf("NewUUID() = %q is not canonical: %v", s, err)
		}
	}
}
