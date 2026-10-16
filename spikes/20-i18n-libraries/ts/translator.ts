// SPDX-License-Identifier: AGPL-3.0-only
// Minimal typed translator over intl-messageformat. Keys and argument types
// come from the generated MessageArgs, so a wrong key or argument is a
// compile-time error (see ts/usage.ts).
import { IntlMessageFormat } from "intl-messageformat";
import { type Catalog, formattingLocale } from "./catalog";
import type { MessageArgs, MessageKey } from "./messages.gen";

type ArgsParam<K extends MessageKey> = keyof MessageArgs[K] extends never ? [] : [args: MessageArgs[K]];

/** zh-Hant-TW -> zh-Hant -> zh -> en (ADR-0009 fallback chain, without the exception table). */
export function fallbackChain(locale: string): string[] {
  const parts = locale.split("-");
  const chain: string[] = [];
  for (let i = parts.length; i > 0; i--) chain.push(parts.slice(0, i).join("-"));
  if (!chain.includes("en")) chain.push("en");
  return chain;
}

export function createTranslator(catalog: Catalog, locale: string) {
  const chain = fallbackChain(locale);
  const cache = new Map<string, IntlMessageFormat>();
  return function t<K extends MessageKey>(key: K, ...[args]: ArgsParam<K>): string {
    const [ns, id] = key.split(/:(.*)/s);
    for (const loc of chain) {
      const source = catalog[loc]?.[ns]?.[id]?.defaultMessage;
      if (source === undefined) continue;
      const cacheKey = `${loc}\u0000${key}`;
      let mf = cache.get(cacheKey);
      if (!mf) {
        mf = new IntlMessageFormat(source, formattingLocale(loc));
        cache.set(cacheKey, mf);
      }
      return String(mf.format(args as Parameters<IntlMessageFormat["format"]>[0]));
    }
    return key;
  };
}
