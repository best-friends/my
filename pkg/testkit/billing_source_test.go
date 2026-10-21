// SPDX-License-Identifier: AGPL-3.0-only

package testkit_test

import (
	"fmt"
	"testing"

	appfinance "github.com/best-friends/my/internal/app/finance"
	"github.com/best-friends/my/pkg/testkit"
)

// fakeFailures are the provider errors of the fake, wrapped like an adapter wraps its provider's.
var fakeFailures = map[appfinance.ImportErrorKind]error{
	appfinance.ImportErrorPermission:      appfinance.ErrBillingPermission,
	appfinance.ImportErrorNotEnabled:      appfinance.ErrBillingNotEnabled,
	appfinance.ImportErrorNotFound:        appfinance.ErrBillingNotFound,
	appfinance.ImportErrorThrottled:       appfinance.ErrBillingThrottled,
	appfinance.ImportErrorBytesCap:        appfinance.ErrBytesCap,
	appfinance.ImportErrorInvalidResponse: appfinance.ErrInvalidBillingResponse,
	appfinance.ImportErrorUnavailable:     appfinance.ErrBillingUnavailable,
}

func TestFakeBillingSourceContract(t *testing.T) {
	testkit.BillingSourceContract(t, func(_ *testing.T, s testkit.BillingScenario) appfinance.BillingSource {
		src := testkit.NewFakeBillingSource("aws", s.Location)
		for _, m := range s.Months {
			src.SetMonth(m.Month, testkit.FakeBillingMonth{Final: m.Final, Through: m.Through, Rows: m.Rows})
		}
		src.SetPages(s.Pages)
		if s.Failure != "" {
			src.SetFailure(fmt.Errorf("fake provider: %w", fakeFailures[s.Failure]))
		}
		return src
	})
}
