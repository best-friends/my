// SPDX-License-Identifier: AGPL-3.0-only

package federation_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	appfederation "github.com/mycroblog/my/internal/app/federation"
	"github.com/mycroblog/my/internal/app/kernel"
)

// TestRejectionReason checks that a refusal is named from the fixed vocabulary, never with the
// header values that its error quotes.
func TestRejectionReason(t *testing.T) {
	const header = `keyId="https://evil.example/secret-value"`
	quoted := func(sentinel error) error { return fmt.Errorf("%w: %s", sentinel, header) }
	sigErr := func(status int, err error) *appfederation.SignatureError {
		return &appfederation.SignatureError{Status: status, Err: err}
	}
	for _, tc := range []struct {
		name string
		rej  *appfederation.InboxRejection
		want string
	}{
		{"no signature", &appfederation.InboxRejection{Status: http.StatusUnauthorized, Err: sigErr(http.StatusUnauthorized, quoted(appfederation.ErrNoSignature))}, appfederation.ReasonNoSignature},
		{"malformed", &appfederation.InboxRejection{Status: http.StatusBadRequest, Err: sigErr(http.StatusBadRequest, quoted(appfederation.ErrMalformedSignature))}, appfederation.ReasonMalformedSignature},
		{"malformed without the sentinel", &appfederation.InboxRejection{Status: http.StatusBadRequest, Err: sigErr(http.StatusBadRequest, errors.New(header))}, appfederation.ReasonMalformedSignature},
		{"key not found", &appfederation.InboxRejection{Status: http.StatusUnauthorized, Err: sigErr(http.StatusUnauthorized, quoted(appfederation.ErrPublicKeyNotFound))}, appfederation.ReasonPublicKeyNotFound},
		{"key unavailable", &appfederation.InboxRejection{Status: http.StatusServiceUnavailable, Err: sigErr(http.StatusServiceUnavailable, quoted(appfederation.ErrPublicKeyUnavailable))}, appfederation.ReasonPublicKeyUnavailable},
		{"mismatch", &appfederation.InboxRejection{Status: http.StatusUnauthorized, Err: sigErr(http.StatusUnauthorized, quoted(appfederation.ErrSignatureMismatch))}, appfederation.ReasonSignatureMismatch},
		{"policy", &appfederation.InboxRejection{Status: http.StatusUnauthorized, Err: sigErr(http.StatusUnauthorized, errors.New("unparsable Date header: "+header))}, appfederation.ReasonSignaturePolicy},
		{"rate limited", &appfederation.InboxRejection{Status: http.StatusTooManyRequests, Err: &kernel.RateLimitedError{}}, appfederation.ReasonRateLimited},
		{"suspended", &appfederation.InboxRejection{Status: http.StatusForbidden, Err: errors.New("evil.example is suspended")}, appfederation.ReasonDomainSuspended},
		{"invalid activity", &appfederation.InboxRejection{Status: http.StatusBadRequest, Err: errors.New(header)}, appfederation.ReasonInvalidActivity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.rej.Reason(); got != tc.want {
				t.Fatalf("Reason() = %q, want %q", got, tc.want)
			}
		})
	}
}
