// SPDX-License-Identifier: AGPL-3.0-only
import { type ComponentPropsWithRef, createContext, use, useState } from "react";

/** One rendition of an image: its URL and, when known, its width in pixels. */
export type ImageCandidate = { url: string; width?: number | undefined };

/**
 * The renditions of an image to choose from: the still ones, and for an animation the moving ones (ADR-0054 §3).
 * The browser picks one by the width it shows the image at (`srcset` and `sizes`).
 */
export type ImageSources = {
  still: readonly ImageCandidate[];
  animated?: readonly ImageCandidate[] | undefined;
};

/**
 * Whether animations play by themselves: the user's "play animations automatically" setting, which the app provides
 * at its root. Off, an animation shows its still rendition until it is hovered or tapped. On, it still stays still
 * while the system asks for reduced motion (`prefers-reduced-motion: reduce`).
 */
export const AutoplayContext = createContext(true);

/** The `src`, `srcset` and `sizes` of a set of renditions: a `srcset` only when every one has a known width. */
function sourceAttributes(candidates: readonly ImageCandidate[], sizes: string | undefined): { src: string; srcSet?: string; sizes?: string } {
  const unique = candidates.filter((candidate, i) => candidates.findIndex((other) => other.url === candidate.url) === i);
  const src = unique[0]?.url ?? "";
  if (unique.length < 2 || sizes === undefined || unique.some((candidate) => candidate.width === undefined)) return { src };
  return { src, srcSet: unique.map((candidate) => `${candidate.url} ${String(candidate.width)}w`).join(", "), sizes };
}

export type PictureProps = Omit<ComponentPropsWithRef<"img">, "src" | "srcSet" | "sizes"> & {
  sources: ImageSources;
  /** The width the image is shown at, as the `sizes` attribute; without it, the first rendition is loaded. */
  sizes?: string | undefined;
};

/**
 * An image in the rendition that fits: a plain `<img>` for a still image, and for an animation a `<picture>` whose
 * moving `<source>` applies only without a reduced-motion preference. When animations do not play by themselves
 * (`AutoplayContext`), the still rendition is shown and the moving one replaces it while the pointer is over it or a
 * finger is on it.
 */
export function Picture({ sources, sizes, onPointerEnter, onPointerLeave, ...props }: PictureProps) {
  const autoplay = use(AutoplayContext);
  const [playing, setPlaying] = useState(false);
  const still = sourceAttributes(sources.still, sizes);
  const animated = sources.animated;
  if (animated === undefined || animated.length === 0) return <img {...props} {...still} />;
  const moving = sourceAttributes(animated, sizes);
  if (autoplay) {
    return (
      <picture>
        <source media="(prefers-reduced-motion: no-preference)" srcSet={moving.srcSet ?? moving.src} {...(moving.sizes === undefined ? {} : { sizes: moving.sizes })} />
        <img {...props} {...still} onPointerEnter={onPointerEnter} onPointerLeave={onPointerLeave} />
      </picture>
    );
  }
  return (
    <img
      {...props}
      {...(playing ? moving : still)}
      onPointerEnter={(event) => {
        setPlaying(true);
        onPointerEnter?.(event);
      }}
      onPointerLeave={(event) => {
        setPlaying(false);
        onPointerLeave?.(event);
      }}
    />
  );
}
