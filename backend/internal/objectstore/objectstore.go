// Package objectstore is what the platform needs from an object store, and
// nothing else.
//
// Every read the platform makes of a dataset went through `*minio.Client`
// directly: the file explorer, the previews, the extracts, the ontology scan,
// the snapshots and the fingerprint crossing. That is twelve operations
// spread over forty-one call sites and six files, with minio-go's own types -
// ObjectInfo, ListObjectsOptions, StatObjectOptions - reaching into handler
// signatures.
//
// Two consequences, and the second is the one that bites daily.
//
// A store that does not speak S3 cannot be supported without touching all of
// it. Azure Blob is the live example: it has no S3 API, so a client on Azure
// either runs our MinIO or does not mount their own data. Note what this is
// *not* for: swapping MinIO for Garage or SeaweedFS needs none of this, since
// both speak S3 and minio-go already talks to them.
//
// And a handler that takes a concrete client cannot be tested without one.
// The snapshot handlers, the explorer and the crossing all need a real
// endpoint to exercise at all, which is why their tests cover the arithmetic
// around the storage and not the part that talks to it. An interface with an
// in-memory implementation makes that testable, today, with no Azure client
// anywhere in sight.
//
// The interface is derived from what the call sites do, not from what an
// object store could do. Twelve operations, no options structs nobody passes,
// and no bucket policy or lifecycle management - the platform has never asked
// for those and a method nobody calls is a method nobody notices breaking.
package objectstore

import (
	"context"
	"errors"
	"io"
	"net/url"
	"time"
)

// Object is one stored object, carrying the four fields the platform reads.
//
// Not minio.ObjectInfo trimmed down: ownership, storage class, user metadata,
// replication status and the rest are things no screen shows and no decision
// uses. A field here is a field something reads.
type Object struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag"`
	LastModified time.Time `json:"lastModified"`
}

// ListOptions is how a listing is narrowed.
type ListOptions struct {
	Prefix string
	// Recursive lists everything beneath the prefix. False stops at the
	// first delimiter, which is what a file explorer showing one folder
	// wants.
	Recursive bool
}

// ErrNotFound is what every implementation reports for an object or bucket
// that is not there.
//
// It exists because the alternative is each call site inspecting the
// provider's error shape - `minio.ToErrorResponse(err).Code == "NoSuchKey"`
// in one place and a status code comparison in another, both of which are
// silently wrong against a store that words it differently.
var ErrNotFound = errors.New("object not found")

// Store is what the platform asks of an object store.
type Store interface {
	// List walks the objects under a prefix, calling visit for each. A
	// callback rather than a slice or a channel: a dataset can hold hundreds
	// of thousands of objects, a caller that has seen enough stops by
	// returning an error, and nothing accumulates that the caller did not ask
	// to accumulate.
	List(ctx context.Context, bucket string, options ListOptions, visit func(Object) error) error

	// Stat answers ErrNotFound rather than an implementation's own error.
	Stat(ctx context.Context, bucket, key string) (Object, error)

	// Get hands back a reader the caller closes.
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)

	// Put stores size bytes. The size is required rather than optional: a
	// store that has to buffer an unknown length to find out is a store that
	// holds a multi-gigabyte upload in memory.
	Put(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error

	// Remove deletes one object. Removing what is not there is not an error:
	// the caller wanted it gone and it is gone.
	Remove(ctx context.Context, bucket, key string) error

	// RemoveAll deletes many, and reports the first failure with the key it
	// failed on - "a deletion failed" sends somebody reading logs.
	RemoveAll(ctx context.Context, bucket string, keys []string) error

	// Copy duplicates an object server-side, without the bytes travelling
	// through the platform.
	Copy(ctx context.Context, sourceBucket, sourceKey, destinationBucket, destinationKey string) error

	BucketExists(ctx context.Context, bucket string) (bool, error)
	MakeBucket(ctx context.Context, bucket, region string) error
	ListBuckets(ctx context.Context) ([]string, error)

	// PresignedGet and PresignedPut hand a URL to a browser so bytes travel
	// between the person and the store, never through the platform.
	//
	// They are the one place this interface admits it is shaped by HTTP
	// object stores. A store without them cannot be supported by handing the
	// caller a URL, and the honest thing is for its implementation to refuse
	// rather than to proxy the bytes silently.
	PresignedGet(ctx context.Context, bucket, key string, lifetime time.Duration, parameters url.Values) (*url.URL, error)
	PresignedPut(ctx context.Context, bucket, key string, lifetime time.Duration) (*url.URL, error)
}
