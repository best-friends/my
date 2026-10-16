// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { PlaceholderPage } from "./-layout/PlaceholderPage";

/** The settings (placeholder until the settings feature exists). */
export const Route = createFileRoute("/settings")({
  component: SettingsPage,
});

function SettingsPage() {
  return <PlaceholderPage titleKey="client:settings.title" />;
}
