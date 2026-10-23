// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"net"
	"strings"

	"github.com/mycroblog/my/internal/adapter/webauthn"
	"github.com/mycroblog/my/internal/platform/config"
)

// newPasskeys returns the WebAuthn ceremonies of the instance's passkeys (FR-004). The relying
// party is the host name of the instance (host without its port), which the browser checks the
// page's origin against; the sign-in pages run at the server's origin and the settings at the
// web client's, so both may run the ceremonies. A web client on an origin outside the host
// name cannot register passkeys: the browser refuses.
func newPasskeys(cfg *config.Config) (*webauthn.Ceremonies, error) {
	rpID := cfg.Host
	if h, _, err := net.SplitHostPort(rpID); err == nil {
		rpID = h
	}
	origins := []string{cfg.ServerOrigin()}
	if cfg.WebOrigin != "" && !strings.EqualFold(cfg.WebOrigin, origins[0]) {
		origins = append(origins, cfg.WebOrigin)
	}
	return webauthn.New(webauthn.Options{RPID: rpID, DisplayName: cfg.Host, Origins: origins})
}
