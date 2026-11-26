package objectstore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

// S3 adapts minio-go, which is the client for every S3-compatible store the
// platform meets: the internal MinIO, Clever Cloud Cellar, AWS S3, GCS through
// its interoperability endpoint, and whatever a client arrives with.
type S3 struct {
	client *minio.Client
	// region is carried because MakeBucket needs it and the rest does not.
	region string
}

// NewS3 wraps a configured client. The client is built where the credentials
// are resolved; this package does not read secrets.
func NewS3(client *minio.Client, region string) *S3 {
	return &S3{client: client, region: region}
}

func (s *S3) List(ctx context.Context, bucket string, options ListOptions, visit func(Object) error) error {
	// A cancellable child context, because minio-go's channel is only closed
	// when the listing finishes or the context ends: returning from the range
	// without cancelling leaks the goroutine feeding it.
	listCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	prefix := strings.TrimPrefix(strings.TrimSpace(options.Prefix), "/")
	for item := range s.client.ListObjects(listCtx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: options.Recursive,
	}) {
		if item.Err != nil {
			return translateError(item.Err)
		}
		if err := visit(Object{
			Path:         item.Key,
			Size:         item.Size,
			ETag:         strings.Trim(item.ETag, `"`),
			LastModified: item.LastModified.UTC(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *S3) Stat(ctx context.Context, bucket, key string) (Object, error) {
	info, err := s.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return Object{}, translateError(err)
	}
	return Object{
		Path:         info.Key,
		Size:         info.Size,
		ETag:         strings.Trim(info.ETag, `"`),
		LastModified: info.LastModified.UTC(),
	}, nil
}

func (s *S3) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	object, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, translateError(err)
	}
	// GetObject defers the request, so a missing object surfaces on the first
	// read rather than here. Stat forces the answer now, which is what every
	// caller expects from a function that returns an error.
	if _, err := object.Stat(); err != nil {
		object.Close()
		return nil, translateError(err)
	}
	return object, nil
}

func (s *S3) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	return translateError(err)
}

func (s *S3) Remove(ctx context.Context, bucket, key string) error {
	err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
	if errors.Is(translateError(err), ErrNotFound) {
		return nil
	}
	return translateError(err)
}

func (s *S3) RemoveAll(ctx context.Context, bucket string, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	objects := make(chan minio.ObjectInfo, len(keys))
	for _, key := range keys {
		objects <- minio.ObjectInfo{Key: key}
	}
	close(objects)
	for failure := range s.client.RemoveObjects(ctx, bucket, objects, minio.RemoveObjectsOptions{}) {
		if failure.Err != nil {
			return errors.New("failed to remove " + failure.ObjectName + ": " + failure.Err.Error())
		}
	}
	return nil
}

func (s *S3) Copy(ctx context.Context, sourceBucket, sourceKey, destinationBucket, destinationKey string) error {
	_, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: destinationBucket, Object: destinationKey},
		minio.CopySrcOptions{Bucket: sourceBucket, Object: sourceKey},
	)
	return translateError(err)
}

func (s *S3) BucketExists(ctx context.Context, bucket string) (bool, error) {
	exists, err := s.client.BucketExists(ctx, bucket)
	return exists, translateError(err)
}

func (s *S3) MakeBucket(ctx context.Context, bucket, region string) error {
	if strings.TrimSpace(region) == "" {
		region = s.region
	}
	return translateError(s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: region}))
}

func (s *S3) ListBuckets(ctx context.Context) ([]string, error) {
	buckets, err := s.client.ListBuckets(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	names := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		names = append(names, bucket.Name)
	}
	return names, nil
}

func (s *S3) PresignedGet(ctx context.Context, bucket, key string, lifetime time.Duration, parameters url.Values) (*url.URL, error) {
	address, err := s.client.PresignedGetObject(ctx, bucket, key, lifetime, parameters)
	return address, translateError(err)
}

func (s *S3) PresignedPut(ctx context.Context, bucket, key string, lifetime time.Duration) (*url.URL, error) {
	address, err := s.client.PresignedPutObject(ctx, bucket, key, lifetime)
	return address, translateError(err)
}

// translateError turns "not there" into ErrNotFound and leaves everything
// else alone.
//
// Both shapes are checked because both occur: the S3 error code on a
// StatObject, and a bare 404 from a store that answers without a body. The
// call sites used to check one or the other, each of them silently wrong
// against the half it did not check.
func translateError(err error) error {
	if err == nil {
		return nil
	}
	response := minio.ToErrorResponse(err)
	switch {
	case response.Code == "NoSuchKey", response.Code == "NoSuchBucket", response.Code == "NotFound":
		return ErrNotFound
	case response.StatusCode == http.StatusNotFound:
		return ErrNotFound
	}
	return err
}
