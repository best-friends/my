// SPDX-License-Identifier: AGPL-3.0-only

package db

// toNullString maps the empty string, which stands for "none" in the queue ports, to NULL.
func toNullString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
