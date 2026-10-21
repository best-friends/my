// SPDX-License-Identifier: AGPL-3.0-only
// The links to the server administration screens (ADR-0058): a section of the settings page, and the
// administration page that the navigation's Administration entry opens. They show no counts: what awaits the
// staff is in the moderation.
import { useT } from "../../../lib/i18n";
import { type StaffPage, StaffMenuSection, StaffPages, useShownStaffPages } from "../../moderation";

/**
 * The server administration screens, in the order of the menu, with the permission each needs. A new screen
 * adds a line here and its route `routes/admin.<screen>.tsx`.
 */
const adminPages: readonly StaffPage[] = [
  { href: "/admin/settings", permission: "manage_settings", label: "client:admin.settings.title", description: "client:admin.settings.summary" },
  { href: "/admin/rules", permission: "manage_rules", label: "client:admin.rules.title", description: "client:admin.rules.summary" },
  { href: "/admin/theme", permission: "manage_settings", label: "client:admin.theme.title", description: "client:admin.theme.summary" },
  { href: "/admin/documents", permission: "manage_settings", label: "client:admin.documents.title", description: "client:admin.documents.summary" },
  {
    href: "/admin/custom-emojis",
    permission: "manage_custom_emojis",
    label: "client:admin.custom_emojis.title",
    description: "client:admin.custom_emojis.summary",
  },
  {
    href: "/admin/announcements",
    permission: "manage_announcements",
    label: "client:admin.announcements.title",
    description: "client:admin.announcements.summary",
  },
  { href: "/admin/webhooks", permission: "manage_webhooks", label: "client:admin.webhooks.title", description: "client:admin.webhooks.summary" },
  { href: "/admin/relays", permission: "manage_settings", label: "client:admin.relays.title", description: "client:admin.relays.summary" },
  { href: "/admin/finance", permission: "manage_finance", label: "client:admin.finance.title", description: "client:admin.finance.summary" },
  { href: "/admin/roles", permission: "manage_roles", label: "client:admin.roles.title", description: "client:admin.roles.summary" },
  { href: "/admin/jobs", permission: "manage_jobs", label: "client:admin.jobs.title", description: "client:admin.jobs.summary" },
];

/** The settings page's section of links to the server administration screens; nothing for a viewer without any. */
export function AdminMenu() {
  const t = useT();
  return <StaffMenuSection title={t("client:admin.menu.title")} pages={adminPages} />;
}

/** The administration page's links to the server administration screens, or why there are none for the viewer's roles. */
export function AdminPages() {
  return <StaffPages pages={adminPages} none={{ heading: "client:admin.home.none.heading", description: "client:admin.home.none.description" }} />;
}

/** The navigation's Administration entry: whether the viewer may use a server administration screen. */
export function useAdminEntry(): { available: boolean } {
  return { available: useShownStaffPages(adminPages).length > 0 };
}
