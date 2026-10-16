// SPDX-License-Identifier: AGPL-3.0-only
import { Link } from "@my/ui";
import { useT } from "../../lib/i18n";
import { PageHeading } from "../-layout/PageHeading";

/** The page of a handle that names no account the viewer may see. */
export function ProfileNotFound() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:profile.not_found.title")}</PageHeading>
      <p>{t("client:profile.not_found.body")}</p>
      <p>
        <Link href="/home" variant="standalone">
          {t("client:shell.not_found.home")}
        </Link>
      </p>
    </>
  );
}
