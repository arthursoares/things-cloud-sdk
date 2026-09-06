package thingscloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type task7WritePlan struct {
	modificationTargets map[string]struct{}
	creationTargets     map[string]struct{}
	tombstoneTargets    map[string]struct{}
}

func (p *task7WritePlan) needsPreflight() bool { return len(p.modificationTargets) != 0 }

func validateTaskWrites(body []byte) (*task7WritePlan, error) {
	var envelopes map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelopes); err != nil {
		return nil, fmt.Errorf("refusing to write an invalid item envelope")
	}
	plan := &task7WritePlan{
		modificationTargets: make(map[string]struct{}),
		creationTargets:     make(map[string]struct{}),
		tombstoneTargets:    make(map[string]struct{}),
	}
	type tombstoneEnvelope struct {
		kind string
		raw  json.RawMessage
	}
	var tombstones []tombstoneEnvelope
	for id, raw := range envelopes {
		if err := rejectDuplicateJSONKeys(raw, false); err != nil {
			return nil, fmt.Errorf("refusing to write an invalid item envelope")
		}
		var legacyEnvelope struct {
			Kind ItemKind `json:"e"`
		}
		if err := json.Unmarshal(raw, &legacyEnvelope); err != nil {
			return nil, fmt.Errorf("refusing to write an invalid item envelope")
		}
		kind := string(legacyEnvelope.Kind)
		switch kind {
		case "Task7":
			if !utf8.Valid(raw) || rejectUnpairedJSONSurrogates(raw) != nil {
				return nil, fmt.Errorf("refusing to write invalid Task7 envelope")
			}
			if err := rejectDuplicateJSONKeys(raw, true); err != nil {
				return nil, fmt.Errorf("refusing to write invalid Task7 envelope")
			}
			fields, err := jsonObject(raw)
			if err != nil {
				return nil, fmt.Errorf("refusing to write invalid Task7 envelope")
			}
			action, ok := jsonInteger(fields["t"])
			if !ok || len(fields) != 3 {
				return nil, fmt.Errorf("refusing to write invalid Task7 envelope")
			}
			payload, err := jsonObject(fields["p"])
			if err != nil {
				return nil, fmt.Errorf("refusing to write invalid Task7 payload")
			}
			switch action {
			case int64(ItemActionCreated):
				err = validateTask7Create(payload)
				if err == nil {
					plan.creationTargets[id] = struct{}{}
				}
			case int64(ItemActionModified):
				err = validateTask7Modification(payload)
				if err == nil {
					plan.modificationTargets[id] = struct{}{}
				}
			default:
				err = fmt.Errorf("Task7 action %d is outside the verified write scope", action)
			}
			if err != nil {
				return nil, fmt.Errorf("refusing to write Task7: %w", err)
			}
		case "Task6", "Task4", "Task3", "Task":
			// Preserve existing write kinds and serialization behavior.
		case "Tombstone2", "Tombstone":
			tombstones = append(tombstones, tombstoneEnvelope{kind: kind, raw: raw})
		default:
			if strings.HasPrefix(kind, "Task") {
				return nil, fmt.Errorf("refusing to write unverified task kind %s", taskKindDiagnostic(ItemKind(kind)))
			}
		}
	}
	if len(plan.creationTargets)+len(plan.modificationTargets) != 0 {
		for _, tombstone := range tombstones {
			target, err := validateTask7CompanionTombstone(tombstone.raw, tombstone.kind)
			if err != nil {
				return nil, fmt.Errorf("refusing to write Task7 batch: invalid tombstone companion")
			}
			plan.tombstoneTargets[target] = struct{}{}
		}
	}
	for _, targets := range []map[string]struct{}{plan.creationTargets, plan.modificationTargets} {
		for target := range targets {
			if _, ambiguous := plan.tombstoneTargets[target]; ambiguous {
				return nil, fmt.Errorf("refusing to write Task7: target %s is both changed and deleted in one commit", target)
			}
		}
	}
	return plan, nil
}

