// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"github.com/best-friends/my/internal/interface/oauth"
	"github.com/best-friends/my/internal/platform/config"
)

// setUpOAuth registers the OAuth token endpoints.
func (a *App) setUpOAuth(cfg *config.Config) error {
	tokens, err := oauth.New(oauth.Options{Identity: a.Identity, WebOrigin: cfg.WebOrigin, Logger: a.Logger})
	if err != nil {
		return err
	}
	tokens.Register(surfaceRouter{a, surfaceOAuth})
	return nil
}
