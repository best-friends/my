// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"

	"github.com/mycroblog/my/internal/app/federation"
	domainfederation "github.com/mycroblog/my/internal/domain/federation"
)

// federationModes reads the federation mode (FR-413, ADR-0076) from the instance setting
// federation_mode, per request and per job, for every entry point of federation. It reads
// a.Instance per call, so that it can be wired before the instance service is.
type federationModes struct{ a *App }

// FederationMode implements federation.ModeSettings.
func (m federationModes) FederationMode(ctx context.Context) (domainfederation.Mode, error) {
	v, err := m.a.Instance.FederationMode(ctx)
	return domainfederation.Mode(v), err
}

// graphFollowers tells the media proxy the remote followers of local accounts through the graph
// module, which it reads per call, since the media proxy is built before it.
type graphFollowers struct{ a *App }

// FollowsAnyone implements federation.FollowerReader.
func (g graphFollowers) FollowsAnyone(ctx context.Context, accountID string) (bool, error) {
	return g.a.Graph.FollowsAnyone(ctx, accountID)
}

// mediaGate is the federation mode's gate of the media proxy.
func (a *App) mediaGate() federation.MediaGate {
	return federation.MediaGate{Modes: federationModes{a}, Followers: graphFollowers{a}}
}

// setUpFederationSever builds the severing of the follows with remote accounts that `my admin
// federation sever` runs (ADR-0076).
func (a *App) setUpFederationSever() {
	a.FederationSever = federation.NewSeverance(federation.SeveranceDeps{
		UnitOfWork: a.UnitOfWork, Graph: a.Graph, Accounts: a.Accounts, Modes: federationModes{a}, Clock: a.Clock,
	})
}
