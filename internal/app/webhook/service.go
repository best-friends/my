// SPDX-License-Identifier: AGPL-3.0-only

package webhook

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/best-friends/my/internal/app/kernel"
	appmoderation "github.com/best-friends/my/internal/app/moderation"
	"github.com/best-friends/my/internal/app/queue"
	"github.com/best-friends/my/internal/app/uow"
	"github.com/best-friends/my/internal/domain/webhook"
)

// MissingPermissionError reports a subscription to events that need permissions the user does
// not hold (ADR-0060: the person who sets up an endpoint needs the permission of every event).
type MissingPermissionError struct {
	Missing []webhook.Permission
}

func (e *MissingPermissionError) Error() string {
	names := make([]string, len(e.Missing))
	for i, p := range e.Missing {
		names[i] = string(p)
	}
	return "webhook: the events need the permissions " + strings.Join(names, ", ")
}

// Deps are the ports of a Service.
type Deps struct {
	UnitOfWork uow.UnitOfWork
	Repository Repository
	// Outbox enqueues the webhook.deliver jobs in the transaction of the messages.
	Outbox queue.Outbox
	// Sealer seals the secrets (ADR-0019 §6).
	Sealer *kernel.Sealer
	// HTTP sends the deliveries through the SSRF guard with kernel.HTTPPurposeWebhook.
	HTTP  kernel.HTTPClient
	Clock kernel.Clock
	IDs   kernel.IDGenerator
	// Permissions returns the permissions that the roles of a user grant; the use cases check
	// the events of an endpoint against them. A "" user is the CLI, which holds every permission.
	Permissions func(ctx context.Context, userID string) ([]webhook.Permission, error)
	Logger      *slog.Logger
}

// Service runs the use cases of the webhooks.
type Service struct {
	d Deps
	// presenter presents the data of the messages (SetPresenter).
	presenter Presenter
}

// NewService returns a Service.
func NewService(d Deps) *Service {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	return &Service{d: d}
}

// Created is a new endpoint with its secret, which is shown only now.
type Created struct {
	Endpoint webhook.Endpoint
	Secret   string
}

// List returns the endpoints that are not deleted, newest first.
func (s *Service) List(ctx context.Context) ([]webhook.Endpoint, error) {
	return s.d.Repository.ListEndpoints(ctx)
}

// Get returns an endpoint; it fails with ErrNotFound for an unknown or deleted one.
func (s *Service) Get(ctx context.Context, id string) (webhook.Endpoint, error) {
	return s.d.Repository.FindEndpoint(ctx, id)
}

// checkPermissions fails with a MissingPermissionError when byUserID lacks a permission that the
// events need.
func (s *Service) checkPermissions(ctx context.Context, byUserID string, events []webhook.EventType) error {
	if byUserID == "" || s.d.Permissions == nil {
		return nil
	}
	granted, err := s.d.Permissions(ctx, byUserID)
	if err != nil {
		return fmt.Errorf("webhook: the permissions of %s: %w", byUserID, err)
	}
	if missing := webhook.MissingPermissions(events, granted); len(missing) > 0 {
		return &MissingPermissionError{Missing: missing}
	}
	return nil
}

// newSecret returns a new random secret and its sealed row.
func (s *Service) newSecret(ctx context.Context, endpointID, byUserID string, now time.Time) ([]byte, webhook.Secret, error) {
	secret := make([]byte, webhook.SecretSize)
	_, _ = rand.Read(secret) // crypto/rand.Read never returns an error
	row := webhook.Secret{ID: s.d.IDs.NewID(), EndpointID: endpointID, CreatedByUserID: byUserID, CreatedAt: now}
	sealed, err := s.d.Sealer.Seal(ctx, secret, SecretAAD(row.ID))
	if err != nil {
		return nil, webhook.Secret{}, fmt.Errorf("webhook: seal the secret: %w", err)
	}
	row.Sealed = sealed
	return secret, row, nil
}

// SecretAAD is the additional authenticated data of a sealed secret: its row and column.
func SecretAAD(secretID string) []byte {
	return kernel.AAD("webhook_secrets", secretID, "sealed_secret")
}

