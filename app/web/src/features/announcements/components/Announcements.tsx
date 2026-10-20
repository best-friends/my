// SPDX-License-Identifier: AGPL-3.0-only
// The server's announcements above the home timeline (FR-726): a region with a heading, nothing when there are
// none. Each announcement is an article with its date as heading; an unread one says so in its heading, in words
// rather than by color alone (WCAG 1.4.1), and has a "Mark as read" button. Once marked, the button goes and focus
// moves to the announcement's heading (useFocusHeadingAfterLift). The body is the server's sanitized HTML of the
// administrator's Markdown (web-client.md A8), in the direction of its own text (Y10).
import { Badge, Button, PostContent, serverSanitizedHtml } from "@my/ui";
import { useId, useRef } from "react";
import { useIntl } from "react-intl";
import { useContentLinkClick } from "../../../lib/contentLinks";
import { useT } from "../../../lib/i18n";
import { useFocusHeadingAfterLift } from "../../../lib/useFocusAfterRemoval";
import type { Announcement } from "../api/announcements";
import { type Announcements as AnnouncementsState, useAnnouncements } from "../hooks/useAnnouncements";

export function Announcements() {
  const t = useT();
  const headingId = useId();
  const announcements = useAnnouncements();
  const listRef = useRef<HTMLUListElement>(null);
  const read = useFocusHeadingAfterLift(listRef);
  if (announcements.items.length === 0) return null;
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3 border-b border-border p-4">
      <h2 id={headingId} className="flex items-center gap-2 text-lg font-semibold">
        {t("client:announcements.title")}
        {announcements.unread > 0 && (
          <Badge count={announcements.unread} label={t("client:announcements.unread_count", { count: announcements.unread })} />
        )}
      </h2>
      {announcements.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:announcements.mark_read_failed", { reason: announcements.error })}
        </p>
      )}
      <ul ref={listRef} className="flex flex-col gap-3">
        {announcements.items.map((announcement) => (
          <li key={announcement.id} data-row-id={announcement.id}>
            <AnnouncementItem announcement={announcement} state={announcements} onRead={read} />
          </li>
        ))}
      </ul>
    </section>
  );
}

type AnnouncementItemProps = { announcement: Announcement; state: AnnouncementsState; onRead: (id: string) => void };

function AnnouncementItem({ announcement, state, onRead }: AnnouncementItemProps) {
  const t = useT();
  const intl = useIntl();
  const headingId = useId();
  const onLinkClick = useContentLinkClick();
  const date = intl.formatDate(announcement.published_at, { dateStyle: "medium" });
  return (
    <article aria-labelledby={headingId} className="flex flex-col gap-2 rounded-lg border border-border p-3">
      <h3 id={headingId} tabIndex={-1} className="flex flex-wrap items-center gap-2 text-sm text-fg-muted outline-none">
        <time dateTime={announcement.published_at}>{t("client:announcements.heading", { date })}</time>
        {!announcement.read && (
          <span className="rounded-full bg-accent px-2 text-xs font-semibold text-accent-fg">{t("client:announcements.unread")}</span>
        )}
      </h3>
      <PostContent html={serverSanitizedHtml(announcement.content)} onLinkClick={onLinkClick} />
      {announcement.ends_at !== undefined && (
        <p className="text-sm text-fg-muted">
          {t("client:announcements.until", { date: intl.formatDate(announcement.ends_at, { dateStyle: "medium", timeStyle: "short" }) })}
        </p>
      )}
      {!announcement.read && (
        <div>
          <Button
            size="sm"
            isPending={state.markingIds.includes(announcement.id)}
            aria-describedby={headingId}
            onPress={() => {
              state.markRead(announcement, () => {
                onRead(announcement.id);
              });
            }}
          >
            {t("client:announcements.mark_read")}
          </Button>
        </div>
      )}
    </article>
  );
}
