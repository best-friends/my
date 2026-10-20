// SPDX-License-Identifier: AGPL-3.0-only
// The emoji reactions under a post (FR-116, ADR-0052 §5): one toggle per emoji with its count, which the viewer
// presses to react with that emoji or to take the reaction back, and a button that lists the accounts that reacted.
// A custom emoji of another server is shown with its count but is no button: nobody here can react with it.
import { Avatar, Button, CustomEmoji, Dialog, EmojiText, Link, ToggleButton } from "@my/ui";
import { useId, useState } from "react";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import type { ReactionEmoji, Status } from "../api/status";
import { type ReactionChip, useReactors, useStatusReactions } from "../hooks/useStatusReactions";

export type StatusReactionsProps = {
  status: Status;
};

/** The reactions of `status` in a group, or nothing without any. */
export function StatusReactions({ status }: StatusReactionsProps) {
  const t = useT();
  const chips = useStatusReactions(status);
  const [isListOpen, setIsListOpen] = useState(false);
  if (chips.length === 0) return null;
  const post = status.boost ?? status;
  return (
    <div role="group" aria-label={t("client:reaction.list.label")} className="flex min-w-0 flex-wrap items-center gap-1">
      {chips.map((chip) => (
        <Chip key={chip.key} chip={chip} />
      ))}
      <Button
        variant="ghost"
        size="sm"
        className="text-fg-muted"
        onPress={() => {
          setIsListOpen(true);
        }}
      >
        {t("client:reaction.list.open")}
      </Button>
      <ReactorsDialog statusId={post.id} isOpen={isListOpen} onOpenChange={setIsListOpen} />
    </div>
  );
}

/** The emoji of a reaction: the Unicode emoji, or the image of a custom emoji (its `:shortcode:` without one). */
function Emoji({ emoji, text }: { emoji: ReactionEmoji; text: string }) {
  if (emoji.emoji !== undefined) return <CustomEmoji emoji={emoji.emoji} />;
  return (
    <span dir="auto" className="text-base">
      {emoji.shortcode === undefined ? emoji.name : text}
    </span>
  );
}

/** One emoji with its count: a toggle (named by the emoji, described by the count), or the same shown only. */
function Chip({ chip }: { chip: ReactionChip }) {
  const t = useT();
  const intl = useIntl();
  const countId = useId();
  const count = (
    <span id={countId} className="tabular-nums">
      {intl.formatNumber(chip.count, { notation: "compact" })}
    </span>
  );
  if (chip.onChange === null) {
    return (
      <span className="inline-flex min-h-8 items-center gap-1.5 rounded-md px-2 text-sm font-medium text-fg-muted">
        <Emoji emoji={chip.emoji} text={chip.text} />
        {count}
      </span>
    );
  }
  return (
    <ToggleButton
      aria-label={t("client:reaction.toggle", { emoji: chip.text })}
      aria-describedby={countId}
      tone="accent"
      isSelected={chip.isSelected}
      onChange={chip.onChange}
      className="border border-border data-selected:border-accent"
    >
      <Emoji emoji={chip.emoji} text={chip.text} />
      {count}
    </ToggleButton>
  );
}

type ReactorsDialogProps = {
  statusId: string;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
};

/**
 * The accounts that reacted to the post, newest first, each with its emoji, in a dialog. A button rather than loading
 * at the end of the scroll loads the next page, which keeps the end of the list reachable by keyboard (WCAG 2.1.1).
 */
function ReactorsDialog({ statusId, isOpen, onOpenChange }: ReactorsDialogProps) {
  const t = useT();
  const state = useReactors(statusId, isOpen);
  return (
    <Dialog title={t("client:reaction.list.title")} closeLabel={t("client:reaction.list.close")} isOpen={isOpen} onOpenChange={onOpenChange}>
      {state.status === "loading" ? (
        <p role="status">{t("client:reaction.list.loading")}</p>
      ) : state.status === "error" ? (
        <div role="alert" className="flex flex-col items-start gap-2">
          <p>{t("client:reaction.list.failed", { reason: state.error })}</p>
          <Button onPress={state.retry}>{t("client:reaction.list.retry")}</Button>
        </div>
      ) : (
        <div className="flex flex-col gap-3">
          {state.items.length === 0 ? (
            <p>{t("client:reaction.list.empty")}</p>
          ) : (
            <ul className="flex flex-col divide-y divide-border">
              {state.items.map((item) => (
                <li key={item.key} className="flex min-w-0 items-center gap-3 py-2">
                  <Link
                    href={item.href}
                    variant="standalone"
                    className="grid min-w-0 flex-1 grid-cols-[auto_minmax(0,1fr)] items-center gap-x-3 text-fg data-hovered:text-fg"
                  >
                    <>
                      <Avatar sources={item.avatar} name={item.name} decorative className="row-span-2" />
                      <span dir="auto" className="font-semibold wrap-anywhere">
                        <EmojiText text={item.name} emojis={item.emojis} />
                      </span>
                      {/* The space keeps the name and the handle apart in the link's name. */}{" "}
                      <span dir="ltr" className="text-sm font-normal wrap-anywhere text-fg-muted">
                        {item.handle}
                      </span>
                    </>
                  </Link>
                  <span className="shrink-0">
                    <Emoji emoji={item.emoji} text={item.emojiText} />
                  </span>
                </li>
              ))}
            </ul>
          )}
          {state.loadMoreError !== null && <p>{t("client:reaction.list.load_more_failed", { reason: state.loadMoreError })}</p>}
          {state.hasMore && (
            <div className="flex justify-center">
              {/* A pending button keeps its focus while the page loads (React Aria Button, isPending). */}
              <Button isPending={state.isLoadingMore} onPress={state.loadMore}>
                {t("client:reaction.list.load_more")}
              </Button>
            </div>
          )}
        </div>
      )}
    </Dialog>
  );
}
