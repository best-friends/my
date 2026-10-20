// SPDX-License-Identifier: AGPL-3.0-only
// The size of Tailwind's spacing steps for the play functions of the stories: `size-5` is 5 × `--spacing`, which a server theme
// may override (ADR-0059), so a play function that checks a size asks the page for the step instead of
// assuming 0.25rem. Compare with toBeCloseTo: a fractional step can land on a sub-pixel.

/** The CSS px of `steps` spacing steps on this page; the step is a rem length, as theme.css and themes write it. */
export function spacing(steps: number): number {
  const root = getComputedStyle(document.documentElement);
  const step = root.getPropertyValue("--spacing").trim();
  if (!step.endsWith("rem")) throw new Error(`--spacing is ${step}, not a rem length`);
  return steps * Number.parseFloat(step) * Number.parseFloat(root.fontSize);
}
