package logbuffer

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"testing/slogtest"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestHandlerContract(t *testing.T) {
	var buffer *Buffer
	slogtest.Run(t, func(t *testing.T) slog.Handler {
		buffer = New("info")
		return buffer.Handler(slog.NewJSONHandler(io.Discard, nil))
	}, func(t *testing.T) map[string]any {
		entry := buffer.Snapshot().Entries[0]
		result := entry.Attrs
		result[slog.MessageKey] = entry.Message
		result[slog.LevelKey] = entry.Level
		if !entry.Time.IsZero() {
			result[slog.TimeKey] = entry.Time
		}
		return result
	})
}

func TestHandlerPreservesOutputAndSnapshot(t *testing.T) {
	var output bytes.Buffer
	buffer := New("info")
	logger := slog.New(buffer.Handler(slog.NewJSONHandler(&output, nil)))
	contextFields := map[string]any{"password": "example-secret", "url": "https://example.com/feed?token=example"}
	derived := logger.With("context", contextFields).WithGroup("operation").With("kind", "fetch")
	contextFields["password"] = "changed"
	derived.Debug("not enabled")
	derived.Error("fetch failed https://example.com/feed?token=example", "error", errors.New("failure"), "duration_ms", 12)
	snapshot := buffer.Snapshot()
	require.Len(t, snapshot.Entries, 1)
	entry := snapshot.Entries[0]
	require.Equal(t, "error", entry.Level)
	require.Contains(t, entry.Message, "token=example")
	require.Equal(t, "example-secret", entry.Attrs["context"].(map[string]any)["password"])
	require.Equal(t, map[string]any{"kind": "fetch", "error": "failure", "duration_ms": float64(12)}, entry.Attrs["operation"])
	require.Contains(t, output.String(), "example-secret")
	require.NotContains(t, output.String(), "not enabled")
	entry.Attrs["operation"].(map[string]any)["kind"] = "mutated"
	require.Equal(t, "fetch", buffer.Snapshot().Entries[0].Attrs["operation"].(map[string]any)["kind"])
	logger.InfoContext(WithoutCapture(t.Context()), "output only")
	require.Len(t, buffer.Snapshot().Entries, 1)
	require.Contains(t, output.String(), "output only")
	logger.Info("parent")
	require.Empty(t, buffer.Snapshot().Entries[0].Attrs)
}

func TestCapacityAndTruncation(t *testing.T) {
	for _, test := range []struct {
		name, message string
		count         int
	}{
		{"count", "small", 1005},
		{"bytes", strings.Repeat("x", 4000), 1000},
		{"entry", strings.Repeat("日志\n", 10000), 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			buffer := New("info")
			logger := slog.New(buffer.Handler(slog.NewTextHandler(io.Discard, nil)))
			for range test.count {
				logger.Info(test.message)
			}
			snapshot := buffer.Snapshot()
			require.NotEmpty(t, snapshot.InstanceID)
			require.Equal(t, uint64(test.count), snapshot.Entries[0].ID)
			require.Equal(t, uint64(test.count-len(snapshot.Entries)), snapshot.Dropped)
			total := 0
			for i, entry := range snapshot.Entries {
				raw, err := json.Marshal(entry)
				require.NoError(t, err)
				require.LessOrEqual(t, len(raw), MaxEntryBytes)
				require.True(t, utf8.ValidString(entry.Message))
				total += len(raw)
				if i > 0 {
					require.Equal(t, snapshot.Entries[i-1].ID-1, entry.ID)
				}
			}
			require.LessOrEqual(t, total, MaxBytes)
			require.LessOrEqual(t, len(snapshot.Entries), MaxEntries)
			if test.name == "entry" {
				require.True(t, snapshot.Entries[0].Truncated)
			} else {
				require.Positive(t, snapshot.Dropped)
			}
		})
	}
}

func TestLargeStructuredContextIsBounded(t *testing.T) {
	buffer := New("info")
	logger := slog.New(buffer.Handler(slog.NewTextHandler(io.Discard, nil)))
	for range 20 {
		logger = logger.With("large", strings.Repeat("z", MaxEntryBytes))
	}
	logger.Info("message remains", "nested", map[string]any{"data": strings.Repeat("a", MaxBytes)})
	snapshot := buffer.Snapshot()
	require.Len(t, snapshot.Entries, 1)
	require.True(t, snapshot.Entries[0].Truncated)
	require.Equal(t, "message remains", snapshot.Entries[0].Message)
	raw, err := json.Marshal(snapshot.Entries[0])
	require.NoError(t, err)
	require.LessOrEqual(t, len(raw), MaxEntryBytes)
}

func TestConcurrentSnapshotsAndIndependentBuffers(t *testing.T) {
	buffer := New("info")
	logger := slog.New(buffer.Handler(slog.NewTextHandler(io.Discard, nil)))
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 200 {
				logger.Info("concurrent")
				snapshot := buffer.Snapshot()
				for i := 1; i < len(snapshot.Entries); i++ {
					if snapshot.Entries[i-1].ID != snapshot.Entries[i].ID+1 {
						t.Error("non-contiguous snapshot")
					}
				}
			}
		})
	}
	wg.Wait()
	snapshot := buffer.Snapshot()
	require.Equal(t, uint64(1600), snapshot.Entries[0].ID)
	fresh := New("warn").Snapshot()
	require.NotEqual(t, snapshot.InstanceID, fresh.InstanceID)
	require.Empty(t, fresh.Entries)
}

func TestUnusualRecordsRemainBounded(t *testing.T) {
	buffer := New("info")
	handler := buffer.Handler(slog.NewJSONHandler(io.Discard, nil))
	record := slog.NewRecord(time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), slog.LevelInfo, "invalid byte: \xff", 0)
	// The original handler may reject this timestamp; the web copy still must not block.
	_ = handler.Handle(t.Context(), record)
	snapshot := buffer.Snapshot()
	require.Len(t, snapshot.Entries, 1)
	require.True(t, snapshot.Entries[0].Truncated)
	require.True(t, utf8.ValidString(snapshot.Entries[0].Message))
	logger := slog.New(handler)
	logger.Info(strings.Repeat("x", 6000))
	require.False(t, buffer.Snapshot().Entries[0].Truncated)
	require.Len(t, buffer.Snapshot().Entries[0].Message, 6000)
}
