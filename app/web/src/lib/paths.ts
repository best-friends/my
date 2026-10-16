// SPDX-License-Identifier: AGPL-3.0-only
// The addresses of the app's pages that more than one feature links to. They follow the server's HTML
// shell routes (internal/interface/web, ADR-0028): a profile is `/@username` for a local account and
// `/@username@domain` for a remote one, the same as the `acct` of the client API's Account.

/** The profile page of the account whose `acct` is given. */
export function profilePath(acct: string): string {
  return `/@${acct}`;
}

/** The pages of the accounts that follow an account, and that an account follows. */
export function followListPath(acct: string, list: "followers" | "following"): string {
  return `/@${acct}/${list}`;
}
