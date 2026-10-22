// SPDX-License-Identifier: AGPL-3.0-only
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Box } from "./Box";
import { FeatureCard } from "./FeatureCard";
import { Grid } from "./Grid";
import { EmojiIcon, FederatedIcon, LanguageIcon } from "./Icons";

const meta = {
  title: "Layout/FeatureCard",
  component: FeatureCard,
  args: {
    icon: <FederatedIcon />,
    heading: "Connect beyond this server",
    children: "Follow and talk with people on Mastodon, Misskey and other servers that speak ActivityPub.",
  },
} satisfies Meta<typeof FeatureCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: (args) => (
    <ul className="max-w-xs">
      <FeatureCard {...args} />
    </ul>
  ),
};

/** Three cards in a row from a container of `md`, one above another below it. */
export const Row: Story = {
  render: () => (
    <Box container>
      <Grid as="ul" columns={3} at="md" gap="4">
        <FeatureCard icon={<FederatedIcon />} heading="Connect beyond this server">
          Follow and talk with people on Mastodon, Misskey and other servers that speak ActivityPub.
        </FeatureCard>
        <FeatureCard icon={<EmojiIcon />} heading="Say it with emoji">
          React with custom emoji, including those from other servers.
        </FeatureCard>
        <FeatureCard icon={<LanguageIcon />} heading="In your language">
          Use the app in English or Japanese, and tag each post with its language.
        </FeatureCard>
      </Grid>
    </Box>
  ),
};
