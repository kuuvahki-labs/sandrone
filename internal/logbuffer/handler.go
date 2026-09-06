package logbuffer

import (
	"context"
	"encoding/json/v2"
	"log/slog"
	"slices"
	"strings"
)

type skipKey struct{}

// WithoutCapture excludes this record from the web copy, not the output handler.
func WithoutCapture(ctx context.Context) context.Context {
	return context.WithValue(ctx, skipKey{}, true)
}

type handler struct {
	next      slog.Handler
	buffer    *Buffer
	attrs     map[string]any
	groups    []string
	truncated bool
}

func (b *Buffer) Handler(next slog.Handler) slog.Handler {
	return &handler{next: next, buffer: b, attrs: map[string]any{}}
}

func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *handler) Handle(ctx context.Context, record slog.Record) error {
	if skip, _ := ctx.Value(skipKey{}).(bool); !skip {
		s := normalizer{remaining: MaxEntryBytes, truncated: h.truncated}
		fields := h.fields()
		message := s.text(record.Message)
		target := groupTarget(fields, h.groups)
		record.Attrs(func(attr slog.Attr) bool { s.attrs(target, []slog.Attr{attr}); return true })
		pruneGroups(fields, h.groups)
		h.buffer.append(Entry{Time: record.Time.UTC(), Level: strings.ToLower(record.Level.String()),
			Message: message, Attrs: fields, Truncated: s.truncated})
	}
	return h.next.Handle(ctx, record)
}

func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := *h
	clone.next = h.next.WithAttrs(attrs)
	clone.attrs = h.fields()
	s := normalizer{remaining: MaxEntryBytes, truncated: h.truncated}
	s.attrs(groupTarget(clone.attrs, h.groups), attrs)
	// Bound accumulated context too, including repeated Logger.With calls.
	raw := encodeEntry(Entry{Attrs: clone.attrs})
	var bounded Entry
	_ = json.Unmarshal(raw, &bounded)
	clone.attrs = bounded.Attrs
	clone.truncated = s.truncated || bounded.Truncated
	return &clone
}

func (h *handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	clone := *h
	clone.next = h.next.WithGroup(name)
	if len(h.groups) >= 8 {
		clone.truncated = true
		return &clone
	}
	s := normalizer{remaining: 256, truncated: h.truncated}
	clone.groups = append(slices.Clone(h.groups), s.text(name))
	clone.truncated = s.truncated
	return &clone
}

func (h *handler) fields() map[string]any {
	raw, _ := json.Marshal(h.attrs)
	fields := map[string]any{}
	_ = json.Unmarshal(raw, &fields)
	return fields
}

func groupTarget(fields map[string]any, groups []string) map[string]any {
	for _, group := range groups {
		key := prefix(group, 256)
		nested, ok := fields[key].(map[string]any)
		if !ok {
			nested = map[string]any{}
			fields[key] = nested
		}
		fields = nested
	}
	return fields
}

func pruneGroups(fields map[string]any, groups []string) {
	if len(groups) == 0 {
		return
	}
	key := groups[0]
	nested, ok := fields[key].(map[string]any)
	if !ok {
		return
	}
	pruneGroups(nested, groups[1:])
	if len(nested) == 0 {
		delete(fields, key)
	}
}
