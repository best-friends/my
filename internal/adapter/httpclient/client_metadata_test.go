// SPDX-License-Identifier: AGPL-3.0-only

package httpclient

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mycroblog/my/internal/app/kernel"
)

// TestClientMetadataPurpose checks the fetch of OAuth Client ID Metadata Documents (ADR-0090
// §1): the guard refuses internal addresses, a redirect is answered as is and never followed,
// and a body over 5 KB fails.
func TestClientMetadataPurpose(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/moved":
			http.Redirect(w, r, "/client.json", http.StatusFound)
		case "/large":
			_, _ = w.Write([]byte(strings.Repeat("x", 5<<10+1)))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	get := func(c *Client, path string) (*kernel.HTTPResponse, error) {
		return c.Do(t.Context(), &kernel.HTTPRequest{URL: srv.URL + path, Purpose: kernel.HTTPPurposeClientMetadata})
	}
	if _, err := get(New(Options{AllowHTTP: true, AnyPort: true}), "/client.json"); !errors.Is(err, kernel.ErrForbiddenDestination) {
		t.Errorf("Do(loopback) = %v, want ErrForbiddenDestination", err)
	}
	open := New(Options{AllowHTTP: true, AnyPort: true, AllowPrivateNetworks: true})
	resp, err := get(open, "/moved")
	if err != nil {
		t.Fatalf("Do(redirect) = %v", err)
	}
	_, _ = resp.ReadAll()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want the 302 itself", resp.StatusCode)
	}
	resp, err = get(open, "/large")
	if err == nil {
		_, err = resp.ReadAll()
	}
	if !errors.Is(err, kernel.ErrResponseTooLarge) {
		t.Errorf("Do(over 5 KB) = %v, want ErrResponseTooLarge", err)
	}
}
