// SPDX-License-Identifier: AGPL-3.0-only

/** A text after an insertion, and where the caret goes: after what was inserted. */
export type Insertion = { value: string; caret: number };

/**
 * Puts `text` (a Unicode emoji) in place of the selection from `start` to `end` of `value`, without spaces around it:
 * an emoji is read as itself wherever it stands (ADR-0091 §5).
 */
export function insertText(value: string, start: number, end: number, text: string): Insertion {
  const before = value.slice(0, start);
  return { value: before + text + value.slice(end), caret: before.length + text.length };
}

/** Whether a character separates words for a shortcode: the server finds `:shortcode:` only between such characters. */
function isSpace(character: string | undefined): boolean {
  return character === undefined || /\s/u.test(character);
}

/**
 * Puts `:shortcode:` in place of the selection from `start` to `end` of `value`. The server reads a shortcode only
 * when no letter, digit or colon touches it (ADR-0051 §3), so a space goes before it after other text, and after it
 * unless a space follows already, so that typing goes on as a new word.
 */
export function insertShortcode(value: string, start: number, end: number, shortcode: string): Insertion {
  const before = value.slice(0, start);
  const after = value.slice(end);
  const inserted = `${before === "" || isSpace(before.at(-1)) ? "" : " "}:${shortcode}:${isSpace(after[0]) && after !== "" ? "" : " "}`;
  return { value: before + inserted + after, caret: before.length + inserted.length };
}
