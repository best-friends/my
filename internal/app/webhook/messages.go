// SPDX-License-Identifier: AGPL-3.0-only

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	appaccount "github.com/best-friends/my/internal/app/account"
	appcontent "github.com/best-friends/my/internal/app/content"
	appidentity "github.com/best-friends/my/internal/app/identity"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/webhook"
)

// The messages (ADR-0060 "イベントとペイロード"): the events of the other modules become, in their
// transactions, a message to the enabled endpoints that subscribe to its type, and a
// webhook.deliver job per endpoint. Without such an endpoint nothing is written. The body is
// {"type", "timestamp", "data"}; data names its fields as the admin API does
// (docs/reference/webhooks.md, the Webhook* schemas of api/openapi.yaml).

// envelope is the body of a message.
type envelope struct {
	Type      webhook.EventType `json:"type"`
	Timestamp string            `json:"timestamp"`
	Data      any               `json:"data"`
}

// timestamp formats a time of a body: RFC 3339 in UTC with milliseconds.
func timestamp(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05.000Z07:00") }

func optTimestamp(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := timestamp(*t)
	return &s
}

func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func list(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// reportData is the data of report.* (WebhookReport).
type reportData struct {
	ID                string   `json:"id"`
	URI               *string  `json:"uri,omitempty"`
	Category          string   `json:"category"`
	Comment           string   `json:"comment"`
	Forwarded         bool     `json:"forwarded"`
	FiledAt           string   `json:"filed_at"`
	State             string   `json:"state"`
	Outcome           *string  `json:"outcome,omitempty"`
	StateChangedAt    string   `json:"state_changed_at"`
	ReporterAccountID string   `json:"reporter_account_id"`
	TargetAccountID   string   `json:"target_account_id"`
	StatusIDs         []string `json:"status_ids"`
	RuleIDs           []string `json:"rule_ids"`
}

func newReportData(r appmoderation.Report, state string, outcome string, changedAt time.Time) reportData {
	return reportData{
		ID: r.ID, URI: optString(r.URI), Category: string(r.Category), Comment: r.Comment, Forwarded: r.Forwarded,
		FiledAt: timestamp(r.FiledAt), State: state, Outcome: optString(outcome), StateChangedAt: timestamp(changedAt),
		ReporterAccountID: r.ReporterAccountID, TargetAccountID: r.TargetAccountID,
		StatusIDs: list(r.StatusIDs), RuleIDs: list(r.RuleIDs),
	}
}

// registrationData is the data of registration.pending and account.created
// (WebhookRegistration).
type registrationData struct {
	UserID    string `json:"user_id"`
	AccountID string `json:"account_id"`
	Username  string `json:"username"`
	State     string `json:"state"`
	Since     string `json:"since"`
}

// accountData is the data of account.updated (WebhookAccount).
type accountData struct {
	ID           string             `json:"id"`
	DisplayName  string             `json:"display_name"`
	Note         string             `json:"note"`
	Locked       bool               `json:"locked"`
	Bot          bool               `json:"bot"`
	Discoverable bool               `json:"discoverable"`
	Indexable    bool               `json:"indexable"`
	Fields       []accountFieldData `json:"fields"`
	UpdatedAt    string             `json:"updated_at"`
}

type accountFieldData struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// accountActionData is the data of moderation_action.* (WebhookAccountAction).
type accountActionData struct {
	ID             string   `json:"id"`
	AccountID      string   `json:"account_id"`
	Kind           string   `json:"kind"`
	ReasonCategory string   `json:"reason_category"`
	PublicReason   *string  `json:"public_reason,omitempty"`
	PrivateNote    *string  `json:"private_note,omitempty"`
	ReportID       *string  `json:"report_id,omitempty"`
	Issuer         string   `json:"issuer"`
	NotifyTarget   bool     `json:"notify_target"`
	State          string   `json:"state"`
	IssuedAt       string   `json:"issued_at"`
	ExpiresAt      *string  `json:"expires_at,omitempty"`
	LiftedAt       *string  `json:"lifted_at,omitempty"`
	LiftReason     *string  `json:"lift_reason,omitempty"`
	StatusIDs      []string `json:"status_ids"`
}

func newAccountActionData(a appmoderation.Action) accountActionData {
	return accountActionData{
		ID: a.ID, AccountID: a.TargetAccountID, Kind: string(a.Kind), ReasonCategory: string(a.ReasonCategory),
		PublicReason: optString(a.PublicReason), PrivateNote: optString(a.PrivateNote), ReportID: optString(a.ReportID),
		Issuer: string(a.Issuer), NotifyTarget: a.NotifyTarget, State: "active", IssuedAt: timestamp(a.IssuedAt),
		ExpiresAt: optTimestamp(a.ExpiresAt), StatusIDs: list(a.StatusIDs),
	}
}

// appealData is the data of appeal.created (WebhookAppeal).
type appealData struct {
	ID          string `json:"id"`
	AccountID   string `json:"account_id"`
	ActionID    string `json:"action_id"`
	Text        string `json:"text"`
	State       string `json:"state"`
	SubmittedAt string `json:"submitted_at"`
}

// domainActionData is the data of domain_moderation_action.* (WebhookDomainAction).
type domainActionData struct {
	ID            string  `json:"id"`
	Domain        string  `json:"domain"`
	Kind          string  `json:"kind"`
	PublicComment *string `json:"public_comment,omitempty"`
	PrivateNote   *string `json:"private_note,omitempty"`
	Obfuscate     bool    `json:"obfuscate"`
	State         string  `json:"state"`
	IssuedAt      string  `json:"issued_at"`
	ExpiresAt     *string `json:"expires_at,omitempty"`
	LiftedAt      *string `json:"lifted_at,omitempty"`
}

func newDomainActionData(a appmoderation.DomainAction) domainActionData {
	return domainActionData{
		ID: a.ID, Domain: a.Domain, Kind: string(a.Kind), PublicComment: optString(a.PublicComment),
		PrivateNote: optString(a.PrivateNote), Obfuscate: a.Obfuscate, State: "active", IssuedAt: timestamp(a.IssuedAt),
		ExpiresAt: optTimestamp(a.ExpiresAt),
	}
}

// statusData is the data of status.created (WebhookStatus).
type statusData struct {
	ID           string  `json:"id"`
	URI          string  `json:"uri"`
	AccountID    string  `json:"account_id"`
	Visibility   string  `json:"visibility"`
	InReplyToURI *string `json:"in_reply_to_uri,omitempty"`
	SpoilerText  string  `json:"spoiler_text"`
	Sensitive    bool    `json:"sensitive"`
	Language     string  `json:"language"`
	Content      string  `json:"content"`
	CreatedAt    string  `json:"created_at"`
}

// customEmojiData is the data of custom_emoji.* (WebhookCustomEmoji).
type customEmojiData struct {
	ID        string  `json:"id"`
	Shortcode string  `json:"shortcode"`
	Local     bool    `json:"local"`
	Domain    *string `json:"domain,omitempty"`
	URI       *string `json:"uri,omitempty"`
	Operation string  `json:"operation"`
	CreatedAt string  `json:"created_at"`
}

// testData is the data of webhook.test (WebhookTest).
type testData struct {
	EndpointID string `json:"endpoint_id"`
}

// customEmojiEvents maps the operations on custom emoji to their events (ADR-0060).
var customEmojiEvents = map[string]webhook.EventType{
	appcontent.CustomEmojiCreated:       webhook.EventCustomEmojiCreated,
	appcontent.CustomEmojiImageReplaced: webhook.EventCustomEmojiUpdated,
	appcontent.CustomEmojiUpdated:       webhook.EventCustomEmojiUpdated,
	appcontent.CustomEmojiEnabled:       webhook.EventCustomEmojiUpdated,
	appcontent.CustomEmojiDisabled:      webhook.EventCustomEmojiUpdated,
	appcontent.CustomEmojiDeleted:       webhook.EventCustomEmojiDeleted,
}

// message turns a domain event of another module into the type and data of a message, and
// false for an event that sends nothing.
type message func(e uow.Event) (webhook.EventType, any, time.Time, bool)

// sources lists the domain events that become messages.
var sources = map[string]message{
	appmoderation.EventReportFiled: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		r := e.(appmoderation.ReportFiled).Report
		return webhook.EventReportCreated, newReportData(r, "open", "", r.FiledAt), r.FiledAt, true
	},
	appmoderation.EventReportResolved: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		ev := e.(appmoderation.ReportResolved)
		at := ev.Resolution.ResolvedAt
		return webhook.EventReportResolved, newReportData(ev.Report, "resolved", string(ev.Resolution.Outcome), at), at, true
	},
	appmoderation.EventReportReopened: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		ev := e.(appmoderation.ReportReopened)
		at := ev.Reopening.ReopenedAt
		return webhook.EventReportReopened, newReportData(ev.Report, "open", "", at), at, true
	},
	appidentity.EventRegistrationPending: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		ev := e.(appidentity.RegistrationPending)
		return webhook.EventRegistrationPending, registrationData{
			UserID: ev.UserID, AccountID: ev.AccountID, Username: ev.Username, State: "pending", Since: timestamp(ev.SubmittedAt),
		}, ev.SubmittedAt, true
	},
	appidentity.EventRegistrationApproved: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		ev := e.(appidentity.RegistrationApproved)
		return webhook.EventAccountCreated, registrationData{
			UserID: ev.UserID, AccountID: ev.AccountID, Username: ev.Username, State: "approved", Since: timestamp(ev.ApprovedAt),
		}, ev.ApprovedAt, true
	},
	appaccount.EventProfileRevised: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		ev := e.(appaccount.ProfileRevised)
		// Only the changes of a local profile after the one it is created with (ADR-0060).
		if !ev.Federates() {
			return "", nil, time.Time{}, false
		}
		r := ev.Revision
		d := accountData{
			ID: r.AccountID, DisplayName: r.DisplayName, Note: r.NoteHTML, Locked: r.Locked, Bot: r.Bot,
			Discoverable: r.Discoverable, Indexable: r.Indexable, Fields: []accountFieldData{}, UpdatedAt: timestamp(r.RevisedAt),
		}
		for _, f := range r.Fields {
			d.Fields = append(d.Fields, accountFieldData{Name: f.Name, Value: f.ValueHTML})
		}
		return webhook.EventAccountUpdated, d, r.RevisedAt, true
	},
	appmoderation.EventAccountActionIssued: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		a := e.(appmoderation.AccountActionIssued).Action
		return webhook.EventModerationActionCreated, newAccountActionData(a), a.IssuedAt, true
	},
	appmoderation.EventAccountActionLifted: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		ev := e.(appmoderation.AccountActionLifted)
		d := newAccountActionData(ev.Action)
		d.State, d.LiftedAt, d.LiftReason = "lifted", optTimestamp(&ev.Lift.LiftedAt), optString(ev.Lift.Reason)
		return webhook.EventModerationActionLifted, d, ev.Lift.LiftedAt, true
	},
	appmoderation.EventAppealFiled: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		a := e.(appmoderation.AppealFiled).Appeal
		return webhook.EventAppealCreated, appealData{
			ID: a.ID, AccountID: a.AccountID, ActionID: a.ActionID, Text: a.Text, State: "pending", SubmittedAt: timestamp(a.SubmittedAt),
		}, a.SubmittedAt, true
	},
	appmoderation.EventDomainActionIssued: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		a := e.(appmoderation.DomainActionIssued).Action
		return webhook.EventDomainModerationActionCreated, newDomainActionData(a), a.IssuedAt, true
	},
	appmoderation.EventDomainActionLifted: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		ev := e.(appmoderation.DomainActionLifted)
		d := newDomainActionData(ev.Action)
		d.State, d.LiftedAt = "lifted", optTimestamp(&ev.Lift.LiftedAt)
		return webhook.EventDomainModerationActionLifted, d, ev.Lift.LiftedAt, true
	},
	appcontent.EventStatusCreated: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		ev := e.(appcontent.StatusCreated)
		st, rev := ev.Status, ev.Revision
		// Only the public posts of local accounts (ADR-0060, ADR-0017).
		if !st.IsLocal || st.Visibility != appcontent.VisibilityPublic {
			return "", nil, time.Time{}, false
		}
		d := statusData{
			ID: st.ID, URI: st.URI, AccountID: st.AuthorAccountID, Visibility: string(st.Visibility),
			InReplyToURI: optString(st.InReplyToURI), SpoilerText: rev.SpoilerText, Sensitive: rev.Sensitive,
			Language: rev.Language, CreatedAt: timestamp(st.PublishedAt),
		}
		for _, b := range rev.Bodies {
			if b.Language == rev.Language || d.Content == "" {
				d.Content = b.ContentHTML
			}
		}
		return webhook.EventStatusCreated, d, st.PublishedAt, true
	},
	appcontent.EventCustomEmojiAdministered: func(e uow.Event) (webhook.EventType, any, time.Time, bool) {
		ev := e.(appcontent.CustomEmojiAdministered)
		t, ok := customEmojiEvents[ev.Action]
		if !ok {
			return "", nil, time.Time{}, false
		}
		em := ev.Emoji
		local := em.Local()
		d := customEmojiData{
			ID: em.ID, Shortcode: em.Shortcode, Local: local, URI: optString(em.URI), Operation: ev.Action,
			CreatedAt: timestamp(em.CreatedAt),
		}
		if !local {
			d.Domain = &em.Domain
		}
		return t, d, ev.At, true
	},
}

