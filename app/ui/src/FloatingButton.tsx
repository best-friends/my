// SPDX-License-Identifier: AGPL-3.0-only
// The round primary button that floats over a page to start the page's main action (opening the compose sheet).
import type { ReactNode } from "react";
import { tv } from "tailwind-variants";
import { Button } from "./Button";
import { Tooltip, TooltipTrigger } from "./Tooltip";

const floatingButton = tv({
  base: "rounded-full shadow-md",
  variants: { iconOnly: { true: "size-12 px-0", false: "min-h-12 px-4" } },
});

export type FloatingButtonProps = {
  /** The action's words: the button's text, or with `iconOnly` its accessible name and its tooltip (web-client.md G12). */
  label: string;
  /** The icon before the text, or alone with `iconOnly`. */
  icon: ReactNode;
  /** Shows only the icon, in a 3rem circle. */
  iconOnly?: boolean;
  onPress: () => void;
};

/** A primary, pill-shaped (text) or circular (icon only) button with a shadow. Its place on the page is the app shell's. */
export function FloatingButton({ label, icon, iconOnly = false, onPress }: FloatingButtonProps) {
  if (iconOnly) {
    return (
      <TooltipTrigger>
        <Button variant="primary" aria-label={label} onPress={onPress} className={floatingButton({ iconOnly })}>
          {icon}
        </Button>
        <Tooltip placement="start">{label}</Tooltip>
      </TooltipTrigger>
    );
  }
  return (
    <Button variant="primary" onPress={onPress} className={floatingButton({ iconOnly })}>
      {icon}
      {label}
    </Button>
  );
}
