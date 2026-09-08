package store

import (
	"context"
	"fmt"
	"io/fs"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type bulkS3Client struct {
	*fakeS3Client
	batches  []int
	bulkErr  error
	failures []types.Error
}

func (f *bulkS3Client) DeleteObjects(ctx context.Context, input *s3.DeleteObjectsInput, _ ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error) {
	f.batches = append(f.batches, len(input.Delete.Objects))
	if f.bulkErr != nil {
		return nil, f.bulkErr
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, object := range input.Delete.Objects {
		delete(f.objects, aws.ToString(object.Key))
	}
	return &s3.DeleteObjectsOutput{Errors: f.failures}, nil
}

func TestS3DeleteManyBatchesAndStrictDelete(t *testing.T) {
	client := &bulkS3Client{fakeS3Client: newFakeS3Client()}
	st := Coordinate(newS3Store(client, "bucket", "ns/"))
	keys := make([]string, 2001)
	for i := range keys {
		keys[i] = fmt.Sprintf("cache/a/%04d.json", i)
		require.NoError(t, st.Write(t.Context(), keys[i], []byte("a")))
	}
	require.NoError(t, DeleteMany(t.Context(), st, keys))
	require.Equal(t, []int{1000, 1000, 1}, client.batches)
	require.Empty(t, client.objects)
	require.Zero(t, client.headCalls)
	require.Zero(t, client.deleteCalls)
	require.NoError(t, DeleteMany(t.Context(), st, []string{"cache/missing.json"}))
	require.ErrorIs(t, st.Delete(t.Context(), "cache/missing.json"), fs.ErrNotExist)
	require.Equal(t, 1, client.headCalls)
}

func TestS3DeleteManyErrorsAndUnsupportedFallback(t *testing.T) {
	for _, code := range []string{"AccessDenied", "InternalError", "NotImplemented", "MethodNotAllowed"} {
		t.Run(code, func(t *testing.T) {
			client := &bulkS3Client{fakeS3Client: newFakeS3Client(), bulkErr: &smithy.GenericAPIError{Code: code}}
			st := newS3Store(client, "bucket", "ns/")
			require.NoError(t, st.Write(t.Context(), "cache/a", []byte("a")))
			err := DeleteMany(t.Context(), Coordinate(st), []string{"cache/a", "cache/missing"})
			if code == "NotImplemented" || code == "MethodNotAllowed" {
				require.NoError(t, err)
				require.Empty(t, client.objects)
				require.Equal(t, 2, client.headCalls)
			} else {
				require.ErrorContains(t, err, code)
				require.Len(t, client.objects, 1)
				require.Zero(t, client.headCalls)
			}
		})
	}
	for _, code := range []string{"AccessDenied", "NoSuchKey", ""} {
		client := &bulkS3Client{fakeS3Client: newFakeS3Client(), failures: []types.Error{{Code: aws.String(code)}}}
		err := DeleteMany(t.Context(), newS3Store(client, "bucket", "ns/"), []string{"cache/a"})
		if code == "NoSuchKey" {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
		require.Zero(t, client.deleteCalls)
	}
}

type deletionFailureStore struct{ Store }

func (s deletionFailureStore) Delete(context.Context, string) error { return fs.ErrPermission }
func TestDeleteManyFallbackAndValidation(t *testing.T) {
	for _, st := range []Store{NewFSStore(afero.NewMemMapFs()), plainStore{NewFSStore(afero.NewMemMapFs())}, newS3Store(newFakeS3Client(), "bucket", "ns/")} {
		st = Coordinate(st)
		require.NoError(t, st.Write(t.Context(), "cache/a", []byte("a")))
		require.ErrorIs(t, DeleteMany(t.Context(), st, []string{"cache/a", "../unsafe"}), ErrInvalidKey)
		_, err := st.Read(t.Context(), "cache/a")
		require.NoError(t, err)
		require.NoError(t, DeleteMany(t.Context(), st, []string{"cache/a", "cache/missing"}))
		_, err = st.Read(t.Context(), "cache/a")
		require.ErrorIs(t, err, fs.ErrNotExist)
	}
	require.ErrorIs(t, DeleteMany(t.Context(), Coordinate(deletionFailureStore{}), []string{"cache/a"}), fs.ErrPermission)
}
