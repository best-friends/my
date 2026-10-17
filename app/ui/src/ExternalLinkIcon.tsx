// SPDX-License-Identifier: AGPL-3.0-only

/**
 * A box with an arrow leaving it, after the text of a link to another site. Decorative: the link carries the
 * accessible name. The arrow points to the end of the line, so it flips in right-to-left text.
 */
export function ExternalLinkIcon() {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="size-4 shrink-0 rtl:-scale-x-100"
    >
      <path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" />
    </svg>
  );
}
