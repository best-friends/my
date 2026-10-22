// SPDX-License-Identifier: AGPL-3.0-only
import type { ComponentPropsWithRef } from "react";
import { tv, type VariantProps } from "tailwind-variants";
import darkBackground from "./logos/Wordmark-DarkBg.svg";
import monochrome from "./logos/Wordmark-Monochrome.svg";
import primaryColor from "./logos/Wordmark-Primary-Color.svg";

const wordmark = tv({
  slots: {
    base: "inline-block shrink-0 align-middle leading-none",
    image: "h-full w-auto",
    dark: "hidden dark:block",
    light: "block dark:hidden",
  },
  variants: {
    // The artwork is 1250 x 700, so the height sets the width.
    size: {
      sm: { base: "h-8" },
      md: { base: "h-12" },
      lg: { base: "h-16" },
    },
  },
  defaultVariants: { size: "md" },
});

export type WordmarkProps = Omit<ComponentPropsWithRef<"span">, "children"> &
  VariantProps<typeof wordmark> & {
    /** The accessible name of a standalone wordmark; the product name ("my"). Ignored when `decorative`. */
    name: string;
    /**
     * Whether the wordmark is decorative. Pass `true` when the server name or the product name is shown as text
     * next to it. Pass `false` for a standalone logo, which is then an image named `name`.
     */
    decorative: boolean;
    /**
     * The artwork. `adaptive` (default) is the coral wordmark on a light ground and the white one on a dark ground,
     * following the theme. `monochrome` is the single charcoal ink, for a ground that is neither.
     */
    tone?: "adaptive" | "monochrome";
  };

/**
 * The hand-lettered "my" with its three-stroke spark, drawn as given in `assets/Logos` (never redrawn or recolored:
 * the inks are fixed in the files, so the variant that matches the ground is picked here).
 */
export function Wordmark({ name, decorative, size, tone = "adaptive", className, ...props }: WordmarkProps) {
  const { base, image, dark, light } = wordmark({ size });
  const alt = decorative ? "" : name;
  return (
    <span {...props} className={base({ className })}>
      {tone === "monochrome" ? (
        <img src={monochrome} alt={alt} className={image()} />
      ) : (
        <>
          <img src={primaryColor} alt={alt} className={image({ className: light() })} />
          <img src={darkBackground} alt={alt} className={image({ className: dark() })} />
        </>
      )}
    </span>
  );
}
