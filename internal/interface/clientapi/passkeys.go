// SPDX-License-Identifier: AGPL-3.0-only

package clientapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	appidentity "github.com/best-friends/my/internal/app/identity"
	"github.com/best-friends/my/internal/app/kernel"
	"github.com/best-friends/my/internal/domain/identity"
)

// The viewer's passkeys (FR-004): registering one after the password, listing, and removing
// them. The identity module runs the WebAuthn ceremonies; signing in with a passkey is the
// server-rendered sign-in page's.

// Problem types of passkeys.
const (
	// ProblemTypeInvalidPasskey is the problem of a registration response that does not verify.
	ProblemTypeInvalidPasskey = "/api/v1/problems/invalid-passkey"
	// ProblemTypePasskeyRegistrationExpired is the problem of an unknown or expired registration.
	ProblemTypePasskeyRegistrationExpired = "/api/v1/problems/passkey-registration-expired"
	// ProblemTypeTooManyPasskeys is the problem of a registration by a user with the most
	// passkeys allowed.
	ProblemTypeTooManyPasskeys = "/api/v1/problems/too-many-passkeys"
)

// PasskeySettings runs the viewer's passkeys. *appidentity.Service implements it.
type PasskeySettings interface {
	Passkeys(ctx context.Context, userID string) ([]identity.PasskeyCredential, error)
	BeginPasskeyRegistration(ctx context.Context, userID, name, password string) (appidentity.PasskeyRegistration, error)
	FinishPasskeyRegistration(ctx context.Context, userID, token string, response []byte) (identity.PasskeyCredential, error)
	RemovePasskey(ctx context.Context, userID, passkeyID string) error
}

func newPasskey(c identity.PasskeyCredential) Passkey {
	return Passkey{Id: c.ID, Name: c.Name, CreatedAt: c.CreatedAt.UTC()}
}

// passkeyProblem returns the problem of an error of the identity module's passkey use cases,
// or nil for another error.
func (s *Server) passkeyProblem(ctx context.Context, err error) *problem {
	l := s.Localizer(ctx)
	var limited *kernel.RateLimitedError
	var p problem
	switch {
	case errors.As(err, &limited):
		retry := seconds(limited.RetryAfter)
		p = s.problem(ctx, http.StatusTooManyRequests, ProblemTypeBlank, l.ServerErrorsRateLimited(), l.ServerErrorsRateLimitedDetail(retry))
		p.Headers.RetryAfter = &retry
	case errors.Is(err, identity.ErrInvalidPasskeyName):
		p = s.invalidParameterResponse(ctx, "name")
	case errors.Is(err, appidentity.ErrWrongPassword):
		p = s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeWrongPassword, l.ServerErrorsWrongPassword(), "")
	case errors.Is(err, appidentity.ErrInvalidPasskey):
		p = s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypeInvalidPasskey, l.ServerErrorsPasskeyInvalid(), "")
	case errors.Is(err, appidentity.ErrPasskeyCeremonyExpired):
		p = s.problem(ctx, http.StatusUnprocessableEntity, ProblemTypePasskeyRegistrationExpired, l.ServerErrorsPasskeyRegistrationExpired(), "")
	case errors.Is(err, appidentity.ErrTooManyPasskeys):
		p = s.problem(ctx, http.StatusConflict, ProblemTypeTooManyPasskeys, l.ServerErrorsPasskeyTooMany(), "")
	default:
		return nil
	}
	return &p
}

// ListPasskeys serves GET /api/v1/passkeys.
func (s *Server) ListPasskeys(ctx context.Context, _ ListPasskeysRequestObject) (ListPasskeysResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if s.o.Passkeys == nil {
		return ListPasskeysdefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	}
	list, err := s.o.Passkeys.Passkeys(ctx, v.UserID)
	if err != nil {
		return nil, fmt.Errorf("clientapi: the passkeys of %s: %w", v.UserID, err)
	}
	items := make([]Passkey, len(list))
	for i, c := range list {
		items[i] = newPasskey(c)
	}
	cache := cacheNoStore
	return ListPasskeys200JSONResponse{Body: PasskeyList{Items: items}, Headers: ListPasskeys200ResponseHeaders{CacheControl: &cache}}, nil
}

// BeginPasskeyRegistration serves POST /api/v1/passkeys/registration.
func (s *Server) BeginPasskeyRegistration(ctx context.Context, req BeginPasskeyRegistrationRequestObject) (BeginPasskeyRegistrationResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if s.o.Passkeys == nil {
		return BeginPasskeyRegistrationdefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	}
	if req.Body == nil || req.Body.Password == "" {
		return BeginPasskeyRegistrationdefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "password")), nil
	}
	reg, err := s.o.Passkeys.BeginPasskeyRegistration(ctx, v.UserID, req.Body.Name, req.Body.Password)
	if p := s.passkeyProblem(ctx, err); p != nil {
		return BeginPasskeyRegistrationdefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: begin a passkey registration for %s: %w", v.UserID, err)
	}
	var options map[string]any
	if err := json.Unmarshal(reg.Options, &options); err != nil {
		return nil, fmt.Errorf("clientapi: the options of a passkey registration: %w", err)
	}
	cache := cacheNoStore
	return BeginPasskeyRegistration200JSONResponse{
		Body:    PasskeyRegistration{Options: options, RegistrationToken: reg.Token, ExpiresAt: reg.ExpiresAt.UTC()},
		Headers: BeginPasskeyRegistration200ResponseHeaders{CacheControl: &cache},
	}, nil
}

// CreatePasskey serves POST /api/v1/passkeys.
func (s *Server) CreatePasskey(ctx context.Context, req CreatePasskeyRequestObject) (CreatePasskeyResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if s.o.Passkeys == nil {
		return CreatePasskeydefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	}
	if req.Body == nil || len(req.Body.Credential) == 0 {
		return CreatePasskeydefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "credential")), nil
	}
	response, err := json.Marshal(req.Body.Credential)
	if err != nil {
		return CreatePasskeydefaultApplicationProblemPlusJSONResponse(s.invalidParameterResponse(ctx, "credential")), nil
	}
	c, err := s.o.Passkeys.FinishPasskeyRegistration(ctx, v.UserID, req.Body.RegistrationToken, response)
	if p := s.passkeyProblem(ctx, err); p != nil {
		return CreatePasskeydefaultApplicationProblemPlusJSONResponse(*p), nil
	}
	if err != nil {
		return nil, fmt.Errorf("clientapi: register a passkey for %s: %w", v.UserID, err)
	}
	return CreatePasskey201JSONResponse(newPasskey(c)), nil
}

// DeletePasskey serves DELETE /api/v1/passkeys/{id}.
func (s *Server) DeletePasskey(ctx context.Context, req DeletePasskeyRequestObject) (DeletePasskeyResponseObject, error) {
	v, err := requireViewer(ctx)
	if err != nil {
		return nil, err
	}
	if s.o.Passkeys == nil {
		return DeletePasskeydefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	}
	switch err := s.o.Passkeys.RemovePasskey(ctx, v.UserID, req.Id); {
	case errors.Is(err, appidentity.ErrNotFound):
		return DeletePasskeydefaultApplicationProblemPlusJSONResponse(s.notFoundResponse(ctx)), nil
	case err != nil:
		return nil, fmt.Errorf("clientapi: remove passkey %s of %s: %w", req.Id, v.UserID, err)
	}
	return DeletePasskey204Response{}, nil
}
