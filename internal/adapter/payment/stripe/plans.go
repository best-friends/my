// SPDX-License-Identifier: AGPL-3.0-only

package stripe

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"

	stripe "github.com/stripe/stripe-go/v87"

	"github.com/best-friends/my/internal/app/supporters"
)

// planIDPattern are the plan IDs that fit in the IDs and lookup keys of Stripe.
var planIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// productID returns the ID of the product of a plan. Stripe takes a product ID on creation, so
// the product is found by it without the search API, whose index lags behind the writes.
func productID(planID string) string { return "my_plan_" + planID }

// lookupKey returns the lookup key of a plan's price: my_<plan_id>_<interval>_<currency>.
func lookupKey(planID string, ps supporters.PriceSpec) string {
	return "my_" + planID + "_" + string(ps.Interval) + "_" + strings.ToLower(ps.Currency)
}

// EnsurePlan implements supporters.PaymentProvider (ADR-0074 §2): the product carries the plan's
// ID in its ID and its metadata, and each price has the lookup key of its interval and currency.
// A price whose amount changed is replaced by a new one that takes over the lookup key, and the
// old one is archived; Stripe keeps renewing the subscriptions already on it. The product's
// other active prices are archived too.
func (p *Provider) EnsurePlan(ctx context.Context, spec supporters.PlanSpec) (supporters.PlanRefs, error) {
	if err := p.checkPlan(spec); err != nil {
		return supporters.PlanRefs{}, err
	}
	prod, err := p.ensureProduct(ctx, spec)
	if err != nil {
		return supporters.PlanRefs{}, err
	}
	refs := supporters.PlanRefs{Product: supporters.ProductRef(prod)}
	keep := map[string]bool{}
	for _, ps := range spec.Prices {
		ref, replaced, err := p.ensurePrice(ctx, prod, spec.PlanID, ps)
		if err != nil {
			return supporters.PlanRefs{}, err
		}
		keep[ref] = true
		refs.Prices = append(refs.Prices, supporters.PlanPrice{Spec: ps, Ref: supporters.PriceRef(ref)})
		if replaced != "" {
			if err := p.archivePrice(ctx, replaced); err != nil {
				return supporters.PlanRefs{}, err
			}
			refs.Archived = append(refs.Archived, supporters.PriceRef(replaced))
		}
	}
	var active *stripe.V1List[*stripe.Price]
	err = p.do(ctx, "list the prices of "+prod, func() error {
		active = p.sc.V1Prices.List(ctx, &stripe.PriceListParams{
			ListParams: stripe.ListParams{Limit: stripe.Int64(supporters.PageSize), Single: true},
			Product:    stripe.String(prod), Active: stripe.Bool(true),
		})
		return active.Err()
	})
	if err != nil {
		return supporters.PlanRefs{}, err
	}
	for _, pr := range active.Data() {
		if keep[pr.ID] || slices.Contains(refs.Archived, supporters.PriceRef(pr.ID)) {
			continue
		}
		if err := p.archivePrice(ctx, pr.ID); err != nil {
			return supporters.PlanRefs{}, err
		}
		refs.Archived = append(refs.Archived, supporters.PriceRef(pr.ID))
	}
	return refs, nil
}

// checkPlan refuses a spec that Stripe would refuse or that the lookup keys cannot tell apart.
func (p *Provider) checkPlan(spec supporters.PlanSpec) error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("stripe: plan %q: %w: %s", spec.PlanID, supporters.ErrPaymentInvalidRequest, fmt.Sprintf(format, args...))
	}
	if !planIDPattern.MatchString(spec.PlanID) {
		return invalid("the plan ID must be 1 to 64 letters, digits, '_', or '-'")
	}
	if strings.TrimSpace(spec.Name) == "" {
		return invalid("no name")
	}
	seen := map[string]bool{}
	for _, ps := range spec.Prices {
		if ps.Interval != supporters.IntervalMonth && ps.Interval != supporters.IntervalYear {
			return invalid("unknown interval %q", ps.Interval)
		}
		if len(ps.Currency) != 3 {
			return invalid("the currency %q is not an ISO 4217 code", ps.Currency)
		}
		if ps.AmountMinor <= 0 {
			return invalid("the amount of %s %s must be positive", ps.Interval, ps.Currency)
		}
		if minimum, ok := p.MinimumAmount(ps.Currency); ok && ps.AmountMinor < minimum {
			return invalid("the amount of %s %s is below Stripe's minimum of %d", ps.Interval, ps.Currency, minimum)
		}
		key := lookupKey(spec.PlanID, ps)
		if seen[key] {
			return invalid("two prices of %s %s", ps.Interval, ps.Currency)
		}
		seen[key] = true
	}
	return nil
}

