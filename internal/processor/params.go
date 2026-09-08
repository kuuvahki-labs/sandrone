package processor

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

// UnmarshalParams decodes spec.Params into target. Missing or empty params is
// not an error: target is left at its zero value (which can be a populated
// struct with defaults supplied by the caller).
//
// Decoding rejects unknown fields so a typo in the spec surfaces as
// processor_config_invalid instead of being silently ignored.
func UnmarshalParams[T any](spec domain.ProcessorSpec, target *T) error {
	if len(spec.Params) == 0 {
		return nil
	}
	full := map[string]jsontext.Value{}
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
	if err := json.Unmarshal(body, target, json.RejectUnknownMembers(true)); err != nil {
		return &domain.AppError{
			Code:      domain.CodeProcessorConfigInvalid,
			Message:   fmt.Sprintf("decode processor params for %q", spec.Type),
			Processor: spec.Type,
			Cause:     err,
		}
	}
	return nil
}
