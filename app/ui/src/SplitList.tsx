// SPDX-License-Identifier: AGPL-3.0-only
// The list and its detail of the admin and moderation screens (design/components/LayoutAdmin): where the container is wide
// enough (`@4xl`, 56rem), the list (a table in a card) and a sticky 20rem card with the selected row's detail beside it;
// narrower, one of the two is on screen: the list, or the detail as a page of its own. It asks its own container, not
// the viewport (web-client.md D2).
import type { ReactNode } from "react";
import { tv } from "tailwind-variants";
import { Card } from "./Card";

const splitList = tv({
  slots: {
    root: "@container",
    base: "grid grid-cols-1 items-start gap-6",
    list: "min-w-0",
    detail: "min-w-0 @4xl:sticky @4xl:top-6",
  },
  variants: {
    // With a detail, the second column; without, the list alone.
    split: { true: { base: "@4xl:grid-cols-[minmax(0,1fr)_20rem]" } },
    // Narrower, one of the two is on screen.
    view: {
      list: { detail: "@max-4xl:hidden" },
      detail: { list: "@max-4xl:hidden" },
    },
  },
  defaultVariants: { view: "list" },
});

export type SplitListProps = {
  /** The list: a `Table` (or, narrow, a stack of cards); the layout brings the card. */
  list: ReactNode;
  /**
   * The detail of the selected row (`DescriptionList`, then the actions); the layout brings the card. Leave it out
   * while no row is selected, and the list takes the whole width.
   */
  detail?: ReactNode;
  /** The name of the detail's region ("Report 1042"), which comes with `detail`. */
  detailLabel?: string;
  /** Narrow: `list` shows the list only (default); `detail` shows the detail only, as its own page. */
  view?: "list" | "detail";
};

/** A list in a card beside the sticky card of the selected row's detail; narrow, one of them at a time. */
export function SplitList({ list, detail, detailLabel, view = "list" }: SplitListProps) {
  const slots = splitList({ view, split: detail !== undefined });
  return (
    <div className={slots.root()}>
      <div className={slots.base()}>
        <div className={slots.list()}>
          <Card padding="0">{list}</Card>
        </div>
        {detail !== undefined && (
          <aside {...(detailLabel !== undefined && { "aria-label": detailLabel })} className={slots.detail()}>
            <Card padding="6">{detail}</Card>
          </aside>
        )}
      </div>
    </div>
  );
}
