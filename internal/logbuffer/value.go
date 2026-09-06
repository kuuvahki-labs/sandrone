package logbuffer

import (
	"encoding/json/v2"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"
)

func prefix(value string, limit int) string {
	if len(value) <= limit {
		return strings.Clone(value)
	}
	for limit > 0 && !utf8.RuneStart(value[limit]) {
		limit--
	}
	return strings.Clone(value[:limit])
}

type normalizer struct {
	remaining int
	truncated bool
	depth     int
}

func (s *normalizer) text(value string) string {
	if !utf8.ValidString(value) {
		s.truncated = true
		value = strings.ToValidUTF8(value, "�")
	}
	limit := s.remaining
	if len(value) > limit {
		s.truncated = true
	}
	value = prefix(value, limit)
	s.remaining -= len(value)
	return value
}

func (s *normalizer) value(value any, depth int) any {
	if depth > 8 || s.remaining <= 0 {
		s.truncated = true
		return "[truncated]"
	}
	s.remaining--
	switch v := value.(type) {
	case string:
		return s.text(v)
	case map[string]any:
		out := make(map[string]any)
		for key, child := range v {
			if s.remaining <= 0 || len(out) >= 64 {
				s.truncated = true
				break
			}
			cleanKey := s.text(key)
			out[cleanKey] = s.value(child, depth+1)
		}
		return out
	case []any:
		out := make([]any, 0, min(len(v), 64))
		for _, child := range v {
			if s.remaining <= 0 || len(out) >= 64 {
				s.truncated = true
				break
			}
			out = append(out, s.value(child, depth+1))
		}
		return out
	default:
		return value // JSON scalars only; arbitrary objects are normalized below.
	}
}

func (s *normalizer) attrValue(value slog.Value) any {
	value = value.Resolve()
	if value.Kind() == slog.KindGroup {
		out := make(map[string]any)
		s.attrs(out, value.Group())
		return out
	}
	var rawValue any
	switch value.Kind() {
	case slog.KindString:
		return s.text(value.String())
	case slog.KindDuration:
		return value.Duration().Nanoseconds()
	case slog.KindTime:
		return s.text(value.Time().Format(time.RFC3339Nano))
	default:
		rawValue = value.Any()
	}
	if err, ok := rawValue.(error); ok {
		return s.text(err.Error())
	}
	raw, err := json.Marshal(rawValue)
	if err != nil {
		return "[unsupported value]"
	}
	var normalized any
	if err := json.Unmarshal(raw, &normalized); err != nil {
		return "[unsupported value]"
	}
	return s.value(normalized, 0)
}

func (s *normalizer) attrs(out map[string]any, attrs []slog.Attr) {
	if s.depth >= 8 {
		s.truncated = true
		return
	}
	s.depth++
	defer func() { s.depth-- }()
	for _, attr := range attrs {
		if s.remaining <= 0 || len(out) >= 64 {
			s.truncated = true
			return
		}
		if attr.Equal(slog.Attr{}) {
			continue
		}
		attr.Value = attr.Value.Resolve()
		if attr.Key == "" && attr.Value.Kind() == slog.KindGroup {
			s.attrs(out, attr.Value.Group())
			continue
		}
		key := s.text(attr.Key)
		if attr.Value.Kind() == slog.KindGroup && len(attr.Value.Group()) == 0 {
			continue
		}
		value := s.attrValue(attr.Value)
		if attr.Value.Kind() == slog.KindGroup {
			if group, ok := value.(map[string]any); ok && len(group) == 0 {
				continue
			}
		}
		out[key] = value
	}
}