func validateTask7CompanionTombstone(raw json.RawMessage, kind string) (string, error) {
	if !utf8.Valid(raw) || rejectUnpairedJSONSurrogates(raw) != nil || rejectDuplicateJSONKeys(raw, true) != nil {
		return "", fmt.Errorf("invalid tombstone envelope")
	}
	fields, err := jsonObject(raw)
	if err != nil || exactKeys(fields, map[string]struct{}{"e": {}, "t": {}, "p": {}}) != nil {
		return "", fmt.Errorf("invalid tombstone envelope")
	}
	action, ok := jsonInteger(fields["t"])
	if !ok || action != 0 {
		return "", fmt.Errorf("invalid tombstone action")
	}
	payload, err := jsonObject(fields["p"])
	if err != nil || exactKeys(payload, map[string]struct{}{"dloid": {}, "dld": {}}) != nil {
		return "", fmt.Errorf("invalid tombstone payload")
	}
	target, ok := jsonString(payload["dloid"])
	if !ok || target == "" || requiredTimestamp(payload["dld"], true) != nil {
		return "", fmt.Errorf("invalid tombstone payload")
	}
	if kind == "Tombstone2" {
		if ValidateUUID(target) != nil {
			return "", fmt.Errorf("invalid tombstone target")
		}
	} else if ValidateUUID(target) != nil {
		target = EncodeLegacyIdentifier(target)
	}
	return target, nil
}

var task7CreateKeys = map[string]struct{}{
	"tp": {}, "sr": {}, "dds": {}, "rt": {}, "rmd": {}, "ss": {}, "tr": {}, "dl": {},
	"icp": {}, "st": {}, "ar": {}, "tt": {}, "do": {}, "lai": {}, "tir": {}, "tg": {},
	"agr": {}, "ix": {}, "cd": {}, "lt": {}, "icc": {}, "md": {}, "ti": {}, "dd": {},
	"ato": {}, "nt": {}, "icsd": {}, "pr": {}, "rp": {}, "acrd": {}, "sp": {}, "sb": {},
	"rr": {}, "xx": {},
}

func validateTask7Create(p map[string]json.RawMessage) error {
	if err := exactKeys(p, task7CreateKeys); err != nil {
		return fmt.Errorf("create payload %w", err)
	}
	tp, ok := jsonInteger(p["tp"])
	if !ok || tp < 0 || tp > 2 {
		return fmt.Errorf("create tp must be 0, 1, or 2")
	}
	st, ok := jsonInteger(p["st"])
	if !ok || st < 0 || st > 2 || (tp != 0 && st != 1) {
		return fmt.Errorf("create st is invalid for task type")
	}
	if ss, ok := jsonInteger(p["ss"]); !ok || ss != 0 {
		return fmt.Errorf("create ss must be 0")
	}
	if tr, ok := jsonBool(p["tr"]); !ok || tr {
		return fmt.Errorf("create tr must be false")
	}
	for _, key := range []string{"icp", "lt"} {
		if value, ok := jsonBool(p[key]); !ok || value {
			return fmt.Errorf("create %s must be false", key)
		}
	}
	for _, key := range []string{"do", "ix", "icc", "ti", "sb"} {
		if value, ok := jsonInteger(p[key]); !ok || value != 0 {
			return fmt.Errorf("create %s must be 0", key)
		}
	}
	if _, ok := jsonString(p["tt"]); !ok {
		return fmt.Errorf("create tt must be a string")
	}
	if err := requiredTimestamp(p["cd"], true); err != nil {
		return fmt.Errorf("create cd %w", err)
	}
	for _, key := range []string{"sr", "tir", "dd"} {
		if err := nullableTimestamp(p[key]); err != nil {
			return fmt.Errorf("create %s %w", key, err)
		}
	}
	srNull, tirNull := jsonNull(p["sr"]), jsonNull(p["tir"])
	if srNull != tirNull {
		return fmt.Errorf("create sr and tir must have the same date")
	}
	if st == 0 && !srNull {
		return fmt.Errorf("create st 0 requires null sr and tir")
	}
	if !srNull {
		var sr, tir float64
		_ = json.Unmarshal(p["sr"], &sr)
		_ = json.Unmarshal(p["tir"], &tir)
		if sr != tir {
			return fmt.Errorf("create sr and tir must have the same date")
		}
	}
	for _, key := range []string{"dds", "rmd", "lai", "md", "ato", "icsd", "rp", "acrd", "sp", "rr"} {
		if !jsonNull(p[key]) {
			return fmt.Errorf("create %s must be null", key)
		}
	}
	for _, key := range []string{"rt", "dl"} {
		values, err := jsonStringArray(p[key])
		if err != nil || len(values) != 0 {
			return fmt.Errorf("create %s must be an empty array", key)
		}
	}
	for _, key := range []string{"ar", "pr", "agr", "tg"} {
		if err := validateReferenceArray(p[key]); err != nil {
			return fmt.Errorf("create %s %w", key, err)
		}
		if key != "tg" {
			values, _ := jsonStringArray(p[key])
			if len(values) > 1 {
				return fmt.Errorf("create %s supports at most one parent identifier", key)
			}
		}
	}
	if err := validateFullTextNote(p["nt"]); err != nil {
		return fmt.Errorf("create nt %w", err)
	}
	if err := validateDefaultExtension(p["xx"]); err != nil {
		return fmt.Errorf("create xx %w", err)
	}
	return nil
}

