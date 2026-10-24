// SPDX-License-Identifier: AGPL-3.0-only
// The ranges a text field highlights while the user types (ADR-0092 §4): the tokens of lib/textTokens.ts that the
// server reads in that field, and the text over the field's character limit. They are the `highlights` of the text
// fields of @my/ui (its TextHighlight, which this type matches), which paint them on the layer behind the text.
import { type TextToken, textTokens } from "./textTokens";

/** A highlighted range, [start, end) in UTF-16 units: a token, or the text over the limit. */
interface TextHighlight {
  readonly kind: TextToken["kind"] | "over";
  readonly start: number;
  readonly end: number;
}

type HighlightRules = {
  /**
   * The tokens of the field (ADR-0092 §5): `all` for a post or a profile's note and field values, which the server
   * links, or `shortcodes` for a content warning or a display name, where it reads only custom emoji.
   */
  tokens: "all" | "shortcodes";
  /** The shortcodes of the local custom emoji; a `:shortcode:` that is not one is not highlighted. */
  shortcodes: ReadonlySet<string>;
  /** The most characters the field takes, counted in code points as the server counts them; the rest is `over`. */
  limit?: number;
};

/** The UTF-16 offset after the first `count` code points of `text`, or null when it has no more than that. */
function offsetAfterCodePoints(text: string, count: number): number | null {
  let offset = 0;
  let seen = 0;
  for (const codePoint of text) {
    if (seen === count) return offset;
    offset += codePoint.length;
    seen++;
  }
  return null;
}

/** The highlighted ranges of `text` under `rules`, in order; the `over` range may overlap the last tokens. */
export function textHighlights(text: string, { tokens, shortcodes, limit }: HighlightRules): TextHighlight[] {
  const highlights: TextHighlight[] = [];
  for (const token of textTokens(text)) {
    if (token.kind === "shortcode" ? !shortcodes.has(token.text.slice(1, -1)) : tokens !== "all") continue;
    highlights.push({ kind: token.kind, start: token.start, end: token.end });
  }
  const over = limit === undefined ? null : offsetAfterCodePoints(text, Math.max(limit, 0));
  if (over !== null) highlights.push({ kind: "over", start: over, end: text.length });
  return highlights;
}
