// SPDX-License-Identifier: AGPL-3.0-only
// The result of an operator's action that stays on the page ("example.com is suspended until 24 Oct"):
// a polite live region (role="status") that is in the page from the first render, empty, and shows the
// message as a box once there is one. A live region announces only what changes after it is in the page,
// so the region must not mount together with its message (web-client.md Y7).
// A failed action is not a status: it stays in the dialog that ran it (`ConfirmDialog`'s errorMessage) or goes
// to a toast (web-client.md E2).
import type { ComponentPropsWithRef, ReactNode } from "react";
import { tv } from "tailwind-variants";

const statusMessage = tv({
  slots: {
    region: "",
    // A domain or a handle in the message is one long word; it wraps rather than scroll the page.
    box: "rounded-md border border-s-4 border-border bg-surface-raised px-4 py-3 text-base wrap-anywhere text-fg",
  },
  variants: {
    tone: {
      neutral: { box: "border-s-border-strong" },
      success: { box: "border-s-success" },
    },
  },
  defaultVariants: { tone: "success" },
});

export type StatusMessageProps = Omit<ComponentPropsWithRef<"div">, "role" | "children"> & {
  /** The message; `null` or `undefined` leaves the region empty and invisible. */
  children?: ReactNode;
  /** `success` for an action that took effect, `neutral` for other news. */
  tone?: "neutral" | "success";
};

/** Render it before the action, empty, and pass the result as children when the action has succeeded. */
export function StatusMessage({ children, tone, className, ...props }: StatusMessageProps) {
  const styles = statusMessage({ tone });
  return (
    <div {...props} role="status" className={styles.region({ className })}>
      {children !== undefined && children !== null && <p className={styles.box()}>{children}</p>}
    </div>
  );
}
