package thingscloud

import (
	"crypto/sha1"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

// base58Alphabet is the Bitcoin Base58 alphabet Things.app uses for
// identifiers (no 0, O, I, l). Things decodes identifiers with
// BSIdentifierFromBase58String; anything it cannot decode to 16 bytes
// permanently corrupts the sync history and crashes the client.
const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// EncodeUUID encodes a UUID as canonical Base58: one leading '1' per
// leading zero byte, matching the identifiers Things.app itself emits.
func EncodeUUID(u uuid.UUID) string {
	zeros := 0
	for zeros < len(u) && u[zeros] == 0 {
		zeros++
	}

	n := new(big.Int).SetBytes(u[:])
	base := big.NewInt(58)
	mod := new(big.Int)
	var encoded []byte
	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		encoded = append(encoded, base58Alphabet[mod.Int64()])
	}
	for i := 0; i < zeros; i++ {
		encoded = append(encoded, '1')
	}
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}
	return string(encoded)
}

// EncodeLegacyIdentifier derives the current Base58 identifier for an object
// created before Things Cloud's identifier migration. Old history items
// (Task3, Task4, Area2, ...) key objects by identifier strings such as
// uppercase UUIDs; newer items (Task6, ...) key the same objects by
//
//	Base58(SHA1(legacyID)[:16])
//
// The history contains no records linking the two generations; this
// derivation is the only link. legacyID must be the identifier text exactly
// as stored in the history: hashing is byte-exact, so any normalization
// (such as lowercasing a UUID) yields a different identifier. SHA-1 mirrors
// the derivation Things itself performs; it is not a security primitive
// here.
//
// Recurring-task instances use composite identifiers of the form
// <uuid>-YYYYMMDD and hash in two steps: first the <uuid> prefix alone,
// then its raw 16-byte digest followed by the "-YYYYMMDD" text.
//
// The empty string is returned unchanged: a malformed item should keep an
// obviously-bogus key rather than gain a plausible-looking derived one.
func EncodeLegacyIdentifier(legacyID string) string {
	if legacyID == "" {
		return ""
	}
	input := []byte(legacyID)
	if prefix, suffix, ok := splitLegacyRecurrenceIdentifier(legacyID); ok {
		prefixSum := sha1.Sum([]byte(prefix))
		input = make([]byte, 0, 16+len(suffix))
		input = append(input, prefixSum[:16]...)
		input = append(input, suffix...)
	}
	sum := sha1.Sum(input)
	var u uuid.UUID
	copy(u[:], sum[:len(u)])
	return EncodeUUID(u)
}

func splitLegacyRecurrenceIdentifier(id string) (string, []byte, bool) {
	const uuidLength = 36
	const dateSuffixLength = len("-YYYYMMDD")
	if len(id) != uuidLength+dateSuffixLength {
		return "", nil, false
	}
	prefix := id[:uuidLength]
	if _, err := uuid.Parse(prefix); err != nil || id[uuidLength] != '-' {
		return "", nil, false
	}
	for i := uuidLength + 1; i < len(id); i++ {
		if id[i] < '0' || id[i] > '9' {
			return "", nil, false
		}
	}
	return prefix, []byte(id[uuidLength:]), true
}

// DecodeUUID decodes a canonical Base58 identifier back into a UUID.
// It rejects strings that are not canonical encodings of exactly 16 bytes.
func DecodeUUID(s string) (uuid.UUID, error) {
	var u uuid.UUID
	if s == "" {
		return u, fmt.Errorf("thingscloud: empty identifier")
	}

	zeros := 0
	for zeros < len(s) && s[zeros] == '1' {
		zeros++
	}

	n := new(big.Int)
	base := big.NewInt(58)
	for i := zeros; i < len(s); i++ {
		idx := strings.IndexByte(base58Alphabet, s[i])
		if idx < 0 {
			return u, fmt.Errorf("thingscloud: invalid Base58 character %q in identifier %q", s[i], s)
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(idx)))
	}

	value := n.Bytes()
	if zeros+len(value) != len(u) {
		return u, fmt.Errorf("thingscloud: identifier %q decodes to %d bytes, want 16 (non-canonical or wrong length)", s, zeros+len(value))
	}
	copy(u[len(u)-len(value):], value)
	return u, nil
}

// ValidateUUID reports whether s is a canonical Base58-encoded 16-byte
// identifier safe to write to Things Cloud.
func ValidateUUID(s string) error {
	_, err := DecodeUUID(s)
	return err
}

// NewUUID returns a new random identifier in the canonical Base58 wire
// format expected by Things Cloud.
func NewUUID() string {
	return EncodeUUID(uuid.New())
}
