// SPDX-License-Identifier: AGPL-3.0-only
import { Link } from "@my/ui";
import { useLocation } from "@tanstack/react-router";
import { useId } from "react";
import { useT } from "../../lib/i18n";
import { useInstanceName } from "../../lib/instanceName";

/**
 * The server's name as a link to the root address: the landing page for people who are not signed in, the home
 * timeline for signed-in users (routes/index.tsx), the same link for both. It is named by the name it shows and
 * described as the server's top page; on the landing page itself it has aria-current="page". The name is the
 * administrator's text, in any language (web-client.md Y10), so the paragraph around it takes its direction.
 */
export function ServerNameLink({ className }: { className?: string }) {
  const t = useT();
  const instanceName = useInstanceName();
  const descriptionId = useId();
  const atRoot = useLocation({ select: (location) => location.pathname === "/" });
  return (
    <p dir="auto" className={className}>
      <Link href="/" aria-current={atRoot ? "page" : undefined} aria-describedby={descriptionId} variant="standalone" className="max-w-full font-semibold text-fg wrap-anywhere">
        {instanceName}
      </Link>
      {/* A description of a hidden element is still read (accname), and the element stays out of the tree. */}
      <span id={descriptionId} hidden>
        {t("client:shell.nav.landing")}
      </span>
    </p>
  );
}
