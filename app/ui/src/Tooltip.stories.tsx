// SPDX-License-Identifier: AGPL-3.0-only
// Tooltip stories. The play function tests that keyboard focus shows the tooltip, that it describes the
// trigger, and that Esc hides it while focus stays on the trigger.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "react-aria-components";
import { expect, waitFor } from "storybook/test";
import { Tooltip, TooltipTrigger } from "./Tooltip";

const iconButton =
  "grid size-10 place-items-center rounded-md border border-border-strong bg-surface-raised text-fg data-hovered:bg-surface-sunken data-focus-visible:focus-ring";

function BoostIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 16 16" className="size-4 fill-none stroke-current stroke-[1.5]">
      <path d="M3 6l2-2 2 2M5 4v6a2 2 0 002 2h4M13 10l-2 2-2-2M11 12V6a2 2 0 00-2-2H5" />
    </svg>
  );
}

const meta = {
  title: "Overlays/Tooltip",
  component: Tooltip,
  decorators: [
    (Story) => (
      <div className="grid min-h-40 place-items-center">
        <Story />
      </div>
    ),
  ],
  args: { children: "Boost to your followers" },
  render: (args) => (
    <TooltipTrigger>
      <Button aria-label="Boost" className={iconButton}>
        <BoostIcon />
      </Button>
      <Tooltip {...args} />
    </TooltipTrigger>
  ),
} satisfies Meta<typeof Tooltip>;

export default meta;
type Story = StoryObj<typeof meta>;

export const KeyboardFocus: Story = {
  play: async ({ canvas, userEvent }) => {
    const trigger = canvas.getByRole("button", { name: "Boost" });
    await userEvent.tab();
    await expect(trigger).toHaveFocus();
    const tooltip = await canvas.findByRole("tooltip");
    await expect(tooltip).toHaveTextContent("Boost to your followers");
    await expect(trigger).toHaveAttribute("aria-describedby", tooltip.id);

    await userEvent.keyboard("{Escape}");
    await waitFor(() => expect(canvas.queryByRole("tooltip")).not.toBeInTheDocument());
    await expect(trigger).toHaveFocus();
  },
};

export const Open: Story = {
  render: (args) => (
    <TooltipTrigger defaultOpen>
      <Button aria-label="Boost" className={iconButton}>
        <BoostIcon />
      </Button>
      <Tooltip {...args} />
    </TooltipTrigger>
  ),
};

export const Bottom: Story = { ...Open, args: { placement: "bottom" } };

/** start / end follow the reading direction. */
export const End: Story = { ...Open, args: { placement: "end" } };
