// SPDX-License-Identifier: AGPL-3.0-only

// Package supporters holds the rules of the supporter plans and their subscriptions (ADR-0072,
// FR-910 to FR-912): what a plan is and how its yearly price follows from the monthly one, how
// the plans rank, which change of plan applies at once and which at the end of the period, and
// how the state of a subscription follows from the provider's snapshot and our own facts, with
// the role that each state carries. It knows nothing of the provider nor of the database.
package supporters
