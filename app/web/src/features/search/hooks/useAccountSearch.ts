// SPDX-License-Identifier: AGPL-3.0-only
// The account search form's logic (web-client.md A4): the typed text, sending it, and why the last search
// found nothing. The hook returns no JSX.
import { useAnnouncer } from "@my/ui";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type SubmitEvent, useState } from "react";
import { useApi, useSessionStatus } from "../../../lib/auth/SessionProvider";
import { ApiError, errorMessage } from "../../../lib/errors";
import { type Translate, useT } from "../../../lib/i18n";
import { accountSearchMutationKey, lookupAccount, storeFoundAccount } from "../api/lookup";

export type AccountSearchOptions = {
  /** The text the field starts with: the search in the page's address (web-client.md S3). */
  initialQuery: string;
  /** Called with the found account's `acct` and the text that found it, to open its profile. */
  onFound: (acct: string, query: string) => void;
};

export type AccountSearchState = {
  value: string;
  onChange: (value: string) => void;
  /** Sends the search: the form's submit handler. */
  submit: (event: SubmitEvent<HTMLFormElement>) => void;
  isPending: boolean;
  /** Why the search of the text in the field found nothing (localized), or null. */
  error: string | null;
  /** Whether the viewer is signed out, so that only the accounts this server knows can be found. */
  signedOut: boolean;
};

/** The localized reason a search failed. 404 and 429 come from the client's catalog (web-client.md E2). */
function failureOf(t: Translate, error: unknown, signedOut: boolean): string {
  if (error instanceof ApiError && error.status === 404) {
    return signedOut ? t("client:search.not_found_signed_out") : t("client:search.not_found");
  }
  if (error instanceof ApiError && error.status === 429) return t("client:search.rate_limited");
  return t("client:search.failed", { reason: errorMessage(error) });
}

/** Returns the search form's state; a found account goes to `onFound`, a failure stays with the text that caused it. */
export function useAccountSearch({ initialQuery, onFound }: AccountSearchOptions): AccountSearchState {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const announce = useAnnouncer();
  const session = useSessionStatus();
  const [value, setValue] = useState(initialQuery);
  // Resolving needs a token; while the session is still resuming, the request waits for it (lib/api/client.ts).
  const signedOut = session === "signed-out";
  const mutation = useMutation({
    mutationKey: accountSearchMutationKey,
    mutationFn: ({ query, resolve }: { query: string; resolve: boolean }) => lookupAccount(api, query, resolve),
    onSuccess: (account, { query }) => {
      storeFoundAccount(queryClient, api, account);
      onFound(account.acct, query);
    },
    onError: (error, { resolve }) => {
      // The message is linked to the field, which keeps the focus; a screen reader hears it at once.
      announce(failureOf(t, error, !resolve), "assertive");
    },
  });

  const query = value.trim();
  return {
    value,
    onChange: setValue,
    submit: (event) => {
      event.preventDefault();
      if (query === "" || mutation.isPending) return;
      mutation.mutate({ query, resolve: !signedOut });
    },
    isPending: mutation.isPending,
    // The failure belongs to the text that caused it: editing the text clears it.
    error: mutation.isError && mutation.variables.query === query ? failureOf(t, mutation.error, !mutation.variables.resolve) : null,
    signedOut,
  };
}
