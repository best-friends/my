// SPDX-License-Identifier: AGPL-3.0-only

package fs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	iofs "io/fs"
	"mime"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mycroblog/my/internal/app/blob"
)

// Directories under the store's root. A blob's content lives at objects/<key> and its media
// type at meta/<key>; writes go to tmp/ first and are renamed into place.
const (
	dirObjects = "objects"
	dirMeta    = "meta"
	dirTmp     = "tmp"
)

const defaultContentType = "application/octet-stream"

// maxMetaSize bounds the metadata file that Get and Head read.
const maxMetaSize = 1024

// Options configure a Store.
type Options struct {
	// Dir is the root directory of the store; it is created when missing.
	Dir string
	// BaseURL is the URL under which the HTTP server serves the public blobs, without a trailing
	// slash: PublicURL(key) is BaseURL + "/" + key. The server serves keys under
	// blob.PublicPrefix at /media/, so BaseURL is the instance origin ("https://example.com").
	BaseURL string
}

// Store implements blob.Store on a directory. Every file operation goes through an os.Root
// opened on the directory, so that no key, even one that slipped past blob.ValidateKey, and no
// symbolic link reaches a file outside it.
//
// Put writes the content and the media type to temporary files, syncs them, and renames them
// into place (the media type first), which is atomic on the same file system: a reader sees the
// previous content or the new one, never a part. While a blob is overwritten with a different
// media type, a reader may briefly get the previous content with the new type. Several
// processes (web and worker) may share the directory.
type Store struct {
	root    *os.Root
	baseURL string
}

var _ blob.Store = (*Store)(nil)

// New opens the store in o.Dir, creating the directory when missing. Close the Store to
// release the directory.
func New(o Options) (*Store, error) {
	if o.Dir == "" {
		return nil, errors.New("blob/fs: Options.Dir is required")
	}
	if err := os.MkdirAll(o.Dir, 0o750); err != nil {
		return nil, fmt.Errorf("blob/fs: create %s: %w", o.Dir, err)
	}
	root, err := os.OpenRoot(o.Dir)
	if err != nil {
		return nil, fmt.Errorf("blob/fs: open %s: %w", o.Dir, err)
	}
	for _, d := range []string{dirObjects, dirMeta, dirTmp} {
		if err := root.MkdirAll(d, 0o750); err != nil {
			_ = root.Close()
			return nil, fmt.Errorf("blob/fs: create %s: %w", filepath.Join(o.Dir, d), err)
		}
	}
	return &Store{root: root, baseURL: strings.TrimSuffix(o.BaseURL, "/")}, nil
}

// Close releases the directory.
func (s *Store) Close() error { return s.root.Close() }

func objectPath(key string) string { return filepath.FromSlash(dirObjects + "/" + key) }
func metaPath(key string) string   { return filepath.FromSlash(dirMeta + "/" + key) }

// Put implements blob.Store.
func (s *Store) Put(ctx context.Context, key string, r io.Reader, opts blob.PutOptions) (blob.Info, error) {
	if err := blob.ValidateKey(key); err != nil {
		return blob.Info{}, err
	}
	contentType := opts.ContentType
	if contentType == "" {
		contentType = defaultContentType
	}
	if _, _, err := mime.ParseMediaType(contentType); err != nil || strings.ContainsAny(contentType, "\r\n") {
		return blob.Info{}, fmt.Errorf("blob/fs: invalid content type %q", contentType)
	}
	if err := ctx.Err(); err != nil {
		return blob.Info{}, err
	}
	// Both files are written in full before either is renamed into place, so that a failed
	// write leaves the previous blob as it was.
	dataTmp, err := s.writeTemp(ctx, r)
	if err != nil {
		return blob.Info{}, fmt.Errorf("blob/fs: put %s: %w", key, err)
	}
	metaTmp, err := s.writeTemp(ctx, strings.NewReader(contentType))
	if err != nil {
		_ = s.root.Remove(dataTmp)
		return blob.Info{}, fmt.Errorf("blob/fs: put %s: %w", key, err)
	}
	if err := s.rename(metaTmp, metaPath(key)); err != nil {
		_ = s.root.Remove(dataTmp)
		_ = s.root.Remove(metaTmp)
		return blob.Info{}, fmt.Errorf("blob/fs: put %s: %w", key, err)
	}
	if err := s.rename(dataTmp, objectPath(key)); err != nil {
		_ = s.root.Remove(dataTmp)
		return blob.Info{}, fmt.Errorf("blob/fs: put %s: %w", key, err)
	}
	return s.Head(ctx, key)
}

