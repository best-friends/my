// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { CodeList } from "./CodeList";

const meta = {
  title: "Display/CodeList",
  component: CodeList,
  args: {
    codes: ["a1b2-c3d4", "e5f6-a7b8", "c9d0-e1f2", "a3b4-c5d6", "e7f8-a9b0", "c1d2-e3f4"],
  },
  decorators: [
    (Story) => (
      <div className="@container">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CodeList>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
