// SPDX-License-Identifier: AGPL-3.0-only

package clientapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/best-friends/my/internal/i18n"
	"github.com/best-friends/my/internal/interface/clientapi"
	"github.com/best-friends/my/internal/interface/clientapi/apitest"
)

func newServer(t *testing.T) (*clientapi.Server, http.Handler) {
	t.Helper()
	return newServerWithLocales(t, func(b *i18n.Bundle) *i18n.Resolver { return &i18n.Resolver{Bundle: b} })
}

func newServerWithLocales(t *testing.T, locales func(*i18n.Bundle) *i18n.Resolver) (*clientapi.Server, http.Handler) {
	t.Helper()
	messages, err := i18n.Load()
	if err != nil {
		t.Fatal(err)
	}
	s := clientapi.New(clientapi.Options{
		Host: "social.example", Name: "Example Social", SoftwareVersion: "1.2.3",
		SourceURL: "https://git.example/my", MaxPostCharacters: 500,
		Locales:   locales(messages),
		RequestID: func(context.Context) string { return "req-1" },
		Logger:    slog.New(slog.DiscardHandler),
	})
	mux := http.NewServeMux()
	s.Register(mux)
	return s, mux
}

func TestGetInstance(t *testing.T) {
	_, h := newServer(t)
	spec := apitest.Load(t)

	res := spec.Serve(t, h, httptest.NewRequest(http.MethodGet, clientapi.BasePath+"/instance", nil))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.StatusCode)
	}
	var got clientapi.Instance
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	want := clientapi.Instance{
		Domain: "social.example", Name: "Example Social", Version: "1.2.3", SourceUrl: "https://git.example/my",
		Limits: clientapi.InstanceLimits{MaxPostCharacters: 500},
	}
	if got != want {
		t.Errorf("instance = %+v, want %+v", got, want)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "public, max-age=60" {
		t.Errorf("Cache-Control = %q, want public, max-age=60", cc)
	}
}

// TestProblemsAreLocalized checks the problem documents that the generated code writes for
// unparsable requests and failed handlers: they conform to the spec and are localized from
// Accept-Language.
func TestProblemsAreLocalized(t *testing.T) {
	s, _ := newServer(t)
	spec := apitest.Load(t)
	tests := []struct {
		name, acceptLanguage, wantLanguage string
		write                              func(w http.ResponseWriter, r *http.Request)
		wantStatus                         int
		wantTitle, wantDetail              string
	}{
		{
			name: "invalid parameter in en", acceptLanguage: "en-US,en;q=0.9", wantLanguage: "en",
			write: func(w http.ResponseWriter, r *http.Request) {
				clientapi.RequestErrorHandler(s)(w, r, &clientapi.InvalidParamFormatError{ParamName: "limit"})
			},
			wantStatus: http.StatusBadRequest,
			wantTitle:  "The request is invalid.", wantDetail: "The parameter limit has an invalid value.",
		},
		{
			name: "invalid parameter in ja", acceptLanguage: "ja", wantLanguage: "ja",
			write: func(w http.ResponseWriter, r *http.Request) {
				clientapi.RequestErrorHandler(s)(w, r, &clientapi.RequiredParamError{ParamName: "cursor"})
			},
			wantStatus: http.StatusBadRequest,
			wantTitle:  "リクエストが正しくありません。", wantDetail: "パラメーター cursor の値が正しくありません。",
		},
		{
			name: "handler error in ja", acceptLanguage: "ja-JP", wantLanguage: "ja",
			write: func(w http.ResponseWriter, r *http.Request) {
				clientapi.ResponseErrorHandler(s)(w, r, context.DeadlineExceeded)
			},
			wantStatus: http.StatusInternalServerError,
			wantTitle:  "予期しないエラーが発生しました。しばらくしてからもう一度お試しください。",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, clientapi.BasePath+"/instance", nil)
			req.Header.Set("Accept-Language", tt.acceptLanguage)
			res := spec.Serve(t, http.HandlerFunc(tt.write), req)
			if res.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", res.StatusCode, tt.wantStatus)
			}
			if got := res.Header.Get("Content-Language"); got != tt.wantLanguage {
				t.Errorf("Content-Language = %q, want %q", got, tt.wantLanguage)
			}
			var p clientapi.Problem
			if err := json.NewDecoder(res.Body).Decode(&p); err != nil {
				t.Fatal(err)
			}
			var detail string
			if p.Detail != nil {
				detail = *p.Detail
			}
			if p.Type != clientapi.ProblemTypeBlank || p.Status != tt.wantStatus || p.Title != tt.wantTitle || detail != tt.wantDetail {
				t.Errorf("problem = %+v (detail %q), want title %q and detail %q", p, detail, tt.wantTitle, tt.wantDetail)
			}
			if p.RequestId == nil || *p.RequestId != "req-1" {
				t.Errorf("request_id = %v, want req-1", p.RequestId)
			}
		})
	}
}

// viewerKey is the context key of a fake authenticated viewer: its saved locale.
type viewerKey struct{}

// TestProblemLocaleOrder checks that problem documents follow ADR-0009 §2: the
// authenticated user's locale, then Accept-Language, then the instance default.
func TestProblemLocaleOrder(t *testing.T) {
	instanceDefault := "ja"
	s, _ := newServerWithLocales(t, func(b *i18n.Bundle) *i18n.Resolver {
		return &i18n.Resolver{
			Bundle:          b,
			UserLocale:      func(ctx context.Context) string { l, _ := ctx.Value(viewerKey{}).(string); return l },
			InstanceDefault: func(context.Context) string { return instanceDefault },
		}
	})
	tests := []struct {
		name, viewer, acceptLanguage, instanceDefault, want string
	}{
		{name: "anonymous without accept-language", instanceDefault: "ja", want: "ja"},
		{name: "default changed", instanceDefault: "en", want: "en"},
		{name: "accept-language before the default", acceptLanguage: "en", instanceDefault: "ja", want: "en"},
		{name: "user before accept-language", viewer: "ja", acceptLanguage: "en", instanceDefault: "en", want: "ja"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instanceDefault = tt.instanceDefault
			req := httptest.NewRequest(http.MethodGet, clientapi.BasePath+"/instance", nil)
			if tt.acceptLanguage != "" {
				req.Header.Set("Accept-Language", tt.acceptLanguage)
			}
			if tt.viewer != "" {
				req = req.WithContext(context.WithValue(req.Context(), viewerKey{}, tt.viewer))
			}
			w := httptest.NewRecorder()
			clientapi.RequestErrorHandler(s)(w, req, &clientapi.RequiredParamError{ParamName: "cursor"})
			if got := w.Header().Get("Content-Language"); got != tt.want {
				t.Errorf("Content-Language = %q, want %q", got, tt.want)
			}
		})
	}
}
