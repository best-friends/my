// SPDX-License-Identifier: AGPL-3.0-only
import { Button, PostContent, serverSanitizedHtml, Spinner } from "@my/ui";
import { useT } from "../../../lib/i18n";
import type { ReplyTarget } from "../hooks/useComposeForm";

export type ReplyContextProps = {
  replyTo: Exclude<ReplyTarget, { status: "none" }>;
  onCancel: () => void;
  isDisabled?: boolean;
  /** Whether the post is replied to (the default) or quoted (FR-114). */
  kind?: "reply" | "quote";
};

const messages = {
  reply: { label: "client:compose.reply.label", unavailable: "client:compose.reply.unavailable", cancel: "client:compose.reply.cancel" },
  quote: { label: "client:compose.quote.label", unavailable: "client:compose.quote.unavailable", cancel: "client:compose.quote.cancel" },
} as const;

/**
 * The post being replied to or quoted, above the form: its author and the start of its text (or only its
 * content warning, which the reply does not open), and a button that drops it from the draft.
 */
export function ReplyContext({ replyTo, onCancel, isDisabled = false, kind = "reply" }: ReplyContextProps) {
  const t = useT();
  const m = messages[kind];
  return (
    <div className="flex flex-col gap-2 rounded-md border border-border bg-surface-sunken p-3">
      {replyTo.status === "loading" && <Spinner size="sm" label={t("client:shell.loading")} />}
      {replyTo.status === "error" && <p className="text-sm text-fg-muted">{t(m.unavailable)}</p>}
      {replyTo.status === "ready" && (
        <>
          <p className="text-sm font-medium">{t(m.label, { name: replyTo.post.account.display_name || `@${replyTo.post.account.acct}` })}</p>
          {replyTo.post.spoiler_text === "" ? (
            <PostContent html={serverSanitizedHtml(replyTo.post.content)} lang={replyTo.post.language} className="line-clamp-3 text-sm" />
          ) : (
            <p dir="auto" lang={replyTo.post.language} className="text-sm wrap-anywhere">
              {replyTo.post.spoiler_text}
            </p>
          )}
        </>
      )}
      <Button size="sm" variant="ghost" className="self-start" onPress={onCancel} isDisabled={isDisabled}>
        {t(m.cancel)}
      </Button>
    </div>
  );
}
