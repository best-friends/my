// SPDX-License-Identifier: AGPL-3.0-only
// The name that the app calls the instance by (the sidebar, "about this server", the tab title, the sign-in
// page): its display name, which administrators change in the admin screens, or its domain when it has none.
// Handles and links use the domain (`accountDomain` in the runtime config) whatever the name is.
import { useQuery } from "@tanstack/react-query";
import { useApi } from "./auth/SessionProvider";
import { useConfig } from "./config";
import { shellQueryReader } from "./query";

/**
 * The instance's name from GET /instance, in the viewer's language; until it loads, and when it cannot, the
 * name the server wrote into the shell (in the instance's default language). A change of the name shows once
 * GET /instance is fetched again, without a reload.
 */
export function useInstanceName(): string {
  const api = useApi();
  const { instanceName } = useConfig();
  // The shell reads GET /instance on every page and handles its failures; this only reads its answer.
  const instance = useQuery({ ...api.query.queryOptions("get", "/instance"), ...shellQueryReader });
  return instance.data?.name ?? instanceName;
}
