// SPDX-License-Identifier: AGPL-3.0-only
import { describe, expect, test } from "vitest";
import vectors from "../../../../pkg/testkit/fixtures/text/tokens.json";
import { activeWord, textTokens } from "./textTokens";

/** The length of `s` in UTF-8 bytes. */
function utf8Length(s: string): number {
  return new TextEncoder().encode(s).length;
}

// The vectors the server's ParseText and ScanShortcodes tests read too (ADR-0092 §4).
describe("textTokens follows the shared vectors", () => {
  test.each(vectors.cases.map((c) => [c.name, c] as const))("%s", (_, c) => {
    for (const token of c.tokens) {
      const [start = 0, end = 0] = token.utf16;
      const before = c.text.slice(0, start);
      expect(token.utf8).toEqual([utf8Length(before), utf8Length(before) + utf8Length(token.text)]);
      expect(c.text.slice(start, end)).toBe(token.text);
    }
    expect(textTokens(c.text)).toEqual(
      c.tokens.map((t) => ({ kind: t.kind, start: t.utf16[0], end: t.utf16[1], text: t.text })),
    );
  });
});

describe("textTokens", () => {
  test("leaves a shortcode inside a URL to the URL", () => {
    expect(textTokens("https://example.com/:blobcat:/x :wave:")).toEqual([
      { kind: "url", start: 0, end: 31, text: "https://example.com/:blobcat:/x" },
      { kind: "shortcode", start: 32, end: 38, text: ":wave:" },
    ]);
  });

  test("reads text with surrounding white space and carriage returns", () => {
    expect(textTokens(" \r\n#tag\r@bob ")).toEqual([
      { kind: "hashtag", start: 3, end: 7, text: "#tag" },
      { kind: "mention", start: 8, end: 12, text: "@bob" },
    ]);
  });
});

describe("activeWord", () => {
  test("finds a mention being typed, with or without a domain", () => {
    expect(activeWord("hi @ali", 7)).toEqual({ trigger: "@", start: 3, end: 7, query: "ali" });
    expect(activeWord("hi @alice@exa", 13)).toEqual({ trigger: "@", start: 3, end: 13, query: "alice@exa" });
    expect(activeWord("@", 1)).toEqual({ trigger: "@", start: 0, end: 1, query: "" });
  });

  test("reads the word up to the cursor only", () => {
    expect(activeWord("@alice and more", 4)).toEqual({ trigger: "@", start: 0, end: 4, query: "ali" });
    expect(activeWord("@alice and", 10)).toBeNull();
  });

  test("finds hashtags in any script and shortcodes being typed", () => {
    expect(activeWord("今日は #日本", 7)).toEqual({ trigger: "#", start: 4, end: 7, query: "日本" });
    expect(activeWord("so :blob", 8)).toEqual({ trigger: ":", start: 3, end: 8, query: "blob" });
  });

  test("does not start where the server would not read a token", () => {
    expect(activeWord("me@exa", 6)).toBeNull();
    expect(activeWord("a/@bob", 6)).toBeNull();
    expect(activeWord("C#sh", 4)).toBeNull();
    expect(activeWord("(a)#x", 5)).toBeNull();
    expect(activeWord("12:30", 5)).toBeNull();
    expect(activeWord(":blobcat:", 9)).toBeNull();
    expect(activeWord("＠alice ＃tag ：blob", 17)).toBeNull();
  });

  test("does not start inside a URL", () => {
    expect(activeWord("https://example.com/a?b#c", 25)).toBeNull();
    expect(activeWord("https://example.com/?q=1&@bob", 29)).toBeNull();
  });

  test("starts after a superscript digit, which the server does not count as a word character", () => {
    expect(activeWord("²#ta", 4)).toEqual({ trigger: "#", start: 1, end: 4, query: "ta" });
  });
});
