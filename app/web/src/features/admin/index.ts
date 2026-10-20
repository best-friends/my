// SPDX-License-Identifier: AGPL-3.0-only
// The server administration feature (ADR-0058): the server's settings (its name and descriptions in each language
// and its policies), its rules, its terms of service and privacy policy (#489), the custom emoji (ADR-0051), the announcements (FR-726), the webhooks (ADR-0060), and the dead jobs of the job queue with running them again. Each screen
// is for a viewer whose roles have its permission (manage_settings, manage_rules, manage_custom_emojis, manage_announcements, manage_webhooks, manage_jobs); the server checks it on every
// request. The parts and the permissions that the screens share with the moderation screens come from the
// moderation feature.
// Routes and other features use it through this file only (web-client.md A3).
export { AdminMenu, AdminPages, useAdminEntry } from "./components/AdminMenu";
export { deadJobsQueryOptions } from "./api/deadJobs";
export { DeadJobList } from "./components/DeadJobList";
// The terms of service and the privacy policy (#489), for manage_settings.
export { instanceTextsQueryOptions } from "./api/documents";
export { DocumentSettings } from "./components/DocumentSettings";
// The server's settings: its name and descriptions in each language, and its policies, for manage_settings.
export { instanceQueryOptions as ownInstanceQueryOptions } from "./api/documents";
export { instanceSettingsQueryOptions } from "./api/settings";
export { ServerSettings } from "./components/ServerSettings";
// The server's rules (FR-705), for manage_rules.
export { rulesQueryOptions } from "./api/rules";
export { RuleSettings } from "./components/RuleSettings";
// The server's theme: overrides of the design tokens and a custom CSS (ADR-0059), for manage_settings.
export { themeQueryOptions } from "./api/theme";
export { ThemeSettings } from "./components/ThemeSettings";
// The custom emoji: registering this server's and managing those of other servers (ADR-0051), for manage_custom_emojis.
export { adminCustomEmojisQueryOptions, type CustomEmojiFilter } from "./api/customEmojis";
export { CustomEmojiSettings } from "./components/CustomEmojiSettings";
// The server's announcements to every local user (FR-726), for manage_announcements.
export { adminAnnouncementsQueryOptions } from "./api/announcements";
export { AnnouncementSettings } from "./components/AnnouncementSettings";
// The webhooks: setting them up, their deliveries and sending one again (ADR-0060), for manage_webhooks.
export { webhookDeliveriesQueryOptions, webhookQueryOptions, webhooksQueryOptions } from "./api/webhooks";
export { WebhookDeliveries } from "./components/WebhookDeliveries";
export { WebhookSettings } from "./components/WebhookSettings";
