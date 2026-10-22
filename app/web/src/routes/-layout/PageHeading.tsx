// SPDX-License-Identifier: AGPL-3.0-only
import { PageHeading as PageHeadingView } from "@my/ui";
import { useMatch } from "@tanstack/react-router";
import type { ReactNode } from "react";

export type PageHeadingProps = {
  children: ReactNode;
  /** For a region named by the page heading (a feed with `aria-labelledby`). */
  id?: string;
  /**
   * Whether the heading is for assistive technology only. By default it follows the route's
   * `staticData.hideHeading`; a state of such a page that says more than where the user is (a timeline the
   * server has turned off) shows its heading with `false`.
   */
  visuallyHidden?: boolean;
};

/**
 * The page's only level-one heading. After a navigation the layout moves focus to it and names the
 * document after its text (useViewChange), so it is focusable by script but not in the tab order.
 * On a page whose heading only says where the user is, which the navigation already shows (a timeline,
 * the notifications), it is visually hidden but stays in the accessibility tree (web-client.md Y5).
 */
export function PageHeading({ children, id, visuallyHidden }: PageHeadingProps) {
  // The heading renders inside its route's match, as do the route's pending and error components.
  const hideHeading = useMatch({ strict: false, select: (match) => match.staticData.hideHeading === true });
  const hidden = visuallyHidden ?? hideHeading;
  return (
    <PageHeadingView {...(id !== undefined && { id })} visuallyHidden={hidden}>
      {children}
    </PageHeadingView>
  );
}
