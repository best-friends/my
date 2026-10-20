// SPDX-License-Identifier: AGPL-3.0-only
// A profile (FR-204): the header with the account's picture, name, handle, profile text, metadata fields,
// follow counts and the relationship buttons, then its posts in tabs (WAI-ARIA APG, tabs pattern) whose
// selection the route keeps in the URL (web-client.md S3).
import { Avatar, EmojiText, ExternalLinkIcon, Link, PostContent, Tab, TabList, TabPanel, Tabs, VisuallyHidden } from "@my/ui";
import { type ReactNode, useId, useState } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { type AccountTimelineFilter, accountTimelineFilters, TimelineFeed, useRemoteFetch } from "../../timeline";
import { isolate, type ProfileView as ProfileViewData, useProfile } from "../hooks/useProfile";
import { useRelationship } from "../hooks/useRelationship";
import { RelationshipActions } from "./RelationshipActions";

/** The label of each tab. */
const tabLabels: Readonly<Record<AccountTimelineFilter, ClientMessageKey>> = {
  posts: "client:profile.tab.posts",
  replies: "client:profile.tab.replies",
  no_boosts: "client:profile.tab.no_boosts",
};

export type ProfileViewProps = {
  /** The handle in the address: `username`, or `username@domain`. */
  acct: string;
  /** The selected tab. */
  tab: AccountTimelineFilter;
  onTabChange: (tab: AccountTimelineFilter) => void;
  /** Renders the page heading (the route's level-one heading) around the account's name. */
  renderHeading: (name: ReactNode) => ReactNode;
};

/** The profile of the account whose handle is `acct`; the route loader has fetched the account and the selected tab. */
export function ProfileView({ acct, tab, onTabChange, renderHeading }: ProfileViewProps) {
  const t = useT();
  const profile = useProfile(acct);
  const relationship = useRelationship(profile.account);
  const { account } = profile;
  const relation = relationship.kind === "other" ? relationship.relationship : null;
  const tags = [
    relation?.followed_by === true && t("client:profile.tag.follows_you"),
    relation?.requested_by === true && t("client:profile.tag.requested_you"),
    relation?.muting === true && t("client:profile.tag.muted"),
    account.locked && t("client:profile.tag.locked"),
    account.bot && t("client:profile.tag.bot"),
  ].filter((tag) => tag !== false);

  return (
    <div className="flex flex-col gap-4">
      {/* The size container of the header and the tabs (web-client.md D2). */}
      <header className="@container flex flex-col">
        <HeaderImage src={account.header_url} />
        <div className="flex flex-wrap items-end justify-between gap-3 px-4">
          <Avatar src={account.avatar_url} name={profile.name} decorative size="xl" className="-mt-10 ring-4 ring-surface" />
          <RelationshipActions account={account} name={profile.name} state={relationship} />
        </div>
        <div className="mt-3 flex flex-col gap-3 px-4">
          <div className="min-w-0 wrap-anywhere">
            {renderHeading(
              <span dir="auto">
                <EmojiText text={profile.name} emojis={account.emojis} />
              </span>,
            )}
            {/* The handle reads left to right, and lines up with the page's start in either direction. */}
            <p className="text-fg-muted">
              <span dir="ltr">{profile.handle}</span>
            </p>
          </div>
          {tags.length > 0 && (
            <ul className="flex flex-wrap gap-2">
              {tags.map((tag) => (
                <li key={tag} className="rounded-full bg-surface-sunken px-2 py-0.5 text-sm text-fg">
                  {tag}
                </li>
              ))}
            </ul>
          )}
          {profile.note !== null && <PostContent html={profile.note} emojis={account.emojis} onLinkClick={profile.onLinkClick} />}
          {profile.fields.length > 0 && (
            <dl className="flex flex-col divide-y divide-border rounded-lg border border-border">
              {profile.fields.map((field) => (
                <div key={field.key} className="flex flex-col gap-1 px-3 py-2 @md:flex-row @md:gap-4">
                  <dt dir="auto" className="text-sm font-semibold wrap-anywhere text-fg-muted align-match-parent @md:w-1/3 @md:shrink-0">
                    <EmojiText text={field.name} emojis={account.emojis} />
                  </dt>
                  <dd className="min-w-0">
                    <PostContent html={field.value} emojis={account.emojis} onLinkClick={profile.onLinkClick} />
                  </dd>
                </div>
              ))}
            </dl>
          )}
          <p className="text-sm text-fg-muted">
            {t("client:profile.joined", { date: profile.joined.date })}
          </p>
          {(profile.following !== null || profile.followers !== null) && (
            <ul className="flex flex-wrap gap-x-4 gap-y-1">
              {profile.following !== null && (
                <li>
                  <Link href={profile.following.href} variant="standalone">
                    {t("client:profile.following_count", { count: profile.following.count })}
                  </Link>
                </li>
              )}
              {profile.followers !== null && (
                <li>
                  <Link href={profile.followers.href} variant="standalone">
                    {t("client:profile.followers_count", { count: profile.followers.count })}
                  </Link>
                </li>
              )}
            </ul>
          )}
        </div>
      </header>
      <ProfileTabs accountId={account.id} name={profile.name} original={profile.original} tab={tab} onTabChange={onTabChange} />
    </div>
  );
}

