// SPDX-License-Identifier: AGPL-3.0-only

package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/best-friends/my/internal/app/supporters"
)

// eventKinds are the types of the events that the operator enables on the webhook endpoint
// (ADR-0074 §3) and that ListChanges reads, with what they point at. Any other type is ignored.
var eventKinds = map[string]supporters.NoticeKind{
	"checkout.session.completed":      supporters.NoticeCheckout,
	"checkout.session.expired":        supporters.NoticeCheckout,
	"customer.subscription.created":   supporters.NoticeSubscription,
	"customer.subscription.updated":   supporters.NoticeSubscription,
	"customer.subscription.deleted":   supporters.NoticeSubscription,
	"customer.subscription.paused":    supporters.NoticeSubscription,
	"customer.subscription.resumed":   supporters.NoticeSubscription,
	"invoice.paid":                    supporters.NoticeMoney,
	"invoice.payment_failed":          supporters.NoticeMoney,
	"charge.refunded":                 supporters.NoticeMoney,
	"refund.created":                  supporters.NoticeMoney,
	"refund.updated":                  supporters.NoticeMoney,
	"charge.dispute.created":          supporters.NoticeMoney,
	"charge.dispute.closed":           supporters.NoticeMoney,
	"charge.dispute.funds_withdrawn":  supporters.NoticeMoney,
	"charge.dispute.funds_reinstated": supporters.NoticeMoney,
}

// EventTypes returns the types of the events to enable on the webhook endpoint, sorted.
func EventTypes() []string { return slices.Sorted(maps.Keys(eventKinds)) }

// event is what the adapter reads of an event: the fields whose place no API version changes.
type event struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Livemode bool   `json:"livemode"`
	Created  int64  `json:"created"`
	Data     struct {
		Object struct {
			ID     string `json:"id"`
			Object string `json:"object"`
		} `json:"object"`
	} `json:"data"`
}

// ParseWebhook implements supporters.PaymentProvider with the standard library (ADR-0074 §3,
// W1): one of the v1 signatures of the Stripe-Signature header must be the HMAC-SHA256 of
// "<t>.<body>" with the endpoint's secret, compared in constant time, t must be within
// supporters.WebhookTolerance of now, and the event's livemode must be the key's mode.
func (p *Provider) ParseWebhook(header http.Header, body []byte, now time.Time) (supporters.Notice, error) {
	t, sigs := parseSignature(header.Get(SignatureHeader))
	if t == "" || len(sigs) == 0 {
		return supporters.Notice{}, fmt.Errorf("stripe: %w: no signature", supporters.ErrInvalidSignature)
	}
	mac := hmac.New(sha256.New, []byte(p.o.WebhookSecret))
	mac.Write([]byte(t))
	mac.Write([]byte("."))
	mac.Write(body)
	want := mac.Sum(nil)
	valid := false
	for _, s := range sigs {
		got, err := hex.DecodeString(s)
		if err == nil && hmac.Equal(got, want) {
			valid = true
		}
	}
	if !valid {
		return supporters.Notice{}, fmt.Errorf("stripe: %w", supporters.ErrInvalidSignature)
	}
	unix, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return supporters.Notice{}, fmt.Errorf("stripe: %w: bad timestamp", supporters.ErrInvalidSignature)
	}
	if d := now.Sub(time.Unix(unix, 0)); d > supporters.WebhookTolerance || d < -supporters.WebhookTolerance {
		return supporters.Notice{}, fmt.Errorf("stripe: %w: signed %s from now", supporters.ErrStaleSignature, d.Round(time.Second))
	}
	var e event
	if err := json.Unmarshal(body, &e); err != nil || e.ID == "" || e.Type == "" {
		return supporters.Notice{}, fmt.Errorf("stripe: %w: the webhook is not an event", supporters.ErrInvalidPaymentResponse)
	}
	if e.Livemode != (p.mode == supporters.ModeLive) {
		return supporters.Notice{}, fmt.Errorf("stripe: %w: livemode %t with a %s key", supporters.ErrModeMismatch, e.Livemode, p.mode)
	}
	return notice(e.ID, e.Type, e.Data.Object.ID, e.Created, ""), nil
}

// parseSignature splits a Stripe-Signature header, t=<unix>,v1=<hex>[,v1=<hex>...], into its
// time and its v1 signatures; the other schemes are ignored.
func parseSignature(h string) (t string, v1 []string) {
	for part := range strings.SplitSeq(h, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			if t == "" {
				t = v
			}
		case "v1":
			v1 = append(v1, v)
		}
	}
	return t, v1
}

// notice returns the notice of an event.
func notice(id, typ, object string, created int64, sub string) supporters.Notice {
	kind, ok := eventKinds[typ]
	if !ok {
		kind = supporters.NoticeIgnored
	}
	return supporters.Notice{
		EventID: id, Type: typ, Kind: kind, ObjectID: object,
		Subscription: supporters.SubscriptionRef(sub), OccurredAt: unixTime(created),
	}
}
