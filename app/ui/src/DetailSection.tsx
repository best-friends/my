// SPDX-License-Identifier: AGPL-3.0-only
// A titled region of a detail page below its `h1`: an `h2` that names a `<section>` on a card.
import { type ReactNode, type RefObject, useId } from "react";
import { Card } from "./Card";
import { Heading } from "./Heading";
import { Stack } from "./Stack";

export type DetailSectionProps = {
  /** The title: the `h2` that names the section. */
  title: ReactNode;
  /** With a ref, the heading takes focus from script (`tabindex="-1"`) when the region changes under it. */
  headingRef?: RefObject<HTMLHeadingElement | null>;
  children: ReactNode;
};

/** A `<section>` named by its `h2`, with the space of a region's content between them. */
export function DetailSection({ title, headingRef, children }: DetailSectionProps) {
  const headingId = useId();
  return (
    <Card as="section" padding="6" aria-labelledby={headingId}>
      <Stack gap="3">
        <Heading level={2} ref={headingRef} id={headingId} focusable={headingRef !== undefined}>
          {title}
        </Heading>
        {children}
      </Stack>
    </Card>
  );
}
