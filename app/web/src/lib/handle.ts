// SPDX-License-Identifier: AGPL-3.0-only
// The handle an account is shown by (FR-006). The API's `acct` already tells local and remote accounts
// apart: it is `username` for an account of this server and `username@domain` for one of another server
// (api/openapi.yaml, Account.acct), so the client never compares domains itself.

/** Whether `acct` is an account of another server: only a remote account's `acct` has a domain. */
export function isRemoteAcct(acct: string): boolean {
  return acct.includes("@");
}

/** The handle of an account: `@alice` for a local account, `@bob@example.com` for a remote one. */
export function handleOf(acct: string): string {
  return `@${acct}`;
}
