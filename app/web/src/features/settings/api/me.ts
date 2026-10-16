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
