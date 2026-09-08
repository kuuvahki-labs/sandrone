// Package jsonvalue provides exact JSON values for untyped protocol data.
package jsonvalue

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
)

// PreserveNumbers keeps JSON numbers in untyped maps as raw JSON values, so
// forwarding provider fields does not round integers through float64.
var PreserveNumbers = json.WithUnmarshalers(json.UnmarshalFromFunc(func(dec *jsontext.Decoder, value *any) error {
	if dec.PeekKind() == '0' {
		*value = jsontext.Value(nil)
	}
	return errors.ErrUnsupported
}))
