// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	appcontent "github.com/best-friends/my/internal/app/content"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/app/media"
	"github.com/best-friends/my/internal/domain/content"
)

// MediaFiles runs the media use cases for the client API; *content.MediaService implements it.
type MediaFiles interface {
	// Upload checks and stores an upload and enqueues its processing.
	Upload(ctx context.Context, in appcontent.UploadInput) (appcontent.Media, error)
	// OwnMedia returns a media file of an account, failing with appcontent.ErrNotFound for a
	// file of another account.
	OwnMedia(ctx context.Context, accountID, id string) (appcontent.Media, error)
	// Update changes an unattached media file of an account.
	Update(ctx context.Context, accountID string, u appcontent.MediaUpdate) (appcontent.Media, error)
	// URL returns the public URL of a variant.
	URL(v content.MediaVariant) string
	// ImageURLs returns the URLs of the images that accounts show as their own (avatars and
	// headers), keyed by ref, leaving out files of other accounts and unprocessed files.
	ImageURLs(ctx context.Context, refs []appcontent.ImageRef) (map[appcontent.ImageRef]string, error)
	// CustomEmojis returns the custom emoji with an ID in ids that can be shown, keyed by ID.
	CustomEmojis(ctx context.Context, ids []string) (map[string]appcontent.CustomEmojiView, error)
	// Limits returns the upload limits.
	Limits() media.Limits
}

// UploadLimit is the per-account limit of media uploads (ADR-0019 §5), on top of the
// API-wide AccountLimit.
var UploadLimit = kernel.Limit{Name: "api.media.upload", Max: 30, Window: 30 * time.Minute}

// Problem types of media files; api/openapi.yaml documents them.
const (
	ProblemTypeMediaTooLarge          = "/api/v1/problems/media-too-large"
	ProblemTypeUnsupportedMediaFormat = "/api/v1/problems/unsupported-media-format"
	ProblemTypeMediaDimensions        = "/api/v1/problems/media-dimensions"
	ProblemTypeMediaCorrupt           = "/api/v1/problems/media-corrupt"
	ProblemTypeMediaAttached          = "/api/v1/problems/media-attached"
)

// uploadPattern is the pattern of uploadMedia, whose body may be larger than other requests'.
const uploadPattern = "POST " + BasePath + "/media"

// Request body limits. A multipart upload may carry the file, at most media.Limits.MaxBytes,
// plus its part headers and the description within uploadOverhead.
const (
	maxRequestBody = 1 << 20
	uploadOverhead = 64 << 10
)

// maxDescriptionBytes bounds the bytes of a description part: four bytes per character at
// most in UTF-8.
const maxDescriptionBytes = 4 * content.MaxMediaDescriptionLength

// maxBody returns the largest request body that the operation registered with pattern
// accepts.
func (s *Server) maxBody(pattern string) int64 {
	if pattern == uploadPattern && s.o.Media != nil {
		return s.o.Media.Limits().MaxBytes + uploadOverhead
	}
	return maxRequestBody
}

// limitBody refuses a request whose declared length is over the operation's body limit with a
// 413 before reading any of it, and bounds the body of the others, so that a body sent without
// a length is cut off at the limit as well. It returns false when it has answered the request.
func (s *Server) limitBody(w http.ResponseWriter, r *http.Request, pattern string) bool {
	n := s.maxBody(pattern)
	if r.ContentLength > n {
		// The rest of the body is not read; the connection is closed instead of drained.
		w.Header().Set("Connection", "close")
		s.writeTooLarge(w, r, pattern)
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, n)
	return true
}

func (s *Server) writeTooLarge(w http.ResponseWriter, r *http.Request, pattern string) {
	l := s.localizer(r)
	if pattern == uploadPattern {
		s.writeProblem(w, r, http.StatusRequestEntityTooLarge, ProblemTypeMediaTooLarge, l.ServerErrorsMediaTooLarge(), "")
		return
	}
	s.writeProblem(w, r, http.StatusRequestEntityTooLarge, ProblemTypeBlank, l.ServerErrorsRequestTooLarge(), "")
}

// mediaProblem returns the problem of an error of the media use cases, or false for an
// unexpected error.
func (s *Server) mediaProblem(ctx context.Context, err error) (problem, bool) {
	p := s.mediaProblemOf(ctx, err)
	return p, p.StatusCode != 0
}

