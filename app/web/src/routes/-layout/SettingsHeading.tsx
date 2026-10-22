// SPDX-License-Identifier: AGPL-3.0-only
import { BackLinkHeading } from "@my/ui";
import type { ReactNode } from "react";
import { useT } from "../../lib/i18n";
import { PageHeading } from "./PageHeading";

export type SettingsHeadingProps = { children: ReactNode };

/** The heading of a page of the settings, with the way back to the settings after it. */
export function SettingsHeading({ children }: SettingsHeadingProps) {
  const t = useT();
  return (
    <BackLinkHeading backHref="/settings" backLabel={t("client:settings.back")}>
      <PageHeading>{children}</PageHeading>
    </BackLinkHeading>
  );
}
