// SPDX-License-Identifier: AGPL-3.0-only
import type { ComponentPropsWithRef } from "react";
import { ProgressBar } from "react-aria-components";
import { tv, type VariantProps } from "tailwind-variants";

const spinner = tv({
  slots: {
    base: "inline-flex shrink-0 align-middle text-accent",
    // A decorative track and a quarter arc in the current color (accent, or the caller's text color
    // through className). With reduced motion the arc stands still and the accessible name still
    // says that something is loading (web-client/README.md D6).
    ring: "rounded-full border-border border-t-current motion-safe:animate-spin",
  },
  variants: {
    size: {
      sm: { ring: "size-4 border-2" },
      md: { ring: "size-6 border-3" },
      lg: { ring: "size-10 border-4" },
    },
  },
  defaultVariants: { size: "md" },
});

export type SpinnerProps = Omit<
  ComponentPropsWithRef<typeof ProgressBar>,
  "children" | "className" | "value" | "minValue" | "maxValue" | "isIndeterminate" | "formatOptions" | "valueLabel" | "aria-label"
> &
  VariantProps<typeof spinner> & {
    /** What is in progress ("Loading posts"). It is the progress bar's accessible name. */
    label: string;
    className?: string;
  };

/**
 * An indeterminate progress indicator: an indeterminate `progressbar` named by `label`. It spins only
 * under `motion-safe`. Inside a region that is loading, the region also sets `aria-busy="true"`.
 */
export function Spinner({ label, size, className, ...props }: SpinnerProps) {
  const { base, ring } = spinner({ size });
  return (
    <ProgressBar {...props} aria-label={label} isIndeterminate className={base({ className })}>
      <span aria-hidden="true" className={ring()} />
    </ProgressBar>
  );
}
