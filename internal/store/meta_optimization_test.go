package store

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type plainStore struct{ Store }

func TestPrefixListingContract(t *testing.T) {
	for _, backend := range []string{"filesystem", "s3", "custom"} {
		t.Run(backend, func(t *testing.T) {
			var st Store = NewFSStore(afero.NewMemMapFs())
			if backend == "s3" {
				st = newS3Store(newFakeS3Client(), "bucket", "ns/")
			}
			if backend == "custom" {
				st = plainStore{st}
			}
			st = Coordinate(st)
			ctx := t.Context()
			require.NoError(t, st.Write(ctx, "files/nested/a.json", []byte("{}")))
			entries, err := ListPrefix(ctx, st, "files/")
			require.NoError(t, err)
			require.Len(t, entries, 2)
			require.Equal(t, "files/nested", entries[0].Key)
			require.True(t, entries[0].IsDir)
			for _, prefix := range []string{"missing", "files/nested/a.json"} {
				entries, err = ListPrefix(ctx, st, prefix)
				require.NoError(t, err)
				require.Empty(t, entries)
			}
			_, err = ListPrefix(ctx, st, "../escape")
			require.ErrorIs(t, err, ErrInvalidKey)
		})
	}
}

func TestS3MetadataSummaryOperationCounts(t *testing.T) {
	client := newFakeS3Client()
	st := newS3Store(client, "bucket", "ns/")
	meta := NewMetaStore(Coordinate(st))
	ctx := t.Context()
	now := time.Unix(1700000000, 0)
	meta.summaries.now = func() time.Time { return now }
	for i := range 20 {
		require.NoError(t, meta.PutSubscription(ctx, domain.Subscription{Name: fmt.Sprintf("sub-%02d", i), DisplayName: "original", Meta: map[string]string{"value": "safe"}}))
	}
	first, err := meta.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.Len(t, first, 20)
	require.Equal(t, 20, client.getCalls)
	require.Equal(t, 1, client.listCalls)
	require.Zero(t, client.headCalls)
	first[0].Meta["value"] = "mutated"
	now = now.Add(9 * time.Second)
	second, err := meta.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.Equal(t, "safe", second[0].Meta["value"])
	require.Equal(t, 20, client.getCalls)
	require.Equal(t, 2, client.listCalls)
	// A different writer uses the same physical namespace.
	other := NewMetaStore(st)
	require.NoError(t, other.PutSubscription(ctx, domain.Subscription{Name: "sub-00", DisplayName: "external"}))
	second, err = meta.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.Equal(t, "external", second[0].DisplayName)
	require.Equal(t, 21, client.getCalls)
	// Hits did not slide the deadline of the other nineteen entries.
	now = now.Add(time.Second)
	_, err = meta.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.Equal(t, 40, client.getCalls)
	require.NoError(t, other.DeleteSubscription(ctx, "sub-01"))
	second, err = meta.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.Len(t, second, 19)
	_, exists := meta.summaries.items["subscriptions/sub-01.json"]
	require.False(t, exists)
	// Own writes invalidate even when the provider returns the same ETag.
	require.NoError(t, meta.PutSubscription(ctx, domain.Subscription{Name: "sub-00", DisplayName: "external"}))
	_, err = meta.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.Equal(t, 41, client.getCalls)
}

type versionTestClient struct {
	*fakeS3Client
	getVersion *string
	denyRead   bool
}

func (f *versionTestClient) GetObject(ctx context.Context, input *s3.GetObjectInput, options ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if f.denyRead {
		return nil, &smithy.GenericAPIError{Code: "AccessDenied"}
	}
	out, err := f.fakeS3Client.GetObject(ctx, input, options...)
	if err == nil {
		out.ETag = f.getVersion
	}
	return out, err
}

func TestSummaryVersionMismatchAndReadPermissionRecheck(t *testing.T) {
	for _, version := range []*string{nil, aws.String("different")} {
		client := &versionTestClient{fakeS3Client: newFakeS3Client(), getVersion: version}
		meta := NewMetaStore(newS3Store(client, "bucket", "ns/"))
		require.NoError(t, meta.PutSubscription(t.Context(), domain.Subscription{Name: "a"}))
		for range 2 {
			result, err := meta.ListSubscriptions(t.Context())
			require.NoError(t, err)
			require.Empty(t, result[0].Warning)
		}
		require.Equal(t, 2, client.getCalls)
		require.Empty(t, meta.summaries.items)
	}
	client := &versionTestClient{fakeS3Client: newFakeS3Client()}
	st := newS3Store(client, "bucket", "ns/")
	meta := NewMetaStore(st)
	now := time.Now()
	meta.summaries.now = func() time.Time { return now }
	require.NoError(t, meta.PutSubscription(t.Context(), domain.Subscription{Name: "a"}))
	// Get the actual opaque token, then make LIST and GET agree.
	listed, err := ListPrefix(t.Context(), st, "subscriptions")
	require.NoError(t, err)
	client.getVersion = aws.String(listed[0].Version)
	_, err = meta.ListSubscriptions(t.Context())
	require.NoError(t, err)
	client.denyRead = true
	now = now.Add(summaryRecheckInterval)
	result, err := meta.ListSubscriptions(t.Context())
	require.NoError(t, err)
	require.Contains(t, result[0].Warning, "AccessDenied")
	require.Empty(t, meta.summaries.items)
	client.denyRead = false
	result, err = meta.ListSubscriptions(t.Context())
	require.NoError(t, err)
	require.Empty(t, result[0].Warning)
}

