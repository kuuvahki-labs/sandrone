package file

import "encoding/json"

// marshalStableJSON returns a 2-space indented JSON representation. JSON's
// own object key order is preserved by encoding/json's sorting of map keys,
// which is good enough for our deterministic output requirements.
func marshalStableJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}
