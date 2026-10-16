// SPDX-License-Identifier: AGPL-3.0-only
// What a profile's header shows, as plain values for the components (web-client.md A4). The hook returns
// no JSX.
import { type SanitizedHtml, serverSanitizedHtml } from "@my/ui";
import { useSuspenseQuery } from "@tanstack/react-query";
import { useIntl } from "react-intl";
import { useApi } from "../../../lib/auth/SessionProvider";
import { useConfig } from "../../../lib/config";
import { followListPath } from "../../../lib/paths";
import { type Account, accountQueryOptions } from "../api/accounts";

export type ProfileView = {
  account: Account;
  /** The display name, or the username when there is none. */
  name: string;
  /** The full handle, `@alice@example.com`. */
  handle: string;
  /** The profile text as the server sanitized it, or null when there is none. */
  note: SanitizedHtml | null;
  /** The metadata fields: a plain-text name and a sanitized value. */
  fields: { key: string; name: string; value: SanitizedHtml }[];
  /** The follow counts and the pages that list them, or null where the server left a count out. */
  following: { count: number; href: string } | null;
  followers: { count: number; href: string } | null;
  /** When the account was created: the ISO date for `<time>` and the date to show. */
  joined: { dateTime: string; date: string };
};

/** The full handle of an account: `acct` is `username` for local accounts and `username@domain` for remote ones. */
export function handleOf(acct: string, localDomain: string): string {
  return acct.includes("@") ? `@${acct}` : `@${acct}@${localDomain}`;
}

/**
 * A name inside a sentence of the UI's language. It is the user's text in any script, so it is wrapped in
 * Unicode directional isolates (FSI … PDI): a right-to-left name does not reorder the sentence around it
 * (web-client.md Y10).
 */
export function isolate(name: string): string {
  return `⁨${name}⁩`;
}

/** The name an account goes by in the UI. */
export function nameOf(account: Account): string {
  return account.display_name || account.username;
}

/** Reads the account whose handle is `acct`; the route loader has fetched it, and a failure goes to its error boundary. */
export function useProfile(acct: string): ProfileView {
  const api = useApi();
  const intl = useIntl();
  const { accountDomain } = useConfig();
  const { data: account } = useSuspenseQuery(accountQueryOptions(api, acct));
  return {
    account,
    name: nameOf(account),
    handle: handleOf(account.acct, accountDomain),
    // `note` and the field values are sanitized HTML (api/openapi.yaml, Account.note and ProfileField.value).
    note: account.note === "" ? null : serverSanitizedHtml(account.note),
    // A profile's fields have no IDs; their order is their identity, and the list only changes as a whole.
    fields: account.fields.map((field, index) => ({ key: `${String(index)}:${field.name}`, name: field.name, value: serverSanitizedHtml(field.value) })),
    following: account.following_count === undefined ? null : { count: account.following_count, href: followListPath(account.acct, "following") },
    followers: account.followers_count === undefined ? null : { count: account.followers_count, href: followListPath(account.acct, "followers") },
    joined: { dateTime: account.created_at, date: intl.formatDate(account.created_at, { dateStyle: "long" }) },
  };
}
