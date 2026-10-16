// SPDX-License-Identifier: AGPL-3.0-only
import type { ReactNode } from "react";
import { composeRenderProps, SwitchButton, SwitchField, type SwitchFieldProps } from "react-aria-components";
import { tv } from "tailwind-variants";
import { FieldDescription, FieldError, type FieldTextProps } from "./Field";

const switchStyles = tv({
  slots: {
    root: "group flex flex-col gap-1",
    button: [
      "flex min-h-6 cursor-default items-center gap-3 text-base text-fg",
      "data-disabled:cursor-not-allowed data-disabled:opacity-50 pointer-coarse:min-h-11",
    ],
    // The thumb sits at the start and moves to the end with justify-content, so it follows the direction.
    track: [
      "flex h-6 w-10 shrink-0 items-center justify-start rounded-full border-2 border-border-strong bg-surface-sunken px-0.5",
      "motion-safe:transition-colors",
    ],
    thumb: "size-4 rounded-full bg-border-strong",
    // Aligns the description and error with the label text: the track (2.5rem) plus the gap (0.75rem).
    text: "ms-13",
  },
  variants: {
    isSelected: { true: { track: "justify-end border-accent bg-accent", thumb: "bg-accent-fg" } },
    isFocusVisible: { true: { track: "focus-ring" } },
  },
});

export interface SwitchProps extends Omit<SwitchFieldProps, "children">, Omit<FieldTextProps, "label"> {
  /** The label. */
  children?: ReactNode;
}

/**
 * A switch (React Aria SwitchField, `role="switch"`) for a setting that takes effect at once. Toggled
 * with a pointer or Space; controlled with `isSelected` / `onChange` or uncontrolled with `defaultSelected`.
 */
export function Switch({ children, description, errorMessage, className, ...props }: SwitchProps) {
  const styles = switchStyles();
  return (
    <SwitchField {...props} className={composeRenderProps(className, (className) => styles.root({ className }))}>
      <SwitchButton className={styles.button()}>
        {({ isSelected, isFocusVisible }) => {
          const state = switchStyles({ isSelected, isFocusVisible });
          return (
            <>
              <span className={state.track()}>
                <span className={state.thumb()} />
              </span>
              {children}
            </>
          );
        }}
      </SwitchButton>
      {description !== undefined && <FieldDescription className={styles.text()}>{description}</FieldDescription>}
      <FieldError className={styles.text()}>{errorMessage}</FieldError>
    </SwitchField>
  );
}
