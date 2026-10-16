// SPDX-License-Identifier: AGPL-3.0-only
// Tabs stories. The play functions test the arrow keys in the reading direction (in ar-XB, Left moves
// to the next tab) and controlled selection, the way a route binds the selected tab to the URL.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import type { Key } from "react-aria-components";
import { expect, fn } from "storybook/test";
import { Tab, TabList, TabPanel, Tabs, type TabsProps } from "./Tabs";

const timelines = [
  { id: "home", label: "Home", body: "Posts from the people you follow." },
  { id: "local", label: "Local", body: "Public posts from this instance." },
  { id: "hashtags", label: "Hashtags", body: "Posts with the hashtags you follow." },
  { id: "lists", label: "Lists", body: "Your lists." },
] as const;

function Timelines(props: Omit<TabsProps, "children">) {
  return (
    <Tabs {...props}>
      <TabList aria-label="Timelines">
        {timelines.map(({ id, label }) => (
          <Tab key={id} id={id} isDisabled={id === "lists"}>
            {label}
          </Tab>
        ))}
      </TabList>
      {timelines.map(({ id, body }) => (
        <TabPanel key={id} id={id}>
          <p>{body}</p>
        </TabPanel>
      ))}
    </Tabs>
  );
}

/** The keys that move to the next and the previous tab in the page's reading direction. */
function arrowKeys() {
  return document.documentElement.dir === "rtl" ? { next: "{ArrowLeft}", previous: "{ArrowRight}" } : { next: "{ArrowRight}", previous: "{ArrowLeft}" };
}

const meta = {
  title: "Navigation/Tabs",
  component: Timelines,
} satisfies Meta<typeof Timelines>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const ArrowKeys: Story = {
  play: async ({ canvas, userEvent }) => {
    const keys = arrowKeys();
    await userEvent.tab();
    await expect(canvas.getByRole("tab", { name: "Home" })).toHaveFocus();

    await userEvent.keyboard(keys.next);
    const local = canvas.getByRole("tab", { name: "Local" });
    await expect(local).toHaveFocus();
    await expect(local).toHaveAttribute("aria-selected", "true");
    await expect(canvas.getByRole("tabpanel")).toHaveTextContent("Public posts from this instance.");

    // The disabled last tab is skipped, so moving on wraps around to the first.
    await userEvent.keyboard(keys.next);
    await userEvent.keyboard(keys.next);
    await expect(canvas.getByRole("tab", { name: "Home" })).toHaveFocus();
    await userEvent.keyboard(keys.previous);
    await expect(canvas.getByRole("tab", { name: "Hashtags" })).toHaveFocus();
    await userEvent.keyboard("{Home}");
    await expect(canvas.getByRole("tab", { name: "Home" })).toHaveAttribute("aria-selected", "true");
  },
};

/**
 * The selection lives outside, as a route keeps it in the search params: the tabs show `selectedKey`
 * and report a change through `onSelectionChange`, which updates the "URL".
 */
function UrlBoundTabs({ onNavigate }: { onNavigate: (key: Key) => void }) {
  const [tab, setTab] = useState<Key>("local");
  return (
    <div className="flex flex-col gap-2">
      <code className="font-mono text-xs text-fg-muted">?tab={String(tab)}</code>
      <Timelines
        selectedKey={tab}
        onSelectionChange={(key) => {
          onNavigate(key);
          setTab(key);
        }}
      />
    </div>
  );
}

export const Controlled: StoryObj<typeof UrlBoundTabs> = {
  args: { onNavigate: fn() },
  render: (args) => <UrlBoundTabs {...args} />,
  play: async ({ args, canvas, userEvent }) => {
    await expect(canvas.getByRole("tab", { name: "Local" })).toHaveAttribute("aria-selected", "true");
    await userEvent.click(canvas.getByRole("tab", { name: "Hashtags" }));
    await expect(args.onNavigate).toHaveBeenCalledWith("hashtags");
    await expect(canvas.getByText("?tab=hashtags")).toBeInTheDocument();
    await expect(canvas.getByRole("tab", { name: "Hashtags" })).toHaveAttribute("aria-selected", "true");
  },
};