// Create sets up an endpoint, enabled, with a new secret, and audits it. byUserID is the
// administrator, "" for the CLI. It fails with errors of webhook.EndpointSettings.Validate and
// with a MissingPermissionError.
func (s *Service) Create(ctx context.Context, settings webhook.EndpointSettings, byUserID string) (Created, error) {
	settings = settings.Normalize()
	if err := settings.Validate(); err != nil {
		return Created{}, err
	}
	if err := s.checkPermissions(ctx, byUserID, settings.Events); err != nil {
		return Created{}, err
	}
	now := kernel.Normalize(s.d.Clock.Now())
	id := s.d.IDs.NewID()
	rev := Revision{ID: s.d.IDs.NewID(), EndpointID: id, Settings: settings, RevisedByUserID: byUserID, RevisedAt: now}
	change := StateChange{
		ID: s.d.IDs.NewID(), EndpointID: id, State: webhook.StateEnabled, Reason: webhook.ReasonAdmin,
		ChangedByUserID: byUserID, ChangedAt: now,
	}
	secret, secretRow, err := s.newSecret(ctx, id, byUserID, now)
	if err != nil {
		return Created{}, err
	}
	auditID := s.d.IDs.NewID()
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		repo := s.d.Repository
		if err := repo.InsertEndpoint(ctx, tx, id, byUserID, now); err != nil {
			return err
		}
		if err := repo.InsertRevision(ctx, tx, rev); err != nil {
			return err
		}
		if err := repo.InsertSecret(ctx, tx, secretRow); err != nil {
			return err
		}
		if err := repo.InsertStateChange(ctx, tx, change); err != nil {
			return err
		}
		head := Head{EndpointID: id, RevisionID: rev.ID, State: webhook.StateEnabled, SecretID: secretRow.ID}
		if err := tx.Emit(ctx, EndpointChanged{Head: head}); err != nil {
			return err
		}
		return s.emitAudit(ctx, tx, auditID, appmoderation.AuditWebhookCreated, id, settings, rev.ID, byUserID, now)
	})
	if err != nil {
		return Created{}, err
	}
	e, err := s.Get(ctx, id)
	if err != nil {
		return Created{}, err
	}
	return Created{Endpoint: e, Secret: webhook.FormatSecret(secret)}, nil
}

// emitAudit emits the audit log entry of an operation on an endpoint, labeled with its
// description and host name (ADR-0060: never the URL). byUserID "" is the CLI.
func (s *Service) emitAudit(ctx context.Context, tx uow.Tx, id string, action appmoderation.AuditAction, endpointID string,
	settings webhook.EndpointSettings, source, byUserID string, at time.Time,
) error {
	entry := appmoderation.AuditEntry{
		ID: id, Action: action, TargetKind: appmoderation.AuditTargetWebhook, TargetID: endpointID,
		TargetLabel: webhook.AuditLabel(settings), SourceEventID: source, OccurredAt: at,
	}
	entry.ActorKind, entry.ActorUserID = appmoderation.AuditActor(byUserID)
	return tx.Emit(ctx, appmoderation.AuditRecorded{Entry: entry})
}

// current returns the head of an endpoint in tx, failing with ErrDeleted for a deleted one.
func (s *Service) current(ctx context.Context, tx uow.Tx, id string) (Head, error) {
	h, err := s.d.Repository.HeadForUpdate(ctx, tx, id)
	if err != nil {
		return Head{}, err
	}
	if h.State == webhook.StateDeleted {
		return Head{}, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return h, nil
}

// Update replaces the settings of an endpoint with a new revision, unless they are the same, and
// audits it. It fails as Create does, and with ErrNotFound.
func (s *Service) Update(ctx context.Context, id string, settings webhook.EndpointSettings, byUserID string) (webhook.Endpoint, error) {
	settings = settings.Normalize()
	if err := settings.Validate(); err != nil {
		return webhook.Endpoint{}, err
	}
	if err := s.checkPermissions(ctx, byUserID, settings.Events); err != nil {
		return webhook.Endpoint{}, err
	}
	old, err := s.Get(ctx, id)
	if err != nil {
		return webhook.Endpoint{}, err
	}
	if old.Settings.URL == settings.URL && old.Settings.Description == settings.Description &&
		slices.Equal(old.Settings.Events, settings.Events) {
		return old, nil
	}
	now := kernel.Normalize(s.d.Clock.Now())
	rev := Revision{ID: s.d.IDs.NewID(), EndpointID: id, Settings: settings, RevisedByUserID: byUserID, RevisedAt: now}
	auditID := s.d.IDs.NewID()
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		h, err := s.current(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.d.Repository.InsertRevision(ctx, tx, rev); err != nil {
			return err
		}
		h.RevisionID = rev.ID
		if err := tx.Emit(ctx, EndpointChanged{Head: h}); err != nil {
			return err
		}
		return s.emitAudit(ctx, tx, auditID, appmoderation.AuditWebhookUpdated, id, settings, rev.ID, byUserID, now)
	})
	if err != nil {
		return webhook.Endpoint{}, err
	}
	return s.Get(ctx, id)
}

// Enable enables an endpoint; an enabled one stays as it is. The deliveries made while it was
// disabled are not sent again.
func (s *Service) Enable(ctx context.Context, id, byUserID string) (webhook.Endpoint, error) {
	return s.changeState(ctx, id, webhook.StateEnabled, byUserID)
}

// Disable disables an endpoint; a disabled one stays as it is.
func (s *Service) Disable(ctx context.Context, id, byUserID string) (webhook.Endpoint, error) {
	return s.changeState(ctx, id, webhook.StateDisabled, byUserID)
}

// Delete deletes an endpoint for good: it is no longer listed and receives nothing.
func (s *Service) Delete(ctx context.Context, id, byUserID string) error {
	_, err := s.changeState(ctx, id, webhook.StateDeleted, byUserID)
	return err
}

