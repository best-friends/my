// SPDX-License-Identifier: AGPL-3.0-only

// Package finance is the pure domain model of the finance module (ADR-0067, FR-901, FR-902):
// the instance's cost items, their payment schedules, price revisions, and ends, the calendar
// dates and the ISO 4217 amounts they are recorded in, and the rules that expand the schedules
// into payments, spread the payments over the dates they cover, and total them by week, month,
// and year. The totals are computed on read from the items; nothing here holds a projection.
// It imports only the standard library.
package finance
