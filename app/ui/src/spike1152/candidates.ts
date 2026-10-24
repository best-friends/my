// SPDX-License-Identifier: AGPL-3.0-only
// Spike #1152: fixed candidates for `@`, `#` and `:` (production asks the API through features/compose).
import type { ActiveWord } from "./textTokens";

export interface Candidate {
  id: string;
  /** What replaces the word: `@alice`, `#日本語`, `:blobcat:`. */
  value: string;
  label: string;
}

const data = {
  "@": ["alice", "alicia", "bob", "carol@example.org", "dave"],
  "#": ["日本語", "日本", "golang", "go", "goodnight"],
  ":": ["blobcat", "blush", "blobfox", "thinking"],
} as const;

export function fixedCandidates(word: ActiveWord): Candidate[] {
  const q = word.query.toLowerCase();
  return data[word.trigger]
    .filter((name) => name.toLowerCase().startsWith(q))
    .map((name) => {
      const value = word.trigger === ":" ? `:${name}:` : `${word.trigger}${name}`;
      return { id: `${word.trigger}${name}`, value, label: value };
    });
}