var stateActions = map[webhook.State]appmoderation.AuditAction{
	webhook.StateEnabled:  appmoderation.AuditWebhookEnabled,
	webhook.StateDisabled: appmoderation.AuditWebhookDisabled,
	webhook.StateDeleted:  appmoderation.AuditWebhookDeleted,
}

func (s *Service) changeState(ctx context.Context, id string, state webhook.State, byUserID string) (webhook.Endpoint, error) {
	old, err := s.Get(ctx, id)
	if err != nil {
		return webhook.Endpoint{}, err
	}
	now := kernel.Normalize(s.d.Clock.Now())
	change := StateChange{
		ID: s.d.IDs.NewID(), EndpointID: id, State: state, Reason: webhook.ReasonAdmin, ChangedByUserID: byUserID, ChangedAt: now,
	}
	auditID := s.d.IDs.NewID()
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		h, err := s.current(ctx, tx, id)
		if err != nil || h.State == state {
			return err
		}
		if err := s.d.Repository.InsertStateChange(ctx, tx, change); err != nil {
			return err
		}
		h.State = state
		if err := tx.Emit(ctx, EndpointChanged{Head: h}); err != nil {
			return err
		}
		return s.emitAudit(ctx, tx, auditID, stateActions[state], id, old.Settings, change.ID, byUserID, now)
	})
	if err != nil || state == webhook.StateDeleted {
		return webhook.Endpoint{}, err
	}
	return s.Get(ctx, id)
}

// RotateSecret gives an endpoint a new secret, shown only now, and audits it. The previous
// secret still signs beside it for webhook.SecretOverlap.
func (s *Service) RotateSecret(ctx context.Context, id, byUserID string) (string, error) {
	e, err := s.Get(ctx, id)
	if err != nil {
		return "", err
	}
	now := kernel.Normalize(s.d.Clock.Now())
	secret, row, err := s.newSecret(ctx, id, byUserID, now)
	if err != nil {
		return "", err
	}
	auditID := s.d.IDs.NewID()
	err = s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		h, err := s.current(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := s.d.Repository.InsertSecret(ctx, tx, row); err != nil {
			return err
		}
		h.SecretID = row.ID
		if err := tx.Emit(ctx, EndpointChanged{Head: h}); err != nil {
			return err
		}
		return s.emitAudit(ctx, tx, auditID, appmoderation.AuditWebhookSecretRotated, id, e.Settings, row.ID, byUserID, now)
	})
	if err != nil {
		return "", err
	}
	return webhook.FormatSecret(secret), nil
}

// Test sends a webhook.test message to one endpoint, whatever it subscribes to, and returns its
// delivery. A disabled endpoint receives nothing, as for every message. It is not audited.
func (s *Service) Test(ctx context.Context, id string) (webhook.DeliveryRecord, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return webhook.DeliveryRecord{}, err
	}
	now := kernel.Normalize(s.d.Clock.Now())
	var delivery webhook.Delivery
	err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error {
		if _, err := s.current(ctx, tx, id); err != nil {
			return err
		}
		var err error
		delivery, err = s.send(ctx, tx, webhook.EventTest, testData{EndpointID: id}, now, []string{id})
		return err
	})
	if err != nil {
		return webhook.DeliveryRecord{}, err
	}
	return s.d.Repository.FindDelivery(ctx, id, delivery.ID)
}

// Deliveries returns up to limit deliveries of an endpoint, newest first, before the delivery
// before ("" for the first page). It fails with ErrNotFound for an unknown or deleted endpoint.
func (s *Service) Deliveries(ctx context.Context, endpointID, before string, limit int) ([]webhook.DeliveryRecord, error) {
	if _, err := s.Get(ctx, endpointID); err != nil {
		return nil, err
	}
	return s.d.Repository.ListDeliveries(ctx, endpointID, before, limit)
}

// Redeliver sends a delivery of an endpoint again, with the same webhook-id, also after it
// succeeded or used up its attempts. It is not audited.
func (s *Service) Redeliver(ctx context.Context, endpointID, deliveryID string) (webhook.DeliveryRecord, error) {
	if _, err := s.Get(ctx, endpointID); err != nil {
		return webhook.DeliveryRecord{}, err
	}
	d, err := s.d.Repository.FindDelivery(ctx, endpointID, deliveryID)
	if err != nil {
		return webhook.DeliveryRecord{}, err
	}
	job, err := deliverJob(d.Delivery.ID, s.d.IDs.NewID())
	if err != nil {
		return webhook.DeliveryRecord{}, err
	}
	if err := s.d.UnitOfWork.Tx(ctx, func(tx uow.Tx) error { return s.d.Outbox.EnqueueTx(ctx, tx, job) }); err != nil {
		return webhook.DeliveryRecord{}, err
	}
	return d, nil
}

// IsValidation reports whether err is a rejection of the settings of an endpoint.
func IsValidation(err error) bool {
	return errors.Is(err, webhook.ErrInvalidURL) || errors.Is(err, webhook.ErrInvalidDescription) ||
		errors.Is(err, webhook.ErrInvalidEvents)
}
