// SPDX-License-Identifier: AGPL-3.0-only
// A group of related controls in a frame with a caption on its edge: a `fieldset` and its `legend`.
import type { ComponentPropsWithRef, ReactNode } from "react";
import { tv } from "tailwind-variants";

const fieldset = tv({
  slots: {
    root: "flex min-w-0 flex-col gap-3 rounded-md border border-border p-3",
    legend: "px-1 text-sm font-medium text-fg",
  },
});

export type FieldsetProps = Omit<ComponentPropsWithRef<"fieldset">, "className" | "style" | "children"> & {
  /** The group's name: the `legend`, which names the `fieldset` group for assistive technology. */
  legend: ReactNode;
  children: ReactNode;
};

/** The controls of a group one above another (`gap-3`) inside a border, named by `legend`. */
export function Fieldset({ legend, children, ...props }: FieldsetProps) {
  const styles = fieldset();
  return (
    <fieldset {...props} className={styles.root()}>
      <legend className={styles.legend()}>{legend}</legend>
      {children}
    </fieldset>
  );
}
