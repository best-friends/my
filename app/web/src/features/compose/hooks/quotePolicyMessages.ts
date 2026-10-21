// SPDX-License-Identifier: AGPL-3.0-only
import type { ClientMessageKey } from "../../../lib/i18n";
import type { QuotePolicy } from "../api/compose";

/** The quote policies (FR-114, ADR-0069 §1), widest first. */
export const quotePolicies: readonly QuotePolicy[] = ["public", "followers", "nobody"];

/** The name of each quote policy. */
export const quotePolicyLabel: Readonly<Record<QuotePolicy, ClientMessageKey>> = {
  public: "client:compose.quote_policy.public",
  followers: "client:compose.quote_policy.followers",
  nobody: "client:compose.quote_policy.nobody",
};

/** Who may quote a post under each quote policy. */
export const quotePolicyDescription: Readonly<Record<QuotePolicy, ClientMessageKey>> = {
  public: "client:compose.quote_policy.public.description",
  followers: "client:compose.quote_policy.followers.description",
  nobody: "client:compose.quote_policy.nobody.description",
};