/**
 * The account's header image, or a plain band when it has none or the image fails to load. Either is
 * decorative (`alt=""`, ADR-0027): the name and the profile below it say whose page this is.
 */
function HeaderImage({ src }: { src: string | undefined }) {
  // The URL that failed, not a flag: a new src is tried again without resetting state in an Effect.
  const [failedSrc, setFailedSrc] = useState<string>();
  if (src === undefined || src === "" || src === failedSrc) {
    return <div aria-hidden="true" className="h-24 rounded-lg bg-surface-sunken @md:h-36" />;
  }
  return (
    <img
      src={src}
      alt=""
      decoding="async"
      className="h-24 w-full rounded-lg bg-surface-sunken object-cover @md:h-36"
      onError={() => {
        setFailedSrc(src);
      }}
    />
  );
}

type ProfileTabsProps = {
  accountId: string;
  name: string;
  original: ProfileViewData["original"];
  tab: AccountTimelineFilter;
  onTabChange: (tab: AccountTimelineFilter) => void;
};

function ProfileTabs({ accountId, name, original, tab, onTabChange }: ProfileTabsProps) {
  const t = useT();
  const headingId = useId();
  return (
    <Tabs
      selectedKey={tab}
      onSelectionChange={(key) => {
        const next = accountTimelineFilters.find((filter) => filter === key);
        if (next !== undefined) onTabChange(next);
      }}
    >
      <TabList aria-label={t("client:profile.tabs", { name: isolate(name) })}>
        {accountTimelineFilters.map((filter) => (
          <Tab key={filter} id={filter}>
            {t(tabLabels[filter])}
          </Tab>
        ))}
      </TabList>
      {/* Only the selected panel is mounted (React Aria Tabs), so only its timeline loads. */}
      {accountTimelineFilters.map((filter) => (
        <TabPanel key={filter} id={filter}>
          <VisuallyHidden>
            <h2 id={headingId}>{t(tabLabels[filter])}</h2>
          </VisuallyHidden>
          {/* Another tab is another timeline: nothing of the previous one carries over (web-client.md R10). */}
          <AccountPosts key={filter} accountId={accountId} filter={filter} original={original} labelledBy={headingId} />
        </TabPanel>
      ))}
    </Tabs>
  );
}

type AccountPostsProps = { accountId: string; filter: AccountTimelineFilter; original: ProfileViewData["original"]; labelledBy: string };

/**
 * A tab's posts. For an account of another server they are only those this instance has received, which
 * the feed says above them, with a link to the profile on the account's own server; while the server fetches
 * the account's recent posts from there, a line says so until they are read again (ADR-0044).
 */
function AccountPosts({ accountId, filter, original, labelledBy }: AccountPostsProps) {
  const t = useT();
  const fetching = useRemoteFetch(accountId, filter);
  const origin =
    original === null
      ? undefined
      : {
          notice: t("client:profile.remote.notice"),
          link: (
            // The profile on the account's own server leaves the app.
            <Link href={original.href} rel="noreferrer" variant="standalone" className="gap-1">
              {t("client:profile.remote.original", { host: isolate(original.host) })}
              <ExternalLinkIcon />
            </Link>
          ),
        };
  return (
    <div className="flex flex-col">
      {/* Visible only: useRemoteFetch announces it in the app's polite live region, which was in the page before it. */}
      {fetching && (
        <p className="px-4 py-3 text-sm text-fg-muted">{t("client:profile.remote.fetching")}</p>
      )}
      <TimelineFeed timeline={{ kind: "account", id: accountId, filter }} aria-labelledby={labelledBy} {...(origin !== undefined && { origin })} />
    </div>
  );
}
