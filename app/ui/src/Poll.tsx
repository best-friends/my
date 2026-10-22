// SPDX-License-Identifier: AGPL-3.0-only
// The display of a poll (FR-112): the results with a bar of each option's share, the form to vote, and the line of
// the total and the deadline. The strings arrive as props (rule A2); the section around them is the caller's.
import { type ReactNode, useId } from "react";
import { tv } from "tailwind-variants";
import { Button } from "./Button";
import { Checkbox } from "./Checkbox";
import { type CustomEmojiImage, EmojiText } from "./CustomEmoji";
import { Radio, RadioGroup } from "./RadioGroup";
import { VisuallyHidden } from "./VisuallyHidden";

const poll = tv({
  slots: {
    section: "flex min-w-0 flex-col gap-2",
    form: "flex min-w-0 flex-col items-start gap-2",
    fieldset: "flex min-w-0 flex-col gap-2",
    list: "flex min-w-0 flex-col gap-2",
    item: "flex min-w-0 flex-col gap-1",
    row: "flex min-w-0 items-baseline gap-2",
    share: "w-12 shrink-0 text-sm font-semibold tabular-nums",
    own: "shrink-0 text-sm text-fg-muted",
    track: "block h-1.5 w-full overflow-hidden rounded-full bg-surface-sunken",
    fill: "block h-full rounded-full",
    title: "min-w-0 wrap-anywhere",
    hint: "text-sm text-fg-muted",
    summary: "flex min-w-0 flex-wrap gap-x-3 text-sm text-fg-muted",
    total: "tabular-nums",
  },
  variants: {
    own: { true: { fill: "bg-accent" }, false: { fill: "bg-border-strong" } },
  },
});

export type PollOptionProps = {
  /** Plain text, with the custom emoji of `emojis` drawn as images. */
  title: string;
  /** The share in words ("42%"), or null while the votes are hidden. */
  share?: string | null;
  /** The share from 0 to 1, drawn as the bar. */
  ratio?: number;
  /** Whether the viewer voted for it. */
  ownChoice?: boolean;
};

type TitleProps = { title: string; emojis?: readonly CustomEmojiImage[] | undefined; lang?: string | undefined };

/** The title of an option, in the post's language and with its custom emoji. */
function Title({ title, emojis, lang }: TitleProps) {
  return (
    <span dir="auto" lang={lang} className={poll().title()}>
      <EmojiText text={title} emojis={emojis} />
    </span>
  );
}

export type PollProps = {
  /** The accessible name of the poll's section. */
  label: string;
  children: ReactNode;
};

/** The section that holds a poll: its results or form, a hint and the summary line. */
export function Poll({ label, children }: PollProps) {
  return (
    <section aria-label={label} className={poll().section()}>
      {children}
    </section>
  );
}

export type PollResultsProps = {
  /** The accessible name of the list of results. */
  label: string;
  options: readonly (PollOptionProps & { key: string | number })[];
  emojis?: readonly CustomEmojiImage[] | undefined;
  /** The language of the post, for the titles. */
  lang?: string | undefined;
  /** The mark of the viewer's choices, in words ("Your vote"). */
  ownChoiceLabel: string;
};

/** Each option's share of the votes as a percentage and a bar; the bar is decoration, the percentage says the same. */
export function PollResults({ label, options, emojis, lang, ownChoiceLabel }: PollResultsProps) {
  const styles = poll();
  return (
    <ul aria-label={label} className={styles.list()}>
      {options.map((option) => (
        <li key={option.key} className={styles.item()}>
          <span className={styles.row()}>
            {option.share != null && <span className={styles.share()}>{option.share}</span>}
            <Title title={option.title} emojis={emojis} lang={lang} />
            {option.ownChoice === true && <span className={styles.own()}>{ownChoiceLabel}</span>}
          </span>
          {option.share != null && (
            <span aria-hidden="true" className={styles.track()}>
              <span
                className={poll({ own: option.ownChoice === true }).fill()}
                style={{ inlineSize: `${String(Math.round((option.ratio ?? 0) * 100))}%` }}
              />
            </span>
          )}
        </li>
      ))}
    </ul>
  );
}

export type PollChoicesProps = {
  /** The accessible name of the group of options. */
  label: string;
  /** Checkboxes when true, radio buttons otherwise. */
  multiple: boolean;
  /** The instruction ("Choose one", "Choose one or more"). */
  description: string;
  options: readonly { value: string; title: string }[];
  emojis?: readonly CustomEmojiImage[] | undefined;
  lang?: string | undefined;
  /** The values chosen. */
  selected: readonly string[];
  onChange: (value: string, on: boolean) => void;
  isDisabled?: boolean;
  /** The label of the button that votes. */
  submitLabel: string;
  canSubmit: boolean;
  isPending?: boolean;
  onSubmit: () => void;
};

/** The options to choose from and the button that votes. */
export function PollChoices({
  label,
  multiple,
  description,
  options,
  emojis,
  lang,
  selected,
  onChange,
  isDisabled = false,
  submitLabel,
  canSubmit,
  isPending = false,
  onSubmit,
}: PollChoicesProps) {
  const styles = poll();
  const hintId = useId();
  return (
    <form
      className={styles.form()}
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      {multiple ? (
        <fieldset aria-describedby={hintId} className={styles.fieldset()}>
          <legend>
            <VisuallyHidden>{label}</VisuallyHidden>
          </legend>
          <p id={hintId} className={styles.hint()}>
            {description}
          </p>
          {options.map((option) => (
            <Checkbox
              key={option.value}
              isSelected={selected.includes(option.value)}
              onChange={(on) => {
                onChange(option.value, on);
              }}
              isDisabled={isDisabled}
            >
              <Title title={option.title} emojis={emojis} lang={lang} />
            </Checkbox>
          ))}
        </fieldset>
      ) : (
        <RadioGroup
          aria-label={label}
          description={description}
          value={selected[0] ?? null}
          onChange={(value) => {
            onChange(value, true);
          }}
          isDisabled={isDisabled}
        >
          {options.map((option) => (
            <Radio key={option.value} value={option.value}>
              <Title title={option.title} emojis={emojis} lang={lang} />
            </Radio>
          ))}
        </RadioGroup>
      )}
      <Button type="submit" size="sm" isDisabled={!canSubmit} isPending={isPending}>
        {submitLabel}
      </Button>
    </form>
  );
}

export type PollSummaryProps = {
  /** How many voted, in words. */
  total: string;
  /** Shown when the poll has ended. */
  closedLabel?: string | undefined;
  /** The deadline of an open poll: `text` in words, `dateTime` machine readable, `absolute` as the tooltip. */
  ends?: { dateTime: string; absolute: string; text: string } | null | undefined;
  /** A note that the counts are those another server reported. */
  remoteLabel?: string | undefined;
};

/** The line under a poll: the total, and the deadline or that it has closed. */
export function PollSummary({ total, closedLabel, ends, remoteLabel }: PollSummaryProps) {
  const styles = poll();
  return (
    <p className={styles.summary()}>
      <span className={styles.total()}>{total}</span>
      {closedLabel !== undefined && <span>{closedLabel}</span>}
      {ends != null && (
        <time dateTime={ends.dateTime} title={ends.absolute}>
          {ends.text}
        </time>
      )}
      {remoteLabel !== undefined && <span>{remoteLabel}</span>}
    </p>
  );
}

export type PollHintProps = { children: ReactNode };

/** A note under the results ("The votes are hidden until the poll ends"). */
export function PollHint({ children }: PollHintProps) {
  return <p className={poll().hint()}>{children}</p>;
}
