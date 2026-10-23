// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { TextArea } from "./TextArea";

// Stories may spell out English; apps pass a react-intl message instead (web-client/README.md A9).
const formatRemaining = (remaining: number) =>
  remaining < 0 ? `${-remaining} characters too many` : `${remaining} characters left`;

const meta = {
  title: "Forms/TextArea",
  component: TextArea,
  args: { label: "Bio" },
} satisfies Meta<typeof TextArea>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithDescription: Story = { args: { description: "A few words about you.", placeholder: "I post about…", rows: 3 } };

export const Invalid: Story = {
  args: { defaultValue: "", isInvalid: true, errorMessage: "Write something before saving." },
};

export const Disabled: Story = { args: { defaultValue: "Gone fishing.", isDisabled: true } };

/** With a limit, the count follows the control; type to hear it announced. */
export const CharacterCount: Story = {
  args: { label: "Post", characterLimit: 500, formatRemaining, defaultValue: "Hello, fediverse!" },
};

export const NearTheLimit: Story = {
  args: { label: "Post", characterLimit: 40, formatRemaining, defaultValue: "This post is nearly at the limit." },
};

/** Longer text is kept, the count turns to an error and the field is invalid. */
export const OverTheLimit: Story = {
  args: {
    label: "Post",
    characterLimit: 20,
    formatRemaining,
    defaultValue: "This post is longer than the limit allows.",
    errorMessage: "Shorten the post to send it.",
  },
};
