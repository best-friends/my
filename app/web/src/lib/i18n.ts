// SPDX-License-Identifier: AGPL-3.0-only
// Messages for react-intl, read from the shared catalogs in locales/ (ADR-0009).
// Locale negotiation, fallback and the typed accessors from tools/i18n come with the app shell (#126);
// until then the app renders the source locale.
import client from "../../../../locales/en/client.json";

/** One locales/<locale>/<namespace>.json file (the FormatJS JSON catalog shape). */
type Namespace = Record<string, { defaultMessage: string }>;

/** Keys messages as "<namespace>:<key>", the same IDs as MessageKey in tools/i18n. */
function toMessages(namespace: string, catalog: Namespace): Record<string, string> {
  return Object.fromEntries(Object.entries(catalog).map(([key, entry]) => [`${namespace}:${key}`, entry.defaultMessage]));
}

export const locale = "en";
export const messages = toMessages("client", client);
