// SPDX-License-Identifier: AGPL-3.0-only
import type { ComponentPropsWithRef } from "react";
import { tv } from "tailwind-variants";

const center = tv({ base: "grid min-h-svh place-items-center p-4" });

export type CenterProps = ComponentPropsWithRef<"div">;

/** Centers its children in a box at least as tall as the small viewport. */
export function Center({ className, ...props }: CenterProps) {
  return <div {...props} className={center({ className })} />;
}
