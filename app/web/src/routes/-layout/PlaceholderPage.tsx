// SPDX-License-Identifier: AGPL-3.0-only
import { type ClientMessageKey, useT } from "../../lib/i18n";
import { PageHeading } from "./PageHeading";

export type PlaceholderPageProps = { titleKey: Extract<ClientMessageKey, `client:${string}.title`> };

/** A page whose screen does not exist yet: its heading and a note. */
export function PlaceholderPage({ titleKey }: PlaceholderPageProps) {
  const t = useT();
  return (
    <>
      <PageHeading>{t(titleKey)}</PageHeading>
      <p className="text-fg-muted">{t("client:page.not_built")}</p>
    </>
  );
}
