// SPDX-License-Identifier: AGPL-3.0-only
import { type ComponentPropsWithRef, useState } from "react";
import { useLocale } from "react-aria-components";
import { tv, type VariantProps } from "tailwind-variants";
import { type ImageSources, Picture } from "./Picture";
import { type AvatarTint, avatarTints } from "./tokens";

const avatar = tv({
  slots: {
    base: "inline-grid shrink-0 place-items-center overflow-hidden rounded-avatar bg-surface-sunken align-middle text-fg-muted select-none",
    image: "size-full object-cover",
    fallback: "grid size-full place-items-center font-serif leading-none font-bold",
    placeholder: "size-3/5",
  },
  variants: {
    size: {
      xs: { base: "size-6", fallback: "text-xs" },
      sm: { base: "size-8", fallback: "text-sm" },
      md: { base: "size-10", fallback: "text-base" },
      lg: { base: "size-12", fallback: "text-lg" },
      xl: { base: "size-20", fallback: "text-3xl" },
    },
  },
  defaultVariants: { size: "md" },
});

/** The tints, written out so Tailwind sees every class. */
const tints = {
  lilac: "bg-avatar-lilac text-avatar-lilac-fg",
  peach: "bg-avatar-peach text-avatar-peach-fg",
  mint: "bg-avatar-mint text-avatar-mint-fg",
  lavender: "bg-avatar-lavender text-avatar-lavender-fg",
  yellow: "bg-avatar-yellow text-avatar-yellow-fg",
} satisfies Record<AvatarTint, string>;

/** The tint of a seed: the same seed always gets the same tint (FNV-1a over its UTF-16 code units). */
function tintOf(seed: string): AvatarTint {
  let hash = 0x811c9dc5;
  for (let i = 0; i < seed.length; i++) hash = Math.imul(hash ^ seed.charCodeAt(i), 0x01000193) >>> 0;
  return avatarTints[hash % avatarTints.length] as AvatarTint;
}

/** The CSS width of each size, which picks the rendition of `sources`. */
const widths = { xs: "1.5rem", sm: "2rem", md: "2.5rem", lg: "3rem", xl: "5rem" } as const;

export type AvatarProps = Omit<ComponentPropsWithRef<"span">, "children"> &
  VariantProps<typeof avatar> & {
    /** The image URL. Without one, or when it fails to load, the avatar shows the first letter of `name`. */
    src?: string | undefined;
    /**
     * The renditions of the image, used instead of `src`: the browser loads the one that fits the avatar's size, and
     * an animation moves only as `Picture` lets it.
     */
    sources?: ImageSources | undefined;
    /** The account's display name: the accessible name of a standalone avatar and the source of the fallback letter. */
    name: string;
    /**
     * What picks the tint of the initial, such as the account's ID: the same seed always gets the same tint, so an
     * account keeps its color. Without one, the tint follows `name`.
     */
    seed?: string | undefined;
    /**
     * Whether the avatar is decorative. Pass `true` when the name is shown next to it (a post header, an account
     * row), so screen readers do not read the name twice: the image gets `alt=""`. Pass `false` for a standalone
     * avatar (a compact list, a button with no text), which is then an image named `name`.
     */
    decorative: boolean;
  };

/** The first user-perceived character of a name, upper-cased for the locale; empty for a blank name. */
function initialOf(name: string, locale: string): string {
  const [first] = new Intl.Segmenter(locale, { granularity: "grapheme" }).segment(name.trim());
  return first === undefined ? "" : first.segment.toLocaleUpperCase(locale);
}

/**
 * A rounded-square profile picture. When the image is missing or fails to load, it shows the first letter of
 * the name, or a generic silhouette when the name is blank. Whether it is announced is the caller's
 * explicit choice (`decorative`).
 */
export function Avatar({ src: plainSrc, sources, name, seed, decorative, size, className, ...props }: AvatarProps) {
  const src = sources?.still[0]?.url ?? plainSrc;
  const { locale } = useLocale();
  // The URL that failed, not a flag: a new src is tried again without resetting state in an Effect.
  const [failedSrc, setFailedSrc] = useState<string>();
  const { base, image, fallback, placeholder } = avatar({ size });
  const showImage = src !== undefined && src !== "" && src !== failedSrc;
  const initial = initialOf(name, locale);
  const tint = initial === "" ? "" : tints[tintOf(seed ?? name)];
  const label = decorative ? {} : { role: "img", "aria-label": name };

  return (
    <span {...props} className={base({ className })}>
      {showImage ? (
        sources === undefined ? (
          <img
            src={src}
            alt={decorative ? "" : name}
            className={image()}
            decoding="async"
            onError={() => {
              setFailedSrc(src);
            }}
          />
        ) : (
          <Picture
            sources={sources}
            sizes={widths[size ?? "md"]}
            alt={decorative ? "" : name}
            className={image()}
            decoding="async"
            onError={() => {
              setFailedSrc(src);
            }}
          />
        )
      ) : (
        <span {...label} aria-hidden={decorative || undefined} className={fallback({ className: tint })}>
          {initial === "" ? (
            <svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" className={placeholder()}>
              <circle cx="12" cy="8" r="4.5" />
              <path d="M3 21.5c0-4.7 4-8 9-8s9 3.3 9 8z" />
            </svg>
          ) : (
            initial
          )}
        </span>
      )}
    </span>
  );
}
