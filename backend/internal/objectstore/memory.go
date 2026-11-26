package objectstore

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// Memory is an object store that holds everything in a map.
//
// It exists so the handlers that read datasets can be tested at all. Today
// they take a concrete S3 client, so the explorer, the previews, the ontology
// scan, the snapshots and the fingerprint crossing are exercised only around
// the storage, never through it - and the parts that talk to the store are
// the parts where the interesting mistakes live.
//
// Deliberately not a fake that agrees with everything: it enforces what the
// interface promises, so a caller that relies on behaviour the real store does
// not have fails here rather than in production. A missing object is
// ErrNotFound, listing is ordered, and a non-recursive listing collapses at the
// delimiter the way S3 does.
type Memory struct {
	mutex   sync.RWMutex
	buckets map[string]map[string][]byte
	// types holds what Put was told, because Stat answers with it and a fake
	// that always says "" would hide a handler reading the wrong field.
	types map[string]string
	// Clock lets a test fix LastModified. Zero means time.Now.
	Clock func() time.Time
}

func NewMemory(buckets ...string) *Memory {
	store := &Memory{buckets: map[string]map[string][]byte{}, types: map[string]string{}}
	for _, bucket := range buckets {
		store.buckets[bucket] = map[string][]byte{}
	}
	return store
}

func (m *Memory) now() time.Time {
	if m.Clock != nil {
		return m.Clock()
	}
	return time.Now().UTC()
}

// Seed puts an object in without going through the interface, for a test that
// is arranging a situation rather than exercising a write.
func (m *Memory) Seed(bucket, key string, content []byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.buckets[bucket] == nil {
		m.buckets[bucket] = map[string][]byte{}
	}
	if m.types == nil {
		m.types = map[string]string{}
	}
	m.buckets[bucket][key] = append([]byte(nil), content...)
}

func (m *Memory) object(bucket, key string, content []byte) Object {
	sum := md5.Sum(content)
	return Object{
		Path:         key,
		Size:         int64(len(content)),
		ETag:         hex.EncodeToString(sum[:]),
		LastModified: m.now(),
		ContentType:  m.types[bucket+"/"+key],
	}
}

func (m *Memory) List(_ context.Context, bucket string, options ListOptions, visit func(Object) error) error {
	m.mutex.RLock()
	objects, known := m.buckets[bucket]
	keys := make([]string, 0, len(objects))
	for key := range objects {
		keys = append(keys, key)
	}
	contents := make(map[string][]byte, len(objects))
	for key, content := range objects {
		contents[key] = content
	}
	m.mutex.RUnlock()

	if !known {
		return ErrNotFound
	}
	sort.Strings(keys)

	prefix := strings.TrimPrefix(strings.TrimSpace(options.Prefix), "/")
	seenFolders := map[string]bool{}
	for _, key := range keys {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if !options.Recursive {
			// S3 collapses everything past the next delimiter into one
			// common prefix. A fake that returns the full depth lets a
			// caller believe a flat listing walks the tree.
			rest := strings.TrimPrefix(key, prefix)
			if slash := strings.Index(rest, "/"); slash >= 0 {
				folder := prefix + rest[:slash+1]
				if seenFolders[folder] {
					continue
				}
				seenFolders[folder] = true
				if err := visit(Object{Path: folder}); err != nil {
					return err
				}
				continue
			}
		}
		if err := visit(m.object(bucket, key, contents[key])); err != nil {
			return err
		}
	}
	return nil
}

func (m *Memory) Stat(_ context.Context, bucket, key string) (Object, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	content, found := m.buckets[bucket][key]
	if !found {
		return Object{}, ErrNotFound
	}
	return m.object(bucket, key, content), nil
}

func (m *Memory) Get(_ context.Context, bucket, key string) (io.ReadCloser, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	content, found := m.buckets[bucket][key]
	if !found {
		return nil, ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), content...))), nil
}

func (m *Memory) GetRange(_ context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	content, found := m.buckets[bucket][key]
	if !found {
		return nil, ErrNotFound
	}
	if offset < 0 || offset > int64(len(content)) {
		return nil, errors.New("objectstore: range starts outside the object")
	}
	end := int64(len(content))
	if length >= 0 && offset+length < end {
		end = offset + length
	}
	return io.NopCloser(bytes.NewReader(append([]byte(nil), content[offset:end]...))), nil
}

func (m *Memory) Put(_ context.Context, bucket, key string, body io.Reader, size int64, contentType string) (Object, error) {
	content, err := io.ReadAll(body)
	if err != nil {
		return Object{}, err
	}
	// The interface says size is the number of bytes. Honouring that here
	// catches a caller that computed it wrong, which against a real store
	// shows up as a truncated object nobody notices until it is read.
	if size >= 0 && int64(len(content)) != size {
		return Object{}, errors.New("objectstore: body length does not match the declared size")
	}
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if m.buckets[bucket] == nil {
		return Object{}, ErrNotFound
	}
	m.buckets[bucket][key] = content
	if contentType != "" {
		m.types[bucket+"/"+key] = contentType
	}
	return m.object(bucket, key, content), nil
}

func (m *Memory) Remove(_ context.Context, bucket, key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.buckets[bucket], key)
	return nil
}

func (m *Memory) RemoveAll(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		if err := m.Remove(ctx, bucket, key); err != nil {
			return &RemoveFailure{Key: key, Err: err}
		}
	}
	return nil
}

func (m *Memory) Copy(_ context.Context, sourceBucket, sourceKey, destinationBucket, destinationKey string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	content, found := m.buckets[sourceBucket][sourceKey]
	if !found {
		return ErrNotFound
	}
	if m.buckets[destinationBucket] == nil {
		return ErrNotFound
	}
	m.buckets[destinationBucket][destinationKey] = append([]byte(nil), content...)
	return nil
}

func (m *Memory) BucketExists(_ context.Context, bucket string) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	_, found := m.buckets[bucket]
	return found, nil
}

func (m *Memory) MakeBucket(_ context.Context, bucket, _ string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if _, found := m.buckets[bucket]; found {
		return errors.New("objectstore: bucket already exists")
	}
	m.buckets[bucket] = map[string][]byte{}
	return nil
}

func (m *Memory) RemoveBucket(_ context.Context, bucket string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	objects, found := m.buckets[bucket]
	if !found {
		return ErrNotFound
	}
	if len(objects) > 0 {
		return errors.New("objectstore: the bucket is not empty")
	}
	delete(m.buckets, bucket)
	return nil
}

func (m *Memory) ListBuckets(context.Context) ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	names := make([]string, 0, len(m.buckets))
	for name := range m.buckets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// The two presigned operations answer a URL that goes nowhere. A test that
// checks a handler hands one back is testing the handler; a test that follows
// it is testing nothing, and should use a real store.
func (m *Memory) PresignedGet(_ context.Context, bucket, key string, _ time.Duration, parameters url.Values) (*url.URL, error) {
	address := &url.URL{Scheme: "https", Host: "objectstore.test", Path: "/" + bucket + "/" + key}
	if len(parameters) > 0 {
		address.RawQuery = parameters.Encode()
	}
	return address, nil
}

func (m *Memory) PresignedPut(_ context.Context, bucket, key string, _ time.Duration) (*url.URL, error) {
	return &url.URL{Scheme: "https", Host: "objectstore.test", Path: "/" + bucket + "/" + key}, nil
}
