// SPDX-License-Identifier: AGPL-3.0-only

//go:build no_billing_aws

package platform

import (
	"context"

	"github.com/best-friends/my/internal/app/finance"
	"github.com/best-friends/my/internal/platform/config"
)

func init() { billingSourceAdapters["aws"] = newAWSBillingSource }

// newAWSBillingSource reports that the build tag no_billing_aws left the AWS billing adapter out.
func newAWSBillingSource(context.Context, *config.Config) (finance.BillingSource, error) {
	return nil, config.ExcludedError("billing_sources", "aws", "no_billing_aws")
}
