// SPDX-License-Identifier: AGPL-3.0-only
// The mutation of the server settings screen (web-client.md S1): the instance's display name in some locales,
// through the instance texts (PUT /admin/instance_texts/name/{locale}, manage_settings). The name names the
// instance to people in place of its domain; GET /instance reads it in the reader's locale. The texts and the
// instance are read with the queries of api/documents.ts.
import type { QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import { cacheSavedDocument, type InstanceText } from "./documents";

/** The most characters of the instance's name (InstanceTextKey `name`), counted in code points. */
export const instanceNameLimit = 100;

/** The instance's name in one locale; an empty `body` clears it there. */
export type InstanceNameChange = { locale: string; body: string };

/**
 * Sets the instance's name in each locale of the changes (one audited change each), as the options of
 * useMutation. Returns the names as recorded.
 */
export function setInstanceNamesMutation(api: Api) {
  return {
    mutationKey: ["put", "/admin/instance_texts/{key}/{locale}"] as const,
    mutationFn: (changes: readonly InstanceNameChange[]): Promise<InstanceText[]> =>
      Promise.all(
        changes.map(async ({ locale, body }) => {
          const { data, error } = await api.fetch.PUT("/admin/instance_texts/{key}/{locale}", { params: { path: { key: "name", locale } }, body: { body } });
          // The middleware throws an ApiError for every error status; this narrows the type.
          if (data === undefined) throw new ApiError(error);
          return data;
        }),
      ),
  };
}

/**
 * Marks what a new name changes (ADR-0045): the texts, GET /instance (the navigation, the tab title and the
 * sign-in page show its name) and the audit log, the same queries as a new document.
 */
export function cacheSavedName(queryClient: QueryClient): Promise<void> {
  return cacheSavedDocument(queryClient);
}
