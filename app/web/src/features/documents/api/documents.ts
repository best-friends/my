// SPDX-License-Identifier: AGPL-3.0-only
// The server data of the documents feature (web-client.md S1): the instance, whose `terms_url` and
// `privacy_policy_url` lead to the public pages of its terms of service and privacy policy.
import type { Api } from "../../../lib/api/client";

/** The instance, for the links to its documents. */
export function instanceQueryOptions(api: Api) {
  return api.query.queryOptions("get", "/instance");
}
