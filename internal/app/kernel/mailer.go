// SPDX-License-Identifier: AGPL-3.0-only

package kernel

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"
)

// Mailer sends email (overview §3, the Mailer port). The adapters live in internal/adapter/mail:
// mail/none logs messages instead of sending them, and mail/smtp sends them over SMTP. Every
// adapter passes testkit.MailerContract.
//
// Send runs outside transactions (ADR-0003): callers commit first and send afterwards. A
// message may carry a secret, such as the link of a verification email, so adapters never log
// a message except mail/none, whose purpose is to show it to the operator.
type Mailer interface {
	// Send delivers m or fails. It fails with an error wrapping ErrInvalidMail when m does
	// not pass ValidateMail.
	Send(ctx context.Context, m MailMessage) error
}

// MailMessage is a plain-text email to one recipient.
type MailMessage struct {
	// To is a bare address (local@domain).
	To      string
	Subject string
	// Text is the body in plain text, with "\n" line ends.
	Text string
}

// ErrInvalidMail reports a message that ValidateMail rejects.
var ErrInvalidMail = errors.New("invalid mail message")

// maxSubjectLength bounds the subject in bytes; longer subjects are an error, not cut.
const maxSubjectLength = 998

// ValidateMail checks that m can be sent without header injection: To is one bare address
// and Subject is one line of UTF-8; Text is UTF-8.
func ValidateMail(m MailMessage) error {
	a, err := mail.ParseAddress(m.To)
	if err != nil || a.Name != "" || a.Address != m.To {
		return fmt.Errorf("%w: the recipient must be a bare address", ErrInvalidMail)
	}
	if m.Subject == "" || len(m.Subject) > maxSubjectLength || !utf8.ValidString(m.Subject) || strings.ContainsAny(m.Subject, "\r\n") {
		return fmt.Errorf("%w: the subject must be one line of 1 to %d bytes of UTF-8", ErrInvalidMail, maxSubjectLength)
	}
	if !utf8.ValidString(m.Text) {
		return fmt.Errorf("%w: the body must be UTF-8", ErrInvalidMail)
	}
	return nil
}
