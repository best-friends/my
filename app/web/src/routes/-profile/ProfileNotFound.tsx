// SPDX-License-Identifier: AGPL-3.0-only
import { useT } from "../../lib/i18n";
import { PageHeading } from "../-layout/PageHeading";
import { HomeLink } from "../-layout/HomeLink";

/** The page of a handle that names no account the viewer may see. */
export function ProfileNotFound() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:profile.not_found.title")}</PageHeading>
      <p>{t("client:profile.not_found.body")}</p>
      <HomeLink />
    </>
  );
}
