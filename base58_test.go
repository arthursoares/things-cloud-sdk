package thingscloud

import (
	"crypto/sha1"
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

func TestEncodeLegacyIdentifier(t *testing.T) {
	t.Parallel()

	const legacyID = "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"
	const want = "LXmxn9gakySzcEjKj1DtgD"
	if got := EncodeLegacyIdentifier(legacyID); got != want {
		t.Errorf("EncodeLegacyIdentifier(%q) = %q, want %q", legacyID, got, want)
	}
	if got := EncodeLegacyIdentifier(strings.ToLower(legacyID)); got == want {
		t.Errorf("EncodeLegacyIdentifier normalized identifier case; input must be hashed exactly as stored")
	}
}

func TestEncodeLegacyIdentifier_RecurrenceInstance(t *testing.T) {
	t.Parallel()

	const legacyID = "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE-20240131"
	const want = "H8Xu72gj7fooPuYoBMZ5TK"
	if got := EncodeLegacyIdentifier(legacyID); got != want {
		t.Errorf("EncodeLegacyIdentifier(%q) = %q, want %q", legacyID, got, want)
	}
	if got := EncodeLegacyIdentifier(strings.ToLower(legacyID)); got == want {
		t.Errorf("EncodeLegacyIdentifier normalized recurrence identifier case; input must be hashed exactly as stored")
	}
}

func TestEncodeLegacyIdentifier_MalformedRecurrenceSuffixes(t *testing.T) {
	t.Parallel()

	// Near misses of the <uuid>-YYYYMMDD form must hash as ordinary
	// identifier text, never through the recurrence formula.
	tests := []struct {
		name string
		id   string
	}{
		{"non-digit in date", "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE-2024013X"},
		{"date too short", "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE-2024013"},
		{"date too long", "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE-202401310"},
		{"invalid uuid prefix", "GGGGGGGG-BBBB-CCCC-DDDD-EEEEEEEEEEEE-20240131"},
		{"missing date separator", "AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE920240131"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sum := sha1.Sum([]byte(test.id))
			var u uuid.UUID
			copy(u[:], sum[:len(u)])
			want := EncodeUUID(u)
			if got := EncodeLegacyIdentifier(test.id); got != want {
				t.Errorf("EncodeLegacyIdentifier(%q) = %q, want ordinary derivation %q", test.id, got, want)
			}
		})
	}
}

func TestEncodeLegacyIdentifier_LeadingZeroClasses(t *testing.T) {
	t.Parallel()

	// Derived identifiers are 21 or 22 characters depending on leading zero
	// bytes in the truncated digest; roughly 3% land at 21 characters and
	// some start with '1'. These are exactly the classes behind the
	// leading-zero corruption documented in docs/client-side-bugs.md, so pin
	// one exact vector for each.
	tests := []struct {
		name string
		id   string
		want string
	}{
		{"21-character result", "00000006-1111-2222-3333-000000000006", "fxsSvCT97pJn3XZ4wp5t4"},
		{"leading-1 result", "000000B4-1111-2222-3333-0000000000B4", "14q4keicwiREVK8EKAuowZ"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := EncodeLegacyIdentifier(test.id); got != test.want {
				t.Errorf("EncodeLegacyIdentifier(%q) = %q, want %q", test.id, got, test.want)
			}
		})
	}
}

func TestEncodeLegacyIdentifier_Empty(t *testing.T) {
	t.Parallel()

	if got := EncodeLegacyIdentifier(""); got != "" {
		t.Errorf("EncodeLegacyIdentifier(\"\") = %q, want empty string passed through", got)
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
