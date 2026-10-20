// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the server settings screen (web-client.md S1): the instance's texts in some locales
// (its display name and its descriptions, PUT /admin/instance_texts/{key}/{locale}), and the instance settings
// (GET and PATCH /admin/settings): the registration mode, the default language, the public timelines, the
// federation and the publication of the domain actions, all for manage_settings. GET /instance reads the name,
// the descriptions, the registration mode and the timelines in the reader's locale. The texts and the instance
// are read with the queries of api/documents.ts.
import type { QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import type { components } from "../../../lib/api/schema.gen";
import { ApiError } from "../../../lib/errors";
import { refreshAuditLog } from "../../moderation";
import { cacheSavedDocument, type InstanceText } from "./documents";

type Schemas = components["schemas"];

export type InstanceSettingKey = Schemas["InstanceSettingKey"];
export type InstanceSettingList = Schemas["AdminSettingList"];

/** The most characters of the instance's name (InstanceTextKey `name`), counted in code points. */
export const instanceNameLimit = 100;

/** The most characters of the short and full descriptions (internal/domain/instance, text.go). */
export const shortDescriptionLimit = 500;
export const descriptionLimit = 20_000;

/** The texts of the server settings screen, edited in each language. */
type SettingsTextKey = Extract<Schemas["InstanceTextKey"], "name" | "short_description" | "description">;

/** A text in one locale; an empty `body` clears it there. */
export type InstanceTextChange = { key: SettingsTextKey; locale: string; body: string };

/**
 * Sets each text of the changes in its locale (one audited change each), as the options of useMutation. Returns
 * the texts as recorded.
 */
export function setInstanceTextsMutation(api: Api) {
  return {
    mutationKey: ["put", "/admin/instance_texts/{key}/{locale}"] as const,
    mutationFn: (changes: readonly InstanceTextChange[]): Promise<InstanceText[]> =>
      Promise.all(
        changes.map(async ({ key, locale, body }) => {
          const { data, error } = await api.fetch.PUT("/admin/instance_texts/{key}/{locale}", { params: { path: { key, locale } }, body: { body } });
          // The middleware throws an ApiError for every error status; this narrows the type.
          if (data === undefined) throw new ApiError(error);
          return data;
        }),
      ),
  };
}

/**
 * Marks what new texts change (ADR-0045): the texts, GET /instance (the navigation, the tab title, the sign-in page
 * and the about page show the name and the descriptions) and the audit log, the same queries as a new document.
 */
export function cacheSavedTexts(queryClient: QueryClient): Promise<void> {
  return cacheSavedDocument(queryClient);
}

/** Every instance setting with what it accepts (GET /admin/settings). */
export function instanceSettingsQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/admin/settings");
}

/**
 * Sets some instance settings together, all or none (PATCH /admin/settings, one audited change each), as the
 * options of useMutation. Returns every setting as it is now.
 */
export function updateInstanceSettingsMutation(api: Api) {
  return {
    mutationKey: ["patch", "/admin/settings"] as const,
    mutationFn: async (values: Readonly<Partial<Record<InstanceSettingKey, string>>>): Promise<InstanceSettingList> => {
      const { data, error } = await api.fetch.PATCH("/admin/settings", { body: { values } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/**
 * Writes the settings from the answer of a change (ADR-0045), and marks what they change: GET /instance (the
 * registration mode and the timelines that the navigation offers) and the audit log, fetched again and awaited
 * while shown.
 */
export async function cacheSavedSettings(queryClient: QueryClient, api: Api, settings: InstanceSettingList): Promise<void> {
  queryClient.setQueryData(instanceSettingsQueryOptions(api).queryKey, settings);
  await Promise.all([queryClient.invalidateQueries({ queryKey: ["get", "/instance"] }), refreshAuditLog(queryClient)]);
}
