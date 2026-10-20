// SPDX-License-Identifier: AGPL-3.0-only
// Changing the user's password (FR-002). The server ends every other session and grant and keeps this one, so no
// cached data changes. Resetting a forgotten password is the server's own sign-in pages (web-client.md §8).
import type { Api } from "../../../lib/api/client";
import { ApiError } from "../../../lib/errors";

/** The lengths of a password that the server takes, in characters (identity.MinPasswordLength / MaxPasswordLength). */
export const passwordMinLength = 8;
export const passwordMaxLength = 128;

/** Changes the password after the current one confirms it (POST /accounts/password), as the options of useMutation. */
export function changePasswordMutation(api: Api) {
  return {
    mutationKey: ["post", "/accounts/password"] as const,
    mutationFn: async (body: { current_password: string; new_password: string }): Promise<void> => {
      await api.fetch.POST("/accounts/password", { body });
    },
  };
}

/** The problem type of a current password that is wrong (api/openapi.yaml, Problem). */
const wrongPasswordType = "/api/v1/problems/wrong-password";

/** Why a change failed, by what the user can do about it. */
export type PasswordChangeFailure = "wrong-password" | "rate-limited" | "other";

/** Classifies an error of a change (web-client.md E2). */
export function passwordChangeFailure(error: unknown): PasswordChangeFailure {
  if (!(error instanceof ApiError)) return "other";
  if (error.status === 429) return "rate-limited";
  return error.type === wrongPasswordType ? "wrong-password" : "other";
}
