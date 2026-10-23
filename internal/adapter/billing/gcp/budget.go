// SPDX-License-Identifier: AGPL-3.0-only

package gcp

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	pubsubv2 "cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/api/option"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appfinance "github.com/mycroblog/my/internal/app/finance"
	"github.com/mycroblog/my/internal/domain/finance"
)

// The defaults of BudgetOptions.
const (
	// DefaultMaxMessages is the most messages of a pull, the most Pub/Sub returns.
	DefaultMaxMessages = 1000
	// DefaultPullWait is how long a pull waits for messages before the subscription counts as
	// read through.
	DefaultPullWait = 5 * time.Second
)

// subscriptionRe is the full name of a subscription: projects/{project}/subscriptions/{id}, the
// ID of 3 to 255 letters, digits, and -_.~+%, starting with a letter and not with goog.
var subscriptionRe = regexp.MustCompile(`^projects/([a-z][a-z0-9.:-]{4,61}[a-z0-9])/subscriptions/([A-Za-z][A-Za-z0-9._~+%-]{2,254})$`)

// CheckSubscription accepts the full name of a Pub/Sub subscription,
// projects/{project}/subscriptions/{subscription}.
func CheckSubscription(v string) error {
	m := subscriptionRe.FindStringSubmatch(strings.TrimSpace(v))
	if m == nil || strings.HasPrefix(m[2], "goog") {
		return fmt.Errorf("%q is not the full name of a Pub/Sub subscription, such as projects/my-project/subscriptions/my-billing-budget", v)
	}
	return CheckProject(m[1])
}

// BudgetOptions configure a BudgetSource.
type BudgetOptions struct {
	// Subscription is the pull subscription of the budget's topic,
	// projects/{project}/subscriptions/{subscription}.
	Subscription string
	// MaxMessages is the most messages of a pull, at most DefaultMaxMessages; DefaultMaxMessages
	// when 0.
	MaxMessages int32
	// PullWait is how long a pull waits for messages; DefaultPullWait when 0.
	PullWait time.Duration
	// Now returns the current time; time.Now when nil.
	Now func() time.Time
}

// BudgetSource is the BillingSource of Google Cloud by the Pub/Sub notifications of a Cloud
// Billing budget (billing_gcp_method = budget, ADR-0068 §7). A notification carries the budget's
// cost so far in its period (costAmount, an estimate, with the credits and taxes) and the start of
// the period (costIntervalStart), so a month of a budget is one line: the service is the budget's
// display name, the charge type usage, the category other.
//
// The notifications are cumulative and arrive several times a day, duplicated and out of order.
// A fetch pulls the subscription until a pull brings nothing new and takes, for each budget and
// month, the notification published last. It acknowledges the notifications that a later one of
// the same budget and month replaces and those of the months older than the ones asked for, which
// the import has recorded as final (finance.RefetchMonths drops the previous month once its
// revisions are final); it hands the latest of each month asked for back to the subscription
// unacknowledged (a deadline of 0), so the next fetch reads it again. Fetching the same
// notifications again therefore returns the same lines, and the import writes nothing new; no
// value of a month is lost before the import has recorded it final.
//
// Each Pull counts as a billed request against FetchRequest.MaxRequests; the Acknowledge and
// ModifyAckDeadline calls of the end of a fetch are not counted.
type BudgetSource struct {
	client      *pubsubv2.Client
	sub         string
	maxMessages int32
	wait        time.Duration
	loc         *time.Location
	now         func() time.Time
}

var _ appfinance.BillingSource = (*BudgetSource)(nil)

