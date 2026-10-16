// SPDX-License-Identifier: AGPL-3.0-only
// Tabs: React Aria's Tabs + TabList + Tab + TabPanel (WAI-ARIA APG, tabs pattern with automatic activation).
// Left / Right follow the reading direction (in RTL, Left moves to the next tab), Home / End jump to the ends.
//
// The selection is controllable, so a route can keep the tab in the URL (web-client.md S3): pass the key
// from the validated search params as `selectedKey`, and navigate in `onSelectionChange`. Each Tab may
// instead have an `href`, so that the tabs are links and the router's own navigation selects them.
import type { ReactNode, Ref } from "react";
import {
  Tab as AriaTab,
  TabList as AriaTabList,
  TabPanel as AriaTabPanel,
  Tabs as AriaTabs,
  type TabListProps as AriaTabListProps,
  type TabPanelProps as AriaTabPanelProps,
  type TabProps as AriaTabProps,
  type Key,
  type TabsProps as AriaTabsProps,
} from "react-aria-components";
import { tv } from "tailwind-variants";

const tabs = tv({
  slots: {
    root: "flex flex-col gap-4",
    list: "flex gap-1 overflow-x-auto border-b border-border",
    tab: [
      "flex shrink-0 cursor-default items-center border-b-2 border-transparent px-4 py-2 text-sm font-medium",
      "text-fg-muted outline-none select-none pointer-coarse:min-h-11",
      "data-hovered:text-fg data-selected:border-accent data-selected:text-accent data-disabled:opacity-60",
      // The list scrolls sideways on narrow screens and would clip an outer ring, so the tab's ring is inset.
      "data-focus-visible:rounded-t-md data-focus-visible:outline-2 data-focus-visible:-outline-offset-2 data-focus-visible:outline-focus",
      "motion-safe:transition-colors",
    ],
    panel: "outline-none data-focus-visible:rounded-md data-focus-visible:focus-ring",
  },
});

export type TabsProps = Omit<AriaTabsProps, "className" | "style" | "children" | "orientation"> & {
  children: ReactNode;
  /** The selected tab's id, for a selection kept outside (the URL). */
  selectedKey?: Key | null;
  onSelectionChange?: (key: Key) => void;
  /** Classes merged onto the root with tailwind-merge. */
  className?: string;
  ref?: Ref<HTMLDivElement>;
};

/** A set of tabs: a TabList followed by one TabPanel per Tab, matched by `id`. */
export function Tabs({ className, ...props }: TabsProps) {
  return <AriaTabs {...props} className={tabs().root({ class: className })} />;
}

export type TabListProps<T extends object> = Omit<AriaTabListProps<T>, "className" | "style"> & {
  /** What the tabs switch between, for assistive technology (or use aria-labelledby). */
  "aria-label"?: string;
  className?: string;
  ref?: Ref<HTMLDivElement>;
};

export function TabList<T extends object>({ className, ...props }: TabListProps<T>) {
  return <AriaTabList {...props} className={tabs().list({ class: className })} />;
}

export type TabProps = Omit<AriaTabProps, "className" | "style" | "children"> & {
  /** Matches the `id` of its TabPanel and is the key of `selectedKey`. */
  id: Key;
  children: ReactNode;
  className?: string;
  ref?: Ref<HTMLDivElement>;
};

export function Tab({ className, ...props }: TabProps) {
  return <AriaTab {...props} className={tabs().tab({ class: className })} />;
}

export type TabPanelProps = Omit<AriaTabPanelProps, "className" | "style" | "children"> & {
  id: Key;
  children: ReactNode;
  className?: string;
  ref?: Ref<HTMLDivElement>;
};

export function TabPanel({ className, ...props }: TabPanelProps) {
  return <AriaTabPanel {...props} className={tabs().panel({ class: className })} />;
}
