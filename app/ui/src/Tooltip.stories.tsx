// SPDX-License-Identifier: AGPL-3.0-only
// Tooltip stories. The play function tests that keyboard focus shows the tooltip, that it describes the
// trigger, and that Esc hides it while focus stays on the trigger.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, waitFor } from "storybook/test";
import { Button } from "./Button";
import { BoostIcon } from "./Icons";
import { Tooltip, TooltipTrigger } from "./Tooltip";

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
      <Button aria-label="Boost" className="size-10 px-0">
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
      <Button aria-label="Boost" className="size-10 px-0">
        <BoostIcon />
      </Button>
      <Tooltip {...args} />
    </TooltipTrigger>
  ),
};

export const Bottom: Story = { ...Open, args: { placement: "bottom" } };

/** start / end follow the reading direction. */
export const End: Story = { ...Open, args: { placement: "end" } };
