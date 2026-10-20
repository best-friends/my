// SPDX-License-Identifier: AGPL-3.0-only
import { Button, Checkbox, CloseIcon, Select, SelectItem, TextField, Tooltip, TooltipTrigger } from "@my/ui";
import { useId } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { durationParts, maxPollOptionLength, maxPollOptions, minPollOptions, type PollDraft, pollDurations } from "../hooks/poll";
import type { PollForm } from "../hooks/useComposeForm";

export type PollEditorProps = { poll: PollForm; draft: PollDraft };

/** The label of a duration by its unit. */
const durationLabels = {
  minutes: "client:compose.poll.duration.minutes",
  hours: "client:compose.poll.duration.hours",
  days: "client:compose.poll.duration.days",
} as const satisfies Record<ReturnType<typeof durationParts>["unit"], ClientMessageKey>;

/**
 * The poll of a new post (FR-112, web-client.md §12): a group of its options, each a text field with a button that
 * removes it while there are more than the fewest, a button that adds one up to the most, how long it takes votes,
 * and whether a vote may choose several options and whether its counts stay hidden until it ends.
 */
export function PollEditor({ poll, draft }: PollEditorProps) {
  const t = useT();
  const problemId = useId();
  const canRemove = draft.options.length > minPollOptions;
  return (
    <fieldset
      aria-describedby={poll.problem === "duplicate" ? problemId : undefined}
      className="flex min-w-0 flex-col gap-3 rounded-md border border-border p-3"
    >
      <legend className="px-1 text-sm font-medium text-fg">{t("client:compose.poll.legend")}</legend>
      <ol className="flex flex-col gap-2">
        {draft.options.map((option, index) => {
          const number = index + 1;
          const removeLabel = t("client:compose.poll.option.remove", { number: String(number) });
          return (
            <li key={option.key} className="flex items-end gap-1">
              <TextField
                className="min-w-0 flex-1"
                label={t("client:compose.poll.option", { number: String(number) })}
                value={option.title}
                maxLength={maxPollOptionLength}
                onChange={(title) => {
                  poll.editOption(option.key, title);
                }}
              />
              {canRemove && (
                <TooltipTrigger>
                  <Button
                    variant="ghost"
                    size="toolbar"
                    aria-label={removeLabel}
                    onPress={() => {
                      poll.removeOption(option.key);
                    }}
                  >
                    <CloseIcon />
                  </Button>
                  <Tooltip>{removeLabel}</Tooltip>
                </TooltipTrigger>
              )}
            </li>
          );
        })}
      </ol>
      {poll.problem === "duplicate" && (
        <p id={problemId} className="text-sm text-danger">
          {t("client:compose.poll.duplicate")}
        </p>
      )}
      {draft.options.length < maxPollOptions && (
        <div>
          <Button size="sm" onPress={poll.addOption}>
            {t("client:compose.poll.option.add")}
          </Button>
        </div>
      )}
      <Select
        label={t("client:compose.poll.duration")}
        value={String(draft.expiresIn)}
        onChange={(key) => {
          if (typeof key === "string") poll.edit({ expiresIn: Number(key) });
        }}
        items={pollDurations.map((seconds) => {
          const { unit, count } = durationParts(seconds);
          return { id: String(seconds), name: t(durationLabels[unit], { count }) };
        })}
      >
        {(choice) => (
          <SelectItem id={choice.id} textValue={choice.name}>
            {choice.name}
          </SelectItem>
        )}
      </Select>
      <Checkbox
        isSelected={draft.multiple}
        onChange={(multiple) => {
          poll.edit({ multiple });
        }}
      >
        {t("client:compose.poll.multiple")}
      </Checkbox>
      <Checkbox
        isSelected={draft.hideTotals}
        onChange={(hideTotals) => {
          poll.edit({ hideTotals });
        }}
      >
        {t("client:compose.poll.hide_totals")}
      </Checkbox>
    </fieldset>
  );
}
