// SPDX-License-Identifier: AGPL-3.0-only
import { composeRenderProps, Link as RACLink, type LinkProps as RACLinkProps } from "react-aria-components";
import { tv, type VariantProps } from "tailwind-variants";

const link = tv({
  base: [
    "cursor-pointer rounded-sm text-accent underline-offset-2 data-hovered:text-accent-hover data-focus-visible:focus-ring",
    "data-disabled:cursor-not-allowed data-disabled:opacity-50 motion-safe:transition-colors",
    // The link sizes a child icon next to its text, which sets no size of its own (web-client.md G11).
    "[&>svg]:size-4 [&>svg]:shrink-0",
  ],
  variants: {
    variant: {
      /** A link inside running text: always underlined, so color is not the only cue (WCAG 1.4.1). */
      inline: "underline data-hovered:decoration-2",
      /** A link on its own (a navigation item, "show more"): a 44 px target on coarse pointers. */
      standalone: "inline-flex min-h-6 items-center font-medium no-underline data-hovered:underline pointer-coarse:min-h-11",
    },
  },
  defaultVariants: { variant: "inline" },
});

export type LinkProps = RACLinkProps & VariantProps<typeof link>;

/**
 * A link (React Aria Link). With `href` it renders an `<a>`; client-side navigation comes from the
 * app's React Aria `RouterProvider`, so the kit knows no router.
 */
export function Link({ variant, className, ...props }: LinkProps) {
  return <RACLink {...props} className={composeRenderProps(className, (className) => link({ variant, className }))} />;
}
