// SPDX-License-Identifier: AGPL-3.0-only
import { describe, expect, test } from "vitest";
import { domainToAscii, domainToUnicode } from "./idn";

describe("domainToUnicode", () => {
  test.each([
    ["xn--r8jz45g.jp", "例え.jp"],
    ["xn--mnchen-3ya.de", "münchen.de"],
    ["social.xn--bcher-kva.example", "social.bücher.example"],
    ["XN--mnchen-3ya.de", "münchen.de"],
    // RFC 3492 §7.1 (A) Arabic (Egyptian) and (L) Japanese with ASCII.
    ["xn--egbpdaj6bu4bxfgehfvwxn", "ليهمابتكلموشعربي؟"],
    ["xn--3B-ww4c5e180e575a65lsy2b", "3年B組金八先生"],
  ])("decodes %s to %s", (aLabel, uLabel) => {
    expect(domainToUnicode(aLabel)).toBe(uLabel);
  });

  test.each(["spam.example", "xn--", "xn--abc!.example", "a..b"])("keeps %s as it is", (domain) => {
    expect(domainToUnicode(domain)).toBe(domain);
  });
});

describe("domainToAscii", () => {
  test.each([
    ["例え.jp", "xn--r8jz45g.jp"],
    ["München.DE", "xn--mnchen-3ya.de"],
    ["Spam.Example", "spam.example"],
    ["xn--r8jz45g.jp", "xn--r8jz45g.jp"],
  ])("encodes %s to %s", (domain, aLabel) => {
    expect(domainToAscii(domain)).toBe(aLabel);
  });

  test.each([
    ["spam.example/path", "spam.example/path"],
    ["user@Spam.Example", "user@spam.example"],
    ["", ""],
  ])("keeps %s, which is no domain, as %s", (text, kept) => {
    expect(domainToAscii(text)).toBe(kept);
  });
});
