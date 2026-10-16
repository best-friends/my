// SPDX-License-Identifier: AGPL-3.0-only
import type { ReactNode } from "react";
import {
  Button,
  composeRenderProps,
  ListBox,
  ListBoxItem,
  type ListBoxItemProps,
  type ListBoxProps,
  Popover,
  Select as RACSelect,
  type SelectProps as RACSelectProps,
  SelectValue,
} from "react-aria-components";
import { tv } from "tailwind-variants";
import { FieldDescription, FieldError, FieldLabel, type FieldTextProps, fieldControl } from "./Field";

const select = tv({
  slots: {
    root: "group flex flex-col gap-1",
    trigger: [
      fieldControl(),
      "flex cursor-default items-center justify-between gap-2 text-start",
      "group-data-invalid:border-danger data-focus-visible:focus-ring data-pressed:bg-surface-raised",
    ],
    value: "truncate data-placeholder:text-fg-muted",
    chevron: "size-4 shrink-0 fill-none stroke-current stroke-2 text-fg-muted",
    // overscroll-contain keeps the scroll from chaining to the page on iOS WebKit (web-client.md §9).
    popover: "min-w-(--trigger-width) overflow-auto overscroll-contain rounded-md border border-border bg-surface-raised p-1 shadow-md",
    listBox: "outline-hidden",
  },
});

export interface SelectProps<T extends object> extends Omit<RACSelectProps<T>, "children" | "selectionMode">, FieldTextProps {
  /** The options, for a dynamic collection rendered by a `children` function. */
  items?: ListBoxProps<T>["items"];
  /** `SelectItem`s, or a function from an item of `items` to its `SelectItem`. */
  children: ReactNode | ((item: T) => ReactNode);
}

/**
 * A select (React Aria Select, the APG select-only combobox): Enter, Space or the arrow keys open the
 * list, typing jumps to an option. Controlled with `value` / `onChange` or uncontrolled with `defaultValue`;
 * the value is the selected `SelectItem`'s `id`.
 */
export function Select<T extends object>({ label, description, errorMessage, items, children, className, ...props }: SelectProps<T>) {
  const styles = select();
  return (
    <RACSelect {...props} className={composeRenderProps(className, (className) => styles.root({ className }))}>
      {label !== undefined && <FieldLabel>{label}</FieldLabel>}
      <Button className={styles.trigger()}>
        <SelectValue className={styles.value()} />
        <svg viewBox="0 0 24 24" aria-hidden className={styles.chevron()}>
          <path d="m6 9 6 6 6-6" />
        </svg>
      </Button>
      {description !== undefined && <FieldDescription>{description}</FieldDescription>}
      <FieldError>{errorMessage}</FieldError>
      <Popover className={styles.popover()}>
        <ListBox className={styles.listBox()} {...(items === undefined ? {} : { items })}>
          {children}
        </ListBox>
      </Popover>
    </RACSelect>
  );
}

const selectItem = tv({
  slots: {
    root: [
      "flex min-h-8 cursor-default items-center gap-2 rounded-sm px-2 py-1 text-base text-fg outline-hidden",
      "data-disabled:opacity-50 data-focus-visible:focus-ring data-focused:bg-surface-sunken pointer-coarse:min-h-11",
    ],
    check: "size-4 shrink-0 fill-none stroke-current stroke-3 text-accent",
    // Keeps the label aligned with the selected one, whose check takes this space.
    spacer: "size-4 shrink-0",
  },
});

export type SelectItemProps<T extends object> = Omit<ListBoxItemProps<T>, "children"> & {
  /** The option's label. A non-string label needs `textValue` for type-ahead and the closed select. */
  children?: ReactNode;
};

/** One option of a `Select`; its `id` is the select's value when it is chosen. */
export function SelectItem<T extends object>({ children, className, ...props }: SelectItemProps<T>) {
  const styles = selectItem();
  const textValue = props.textValue ?? (typeof children === "string" ? children : undefined);
  return (
    <ListBoxItem {...props} {...(textValue === undefined ? {} : { textValue })} className={composeRenderProps(className, (className) => styles.root({ className }))}>
      {({ isSelected }) => (
        <>
          {isSelected ? (
            <svg viewBox="0 0 24 24" aria-hidden className={styles.check()}>
              <path d="m5 12 5 5 9-10" />
            </svg>
          ) : (
            <span className={styles.spacer()} />
          )}
          {children}
        </>
      )}
    </ListBoxItem>
  );
}
