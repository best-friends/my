// SPDX-License-Identifier: AGPL-3.0-only
// "N new posts" (web-client.md Y7, ADR-0027): new items wait above a feed until the reader asks for them,
// because content that moves on its own breaks WCAG 2.2.2 and takes the reader's place away.
import type { ComponentPropsWithRef } from "react";
import { tv } from "tailwind-variants";
import { Button } from "./Button";
import { NewItemsIcon } from "./Icons";

const newItemsButton = tv({
  slots: {
    // Never display: none, even when empty: a live region must stay rendered to announce the next count.
    base: "flex justify-center",
    button: "rounded-full shadow-md",
  },
  variants: {
    sticky: {
      // The region keeps the button's height; only the button takes pointer input, so the region never blocks the post under it.
      true: {
        base: "pointer-events-none sticky top-[calc(env(safe-area-inset-top)+0.75rem)] z-10 h-8 items-start pointer-coarse:h-11 *:pointer-events-auto",
      },
    },
  },
});

export type NewItemsButtonProps = Omit<ComponentPropsWithRef<"div">, "children" | "role"> & {
  /** How many new items are waiting. At zero the button is not drawn; the status region stays in the page. */
  count: number;
  /** The button's text for this count, formatted by the caller with a plural message ("3 new posts"). */
  label: string;
  /** Shows the waiting items: the caller inserts them and moves the reader to the top of the feed. */
  onPress: () => void;
  /** Sticks the region to the top of the viewport while the reader scrolls, always as tall as the button. */
  sticky?: boolean;
};

/**
 * A button that shows how many new items are waiting and inserts them only when pressed. It sits in a
 * polite `role="status"` region that is in the page from the first render, so a screen reader reads the
 * new count when it changes without the button taking focus.
 */
export function NewItemsButton({ count, label, onPress, sticky, className, ...props }: NewItemsButtonProps) {
  const slots = newItemsButton({ sticky });
  return (
    <div {...props} role="status" className={slots.base({ className })}>
      {count > 0 && (
        <Button variant="primary" size="sm" onPress={onPress} className={slots.button()}>
          <NewItemsIcon />
          {label}
        </Button>
      )}
    </div>
  );
}
