// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"sync"

	"github.com/mycroblog/my/internal/app/blob"
)

// MemoryBlobStore is an in-memory blob.Store for tests. It passes BlobStoreContract. Its
// public URLs are under https://blobs.test.
type MemoryBlobStore struct {
	clock *Clock

	mu    sync.Mutex
	blobs map[string]memoryBlob
}

type memoryBlob struct {
	data []byte
	info blob.Info
}

var _ blob.Store = (*MemoryBlobStore)(nil)

// NewMemoryBlobStore returns an empty store whose modification times come from clock.
func NewMemoryBlobStore(clock *Clock) *MemoryBlobStore {
	return &MemoryBlobStore{clock: clock, blobs: map[string]memoryBlob{}}
}

// Put implements blob.Store.
func (m *MemoryBlobStore) Put(ctx context.Context, key string, r io.Reader, opts blob.PutOptions) (blob.Info, error) {
	if err := blob.ValidateKey(key); err != nil {
		return blob.Info{}, err
	}
	if err := ctx.Err(); err != nil {
		return blob.Info{}, err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return blob.Info{}, fmt.Errorf("testkit blob: put %s: %w", key, err)
	}
	ct := opts.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}
	info := blob.Info{Key: key, ContentType: ct, Size: int64(len(data)), ModTime: m.clock.Now()}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobs[key] = memoryBlob{data: data, info: info}
	return info, nil
}

// Get implements blob.Store.
func (m *MemoryBlobStore) Get(ctx context.Context, key string) (io.ReadCloser, blob.Info, error) {
	b, err := m.lookup(ctx, key)
	if err != nil {
		return nil, blob.Info{}, err
	}
	return readSeekNopCloser{bytes.NewReader(b.data)}, b.info, nil
}

type readSeekNopCloser struct{ *bytes.Reader }

func (readSeekNopCloser) Close() error { return nil }

// Head implements blob.Store.
func (m *MemoryBlobStore) Head(ctx context.Context, key string) (blob.Info, error) {
	b, err := m.lookup(ctx, key)
	return b.info, err
}

func (m *MemoryBlobStore) lookup(ctx context.Context, key string) (memoryBlob, error) {
	if err := blob.ValidateKey(key); err != nil {
		return memoryBlob{}, err
	}
	if err := ctx.Err(); err != nil {
		return memoryBlob{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blobs[key]
	if !ok {
		return memoryBlob{}, fmt.Errorf("testkit blob: %s: %w", key, blob.ErrNotFound)
	}
	return b, nil
}

// Delete implements blob.Store.
func (m *MemoryBlobStore) Delete(ctx context.Context, key string) error {
	if err := blob.ValidateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.blobs, key)
	return nil
}

// PublicURL implements blob.Store.
func (m *MemoryBlobStore) PublicURL(key string) string { return "https://blobs.test/" + key }

// Keys returns the keys of the stored blobs in order.
func (m *MemoryBlobStore) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.blobs))
	for k := range m.blobs {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// Bytes returns the content of the blob of key, or nil when there is none.
func (m *MemoryBlobStore) Bytes(key string) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.blobs[key].data)
}
