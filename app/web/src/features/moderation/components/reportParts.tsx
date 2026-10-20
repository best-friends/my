// SPDX-License-Identifier: AGPL-3.0-only
// Pieces of the report queue and the report's page: an account as a row shows it (its display name and handle,
// or that it is gone) and an account known by ID only (the moderator who handles a report or wrote a note, or the
// user who appealed an action).
import { useT } from "../../../lib/i18n";
import type { Account } from "../../profile";
import { useViewer } from "../../session";
import { accountName, handleOf, useAccountById } from "../hooks/useReports";

/** An account as its display name over its handle; `undefined` is an account that is gone. */
export function AccountLabel({ account }: { account: Account | undefined }) {
  const t = useT();
  if (account === undefined) return t("client:admin.report.account.deleted");
  // Without a display name, the handle is the name: it is shown once.
  if (account.display_name === "") {
    return (
      <span dir="ltr" className="text-start wrap-anywhere">
        {handleOf(account)}
      </span>
    );
  }
  return (
    <span className="flex min-w-0 flex-col">
      {/* A name the user chose, in any language (web-client.md G9). */}
      <span dir="auto" className="text-start wrap-anywhere">
        {accountName(account)}
      </span>
      <span dir="ltr" className="text-start text-sm wrap-anywhere text-fg-muted">
        {handleOf(account)}
      </span>
    </span>
  );
}

/** A moderator by account ID: "you" for the viewer, else the account's name once loaded. */
export function AssigneeName({ id }: { id: string }) {
  const t = useT();
  const viewer = useViewer();
  if (viewer.status === "signed-in" && viewer.account.id === id) return t("client:admin.report.assignee.you");
  return <AccountByIdLabel id={id} />;
}

/** An account known by ID only (a moderator, or the user who appealed): its name once loaded, or that it is gone. */
export function AccountByIdLabel({ id }: { id: string }) {
  const t = useT();
  const account = useAccountById(id);
  switch (account.status) {
    case "loading":
      return <span className="text-fg-muted">{t("client:admin.report.account.loading")}</span>;
    case "missing":
      return t("client:admin.report.account.deleted");
    case "found":
      return <AccountLabel account={account.account} />;
  }
}
