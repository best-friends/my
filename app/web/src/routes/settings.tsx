// SPDX-License-Identifier: AGPL-3.0-only
import { SettingsLayout } from "@my/ui";
import { createFileRoute, Outlet, useLocation } from "@tanstack/react-router";
import { SettingsMenu } from "../features/settings";
import { useT } from "../lib/i18n";

/**
 * The frame of the settings pages: the section menu and the page beside it from `md`; below `md`, the settings page is
 * the menu and a section's page is the section.
 */
export const Route = createFileRoute("/settings")({
  component: SettingsFrame,
});

function SettingsFrame() {
  const t = useT();
  const pathname = useLocation({ select: (location) => location.pathname.replace(/(.)\/$/, "$1") });
  return (
    <SettingsLayout
      menu={<SettingsMenu pathname={pathname} />}
      menuLabel={t("client:settings.menu.title")}
      view={pathname === "/settings" ? "menu" : "pane"}
    >
      <Outlet />
    </SettingsLayout>
  );
}
