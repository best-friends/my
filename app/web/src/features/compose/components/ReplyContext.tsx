// SPDX-License-Identifier: AGPL-3.0-only
import { Button, PostContent, serverSanitizedHtml, Spinner } from "@my/ui";
import { useT } from "../../../lib/i18n";
import type { ReplyTarget } from "../hooks/useComposeForm";

export type ReplyContextProps = { replyTo: Exclude<ReplyTarget, { status: "none" }>; onCancel: () => void; isDisabled?: boolean };

/**
 * The post being replied to, above the form: its author and the start of its text (or only its content
 * warning, which the reply does not open), and a button that makes the draft a normal post.
 */
export function ReplyContext({ replyTo, onCancel, isDisabled = false }: ReplyContextProps) {
  const t = useT();
  return (
    <div className="flex flex-col gap-2 rounded-md border border-border bg-surface-sunken p-3">
      {replyTo.status === "loading" && <Spinner size="sm" label={t("client:shell.loading")} />}
      {replyTo.status === "error" && <p className="text-sm text-fg-muted">{t("client:compose.reply.unavailable")}</p>}
      {replyTo.status === "ready" && (
        <>
          <p className="text-sm font-medium">{t("client:compose.reply.label", { name: replyTo.post.account.display_name || `@${replyTo.post.account.acct}` })}</p>
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
        {t("client:compose.reply.cancel")}
      </Button>
    </div>
  );
}
