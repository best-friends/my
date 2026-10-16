// SPDX-License-Identifier: AGPL-3.0-only
import { Skeleton, Spinner } from "@my/ui";
import { useT } from "../../../lib/i18n";

/** Placeholder cards, for as long as the first page of a timeline loads. */
const placeholders = ["first", "second", "third"] as const;

/**
 * What a timeline shows while its first page loads: a busy region with the shape of a few cards (hidden
 * from assistive technology) and a progress indicator that names what is loading (web-client.md E3).
 */
export function TimelineSkeleton() {
  const t = useT();
  return (
    <div aria-busy="true" className="flex flex-col">
      <Spinner label={t("client:timeline.loading")} className="self-center" />
      {placeholders.map((key) => (
        <div key={key} className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-2 border-b border-border px-4 py-3">
          <Skeleton shape="circle" />
          <div className="flex flex-col gap-2">
            <Skeleton className="w-1/3" />
            <Skeleton />
            <Skeleton className="w-2/3" />
          </div>
        </div>
      ))}
    </div>
  );
}
