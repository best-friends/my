// SPDX-License-Identifier: AGPL-3.0-only
import type { ReactNode } from "react";
import { tv } from "tailwind-variants";
import { Link } from "./Link";
import { Spark, type SparkMark } from "./Spark";

const pageHeading = tv({
  variants: {
    // The brand's page-title: the serif is for brand moments, and this is the first of them.
    visuallyHidden: { true: "sr-only", false: "font-serif text-3xl font-medium tracking-tight" },
  },
});

export type PageHeadingProps = {
  children: ReactNode;
  /** For a region named by the page heading (a feed with `aria-labelledby`). */
  id?: string;
  /**
   * For assistive technology only: on a page whose heading only says where the user is, which the navigation already
   * shows (a timeline, the notifications), it is visually hidden but stays in the accessibility tree (web-client.md Y5).
   */
  visuallyHidden?: boolean;
  /** A decorative mark (`✳` or `✧`, mango) after the text, hidden from assistive technology. */
  spark?: SparkMark;
};

/**
 * The page's only level-one heading. After a navigation the app moves focus to it and names the document after its
 * text, so it is focusable by script (`tabIndex={-1}`) but not in the tab order.
 */
export function PageHeading({ children, id, visuallyHidden = false, spark }: PageHeadingProps) {
  return (
    <h1 id={id} tabIndex={-1} className={pageHeading({ visuallyHidden })}>
      {children}
      {spark !== undefined && !visuallyHidden && <Spark mark={spark} />}
    </h1>
  );
}

const backLinkHeading = tv({
  slots: {
    base: "flex flex-col items-start gap-1",
    link: "text-sm",
  },
});

export type BackLinkHeadingProps = {
  /** The page's heading (`PageHeading`). */
  children: ReactNode;
  /** Where the way back leads (the settings). */
  backHref: string;
  /** The way back's text ("Back to settings"). */
  backLabel: string;
};

/** A page's heading with the way back to the page above it under it, such as a page of the settings. */
export function BackLinkHeading({ children, backHref, backLabel }: BackLinkHeadingProps) {
  const slots = backLinkHeading();
  return (
    <div className={slots.base()}>
      {children}
      <Link href={backHref} variant="standalone" className={slots.link()}>
        {backLabel}
      </Link>
    </div>
  );
}