// ensureProduct returns the ID of the plan's product, created or updated to the spec.
func (p *Provider) ensureProduct(ctx context.Context, spec supporters.PlanSpec) (string, error) {
	id := productID(spec.PlanID)
	var prod *stripe.Product
	err := p.do(ctx, "read the product "+id, func() (err error) {
		prod, err = p.sc.V1Products.Retrieve(ctx, id, &stripe.ProductRetrieveParams{})
		return err
	})
	if errors.Is(err, supporters.ErrPaymentNotFound) {
		err = p.do(ctx, "create the product "+id, func() (err error) {
			prod, err = p.sc.V1Products.Create(ctx, &stripe.ProductCreateParams{
				ID: stripe.String(id), Name: stripe.String(spec.Name), Description: optional(spec.Description),
				Metadata: map[string]string{metaPlanID: spec.PlanID},
			})
			return err
		})
		if err == nil {
			return prod.ID, nil
		}
		// Another call created it in between: read it again.
		err = p.do(ctx, "read the product "+id, func() (err error) {
			prod, err = p.sc.V1Products.Retrieve(ctx, id, &stripe.ProductRetrieveParams{})
			return err
		})
	}
	if err != nil {
		return "", err
	}
	if prod.Name == spec.Name && prod.Description == spec.Description && prod.Active && prod.Metadata[metaPlanID] == spec.PlanID {
		return prod.ID, nil
	}
	params := &stripe.ProductUpdateParams{
		Name: stripe.String(spec.Name), Active: stripe.Bool(true), Metadata: map[string]string{metaPlanID: spec.PlanID},
	}
	if spec.Description != "" {
		params.Description = stripe.String(spec.Description)
	} else if prod.Description != "" {
		params.AddExtra("description", "")
	}
	err = p.do(ctx, "update the product "+id, func() (err error) {
		_, err = p.sc.V1Products.Update(ctx, id, params)
		return err
	})
	return id, err
}

// ensurePrice returns the price of ps: the one of its lookup key when it matches, or a new one
// that takes over the key, with the price it replaced.
func (p *Provider) ensurePrice(ctx context.Context, product, planID string, ps supporters.PriceSpec) (ref, replaced string, err error) {
	key := lookupKey(planID, ps)
	var list *stripe.V1List[*stripe.Price]
	err = p.do(ctx, "find the price "+key, func() error {
		list = p.sc.V1Prices.List(ctx, &stripe.PriceListParams{
			ListParams: stripe.ListParams{Limit: stripe.Int64(10), Single: true},
			LookupKeys: []*string{stripe.String(key)},
		})
		return list.Err()
	})
	if err != nil {
		return "", "", err
	}
	var current *stripe.Price
	for _, pr := range list.Data() {
		if pr.LookupKey == key {
			current = pr
			break
		}
	}
	if current != nil && current.Active && priceMatches(current, product, ps) {
		return current.ID, "", nil
	}
	var created *stripe.Price
	err = p.do(ctx, "create the price "+key, func() (err error) {
		created, err = p.sc.V1Prices.Create(ctx, &stripe.PriceCreateParams{
			Product: stripe.String(product), Currency: stripe.String(strings.ToLower(ps.Currency)),
			UnitAmount: stripe.Int64(ps.AmountMinor),
			Recurring:  &stripe.PriceCreateRecurringParams{Interval: stripe.String(string(ps.Interval)), IntervalCount: stripe.Int64(1)},
			LookupKey:  stripe.String(key), TransferLookupKey: stripe.Bool(current != nil),
			Metadata: map[string]string{metaPlanID: planID},
		})
		return err
	})
	if err != nil {
		return "", "", err
	}
	if current != nil && current.Active {
		replaced = current.ID
	}
	return created.ID, replaced, nil
}

// priceMatches reports whether pr is the price of ps on product.
func priceMatches(pr *stripe.Price, product string, ps supporters.PriceSpec) bool {
	return pr.Product != nil && pr.Product.ID == product && pr.UnitAmount == ps.AmountMinor &&
		strings.EqualFold(string(pr.Currency), ps.Currency) && pr.Recurring != nil &&
		string(pr.Recurring.Interval) == string(ps.Interval) && pr.Recurring.IntervalCount == 1
}

func (p *Provider) archivePrice(ctx context.Context, id string) error {
	return p.do(ctx, "archive the price "+id, func() (err error) {
		_, err = p.sc.V1Prices.Update(ctx, id, &stripe.PriceUpdateParams{Active: stripe.Bool(false)})
		return err
	})
}

// optional returns nil for "", which leaves a parameter out.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return stripe.String(s)
}
