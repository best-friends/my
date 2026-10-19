// SPDX-License-Identifier: AGPL-3.0-only
// The navigation's admin entry and the counts on it (FR-712): whether the viewer may use an admin screen, and
// how many open reports, registrations awaiting approval, and appeals awaiting a decision wait for them. The counts load only for a viewer
// whose roles have a permission that one of them needs; the server leaves out the others.
import { useQuery } from "@tanstack/react-query";
import { useApi, useSessionStatus } from "../../../lib/auth/SessionProvider";
import { shellQueryRetry } from "../../../lib/query";
import type { Permission } from "../api/admin";
import { type AdminCounts, adminCountsQueryOptions, countPermissions } from "../api/counts";
import { useViewerPermissions } from "./usePermissions";

/** The permissions of the admin screens (AdminMenu): a viewer with one of them has an admin screen to open. */
const screenPermissions: ReadonlySet<Permission> = new Set([
  "manage_domains",
  "manage_jobs",
  "view_audit",
  "manage_accounts",
  "manage_reports",
  "manage_settings",
]);

/**
 * The counts the viewer may see: none while signed out, while they load, and when they cannot be loaded (the
 * badges then stay away rather than showing a guess). Counts loaded before stay while a refetch fails.
 */
export function useAdminCounts(): AdminCounts {
  const api = useApi();
  const status = useSessionStatus();
  const permissions = useViewerPermissions();
  const counted = Object.values(countPermissions).some((permission) => permissions.includes(permission));
  const counts = useQuery({ ...adminCountsQueryOptions(api), enabled: status === "signed-in" && counted, retry: shellQueryRetry });
  return status === "signed-in" && counted ? (counts.data ?? {}) : {};
}

/** The navigation's admin entry: whether it shows, and the number of things that await the viewer there. */
type AdminEntry = { available: boolean; pending: number };

export function useAdminEntry(): AdminEntry {
  const permissions = useViewerPermissions();
  const counts = useAdminCounts();
  return {
    available: permissions.some((permission) => screenPermissions.has(permission)),
    pending: (counts.open_reports ?? 0) + (counts.pending_registrations ?? 0) + (counts.pending_appeals ?? 0),
  };
}
