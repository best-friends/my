// SPDX-License-Identifier: AGPL-3.0-only

package testkit

import (
	"context"
	"sync"

	"github.com/best-friends/my/internal/app/kernel"
)

// Mailer is a kernel.Mailer that keeps the messages it is given. Set Err to make Send fail.
// It is safe for concurrent use.
type Mailer struct {
	mu   sync.Mutex
	sent []kernel.MailMessage
	Err  error
}

var _ kernel.Mailer = (*Mailer)(nil)

// Send implements kernel.Mailer.
func (m *Mailer) Send(_ context.Context, msg kernel.MailMessage) error {
	if err := kernel.ValidateMail(msg); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Err != nil {
		return m.Err
	}
	m.sent = append(m.sent, msg)
	return nil
}

// Sent returns the messages sent so far, in order.
func (m *Mailer) Sent() []kernel.MailMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]kernel.MailMessage(nil), m.sent...)
}
