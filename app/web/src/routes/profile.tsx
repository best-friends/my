// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { PlaceholderPage } from "./-layout/PlaceholderPage";

/** The signed-in user's own profile (placeholder until the profile feature exists). */
export const Route = createFileRoute("/profile")({
  component: ProfilePage,
});

function ProfilePage() {
  return <PlaceholderPage titleKey="client:profile.title" />;
}
