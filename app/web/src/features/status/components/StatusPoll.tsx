// SPDX-License-Identifier: AGPL-3.0-only
// The poll of a post (FR-112): radio buttons or checkboxes and a button to vote while the viewer may, and
// otherwise each option's share of the votes, the viewer's choices marked in words, not by color alone.
import { Button, Checkbox, EmojiText, Radio, RadioGroup, VisuallyHidden } from "@my/ui";
import { useId } from "react";
import { useT } from "../../../lib/i18n";
import type { Status } from "../api/status";
import { type PollForm, type PollOptionView, type PollView, usePoll } from "../hooks/usePoll";

export type StatusPollProps = {
  status: Status;
  /** Whether the viewer may vote here; false shows the results only (a moderator's page). */
  canVote: boolean;
};

/** The bar of an option's share: the viewer's choices in the accent color, the others in a neutral one. */
const ownChoiceFill = "block h-full rounded-full bg-accent";
const otherFill = "block h-full rounded-full bg-border-strong";

/** The poll of `status`, or nothing without one. */
export function StatusPoll({ status, canVote }: StatusPollProps) {
  const t = useT();
  const view = usePoll(status, canVote);
  if (view === null) return null;
  return (
    <section aria-label={t("client:status.poll.label")} className="flex min-w-0 flex-col gap-2">
      {view.form === null ? <Results view={view} /> : <Form view={view} form={view.form} />}
      <p className="flex min-w-0 flex-wrap gap-x-3 text-sm text-fg-muted">
        <span className="tabular-nums">{view.total}</span>
        {view.closed && <span>{t("client:status.poll.closed")}</span>}
        {!view.closed && view.ends !== null && (
          <time dateTime={view.ends.dateTime} title={view.ends.absolute}>
            {t("client:status.poll.ends", { time: view.ends.relative })}
          </time>
        )}
        {view.remoteHost !== null && view.form === null && <span>{t("client:status.poll.remote_counts", { host: view.remoteHost })}</span>}
      </p>
    </section>
  );
}

/** The title of an option, in the post's language and with its custom emoji. */
function Title({ view, option }: { view: PollView; option: PollOptionView }) {
  return (
    <span dir="auto" lang={view.lang} className="min-w-0 wrap-anywhere">
      <EmojiText text={option.title} emojis={view.emojis} />
    </span>
  );
}

/** The options to choose from and the button that votes. */
function Form({ view, form }: { view: PollView; form: PollForm }) {
  const t = useT();
  const hintId = useId();
  return (
    <form
      className="flex min-w-0 flex-col items-start gap-2"
      onSubmit={(event) => {
        event.preventDefault();
        form.submit();
      }}
    >
      {view.multiple ? (
        <fieldset aria-describedby={hintId} className="flex min-w-0 flex-col gap-2">
          <legend>
            <VisuallyHidden>{t("client:status.poll.options")}</VisuallyHidden>
          </legend>
          <p id={hintId} className="text-sm text-fg-muted">
            {t("client:status.poll.choose_many")}
          </p>
          {view.options.map((option) => (
            <Checkbox
              key={option.index}
              isSelected={form.selected.includes(option.index)}
              onChange={(on) => {
                form.toggle(option.index, on);
              }}
              isDisabled={form.isSubmitting}
            >
              <Title view={view} option={option} />
            </Checkbox>
          ))}
        </fieldset>
      ) : (
        <RadioGroup
          aria-label={t("client:status.poll.options")}
          description={t("client:status.poll.choose_one")}
          value={form.selected.length > 0 ? String(form.selected[0]) : null}
          onChange={(value) => {
            form.choose(Number(value));
          }}
          isDisabled={form.isSubmitting}
        >
          {view.options.map((option) => (
            <Radio key={option.index} value={String(option.index)}>
              <Title view={view} option={option} />
            </Radio>
          ))}
        </RadioGroup>
      )}
      <Button type="submit" size="sm" isDisabled={!form.canSubmit} isPending={form.isSubmitting}>
        {t("client:status.poll.vote")}
      </Button>
    </form>
  );
}

/** Each option's share of the votes, as a percentage and a bar, with the viewer's choices marked. */
function Results({ view }: { view: PollView }) {
  const t = useT();
  return (
    <ul aria-label={t("client:status.poll.results")} className="flex min-w-0 flex-col gap-2">
      {view.options.map((option) => {
        return (
          <li key={option.index} className="flex min-w-0 flex-col gap-1">
            <span className="flex min-w-0 items-baseline gap-2">
              <span className="w-12 shrink-0 text-sm font-semibold tabular-nums">{option.share}</span>
              <Title view={view} option={option} />
              {option.ownChoice && <span className="shrink-0 text-sm text-fg-muted">{t("client:status.poll.own_vote")}</span>}
            </span>
            {/* The share drawn as a bar; the percentage before it says the same. */}
            <span aria-hidden="true" className="block h-1.5 w-full overflow-hidden rounded-full bg-surface-sunken">
              <span className={option.ownChoice ? ownChoiceFill : otherFill} style={{ inlineSize: `${String(Math.round(option.ratio * 100))}%` }} />
            </span>
          </li>
        );
      })}
    </ul>
  );
}
