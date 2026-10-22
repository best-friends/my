// SPDX-License-Identifier: AGPL-3.0-only
import type { ComponentPropsWithRef } from "react";

export type CalloutProps = Omit<ComponentPropsWithRef<"div">, "className" | "style">;

/**
 * A raised box with a strong edge at its start, for a note and the one action it offers ("Your device is in another
 * time zone": "Use the device's"). The note is text; the action is a button after it.
 */
export function Callout(props: CalloutProps) {
  return (
    <div
      {...props}
      className="flex flex-col items-start gap-2 rounded-md border border-s-4 border-border border-s-border-strong bg-surface-raised px-4 py-3"
    />
  );
}
