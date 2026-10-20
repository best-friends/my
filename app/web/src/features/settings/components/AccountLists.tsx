// SPDX-License-Identifier: AGPL-3.0-only
// The screens of the blocked and muted accounts and of the follow requests (FR-302, FR-303): a list of
// accounts, each with its actions, and a button that loads more.
import { Avatar, Button, EmptyState } from "@my/ui";
import { type ReactNode, useRef } from "react";
import { useIntl } from "react-intl";
import { handleOf } from "../../../lib/handle";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import type { Account } from "../api/relationships";
import { type AccountAction, type AccountList, useBlocks, useFollowRequests, useMutes } from "../hooks/useAccountLists";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";

export function BlockedAccounts() {
  const t = useT();
  const { list, unblock } = useBlocks();
  return (
    <AccountListView
      list={list}
      accountOf={(account) => account}
      empty="client:settings.blocks.empty"
      actions={(account, removed) => (
        <ActionButton action={unblock} account={account} onDone={removed} label={t("client:settings.blocks.unblock")} />
      )}
    />
  );
}

export function MutedAccounts() {
  const t = useT();
  const intl = useIntl();
  const { list, unmute } = useMutes();
  return (
    <AccountListView
      list={list}
      accountOf={(mute) => mute.account}
      empty="client:settings.mutes.empty"
      details={(mute) => (
        <>
          {mute.expires_at !== undefined && (
            <span>{t("client:settings.mutes.until", { date: intl.formatDate(mute.expires_at, { dateStyle: "medium", timeStyle: "short" }) })}</span>
          )}
          {mute.notifications && <span>{t("client:settings.mutes.notifications")}</span>}
        </>
      )}
      actions={(account, removed) => (
        <ActionButton action={unmute} account={account} onDone={removed} label={t("client:settings.mutes.unmute")} />
      )}
    />
  );
}

export function FollowRequests() {
  const t = useT();
  const { list, accept, reject } = useFollowRequests();
  return (
    <AccountListView
      list={list}
      accountOf={(account) => account}
      empty="client:settings.follow_requests.empty"
      actions={(account, removed) => (
        <>
          <ActionButton
            action={accept}
            account={account}
            onDone={removed}
            label={t("client:settings.follow_requests.accept")}
            variant="primary"
            isDisabled={reject.pendingId === account.id}
          />
          <ActionButton
            action={reject}
            account={account}
            onDone={removed}
            label={t("client:settings.follow_requests.reject")}
            isDisabled={accept.pendingId === account.id}
          />
        </>
      )}
    />
  );
}

type AccountListViewProps<T> = {
  list: AccountList<T>;
  accountOf: (item: T) => Account;
  empty: ClientMessageKey;
  /** More about the item, under the account's handle. */
  details?: (item: T) => ReactNode;
  /** The buttons of the item; `removed` is called when one takes the account off the list. */
  actions: (account: Account, removed: () => void) => ReactNode;
};

function AccountListView<T>({ list, accountOf, empty, details, actions }: AccountListViewProps<T>) {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const ids = list.items.map((item) => accountOf(item).id);
  const removed = useFocusAfterRemoval(listRef, ids);
  if (list.items.length === 0) return <EmptyState heading={t(empty)} />;
  return (
    <div className="flex flex-col gap-4">
      <ul ref={listRef} className="flex flex-col divide-y divide-border">
        {list.items.map((item) => {
          const account = accountOf(item);
          return (
            <li key={account.id} data-row-id={account.id} className="@container py-3">
              <AccountRow account={account} details={details?.(item)}>
                {actions(account, () => {
                  removed(account.id);
                })}
              </AccountRow>
            </li>
          );
        })}
      </ul>
      {list.hasMore && (
        <div>
          <Button onPress={list.loadMore} isPending={list.isLoadingMore}>
            {t("client:settings.list.more")}
          </Button>
        </div>
      )}
    </div>
  );
}

type AccountRowProps = { account: Account; details: ReactNode; children: ReactNode };

/** An account: its avatar, display name and handle, with its buttons after them (below them on narrow rows). */
function AccountRow({ account, details, children }: AccountRowProps) {
  const name = account.display_name || account.username;
  const handle = handleOf(account.acct);
  return (
    <div className="flex flex-col gap-3 @md:flex-row @md:items-center">
      <div className="flex min-w-0 flex-1 items-center gap-3">
        <Avatar src={account.avatar?.small.url} name={name} decorative />
        <p className="flex min-w-0 flex-col">
          {/* The user's own text, in any language, lined up with the other rows (web-client.md Y10, G9). */}
          <span dir="auto" className="truncate font-semibold align-match-parent">
            {name}
          </span>
          <span dir="ltr" className="truncate text-start text-sm text-fg-muted">
            {handle}
          </span>
          {details !== undefined && <span className="flex flex-col text-sm text-fg-muted">{details}</span>}
        </p>
      </div>
      <div className="flex flex-wrap gap-2">{children}</div>
    </div>
  );
}

type ActionButtonProps = {
  action: AccountAction;
  account: Account;
  onDone: () => void;
  label: string;
  variant?: "primary" | "secondary";
  isDisabled?: boolean;
};

/** A button of a row; it names the account too ("Unblock Bob"), as every row has the same buttons. */
function ActionButton({ action, account, onDone, label, variant = "secondary", isDisabled = false }: ActionButtonProps) {
  const t = useT();
  const name = account.display_name || account.username;
  return (
    <Button
      size="sm"
      variant={variant}
      isPending={action.pendingId === account.id}
      isDisabled={isDisabled}
      aria-label={t("client:settings.list.action", { action: label, name })}
      onPress={() => {
        action.run(account, onDone);
      }}
    >
      {label}
    </Button>
  );
}
