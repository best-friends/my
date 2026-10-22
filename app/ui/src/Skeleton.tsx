// SPDX-License-Identifier: AGPL-3.0-only
import type { ComponentPropsWithRef } from "react";
import { tv, type VariantProps } from "tailwind-variants";

const skeleton = tv({
  // The pulse runs only when the user has not asked for reduced motion (web-client.md D6).
  base: "block bg-border motion-safe:animate-pulse",
  variants: {
    shape: {
      /** One line of text: as tall as the surrounding line box's text, full width unless the caller narrows it. */
      text: "h-[1em] w-full rounded-sm",
      /** An avatar or an icon; the caller sets the size. */
      circle: "size-10 rounded-avatar",
      /** A media preview or a card; the caller sets the size. */
      rect: "h-24 w-full rounded-lg",
    },
  },
  defaultVariants: { shape: "text" },
});

export type SkeletonProps = Omit<ComponentPropsWithRef<"span">, "children"> & VariantProps<typeof skeleton>;

/**
 * A placeholder shape for content that is loading. It is hidden from assistive technology: the
 * region it fills sets `aria-busy="true"` while loading and carries the announcement (a Spinner or
 * a VisuallyHidden label), then drops `aria-busy` when the content arrives (web-client.md E3, Y6).
 */
export function Skeleton({ shape, className, ...props }: SkeletonProps) {
  return <span aria-hidden="true" {...props} className={skeleton({ shape, className })} />;
}
