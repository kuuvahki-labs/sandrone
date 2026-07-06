package processor

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

// UnmarshalParams decodes spec.Params into target. Missing or empty params is
// not an error: target is left at its zero value (which can be a populated
// struct with defaults supplied by the caller).
//
// Decoding rejects unknown fields so a typo in the spec surfaces as
// processor_config_invalid instead of being silently ignored.
func UnmarshalParams(spec domain.ProcessorSpec, target any) error {
	if len(spec.Params) == 0 {
		return nil
	}
	full := map[string]json.RawMessage{}
	for key, raw := range spec.Params {
		full[key] = raw
	}
	body, err := json.Marshal(full)
	if err != nil {
		return &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   "encode processor params",
			Processor: spec.Type,
			Cause:     err,
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   fmt.Sprintf("decode processor params for %q", spec.Type),
			Processor: spec.Type,
			Cause:     err,
		}
	}
	return nil
}
