// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/best-friends/my/internal/app/blob"
	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/media"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/content"
)

// MediaHarness is one database under test: an empty database with the product schema, its
// UnitOfWork, and the content module's media and status repositories on it.
type MediaHarness struct {
	UnitOfWork uow.UnitOfWork
	Media      appcontent.MediaRepository
	Statuses   appcontent.Repository
}

// MediaFactory returns a harness on a new, empty database whose UnitOfWork uses opts. The
// factory registers the cleanup of everything it creates with t.Cleanup.
type MediaFactory func(t *testing.T, opts uow.Options) MediaHarness

// MediaRepositoryContract verifies the media repository of a database adapter through the
// use cases that drive it: uploads stored with the detected type and their processing job
// enqueued, rejected uploads leaving nothing behind, idempotent processing that records one
// result with its variants and deletes the file as received, failures recorded as results,
// focal points and descriptions as current values that only the uploader reads and changes
// until a status attaches the file, and the cleanup of old unattached uploads that keeps attached
// and referenced ones.
func MediaRepositoryContract(t *testing.T, newHarness MediaFactory) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, e *mediaEnv)
	}{
		{"StoresUploads", mediaStoresUploads},
		{"RejectsUploadsWithoutWriting", mediaRejectsUploads},
		{"ProcessesIdempotently", mediaProcessesIdempotently},
		{"RecordsFailedProcessing", mediaRecordsFailures},
		{"RetriesOnTransientErrors", mediaRetriesTransient},
		{"KeepsTheFocusAsACurrentValue", mediaKeepsFocus},
		{"UpdatesOnlyOwnUnattachedUploads", mediaUpdates},
		{"CleansUpOldUnattachedUploads", mediaCleansUp},
		{"CleanupPagesPastReferencedUploads", mediaCleanupPages},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { tc.run(t, newMediaEnv(t, newHarness)) })
	}
}

// recordingJobs is a queue.JobQueue that keeps what it is given.
type recordingJobs struct {
	mu   sync.Mutex
	jobs []queue.NewJob
}

func (r *recordingJobs) Enqueue(_ context.Context, jobs ...queue.NewJob) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, jobs...)
	return nil
}

// referrerSet is a MediaReferrer that references a fixed set of media files.
type referrerSet map[string]bool

func (r referrerSet) IsMediaReferenced(_ context.Context, id string) (bool, error) { return r[id], nil }

type mediaEnv struct {
	h         MediaHarness
	clock     *Clock
	ids       *IDGenerator
	blobs     *MemoryBlobStore
	jobs      *recordingJobs
	referrers referrerSet
	service   *appcontent.MediaService
}

const mediaOwner = "0192f0c1-0000-7000-8000-00000000a11c"

