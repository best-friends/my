// SPDX-License-Identifier: AGPL-3.0-only
import { BackLinkHeading } from "@my/ui";
import { useLocation } from "@tanstack/react-router";
import type { ReactNode } from "react";
import { useT } from "../../lib/i18n";
import { PageHeading } from "./PageHeading";

export type SettingsHeadingProps = { children: ReactNode };

/** The heading of a page of the settings, with the way back to the settings after it (where narrow for the settings' own pages). */
export function SettingsHeading({ children }: SettingsHeadingProps) {
  const t = useT();
  // A page inside the settings has the menu beside it where wide; the admin and moderation pages have no such menu.
  const inSettings = useLocation({ select: (location) => location.pathname.startsWith("/settings/") });
  return (
    <BackLinkHeading backHref="/settings" backLabel={t("client:settings.back")} hideWhenWide={inSettings}>
      <PageHeading>{children}</PageHeading>
    </BackLinkHeading>
  );
}
