// SPDX-License-Identifier: AGPL-3.0-only
// The admin feature: the health of remote servers with the reasons of their state and resuming deliveries to
// them (federation.md §7.3), the moderation actions on domains (FR-702), the dead jobs of the job queue with
// running them again, the audit log (FR-704), the review of registrations awaiting approval (FR-721), and the
// review of appeals of moderation actions (FR-707), the blocks on sign-ups (FR-719), and the server's settings (its name). Each
// screen is for a viewer whose roles have its permission (manage_domains, manage_jobs, view_audit,
// manage_accounts, manage_settings); the server checks it on every request. The navigation's admin entry and the menu show
// what awaits the moderators (FR-712).
// Routes and other features use it through this file only (web-client.md A3).
export {
  auditActions,
  type AuditLogFilter,
  auditLogMaxPages,
  auditLogQueryOptions,
  auditTargetKinds,
  availabilityEventsQueryOptions,
  deadJobsQueryOptions,
  type DomainActionFilter,
  domainActionFilters,
  domainActionsQueryOptions,
  type InstanceFilter,
  instanceFilters,
  instanceQueryOptions,
  instancesQueryOptions,
  isMissingPermission,
  type Permission,
} from "./api/admin";
export { refreshAdminCounts } from "./api/counts";
export { AdminMenu, AdminPages } from "./components/AdminMenu";
export { AuditLog } from "./components/AuditLog";
export { DeadJobList } from "./components/DeadJobList";
export { DomainActions } from "./components/DomainActions";
export { InstanceDetail } from "./components/InstanceDetail";
export { InstanceList } from "./components/InstanceList";
export { RegistrationList } from "./components/RegistrationList";
export { type SignupBlockFilter, signupBlockFilters, signupBlocksQueryOptions } from "./api/signupBlocks";
export { SignupBlocks } from "./components/SignupBlocks";
export { useAdminEntry } from "./hooks/useAdminCounts";
export { registrationsQueryOptions } from "./api/registrations";
export { DomainName, MissingPermission } from "./components/parts";
// The report queue and a report's page (FR-701, FR-710, NFR-MOD-4), for manage_reports.
export { type ReportFilter, reportQueryOptions, reportsQueryOptions } from "./api/reports";
export { ReportDetailView } from "./components/ReportDetail";
export { ReportQueue } from "./components/ReportQueue";
// The account search and an account's page with its moderation actions (FR-714, FR-703, FR-713), for manage_accounts.
export {
  adminAccountContactFields,
  adminAccountOrigins,
  adminAccountQueryOptions,
  type AdminAccountSearch,
  adminAccountSearchLimit,
  adminAccountsQueryOptions,
  adminAccountStates,
} from "./api/accounts";
export { AccountDetailView } from "./components/AccountDetail";
export { AccountSearch } from "./components/AccountSearch";
// The review of appeals of moderation actions (FR-707), for manage_accounts.
export { type AppealFilter, appealQueryOptions, appealsQueryOptions } from "./api/appeals";
export { AppealDetailView } from "./components/AppealDetail";
export { AppealQueue } from "./components/AppealQueue";
// The terms of service and the privacy policy (#489), for manage_settings.
export { instanceTextsQueryOptions } from "./api/documents";
export { DocumentSettings } from "./components/DocumentSettings";
// The server's settings: its name in each language, which people see in place of its domain, for manage_settings.
export { instanceQueryOptions as ownInstanceQueryOptions } from "./api/documents";
export { ServerSettings } from "./components/ServerSettings";