var task7ModificationKeys = map[string]struct{}{
	"md": {}, "tt": {}, "nt": {}, "st": {}, "sr": {}, "tir": {}, "dd": {}, "ar": {},
	"pr": {}, "agr": {}, "tg": {}, "ss": {}, "sp": {}, "tr": {}, "ix": {}, "ti": {}, "do": {},
}

func validateTask7Modification(p map[string]json.RawMessage) error {
	for key := range p {
		if _, ok := task7ModificationKeys[key]; !ok {
			return fmt.Errorf("modification contains a property outside the verified write scope")
		}
	}
	if err := requiredTimestamp(p["md"], true); err != nil {
		return fmt.Errorf("modification md %w", err)
	}
	if raw, ok := p["tt"]; ok {
		if _, ok := jsonString(raw); !ok {
			return fmt.Errorf("modification tt must be a string")
		}
	}
	if raw, ok := p["nt"]; ok {
		if err := validateFullTextNote(raw); err != nil {
			return fmt.Errorf("modification nt %w", err)
		}
	}
	if raw, ok := p["st"]; ok {
		value, ok := jsonInteger(raw)
		if !ok || value < 0 || value > 2 {
			return fmt.Errorf("modification st must be 0, 1, or 2")
		}
	}
	if raw, ok := p["ss"]; ok {
		value, ok := jsonInteger(raw)
		if !ok || (value != 0 && value != 3) {
			return fmt.Errorf("modification ss must be 0 or 3")
		}
	}
	ssRaw, hasStatus := p["ss"]
	spRaw, hasStopDate := p["sp"]
	if hasStatus != hasStopDate {
		return fmt.Errorf("modification ss and sp must be written together")
	}
	if hasStatus {
		status, _ := jsonInteger(ssRaw)
		if status == 0 && !jsonNull(spRaw) {
			return fmt.Errorf("modification ss 0 requires null sp")
		}
		if status == 3 {
			if jsonNull(spRaw) || requiredTimestamp(spRaw, true) != nil {
				return fmt.Errorf("modification ss 3 requires a positive sp timestamp")
			}
		}
	}
	_, hasSchedule := p["st"]
	_, hasStartDate := p["sr"]
	_, hasTodayReference := p["tir"]
	if hasSchedule || hasStartDate || hasTodayReference {
		if !hasSchedule || !hasStartDate || !hasTodayReference {
			return fmt.Errorf("modification st, sr, and tir must be written together")
		}
		st, _ := jsonInteger(p["st"])
		srNull, tirNull := jsonNull(p["sr"]), jsonNull(p["tir"])
		if srNull != tirNull {
			return fmt.Errorf("modification sr and tir must have the same date")
		}
		if st == 0 && !srNull {
			return fmt.Errorf("modification st 0 requires null sr and tir")
		}
		if !srNull {
			var sr, tir float64
			_ = json.Unmarshal(p["sr"], &sr)
			_ = json.Unmarshal(p["tir"], &tir)
			if sr != tir {
				return fmt.Errorf("modification sr and tir must have the same date")
			}
		}
	}
	for _, key := range []string{"sr", "tir", "dd", "sp"} {
		if raw, ok := p[key]; ok {
			if err := nullableTimestamp(raw); err != nil {
				return fmt.Errorf("modification %s %w", key, err)
			}
		}
	}
	for _, key := range []string{"ar", "pr", "agr", "tg"} {
		if raw, ok := p[key]; ok {
			if err := validateReferenceArray(raw); err != nil {
				return fmt.Errorf("modification %s %w", key, err)
			}
		}
	}
	if raw, ok := p["tr"]; ok {
		if _, ok := jsonBool(raw); !ok {
			return fmt.Errorf("modification tr must be a boolean")
		}
	}
	for _, key := range []string{"ix", "ti", "do"} {
		if raw, ok := p[key]; ok {
			if _, ok := jsonInteger(raw); !ok {
				return fmt.Errorf("modification %s must be an integer", key)
			}
		}
	}
	return nil
}

