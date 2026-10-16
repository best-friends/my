// SPDX-License-Identifier: AGPL-3.0-only
// Route focus management, once for every page (web-client.md Y5). After the router renders a new page
// it names the document after the page's h1 and, when the path changed, moves focus to that h1 so that a
// screen reader reads the new page and keyboard users continue from its start. The first page of a
// visit keeps the browser's own focus; only its title is set.
import { useRouter } from "@tanstack/react-router";
import { type RefObject, useEffect, useEffectEvent } from "react";
import { useConfig } from "../../lib/config";
import { useT } from "../../lib/i18n";

/** Keeps `document.title` and focus in step with the page rendered inside `main`. */
export function useViewChange(main: RefObject<HTMLElement | null>): void {
  const router = useRouter();
  const t = useT();
  const { instanceName } = useConfig();

  const onViewChange = useEffectEvent((moveFocus: boolean) => {
    const heading = main.current?.querySelector("h1") ?? null;
    const page = heading?.textContent.trim() ?? "";
    document.title = page === "" ? instanceName : t("client:shell.document_title", { page, instance: instanceName });
    if (moveFocus && heading !== null) {
      // Focusable by script only; PageHeading already sets it, other headings get it here.
      if (!heading.hasAttribute("tabindex")) heading.tabIndex = -1;
      heading.focus();
    }
  });

  // The router is outside React: subscribe to its render events, and unsubscribe on unmount.
  useEffect(() => {
    onViewChange(false);
    return router.subscribe("onRendered", (event) => {
      // A change of only the search or the hash (a tab, a filter) stays on the same page and keeps focus.
      onViewChange(event.fromLocation !== undefined && event.pathChanged);
    });
  }, [router]);
}
