// SPDX-License-Identifier: AGPL-3.0-only
// "N new posts" (web-client.md Y7, ADR-0027): new items wait above a feed until the reader asks for them,
// because content that moves on its own breaks WCAG 2.2.2 and takes the reader's place away.
import type { ComponentPropsWithRef } from "react";
import { tv } from "tailwind-variants";
import { Button } from "./Button";

const newItemsButton = tv({
  slots: {
    // Never display: none, even when empty: a live region must stay rendered to announce the next count.
    base: "flex justify-center",
    button: "rounded-full shadow-md",
    arrow: "size-4 fill-none stroke-current stroke-2",
  },
});

export type NewItemsButtonProps = Omit<ComponentPropsWithRef<"div">, "children" | "role"> & {
  /** How many new items are waiting. At zero the button is not drawn; the status region stays in the page. */
  count: number;
  /** The button's text for this count, formatted by the caller with a plural message ("3 new posts"). */
  label: string;
  /** Shows the waiting items: the caller inserts them and moves the reader to the top of the feed. */
  onPress: () => void;
};

/**
 * A button that shows how many new items are waiting and inserts them only when pressed. It sits in a
 * polite `role="status"` region that is in the page from the first render, so a screen reader reads the
 * new count when it changes without the button taking focus.
 */
export function NewItemsButton({ count, label, onPress, className, ...props }: NewItemsButtonProps) {
  const slots = newItemsButton();
  return (
    <div {...props} role="status" className={slots.base({ className })}>
      {count > 0 && (
        <Button variant="primary" size="sm" onPress={onPress} className={slots.button()}>
          <svg aria-hidden="true" viewBox="0 0 16 16" className={slots.arrow()}>
            <path d="M8 13V3M4 7l4-4 4 4" />
          </svg>
          {label}
        </Button>
      )}
    </div>
  );
}