func newMediaEnv(t *testing.T, newHarness MediaFactory) *mediaEnv {
	t.Helper()
	projectors := &uow.Projectors{}
	h := newHarness(t, uow.Options{Retry: uow.RetryPolicy{MaxAttempts: 20}, Projectors: projectors})
	appcontent.RegisterProjectors(projectors, h.Statuses)
	clock := NewClock(time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	e := &mediaEnv{h: h, clock: clock, ids: NewIDGenerator(clock), blobs: NewMemoryBlobStore(clock), jobs: &recordingJobs{}, referrers: referrerSet{}}
	e.service = appcontent.NewMediaService(appcontent.MediaDeps{
		UnitOfWork: h.UnitOfWork, Repository: h.Media, Blobs: e.blobs, Processor: FakeMediaProcessor{},
		Jobs: e.jobs, Clock: clock, IDs: e.ids, Limits: media.Limits{MaxBytes: 1 << 20},
		Referrers: []appcontent.MediaReferrer{e.referrers},
	})
	return e
}

func (e *mediaEnv) upload(t *testing.T, data []byte) appcontent.Media {
	t.Helper()
	e.clock.Advance(time.Millisecond)
	m, err := e.service.Upload(t.Context(), appcontent.UploadInput{OwnerAccountID: mediaOwner, Content: bytes.NewReader(data)})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	return m
}

func (e *mediaEnv) process(t *testing.T, id string) {
	t.Helper()
	if res := e.service.Process(t.Context(), id); !res.IsDone() {
		t.Fatalf("Process(%s) = %+v, want Done", id, res)
	}
}

func (e *mediaEnv) media(t *testing.T, id string) appcontent.Media {
	t.Helper()
	m, err := e.service.Media(t.Context(), id)
	if err != nil {
		t.Fatalf("Media(%s): %v", id, err)
	}
	return m
}

func mediaStoresUploads(t *testing.T, e *mediaEnv) {
	data := FixturePNG(4, 3, false)
	m := e.upload(t, data)
	if m.File.OwnerAccountID != mediaOwner || m.File.Origin != content.MediaOriginUpload || !m.File.CreatedAt.Equal(e.clock.Now()) {
		t.Errorf("File = %+v", m.File)
	}
	want := content.UploadedMedia{MediaID: m.File.ID, BlobKey: "uploads/" + m.File.ID, ContentType: "image/png", FileSize: int64(len(data))}
	if m.Upload == nil || *m.Upload != want {
		t.Errorf("Upload = %+v, want %+v", m.Upload, want)
	}
	if m.Result != nil || m.Focus != nil || m.Ready() {
		t.Errorf("a new upload has a result or focus: %+v", m)
	}
	if !bytes.Equal(e.blobs.Bytes(want.BlobKey), data) {
		t.Errorf("the received file is not in blob storage")
	}
	if strings.HasPrefix(want.BlobKey, blob.PublicPrefix) {
		t.Errorf("the received file is public: %s", want.BlobKey)
	}
	if len(e.jobs.jobs) != 1 {
		t.Fatalf("enqueued %d jobs, want 1", len(e.jobs.jobs))
	}
	j := e.jobs.jobs[0]
	var p struct {
		MediaID string `json:"media_id"`
	}
	if err := json.Unmarshal(j.Payload, &p); err != nil || j.Type != appcontent.JobProcessMedia || p.MediaID != m.File.ID || j.IdempotencyKey == "" {
		t.Errorf("job = %+v", j)
	}
	// A declared type that matches, or tells nothing, is accepted.
	for _, declared := range []string{"image/png", "image/png; charset=binary", "application/octet-stream", ""} {
		if _, err := e.service.Upload(t.Context(), appcontent.UploadInput{OwnerAccountID: mediaOwner, Content: bytes.NewReader(data), DeclaredContentType: declared}); err != nil {
			t.Errorf("declared %q: %v", declared, err)
		}
	}
}

func mediaRejectsUploads(t *testing.T, e *mediaEnv) {
	png := FixturePNG(4, 3, false)
	for name, tc := range map[string]struct {
		data     []byte
		declared string
		want     error
	}{
		"svg":           {[]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "image/svg+xml", media.ErrUnsupportedFormat},
		"html as jpeg":  {[]byte("<html><script>alert(1)</script></html>"), "image/jpeg", media.ErrUnsupportedFormat},
		"png as jpeg":   {png, "image/jpeg", appcontent.ErrContentTypeMismatch},
		"png as gif":    {png, "image/gif", appcontent.ErrContentTypeMismatch},
		"bad declared":  {png, "not a type", appcontent.ErrContentTypeMismatch},
		"too large":     {append(slices.Clone(png), make([]byte, 1<<20)...), "", media.ErrTooLarge},
		"corrupt image": {append(slices.Clone(png), "corrupt"...), "", media.ErrCorrupt},
	} {
		_, err := e.service.Upload(t.Context(), appcontent.UploadInput{OwnerAccountID: mediaOwner, Content: bytes.NewReader(tc.data), DeclaredContentType: tc.declared})
		if !errors.Is(err, tc.want) {
			t.Errorf("%s: Upload = %v, want %v", name, err, tc.want)
		}
	}
	if keys := e.blobs.Keys(); len(keys) != 0 {
		t.Errorf("rejected uploads left blobs: %v", keys)
	}
	if len(e.jobs.jobs) != 0 {
		t.Errorf("rejected uploads enqueued %d jobs", len(e.jobs.jobs))
	}
	ids, err := e.h.Media.ListUnattachedUploads(t.Context(), "", e.clock.Now().Add(time.Hour), 10)
	if err != nil || len(ids) != 0 {
		t.Errorf("rejected uploads left media files: %v, %v", ids, err)
	}
}

func mediaProcessesIdempotently(t *testing.T, e *mediaEnv) {
	m := e.upload(t, FixturePNG(4, 3, false))
	e.clock.Advance(time.Second)
	for range 3 {
		e.process(t, m.File.ID)
	}
	got := e.media(t, m.File.ID)
	if !got.Ready() {
		t.Fatalf("after processing: %+v", got.Result)
	}
	r := got.Result
	if r.MediaType != content.MediaTypeImage || r.Width != 4 || r.Height != 3 || r.Blurhash == "" || !r.ProcessedAt.Equal(e.clock.Now()) {
		t.Errorf("Result = %+v", r)
	}
	full, okFull := r.Variant(content.VariantFull)
	thumb, okThumb := r.Variant(content.VariantThumbnail)
	if !okFull || !okThumb || len(r.Variants) != 2 {
		t.Fatalf("Variants = %+v", r.Variants)
	}
	if full.BlobKey != "media/"+m.File.ID+"/full.png" || full.ContentType != "image/png" || full.Width != 4 || full.Height != 3 || full.ResultID != r.ID {
		t.Errorf("full = %+v", full)
	}
	if thumb.BlobKey != "media/"+m.File.ID+"/thumbnail.png" || thumb.Width != 2 || thumb.Height != 1 {
		t.Errorf("thumbnail = %+v", thumb)
	}
	if full.FileSize != int64(len(e.blobs.Bytes(full.BlobKey))) || full.FileSize == 0 {
		t.Errorf("full FileSize = %d, blob has %d bytes", full.FileSize, len(e.blobs.Bytes(full.BlobKey)))
	}
	if want := []string{full.BlobKey, thumb.BlobKey}; !slices.Equal(e.blobs.Keys(), want) {
		t.Errorf("blobs = %v, want only the variants %v (the received file deleted)", e.blobs.Keys(), want)
	}
	results, err := e.h.Media.ListProcessingResults(t.Context(), m.File.ID)
	if err != nil || len(results) != 1 {
		t.Errorf("%d results after three runs (%v), want 1", len(results), err)
	}
	if u := e.service.URL(full); u != e.blobs.PublicURL(full.BlobKey) {
		t.Errorf("URL = %q", u)
	}
	// A result recorded concurrently makes the insert a no-op.
	err = e.h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		has, err := e.h.Media.HasProcessingResult(t.Context(), tx, m.File.ID)
		if err == nil && !has {
			t.Error("HasProcessingResult = false")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func mediaRecordsFailures(t *testing.T, e *mediaEnv) {
	m := e.upload(t, FixturePNG(4, 3, false))
	// The stored file turned out not to decode.
	if _, err := e.blobs.Put(t.Context(), m.Upload.BlobKey, strings.NewReader("\x89PNG\r\n\x1A\ncorrupt"), blob.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	e.process(t, m.File.ID)
	got := e.media(t, m.File.ID)
	if got.Result == nil || got.Result.Outcome != content.MediaFailed || got.Ready() || len(got.Result.Variants) != 0 {
		t.Fatalf("Result = %+v, want a failure without variants", got.Result)
	}
	if got.Result.Width != 0 || got.Result.Blurhash != "" {
		t.Errorf("a failure has dimensions or a blurhash: %+v", got.Result)
	}
	if keys := e.blobs.Keys(); len(keys) != 0 {
		t.Errorf("blobs after a failure: %v", keys)
	}
	e.process(t, m.File.ID)

	// A received file that is gone is a failure too.
	m2 := e.upload(t, FixturePNG(4, 3, false))
	if err := e.blobs.Delete(t.Context(), m2.Upload.BlobKey); err != nil {
		t.Fatal(err)
	}
	e.process(t, m2.File.ID)
	if got := e.media(t, m2.File.ID); got.Result == nil || got.Result.Outcome != content.MediaFailed {
		t.Errorf("missing file: Result = %+v", got.Result)
	}
	// An unknown media file (cleaned up before its job ran) is done.
	e.process(t, "0192f0c1-0000-7000-8000-00000000dead")
}

func mediaRetriesTransient(t *testing.T, e *mediaEnv) {
	m := e.upload(t, FixturePNG(4, 3, false))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if res := e.service.Process(ctx, m.File.ID); !res.IsRetry() {
		t.Errorf("Process with a cancelled context = %+v, want Retry", res)
	}
	if got := e.media(t, m.File.ID); got.Result != nil {
		t.Errorf("a cancelled run recorded %+v", got.Result)
	}
}

func mediaKeepsFocus(t *testing.T, e *mediaEnv) {
	m := e.upload(t, FixturePNG(4, 3, false))
	for _, f := range []content.MediaFocus{{MediaID: m.File.ID, X: 0.5, Y: -0.25}, {MediaID: m.File.ID, X: -1, Y: 1}} {
		if err := e.service.SetFocus(t.Context(), mediaOwner, f); err != nil {
			t.Fatalf("SetFocus(%+v): %v", f, err)
		}
		if got := e.media(t, m.File.ID).Focus; got == nil || *got != f {
			t.Errorf("Focus = %+v, want %+v", got, f)
		}
	}
	if err := e.service.SetFocus(t.Context(), mediaOwner, content.MediaFocus{MediaID: m.File.ID, X: 1.5}); !errors.Is(err, content.ErrInvalidFocus) {
		t.Errorf("out of range: %v", err)
	}
	if err := e.service.SetFocus(t.Context(), "someone-else", content.MediaFocus{MediaID: m.File.ID}); !errors.Is(err, appcontent.ErrNotOwner) {
		t.Errorf("other owner: %v", err)
	}
	if err := e.service.SetFocus(t.Context(), mediaOwner, content.MediaFocus{MediaID: "0192f0c1-0000-7000-8000-00000000dead"}); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("unknown file: %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

func mediaUpdates(t *testing.T, e *mediaEnv) {
	e.clock.Advance(time.Millisecond)
	m, err := e.service.Upload(t.Context(), appcontent.UploadInput{OwnerAccountID: mediaOwner, Content: bytes.NewReader(FixturePNG(4, 3, false)), Description: "a cat"})
	if err != nil {
		t.Fatalf("Upload with a description: %v", err)
	}
	if m.Description != "a cat" {
		t.Errorf("Description after upload = %q", m.Description)
	}
	id := m.File.ID
	if got, err := e.service.OwnMedia(t.Context(), mediaOwner, id); err != nil || got.File.ID != id || got.Description != "a cat" {
		t.Errorf("OwnMedia = %+v, %v", got, err)
	}
	if _, err := e.service.OwnMedia(t.Context(), "someone-else", id); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("OwnMedia of another account: %v, want ErrNotFound", err)
	}

	// Only the given fields change.
	got, err := e.service.Update(t.Context(), mediaOwner, appcontent.MediaUpdate{MediaID: id, Focus: &content.MediaFocus{X: 0.5, Y: -0.5}})
	if err != nil || got.Description != "a cat" || got.Focus == nil || *got.Focus != (content.MediaFocus{MediaID: id, X: 0.5, Y: -0.5}) {
		t.Errorf("Update focus = %+v, %v", got, err)
	}
	got, err = e.service.Update(t.Context(), mediaOwner, appcontent.MediaUpdate{MediaID: id, Description: ptr("a black cat")})
	if err != nil || got.Description != "a black cat" || got.Focus == nil || got.Focus.X != 0.5 {
		t.Errorf("Update description = %+v, %v", got, err)
	}
	got, err = e.service.Update(t.Context(), mediaOwner, appcontent.MediaUpdate{MediaID: id, Description: ptr("")})
	if err != nil || got.Description != "" {
		t.Errorf("removing the description = %+v, %v", got, err)
	}
	if _, err := e.h.Media.FindDescription(t.Context(), id); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("a removed description is still stored: %v", err)
	}

	long := strings.Repeat("あ", content.MaxMediaDescriptionLength+1)
	for name, tc := range map[string]struct {
		account string
		u       appcontent.MediaUpdate
		want    error
	}{
		"too long":      {mediaOwner, appcontent.MediaUpdate{MediaID: id, Description: &long}, content.ErrDescriptionTooLong},
		"bad focus":     {mediaOwner, appcontent.MediaUpdate{MediaID: id, Focus: &content.MediaFocus{X: 2}}, content.ErrInvalidFocus},
		"other account": {"someone-else", appcontent.MediaUpdate{MediaID: id, Description: ptr("mine")}, appcontent.ErrNotFound},
		"unknown":       {mediaOwner, appcontent.MediaUpdate{MediaID: "0192f0c1-0000-7000-8000-00000000dead", Description: ptr("x")}, appcontent.ErrNotFound},
	} {
		if _, err := e.service.Update(t.Context(), tc.account, tc.u); !errors.Is(err, tc.want) {
			t.Errorf("%s: Update = %v, want %v", name, err, tc.want)
		}
	}
	if _, err := e.service.Upload(t.Context(), appcontent.UploadInput{OwnerAccountID: mediaOwner, Content: bytes.NewReader(FixturePNG(4, 3, false)), Description: long}); !errors.Is(err, content.ErrDescriptionTooLong) {
		t.Errorf("Upload with a long description = %v", err)
	}
	if got := e.media(t, id); got.Description != "" || got.Focus.X != 0.5 {
		t.Errorf("rejected updates changed the file: %+v", got)
	}
	// Exactly the maximum is accepted.
	if _, err := e.service.Update(t.Context(), mediaOwner, appcontent.MediaUpdate{MediaID: id, Description: ptr(long[len("あ"):])}); err != nil {
		t.Errorf("a description of the maximum length: %v", err)
	}

	// Once a status attaches the file, it no longer changes on its own.
	e.attach(t, id)
	if _, err := e.service.Update(t.Context(), mediaOwner, appcontent.MediaUpdate{MediaID: id, Description: ptr("changed")}); !errors.Is(err, appcontent.ErrMediaAttached) {
		t.Errorf("Update of an attached file = %v, want ErrMediaAttached", err)
	}
	err = e.h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		attached, err := e.h.Media.IsAttached(t.Context(), tx, id)
		if err == nil && !attached {
			t.Error("IsAttached = false for an attached file")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

// attach inserts a local status whose first revision attaches mediaID.
func (e *mediaEnv) attach(t *testing.T, mediaID string) {
	t.Helper()
	e.clock.Advance(time.Millisecond)
	id := e.ids.NewID()
	s := content.Status{
		ID: id, URI: "https://" + contractHost + "/ap/statuses/" + id, AuthorAccountID: mediaOwner, IsLocal: true,
		Visibility: content.VisibilityPublic, PublishedAt: e.clock.Now(),
	}
	rev := content.Revision{
		ID: e.ids.NewID(), StatusID: id, Seq: 1, Language: "en", RevisedAt: s.PublishedAt,
		Bodies:      []content.Body{{Language: "en", ContentHTML: "<p>look</p>"}},
		Attachments: []content.Attachment{{Position: 0, MediaID: mediaID, Description: "a picture"}},
	}
	err := e.h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		if err := e.h.Statuses.InsertStatus(t.Context(), tx, s); err != nil {
			return err
		}
		if err := e.h.Statuses.InsertRevision(t.Context(), tx, rev); err != nil {
			return err
		}
		return tx.Emit(t.Context(), appcontent.StatusCreated{Status: s, Revision: rev})
	})
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
}

func mediaCleansUp(t *testing.T, e *mediaEnv) {
	unattached := e.upload(t, FixturePNG(4, 3, false))
	e.process(t, unattached.File.ID)
	if _, err := e.service.Update(t.Context(), mediaOwner, appcontent.MediaUpdate{MediaID: unattached.File.ID, Description: ptr("a cat")}); err != nil {
		t.Fatal(err)
	}
	if err := e.service.SetFocus(t.Context(), mediaOwner, content.MediaFocus{MediaID: unattached.File.ID, X: 0.1}); err != nil {
		t.Fatal(err)
	}
	attached := e.upload(t, FixturePNG(4, 3, false))
	e.process(t, attached.File.ID)
	e.attach(t, attached.File.ID)
	referenced := e.upload(t, FixturePNG(4, 3, false))
	e.referrers[referenced.File.ID] = true
	unprocessed := e.upload(t, FixturePNG(4, 3, false))

	e.clock.Advance(appcontent.DefaultUnattachedTTL - time.Minute)
	recent := e.upload(t, FixturePNG(4, 3, false))
	if n, err := e.service.CleanupUnattached(t.Context()); err != nil || n != 0 {
		t.Fatalf("cleanup before the TTL deleted %d (%v)", n, err)
	}

	e.clock.Advance(2 * time.Minute)
	n, err := e.service.CleanupUnattached(t.Context())
	if err != nil || n != 2 {
		t.Fatalf("CleanupUnattached = %d, %v; want 2 (the processed and the unprocessed unattached uploads)", n, err)
	}
	for _, gone := range []appcontent.Media{unattached, unprocessed} {
		if _, err := e.service.Media(t.Context(), gone.File.ID); !errors.Is(err, appcontent.ErrNotFound) {
			t.Errorf("deleted upload: Media = %v, want ErrNotFound", err)
		}
		if _, err := e.h.Media.FindFocus(t.Context(), gone.File.ID); !errors.Is(err, appcontent.ErrNotFound) {
			t.Errorf("deleted upload kept its focus: %v", err)
		}
		if _, err := e.h.Media.FindDescription(t.Context(), gone.File.ID); !errors.Is(err, appcontent.ErrNotFound) {
			t.Errorf("deleted upload kept its description: %v", err)
		}
		if rs, err := e.h.Media.ListProcessingResults(t.Context(), gone.File.ID); err != nil || len(rs) != 0 {
			t.Errorf("deleted upload kept %d results (%v)", len(rs), err)
		}
		for _, k := range e.blobs.Keys() {
			if strings.Contains(k, gone.File.ID) {
				t.Errorf("deleted upload kept blob %s", k)
			}
		}
	}
	for _, kept := range []appcontent.Media{attached, referenced, recent} {
		if _, err := e.service.Media(t.Context(), kept.File.ID); err != nil {
			t.Errorf("kept upload: Media = %v", err)
		}
	}
	if got := e.media(t, attached.File.ID); !got.Ready() || len(e.blobs.Bytes("media/"+attached.File.ID+"/full.png")) == 0 {
		t.Errorf("the attached upload lost its result or blobs")
	}
	if n, err := e.service.CleanupUnattached(t.Context()); err != nil || n != 0 {
		t.Errorf("second cleanup deleted %d (%v)", n, err)
	}
	// Processing a deleted upload, as a late job would, is done and writes nothing.
	e.process(t, unprocessed.File.ID)
	if _, err := e.service.Media(t.Context(), unprocessed.File.ID); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("late processing brought it back: %v", err)
	}
	// The guarded delete keeps a file that is attached.
	err = e.h.UnitOfWork.Tx(t.Context(), func(tx uow.Tx) error {
		deleted, err := e.h.Media.DeleteUnattachedMedia(t.Context(), tx, attached.File.ID)
		if err == nil && deleted {
			t.Error("DeleteUnattachedMedia deleted an attached file")
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}

func mediaCleanupPages(t *testing.T, e *mediaEnv) {
	const n = 105
	var last appcontent.Media
	for i := range n {
		last = e.upload(t, FixturePNG(4, 3, false))
		if i < n-1 {
			e.referrers[last.File.ID] = true
		}
	}
	e.clock.Advance(appcontent.DefaultUnattachedTTL + time.Minute)
	deleted, err := e.service.CleanupUnattached(t.Context())
	if err != nil || deleted != 1 {
		t.Fatalf("CleanupUnattached = %d, %v; want 1", deleted, err)
	}
	if _, err := e.service.Media(t.Context(), last.File.ID); !errors.Is(err, appcontent.ErrNotFound) {
		t.Errorf("the upload after %d referenced ones was kept: %v", n-1, err)
	}
}
