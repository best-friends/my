// SPDX-License-Identifier: AGPL-3.0-only
// Internationalized domain names for display. The server keeps every domain in IDNA A-labels (`xn--r8jz45g.jp`),
// which people cannot read; the UI shows the U-labels they stand for (`例え.jp`) next to them. Browsers convert
// U-labels to A-labels (`new URL()`) but not back, so the Punycode decoder of RFC 3492 §6.2 is written here.
// Display only: the server, not this code, decides what a domain is.

const base = 36;
const tMin = 1;
const tMax = 26;
const skew = 38;
const damp = 700;
const initialBias = 72;
const initialN = 0x80;
const acePrefix = "xn--";

/** The bias adaptation of RFC 3492 §6.1. */
function adapt(delta: number, numPoints: number, firstTime: boolean): number {
  let scaled = firstTime ? Math.floor(delta / damp) : Math.floor(delta / 2);
  scaled += Math.floor(scaled / numPoints);
  let k = 0;
  while (scaled > ((base - tMin) * tMax) / 2) {
    scaled = Math.floor(scaled / (base - tMin));
    k += base;
  }
  return k + Math.floor(((base - tMin + 1) * scaled) / (scaled + skew));
}

/** The value of a basic code point as a Punycode digit (`a`–`z` / `A`–`Z` are 0–25, `0`–`9` are 26–35). */
function digitOf(codePoint: number): number | undefined {
  if (codePoint >= 0x30 && codePoint <= 0x39) return codePoint - 0x30 + 26;
  if (codePoint >= 0x41 && codePoint <= 0x5a) return codePoint - 0x41;
  if (codePoint >= 0x61 && codePoint <= 0x7a) return codePoint - 0x61;
  return undefined;
}

/** Decodes a Punycode string (the part of an A-label after `xn--`); `undefined` when it is not valid Punycode. */
function decodePunycode(input: string): string | undefined {
  const delimiter = input.lastIndexOf("-");
  const output: number[] = [];
  for (let index = 0; index < Math.max(delimiter, 0); index++) {
    const codePoint = input.charCodeAt(index);
    if (codePoint >= initialN) return undefined;
    output.push(codePoint);
  }
  let n = initialN;
  let i = 0;
  let bias = initialBias;
  let index = delimiter > 0 ? delimiter + 1 : 0;
  while (index < input.length) {
    const oldI = i;
    let weight = 1;
    for (let k = base; ; k += base) {
      if (index >= input.length) return undefined;
      const digit = digitOf(input.charCodeAt(index++));
      if (digit === undefined) return undefined;
      i += digit * weight;
      if (i > Number.MAX_SAFE_INTEGER) return undefined;
      const threshold = k <= bias ? tMin : k >= bias + tMax ? tMax : k - bias;
      if (digit < threshold) break;
      weight *= base - threshold;
    }
    const length = output.length + 1;
    bias = adapt(i - oldI, length, oldI === 0);
    n += Math.floor(i / length);
    i %= length;
    if (n > 0x10ffff || (n >= 0xd800 && n <= 0xdfff)) return undefined;
    output.splice(i, 0, n);
    i++;
  }
  return String.fromCodePoint(...output);
}

function isAscii(text: string): boolean {
  for (let index = 0; index < text.length; index++) {
    if (text.charCodeAt(index) >= initialN) return false;
  }
  return true;
}

/**
 * The domain with its A-labels decoded to U-labels (`xn--r8jz45g.jp` → `例え.jp`). A label that is not valid
 * Punycode stays as it is, so a malformed domain is shown as the server sent it.
 */
export function domainToUnicode(domain: string): string {
  return domain
    .split(".")
    .map((label) => {
      if (!label.toLowerCase().startsWith(acePrefix)) return label;
      const decoded = decodePunycode(label.slice(acePrefix.length));
      // An A-label always stands for some non-ASCII code point (RFC 5890 §2.3.2.1).
      return decoded === undefined || isAscii(decoded) ? label : decoded;
    })
    .join(".");
}
