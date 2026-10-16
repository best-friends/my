// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/best-friends/my/internal/i18n"
)

// ContentTypeProblem is the media type of RFC 9457 problem documents.
const ContentTypeProblem = "application/problem+json"

// ProblemTypeBlank is the problem type of problems that mean no more than their HTTP status.
const ProblemTypeBlank = "about:blank"

type localizerKey struct{}

// localize stores the localizer of the request's Accept-Language in its context, where the
// strict handlers, which see only the context, find it with Localizer.
func (s *Server) localize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), localizerKey{}, s.localizer(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) localizer(r *http.Request) *i18n.Localizer {
	if l, ok := r.Context().Value(localizerKey{}).(*i18n.Localizer); ok {
		return l
	}
	b := s.o.Messages
	return b.Localizer(b.Resolve(i18n.Preferences{AcceptLanguage: r.Header.Get("Accept-Language")}))
}

// Localizer returns the localizer of the request that ctx belongs to, for the messages of
// problem documents that a handler returns.
func (s *Server) Localizer(ctx context.Context) *i18n.Localizer {
	if l, ok := ctx.Value(localizerKey{}).(*i18n.Localizer); ok {
		return l
	}
	b := s.o.Messages
	return b.Localizer(b.Resolve(i18n.Preferences{}))
}

// NewProblem returns a problem document of status with the localized title and optional
// detail, and the response headers that go with it. Handlers return it as the body of their
// operation's default response, for example
//
//	body, headers := s.NewProblem(ctx, http.StatusNotFound, ProblemTypeBlank, l.ServerErrorsNotFound(), "")
//	return GetStatusdefaultApplicationProblemPlusJSONResponse{Body: body, Headers: headers, StatusCode: http.StatusNotFound}, nil
func (s *Server) NewProblem(ctx context.Context, status int, typ, title, detail string) (Problem, ProblemResponseHeaders) {
	p := Problem{Type: typ, Title: title, Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	if id := s.o.RequestID(ctx); id != "" {
		p.RequestId = &id
	}
	locale := s.Localizer(ctx).Locale()
	return p, ProblemResponseHeaders{ContentLanguage: &locale}
}

// writeProblem writes a problem document outside the strict handlers: for requests that the
// generated code rejects and for errors that handlers return. Problems are never cached.
func (s *Server) writeProblem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	ctx := context.WithValue(r.Context(), localizerKey{}, s.localizer(r))
	p, headers := s.NewProblem(ctx, status, ProblemTypeBlank, title, detail)
	body, err := json.Marshal(p)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	h := w.Header()
	h.Set("Content-Type", ContentTypeProblem)
	h.Set("Content-Language", *headers.ContentLanguage)
	h.Set("Cache-Control", "no-store")
	h.Add("Vary", "Accept-Language")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

// parameterName returns the name of the parameter that a generated request error is about,
// or "".
func parameterName(err error) string {
	var (
		invalid     *InvalidParamFormatError
		required    *RequiredParamError
		header      *RequiredHeaderError
		unmarshal   *UnmarshalingParamError
		tooMany     *TooManyValuesForParamError
		cookieParam *UnescapedCookieParamError
	)
	switch {
	case errors.As(err, &invalid):
		return invalid.ParamName
	case errors.As(err, &required):
		return required.ParamName
	case errors.As(err, &header):
		return header.ParamName
	case errors.As(err, &unmarshal):
		return unmarshal.ParamName
	case errors.As(err, &tooMany):
		return tooMany.ParamName
	case errors.As(err, &cookieParam):
		return cookieParam.ParamName
	}
	return ""
}

// requestError answers a request that the generated code cannot parse with a 400 problem;
// the detail names the parameter when the error is about one.
func (s *Server) requestError(w http.ResponseWriter, r *http.Request, err error) {
	l := s.localizer(r)
	var detail string
	if name := parameterName(err); name != "" {
		detail = l.ServerErrorsInvalidParameter(name)
	}
	s.writeProblem(w, r, http.StatusBadRequest, l.ServerErrorsBadRequest(), detail)
}

// responseError answers an error that a handler returned, or a response that cannot be
// written, with a 500 problem that reveals nothing about the error.
func (s *Server) responseError(w http.ResponseWriter, r *http.Request, err error) {
	s.o.Logger.ErrorContext(r.Context(), "client api handler failed", slog.Any("error", err))
	l := s.localizer(r)
	s.writeProblem(w, r, http.StatusInternalServerError, l.ServerErrorsInternal(), "")
}
