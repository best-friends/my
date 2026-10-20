// SPDX-License-Identifier: AGPL-3.0-only
// The image viewer of a post's pictures (a lightbox): the shared modal Dialog (WAI-ARIA APG, dialog pattern), so it
// traps focus, closes on Esc, by its × button and by a press on the scrim, and returns focus to the picture that
// opened it. Inside, one picture at its original size, scaled down to fit, with its description, and the buttons
// and the arrow keys that step to the picture before and after it.
import { useEffect, useEffectEvent } from "react";
import { useLocale } from "react-aria-components";
import { tv } from "tailwind-variants";
import { Button } from "./Button";
import { Dialog } from "./Dialog";
import { NextIcon, PreviousIcon } from "./Icons";
import { type ImageSources, Picture } from "./Picture";

const mediaViewer = tv({
  slots: {
    // Wider than any other dialog: a picture uses the room of the screen, up to its own size.
    panel: "max-w-[min(100%,80rem)]",
    body: "flex flex-col items-center gap-3",
    figure: "m-0 flex max-w-full flex-col items-center gap-2",
    // At most its own size, and scaled down to the panel's width and to the height the screen leaves it.
    image: "block size-auto max-h-[calc(85svh-9rem)] max-w-full rounded-md object-contain",
    caption: "max-w-prose text-sm align-match-parent whitespace-pre-line text-fg-muted",
    nav: "flex items-center gap-4",
  },
});

/** One picture of the viewer. */
type MediaViewerItem = {
  id: string;
  /** The renditions of the picture at its original size (still, and moving for an animation). */
  sources: ImageSources;
  /** The author's description, or `null` when there is none. */
  alt: string | null;
  lang?: string | undefined;
  /** The size of the original picture, when known, so that the layout does not move while it loads. */
  width?: number | undefined;
  height?: number | undefined;
};

/** The texts of the viewer, which the app translates. */
export type MediaViewerLabels = {
  /** The heading that names the dialog, for the picture at `position` (1-based) of `count`: "Image 2 of 3". */
  title: (position: number, count: number) => string;
  /** The name of the × button. */
  close: string;
  /** The names of the buttons that show the picture before and after. */
  previous: string;
  next: string;
};

export type MediaViewerProps = {
  items: readonly MediaViewerItem[];
  /** The picture shown; it stays the last one while the viewer closes, so the closing panel keeps its picture. */
  index: number;
  isOpen: boolean;
  onIndexChange: (index: number) => void;
  onOpenChange: (isOpen: boolean) => void;
  labels: MediaViewerLabels;
  /** The name of a picture without a description. */
  noDescriptionLabel: string;
};

type ViewerContentProps = Pick<MediaViewerProps, "items" | "index" | "onIndexChange" | "labels" | "noDescriptionLabel">;

/**
 * The picture `by` places after `index` (before it, when negative), going round from the last picture to the first and
 * back. The buttons stay enabled at either end, so that the focus never sits on a button that the step disabled.
 */
function step(index: number, by: 1 | -1, count: number): number {
  return (index + by + count) % count;
}

/**
 * The picture and its controls, mounted only while the dialog is open, so that its listener for the arrow keys
 * lives as long as the dialog. The dialog itself, and not only one of its controls, has the focus at first, so the
 * keys are heard on the window; the modal dialog keeps the rest of the page out of reach meanwhile.
 */
function ViewerContent({ items, index, onIndexChange, labels, noDescriptionLabel }: ViewerContentProps) {
  const { direction } = useLocale();
  const styles = mediaViewer();
  const item = items[index];
  const count = items.length;

  // Left and right follow the screen: in right-to-left text the picture before is on the right (as its button is).
  const onKeyDown = useEffectEvent((event: KeyboardEvent) => {
    if (count < 2 || event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
    const forward = (event.key === "ArrowRight") === (direction === "ltr");
    event.preventDefault();
    onIndexChange(step(index, forward ? 1 : -1, count));
  });
  useEffect(() => {
    const listener = (event: KeyboardEvent) => {
      onKeyDown(event);
    };
    window.addEventListener("keydown", listener);
    return () => {
      window.removeEventListener("keydown", listener);
    };
  }, []);

  if (item === undefined) return null;
  return (
    <div className={styles.body()}>
      {/* A new picture is announced by its name, as a carousel announces its slide (WAI-ARIA APG). */}
      <figure aria-live="polite" className={styles.figure()}>
        <Picture
          // A new picture is a new image: the old one does not stay on screen while the next one loads.
          key={item.id}
          sources={item.sources}
          alt={item.alt ?? noDescriptionLabel}
          {...(item.alt !== null && { dir: "auto", lang: item.lang })}
          {...(item.width !== undefined && item.height !== undefined && { width: item.width, height: item.height })}
          decoding="async"
          className={styles.image()}
        />
        {/* The picture's name already says it to screen readers; this shows it to everyone else. */}
        <figcaption aria-hidden="true" className={styles.caption()} {...(item.alt !== null && { dir: "auto", lang: item.lang })}>
          {item.alt ?? noDescriptionLabel}
        </figcaption>
      </figure>
      {count > 1 && (
        <div className={styles.nav()}>
          <Button
            variant="ghost"
            aria-label={labels.previous}
            onPress={() => {
              onIndexChange(step(index, -1, count));
            }}
          >
            <PreviousIcon />
          </Button>
          <Button
            variant="ghost"
            aria-label={labels.next}
            onPress={() => {
              onIndexChange(step(index, 1, count));
            }}
          >
            <NextIcon />
          </Button>
        </div>
      )}
    </div>
  );
}

/** A modal viewer of pictures at their original size (controlled: `index`, `isOpen`). */
export function MediaViewer({ items, index, isOpen, onIndexChange, onOpenChange, labels, noDescriptionLabel }: MediaViewerProps) {
  const styles = mediaViewer();
  return (
    <Dialog
      title={labels.title(index + 1, items.length)}
      closeLabel={labels.close}
      size="lg"
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      className={styles.panel()}
    >
      <ViewerContent items={items} index={index} onIndexChange={onIndexChange} labels={labels} noDescriptionLabel={noDescriptionLabel} />
    </Dialog>
  );
}
