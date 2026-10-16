// SPDX-License-Identifier: AGPL-3.0-only
import { type ComponentPropsWithRef, useState } from "react";
import { useLocale } from "react-aria-components";
import { tv, type VariantProps } from "tailwind-variants";

const avatar = tv({
  slots: {
    base: "inline-grid shrink-0 place-items-center overflow-hidden rounded-full bg-surface-sunken align-middle text-fg-muted select-none",
    image: "size-full object-cover",
    fallback: "grid size-full place-items-center leading-none font-semibold",
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

export type AvatarProps = Omit<ComponentPropsWithRef<"span">, "children"> &
  VariantProps<typeof avatar> & {
    /** The image URL. Without one, or when it fails to load, the avatar shows the first letter of `name`. */
    src?: string | undefined;
    /** The account's display name: the accessible name of a standalone avatar and the source of the fallback letter. */
    name: string;
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
 * A round profile picture. When the image is missing or fails to load, it shows the first letter of
 * the name, or a generic silhouette when the name is blank. Whether it is announced is the caller's
 * explicit choice (`decorative`).
 */
export function Avatar({ src, name, decorative, size, className, ...props }: AvatarProps) {
  const { locale } = useLocale();
  // The URL that failed, not a flag: a new src is tried again without resetting state in an Effect.
  const [failedSrc, setFailedSrc] = useState<string>();
  const { base, image, fallback, placeholder } = avatar({ size });
  const showImage = src !== undefined && src !== "" && src !== failedSrc;
  const initial = initialOf(name, locale);
  const label = decorative ? {} : { role: "img", "aria-label": name };

  return (
    <span {...props} className={base({ className })}>
      {showImage ? (
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
        <span {...label} aria-hidden={decorative || undefined} className={fallback()}>
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
