// SPDX-License-Identifier: AGPL-3.0-only
import { type ComponentPropsWithRef, createElement } from "react";
import { tv } from "tailwind-variants";

const tag = tv({
  variants: {
    variant: {
      outlined: "rounded-sm border border-border px-1 text-sm text-fg-muted",
      pill: "rounded-full bg-surface-sunken px-2 py-0.5 text-sm text-fg",
    },
  },
  defaultVariants: { variant: "outlined" },
});

export type TagProps = Omit<ComponentPropsWithRef<"span">, "className" | "style"> & {
  /** `outlined` qualifies the value beside it; `pill` is a filled, rounded label in a list of them. */
  variant?: "outlined" | "pill";
  /** `li` for a tag in a `ul`. */
  as?: "span" | "li";
};

/**
 * A short word that qualifies what it sits beside ("Estimated") or labels an account ("Bot"). It is text, not a control
 * or a count: a count is a `Badge`, a role a `RoleBadge`.
 */
export function Tag({ variant, as = "span", ...props }: TagProps) {
  return createElement(as, { ...props, className: tag({ variant }) });
}
