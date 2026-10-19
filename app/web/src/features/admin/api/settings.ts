// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the server settings screen (web-client.md S1): the instance texts in every
// locale, and changing the instance's display name in some of them (manage_settings). The name names the
// instance to people in place of its domain; GET /instance reads it in the reader's locale.
import type { QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import type { components } from "../../../lib/api/schema.gen";
import { ApiError } from "../../../lib/errors";
import { refreshQueries } from "./admin";

type Schemas = components["schemas"];

export type InstanceText = Schemas["AdminInstanceText"];

/** The most characters of the instance's name (AdminInstanceTextKey `name`), counted in code points. */
export const instanceNameLimit = 100;

/** This instance's public information (GET /instance), for its domain: the name stands in for it. */
export function ownInstanceQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/instance");
}

/** The texts of the instance in every locale that has one (GET /admin/instance/texts). */
export function instanceTextsQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/admin/instance/texts");
}

/** The instance's name in one locale; an empty `text` clears it there. */
export type InstanceNameChange = { locale: string; text: string };

/**
 * Sets the instance's name in each locale of the changes (PUT /admin/instance/texts/name/{locale}, one audited
 * change each), as the options of useMutation. Returns the names as recorded.
 */
export function setInstanceNamesMutation(api: Api) {
  return {
    mutationKey: ["put", "/admin/instance/texts/{key}/{locale}"] as const,
    mutationFn: (changes: readonly InstanceNameChange[]): Promise<InstanceText[]> =>
      Promise.all(
        changes.map(async ({ locale, text }) => {
          const { data, error } = await api.fetch.PUT("/admin/instance/texts/{key}/{locale}", {
            params: { path: { key: "name", locale } },
            body: { text },
          });
          // The middleware throws an ApiError for every error status; this narrows the type.
          if (data === undefined) throw new ApiError(error);
          return data;
        }),
      ),
  };
}

/**
 * Fetches again the instance texts and GET /instance in every language (refreshQueries): the navigation, the
 * tab title and the sign-in page show the name from GET /instance, so the new name shows there at once.
 */
export function refreshInstanceName(queryClient: QueryClient): Promise<void> {
  return refreshQueries(queryClient, { queryKey: ["get", "/admin/instance/texts"] }, { queryKey: ["get", "/instance"] });
}
