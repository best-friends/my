// SPDX-License-Identifier: AGPL-3.0-only
import { Link } from "@my/ui";
import { useSessionStatus, useSessionUnreachable } from "../../lib/auth/SessionProvider";
import { useT } from "../../lib/i18n";

/**
 * The way out of a page that has nothing to show (an address with no page, an account or a timeline that is not
 * there): the home timeline for signed-in users, and the landing page (`/`) for people who are not signed in, who
 * have no home timeline. Offline, a session that could not be resumed is most likely still there, so it keeps the
 * home timeline (web-client.md §11), as the navigation does.
 */
export function HomeLink() {
  const t = useT();
  const status = useSessionStatus();
  const unreachable = useSessionUnreachable();
  const signedOut = status === "signed-out" && !unreachable;
  return (
    <p>
      <Link href={signedOut ? "/" : "/home"} variant="standalone">
        {t(signedOut ? "client:shell.not_found.landing" : "client:shell.not_found.home")}
      </Link>
    </p>
  );
}
