// SPDX-License-Identifier: AGPL-3.0-only
import { type ComponentPropsWithRef, use, useState, useSyncExternalStore } from "react";
import { ToggleButton } from "react-aria-components";
import { tv } from "tailwind-variants";
import { Blurhash } from "./Blurhash";
import { HideMediaIcon } from "./Icons";
import { AutoplayContext, type ImageSources, Picture } from "./Picture";
import { VisuallyHidden } from "./VisuallyHidden";

const mediaGrid = tv({
  slots: {
    base: "relative grid aspect-video w-full gap-1 overflow-hidden rounded-lg",
    cell: "relative min-h-0 min-w-0 overflow-hidden bg-surface-sunken",
    // Above the blur, which is positioned: the image covers it as it loads.
    image: "relative size-full object-cover",
    // A video keeps its whole frame in the cell, letterboxed on the cell's surface.
    video: "relative size-full object-contain",
    audio: "absolute inset-x-2 bottom-2 w-[calc(100%-1rem)]",
    blur: "absolute inset-0",
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

/**
 * A video or audio file that a cell plays (ADR-0054 §6). A `video` and an `audio` file never play by themselves and have
 * controls; a `gifv` (a short silent video in place of an animated image) loops muted without controls while animations
 * play by themselves (`AutoplayContext`) and the system does not ask for reduced motion, and has controls otherwise.
 */
export type MediaPlayback = { kind: "video" | "audio" | "gifv"; url: string };

/** One picture, video, or audio file of a post. */
export type MediaItem = {
  /** A stable ID of the attachment, used as the React key. */
  id: string;
  /** The image URL (the preview size); for a video or audio file, its poster, or undefined without one. */
  src: string | undefined;
  /** The video or audio file to play in the cell, with `src` as its poster; undefined for a picture. */
  playback?: MediaPlayback | undefined;
  /** The renditions of the image, used instead of `src`: the one that fits the cell, still unless `Picture` lets it move. */
  sources?: ImageSources | undefined;
  /**
   * The author's description of the image, or `null` when there is none. It is required so that a missing
   * description is always a decision the caller made; a remote image without one gets the "no description" badge.
   */
  alt: string | null;
  /** The language of the description, when known. */
  lang?: string | undefined;
  /**
   * The blurhash of the image, drawn while it is concealed or loading. Without one, the cell's plain surface
   * stands in for it.
   */
  blurhash?: string | undefined;
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

type MediaCellProps = { item: MediaItem; revealed: boolean; count: 1 | 2 | 3 | 4; noDescriptionLabel: string };

const reducedMotionQuery = "(prefers-reduced-motion: reduce)";

function subscribeReducedMotion(onChange: () => void): () => void {
  if (typeof window.matchMedia !== "function") return () => undefined;
  const query = window.matchMedia(reducedMotionQuery);
  query.addEventListener("change", onChange);
  return () => {
    query.removeEventListener("change", onChange);
  };
}

/** Whether the system asks for reduced motion, rendering again when that changes. */
function usePrefersReducedMotion(): boolean {
  return useSyncExternalStore(
    subscribeReducedMotion,
    () => typeof window.matchMedia === "function" && window.matchMedia(reducedMotionQuery).matches,
    () => false,
  );
}

/**
 * A cell that plays a video or audio file once shown: the blur of its poster while it is concealed or loading, then a
 * `<video>` with its poster, or the poster with an `<audio>` over its bottom edge. The player is named by the
 * description, or by `noDescriptionLabel` with the badge when there is none.
 */
function PlaybackCell({ item, playback, revealed, count, noDescriptionLabel }: MediaCellProps & { playback: MediaPlayback }) {
  const [loaded, setLoaded] = useState(false);
  const autoplay = use(AutoplayContext);
  const reducedMotion = usePrefersReducedMotion();
  const slots = mediaGrid({ count, revealed });
  const name = item.alt ?? noDescriptionLabel;
  const loops = playback.kind === "gifv";
  const playsByItself = loops && autoplay && !reducedMotion;
  const showsPoster = playback.kind === "audio" && item.src !== undefined;
  return (
    <div className={slots.cell()}>
      {item.blurhash !== undefined && !(revealed && (loaded || playback.kind === "audio")) && <Blurhash hash={item.blurhash} className={slots.blur()} />}
      {revealed && (
        <>
          {playback.kind === "audio" ? (
            <>
              {showsPoster && (
                <Picture
                  sources={item.sources ?? { still: [{ url: item.src ?? "" }] }}
                  sizes={count === 1 ? "(min-width: 40rem) 40rem, 100vw" : "(min-width: 40rem) 20rem, 50vw"}
                  // The audio player carries the description; the poster adds nothing for a screen reader.
                  alt=""
                  loading="lazy"
                  decoding="async"
                  className={slots.image()}
                />
              )}
              <audio src={playback.url} controls preload="none" aria-label={name} className={slots.audio()} />
            </>
          ) : (
            <video
              // A new source or a change of autoplay starts the element again.
              key={playsByItself ? "loop" : "still"}
              src={playback.url}
              poster={item.src}
              controls={!playsByItself}
              autoPlay={playsByItself}
              muted={loops}
              loop={loops}
              playsInline
              preload={playsByItself ? "auto" : "metadata"}
              aria-label={name}
              onLoadedData={() => {
                setLoaded(true);
              }}
              className={slots.video()}
            />
          )}
          {item.alt === null && (
            <span aria-hidden="true" className={slots.badge()}>
              {noDescriptionLabel}
            </span>
          )}
        </>
      )}
    </div>
  );
}

/**
 * One cell of the grid: the blur of the picture (its blurhash) while it is concealed or loading, and once shown,
 * the picture over it. The blur goes once the picture has loaded, so that a transparent picture shows the surface
 * behind it, and stays should the picture fail to load.
 */
function MediaCell({ item, revealed, count, noDescriptionLabel }: MediaCellProps) {
  const [loaded, setLoaded] = useState(false);
  const slots = mediaGrid({ count, revealed });
  return (
    <div className={slots.cell()}>
      {item.blurhash !== undefined && !(revealed && loaded) && <Blurhash hash={item.blurhash} className={slots.blur()} />}
      {revealed && (
        <>
          <Picture
            sources={item.sources ?? { still: [{ url: item.src ?? "" }] }}
            // A post is at most about 40rem wide; two or more pictures share it in two columns.
            sizes={count === 1 ? "(min-width: 40rem) 40rem, 100vw" : "(min-width: 40rem) 20rem, 50vw"}
            alt={item.alt ?? noDescriptionLabel}
            // The description is user content (web-client.md Y10); the fallback is the app's own text.
            {...(item.alt !== null && { dir: "auto", lang: item.lang })}
            loading="lazy"
            decoding="async"
            onLoad={() => {
              setLoaded(true);
            }}
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
  );
}

/**
 * The pictures, videos, and audio files of a post in a 16:9 grid of one to four cells. A video or audio file plays in
 * its cell with the browser's controls and never starts by itself (`MediaPlayback`). An item without a description is
 * named by `noDescriptionLabel` and carries a visible badge saying so (web-client.md Y9). Sensitive media are
 * concealed behind a toggle button (`aria-pressed`): while concealed, the images are neither loaded nor
 * exposed and each cell shows the blur of its picture, and the button keeps its place in the tab order and its focus when it is pressed.
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
      {shown.map((item) =>
        item.playback === undefined ? (
          <MediaCell key={item.id} item={item} revealed={revealed} count={count} noDescriptionLabel={noDescriptionLabel} />
        ) : (
          <PlaybackCell key={item.id} item={item} playback={item.playback} revealed={revealed} count={count} noDescriptionLabel={noDescriptionLabel} />
        ),
      )}
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
