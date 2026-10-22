// SPDX-License-Identifier: AGPL-3.0-only
// A card that says one thing the product does: an icon tile, a heading and a sentence (design/components/LayoutLanding).
import type { ReactNode } from "react";
import { tv } from "tailwind-variants";
import { Card } from "./Card";
import type { HeadingLevel } from "./EmptyState";
import { Heading } from "./Heading";

const featureCard = tv({
  slots: {
    base: "flex flex-col items-start gap-3",
    icon: "grid size-12 place-items-center rounded-xl bg-surface-sunken text-accent [&>svg]:size-6",
    text: "text-sm text-fg-muted",
  },
});

export type FeatureCardProps = {
  /** A decorative icon, hidden from assistive technology; the heading carries the meaning. */
  icon: ReactNode;
  /** The feature's name. */
  heading: ReactNode;
  /** The heading's level: one below the heading of the section of cards. */
  headingLevel?: HeadingLevel;
  /** One or two sentences about it. */
  children: ReactNode;
};

/** A raised card for one feature; put several in a `Grid` of `li` (`Card as="li"` is this card's element). */
export function FeatureCard({ icon, heading, headingLevel = 3, children }: FeatureCardProps) {
  const slots = featureCard();
  return (
    <Card as="li">
      <div className={slots.base()}>
        <div aria-hidden="true" className={slots.icon()}>
          {icon}
        </div>
        <Heading level={headingLevel}>{heading}</Heading>
        <p className={slots.text()}>{children}</p>
      </div>
    </Card>
  );
}
