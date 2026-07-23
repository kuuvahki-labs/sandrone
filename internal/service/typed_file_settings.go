package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type mihomoFileSettings struct {
	AdaptiveGroups *domain.FileAdaptiveGroupConfig `json:"adaptive_groups,omitempty"`
	Groups         []map[string]any                `json:"groups,omitempty"`
	RuleSets       []map[string]any                `json:"rule_sets,omitempty"`
	Rules          []string                        `json:"rules,omitempty"`
}

type singBoxFileSettings struct {
	Groups   []map[string]any `json:"groups,omitempty"`
	RuleSets []map[string]any `json:"rule_sets,omitempty"`
	Rules    []map[string]any `json:"rules,omitempty"`
}

func decodeMihomoFileSettings(raw json.RawMessage) (mihomoFileSettings, error) {
	if err := validateMihomoAdaptiveGroupFields(raw); err != nil {
		return mihomoFileSettings{}, domain.NewError(
			domain.CodeInvalidArgument,
			fmt.Sprintf("file kind %q %v", domain.FileKindMihomo, err),
		)
	}
	var settings mihomoFileSettings
	if err := decodeTypedFileSettings(domain.FileKindMihomo, raw, &settings); err != nil {
		return mihomoFileSettings{}, err
	}
	return settings, nil
}

func validateMihomoAdaptiveGroupFields(raw json.RawMessage) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil
	}
	adaptive, ok := fields["adaptive_groups"]
	if !ok || isJSONNull(adaptive) {
		return nil
	}
	adaptiveFields, err := strictJSONObject(adaptive, "config.settings.adaptive_groups")
	if err != nil {
		return err
	}
	return rejectUnknownJSONFields(adaptiveFields, map[string]bool{
		"type":    true,
		"regions": true,
	}, "config.settings.adaptive_groups")
}

func decodeSingBoxFileSettings(raw json.RawMessage) (singBoxFileSettings, error) {
	var settings singBoxFileSettings
	if err := decodeTypedFileSettings(domain.FileKindSingBox, raw, &settings); err != nil {
		return singBoxFileSettings{}, err
	}
	return settings, nil
}

func decodeTypedFileSettings(kind domain.FileKind, raw json.RawMessage, out any) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		if err == nil {
			err = fmt.Errorf("must be an object")
		}
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q config.settings: %v", kind, err))
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := fields[name]
		if string(bytes.TrimSpace(value)) == "null" {
			return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q config.settings.%s must not be null", kind, name))
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q config.settings: %s", kind, settingsDecodeErrorPath(err)))
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q config.settings: %v", kind, err))
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("must contain a single object")
	}
	return err
}

func settingsDecodeErrorPath(err error) string {
	message := err.Error()
	const unknownPrefix = "json: unknown field "
	if strings.HasPrefix(message, unknownPrefix) {
		name := strings.Trim(message[len(unknownPrefix):], `"`)
		return "unknown field config.settings." + name
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		path := "config.settings"
		if typeErr.Field != "" {
			path += "." + typeErr.Field
		}
		return fmt.Sprintf("%s: expected %s", path, typeErr.Type)
	}
	return message
}
