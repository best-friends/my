// SPDX-License-Identifier: AGPL-3.0-only
// The result of an operator's action that stays on the page ("example.com is suspended until 24 Oct"):
// a polite live region (role="status") that is in the page from the first render, empty, and shows the
// message as a box once there is one. A live region announces only what changes after it is in the page,
// so the region must not mount together with its message (web-client.md Y7).
// A failed action is not a status: it stays in the dialog that ran it (`ConfirmDialog`'s errorMessage) or goes
// to a toast (web-client.md E2).
import type { ComponentPropsWithRef, ReactNode } from "react";
import { tv } from "tailwind-variants";
import { AboutIcon, CheckIcon } from "./Icons";

const statusMessage = tv({
  slots: {
    region: "",
    // A domain or a handle in the message is one long word; it wraps rather than scroll the page.
    box: "flex items-start gap-3 rounded-md px-4 py-3 text-base text-fg",
    // The tone shows by this icon as well as by the box's color (WCAG 1.4.1).
    icon: "mt-0.5 shrink-0 [&>svg]:size-5",
    text: "min-w-0 wrap-anywhere",
  },
  variants: {
    tone: {
      neutral: { box: "bg-surface-sunken", icon: "text-fg-muted" },
      success: { box: "bg-success-bg", icon: "text-success" },
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
      {children !== undefined && children !== null && (
        <p className={styles.box()}>
          <span className={styles.icon()}>{tone === "neutral" ? <AboutIcon /> : <CheckIcon />}</span>
          <span className={styles.text()}>{children}</span>
        </p>
      )}
    </div>
  );
}
