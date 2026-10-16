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

// problem is a problem document that a handler returns as its operation's default response.
type problem struct {
	body    Problem
	headers ProblemResponseHeaders
	status  int
}

func (s *Server) problem(ctx context.Context, status int, typ, title, detail string) *problem {
	body, headers := s.NewProblem(ctx, status, typ, title, detail)
	return &problem{body: body, headers: headers, status: status}
}

// mediaProblem returns the problem of an error of the media use cases, or nil for an
// unexpected error.
func (s *Server) mediaProblem(ctx context.Context, err error) *problem {
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
	case errors.Is(err, content.ErrInvalidFocus):
		return s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeBlank, l.ServerErrorsMediaInvalidFocus(), "")
	case errors.Is(err, appcontent.ErrMediaAttached):
		return s.problem(ctx, http.StatusConflict, ProblemTypeMediaAttached, l.ServerErrorsMediaAttached(), l.ServerErrorsMediaAttachedDetail())
	case errors.Is(err, appcontent.ErrNotFound):
		return s.problem(ctx, http.StatusNotFound, ProblemTypeBlank, l.ServerErrorsNotFound(), "")
	}
	return nil
}

// errNoMedia reports a Server without MediaFiles serving a media operation.
var errNoMedia = errors.New("clientapi: no media files configured")

// newMediaAttachment presents a media file as the MediaAttachment schema, with the
// description that the uploader set.
func (s *Server) newMediaAttachment(m appcontent.Media) MediaAttachment {
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
		out.Variants = &MediaVariants{Full: s.newMediaVariant(full), Thumbnail: s.newMediaVariant(thumb)}
	}
	return out
}

func (s *Server) newMediaVariant(v content.MediaVariant) MediaVariant {
	return MediaVariant{Url: s.o.Media.URL(v), ContentType: v.ContentType, Width: v.Width, Height: v.Height, Size: v.FileSize}
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
	fail := func(p *problem) (UploadMediaResponseObject, error) {
		return UploadMediadefaultApplicationProblemPlusJSONResponse{Body: p.body, Headers: p.headers, StatusCode: p.status}, nil
	}
	if p, h, limited := s.LimitProblem(ctx, UploadLimit, v.AccountID); limited {
		return fail(&problem{body: p, headers: h, status: http.StatusTooManyRequests})
	}
	in, p := s.readUpload(ctx, req.Body, v.AccountID)
	if p != nil {
		return fail(p)
	}
	m, err := s.o.Media.Upload(ctx, in)
	if err != nil {
		if p := s.mediaProblem(ctx, err); p != nil {
			return fail(p)
		}
		return nil, err
	}
	cache := cacheNoStore
	return UploadMedia202JSONResponse{Body: s.newMediaAttachment(m), Headers: UploadMedia202ResponseHeaders{CacheControl: &cache}}, nil
}

// readUpload reads the parts of an upload: the file and an optional description. Other parts
// are skipped.
func (s *Server) readUpload(ctx context.Context, body *multipart.Reader, accountID string) (appcontent.UploadInput, *problem) {
	l := s.Localizer(ctx)
	maxBytes := s.o.Media.Limits().MaxBytes
	in := appcontent.UploadInput{OwnerAccountID: accountID}
	var file []byte
	badPart := func(name string) *problem {
		return s.problem(ctx, http.StatusBadRequest, ProblemTypeBlank, l.ServerErrorsBadRequest(), l.ServerErrorsInvalidParameter(name))
	}
	readErr := func(err error, name string) *problem {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return s.mediaProblem(ctx, media.ErrTooLarge)
		}
		return badPart(name)
	}
	for {
		part, err := body.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return in, readErr(err, "file")
		}
		switch part.FormName() {
		case "file":
			if file != nil {
				return in, badPart("file")
			}
			data, err := io.ReadAll(io.LimitReader(part, maxBytes+1))
			if err != nil {
				return in, readErr(err, "file")
			}
			if int64(len(data)) > maxBytes {
				return in, s.mediaProblem(ctx, media.ErrTooLarge)
			}
			file, in.DeclaredContentType = data, part.Header.Get("Content-Type")
		case "description":
			data, err := io.ReadAll(io.LimitReader(part, maxDescriptionBytes+1))
			if err != nil {
				return in, readErr(err, "description")
			}
			if len(data) > maxDescriptionBytes {
				return in, s.mediaProblem(ctx, fmt.Errorf("%w: %d bytes", content.ErrDescriptionTooLong, len(data)))
			}
			in.Description = string(data)
		}
	}
	if file == nil {
		return in, badPart("file")
	}
	in.Content = bytes.NewReader(file)
	return in, nil
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
		if p := s.mediaProblem(ctx, err); p != nil {
			return GetMediadefaultApplicationProblemPlusJSONResponse{Body: p.body, Headers: p.headers, StatusCode: p.status}, nil
		}
		return nil, err
	}
	cache := cacheNoStore
	return GetMedia200JSONResponse{Body: s.newMediaAttachment(m), Headers: GetMedia200ResponseHeaders{CacheControl: &cache}}, nil
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
		if p := s.mediaProblem(ctx, err); p != nil {
			return UpdateMediadefaultApplicationProblemPlusJSONResponse{Body: p.body, Headers: p.headers, StatusCode: p.status}, nil
		}
		return nil, err
	}
	cache := cacheNoStore
	return UpdateMedia200JSONResponse{Body: s.newMediaAttachment(m), Headers: UpdateMedia200ResponseHeaders{CacheControl: &cache}}, nil
}
