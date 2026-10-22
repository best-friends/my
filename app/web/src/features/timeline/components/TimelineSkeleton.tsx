// SPDX-License-Identifier: AGPL-3.0-only
import { PostSkeleton, Stack, VisuallyHidden } from "@my/ui";
import { useT } from "../../../lib/i18n";

/** Placeholder cards, for as long as the first page of a timeline loads. */
const placeholders = ["first", "second", "third"] as const;

/**
 * What a timeline shows while its first page loads: a busy region with the shape of a few posts (hidden
 * from assistive technology) and one visually hidden text that names what is loading. A skeleton and a
 * spinner never share the region (web-client.md G15).
 */
export function TimelineSkeleton() {
  const t = useT();
  return (
    <Stack aria-busy="true">
      <VisuallyHidden>{t("client:timeline.loading")}</VisuallyHidden>
      {placeholders.map((key) => (
        <PostSkeleton key={key} />
      ))}
    </Stack>
  );
}
