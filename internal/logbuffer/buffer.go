// Package logbuffer captures bounded copies of runtime logs.
package logbuffer

import (
	"bytes"
	"encoding/json/v2"
	"sync"
	"time"
	"uuid"
)

const (
	MaxEntries    = 1000
	MaxBytes      = 2 * 1024 * 1024
	MaxEntryBytes = 8 * 1024
)

type Entry struct {
	ID        uint64         `json:"id"`
	Time      time.Time      `json:"time"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Attrs     map[string]any `json:"attrs"`
	Truncated bool           `json:"truncated"`
}

type Snapshot struct {
	InstanceID    string    `json:"instance_id"`
	SnapshotTime  time.Time `json:"snapshot_time"`
	Level         string    `json:"level"`
	MaxEntries    int       `json:"max_entries"`
	MaxBytes      int       `json:"max_bytes"`
	MaxEntryBytes int       `json:"max_entry_bytes"`
	Dropped       uint64    `json:"dropped"`
	Entries       []Entry   `json:"entries"`
}

// Buffer keeps serialized records only; callers cannot mutate retained entries.
type Buffer struct {
	mu                sync.Mutex
	instanceID        string
	level             string
	records           [MaxEntries][]byte
	head, count, size int
	sequence, dropped uint64
}

func New(level string) *Buffer {
	return &Buffer{instanceID: uuid.New().String(), level: level}
}

func (b *Buffer) append(entry Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sequence++
	entry.ID = b.sequence
	raw := encodeEntry(entry)
	for b.count > 0 && (b.count == MaxEntries || b.size+len(raw) > MaxBytes) {
		b.size -= len(b.records[b.head])
		b.records[b.head] = nil
		b.head = (b.head + 1) % MaxEntries
		b.count--
		b.dropped++
	}
	b.records[(b.head+b.count)%MaxEntries] = raw
	b.count++
	b.size += len(raw)
}

func encodeEntry(entry Entry) []byte {
	for {
		raw, err := json.Marshal(entry)
		if err == nil && len(raw) <= MaxEntryBytes {
			return raw
		}
		entry.Truncated = true
		if err != nil {
			// A custom slog.Record can contain a timestamp outside JSON's time range.
			entry.Time = time.Time{}
		}
		if len(entry.Attrs) > 0 {
			// Preserve the message even when structured context alone exceeds the budget.
			entry.Attrs = map[string]any{}
		} else {
			entry.Message = prefix(entry.Message, len(entry.Message)/2)
		}
	}
}

// Snapshot returns newest records first, detached from the live buffer.
func (b *Buffer) Snapshot() Snapshot {
	b.mu.Lock()
	snapshot := Snapshot{
		InstanceID: b.instanceID, SnapshotTime: time.Now().UTC(), Level: b.level,
		MaxEntries: MaxEntries, MaxBytes: MaxBytes, MaxEntryBytes: MaxEntryBytes,
		Dropped: b.dropped, Entries: make([]Entry, b.count),
	}
	records := make([][]byte, b.count)
	for i := range b.count {
		records[i] = bytes.Clone(b.records[(b.head+b.count-1-i)%MaxEntries])
	}
	b.mu.Unlock()
	for i, raw := range records {
		_ = json.Unmarshal(raw, &snapshot.Entries[i])
	}
	return snapshot
}