func validateFullTextNote(raw json.RawMessage) error {
	note, err := jsonObject(raw)
	if err != nil {
		return fmt.Errorf("must be an object")
	}
	want := map[string]struct{}{"_t": {}, "ch": {}, "v": {}, "t": {}}
	if err := exactKeys(note, want); err != nil {
		return err
	}
	tag, tagOK := jsonString(note["_t"])
	typ, typeOK := jsonInteger(note["t"])
	value, valueOK := jsonString(note["v"])
	checksum, checksumOK := jsonInteger(note["ch"])
	if !tagOK || tag != "tx" || !typeOK || typ != NoteTypeFullText || !valueOK || !checksumOK || checksum < 0 || checksum > math.MaxUint32 {
		return fmt.Errorf("must be a tx full-text note with string value and CRC32 checksum")
	}
	if uint32(checksum) != crc32.ChecksumIEEE([]byte(value)) {
		return fmt.Errorf("has a checksum that does not match its full text")
	}
	return nil
}

func validateDefaultExtension(raw json.RawMessage) error {
	extension, err := jsonObject(raw)
	if err != nil {
		return fmt.Errorf("must be an object")
	}
	if err := exactKeys(extension, map[string]struct{}{"sn": {}, "_t": {}}); err != nil {
		return err
	}
	tag, ok := jsonString(extension["_t"])
	if !ok || tag != "oo" {
		return fmt.Errorf("must have _t oo")
	}
	sn, err := jsonObject(extension["sn"])
	if err != nil || len(sn) != 0 {
		return fmt.Errorf("must have an empty sn object")
	}
	return nil
}

func validateReferenceArray(raw json.RawMessage) error {
	values, err := jsonStringArray(raw)
	if err != nil {
		return fmt.Errorf("must be an array of identifiers")
	}
	for _, id := range values {
		if err := ValidateUUID(id); err != nil {
			return fmt.Errorf("contains an invalid identifier")
		}
	}
	return nil
}

func jsonObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	data := bytes.TrimSpace(raw)
	if len(data) == 0 || data[0] != '{' {
		return nil, fmt.Errorf("must be an object")
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil || result == nil {
		return nil, fmt.Errorf("must be an object")
	}
	return result, nil
}