func (s *Server) mediaProblemOf(ctx context.Context, err error) problem {
	l := s.Localizer(ctx)
	switch {
	case errors.Is(err, media.ErrTooLarge):
		return s.problem(ctx, http.StatusRequestEntityTooLarge, ProblemTypeMediaTooLarge, l.ServerErrorsMediaTooLarge(), "")
	case errors.Is(err, media.ErrUnsupportedFormat), errors.Is(err, appcontent.ErrContentTypeMismatch):
		return s.problem(ctx, http.StatusUnsupportedMediaType, ProblemTypeUnsupportedMediaFormat, l.ServerErrorsMediaUnsupportedFormat(), "")
	case errors.Is(err, media.ErrDimensions):
		return s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeMediaDimensions, l.ServerErrorsMediaDimensions(), "")
	case errors.Is(err, media.ErrCorrupt):
		return s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeMediaCorrupt, l.ServerErrorsMediaCorrupt(), "")
	case errors.Is(err, content.ErrDescriptionTooLong):
		return s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank,
			l.ServerErrorsMediaDescriptionTooLong(), l.ServerErrorsMediaDescriptionTooLongDetail(content.MaxMediaDescriptionLength))
	case errors.Is(err, content.ErrInvalidDescription):
		return s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank, l.ServerErrorsUnprocessable(), l.ServerErrorsInvalidValue("description"))
	case errors.Is(err, content.ErrInvalidFocus):
		return s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank, l.ServerErrorsMediaInvalidFocus(), "")
	case errors.Is(err, appcontent.ErrMediaAttached):
		return s.problem(ctx, http.StatusConflict, ProblemTypeMediaAttached, l.ServerErrorsMediaAttached(), l.ServerErrorsMediaAttachedDetail())
	case errors.Is(err, appcontent.ErrNotFound):
		return s.problem(ctx, http.StatusNotFound, ProblemTypeBlank, l.ServerErrorsNotFound(), "")
	}
	return problem{}
}

// errNoMedia reports a Server without MediaFiles serving a media operation.
var errNoMedia = errors.New("clientapi: no media files configured")

// newMediaAttachment presents a media file as the MediaAttachment schema, with the
// description that the uploader set, and the variants served at url. Statuses present their
// attachments with it too, replacing the description with the revision's copy.
func newMediaAttachment(m appcontent.Media, url func(content.MediaVariant) string) MediaAttachment {
	out := MediaAttachment{Id: m.File.ID, State: MediaAttachmentState("processing"), Type: MediaAttachmentType(content.MediaTypeUnknown)}
	if m.Upload != nil && strings.HasPrefix(m.Upload.ContentType, "image/") {
		out.Type = MediaAttachmentType(content.MediaTypeImage)
	}
	if m.Description != "" {
		out.Description = &m.Description
	}
	if m.Focus != nil {
		out.Meta.Focus = MediaFocus{X: m.Focus.X, Y: m.Focus.Y}
	}
	r := m.Result
	if r == nil {
		return out
	}
	out.Type = MediaAttachmentType(r.MediaType)
	if !m.Ready() {
		out.State = MediaAttachmentState("failed")
		return out
	}
	out.State = MediaAttachmentState("ready")
	if r.Width > 0 && r.Height > 0 {
		out.Meta.Width, out.Meta.Height = &r.Width, &r.Height
	}
	if r.Blurhash != "" {
		out.Blurhash = &r.Blurhash
	}
	full, okFull := r.Variant(content.VariantFull)
	thumb, okThumb := r.Variant(content.VariantThumbnail)
	if okFull && okThumb {
		out.Variants = &MediaVariants{Full: newMediaVariant(full, url), Thumbnail: newMediaVariant(thumb, url)}
		out.Url, out.PreviewUrl = &out.Variants.Full.Url, &out.Variants.Thumbnail.Url
	}
	return out
}

// newRemoteAttachment presents a remote media file (appcontent.MediaView of a Media with Remote)
// as the MediaAttachment schema: ready, with what the remote server declared and the file served
// through the media proxy at proxyURL (federation.md §9). The instance does not process it, so it
// has no variants.
func newRemoteAttachment(m appcontent.Media, proxyURL string) MediaAttachment {
	r := m.Remote
	out := MediaAttachment{
		Id: m.File.ID, State: MediaAttachmentState("ready"), Type: MediaAttachmentType(r.DeclaredMediaType),
		Url: &proxyURL, PreviewUrl: &proxyURL, RemoteUrl: &r.RemoteURL,
	}
	if r.Width > 0 && r.Height > 0 {
		out.Meta.Width, out.Meta.Height = &r.Width, &r.Height
	}
	if r.Blurhash != "" {
		out.Blurhash = &r.Blurhash
	}
	if r.Focus != nil {
		out.Meta.Focus = MediaFocus{X: r.Focus.X, Y: r.Focus.Y}
	}
	return out
}

func newMediaVariant(v content.MediaVariant, url func(content.MediaVariant) string) MediaVariant {
	return MediaVariant{Url: url(v), ContentType: v.ContentType, Width: v.Width, Height: v.Height, Size: v.FileSize}
}

