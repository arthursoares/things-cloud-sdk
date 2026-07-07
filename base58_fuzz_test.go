package thingscloud

import (
	"testing"

	"github.com/google/uuid"
)

// FuzzUUIDRoundTrip drives random 16-byte identifiers through
// EncodeUUID -> DecodeUUID and asserts identity. This is the property the
// old encoder violated: a UUID with a leading zero byte encoded one
// character short and decoded back to fewer than 16 bytes. The fuzzer
// reaches the ~1/256 leading-zero case within a handful of iterations.
func FuzzUUIDRoundTrip(f *testing.F) {
	f.Add(make([]byte, 16)) // all zero — worst case
	f.Add([]byte{0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13})
	f.Add([]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255, 255})
	seed := uuid.New()
	f.Add(seed[:])

	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) != 16 {
			return // only 16-byte inputs are valid UUIDs
		}
		var u uuid.UUID
		copy(u[:], b)

		s := EncodeUUID(u)

		// A canonical Things identifier is Base58 of 16 bytes: never
		// empty, and at most 22 characters (log_58(2^128) < 22).
		if len(s) == 0 || len(s) > 22 {
			t.Fatalf("EncodeUUID(% x) = %q has invalid length %d", b, s, len(s))
		}

		back, err := DecodeUUID(s)
		if err != nil {
			t.Fatalf("DecodeUUID(%q) from % x failed: %v", s, b, err)
		}
		if back != u {
			t.Fatalf("round trip broke: % x -> %q -> % x", b, s, back[:])
		}

		// The encoding must be the unique canonical form: re-encoding the
		// decoded value reproduces the same string.
		if again := EncodeUUID(back); again != s {
			t.Fatalf("encoding not canonical: %q re-encodes to %q", s, again)
		}
	})
}

// FuzzValidateUUIDIsSound treats ValidateUUID as the security boundary
// that keeps history-corrupting identifiers off the wire: whatever it
// accepts MUST decode to exactly 16 bytes and survive a re-encode. If it
// ever green-lights a string that doesn't, Write() would poison a history.
func FuzzValidateUUIDIsSound(f *testing.F) {
	f.Add("VJ1edXTP9q3PmFDUuy8EQh")               // real Things id
	f.Add("6f9b2c1e-8a4d-4e5f-9c3b-2a1d0e9f8b7c") // hyphenated
	f.Add("")                                     // empty
	f.Add("1111111111111111")                     // all-zero canonical
	f.Add("0OIl")                                 // non-alphabet chars

	f.Fuzz(func(t *testing.T, s string) {
		if err := ValidateUUID(s); err != nil {
			return // rejected — nothing to prove
		}
		// Accepted: it must be a canonical encoding of exactly 16 bytes.
		u, err := DecodeUUID(s)
		if err != nil {
			t.Fatalf("ValidateUUID accepted %q but DecodeUUID rejects it: %v", s, err)
		}
		if got := EncodeUUID(u); got != s {
			t.Fatalf("ValidateUUID accepted non-canonical %q (canonical form is %q)", s, got)
		}
	})
}

// FuzzDecodeUUIDNeverPanics ensures arbitrary caller/server-supplied
// strings can be validated without crashing the SDK.
func FuzzDecodeUUIDNeverPanics(f *testing.F) {
	f.Add("VJ1edXTP9q3PmFDUuy8EQh")
	f.Add("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")
	f.Add("\x00\xff")

	f.Fuzz(func(t *testing.T, s string) {
		_, _ = DecodeUUID(s) // must not panic for any input
	})
}
