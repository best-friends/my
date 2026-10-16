// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { PlaceholderPage } from "./-layout/PlaceholderPage";

/** The notifications (placeholder until the notifications feature exists). */
export const Route = createFileRoute("/notifications")({
  component: NotificationsPage,
});

function NotificationsPage() {
  return <PlaceholderPage titleKey="client:notifications.title" />;
}
