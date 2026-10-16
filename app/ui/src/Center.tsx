// SPDX-License-Identifier: AGPL-3.0-only
import type { ComponentPropsWithRef } from "react";

// className is left out until the kit composes classes with tailwind-merge (web-client.md D9).
export type CenterProps = Omit<ComponentPropsWithRef<"div">, "className">;

/** Centers its children in a box at least as tall as the small viewport. */
export function Center(props: CenterProps) {
  return <div {...props} className="grid min-h-svh place-items-center p-4" />;
}
