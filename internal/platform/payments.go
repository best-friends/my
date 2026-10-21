// SPDX-License-Identifier: AGPL-3.0-only

package platform

import (
	"context"
	"fmt"

	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/supporters"
	paymentshttp "github.com/best-friends/my/internal/interface/payments"
	"github.com/best-friends/my/internal/platform/config"
)

// paymentProviderAdapters builds the PaymentProvider of each value of payments_provider whose
// adapter is in this build (ADR-0074 §8): the files of the adapters, under their build tags,
// add theirs.
var paymentProviderAdapters = map[string]func(ctx context.Context, cfg *config.Config) (supporters.PaymentProvider, error){}

// setUpPayments builds what carries the payment provider's facts to the supporters module
// (ADR-0074 §3, §4) when payments_provider names a provider, or AppOptions.PaymentProvider
// stands in for it: the webhook endpoint, the jobs of the deliveries, the reconciliation every
// payments_reconcile_interval, and the purge of the receipts. With payments_provider = none
// nothing runs and the endpoint answers 404 like any unknown path.
func (a *App) setUpPayments(ctx context.Context, cfg *config.Config, repo supporters.PaymentRepository, opts AppOptions) error {
	prov := opts.PaymentProvider
	if prov == nil {
		if cfg.PaymentsProvider == "" || cfg.PaymentsProvider == "none" {
			return nil
		}
		build, ok := paymentProviderAdapters[cfg.PaymentsProvider]
		if !ok {
			return fmt.Errorf("payments_provider = %s: this build has no adapter of %s", cfg.PaymentsProvider, cfg.PaymentsProvider)
		}
		var err error
		if prov, err = build(ctx, cfg); err != nil {
			return fmt.Errorf("payments_provider = %s: %w", cfg.PaymentsProvider, err)
		}
	}
	if opts.SupporterSync == nil {
		return fmt.Errorf("payments_provider = %s: no subscriptions of the supporters module apply the payments", prov.Name())
	}
	outbox := a.Outbox
	if outbox == nil {
		// The inline queue has no transactional enqueue; its jobs are enqueued after the commit.
		outbox = queue.AfterCommit(a.Projectors, a.Jobs, a.Logger)
	}
	a.Payments = supporters.NewPayments(supporters.PaymentsDeps{
		UnitOfWork: a.UnitOfWork, Repository: repo, Provider: prov, Syncer: opts.SupporterSync, Outbox: outbox,
		Clock: a.Clock, IDs: a.IDs, RateLimiter: a.RateLimiter,
		ReconcileInterval: cfg.PaymentsReconcileInterval, ReconcileMaxRequests: cfg.PaymentsReconcileMaxRequests,
		Logger: a.Logger,
	})
	a.Payments.RegisterJobs(a.Handlers, a.Scheduler)
	paymentshttp.New(paymentshttp.Options{Webhooks: a.Payments, Problem: a.writeStatusProblem, Logger: a.Logger}).
		Register(surfaceRouter{a, surfacePayments})
	return nil
}
