package file

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// marshalStableJSON returns deterministic, 2-space indented JSON.
func marshalStableJSON(v any) ([]byte, error) {
	return json.Marshal(v, json.Deterministic(true), jsontext.WithIndent("  "))
}
