// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Select, SelectItem } from "./Select";

const languages = [
  { id: "en", name: "English" },
  { id: "ja", name: "日本語" },
  { id: "ar", name: "العربية" },
  { id: "de", name: "Deutsch" },
];

const meta = {
  title: "Forms/Select",
  component: Select<(typeof languages)[number]>,
  args: {
    label: "Post language",
    placeholder: "Choose a language",
    items: languages,
    children: (language) => <SelectItem id={language.id}>{language.name}</SelectItem>,
  },
} satisfies Meta<typeof Select<(typeof languages)[number]>>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Placeholder: Story = {};

export const Selected: Story = { args: { defaultValue: "ja", description: "Readers can filter posts by language." } };

export const Invalid: Story = { args: { isInvalid: true, errorMessage: "Choose a language." } };

export const Disabled: Story = { args: { defaultValue: "en", isDisabled: true } };

/** The open list, for the accessibility check of the options; it renders in a popover outside the frame. */
export const Open: Story = { args: { defaultValue: "ja", defaultOpen: true } };
