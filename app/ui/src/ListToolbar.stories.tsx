// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { ListToolbar } from "./ListToolbar";
import { Select, SelectItem } from "./Select";
import { TextField } from "./TextField";

const meta = {
  title: "Admin/ListToolbar",
  component: ListToolbar,
  args: {
    label: "Filters",
    children: (
      <>
        <Select variant="toolbar" aria-label="State" defaultValue="open">
          <SelectItem id="open">Open</SelectItem>
          <SelectItem id="resolved">Resolved</SelectItem>
        </Select>
        <Select variant="toolbar" aria-label="Reason" defaultValue="all">
          <SelectItem id="all">Any reason</SelectItem>
          <SelectItem id="spam">Spam</SelectItem>
        </Select>
      </>
    ),
    search: <TextField aria-label="Search accounts" placeholder="Search accounts" type="search" grow />,
  },
} satisfies Meta<typeof ListToolbar>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The filters at the start and the search at the end; the row wraps where the container is narrow. */
export const FiltersAndSearch: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("group", { name: "Filters" })).toBeInTheDocument();
    await expect(canvas.getByRole("searchbox", { name: "Search accounts" })).toBeVisible();
  },
};

/** Without a search field. */
export const FiltersOnly: Story = { args: { search: undefined } };
