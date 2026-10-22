// SPDX-License-Identifier: AGPL-3.0-only
// The body of the settings (design/components/LayoutSettings): where the column is wide enough (`@2xl`, 42rem), a
// sticky menu of the sections beside one pane; narrower, the menu is a page of its own and each section fills the
// width. It asks its own container, not the viewport (web-client.md D2).
import type { ReactNode } from "react";
import { tv } from "tailwind-variants";
import { Card } from "./Card";

const settingsLayout = tv({
  slots: {
    root: "@container",
    base: "grid grid-cols-1 items-start gap-6 @2xl:grid-cols-[14rem_minmax(0,1fr)]",
    menu: "@2xl:sticky @2xl:top-4",
    pane: "min-w-0",
  },
  variants: {
    // Narrower, one of the two is on screen: the menu on the page of the settings, the pane on a section's page.
    view: {
      menu: { pane: "@max-2xl:hidden" },
      pane: { menu: "@max-2xl:hidden" },
    },
  },
});

export type SettingsLayoutProps = {
  /** The menu's items (`RailItem`s in a list); the layout brings the card and the navigation landmark. */
  menu: ReactNode;
  /** The name of the menu's navigation landmark. */
  menuLabel: string;
  /** Narrow: `menu` (the page of the settings) shows the menu only; `pane` (a section) shows the section only. */
  view: "menu" | "pane";
  /** The section. */
  children: ReactNode;
};

/** A sticky menu card beside the pane card; narrow, one of them at a time. */
export function SettingsLayout({ menu, menuLabel, view, children }: SettingsLayoutProps) {
  const slots = settingsLayout({ view });
  return (
    <div className={slots.root()}>
      <div className={slots.base()}>
      <nav aria-label={menuLabel} className={slots.menu()}>
        <Card padding="3">{menu}</Card>
      </nav>
      <div className={slots.pane()}>
        <Card padding="8">{children}</Card>
      </div>
      </div>
    </div>
  );
}
