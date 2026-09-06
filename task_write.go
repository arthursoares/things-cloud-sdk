package thingscloud

import (
	"encoding/json"
	"fmt"
	"strings"
)

// validateTaskWriteKinds checks the serialized envelopes, including custom
// Identifiable implementations. Supporting a new kind for reads must not
// accidentally enable writes whose contract has not been verified.
func validateTaskWriteKinds(body []byte) error {
	var envelopes map[string]struct {
		Kind ItemKind `json:"e"`
	}
	if err := json.Unmarshal(body, &envelopes); err != nil {
		return fmt.Errorf("refusing to write an invalid item envelope")
	}
	for _, envelope := range envelopes {
		switch envelope.Kind {
		case "Task6", "Task4", "Task3", "Task":
			continue // Preserve existing write kinds; the CLI still emits Task6.
		}
		if strings.HasPrefix(string(envelope.Kind), "Task") {
			return fmt.Errorf("refusing to write unverified task kind %s", taskKindDiagnostic(envelope.Kind))
		}
	}
	return nil
}
