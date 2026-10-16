// SPDX-License-Identifier: AGPL-3.0-only
import type { ClientMessageKey } from "../../../lib/i18n";
import type { Visibility } from "../api/compose";

/** The name of each visibility. */
export const visibilityLabel: Readonly<Record<Visibility, ClientMessageKey>> = {
  public: "client:compose.visibility.public",
  local: "client:compose.visibility.local",
  unlisted: "client:compose.visibility.unlisted",
  private: "client:compose.visibility.private",
  direct: "client:compose.visibility.direct",
};

/** Who can see a post of each visibility (ADR-0017). */
export const visibilityDescription: Readonly<Record<Visibility, ClientMessageKey>> = {
  public: "client:compose.visibility.public.description",
  local: "client:compose.visibility.local.description",
  unlisted: "client:compose.visibility.unlisted.description",
  private: "client:compose.visibility.private.description",
  direct: "client:compose.visibility.direct.description",
};
