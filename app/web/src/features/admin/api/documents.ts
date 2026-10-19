// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the terms and policies screen (web-client.md S1): the instance texts in every
// locale (GET /admin/instance_texts), writing one in one locale (PUT /admin/instance_texts/{key}/{locale}), and
// the instance, whose links to the public pages of the documents change with them.
import type { QueryClient } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";
import { isAuditLogQuery } from "./admin";

type Schemas = components["schemas"];

export type InstanceText = Schemas["AdminInstanceText"];

/** The instance texts that are documents, written in Markdown. */
export type DocumentKey = Extract<Schemas["InstanceTextKey"], "terms" | "privacy_policy">;

/** The most characters of a document in one locale (internal/domain/instance, text.go). */
export const documentTextLimit = 100_000;

/** Every current instance text, by key and locale. */
export function instanceTextsQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/admin/instance_texts");
}

/** The instance, whose `terms_url` and `privacy_policy_url` lead to the documents' public pages. */
export function instanceQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/instance");
}

/** A new text of a document in one locale; an empty body removes it there. */
export type DocumentRevision = { key: DocumentKey; locale: string; body: string };

/** Records a document's text in one locale, as the options of useMutation. Returns the text as recorded. */
export function setDocumentMutation(api: Api) {
  return {
    mutationKey: ["put", "/admin/instance_texts/{key}/{locale}"] as const,
    mutationFn: async ({ key, locale, body }: DocumentRevision): Promise<InstanceText> => {
      const { data, error } = await api.fetch.PUT("/admin/instance_texts/{key}/{locale}", { params: { path: { key, locale } }, body: { body } });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/**
 * Marks what a new text changes (ADR-0045): the texts, the instance's links to the documents' pages (which
 * every page shows), and the audit log, which records the change. The queries shown now are fetched again and
 * awaited; the others are fetched again by their route's loader when opened.
 */
export async function cacheSavedDocument(queryClient: QueryClient): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ["get", "/admin/instance_texts"] }),
    queryClient.invalidateQueries({ queryKey: ["get", "/instance"] }),
    queryClient.invalidateQueries({ predicate: (query) => isAuditLogQuery(query.queryKey) }),
  ]);
}
