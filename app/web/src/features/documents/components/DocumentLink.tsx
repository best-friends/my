// SPDX-License-Identifier: AGPL-3.0-only
import { Link } from "@my/ui";
import type { ReactNode } from "react";
import { documentPath } from "../documentPaths";

/** A link to a page of a document of the instance (`terms_url` and the like of GET /instance), which opens in the same tab. */
export function DocumentLink({ href, children }: { href: string; children: ReactNode }) {
  return (
    <Link href={documentPath(href)} variant="standalone">
      {children}
    </Link>
  );
}
