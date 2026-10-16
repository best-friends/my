// SPDX-License-Identifier: AGPL-3.0-only
import type { ReactNode } from "react";

export type PageHeadingProps = {
  children: ReactNode;
  /** An ID, for what the heading names (a feed's `aria-labelledby`). */
  id?: string;
};

/**
 * The page's only level-one heading. After a navigation the layout moves focus to it and names the
 * document after its text (useViewChange), so it is focusable by script but not in the tab order.
 */
export function PageHeading({ children, id }: PageHeadingProps) {
  return (
    <h1 id={id} tabIndex={-1} className="text-2xl font-semibold">
      {children}
    </h1>
  );
}