// NewBudget returns the BudgetSource of o on the Pub/Sub API of opts: the Application Default
// Credentials and Google's endpoint when opts is empty, or the emulator at PUBSUB_EMULATOR_HOST.
// Pub/Sub is the operator's own cloud, so its client does not go through the SSRF-guarded
// kernel.HTTPClient. The calls do not retry: a call that fails ends the fetch, and the job's
// retry (ADR-0004) tries again.
func NewBudget(ctx context.Context, o BudgetOptions, opts ...option.ClientOption) (*BudgetSource, error) {
	o.Subscription = strings.TrimSpace(o.Subscription)
	if err := CheckSubscription(o.Subscription); err != nil {
		return nil, fmt.Errorf("billing gcp: subscription: %w", err)
	}
	if o.MaxMessages < 0 || o.MaxMessages > DefaultMaxMessages || o.PullWait < 0 {
		return nil, fmt.Errorf("billing gcp: max messages %d outside 0 to 1000 or a negative pull wait %s", o.MaxMessages, o.PullWait)
	}
	loc, err := Location()
	if err != nil {
		return nil, err
	}
	project := subscriptionRe.FindStringSubmatch(o.Subscription)[1]
	client, err := pubsubv2.NewClient(ctx, project, opts...)
	if err != nil {
		return nil, fmt.Errorf("billing gcp: the Pub/Sub client: %w", err)
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &BudgetSource{
		client: client, sub: o.Subscription, loc: loc,
		maxMessages: cmp.Or(o.MaxMessages, DefaultMaxMessages), wait: cmp.Or(o.PullWait, DefaultPullWait),
		now: o.Now,
	}, nil
}

// Close closes the Pub/Sub client.
func (s *BudgetSource) Close() error { return s.client.Close() }

// Name implements appfinance.BillingSource.
func (s *BudgetSource) Name() string { return Name }

// Location implements appfinance.BillingSource: the budget periods start at midnight US Pacific
// time.
func (s *BudgetSource) Location() *time.Location { return s.loc }

// Method implements appfinance.BillingSource: the budget notifications give one estimated total
// per budget and month, without a breakdown by service.
func (s *BudgetSource) Method() string { return appfinance.MethodBudget }

// noRetry turns off the retries of the client library, so that every call is one request.
var noRetry = gax.WithRetry(func() gax.Retryer { return nil })

// held is a message pulled and not yet settled.
type held struct {
	ackID string
	msg   *pubsubpb.PubsubMessage
}

// notification is a budget notification of a month (Programmatic budget notifications, schema
// version 1.0).
type notification struct {
	id        string
	budget    string // billingAccountId/budgetId
	name      string
	month     finance.YearMonth
	amount    string
	currency  string
	published time.Time
}

// Fetch implements appfinance.BillingSource. It pulls until a pull brings no message it has not
// seen, counting each pull as a request and stopping with ErrRequestCap rather than make one
// beyond req.MaxRequests; a fetch that fails hands every message back unacknowledged, except the
// malformed ones, which it acknowledges so that they do not fail the next imports.
func (s *BudgetSource) Fetch(ctx context.Context, req appfinance.FetchRequest) (appfinance.Snapshot, error) {
	var snap appfinance.Snapshot
	if err := ctx.Err(); err != nil {
		return snap, err
	}
	if req.MaxRequests <= 0 {
		return snap, fmt.Errorf("%w: no billed request is left", appfinance.ErrRequestCap)
	}
	today := finance.DateOf(s.now().In(s.loc))
	var months []finance.YearMonth
	for _, m := range req.Months {
		if !today.Before(m.Range().Start) && !slices.Contains(months, m) {
			months = append(months, m)
		}
	}
	if len(months) == 0 {
		// Every month asked for is in the future: it has no cost yet.
		return snap, nil
	}
	slices.SortFunc(months, finance.YearMonth.Compare)

	pulled := map[string]*held{}
	var order []string
	fail := func(err error, ack []string) (appfinance.Snapshot, error) {
		var nack []string
		for _, id := range order {
			if !slices.Contains(ack, id) {
				nack = append(nack, pulled[id].ackID)
			}
		}
		acks := make([]string, 0, len(ack))
		for _, id := range ack {
			acks = append(acks, pulled[id].ackID)
		}
		// The settling is best effort: an unsettled message comes back after its ack deadline.
		_ = s.settle(context.WithoutCancel(ctx), acks, nack)
		return appfinance.Snapshot{Requests: snap.Requests}, err
	}
	for {
		if snap.Requests >= req.MaxRequests {
			return fail(fmt.Errorf("%w: %d pulls made, more messages may remain", appfinance.ErrRequestCap, snap.Requests), nil)
		}
		snap.Requests++
		msgs, err := s.pull(ctx)
		if err != nil {
			return fail(err, nil)
		}
		fresh := 0
		for _, rm := range msgs {
			if rm.GetMessage() == nil {
				continue
			}
			id := rm.GetMessage().GetMessageId()
			if h, ok := pulled[id]; ok {
				h.ackID = rm.GetAckId() // a redelivery: only its latest ack ID is valid
				continue
			}
			pulled[id] = &held{ackID: rm.GetAckId(), msg: rm.GetMessage()}
			order = append(order, id)
			fresh++
		}
		if fresh == 0 {
			break
		}
	}

	notes := make([]notification, 0, len(order))
	var malformed []string
	var firstErr error
	for _, id := range order {
		n, err := s.parse(pulled[id].msg)
		if err != nil {
			malformed = append(malformed, id)
			firstErr = cmp.Or(firstErr, err)
			continue
		}
		notes = append(notes, n)
	}
	if firstErr != nil {
		return fail(firstErr, malformed)
	}

	type key struct {
		budget string
		month  finance.YearMonth
	}
	latest := map[key]notification{}
	for _, n := range notes {
		k := key{n.budget, n.month}
		if cur, ok := latest[k]; !ok || n.published.After(cur.published) ||
			(n.published.Equal(cur.published) && n.id > cur.id) {
			latest[k] = n
		}
	}
	var acks, nacks []string
	for _, n := range notes {
		kept := latest[key{n.budget, n.month}].id == n.id && n.month.Compare(months[0]) >= 0
		if kept {
			nacks = append(nacks, pulled[n.id].ackID)
		} else {
			acks = append(acks, pulled[n.id].ackID)
		}
	}

	type nameKey struct {
		name  string
		month finance.YearMonth
	}
	lines := make([]appfinance.CostLine, 0, len(latest))
	services := map[nameKey]string{}
	for k, n := range latest {
		if !slices.Contains(months, k.month) {
			continue
		}
		if other, ok := services[nameKey{n.name, k.month}]; ok && other != k.budget {
			return fail(invalid("the budgets %s and %s of %s have the same name %q", other, k.budget, k.month, n.name), nil)
		}
		services[nameKey{n.name, k.month}] = k.budget
		final := Final(k.month, today)
		covered := k.month.Range()
		if !final {
			covered.End = s.through(covered, n.published, today)
		}
		lines = append(lines, appfinance.CostLine{
			Month: k.month, Covered: covered, Service: n.name, ChargeType: finance.ChargeUsage,
			Category: finance.CategoryOther, Amount: n.amount, Currency: n.currency, Final: final,
		})
	}
	if err := s.settle(ctx, acks, nacks); err != nil {
		return appfinance.Snapshot{Requests: snap.Requests}, err
	}
	slices.SortFunc(lines, func(x, y appfinance.CostLine) int {
		return cmp.Or(x.Month.Compare(y.Month), cmp.Compare(x.Service, y.Service))
	})
	snap.Lines = lines
	return snap, nil
}

// pull makes one Pull of the subscription, which waits up to s.wait for a message; a pull that
// the wait ends brings nothing.
func (s *BudgetSource) pull(ctx context.Context) ([]*pubsubpb.ReceivedMessage, error) {
	pullCtx, cancel := context.WithTimeout(ctx, s.wait)
	defer cancel()
	res, err := s.client.SubscriptionAdminClient.Pull(pullCtx, &pubsubpb.PullRequest{
		Subscription: s.sub, MaxMessages: s.maxMessages,
	}, noRetry)
	if err != nil {
		if ctx.Err() == nil && (errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded) {
			return nil, nil
		}
		return nil, classifyPubSub(ctx, err)
	}
	return res.GetReceivedMessages(), nil
}

// settle acknowledges acks and hands nacks back to the subscription, in calls of at most
// DefaultMaxMessages ack IDs.
func (s *BudgetSource) settle(ctx context.Context, acks, nacks []string) error {
	for c := range slices.Chunk(acks, DefaultMaxMessages) {
		if err := s.client.SubscriptionAdminClient.Acknowledge(ctx, &pubsubpb.AcknowledgeRequest{Subscription: s.sub, AckIds: c}, noRetry); err != nil {
			return classifyPubSub(ctx, err)
		}
	}
	for c := range slices.Chunk(nacks, DefaultMaxMessages) {
		if err := s.client.SubscriptionAdminClient.ModifyAckDeadline(ctx, &pubsubpb.ModifyAckDeadlineRequest{Subscription: s.sub, AckIds: c}, noRetry); err != nil {
			return classifyPubSub(ctx, err)
		}
	}
	return nil
}

// budgetData is the JSON of a budget notification.
type budgetData struct {
	BudgetDisplayName string      `json:"budgetDisplayName"`
	CostAmount        json.Number `json:"costAmount"`
	CostIntervalStart string      `json:"costIntervalStart"`
	CurrencyCode      string      `json:"currencyCode"`
}

// parse reads a budget notification: a monthly budget's cost so far in a month, in Pacific time.
func (s *BudgetSource) parse(m *pubsubpb.PubsubMessage) (notification, error) {
	id := m.GetMessageId()
	attrs := m.GetAttributes()
	if attrs["budgetId"] == "" {
		return notification{}, invalid("message %s is not a budget notification: no budgetId", id)
	}
	var d budgetData
	dec := json.NewDecoder(bytes.NewReader(m.GetData()))
	dec.UseNumber()
	if err := dec.Decode(&d); err != nil {
		return notification{}, invalid("budget notification %s: %v", id, err)
	}
	n := notification{
		id: id, budget: attrs["billingAccountId"] + "/" + attrs["budgetId"], name: strings.TrimSpace(d.BudgetDisplayName),
		amount: d.CostAmount.String(), currency: d.CurrencyCode, published: m.GetPublishTime().AsTime(),
	}
	if n.name == "" {
		n.name = attrs["budgetId"]
	}
	start, err := time.Parse(time.RFC3339, d.CostIntervalStart)
	if err != nil {
		return notification{}, invalid("budget notification %s: costIntervalStart %q", id, d.CostIntervalStart)
	}
	start = start.In(s.loc)
	if start.Day() != 1 || start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 {
		return notification{}, invalid("budget notification %s: the period starts on %s, not on the first of a month in Pacific time: the budget is not monthly", id, start.Format(time.RFC3339))
	}
	n.month = finance.YearMonthOf(finance.DateOf(start))
	if _, err := finance.ParseDecimal(n.amount); err != nil {
		return notification{}, invalid("budget notification %s: costAmount %q", id, n.amount)
	}
	if _, err := finance.ParseCurrency(n.currency); err != nil {
		return notification{}, invalid("budget notification %s: currencyCode %q", id, n.currency)
	}
	return n, nil
}

// through is the last day with data of an open month: the Pacific day on which its latest
// notification was published, within the month and not after today.
func (s *BudgetSource) through(month finance.DateRange, published time.Time, today finance.Date) finance.Date {
	end := month.End
	if !today.After(end) {
		end = today
	}
	last := finance.DateOf(published.In(s.loc))
	switch {
	case last.Before(month.Start):
		return month.Start
	case last.After(end):
		return end
	}
	return last
}

// classifyPubSub wraps an error of the Pub/Sub API in the error of its kind (ADR-0068 §9).
func classifyPubSub(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		status.Code(err) == codes.Canceled || status.Code(err) == codes.DeadlineExceeded) {
		return fmt.Errorf("billing gcp: %w", ctxErr)
	}
	reason := ""
	var api *apierror.APIError
	if errors.As(err, &api) {
		reason = api.Reason()
	}
	msg := status.Convert(err).Message()
	switch code := status.Code(err); code {
	case codes.PermissionDenied, codes.Unauthenticated:
		if reason == "SERVICE_DISABLED" || strings.Contains(msg, "has not been used in project") || strings.Contains(msg, "it is disabled") {
			return fmt.Errorf("%w: the Pub/Sub API is not enabled for the subscription's project: %w", appfinance.ErrBillingNotEnabled, err)
		}
		return fmt.Errorf("%w: %w", appfinance.ErrBillingPermission, err)
	case codes.NotFound:
		return fmt.Errorf("%w: the subscription of billing_gcp_budget_subscription does not exist: %w", appfinance.ErrBillingNotFound, err)
	case codes.ResourceExhausted:
		return fmt.Errorf("%w: %w", appfinance.ErrBillingThrottled, err)
	case codes.Unavailable, codes.Internal, codes.Unknown, codes.DeadlineExceeded, codes.Aborted, codes.Canceled:
		// codes.Unknown is also an error without a status: the credentials could not be
		// retrieved or the request could not be sent.
		return fmt.Errorf("%w: %w", appfinance.ErrBillingUnavailable, err)
	default:
		return fmt.Errorf("%w: %w", appfinance.ErrInvalidBillingResponse, err)
	}
}
