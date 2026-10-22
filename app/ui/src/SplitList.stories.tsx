// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";
import { Button } from "./Button";
import { DescriptionItem, DescriptionList } from "./DescriptionList";
import { Heading } from "./Heading";
import { SplitList } from "./SplitList";
import { Stack } from "./Stack";
import { Table, TableBody, TableCell, TableColumnHeader, TableHead, TableRow, TableRowHeader } from "./Table";
import { Tag } from "./Tag";

const reports = [
  { id: "1042", account: "@spam_bot@junk.example", reason: "Spam", state: "Open" },
  { id: "1041", account: "@ren@other.example", reason: "Harassment", state: "In progress" },
  { id: "1040", account: "@aoi", reason: "Other", state: "Resolved" },
];

const list = (
  <Table density="compact" caption="Reports">
    <TableHead>
      <TableColumnHeader>Account</TableColumnHeader>
      <TableColumnHeader>Reason</TableColumnHeader>
      <TableColumnHeader>State</TableColumnHeader>
    </TableHead>
    <TableBody>
      {reports.map((report) => (
        <TableRow key={report.id}>
          <TableRowHeader>{report.account}</TableRowHeader>
          <TableCell>{report.reason}</TableCell>
          <TableCell>
            <Tag>{report.state}</Tag>
          </TableCell>
        </TableRow>
      ))}
    </TableBody>
  </Table>
);

const detail = (
  <Stack gap="4">
    <Heading level={2}>@spam_bot@junk.example</Heading>
    <DescriptionList>
      <DescriptionItem term="Reason">Spam</DescriptionItem>
      <DescriptionItem term="Reporter">@sora</DescriptionItem>
      <DescriptionItem term="Posts">3</DescriptionItem>
    </DescriptionList>
    <Button variant="danger">Suspend account</Button>
  </Stack>
);

const meta = {
  title: "Admin/SplitList",
  component: SplitList,
  args: { list, detail, detailLabel: "Report 1042" },
} satisfies Meta<typeof SplitList>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Wide: the list and the sticky detail card side by side. */
export const ListAndDetail: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("table", { name: "Reports" })).toBeVisible();
    await expect(canvas.getByRole("complementary", { name: "Report 1042" })).toBeVisible();
  },
};

/** Nothing selected: the list takes the whole width. */
export const ListOnly: Story = { args: { list } };

/** Narrow, the detail as a page of its own (below `@4xl` the list is hidden). */
export const DetailPage: Story = { args: { view: "detail" } };
