package store

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/require"
)

type fakeS3Object struct {
	body    []byte
	modTime time.Time
}

type fakeS3Client struct {
	objects      map[string]fakeS3Object
	pageSize     int
	headCalls    int
	putCalls     int
	deleteCalls  int
	listCalls    int
	operationErr error
}

func newFakeS3Client() *fakeS3Client {
	return &fakeS3Client{objects: map[string]fakeS3Object{}}
}

func (f *fakeS3Client) GetObject(ctx context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.operationErr != nil {
		return nil, f.operationErr
	}
	object, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, noSuchKey()
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(string(object.body)))}, nil
}

func (f *fakeS3Client) PutObject(ctx context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.operationErr != nil {
		return nil, f.operationErr
	}
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	f.putCalls++
	f.objects[aws.ToString(in.Key)] = fakeS3Object{body: body, modTime: time.Unix(1_700_000_000, 0).UTC()}
	return &s3.PutObjectOutput{}, nil
}

func (f *fakeS3Client) DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.operationErr != nil {
		return nil, f.operationErr
	}
	f.deleteCalls++
	delete(f.objects, aws.ToString(in.Key))
	return &s3.DeleteObjectOutput{}, nil
}

func (f *fakeS3Client) HeadObject(ctx context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.operationErr != nil {
		return nil, f.operationErr
	}
	f.headCalls++
	object, ok := f.objects[aws.ToString(in.Key)]
	if !ok {
		return nil, noSuchKey()
	}
	return &s3.HeadObjectOutput{ContentLength: aws.Int64(int64(len(object.body))), LastModified: aws.Time(object.modTime)}, nil
}

func (f *fakeS3Client) ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if f.operationErr != nil {
		return nil, f.operationErr
	}
	f.listCalls++
	keys := make([]string, 0, len(f.objects))
	for key := range f.objects {
		if strings.HasPrefix(key, aws.ToString(in.Prefix)) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	start := 0
	if token := aws.ToString(in.ContinuationToken); token != "" {
		value, err := strconv.Atoi(token)
		if err != nil {
			return nil, err
		}
		start = value
	}
	limit := len(keys)
	if f.pageSize > 0 && start+f.pageSize < limit {
		limit = start + f.pageSize
	}
	if max := int(aws.ToInt32(in.MaxKeys)); max > 0 && start+max < limit {
		limit = start + max
	}
	out := &s3.ListObjectsV2Output{}
	for _, key := range keys[start:limit] {
		object := f.objects[key]
		out.Contents = append(out.Contents, types.Object{
			Key:          aws.String(key),
			Size:         aws.Int64(int64(len(object.body))),
			LastModified: aws.Time(object.modTime),
		})
	}
	if limit < len(keys) {
		out.IsTruncated = aws.Bool(true)
		out.NextContinuationToken = aws.String(strconv.Itoa(limit))
	}
	return out, nil
}

func noSuchKey() error {
	return &smithy.GenericAPIError{Code: "NoSuchKey", Message: "not found", Fault: smithy.FaultClient}
}

func TestS3StoreReadWriteOverwriteAndDelete(t *testing.T) {
	client := newFakeS3Client()
	storage := newS3Store(client, "bucket", "namespace/")
	ctx := context.Background()

	require.NoError(t, storage.Write(ctx, "subscriptions/demo.json", []byte("first")))
	require.NoError(t, storage.Write(ctx, "subscriptions/demo.json", []byte("second")))
	body, err := storage.Read(ctx, "subscriptions/demo.json")
	require.NoError(t, err)
	require.Equal(t, []byte("second"), body)
	require.NoError(t, storage.Delete(ctx, "subscriptions/demo.json"))
	_, err = storage.Read(ctx, "subscriptions/demo.json")
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.Equal(t, 1, client.deleteCalls)
}

func TestS3StoreDeleteChecksExistence(t *testing.T) {
	client := newFakeS3Client()
	storage := newS3Store(client, "bucket", "namespace/")

	err := storage.Delete(context.Background(), "missing.json")
	require.ErrorIs(t, err, fs.ErrNotExist)
	require.Equal(t, 1, client.headCalls)
	require.Zero(t, client.deleteCalls)
}

