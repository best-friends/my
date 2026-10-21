// SPDX-License-Identifier: AGPL-3.0-only

package aws

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// RoleSessionName is the session name of the role that billing_aws_role_arn assumes, in the
// role's CloudTrail events.
const RoleSessionName = "my-billing-import"

// identityTimeout bounds a read of the ID token from the metadata server.
const identityTimeout = 10 * time.Second

// MetadataClient is the part of *metadata.Client that GoogleIDToken uses.
type MetadataClient interface {
	GetWithContext(ctx context.Context, suffix string) (string, error)
}

// GoogleIDToken is the stscreds.IdentityTokenRetriever of a process on Google Cloud (Cloud Run):
// the ID token of the service account of the instance, from the metadata server, for Audience.
// AWS trusts it through the identity provider accounts.google.com (ADR-0068 §8).
type GoogleIDToken struct {
	Client   MetadataClient
	Audience string
}

// GetIdentityToken implements stscreds.IdentityTokenRetriever.
func (g GoogleIDToken) GetIdentityToken() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), identityTimeout)
	defer cancel()
	token, err := g.Client.GetWithContext(ctx, "instance/service-accounts/default/identity?audience="+url.QueryEscape(g.Audience)+"&format=full")
	if err != nil {
		return nil, err
	}
	if token = strings.TrimSpace(token); token == "" {
		return nil, errors.New("billing aws: the metadata server returned an empty ID token")
	}
	return []byte(token), nil
}

// WithWebIdentityRole returns cfg whose credentials come from AssumeRoleWithWebIdentity of
// roleARN with the Google ID token of the instance (audience: roleARN), refreshed before they
// expire. The SDK sends AssumeRoleWithWebIdentity unsigned: it needs no other credentials.
func WithWebIdentityRole(cfg awssdk.Config, roleARN string) awssdk.Config {
	return withWebIdentityRole(cfg, roleARN, metadata.NewClient(nil))
}

func withWebIdentityRole(cfg awssdk.Config, roleARN string, md MetadataClient) awssdk.Config {
	out := cfg.Copy()
	client := sts.NewFromConfig(cfg, func(o *sts.Options) {
		if o.Region == "" {
			o.Region = Region
		}
	})
	provider := stscreds.NewWebIdentityRoleProvider(client, roleARN, GoogleIDToken{Client: md, Audience: roleARN},
		func(o *stscreds.WebIdentityRoleOptions) { o.RoleSessionName = RoleSessionName })
	out.Credentials = awssdk.NewCredentialsCache(provider)
	return out
}
