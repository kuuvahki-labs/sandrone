package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type s3API interface {
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	HeadObject(context.Context, *s3.HeadObjectInput, ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

type S3Store struct {
	client s3API
	bucket string
	prefix string
}

func NewS3Store(_ context.Context, cfg S3Config) (*S3Store, error) {
	cfg, err := NormalizeS3Config(cfg)
	if err != nil {
		return nil, err
	}
	awsCfg := aws.Config{
		Region:                     cfg.Region,
		Credentials:                credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		HTTPClient:                 awshttp.NewBuildableClient(),
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(cfg.Endpoint)
		options.UsePathStyle = cfg.ForcePathStyle
	})
	return newS3Store(client, cfg.Bucket, cfg.Prefix), nil
}

func newS3Store(client s3API, bucket, prefix string) *S3Store {
	return &S3Store{client: client, bucket: bucket, prefix: prefix}
}

func (s *S3Store) Read(ctx context.Context, key string) ([]byte, error) {
	key, err := CleanKey(key)
	if err != nil {
		return nil, err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.objectKey(key))})
	if err != nil {
		return nil, s.operationError("read", key, err)
	}
	defer func() { _ = out.Body.Close() }()
	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, s.operationError("read", key, err)
	}
	return body, nil
}

func (s *S3Store) Write(ctx context.Context, key string, value []byte) error {
	key, err := CleanKey(key)
	if err != nil {
		return err
	}
	length := int64(len(value))
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(s.objectKey(key)),
		Body:          bytes.NewReader(value),
		ContentLength: aws.Int64(length),
	})
	if err != nil {
		return s.operationError("write", key, err)
	}
	return nil
}

func (s *S3Store) WriteAtomic(ctx context.Context, key string, value []byte, _ fs.FileMode) error {
	return s.Write(ctx, key, value)
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	key, err := CleanKey(key)
	if err != nil {
		return err
	}
	if _, err := s.head(ctx, key); err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.objectKey(key))})
	if err != nil {
		return s.operationError("delete", key, err)
	}
	return nil
}

func (s *S3Store) List(ctx context.Context, prefix string) ([]Entry, error) {
	prefix, err := cleanPrefix(prefix)
	if err != nil {
		return nil, err
	}
	if prefix != "" {
		if entry, err := s.head(ctx, prefix); err == nil {
			return []Entry{entry}, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}

	physicalPrefix := s.prefix
	if prefix != "" {
		physicalPrefix += prefix + "/"
	}
	objects, err := s.listObjects(ctx, physicalPrefix, 0)
	if err != nil {
		return nil, s.operationError("list", prefix, err)
	}
	if len(objects) == 0 {
		if prefix == "" {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("s3 list %s: %w", prefix, os.ErrNotExist)
	}

	entries := make(map[string]Entry, len(objects))
	for _, object := range objects {
		physical := aws.ToString(object.Key)
		if !strings.HasPrefix(physical, s.prefix) {
			return nil, fmt.Errorf("s3 list %s: object escaped configured prefix", prefix)
		}
		logical := strings.TrimPrefix(physical, s.prefix)
		clean, err := CleanKey(logical)
		if err != nil || clean != logical {
			return nil, fmt.Errorf("s3 list %s: invalid logical key", prefix)
		}
		if _, exists := entries[logical]; exists {
			return nil, fmt.Errorf("s3 list %s: duplicate logical key %s", prefix, logical)
		}
		entries[logical] = Entry{
			Key:     logical,
			Size:    aws.ToInt64(object.Size),
			ModTime: aws.ToTime(object.LastModified),
		}
	}

	objectKeys := make([]string, 0, len(entries))
	for key := range entries {
		objectKeys = append(objectKeys, key)
	}
	for _, key := range objectKeys {
		for dir := path.Dir(key); dir != "." && dir != prefix; dir = path.Dir(dir) {
			if existing, ok := entries[dir]; ok {
				if !existing.IsDir {
					return nil, fmt.Errorf("s3 list %s: key %s is both object and directory", prefix, dir)
				}
				continue
			}
			entries[dir] = Entry{Key: dir, IsDir: true}
		}
	}

	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}

func (s *S3Store) Stat(ctx context.Context, key string) (Entry, error) {
	key, err := CleanKey(key)
	if err != nil {
		return Entry{}, err
	}
	entry, err := s.head(ctx, key)
	if err == nil {
		return entry, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Entry{}, err
	}
	objects, listErr := s.listObjects(ctx, s.objectKey(key)+"/", 1)
	if listErr != nil {
		return Entry{}, s.operationError("stat", key, listErr)
	}
	if len(objects) == 0 {
		return Entry{}, fmt.Errorf("s3 stat %s: %w", key, os.ErrNotExist)
	}
	return Entry{Key: key, IsDir: true}, nil
}

func (s *S3Store) head(ctx context.Context, key string) (Entry, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(s.objectKey(key))})
	if err != nil {
		return Entry{}, s.operationError("stat", key, err)
	}
	return Entry{Key: key, Size: aws.ToInt64(out.ContentLength), ModTime: aws.ToTime(out.LastModified)}, nil
}

func (s *S3Store) listObjects(ctx context.Context, prefix string, maxKeys int32) ([]s3typesObject, error) {
	input := &s3.ListObjectsV2Input{Bucket: aws.String(s.bucket), Prefix: aws.String(prefix)}
	if maxKeys > 0 {
		input.MaxKeys = aws.Int32(maxKeys)
	}
	var objects []s3typesObject
	for {
		out, err := s.client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, err
		}
		for _, object := range out.Contents {
			objects = append(objects, s3typesObject{Key: object.Key, Size: object.Size, LastModified: object.LastModified})
		}
		if maxKeys > 0 {
			break
		}
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		if aws.ToString(out.NextContinuationToken) == "" {
			return nil, errors.New("truncated S3 listing omitted continuation token")
		}
		input.ContinuationToken = out.NextContinuationToken
	}
	return objects, nil
}

type s3typesObject struct {
	Key          *string
	Size         *int64
	LastModified *time.Time
}

func (s *S3Store) objectKey(key string) string {
	return s.prefix + key
}

func (s *S3Store) operationError(operation, key string, err error) error {
	if isS3NotFound(err) {
		return fmt.Errorf("s3 %s %s: %w", operation, key, os.ErrNotExist)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("s3 %s %s: %w", operation, key, err)
	}
	if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
		return fmt.Errorf("s3 %s %s: provider error %s", operation, key, apiErr.ErrorCode())
	}
	return fmt.Errorf("s3 %s %s: provider request failed", operation, key)
}

func isS3NotFound(err error) bool {
	apiErr, ok := errors.AsType[smithy.APIError](err)
	if !ok {
		return false
	}
	switch apiErr.ErrorCode() {
	case "NoSuchKey", "NotFound", "404":
		return true
	default:
		return false
	}
}
