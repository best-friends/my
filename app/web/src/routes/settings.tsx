// SPDX-License-Identifier: AGPL-3.0-only
import { SettingsLayout } from "@my/ui";
import { createFileRoute, Outlet, useLocation } from "@tanstack/react-router";
import { SettingsMenu } from "../features/settings";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/**
 * The frame of the settings pages: the section menu and the page beside it where the column is wide; narrower, the
 * settings page stacks its heading, the menu and the rest of the page (the staff entry, the account, the documents),
 * and a section's page is the section. The settings page's heading comes before the menu, so the frame renders it.
 */
export const Route = createFileRoute("/settings")({
  component: SettingsFrame,
});

function SettingsFrame() {
  const t = useT();
  const pathname = useLocation({ select: (location) => location.pathname.replace(/(.)\/$/, "$1") });
  const isMenuPage = pathname === "/settings";
  return (
    <SettingsLayout
      menu={<SettingsMenu pathname={pathname} />}
      menuLabel={t("client:settings.menu.title")}
      view={isMenuPage ? "menu" : "pane"}
      // The rail already shows where the reader is, so the heading is for assistive technology (as the route said).
      {...(isMenuPage && { heading: <PageHeading visuallyHidden>{t("client:settings.title")}</PageHeading> })}
    >
      <Outlet />
    </SettingsLayout>
  );
}
