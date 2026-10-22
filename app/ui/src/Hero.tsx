// SPDX-License-Identifier: AGPL-3.0-only
// The opening panel of a signed-out page: a cream wash, an eyebrow, the page heading, a lead and the ways in
// (design/components/LayoutLanding).
import type { ReactNode } from "react";
import { tv } from "tailwind-variants";
import { Text } from "./Text";

const hero = tv({
  slots: {
    base: "@container flex flex-col gap-6 rounded-2xl bg-linear-to-b from-surface-sunken to-surface-raised p-6 @md:p-10",
    body: "flex flex-col items-start gap-4",
    lead: "max-w-[60ch] text-lg text-fg",
    actions: "flex flex-wrap items-center gap-3",
    notes: "flex flex-col gap-2",
    media: "w-full overflow-hidden rounded-xl",
  },
});

export type HeroProps = {
  /** The small line above the heading: uppercase Latin letters ("Welcome to my"). */
  eyebrow?: ReactNode;
  /** The page's heading (`PageHeading`), the only level-one heading. */
  heading: ReactNode;
  /** The lead under the heading, one paragraph (`Text`). */
  children?: ReactNode;
  /** The ways in: one primary `Button` or `ButtonLink` and the others beside it. */
  actions?: ReactNode;
  /** What the actions need said (a `Hint`), under them. */
  note?: ReactNode;
  /** An image that stands for the server (`PreviewImage`), under the panel's text. */
  media?: ReactNode;
};

/** A cream-to-raised panel with the page's heading and the ways in. Lay it at the top of the page, once. */
export function Hero({ eyebrow, heading, children, actions, note, media }: HeroProps) {
  const slots = hero();
  return (
    <section className={slots.base()}>
      <div className={slots.body()}>
        {eyebrow !== undefined && (
          <Text as="p" eyebrow>
            {eyebrow}
          </Text>
        )}
        {heading}
        {children !== undefined && <div className={slots.lead()}>{children}</div>}
        {actions !== undefined && <div className={slots.actions()}>{actions}</div>}
        {note !== undefined && <div className={slots.notes()}>{note}</div>}
      </div>
      {media !== undefined && <div className={slots.media()}>{media}</div>}
    </section>
  );
}
