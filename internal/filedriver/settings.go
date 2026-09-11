package filedriver

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"sort"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/jsonvalue"
)

type MihomoFileSettings struct {
	Groups   []map[string]any `json:"groups" jsonschema:"Explicit Mihomo proxy-group objects"`
	RuleSets []map[string]any `json:"rule_sets" jsonschema:"Explicit Mihomo rule-provider objects"`
	Rules    []string         `json:"rules" jsonschema:"Ordered Mihomo rule strings"`
}

type SingBoxFileSettings struct {
	Groups   []map[string]any `json:"groups" jsonschema:"Explicit sing-box selector or URL-test outbounds"`
	RuleSets []map[string]any `json:"rule_sets" jsonschema:"Explicit sing-box route rule-set objects"`
	Rules    []map[string]any `json:"rules" jsonschema:"Explicit sing-box route rule objects"`
}

func decodeMihomoFileSettings(raw jsontext.Value) (MihomoFileSettings, error) {
	var settings MihomoFileSettings
	if err := decodeTypedFileSettings(domain.FileKindMihomo, raw, &settings, "groups", "rule_sets", "rules"); err != nil {
		return MihomoFileSettings{}, err
	}
	return settings, nil
}

func decodeSingBoxFileSettings(raw jsontext.Value) (SingBoxFileSettings, error) {
	var settings SingBoxFileSettings
	if err := decodeTypedFileSettings(domain.FileKindSingBox, raw, &settings, "groups", "rule_sets", "rules"); err != nil {
		return SingBoxFileSettings{}, err
	}
	return settings, nil
}

func decodeTypedFileSettings[T any](kind domain.FileKind, raw jsontext.Value, out *T, requiredFields ...string) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q config.settings is required", kind))
	}
	var fields map[string]jsontext.Value
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
	if err := json.Unmarshal(raw, out, jsonvalue.PreserveNumbers, json.RejectUnknownMembers(true)); err != nil {
		return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q config.settings: %s", kind, settingsDecodeErrorPath(err)))
	}
	for _, name := range requiredFields {
		if _, ok := fields[name]; !ok {
			return domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("file kind %q config.settings.%s is required", kind, name))
		}
	}
	return nil
}

func settingsDecodeErrorPath(err error) string {
	if semantic, ok := errors.AsType[*json.SemanticError](err); ok {
		path := "config.settings"
		for token := range semantic.JSONPointer.Tokens() {
			path += "." + token
		}
		if errors.Is(err, json.ErrUnknownName) {
			return "unknown field " + path
		}
		if semantic.GoType != nil {
			return fmt.Sprintf("%s: expected %s", path, semantic.GoType)
		}
	}
	return err.Error()
}
