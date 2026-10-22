// SPDX-License-Identifier: AGPL-3.0-only
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
  },
  variants: { grow: { true: { root: "grow" } } },
});

export interface TextFieldProps extends Omit<RACTextFieldProps, "children">, FieldTextProps {
  placeholder?: InputProps["placeholder"];
  /** Takes the room left in a row (`grow`). */
  grow?: boolean;
}

/**
 * A single-line text field (React Aria TextField). The label, description and error message are linked
 * to the input; it is controlled with `value` / `onChange` or uncontrolled with `defaultValue`.
 */
export function TextField({ label, description, errorMessage, placeholder, className, grow, ...props }: TextFieldProps) {
  const { root, input } = textField({ grow });
  return (
    <RACTextField {...props} className={composeRenderProps(className, (className) => root({ className }))}>
      {label !== undefined && <FieldLabel>{label}</FieldLabel>}
      <Input className={input()} {...(placeholder === undefined ? {} : { placeholder })} />
      {description !== undefined && <FieldDescription>{description}</FieldDescription>}
      <FieldError>{errorMessage}</FieldError>
    </RACTextField>
  );
}
