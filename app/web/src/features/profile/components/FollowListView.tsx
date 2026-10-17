// SPDX-License-Identifier: AGPL-3.0-only
// The accounts that follow an account, or that it follows: a list of links to their profiles, with a
// button that loads the next page. A button rather than loading at the end of the scroll keeps the end of
// the list reachable for keyboard users and the following content in reach (WCAG 2.1.1).
import { Avatar, BackIcon, Button, EmptyState, Link } from "@my/ui";
import type { ReactNode } from "react";
import { useT } from "../../../lib/i18n";
import { profilePath } from "../../../lib/paths";
import type { FollowList } from "../api/accounts";
import { useFollowList } from "../hooks/useFollowList";
import { isolate, useProfile } from "../hooks/useProfile";

export type FollowListViewProps = {
  /** The handle in the address: `username`, or `username@domain`. */
  acct: string;
  list: FollowList;
  /** Renders the page heading (the route's level-one heading) around its text. */
  renderHeading: (text: ReactNode) => ReactNode;
};

/** The follow list of the account whose handle is `acct`; the route loader has fetched the account and the first page. */
export function FollowListView({ acct, list, renderHeading }: FollowListViewProps) {
  const t = useT();
  const profile = useProfile(acct);
  const { items, hasMore, isLoadingMore, loadMore, loadMoreError } = useFollowList(profile.account.id, list);
  // The name is the user's text, isolated from the sentence around it (FSI … PDI, web-client.md Y10).
  const isolated = isolate(profile.name);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        {renderHeading(list === "followers" ? t("client:profile.followers.title", { name: isolated }) : t("client:profile.following.title", { name: isolated }))}
        <p>
          <Link href={profilePath(profile.account.acct)} variant="standalone" className="gap-1">
            <BackIcon />
            {t("client:profile.list.back", { name: isolated })}
          </Link>
        </p>
      </div>
      {items.length === 0 ? (
        <EmptyState heading={list === "followers" ? t("client:profile.followers.empty") : t("client:profile.following.empty")} />
      ) : (
        <ul className="flex flex-col divide-y divide-border border-y border-border">
          {items.map((item) => (
            <li key={item.id}>
              <Link
                href={item.href}
                variant="standalone"
                className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)] items-center gap-x-3 px-4 py-3 text-fg data-hovered:bg-surface-sunken data-hovered:text-fg"
              >
                <>
                  <Avatar name={item.name} decorative className="row-span-2" />
                  <span dir="auto" className="font-semibold wrap-anywhere">
                    {item.name}
                  </span>
                  {/* The space keeps the name and the handle apart in the link's name. */}{" "}
                  <span dir="ltr" className="text-sm font-normal wrap-anywhere text-fg-muted">
                    {item.handle}
                  </span>
                </>
              </Link>
            </li>
          ))}
        </ul>
      )}
      {loadMoreError !== null && <p className="text-center">{t("client:profile.list.load_more_failed", { reason: loadMoreError })}</p>}
      {hasMore && (
        <div className="flex justify-center">
          {/* A pending button keeps its focus while the page loads (React Aria Button, isPending). */}
          <Button isPending={isLoadingMore} onPress={loadMore}>
            {loadMoreError === null ? t("client:profile.list.load_more") : t("client:profile.list.retry")}
          </Button>
        </div>
      )}
    </div>
  );
}
