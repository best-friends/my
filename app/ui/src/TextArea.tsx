// SPDX-License-Identifier: AGPL-3.0-only
import { useEffect, useId, useState } from "react";
import {
  composeRenderProps,
  TextArea as RACTextArea,
  type TextAreaProps as RACTextAreaProps,
  TextField as RACTextField,
  type TextFieldProps as RACTextFieldProps,
} from "react-aria-components";
import { tv } from "tailwind-variants";
import { FieldDescription, FieldError, FieldLabel, type FieldTextProps, fieldControl } from "./Field";

const textArea = tv({
  slots: {
    root: "flex flex-col gap-1",
    input: [fieldControl(), "min-h-24 resize-y"],
    count: "text-sm text-fg-muted",
    status: "sr-only",
  },
  variants: {
    over: { true: { count: "font-medium text-danger" } },
  },
});

/** Remaining counts that are announced at once when typing reaches them (ADR-0027). */
const thresholds = [50, 20, 0] as const;

/**
 * How long typing must pause before the count is announced. The GOV.UK character count waits for a
 * pause, because a live region updated on every keystroke interrupts the user's own typing echo.
 */
export const countAnnounceDelay = 1000;

/** The number of thresholds `remaining` has reached; it grows as the text gets closer to the limit. */
function thresholdLevel(remaining: number) {
  return thresholds.filter((threshold) => remaining <= threshold).length;
}

/** Characters as the server counts them: Unicode code points (data-model/README.md). */
function countCharacters(text: string) {
  return Array.from(text).length;
}

type CharacterCountProps =
  | { characterLimit?: undefined; formatRemaining?: undefined }
  | {
      /** The most characters the text may have. Longer text is not cut off; it makes the field invalid. */
      characterLimit: number;
      /** The count message, e.g. "12 characters left" / "3 characters too many"; `remaining` is negative when over. */
      formatRemaining: (remaining: number) => string;
    };

export type TextAreaProps = Omit<RACTextFieldProps, "children"> &
  FieldTextProps &
  CharacterCountProps & {
    placeholder?: RACTextAreaProps["placeholder"];
    rows?: RACTextAreaProps["rows"];
  };

/**
 * A multi-line text field (React Aria TextField with a TextArea), controlled with `value` / `onChange`
 * or uncontrolled with `defaultValue`.
 *
 * With `characterLimit`, a count follows the control. It follows the GOV.UK character count: the visible
 * count is linked to the text area with aria-describedby (hidden from the reading order so it is not
 * read twice), and a separate `role="status"` region announces it when typing pauses for
 * `countAnnounceDelay` ms, or at once when the remaining count reaches 50, 20 or 0.
 */
export function TextArea({
  label,
  description,
  errorMessage,
  placeholder,
  rows,
  characterLimit,
  formatRemaining,
  className,
  value,
  defaultValue,
  onChange,
  isInvalid,
  "aria-describedby": ariaDescribedBy,
  ...props
}: TextAreaProps) {
  const [uncontrolledValue, setUncontrolledValue] = useState(defaultValue ?? "");
  const text = value ?? uncontrolledValue;
  // The remaining count to announce after the pause, and the one the status region says now.
  const [pending, setPending] = useState<number | null>(null);
  const [announced, setAnnounced] = useState<number | null>(null);
  const countId = useId();

  const remaining = characterLimit === undefined ? undefined : characterLimit - countCharacters(text);
  const over = remaining !== undefined && remaining < 0;

  useEffect(() => {
    if (pending === null) return;
    const timer = setTimeout(() => {
      setAnnounced(pending);
      setPending(null);
    }, countAnnounceDelay);
    return () => {
      clearTimeout(timer);
    };
  }, [pending]);

  function handleChange(next: string) {
    if (value === undefined) setUncontrolledValue(next);
    if (characterLimit !== undefined && remaining !== undefined) {
      const nextRemaining = characterLimit - countCharacters(next);
      if (thresholdLevel(nextRemaining) > thresholdLevel(remaining)) {
        setPending(null);
        setAnnounced(nextRemaining);
      } else {
        setPending(nextRemaining);
      }
    }
    onChange?.(next);
  }

  const styles = textArea({ over });
  const describedBy = [ariaDescribedBy, remaining === undefined ? undefined : countId].filter((id) => id !== undefined).join(" ");

  return (
    <RACTextField
      {...props}
      value={text}
      onChange={handleChange}
      // Only a true isInvalid is passed: any defined value overrides React Aria's own validation (`validate`,
      // `isRequired`), which would then never mark the field invalid or stop its form.
      {...(isInvalid === true || over ? { isInvalid: true } : {})}
      {...(describedBy === "" ? {} : { "aria-describedby": describedBy })}
      className={composeRenderProps(className, (className) => styles.root({ className }))}
    >
      {label !== undefined && <FieldLabel>{label}</FieldLabel>}
      <RACTextArea className={styles.input()} {...(placeholder === undefined ? {} : { placeholder })} {...(rows === undefined ? {} : { rows })} />
      {description !== undefined && <FieldDescription>{description}</FieldDescription>}
      {remaining !== undefined && formatRemaining !== undefined && (
        <>
          <span id={countId} aria-hidden className={styles.count()}>
            {formatRemaining(remaining)}
          </span>
          <span role="status" className={styles.status()}>
            {announced === null ? "" : formatRemaining(announced)}
          </span>
        </>
      )}
      <FieldError>{errorMessage}</FieldError>
    </RACTextField>
  );
}
