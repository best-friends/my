// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { PlaceholderPage } from "./-layout/PlaceholderPage";

/** The home timeline (placeholder until the timeline feature exists). */
export const Route = createFileRoute("/")({
  component: HomePage,
});

function HomePage() {
  return <PlaceholderPage titleKey="client:home.title" />;
}
