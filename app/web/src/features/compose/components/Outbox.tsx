// SPDX-License-Identifier: AGPL-3.0-only
import { Button, Spinner } from "@my/ui";
import { useT } from "../../../lib/i18n";
import type { OutboxView } from "../hooks/useComposeForm";

export type OutboxProps = {
  items: readonly OutboxView[];
  onRetry: (idempotencyKey: string) => void;
  onEdit: (idempotencyKey: string) => void;
  canEdit: boolean;
  onDiscard: (idempotencyKey: string) => void;
};

/**
 * The posts on their way to the server, shown from what was sent until the server confirms them
 * (web-client.md S8), and the ones that could not be sent, with what to do about them.
 */
export function Outbox({ items, onRetry, onEdit, canEdit, onDiscard }: OutboxProps) {
  const t = useT();
  if (items.length === 0) return null;
  return (
    <ul aria-label={t("client:compose.outbox.label")} className="flex flex-col gap-2">
      {items.map((item) => (
        <li key={item.idempotencyKey} className="flex flex-col gap-2 rounded-md border border-border p-3" aria-busy={item.status === "sending"}>
          <p dir="auto" lang={item.language} className="line-clamp-3 whitespace-pre-wrap wrap-anywhere text-fg-muted">
            {item.text}
          </p>
          {item.status === "sending" ? (
            <p className="flex items-center gap-2 text-sm text-fg-muted">
              <Spinner size="sm" label={t("client:compose.outbox.sending")} />
              <span aria-hidden="true">{t("client:compose.outbox.sending")}</span>
            </p>
          ) : (
            <>
              <p className="text-sm text-danger">
                {item.status === "failed" ? t("client:compose.outbox.failed", { reason: item.message }) : t("client:compose.outbox.unsent")}
              </p>
              <div className="flex flex-wrap gap-2">
                <Button
                  size="sm"
                  onPress={() => {
                    onRetry(item.idempotencyKey);
                  }}
                >
                  {t("client:compose.outbox.retry")}
                </Button>
                {canEdit && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onPress={() => {
                      onEdit(item.idempotencyKey);
                    }}
                  >
                    {t("client:compose.outbox.edit")}
                  </Button>
                )}
                <Button
                  size="sm"
                  variant="ghost"
                  onPress={() => {
                    onDiscard(item.idempotencyKey);
                  }}
                >
                  {t("client:compose.outbox.discard")}
                </Button>
              </div>
            </>
          )}
        </li>
      ))}
    </ul>
  );
}