// UploadMedia serves POST /api/v1/media: it reads the multipart body part by part, never more
// than the size limit (limitBody bounds the whole body before the handler runs), and answers
// 202 with the file in its processing state.
func (s *Server) UploadMedia(ctx context.Context, req UploadMediaRequestObject) (UploadMediaResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if s.o.Media == nil {
		return nil, errNoMedia
	}
	if p, h, limited := s.LimitProblem(ctx, UploadLimit, v.AccountID); limited {
		return UploadMediadefaultApplicationProblemPlusJSONResponse{Body: p, Headers: h, StatusCode: http.StatusTooManyRequests}, nil
	}
	in, p, ok := s.readUpload(ctx, req.Body, v.AccountID)
	if !ok {
		return UploadMediadefaultApplicationProblemPlusJSONResponse(p), nil
	}
	m, err := s.o.Media.Upload(ctx, in)
	if err != nil {
		if p, ok := s.mediaProblem(ctx, err); ok {
			return UploadMediadefaultApplicationProblemPlusJSONResponse(p), nil
		}
		return nil, err
	}
	cache := cacheNoStore
	return UploadMedia202JSONResponse{Body: newMediaAttachment(m, s.o.Media.URL), Headers: UploadMedia202ResponseHeaders{CacheControl: &cache}}, nil
}

// readUpload reads the parts of an upload: the file and an optional description. Other parts
// are skipped.
func (s *Server) readUpload(ctx context.Context, body *multipart.Reader, accountID string) (appcontent.UploadInput, problem, bool) {
	l := s.Localizer(ctx)
	maxBytes := s.o.Media.Limits().MaxBytes
	in := appcontent.UploadInput{OwnerAccountID: accountID}
	var file []byte
	badPart := func(name string) problem {
		return s.problem(ctx, http.StatusBadRequest, ProblemTypeBlank, l.ServerErrorsBadRequest(), l.ServerErrorsInvalidParameter(name))
	}
	readErr := func(err error, name string) problem {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return s.mediaProblemOf(ctx, media.ErrTooLarge)
		}
		return badPart(name)
	}
	for {
		part, err := body.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return in, readErr(err, "file"), false
		}
		switch part.FormName() {
		case "file":
			if file != nil {
				return in, badPart("file"), false
			}
			data, err := io.ReadAll(io.LimitReader(part, maxBytes+1))
			if err != nil {
				return in, readErr(err, "file"), false
			}
			if int64(len(data)) > maxBytes {
				return in, s.mediaProblemOf(ctx, media.ErrTooLarge), false
			}
			file, in.DeclaredContentType = data, part.Header.Get("Content-Type")
		case "description":
			data, err := io.ReadAll(io.LimitReader(part, maxDescriptionBytes+1))
			if err != nil {
				return in, readErr(err, "description"), false
			}
			if len(data) > maxDescriptionBytes {
				return in, s.mediaProblemOf(ctx, fmt.Errorf("%w: %d bytes", content.ErrDescriptionTooLong, len(data))), false
			}
			in.Description = string(data)
		}
	}
	if file == nil {
		return in, badPart("file"), false
	}
	in.Content = bytes.NewReader(file)
	return in, problem{}, true
}

// GetMedia serves GET /api/v1/media/{id}: a media file of the viewer with its state.
func (s *Server) GetMedia(ctx context.Context, req GetMediaRequestObject) (GetMediaResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if s.o.Media == nil {
		return nil, errNoMedia
	}
	m, err := s.o.Media.OwnMedia(ctx, v.AccountID, req.Id)
	if err != nil {
		if p, ok := s.mediaProblem(ctx, err); ok {
			return GetMediadefaultApplicationProblemPlusJSONResponse(p), nil
		}
		return nil, err
	}
	cache := cacheNoStore
	return GetMedia200JSONResponse{Body: newMediaAttachment(m, s.o.Media.URL), Headers: GetMedia200ResponseHeaders{CacheControl: &cache}}, nil
}

// UpdateMedia serves PATCH /api/v1/media/{id}: it changes the description or focal point of
// an unattached media file of the viewer.
func (s *Server) UpdateMedia(ctx context.Context, req UpdateMediaRequestObject) (UpdateMediaResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if s.o.Media == nil {
		return nil, errNoMedia
	}
	u := appcontent.MediaUpdate{MediaID: req.Id, Description: req.Body.Description}
	if f := req.Body.Focus; f != nil {
		u.Focus = &content.MediaFocus{X: f.X, Y: f.Y}
	}
	m, err := s.o.Media.Update(ctx, v.AccountID, u)
	if err != nil {
		if p, ok := s.mediaProblem(ctx, err); ok {
			return UpdateMediadefaultApplicationProblemPlusJSONResponse(p), nil
		}
		return nil, err
	}
	cache := cacheNoStore
	return UpdateMedia200JSONResponse{Body: newMediaAttachment(m, s.o.Media.URL), Headers: UpdateMedia200ResponseHeaders{CacheControl: &cache}}, nil
}
