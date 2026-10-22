// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"errors"

	"github.com/best-friends/my/internal/app/finance"
	"github.com/best-friends/my/internal/app/supporters"
)

// The two ports between the supporters and the finance modules (ADR-0073 §1): the supporters
// hand their money movements to the income records, and the finance module reads the plans,
// the subscribers, and the provider's time zone of the supporters. Both read the App when they
// are called, since the supporters module is set up before the finance module.

// financeIncome is the supporters.IncomeRecorder of the finance module's income records.
type financeIncome struct{ a *App }

var _ supporters.IncomeRecorder = financeIncome{}

// errNoFinance reports a money movement before the finance module is set up.
var errNoFinance = errors.New("platform: the finance module is not set up")

func (f financeIncome) RecordIncome(ctx context.Context, ms []finance.IncomeMovement) error {
	if f.a.Finance == nil {
		return errNoFinance
	}
	return f.a.Finance.RecordIncome(ctx, ms)
}

func (f financeIncome) KnownIncome(ctx context.Context, provider string, ids []string) (map[string]bool, error) {
	if f.a.Finance == nil {
		return nil, errNoFinance
	}
	return f.a.Finance.KnownIncome(ctx, provider, ids)
}

// supporterDirectory is the finance.SupporterDirectory of the supporters module; without it,
// it reads nothing.
type supporterDirectory struct{ a *App }

var _ finance.SupporterDirectory = supporterDirectory{}

func (d supporterDirectory) PlanCounts(ctx context.Context) ([]finance.PlanCount, error) {
	if d.a.Supporters == nil {
		return nil, nil
	}
	counts, err := d.a.Supporters.PlanCounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]finance.PlanCount, len(counts))
	for i, c := range counts {
		out[i] = finance.PlanCount{PlanID: c.PlanID, Name: c.Name, Members: c.Members}
	}
	return out, nil
}

func (d supporterDirectory) SubscriptionAccounts(ctx context.Context, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if d.a.Supporters == nil {
		return out, nil
	}
	users, err := d.a.Supporters.SubscriptionUsers(ctx, ids)
	if err != nil || len(users) == 0 {
		return out, err
	}
	userIDs := make([]string, len(users))
	for i, u := range users {
		userIDs[i] = u.UserID
	}
	accounts, err := supporterUsers(d).AccountIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if acct, ok := accounts[u.UserID]; ok {
			out[u.SubscriptionID] = acct
		}
	}
	return out, nil
}

func (d supporterDirectory) ProviderRules() []finance.ProviderRule {
	if d.a.Supporters == nil {
		return nil
	}
	name, loc, ok := d.a.Supporters.ProviderLocation()
	if !ok {
		return nil
	}
	return []finance.ProviderRule{{Provider: name, Location: loc}}
}