func exactKeys(values map[string]json.RawMessage, want map[string]struct{}) error {
	if len(values) != len(want) {
		return fmt.Errorf("must contain exactly the verified properties")
	}
	for key := range values {
		if _, ok := want[key]; !ok {
			return fmt.Errorf("contains unsupported property %q", key)
		}
	}
	return nil
}

func jsonNull(raw json.RawMessage) bool { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

func jsonString(raw json.RawMessage) (string, bool) {
	data := bytes.TrimSpace(raw)
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return "", false
	}
	var value string
	if json.Unmarshal(data, &value) != nil {
		return "", false
	}
	return value, true
}

func jsonBool(raw json.RawMessage) (bool, bool) {
	switch string(bytes.TrimSpace(raw)) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}

func jsonInteger(raw json.RawMessage) (int64, bool) {
	data := bytes.TrimSpace(raw)
	if len(data) == 0 {
		return 0, false
	}
	value, err := strconv.ParseInt(string(data), 10, 64)
	return value, err == nil
}

// rejectDuplicateJSONKeys checks the envelope object itself for every write.
// Task7 additionally checks every nested object because validation must not
// approve one interpretation while the server applies another duplicate key.
func rejectDuplicateJSONKeys(raw json.RawMessage, recursive bool) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := consumeJSONValue(decoder, recursive, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("extra JSON value")
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder, recursive bool, depth int) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if depth == 0 || recursive {
				if _, duplicate := seen[key]; duplicate {
					return fmt.Errorf("duplicate object key")
				}
				seen[key] = struct{}{}
			}
			if err := consumeJSONValue(decoder, recursive, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("unterminated object")
		}
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder, recursive, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("unterminated array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter")
	}
	return nil
}

func rejectUnpairedJSONSurrogates(raw []byte) error {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '"' {
			continue
		}
		for i++; i < len(raw) && raw[i] != '"'; i++ {
			if raw[i] != '\\' {
				continue
			}
			i++
			if i >= len(raw) || raw[i] != 'u' {
				continue
			}
			value, ok := jsonHex16(raw, i+1)
			if !ok {
				return fmt.Errorf("invalid unicode escape")
			}
			i += 4
			switch {
			case value >= 0xd800 && value <= 0xdbff:
				if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
					return fmt.Errorf("unpaired unicode surrogate")
				}
				low, ok := jsonHex16(raw, i+3)
				if !ok || low < 0xdc00 || low > 0xdfff {
					return fmt.Errorf("unpaired unicode surrogate")
				}
				i += 6
			case value >= 0xdc00 && value <= 0xdfff:
				return fmt.Errorf("unpaired unicode surrogate")
			}
		}
	}
	return nil
}

func jsonHex16(raw []byte, start int) (uint16, bool) {
	if start+4 > len(raw) {
		return 0, false
	}
	value, err := strconv.ParseUint(string(raw[start:start+4]), 16, 16)
	return uint16(value), err == nil
}

func jsonStringArray(raw json.RawMessage) ([]string, error) {
	var values []string
	if len(raw) == 0 || json.Unmarshal(raw, &values) != nil || values == nil {
		return nil, fmt.Errorf("must be an array of strings")
	}
	return values, nil
}

const (
	minTaskTimestamp = -62135596800.0
	maxTaskTimestamp = 253402300799.0
)

func nullableTimestamp(raw json.RawMessage) error {
	if jsonNull(raw) {
		return nil
	}
	return requiredTimestamp(raw, false)
}

func requiredTimestamp(raw json.RawMessage, positive bool) error {
	var value float64
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("must be a finite timestamp")
	}
	if value < minTaskTimestamp || value > maxTaskTimestamp || (positive && value <= 0) {
		return fmt.Errorf("must be a valid timestamp")
	}
	return nil
}
