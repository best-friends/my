// SPDX-License-Identifier: AGPL-3.0-only
// The queries and mutations of the review of registrations (FR-721): the sign-ups that await a moderator's
// decision, longest waiting first, and approving or rejecting one with an optional note for the moderators.
// The list follows the API's keyset pagination as an infinite query whose key ends with "pages" (web-client.md §10).
import { infiniteQueryOptions } from "@tanstack/react-query";
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";
import type { components } from "../../../lib/api/schema.gen";
import { pageQuery } from "./admin";

type Schemas = components["schemas"];

export type Registration = Schemas["AdminRegistration"];

/** What a moderator decides on a registration; each is the last segment of its operation's path. */
export type RegistrationDecision = "approve" | "reject";

/** The most characters of a decision's note (RegistrationDecisionRequest). */
export const registrationNoteLimit = 1000;

/** The registrations that await approval, longest waiting first (GET /admin/registrations). */
export function registrationsQueryOptions(api: Api) {
  return infiniteQueryOptions({
    queryKey: ["get", "/admin/registrations", "pages"] as const,
    queryFn: async ({ pageParam, signal }) => {
      const { data, error } = await api.fetch.GET("/admin/registrations", { params: { query: pageQuery(pageParam) }, signal });
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (page) => page.next_cursor,
  });
}

/** One decision: on whose registration, which, and the note for the moderators ("" for none). */
export type DecideRegistration = { userId: string; decision: RegistrationDecision; note: string };

/**
 * Approves or rejects a registration (POST /admin/registrations/{user_id}/approve | reject), as the options of
 * TanStack Query's useMutation (web-client.md §10). Returns the registration after it: a registration that was
 * decided already keeps its decision, so the state may differ from the decision sent.
 */
export function decideRegistrationMutation(api: Api) {
  return {
    mutationKey: ["post", "/admin/registrations/{user_id}"] as const,
    mutationFn: async ({ userId, decision, note }: DecideRegistration): Promise<Registration> => {
      const init = { params: { path: { user_id: userId } }, body: note === "" ? {} : { note } };
      const { data, error } =
        decision === "approve"
          ? await api.fetch.POST("/admin/registrations/{user_id}/approve", init)
          : await api.fetch.POST("/admin/registrations/{user_id}/reject", init);
      // The middleware throws an ApiError for every error status; this narrows the type.
      if (data === undefined) throw new ApiError(error);
      return data;
    },
  };
}

/** Whether a query is the moderators' counts (GET /admin/counts), which a decision changes. */
export function isAdminCountsQuery(queryKey: readonly unknown[]): boolean {
  return queryKey[1] === "/admin/counts";
}
