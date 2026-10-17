// SPDX-License-Identifier: AGPL-3.0-only
import { NotificationRowSkeleton, VisuallyHidden } from "@my/ui";
import { useT } from "../../../lib/i18n";

/** Placeholder rows, for as long as the first page of notifications loads. */
const placeholders = ["first", "second", "third"] as const;

/**
 * What the notifications show while their first page loads: a busy region with the shape of a few
 * notifications (hidden from assistive technology) and one visually hidden text that names what is
 * loading (web-client.md G15).
 */
export function NotificationsSkeleton() {
  const t = useT();
  return (
    <div aria-busy="true" className="flex flex-col">
      <VisuallyHidden>{t("client:notifications.loading")}</VisuallyHidden>
      {placeholders.map((key) => (
        <NotificationRowSkeleton key={key} />
      ))}
    </div>
  );
}
