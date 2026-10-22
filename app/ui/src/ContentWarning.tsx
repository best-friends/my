// SPDX-License-Identifier: AGPL-3.0-only
// A content warning as a WAI-ARIA disclosure (web-client.md Y8): a button with aria-expanded and
// aria-controls, and a panel that is `hidden` while closed, so nothing in it is read or focusable.
//
// React Aria's DisclosurePanel is not used: it hides the panel with hidden="until-found", which lets the
// browser's find-in-page open the panel. Behind a content warning that would show the content to someone
// who only searched the page, so the panel here is plain `hidden`.
import { type ComponentPropsWithRef, type ReactNode, useId, useState } from "react";
import { tv } from "tailwind-variants";
import { Button } from "./Button";
import { WarningIcon } from "./Icons";

const contentWarning = tv({
  slots: {
    base: "flex flex-col gap-2",
    summary: "flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg bg-warning-bg py-2 ps-3 pe-3",
    icon: "flex size-5 shrink-0 items-center justify-center text-warning [&>svg]:size-5",
    text: "min-w-0 flex-1 font-bold wrap-anywhere text-fg",
    chevron: "size-4 shrink-0 fill-none stroke-current stroke-2 motion-safe:transition-transform",
    panel: "flex flex-col gap-2",
  },
  variants: {
    expanded: { true: { chevron: "rotate-180" } },
  },
});

export type ContentWarningProps = Omit<ComponentPropsWithRef<"div">, "children"> & {
  /** The author's warning text, shown while the content is hidden. It is user content, so it is drawn with `dir="auto"`. */
  summary: ReactNode;
  /** The language of the warning text, when known. */
  summaryLang?: string | undefined;
  /**
   * The button's text ("Show content"). It stays the same while the panel opens and closes: the button's
   * `aria-expanded` says whether the content is shown, and the chevron shows it.
   */
  toggleLabel: string;
  /** The content behind the warning: the post body, its media. */
  children: ReactNode;
  /** Whether the content is shown (controlled). */
  isExpanded?: boolean;
  /** Whether the content is shown at first (uncontrolled). */
  defaultExpanded?: boolean;
  onExpandedChange?: (isExpanded: boolean) => void;
};

/** The warning, a button that shows or hides what is behind it, and the content. Closed by default. */
export function ContentWarning({
  summary,
  summaryLang,
  toggleLabel,
  children,
  isExpanded,
  defaultExpanded = false,
  onExpandedChange,
  className,
  ...props
}: ContentWarningProps) {
  const [uncontrolled, setUncontrolled] = useState(defaultExpanded);
  const expanded = isExpanded ?? uncontrolled;
  const panelId = useId();
  const slots = contentWarning({ expanded });

  return (
    <div {...props} className={slots.base({ className })}>
      <div className={slots.summary()}>
        <span aria-hidden="true" className={slots.icon()}>
          <WarningIcon filled />
        </span>
        <p dir="auto" lang={summaryLang} className={slots.text()}>
          {summary}
        </p>
        <Button
          size="sm"
          className="bg-surface-raised"
          aria-expanded={expanded}
          aria-controls={panelId}
          onPress={() => {
            setUncontrolled(!expanded);
            onExpandedChange?.(!expanded);
          }}
        >
          {toggleLabel}
          <svg aria-hidden="true" viewBox="0 0 16 16" className={slots.chevron()}>
            <path d="M4 6l4 4 4-4" />
          </svg>
        </Button>
      </div>
      <div id={panelId} hidden={!expanded} className={slots.panel()}>
        {children}
      </div>
    </div>
  );
}
