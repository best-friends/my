// SPDX-License-Identifier: AGPL-3.0-only
// The brand's decorative spark after a heading: mango, and hidden from assistive technology.

/** The marks the brand uses. */
export type SparkMark = "✳" | "✧";

/** A decorative spark, a little apart from the text before it. */
export function Spark({ mark }: { mark: SparkMark }) {
  return (
    <span aria-hidden="true" className="ms-1 text-focus">
      {mark}
    </span>
  );
}
