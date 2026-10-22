// SPDX-License-Identifier: AGPL-3.0-only
import type { ReactNode } from "react";

export type IconLabelProps = {
  /** The decorative icon before the text; it is drawn 20px square and does not shrink. */
  icon: ReactNode;
  /** The text; it wraps anywhere and may shrink in a row. */
  children: ReactNode;
};

/** An icon and the word it stands for on one line, such as the name of a page in a list of pages. */
export function IconLabel({ icon, children }: IconLabelProps) {
  return (
    <span className="flex min-w-0 items-center gap-2 font-extrabold wrap-anywhere [&_svg]:size-5 [&_svg]:shrink-0">
      {icon}
      {children}
    </span>
  );
}
