// SPDX-License-Identifier: AGPL-3.0-only
import { type ComponentPropsWithRef, useState } from "react";
import { ToggleButton } from "react-aria-components";
import { tv } from "tailwind-variants";
import { HideMediaIcon } from "./Icons";
import { VisuallyHidden } from "./VisuallyHidden";

const mediaGrid = tv({
  slots: {
    base: "relative grid aspect-video w-full gap-1 overflow-hidden rounded-lg",
    cell: "relative min-h-0 min-w-0 overflow-hidden bg-surface-sunken",
    image: "size-full object-cover",
    badge: "absolute inset-e-1.5 bottom-1.5 rounded-sm bg-surface-raised px-1.5 py-0.5 text-xs font-semibold text-fg shadow-sm",
    toggle: "absolute cursor-default rounded-md bg-surface-raised font-medium text-fg select-none data-focus-visible:focus-ring data-hovered:bg-surface-sunken motion-safe:transition-colors",
  },
  variants: {
    count: {
      1: {},
      2: { base: "grid-cols-2" },
      // The first item fills the start column; the other two stack in the end column.
      3: { base: "grid-cols-2 grid-rows-2 [&>*:first-child]:row-span-2" },
      4: { base: "grid-cols-2 grid-rows-2" },
    },
    revealed: {
      // Concealed: one button over the middle of the grid, so it is the obvious thing to press.
      false: { toggle: "inset-0 m-auto size-fit min-h-10 max-w-[calc(100%-1rem)] px-4 py-2 text-base shadow-md pointer-coarse:min-h-11" },
      // Shown: a small icon button in the corner that conceals the media again.
      true: { toggle: "inset-s-1.5 top-1.5 grid size-8 place-items-center shadow-sm pointer-coarse:size-11 [&>svg]:size-5" },
    },
  },
});

/** One picture of a post. */
export type MediaItem = {
  /** A stable ID of the attachment, used as the React key. */
  id: string;
  /** The image URL (the preview size). */
  src: string;
  /**
   * The author's description of the image, or `null` when there is none. It is required so that a missing
   * description is always a decision the caller made; a remote image without one gets the "no description" badge.
   */
  alt: string | null;
  /** The language of the description, when known. */
  lang?: string | undefined;
};

export type MediaGridProps = Omit<ComponentPropsWithRef<"div">, "children"> & {
  /** One to four pictures; items after the fourth are not shown. */
  items: readonly MediaItem[];
  /**
   * The text of the badge on a picture that has no description ("No description"). Screen readers read it
   * as the picture's name, so that the picture is not skipped silently.
   */
  noDescriptionLabel: string;
  /** Whether the author marked the media sensitive. Sensitive media are concealed until the reader shows them. */
  sensitive?: boolean;
  /**
   * The name of the toggle button that shows sensitive media ("Show sensitive media"). It stays the same in both
   * states, as a toggle button's name must: `aria-pressed` says whether the media are shown. While they are shown,
   * the button draws an eye icon instead of the text. Only used when `sensitive`.
   */
  revealLabel: string;
  /** Whether sensitive media are shown (controlled). */
  isRevealed?: boolean;
  /** Whether sensitive media are shown at first (uncontrolled). */
  defaultRevealed?: boolean;
  onRevealedChange?: (isRevealed: boolean) => void;
};

/**
 * The pictures of a post in a 16:9 grid of one to four cells. A picture without a description is named
 * by `noDescriptionLabel` and carries a visible badge saying so (web-client.md Y9). Sensitive media are
 * concealed behind a toggle button (`aria-pressed`): while concealed, the images are neither loaded nor
 * exposed, and the button keeps its place in the tab order and its focus when it is pressed.
 */
export function MediaGrid({
  items,
  noDescriptionLabel,
  sensitive = false,
  revealLabel,
  isRevealed,
  defaultRevealed = false,
  onRevealedChange,
  className,
  ...props
}: MediaGridProps) {
  const [uncontrolled, setUncontrolled] = useState(defaultRevealed);
  const revealed = !sensitive || (isRevealed ?? uncontrolled);
  const shown = items.slice(0, 4);
  const count = Math.min(4, Math.max(1, shown.length)) as 1 | 2 | 3 | 4;
  const slots = mediaGrid({ count, revealed });

  return (
    <div {...props} className={slots.base({ className })}>
      {shown.map((item) => (
        <div key={item.id} className={slots.cell()}>
          {revealed && (
            <>
              <img
                src={item.src}
                alt={item.alt ?? noDescriptionLabel}
                // The description is user content (web-client.md Y10); the fallback is the app's own text.
                {...(item.alt !== null && { dir: "auto", lang: item.lang })}
                loading="lazy"
                decoding="async"
                className={slots.image()}
              />
              {item.alt === null && (
                <span aria-hidden="true" className={slots.badge()}>
                  {noDescriptionLabel}
                </span>
              )}
            </>
          )}
        </div>
      ))}
      {sensitive && (
        <ToggleButton
          isSelected={revealed}
          onChange={(next) => {
            setUncontrolled(next);
            onRevealedChange?.(next);
          }}
          className={slots.toggle()}
        >
          {revealed ? (
            <>
              <HideMediaIcon />
              <VisuallyHidden>{revealLabel}</VisuallyHidden>
            </>
          ) : (
            revealLabel
          )}
        </ToggleButton>
      )}
    </div>
  );
}
