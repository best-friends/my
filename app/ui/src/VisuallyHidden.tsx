// SPDX-License-Identifier: AGPL-3.0-only
import type { ComponentPropsWithRef } from "react";
import { tv } from "tailwind-variants";

const visuallyHidden = tv({ base: "sr-only" });

export type VisuallyHiddenProps = ComponentPropsWithRef<"span">;

/**
 * Text that assistive technology reads but the screen does not show, such as the accessible
 * label next to a number that is only drawn (Badge) or the "loading" label of a busy region.
 * It is still in the accessibility tree; to hide something from everyone, use `hidden`.
 */
export function VisuallyHidden({ className, ...props }: VisuallyHiddenProps) {
  return <span {...props} className={visuallyHidden({ className })} />;
}
