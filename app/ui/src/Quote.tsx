// SPDX-License-Identifier: AGPL-3.0-only
// A quoted piece of someone else's text: the rule at the start and the clamped excerpt of a post. A notification quotes
// the post it is about, a moderation notice quotes the moderators' words, and the form quotes the post being replied to.
import type { ReactNode } from "react";
import { Link } from "./Link";
import { Stack } from "./Stack";
import { Text } from "./Text";

export type QuoteBlockProps = {
  /** Lines of text (`Text as="p"`): what the quote says. */
  children: ReactNode;
};

/** A block of lines with a rule at the start (`border-s-2`), one line above another. */
export function QuoteBlock({ children }: QuoteBlockProps) {
  return (
    <Stack gap="1" border="start">
      {children}
    </Stack>
  );
}

export type PostExcerptProps = {
  /** The post's page in the app. */
  href: string;
  /** The excerpt: a `PostExcerptText`, or a sentence for a post that has no text to quote. */
  children: ReactNode;
};

/** The excerpt of a post as a quote that links to the post, in the color of the text and underlined on hover. */
export function PostExcerpt({ href, children }: PostExcerptProps) {
  return (
    <Link href={href} className="block border-s-2 border-border ps-3 wrap-anywhere text-fg no-underline data-hovered:underline">
      {children}
    </Link>
  );
}

export type PostExcerptTextProps = {
  /** The post's language (BCP 47), for hyphenation and the right glyphs. */
  lang?: string | undefined;
  /** The start of the post's text, plain. */
  children: ReactNode;
};

/** The start of a post's text: its own direction, line breaks kept, cut after three lines. */
export function PostExcerptText({ lang, children }: PostExcerptTextProps) {
  return (
    <Text dir="auto" {...(lang !== undefined && { lang })} lineClamp={3} whitespace="preLine">
      {children}
    </Text>
  );
}
