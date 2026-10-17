// SPDX-License-Identifier: AGPL-3.0-only
// The notifications page below its heading (FR-205): tabs that filter by type, kept in the URL by the
// route (web-client.md S3), and the notifications of the selected filter as a feed (Y6). The tabs follow
// the WAI-ARIA APG tabs pattern with automatic activation (React Aria Tabs); only the selected panel is
// mounted, and switching tabs keeps focus on the tab (the layout moves focus only when the path changes).
import { EmptyState, Feed, Link, RegionError, Spinner, Tab, TabList, TabPanel, Tabs } from "@my/ui";
import { useState } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { type NotificationFilter, notificationFilters } from "../api/notifications";
import { useNotifications } from "../hooks/useNotifications";
import { NotificationItem } from "./NotificationItem";

const filterLabels: Readonly<Record<NotificationFilter, ClientMessageKey>> = {
  all: "client:notifications.filter.all",
  mentions: "client:notifications.filter.mentions",
  follows: "client:notifications.filter.follows",
  boosts: "client:notifications.filter.boosts",
  favourites: "client:notifications.filter.favourites",
};

function isFilter(key: unknown): key is NotificationFilter {
  return notificationFilters.some((filter) => filter === key);
}

export type NotificationsViewProps = {
  /** The selected filter, from the URL. */
  filter: NotificationFilter;
  /** Selects another filter: the route puts it into the URL. */
  onFilterChange: (filter: NotificationFilter) => void;
  /** The ID of the page heading, which names the feed. */
  headingId: string;
};

export function NotificationsView({ filter, onFilterChange, headingId }: NotificationsViewProps) {
  const t = useT();
  return (
    <Tabs
      selectedKey={filter}
      onSelectionChange={(key) => {
        if (isFilter(key)) onFilterChange(key);
      }}
    >
      <TabList aria-label={t("client:notifications.filter.label")}>
        {notificationFilters.map((item) => (
          <Tab key={item} id={item}>
            {t(filterLabels[item])}
          </Tab>
        ))}
      </TabList>
      <TabPanel id={filter}>
        <NotificationList filter={filter} headingId={headingId} />
      </TabPanel>
    </Tabs>
  );
}

type NotificationsErrorProps = { message: string; retry: () => void; isRetrying: boolean };

/**
 * The list's failure in place of the list (web-client.md G16). The failure the page opened with is read, not
 * announced; a retry that fails again is the result of the user's action, and interrupts as an alert.
 */
function NotificationsError({ message, retry, isRetrying }: NotificationsErrorProps) {
  const t = useT();
  const [retried, setRetried] = useState(false);
  return (
    <RegionError
      headingLevel={2}
      heading={t("client:notifications.failed")}
      reason={message}
      retryLabel={t("client:notifications.retry")}
      onRetry={() => {
        setRetried(true);
        retry();
      }}
      isRetrying={isRetrying}
      alert={retried && !isRetrying}
    />
  );
}

function NotificationList({ filter, headingId }: { filter: NotificationFilter; headingId: string }) {
  const t = useT();
  const list = useNotifications(filter);
  switch (list.status) {
    case "signed-out":
      return (
        <div className="flex flex-col items-start gap-2">
          <p className="text-fg-muted">{t("client:notifications.signed_out")}</p>
          <Link href="/login?return_to=/notifications">{t("client:notifications.sign_in")}</Link>
        </div>
      );
    case "loading":
      return (
        <div className="grid place-items-center py-10">
          <Spinner label={t("client:notifications.loading")} />
        </div>
      );
    case "error":
      return <NotificationsError message={list.message} retry={list.retry} isRetrying={list.isRetrying} />;
    case "loaded":
      if (list.items.length === 0) {
        return (
          <EmptyState
            heading={t(filter === "all" ? "client:notifications.empty.all" : "client:notifications.empty.filtered")}
            description={t("client:notifications.empty.description")}
          />
        );
      }
      return (
        <Feed
          aria-labelledby={headingId}
          items={list.items}
          getKey={(notification) => notification.id}
          renderItem={(notification) => <NotificationItem notification={notification} isUnread={list.isUnread(notification)} />}
          isLoading={list.isLoadingMore}
          hasMore={list.hasMore}
          onLoadMore={list.loadMore}
          loadMoreLabel={t("client:notifications.load_more")}
          loadingLabel={t("client:notifications.loading")}
          estimateSize={120}
        />
      );
  }
}
