// SPDX-License-Identifier: AGPL-3.0-only
// The toolbar above a list of the admin and moderation screens: the filters (`Select`s
// in their `toolbar` look) and a search field in a row that wraps where the container is narrow. It asks its own
// container, not the viewport (web-client.md D2).
import type { ReactNode } from "react";
import { tv } from "tailwind-variants";

const listToolbar = tv({
  slots: {
    base: "@container flex flex-wrap items-center gap-2",
    filters: "flex min-w-0 flex-wrap items-center gap-2",
    search: "ms-auto flex min-w-0 flex-auto items-center gap-2 @md:max-w-xs @md:flex-none",
  },
});

export type ListToolbarProps = {
  /** The name of the group ("Filters"), which a screen reader reads on entering it. */
  label: string;
  /** The filters: `Select`s with `variant="toolbar"` and the like. */
  children?: ReactNode;
  /** The search field (`SearchField`), at the end of the row. */
  search?: ReactNode;
};

/** The row of filters and the search above a list. */
export function ListToolbar({ label, children, search }: ListToolbarProps) {
  const slots = listToolbar();
  return (
    <div role="group" aria-label={label} className={slots.base()}>
      {children !== undefined && <div className={slots.filters()}>{children}</div>}
      {search !== undefined && <div className={slots.search()}>{search}</div>}
    </div>
  );
}
