// SPDX-License-Identifier: AGPL-3.0-only
// Spike #1152: the B1 prototype (React Aria Autocomplete + the ADR-0092 §3 overrides) with the C1 highlight layer.
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fixedCandidates } from "./candidates";
import { TextAreaAutocompleteB1 } from "./TextAreaAutocompleteB1";

const meta = {
  title: "Spike/1152 TextAreaAutocomplete",
  component: TextAreaAutocompleteB1,
  tags: ["no-screenshot"],
  args: { label: "Post", candidates: fixedCandidates, defaultValue: "Hello @ali #日本 :blob https://example.com/a" },
} satisfies Meta<typeof TextAreaAutocompleteB1>;

export default meta;
type Story = StoryObj<typeof meta>;

export const B1: Story = {};
export const Raw: Story = { args: { variant: "raw" } };
export const OverLimit: Story = { args: { characterLimit: 20 } };
export const RightToLeft: Story = { args: { defaultValue: "مرحبا @alice #日本語 كيف حالك اليوم؟ مرحبا بكم في الخادم" } };
