// SPDX-License-Identifier: AGPL-3.0-only
import type { ComponentPropsWithRef } from "react";
import { tv } from "tailwind-variants";

const tag = tv({ base: "rounded-sm border border-border px-1 text-sm text-fg-muted" });

export type TagProps = Omit<ComponentPropsWithRef<"span">, "className" | "style">;

/**
 * A short outlined word that qualifies the value beside it ("Estimated"). It is text, not a control or a count:
 * a count is a `Badge`, a role a `RoleBadge`.
 */
export function Tag(props: TagProps) {
  return <span {...props} className={tag()} />;
}
