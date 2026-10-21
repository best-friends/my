// SPDX-License-Identifier: AGPL-3.0-only
// What the quote of a post shows (FR-114, ADR-0069 §5): the quoted post, the post behind the notice of the
// reader's mute, or only the notice of why it is not shown. It returns no JSX (web-client.md A4).
import { useT } from "../../../lib/i18n";
import type { Status, StatusQuote } from "../api/status";

export type QuoteView =
  /** The quoted post, behind `notice` when it is set. */
  | { kind: "post"; status: Status; notice: string | null }
  /** Why the quoted post is not shown. */
  | { kind: "notice"; notice: string };

export function useQuoteView(quote: StatusQuote): QuoteView {
  const t = useT();
  const status = quote.quoted_status;
  switch (quote.state) {
    case "accepted":
    case "legacy":
      if (status !== undefined) return { kind: "post", status, notice: null };
      break;
    case "muted_account":
      if (status !== undefined) return { kind: "post", status, notice: t("client:status.quote.muted_account") };
      break;
    case "blocked_domain":
      if (status !== undefined) return { kind: "post", status, notice: t("client:status.quote.blocked_domain") };
      break;
    case "rejected":
      return { kind: "notice", notice: t("client:status.quote.rejected") };
    case "revoked":
      return { kind: "notice", notice: t("client:status.quote.revoked") };
    case "deleted":
      return { kind: "notice", notice: t("client:status.quote.deleted") };
    case "unauthorized":
      return { kind: "notice", notice: t("client:status.quote.unauthorized") };
    case "blocked_account":
      return { kind: "notice", notice: t("client:status.quote.blocked_account") };
    case "pending":
      break;
  }
  return { kind: "notice", notice: t("client:status.quote.pending") };
}