// writeTemp writes the content of r to a new file under tmp/, synced to disk, and returns
// its name.
func (s *Store) writeTemp(ctx context.Context, r io.Reader) (name string, err error) {
	var b [16]byte
	_, _ = rand.Read(b[:])
	name = filepath.Join(dirTmp, hex.EncodeToString(b[:]))
	f, err := s.root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = s.root.Remove(name)
		}
	}()
	if _, err = io.Copy(f, ctxReader{ctx: ctx, r: r}); err != nil {
		return "", err
	}
	if err = f.Sync(); err != nil {
		return "", err
	}
	return name, f.Close()
}

// rename moves the temporary file tmp to name, creating name's directory, which is atomic on
// the same file system.
func (s *Store) rename(tmp, name string) (err error) {
	// A concurrent Delete may remove the directory between MkdirAll and Rename; try again then.
	for range 3 {
		if err = s.root.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			return err
		}
		if err = s.root.Rename(tmp, name); !errors.Is(err, iofs.ErrNotExist) {
			return err
		}
	}
	return err
}

// ctxReader stops a copy when ctx ends.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// Get implements blob.Store. The reader is an *os.File, which implements io.ReadSeeker.
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, blob.Info, error) {
	if err := blob.ValidateKey(key); err != nil {
		return nil, blob.Info{}, err
	}
	if err := ctx.Err(); err != nil {
		return nil, blob.Info{}, err
	}
	f, err := s.root.Open(objectPath(key))
	if err != nil {
		return nil, blob.Info{}, s.notFound(err, key)
	}
	info, err := s.info(f, key)
	if err != nil {
		_ = f.Close()
		return nil, blob.Info{}, err
	}
	return f, info, nil
}

// Head implements blob.Store.
func (s *Store) Head(ctx context.Context, key string) (blob.Info, error) {
	if err := blob.ValidateKey(key); err != nil {
		return blob.Info{}, err
	}
	if err := ctx.Err(); err != nil {
		return blob.Info{}, err
	}
	f, err := s.root.Open(objectPath(key))
	if err != nil {
		return blob.Info{}, s.notFound(err, key)
	}
	defer func() { _ = f.Close() }()
	return s.info(f, key)
}

func (s *Store) info(f *os.File, key string) (blob.Info, error) {
	st, err := f.Stat()
	if err != nil {
		return blob.Info{}, fmt.Errorf("blob/fs: stat %s: %w", key, err)
	}
	if !st.Mode().IsRegular() {
		return blob.Info{}, fmt.Errorf("blob/fs: %s: %w", key, blob.ErrNotFound)
	}
	contentType := defaultContentType
	if m, err := s.root.Open(metaPath(key)); err == nil {
		b, rerr := io.ReadAll(io.LimitReader(m, maxMetaSize))
		_ = m.Close()
		if rerr == nil && len(b) > 0 {
			contentType = string(b)
		}
	}
	return blob.Info{Key: key, ContentType: contentType, Size: st.Size(), ModTime: st.ModTime().UTC()}, nil
}

func (s *Store) notFound(err error, key string) error {
	if errors.Is(err, iofs.ErrNotExist) {
		return fmt.Errorf("blob/fs: %s: %w", key, blob.ErrNotFound)
	}
	return fmt.Errorf("blob/fs: open %s: %w", key, err)
}

// Delete implements blob.Store. It also removes the directories that the deletion leaves
// empty.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := blob.ValidateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, p := range []string{objectPath(key), metaPath(key)} {
		if err := s.root.Remove(p); err != nil && !errors.Is(err, iofs.ErrNotExist) {
			return fmt.Errorf("blob/fs: delete %s: %w", key, err)
		}
	}
	for dir := path.Dir(key); dir != "."; dir = path.Dir(dir) {
		// Remove fails on a directory that is not empty, which ends the walk.
		errObjects := s.root.Remove(objectPath(dir))
		errMeta := s.root.Remove(metaPath(dir))
		if errObjects != nil && errMeta != nil {
			break
		}
	}
	return nil
}

// PublicURL implements blob.Store.
func (s *Store) PublicURL(key string) string { return s.baseURL + "/" + key }
