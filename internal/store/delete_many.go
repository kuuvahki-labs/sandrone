package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type manyDeleter interface {
	DeleteMany(context.Context, []string) error
}
type s3DeleteAPI interface {
	DeleteObjects(context.Context, *s3.DeleteObjectsInput, ...func(*s3.Options)) (*s3.DeleteObjectsOutput, error)
}

// DeleteMany deletes validated file keys idempotently. It may partially succeed;
// errors other than missing files remain errors. Keys must identify files;
// callers obtain them from ListPrefix and filter IsDir before calling.
func DeleteMany(ctx context.Context, st Store, keys []string) error {
	for _, key := range keys {
		if _, err := CleanKey(key); err != nil {
			return err
		}
	}
	if len(keys) == 0 {
		return nil
	}
	if capable, ok := st.(manyDeleter); ok {
		return capable.DeleteMany(ctx, keys)
	}
	return deleteFiles(ctx, st, keys)
}

func deleteFiles(ctx context.Context, st Store, keys []string) error {
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := st.Delete(ctx, key); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

func (s *coordinatedStore) DeleteMany(ctx context.Context, keys []string) error {
	return s.Update(ctx, func(st Store) error { return DeleteMany(ctx, st, keys) })
}

func (s *S3Store) DeleteMany(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if _, err := CleanKey(key); err != nil {
			return err
		}
	}
	client, ok := s.client.(s3DeleteAPI)
	if !ok {
		return deleteFiles(ctx, s, keys)
	}
	for batch := range slices.Chunk(keys, 1000) {
		objects := make([]types.ObjectIdentifier, 0, len(batch))
		for _, key := range batch {
			objects = append(objects, types.ObjectIdentifier{Key: aws.String(s.objectKey(key))})
		}
		out, err := client.DeleteObjects(ctx, &s3.DeleteObjectsInput{Bucket: aws.String(s.bucket), Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)}})
		if err != nil {
			if apiErr, ok := errors.AsType[smithy.APIError](err); ok && (apiErr.ErrorCode() == "NotImplemented" || apiErr.ErrorCode() == "MethodNotAllowed") {
				if err := deleteFiles(ctx, s, batch); err != nil {
					return err
				}
				continue
			}
			return s.operationError("delete many", "", err)
		}
		if out == nil {
			return errors.New("s3 delete many: missing response")
		}
		for _, failure := range out.Errors {
			code := aws.ToString(failure.Code)
			if code != "NoSuchKey" && code != "NotFound" {
				return fmt.Errorf("s3 delete many: provider error %s", code)
			}
		}
	}
	return nil
}
