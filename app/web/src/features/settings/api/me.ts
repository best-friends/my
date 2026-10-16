// SPDX-License-Identifier: AGPL-3.0-only
// The queries of the user's own profile and preferences (web-client.md S1). Their keys come from the
// openapi-react-query client, so the account here shares its cache entry with the session's viewer.
import type { Api } from "../../../lib/api/client";
import type { components } from "../../../lib/api/schema.gen";

type Schemas = components["schemas"];

export type Preferences = Schemas["Preferences"];
export type PreferencesChange = Schemas["UpdatePreferencesRequest"];
export type Visibility = Schemas["Visibility"];
export type CredentialAccount = Schemas["CredentialAccount"];
export type ProfileChange = Schemas["UpdateCredentialsRequest"];

/** The signed-in user's preferences (GET /preferences). */
export function preferencesQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/preferences");
}

/** The signed-in user's own account with the source of its profile (GET /accounts/verify_credentials). */
export function credentialsQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/accounts/verify_credentials");
}

/**
 * Saves a change of the preferences (PATCH /preferences), as the options of TanStack Query's useMutation.
 * The client's own useMutation is a hook read from an object, which React Compiler cannot compile around
 * (react.dev, "Don't dynamically use Hooks").
 */
export function updatePreferencesMutation(api: Api) {
  return {
    mutationKey: ["patch", "/preferences"] as const,
    mutationFn: async (change: PreferencesChange): Promise<Preferences> => {
      const { data, error } = await api.fetch.PATCH("/preferences", { body: change });
      // The middleware throws an ApiError for every error status, so this is only for the types.
      if (data === undefined) throw new Error(error.title);
      return data;
    },
  };
}

/** Saves a change of the profile (PATCH /accounts/update_credentials), as the options of useMutation. */
export function updateProfileMutation(api: Api) {
  return {
    mutationKey: ["patch", "/accounts/update_credentials"] as const,
    mutationFn: async (change: ProfileChange): Promise<CredentialAccount> => {
      const { data, error } = await api.fetch.PATCH("/accounts/update_credentials", { body: change });
      // The middleware throws an ApiError for every error status, so this is only for the types.
      if (data === undefined) throw new Error(error.title);
      return data;
    },
  };
}