func TestS3StoreWriteAtomicUsesSinglePut(t *testing.T) {
	client := newFakeS3Client()
	storage := newS3Store(client, "bucket", "namespace/")

	require.NoError(t, storage.WriteAtomic(context.Background(), "settings.json", []byte("{}"), 0o600))
	require.Equal(t, 1, client.putCalls)
	require.Len(t, client.objects, 1)
}

func TestS3StoreListRootAcrossPages(t *testing.T) {
	client := newFakeS3Client()
	client.pageSize = 1
	storage := newS3Store(client, "bucket", "namespace/")
	for key, body := range map[string]string{
		"namespace/settings.json":                 "{}",
		"namespace/subscriptions/demo.json":       "one",
		"namespace/cache/probe/result.json":       "two",
		"namespace/cache/remote_fetch/value.json": "three",
	} {
		client.objects[key] = fakeS3Object{body: []byte(body)}
	}

	entries, err := storage.List(context.Background(), "")
	require.NoError(t, err)
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	require.Equal(t, []string{
		"cache",
		"cache/probe",
		"cache/probe/result.json",
		"cache/remote_fetch",
		"cache/remote_fetch/value.json",
		"settings.json",
		"subscriptions",
		"subscriptions/demo.json",
	}, keys)
}

func TestS3StoreListPrefixAndVirtualDirectories(t *testing.T) {
	client := newFakeS3Client()
	storage := newS3Store(client, "bucket", "namespace/")
	client.objects["namespace/files/nested/demo.json"] = fakeS3Object{body: []byte("body")}

	entries, err := storage.List(context.Background(), "files")
	require.NoError(t, err)
	require.Equal(t, []Entry{
		{Key: "files/nested", IsDir: true},
		{Key: "files/nested/demo.json", Size: 4},
	}, entries)
	entry, err := storage.Stat(context.Background(), "files/nested")
	require.NoError(t, err)
	require.True(t, entry.IsDir)
}

func TestS3StoreListRejectsUnsafeLogicalKeys(t *testing.T) {
	client := newFakeS3Client()
	storage := newS3Store(client, "bucket", "namespace/")
	client.objects["namespace/../escaped"] = fakeS3Object{body: []byte("body")}

	_, err := storage.List(context.Background(), "")
	require.ErrorContains(t, err, "invalid logical key")
}

func TestS3StoreStatObject(t *testing.T) {
	client := newFakeS3Client()
	storage := newS3Store(client, "bucket", "namespace/")
	client.objects["namespace/settings.json"] = fakeS3Object{body: []byte("{}"), modTime: time.Unix(10, 0)}

	entry, err := storage.Stat(context.Background(), "settings.json")
	require.NoError(t, err)
	require.Equal(t, int64(2), entry.Size)
	require.Equal(t, time.Unix(10, 0), entry.ModTime)
}

func TestS3StoreStatVirtualDirectoryStopsAfterFirstPage(t *testing.T) {
	client := newFakeS3Client()
	client.pageSize = 1
	storage := newS3Store(client, "bucket", "namespace/")
	client.objects["namespace/files/nested/one.json"] = fakeS3Object{body: []byte("one")}
	client.objects["namespace/files/nested/two.json"] = fakeS3Object{body: []byte("two")}

	entry, err := storage.Stat(context.Background(), "files/nested")
	require.NoError(t, err)
	require.True(t, entry.IsDir)
	require.Equal(t, 1, client.listCalls)
}

func TestS3StorePropagatesContextCancellation(t *testing.T) {
	storage := newS3Store(newFakeS3Client(), "bucket", "namespace/")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := storage.Write(ctx, "settings.json", []byte("{}"))
	require.ErrorIs(t, err, context.Canceled)
}

func TestS3StoreErrorsDoNotLeakConfiguredSecretsOrBodies(t *testing.T) {
	client := newFakeS3Client()
	client.operationErr = fmt.Errorf("provider unavailable at https://account-marker.invalid?token=secret-marker")
	storage := newS3Store(client, "bucket", "namespace/")

	err := storage.Write(context.Background(), "settings.json", []byte("private-body-marker"))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "private-body-marker")
	require.NotContains(t, err.Error(), "namespace/")
	require.NotContains(t, err.Error(), "account-marker")
	require.NotContains(t, err.Error(), "secret-marker")
}