type countReadStore struct {
	Store
	reads int
}

func (s *countReadStore) Read(ctx context.Context, key string) ([]byte, error) {
	s.reads++
	return s.Store.Read(ctx, key)
}

func TestFilesystemSummaryDetectsSameSizeAndTimeAndInvalidJSON(t *testing.T) {
	disk := afero.NewMemMapFs()
	st := &countReadStore{Store: NewFSStore(disk)}
	meta := NewMetaStore(Coordinate(st))
	ctx := t.Context()
	for _, name := range []string{"old", "new"} {
		require.NoError(t, st.Write(ctx, "subscriptions/a.json", []byte(`{"name":"a","display_name":"`+name+`"}`)))
		require.NoError(t, disk.Chtimes("subscriptions/a.json", time.Unix(1, 0), time.Unix(1, 0)))
		result, err := meta.ListSubscriptions(ctx)
		require.NoError(t, err)
		require.Equal(t, name, result[0].DisplayName)
	}
	require.Equal(t, 2, st.reads)
	require.NoError(t, st.Write(ctx, "subscriptions/a.json", []byte(`{"bad"`)))
	result, err := meta.ListSubscriptions(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, result[0].Warning)
	require.Empty(t, meta.summaries.items)
	require.NoError(t, st.Write(ctx, "shares/a.json", []byte(`{"bad"`)))
	_, err = meta.ListShares(ctx)
	require.Error(t, err)
}

func TestSummaryCacheBoundsAndInvalidationBarrier(t *testing.T) {
	meta := NewMetaStore(NewFSStore(afero.NewMemMapFs()))
	ctx := t.Context()
	require.NoError(t, meta.store.Write(ctx, "subscriptions/a.json", []byte(`{}`)))
	entry := ListedEntry{Entry: Entry{Key: "subscriptions/a.json"}}
	_, err := meta.readSummary(ctx, entry, meta.summaries.currentGeneration(), func([]byte) (string, error) { meta.InvalidateLists(); return "old", nil })
	require.NoError(t, err)
	require.Empty(t, meta.summaries.items)
	c := &meta.summaries
	generation := c.currentGeneration()
	for i := range summaryCacheEntries + 1 {
		c.put(fmt.Sprint(i), cachedSummary{value: []byte("small")}, generation)
	}
	require.Len(t, c.items, summaryCacheEntries)
	_, exists := c.items["0"]
	require.False(t, exists)
	c.put("large", cachedSummary{value: []byte(strings.Repeat("x", summaryCacheBytes))}, generation)
	_, exists = c.items["large"]
	require.False(t, exists)
	for i := range 20 {
		c.put(fmt.Sprint(i), cachedSummary{value: []byte(strings.Repeat("x", 1<<20))}, generation)
	}
	require.LessOrEqual(t, c.bytes, summaryCacheBytes)
}

type parallelReadStore struct {
	Store
	concurrent bool
}

func (s parallelReadStore) ConcurrentReads() bool { return s.concurrent }
func TestMetadataReadConcurrencyOrderAndCancellation(t *testing.T) {
	for _, parallel := range []bool{false, true} {
		st := Coordinate(parallelReadStore{Store: NewFSStore(afero.NewMemMapFs()), concurrent: parallel})
		entries := make([]ListedEntry, 20)
		for i := range entries {
			entries[i].Key = fmt.Sprint(i)
		}
		release := make(chan struct{})
		var active, maximum atomic.Int32
		values, errs := mapMetadata(t.Context(), st, entries, func(e ListedEntry) (string, error) {
			n := active.Add(1)
			for old := maximum.Load(); n > old; old = maximum.Load() {
				if maximum.CompareAndSwap(old, n) {
					break
				}
			}
			if parallel {
				if n == 4 {
					select {
					case <-release:
					default:
						close(release)
					}
				}
				<-release
			}
			active.Add(-1)
			return e.Key, fmt.Errorf("error %s", e.Key)
		})
		for i := range entries {
			require.Equal(t, entries[i].Key, values[i])
			require.EqualError(t, errs[i], "error "+entries[i].Key)
		}
		if parallel {
			require.EqualValues(t, 4, maximum.Load())
		} else {
			require.EqualValues(t, 1, maximum.Load())
		}
		ctx, cancel := context.WithCancel(t.Context())
		var started atomic.Int32
		_, errs = mapMetadata(ctx, st, entries, func(e ListedEntry) (string, error) { started.Add(1); cancel(); return "", ctx.Err() })
		require.LessOrEqual(t, started.Load(), int32(4))
		for _, err := range errs {
			require.ErrorIs(t, err, context.Canceled)
		}
	}
}

type badListStore struct {
	Store
	entries []Entry
}

func (s badListStore) List(context.Context, string) ([]Entry, error) { return s.entries, nil }
func TestPrefixFallbackRejectsUnsafeAndDuplicateEntries(t *testing.T) {
	for _, entries := range [][]Entry{{{Key: "other/a"}}, {{Key: "files/../a"}}, {{Key: "files/a"}, {Key: "files/a"}}} {
		_, err := ListPrefix(t.Context(), badListStore{entries: entries}, "files")
		require.ErrorIs(t, err, ErrInvalidKey)
	}
	err := NewFSStore(afero.NewMemMapFs()).Delete(t.Context(), "missing")
	require.ErrorIs(t, err, fs.ErrNotExist)
}
