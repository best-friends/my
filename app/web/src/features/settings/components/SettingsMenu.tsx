// SPDX-License-Identifier: AGPL-3.0-only
import { RailItem, Stack } from "@my/ui";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { type Permission, useViewerPermissions } from "../../moderation";
import { useSupporterEntry } from "../../supporters";

/**
 * The pages of the settings, in the order of the menu; a page with a `permission` (a member permission, ADR-0070 §1)
 * is listed only for a viewer whose roles have it, and the page of the supporter plans only while the program is
 * enabled, outside the Tauri app (ADR-0072 §14).
 */
const pages: readonly { href: string; label: ClientMessageKey; description: ClientMessageKey; permission?: Permission; supporters?: true }[] = [
  { href: "/settings/profile", label: "client:settings.profile.title", description: "client:settings.profile.summary" },
  { href: "/settings/preferences", label: "client:settings.preferences.title", description: "client:settings.preferences.summary" },
  { href: "/settings/password", label: "client:settings.password.title", description: "client:settings.password.summary" },
  { href: "/settings/two-factor", label: "client:settings.two_factor.title", description: "client:settings.two_factor.summary" },
  { href: "/settings/passkeys", label: "client:settings.passkeys.title", description: "client:settings.passkeys.summary" },
  { href: "/settings/auto-delete", label: "client:settings.auto_delete.title", description: "client:settings.auto_delete.summary" },
  { href: "/settings/follow-requests", label: "client:settings.follow_requests.title", description: "client:settings.follow_requests.summary" },
  { href: "/settings/blocks", label: "client:settings.blocks.title", description: "client:settings.blocks.summary" },
  { href: "/settings/mutes", label: "client:settings.mutes.title", description: "client:settings.mutes.summary" },
  { href: "/settings/domain-mutes", label: "client:settings.domain_mutes.title", description: "client:settings.domain_mutes.summary" },
  { href: "/settings/import-export", label: "client:settings.import_export.title", description: "client:settings.import_export.summary" },
  { href: "/settings/custom-emojis", label: "client:custom_emojis.title", description: "client:custom_emojis.summary", permission: "create_custom_emojis" },
  { href: "/settings/supporter", label: "client:supporter.title", description: "client:supporter.summary", supporters: true },
  { href: "/settings/account-status", label: "client:account_status.title", description: "client:account_status.summary" },
  { href: "/settings/account", label: "client:settings.account.title", description: "client:settings.account.summary" },
];

/** The links to the pages of the settings, as the items of the section menu; the page the user is on is current. */
export function SettingsMenu({ pathname }: { pathname: string }) {
  const t = useT();
  const permissions = useViewerPermissions();
  const supporters = useSupporterEntry();
  const shown = pages.filter(
    (page) => (page.permission === undefined || permissions.includes(page.permission)) && (page.supporters === undefined || supporters),
  );
  return (
    <Stack as="ul" gap="1">
      {shown.map((page) => (
        <li key={page.href}>
          <RailItem form="section" href={page.href} icon={null} label={t(page.label)} current={pathname === page.href} />
        </li>
      ))}
    </Stack>
  );
}
