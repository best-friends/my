// SPDX-License-Identifier: AGPL-3.0-only
import type { ReactNode } from "react";
import {
  composeRenderProps,
  Input,
  type InputProps,
  TextField as RACTextField,
  type TextFieldProps as RACTextFieldProps,
} from "react-aria-components";
import { tv } from "tailwind-variants";
import { FieldDescription, FieldError, FieldLabel, type FieldTextProps, fieldControl } from "./Field";

const textField = tv({
  slots: {
    root: "flex flex-col gap-1",
    input: fieldControl(),
    // The box of the input and its end action (endAction): the action sits at the end of the input, in the middle
    // of its height (40 px, 44 px on a coarse pointer; the button is 32 px high). The room the text leaves for it is
    // geometry, not a spacing step: the width of the button of size `icon` (32 px, 44 px on a coarse pointer) and its inset.
    control: "relative min-w-0 [--end-action-room:--spacing(10)] pointer-coarse:[--end-action-room:--spacing(12)]",
    action: "absolute inset-e-1 top-1 pointer-coarse:top-1.5",
  },
  variants: {
    grow: { true: { root: "grow" } },
    shrink: { true: { root: "min-w-0" } },
    // The text keeps clear of the end action, so that it does not run under its button (a button of size `icon`).
    withAction: { true: { input: "pe-(--end-action-room)" } },
  },
});

export interface TextFieldProps extends Omit<RACTextFieldProps, "children">, FieldTextProps {
  placeholder?: InputProps["placeholder"];
  /** Takes the room left in a row (`grow`). */
  grow?: boolean;
  /** Lets the field shrink below its content's width in a row (`min-w-0`), so a long value does not push the row wider. */
  shrink?: boolean;
  /**
   * A control at the end of the input: an icon button of size `icon` that adds to the text (the emoji picker's,
   * ADR-0091 §4). The text keeps clear of it on the end side.
   */
  endAction?: ReactNode;
}

/**
 * A single-line text field (React Aria TextField). The label, description and error message are linked
 * to the input; it is controlled with `value` / `onChange` or uncontrolled with `defaultValue`. With `endAction`, that
 * control sits at the end of the input (inset-inline-end), and the text keeps clear of it.
 */
export function TextField({ label, description, errorMessage, placeholder, className, grow, shrink, endAction, ...props }: TextFieldProps) {
  const { root, input, control, action } = textField({ grow, shrink, withAction: endAction !== undefined });
  const field = <Input className={input()} {...(placeholder === undefined ? {} : { placeholder })} />;
  return (
    <RACTextField {...props} className={composeRenderProps(className, (className) => root({ className }))}>
      {label !== undefined && <FieldLabel>{label}</FieldLabel>}
      {endAction === undefined ? (
        field
      ) : (
        <div className={control()}>
          {field}
          <div className={action()}>{endAction}</div>
        </div>
      )}
      {description !== undefined && <FieldDescription>{description}</FieldDescription>}
      <FieldError>{errorMessage}</FieldError>
    </RACTextField>
  );
}
