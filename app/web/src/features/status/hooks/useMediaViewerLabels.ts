// SPDX-License-Identifier: AGPL-3.0-only
import type { MediaViewerLabels } from "@my/ui";
import { useT } from "../../../lib/i18n";

/** The texts of the image viewer that opens a post's picture at its original size (@my/ui's MediaGrid). */
export function useMediaViewerLabels(): MediaViewerLabels {
  const t = useT();
  return {
    title: (position, count) => t("client:timeline.media.viewer.title", { position, count }),
    close: t("client:timeline.media.viewer.close"),
    previous: t("client:timeline.media.viewer.previous"),
    next: t("client:timeline.media.viewer.next"),
  };
}
