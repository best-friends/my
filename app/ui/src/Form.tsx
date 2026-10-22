// SPDX-License-Identifier: AGPL-3.0-only
// React Aria's Form with the layout of a `Stack`: its fields one above another with the gap of fields (README "Spacing"),
// so that a screen needs no classes on it. React Aria's Form adds what a native `<form>` lacks: `validationErrors` from the
// server shown on the named fields, and `validationBehavior` for the fields inside it.
import { Form as AriaForm, type FormProps as AriaFormProps } from "react-aria-components";
import { tv } from "tailwind-variants";
import { flex, type FlexLayoutProps, gaps, type Space, splitBox } from "./layout";

const form = tv({
  extend: flex,
  base: "flex-col",
  variants: { gap: gaps },
});

export type FormProps = Omit<AriaFormProps, "className" | "style"> &
  FlexLayoutProps & {
    /** The gap between the fields, in a spacing step: `4` between fields, `6` between groups of fields. */
    gap?: Space;
  };

/** A form (React Aria's `Form`) laid out as a `Stack`. A plain `<form>` with no server errors can also be `<Stack as="form">`. */
export function Form(props: FormProps) {
  const [boxProps, { align, justify, gap, ...rest }] = splitBox(props);
  return <AriaForm {...rest} className={form({ ...boxProps, align, justify, gap })} />;
}
