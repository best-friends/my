// SPDX-License-Identifier: AGPL-3.0-only
import { Link } from "@my/ui";
import { useT } from "../../lib/i18n";

/**
 * The way in to a page that only signed-in users may read (a timeline the server keeps for them): the sign-in page,
 * which comes back to `returnTo` afterwards.
 */
export function SignInLink({ returnTo }: { returnTo: string }) {
  const t = useT();
  return (
    <p>
      <Link href={`/login?${new URLSearchParams({ return_to: returnTo }).toString()}`} variant="standalone">
        {t("client:shell.sign_in_required.sign_in")}
      </Link>
    </p>
  );
}
