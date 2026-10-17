// SPDX-License-Identifier: AGPL-3.0-only
// The admin feature: the health of remote servers with the reasons of their state and resuming deliveries to
// them (federation.md §7.3), and the dead jobs of the job queue with running them again. Each screen is for a
// viewer whose roles have its permission (manage_domains, manage_jobs); the server checks it on every request.
// Routes and other features use it through this file only (web-client.md A3).
export {
  availabilityEventsQueryOptions,
  deadJobsQueryOptions,
  type InstanceFilter,
  instanceFilters,
  instanceQueryOptions,
  instancesQueryOptions,
  isMissingPermission,
  type Permission,
} from "./api/admin";
export { AdminMenu } from "./components/AdminMenu";
export { DeadJobList } from "./components/DeadJobList";
export { InstanceDetail } from "./components/InstanceDetail";
export { InstanceList } from "./components/InstanceList";
export { MissingPermission } from "./components/parts";
