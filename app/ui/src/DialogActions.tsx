// SPDX-License-Identifier: AGPL-3.0-only
// The buttons at the end of a dialog's content: the way out first, the action last, at the inline end, wrapping
// when they do not fit (README "Spacing": controls in a row are `gap-2`).
import type { ReactNode } from "react";
import { tv } from "tailwind-variants";

const dialogActions = tv({ base: "flex flex-wrap justify-end gap-2" });

export type DialogActionsProps = {
  /** The `Button`s: cancel (or back) first, the action last. */
  children: ReactNode;
};

/** The row of buttons of a `Dialog`, a `Sheet` or a `ConfirmDialog`. */
export function DialogActions({ children }: DialogActionsProps) {
  return <div className={dialogActions()}>{children}</div>;
}
