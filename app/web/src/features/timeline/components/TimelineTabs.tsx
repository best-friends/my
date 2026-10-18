// SPDX-License-Identifier: AGPL-3.0-only
import { Tab, TabList, TabPanel, Tabs } from "@my/ui";
import type { ReactNode } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { type TimelineAvailability, useTimelineAvailability } from "../hooks/useTimelineAvailability";

/** A public timeline that is a tab of the Timelines screen. */
type PublicTimelineKind = "local" | "public";

type TimelineTab = {
  id: PublicTimelineKind;
  href: "/local" | "/public";
  label: ClientMessageKey;
  availability: keyof TimelineAvailability;
};

const timelineTabs: readonly TimelineTab[] = [
  { id: "local", href: "/local", label: "client:timeline.tabs.local", availability: "local" },
  { id: "public", href: "/public", label: "client:timeline.tabs.public", availability: "federated" },
];

type TimelineTabsProps = {
  /** The timeline of the page, whose tab is selected. */
  current: PublicTimelineKind;
  /** The page's timeline (or its skeleton), the panel of the selected tab. */
  children: ReactNode;
};

/**
 * The local and the federated timeline as tabs (ADR-0039): the Timelines screen that the phones' tab bar opens.
 * Each tab is a link to its own page (`/local`, `/public`), so the selection is the URL (web-client.md S3) and
 * the router's navigation selects a tab (React Aria Tabs with `href`). The arrow keys only move focus between
 * the tabs (manual activation), and Enter or Space follows one: following a tab opens another page, which
 * would take focus away from the tabs while the arrow keys are still moving through them.
 *
 * Only the tabs of the timelines the instance serves are shown (FR-202), and the page's own tab always: on a
 * server that serves one of them the screen has one tab. The tabs are there from the first render whatever the
 * instance answers, so that an answer arriving after the first page never remounts the timeline below them.
 */
export function TimelineTabs({ current, children }: TimelineTabsProps) {
  const t = useT();
  const availability = useTimelineAvailability();
  const shown = timelineTabs.filter((tab) => tab.id === current || availability[tab.availability]);
  return (
    <Tabs selectedKey={current} keyboardActivation="manual">
      <TabList aria-label={t("client:timeline.tabs.label")}>
        {shown.map((tab) => (
          <Tab key={tab.id} id={tab.id} href={tab.href}>
            {t(tab.label)}
          </Tab>
        ))}
      </TabList>
      <TabPanel id={current}>{children}</TabPanel>
    </Tabs>
  );
}
