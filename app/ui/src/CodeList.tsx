// SPDX-License-Identifier: AGPL-3.0-only
import { tv } from "tailwind-variants";

const codeList = tv({
  base: "grid grid-cols-1 gap-2 rounded-lg border border-border bg-surface-raised px-4 py-3 font-mono @md:grid-cols-2",
});

export type CodeListProps = {
  /** The codes, each one a list item; they read left to right in every locale. */
  codes: readonly string[];
};

/** Codes to copy down, such as recovery codes: a raised box of monospaced list items, in two columns when it is wide. */
export function CodeList({ codes }: CodeListProps) {
  return (
    <ul dir="ltr" className={codeList()}>
      {codes.map((code) => (
        <li key={code}>{code}</li>
      ))}
    </ul>
  );
}