// RegisterSources registers the projectors that turn the domain events of the other modules into
// messages, in the transactions that emit them. Register them after the projectors of those
// modules.
func (s *Service) RegisterSources(r *uow.Projectors) {
	for eventType, build := range sources {
		r.Register(eventType, func(ctx context.Context, tx uow.Tx, e uow.Event) error {
			t, data, at, ok := build(e)
			if !ok {
				return nil
			}
			endpoints, err := s.d.Repository.SubscribedEndpoints(ctx, tx, t)
			if err != nil || len(endpoints) == 0 {
				return err
			}
			_, err = s.send(ctx, tx, t, data, at, endpoints)
			return err
		})
	}
}

// send records in tx a message of type t with data to the endpoints, and enqueues a
// webhook.deliver job per delivery. It returns the last delivery.
func (s *Service) send(ctx context.Context, tx uow.Tx, t webhook.EventType, data any, at time.Time, endpoints []string) (webhook.Delivery, error) {
	if at.IsZero() {
		at = s.d.Clock.Now()
	}
	body, err := json.Marshal(envelope{Type: t, Timestamp: timestamp(at), Data: data})
	if err != nil {
		return webhook.Delivery{}, fmt.Errorf("webhook: the body of %s: %w", t, err)
	}
	now := s.d.Clock.Now()
	m := webhook.Message{ID: s.d.IDs.NewID(), EventType: t, Body: body, CreatedAt: now}
	if err := s.d.Repository.InsertMessage(ctx, tx, m); err != nil {
		return webhook.Delivery{}, err
	}
	var last webhook.Delivery
	jobs := make([]queue.NewJob, 0, len(endpoints))
	for _, id := range endpoints {
		last = webhook.Delivery{ID: s.d.IDs.NewID(), MessageID: m.ID, EndpointID: id, CreatedAt: now}
		if err := s.d.Repository.InsertDelivery(ctx, tx, last); err != nil {
			return webhook.Delivery{}, err
		}
		job, err := deliverJob(last.ID, "")
		if err != nil {
			return webhook.Delivery{}, err
		}
		jobs = append(jobs, job)
	}
	return last, s.d.Outbox.EnqueueTx(ctx, tx, jobs...)
}
