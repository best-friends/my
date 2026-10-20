// SPDX-License-Identifier: AGPL-3.0-only

// Package webhook is the pure domain model of the webhook module (ADR-0060, FR-722): the
// endpoints that the administrators set up, the events they subscribe to, the Standard Webhooks
// signature of a message, and the rules of delivery, retry, and automatic disabling.
// It holds entities, value objects, and rules, and imports only the standard library.
package webhook
