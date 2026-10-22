// SPDX-License-Identifier: AGPL-3.0-only
// An account's admin page below its heading (FR-714, FR-703, FR-713): the account with its state, registration
// and roles; what its actions take away now; its email address and IP addresses, which a local account shows only
// when the moderator presses "show" (each time is recorded in the audit log, ADR-0038); whether its user has
// two-factor authentication, with its removal, which asks first (recorded in the audit log); for manage_login_recovery,
// issuing a password reset link for its user, which asks first, is recorded in the audit log, and shows the link
// once with a copy button; the moderation actions
// on it, each in effect with a lift that asks for an optional reason first; its reports; and the form of a new
// action, which offers a remote account only the kinds a remote account takes. What a change did stays on the
// page in a StatusMessage (role="status") next to what caused it (web-client.md §14). A viewer with manage_roles
// also has the account's roles, to grant and revoke (AccountRoles); one with manage_domains, the refetch of a
// remote account's posts from its server (RefetchStatusesSection).
import { AccountItem, Button, ConfirmDialog, DescriptionItem, DescriptionList, EmptyState, Link, ReasonField, RegionError, Spinner, StatusMessage } from "@my/ui";
import { useId, useRef, useState } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFocusHeadingAfterLift } from "../../../lib/useFocusAfterRemoval";
import type { AdminAccountDetail, AdminAccountPrivateData } from "../api/accounts";
import { type AccountAction, accountActionTextLimit, type Report } from "../api/reports";
import {
  adminAccountRegistrationStateLabels,
  adminAccountRestrictionLabels,
  adminAccountStateLabels,
  type LiftAccountAction,
  useAdminAccount,
  useLiftAccountAction,
  usePrivateData,
  useCopyPasswordResetLink,
  useIssuePasswordResetLink,
  useRemoveTwoFactor,
} from "../hooks/useAdminAccounts";
import { useHasPermission } from "../hooks/usePermissions";
import {
  accountActionKindLabels,
  accountActionStateLabels,
  accountName,
  handleOf,
  isolate,
  reportCategoryLabels,
  reportOutcomeLabels,
  useTargetActions,
} from "../hooks/useReports";
import { AccountActionForm } from "./AccountActionForm";
import { AccountRoles } from "./AccountRoles";
import { AccountOrigin } from "./AccountSearch";
import { DateTime, DetailSection, RecordList, RequirePermission, useFocusAfterChange } from "./parts";
import { RefetchStatusesSection } from "./RefetchStatuses";
import { AccountLabel, AssigneeName } from "./reportParts";
import { imageSources } from "../../../lib/images";

export function AccountDetailView({ id }: { id: string }) {
  return (
    <RequirePermission permission="manage_accounts">
      <AccountPage id={id} />
    </RequirePermission>
  );
}

function AccountPage({ id }: { id: string }) {
  const t = useT();
  const account = useAdminAccount(id);
  const [issued, setIssued] = useState<string | undefined>(undefined);
  const handle = handleOf(account.account);
  const canManageRoles = useHasPermission("manage_roles");
  // Refetching a remote account's posts from its server (FR-407) is for a viewer with manage_domains.
  const canRefetch = useHasPermission("manage_domains");
  // Recovering a user's sign-in is server administration (ADR-0058).
  const canRecoverSignIn = useHasPermission("manage_login_recovery");
  return (
    <div className="flex flex-col gap-8">
      <DetailSection title={t("client:admin.account.summary.title")}>
        <AccountSummary account={account} />
      </DetailSection>
      <DetailSection title={t("client:admin.account.restrictions.title")}>
        {account.restrictions.length === 0 ? (
          <p>{t("client:admin.account.restrictions.none")}</p>
        ) : (
          <ul className="flex list-inside list-disc flex-col gap-1">
            {account.restrictions.map((restriction) => (
              <li key={restriction}>{t(adminAccountRestrictionLabels[restriction])}</li>
            ))}
          </ul>
        )}
      </DetailSection>
      {!account.local && account.state !== "deleted" && canRefetch && <RefetchStatusesSection target={{ kind: "account", id: account.id }} />}
      {account.local && canManageRoles && <AccountRoles accountId={account.id} handle={isolate(handle)} />}
      {account.local && account.two_factor_enabled !== undefined && (
        <TwoFactorSection id={account.id} enabled={account.two_factor_enabled} handle={isolate(handle)} />
      )}
      {account.local && account.two_factor_enabled !== undefined && canRecoverSignIn && <PasswordResetSection id={account.id} handle={isolate(handle)} />}
      {account.local && <PrivateDataSection id={account.id} />}
      <ActionHistory account={account} />
      <DetailSection title={t("client:admin.account.reports.title")}>
        <AccountReports reports={account.reports} />
      </DetailSection>
      <DetailSection title={t("client:admin.account.action.title")}>
        <StatusMessage tone="success">{issued}</StatusMessage>
        {account.state === "deleted" ? (
          <p>{t("client:admin.account.action.deleted")}</p>
        ) : (
          <>
            <p className="text-sm text-fg-muted">{t("client:admin.account.action.posts")}</p>
            <AccountActionForm
              target={{ accountId: account.id, remote: !account.local, statusIds: [], category: "other", hasEmail: account.email_verified !== undefined }}
              handle={handle}
              posts={[]}
              onIssued={(action) => {
                setIssued(t("client:admin.account.action.issued", { kind: t(accountActionKindLabels[action.kind]) }));
              }}
            />
          </>
        )}
      </DetailSection>
    </div>
  );
}

function AccountSummary({ account }: { account: AdminAccountDetail }) {
  const t = useT();
  const canAudit = useHasPermission("view_audit");
  const { registration } = account;
  return (
    <div className="flex flex-col gap-3">
      <AccountItem
        name={accountName(account.account)}
        handle={account.account.display_name === "" ? undefined : handleOf(account.account)}
        avatar={imageSources(account.account.avatar)}
      />
      <ul className="flex flex-wrap gap-x-4 gap-y-1">
        {account.state !== "deleted" && (
          <li>
            <Link href={`/@${account.account.acct}`} variant="standalone">
              {t("client:admin.account.profile")}
            </Link>
          </li>
        )}
        {canAudit && (
          <li>
            <Link href={`/moderation/audit-log?target_kind=account&target=${encodeURIComponent(account.id)}`} variant="standalone">
              {t("client:admin.account.audit")}
            </Link>
          </li>
        )}
      </ul>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.accounts.state")}>
          <span className="font-semibold">{t(adminAccountStateLabels[account.state])}</span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.accounts.origin")}>
          <AccountOrigin account={account} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.accounts.created_at")}>
          <DateTime value={account.account.created_at} />
        </DescriptionItem>
        {account.registration_state !== undefined && (
          <DescriptionItem term={t("client:admin.accounts.registration")}>{t(adminAccountRegistrationStateLabels[account.registration_state])}</DescriptionItem>
        )}
        {registration !== undefined && (
          <>
            <DescriptionItem term={t("client:admin.account.registration.submitted_at")}>
              <DateTime value={registration.submitted_at} />
            </DescriptionItem>
            <DescriptionItem term={t("client:admin.account.registration.state_changed_at")}>
              <DateTime value={registration.state_changed_at} />
            </DescriptionItem>
            <DescriptionItem term={t("client:admin.account.registration.invite")}>
              {registration.invite_id === undefined ? t("client:admin.account.registration.invite.none") : t("client:admin.account.registration.invite.used")}
            </DescriptionItem>
          </>
        )}
        {account.local && (
          <>
            <DescriptionItem term={t("client:admin.account.email_verified")}>
              {account.email_verified === undefined
                ? t("client:admin.account.email.none")
                : account.email_verified
                  ? t("client:admin.account.email_verified.yes")
                  : t("client:admin.account.email_verified.no")}
            </DescriptionItem>
            <DescriptionItem term={t("client:admin.account.roles")}>
              {account.roles.length === 0 ? (
                t("client:admin.account.roles.none")
              ) : (
                // Role keys are identifiers, left to right in every locale.
                <span dir="ltr" className="wrap-anywhere">
                  {account.roles.join(", ")}
                </span>
              )}
            </DescriptionItem>
          </>
        )}
      </DescriptionList>
      {registration?.reason !== undefined && (
        <DescriptionList>
          <DescriptionItem term={t("client:admin.account.registration.reason")}>
            {/* Written by the applicant, in any language and direction (web-client.md Y10). */}
            <span dir="auto" className="block text-start wrap-anywhere whitespace-pre-wrap">
              {registration.reason}
            </span>
          </DescriptionItem>
        </DescriptionList>
      )}
    </div>
  );
}

/**
 * Whether the user of a local account signs in with a code too, and removing it for a user who lost their
 * authenticator app and recovery codes, which asks first; the removal is recorded in the audit log.
 */
function TwoFactorSection({ id, enabled, handle }: { id: string; enabled: boolean; handle: string }) {
  const t = useT();
  const headingRef = useRef<HTMLHeadingElement>(null);
  // The button goes away with the removal: focus moves to the region's heading.
  const focusHeading = useFocusAfterChange(headingRef, enabled);
  const remove = useRemoveTwoFactor(id, handle, focusHeading);
  // Removing it is server administration (ADR-0058): moderators see the state but not the button.
  const canRemove = useHasPermission("manage_login_recovery");
  return (
    <DetailSection title={t("client:admin.account.two_factor.title")} headingRef={headingRef}>
      <StatusMessage tone="success">{remove.status}</StatusMessage>
      <p>{enabled ? t("client:admin.account.two_factor.on") : t("client:admin.account.two_factor.off")}</p>
      {enabled && canRemove && (
        <div className="flex flex-col items-start gap-3">
          <p className="text-sm text-fg-muted">{t("client:admin.account.two_factor.remove.description")}</p>
          <Button onPress={remove.open} isPending={remove.isPending}>
            {t("client:admin.account.two_factor.remove")}
          </Button>
        </div>
      )}
      <ConfirmDialog
        isOpen={remove.isOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) remove.cancel();
        }}
        title={t("client:admin.account.two_factor.confirm.title", { handle })}
        description={t("client:admin.account.two_factor.confirm.description")}
        confirmLabel={t("client:admin.account.two_factor.remove")}
        cancelLabel={t("client:admin.account_actions.confirm.cancel")}
        onConfirm={() => {
          remove.confirm();
        }}
        isPending={remove.isPending}
        {...(remove.error === undefined ? {} : { errorMessage: t("client:admin.account.two_factor.failed", { reason: remove.error }) })}
      />
    </DetailSection>
  );
}

/**
 * Issuing a link that sets a new password for the user of a local account, to hand to them when email does not
 * reach them (FR-002); it asks first, and the issuance is recorded in the audit log. The link is shown once, with
 * a button that copies it; issuing again replaces it. Like removing two-factor authentication, it lets the holder of
 * the link into the account, so it is server administration (ADR-0058): the region is only for manage_login_recovery.
 */
function PasswordResetSection({ id, handle }: { id: string; handle: string }) {
  const t = useT();
  const intl = useIntl();
  const issue = useIssuePasswordResetLink(id, handle);
  const copy = useCopyPasswordResetLink();
  const linkLabelId = useId();
  const { link } = issue;
  return (
    <DetailSection title={t("client:admin.account.password_reset.title")}>
      <StatusMessage tone="success">{issue.status}</StatusMessage>
      {link !== undefined && (
        <div className="flex flex-col gap-2 rounded-md border border-border p-3">
          <p id={linkLabelId} className="font-medium">
            {t("client:admin.account.password_reset.link")}
          </p>
          <p className="text-sm text-fg-muted">
            {t("client:admin.account.password_reset.link.notice", { expiresAt: intl.formatDate(link.expires_at, { dateStyle: "medium", timeStyle: "short" }) })}
          </p>
          {/* A URL reads left to right in every locale. */}
          <p aria-labelledby={linkLabelId} dir="ltr" className="font-mono wrap-anywhere select-all">
            {link.url}
          </p>
          <div>
            <Button
              size="sm"
              onPress={() => {
                copy(link.url);
              }}
            >
              {t("client:admin.account.password_reset.copy")}
            </Button>
          </div>
        </div>
      )}
      <div className="flex flex-col items-start gap-3">
        <p className="text-sm text-fg-muted">{t("client:admin.account.password_reset.description")}</p>
        <Button onPress={issue.open} isPending={issue.isPending}>
          {t("client:admin.account.password_reset.issue")}
        </Button>
      </div>
      <ConfirmDialog
        isOpen={issue.isOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) issue.cancel();
        }}
        title={t("client:admin.account.password_reset.confirm.title", { handle })}
        description={t("client:admin.account.password_reset.confirm.description")}
        confirmLabel={t("client:admin.account.password_reset.issue")}
        cancelLabel={t("client:admin.account_actions.confirm.cancel")}
        tone="primary"
        onConfirm={() => {
          issue.confirm();
        }}
        isPending={issue.isPending}
        {...(issue.error === undefined ? {} : { errorMessage: t("client:admin.account.password_reset.failed", { reason: issue.error }) })}
      />
    </DetailSection>
  );
}

function PrivateDataSection({ id }: { id: string }) {
  const t = useT();
  const headingRef = useRef<HTMLHeadingElement>(null);
  const [revealed, setRevealed] = useState(false);
  const focusHeading = useFocusAfterChange(headingRef, revealed);
  const privateData = usePrivateData(id, () => {
    setRevealed(true);
    focusHeading();
  });
  return (
    <DetailSection title={t("client:admin.account.private.title")} headingRef={headingRef}>
      <StatusMessage>{privateData.data === undefined ? undefined : t("client:admin.account.private.shown")}</StatusMessage>
      {privateData.data === undefined ? (
        <div className="flex flex-col items-start gap-3">
          <p className="text-sm text-fg-muted">{t("client:admin.account.private.description")}</p>
          <Button onPress={privateData.reveal} isPending={privateData.isPending}>
            {t("client:admin.account.private.reveal")}
          </Button>
        </div>
      ) : (
        <PrivateData data={privateData.data} />
      )}
    </DetailSection>
  );
}

/** The search of the accounts with `query`, from the first page. */
function searchHref(query: Record<string, string>): string {
  return `/moderation/accounts?${new URLSearchParams(query).toString()}`;
}

/** An IP address, which reads left to right in every locale. */
function IpAddress({ ip }: { ip: string | undefined }) {
  const t = useT();
  if (ip === undefined) return t("client:admin.account.private.unknown");
  return (
    <span dir="ltr" className="wrap-anywhere">
      {ip}
    </span>
  );
}

function PrivateData({ data }: { data: AdminAccountPrivateData }) {
  const t = useT();
  const emailDomain = data.email?.split("@").at(-1);
  return (
    <div className="flex flex-col gap-6">
      <DescriptionList columns={2}>
        <DescriptionItem term={t("client:admin.account.private.email")}>
          {data.email === undefined ? (
            t("client:admin.account.email.none")
          ) : (
            <span className="flex flex-col items-start gap-1">
              <span dir="ltr" className="wrap-anywhere">
                {data.email}
              </span>
              <span className="text-sm text-fg-muted">
                {data.email_verified === true ? t("client:admin.account.email_verified.yes") : t("client:admin.account.email_verified.no")}
              </span>
              {emailDomain !== undefined && emailDomain !== "" && (
                <Link href={searchHref({ email_domain: emailDomain })} variant="standalone" className="text-sm">
                  {t("client:admin.account.private.search_email_domain", { domain: `⁦${emailDomain}⁩` })}
                </Link>
              )}
            </span>
          )}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.account.private.registration_ip")}>
          {data.registration_ip === undefined ? (
            t("client:admin.account.private.unknown")
          ) : (
            <span className="flex flex-col items-start gap-1">
              <IpAddress ip={data.registration_ip} />
              {data.registered_at !== undefined && (
                <span className="text-sm text-fg-muted">
                  <DateTime value={data.registered_at} />
                </span>
              )}
            </span>
          )}
        </DescriptionItem>
      </DescriptionList>
      <div className="flex flex-col gap-3">
        <h3 className="font-semibold">{t("client:admin.account.private.sign_ins")}</h3>
        {data.sign_ins.length === 0 ? (
          <p className="text-fg-muted">{t("client:admin.account.private.sign_ins.none")}</p>
        ) : (
          <ul className="flex flex-col divide-y divide-border">
            {data.sign_ins.map((signIn, index) => (
              // A sign-in has no ID: its time and address keep the rows of one answer apart, which never changes.
              <li key={`${signIn.signed_in_at} ${signIn.ip ?? ""} ${String(index)}`} className="py-3">
                <DescriptionList columns={3}>
                  <DescriptionItem term={t("client:admin.account.private.signed_in_at")}>
                    <DateTime value={signIn.signed_in_at} />
                  </DescriptionItem>
                  <DescriptionItem term={t("client:admin.account.private.ip")}>
                    <IpAddress ip={signIn.ip} />
                  </DescriptionItem>
                  <DescriptionItem term={t("client:admin.account.private.user_agent")}>
                    {signIn.user_agent === undefined ? (
                      t("client:admin.account.private.unknown")
                    ) : (
                      <span dir="ltr" className="wrap-anywhere">
                        {signIn.user_agent}
                      </span>
                    )}
                  </DescriptionItem>
                </DescriptionList>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="flex flex-col gap-3">
        <h3 className="font-semibold">{t("client:admin.account.private.same_ip")}</h3>
        {data.same_ip_accounts.length === 0 ? (
          <p className="text-fg-muted">{t("client:admin.account.private.same_ip.none")}</p>
        ) : (
          <ul className="flex flex-col divide-y divide-border">
            {data.same_ip_accounts.map((group) => (
              <li key={group.ip} className="flex flex-col gap-2 py-3">
                <h4 className="font-semibold">
                  <IpAddress ip={group.ip} />
                </h4>
                <ul className="flex flex-col gap-2">
                  {group.accounts.map((other) => (
                    <li key={other.id} className="flex flex-wrap items-baseline gap-x-3">
                      <Link href={`/moderation/accounts/${encodeURIComponent(other.id)}`} variant="standalone">
                        <AccountLabel account={other.account} />
                      </Link>
                      <span className="text-sm text-fg-muted">{t(adminAccountStateLabels[other.state])}</span>
                    </li>
                  ))}
                </ul>
                <Link href={searchHref({ ip: group.ip })} variant="standalone" className="text-sm">
                  {t("client:admin.account.private.search_ip", { ip: `⁦${group.ip}⁩` })}
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

function ActionHistory({ account }: { account: AdminAccountDetail }) {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const actions = useTargetActions(account.id);
  const [lifted, setLifted] = useState<string | undefined>(undefined);
  const keepFocus = useFocusHeadingAfterLift(listRef);
  const lift = useLiftAccountAction(account.id, (action) => {
    keepFocus(action.id);
    setLifted(t("client:admin.account.actions.lifted", { kind: t(accountActionKindLabels[action.kind]) }));
  });
  const { confirmation } = lift;
  const handle = isolate(handleOf(account.account));
  return (
    <DetailSection title={t("client:admin.account.actions.title")}>
      <StatusMessage tone="success">{lifted}</StatusMessage>
      {actions.isLoading ? (
        <Spinner size="sm" label={t("client:admin.report.history.loading")} />
      ) : actions.items.length === 0 && actions.error !== undefined ? (
        <RegionError heading={t("client:admin.report.history.failed")} headingLevel={3} reason={actions.error} />
      ) : (
        <RecordList
          list={actions}
          getId={(action) => action.id}
          listRef={listRef}
          empty={<EmptyState headingLevel={3} heading={t("client:admin.report.history.empty")} />}
          moreError={actions.error !== undefined && <RegionError heading={t("client:admin.report.history.failed")} reason={actions.error} />}
        >
          {(action) => <ActionRow action={action} local={account.local} lift={lift} />}
        </RecordList>
      )}
      <ConfirmDialog
        isOpen={lift.isOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) lift.cancel();
        }}
        title={t("client:admin.account.actions.confirm_lift.title", {
          kind: confirmation === null ? "" : t(accountActionKindLabels[confirmation.action.kind]),
          handle,
        })}
        description={t("client:admin.account.actions.confirm_lift.description")}
        confirmLabel={t("client:admin.account.actions.lift")}
        cancelLabel={t("client:admin.account_actions.confirm.cancel")}
        tone="primary"
        onConfirm={() => {
          lift.confirm();
        }}
        isPending={lift.isPending}
        {...(lift.error === undefined ? {} : { errorMessage: t("client:admin.account.actions.lift_failed", { reason: lift.error }) })}
      >
        <ReasonField
          label={t("client:admin.account.actions.lift_reason")}
          description={t("client:admin.account.actions.lift_reason.description")}
          value={confirmation?.reason ?? ""}
          onChange={lift.changeReason}
          characterLimit={accountActionTextLimit}
          formatRemaining={(remaining) =>
            remaining < 0 ? t("client:admin.account_actions.characters_over", { count: -remaining }) : t("client:admin.account_actions.characters_left", { count: remaining })
          }
          errorMessage={t("client:admin.account_actions.too_long", { max: accountActionTextLimit })}
        />
      </ConfirmDialog>
    </DetailSection>
  );
}

const issuerLabels: Readonly<Record<AccountAction["issuer"], ClientMessageKey>> = {
  moderator: "client:admin.account.actions.issuer.cli",
  system: "client:admin.account.actions.issuer.system",
  remote: "client:admin.account.actions.issuer.remote",
};

function ActionRow({ action, local, lift }: { action: AccountAction; local: boolean; lift: LiftAccountAction }) {
  const t = useT();
  const intl = useIntl();
  const kind = t(accountActionKindLabels[action.kind]);
  // A ban is never lifted (the API refuses it); it is undone only by overturning its appeal.
  const liftable = action.state === "active" && action.kind !== "ban";
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col items-start gap-2 @md:flex-row @md:items-center @md:justify-between">
        <h3 tabIndex={-1} className="font-semibold outline-none">
          {kind}
        </h3>
        {liftable && (
          <Button
            size="sm"
            isPending={lift.isPending && lift.confirmation?.action.id === action.id}
            aria-label={t("client:admin.account.actions.lift_action", {
              kind,
              date: intl.formatDate(action.issued_at, { dateStyle: "medium", timeStyle: "short" }),
            })}
            onPress={() => {
              lift.open(action);
            }}
          >
            {t("client:admin.account.actions.lift")}
          </Button>
        )}
      </div>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.account_actions.state")}>
          <span className={action.state === "active" ? "font-semibold" : undefined}>{t(accountActionStateLabels[action.state])}</span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.account_actions.issued_at")}>
          <DateTime value={action.issued_at} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.account_actions.expires_at")}>
          {action.expires_at === undefined ? t("client:admin.account_actions.duration.indefinite") : <DateTime value={action.expires_at} />}
        </DescriptionItem>
        {action.lifted_at !== undefined && (
          <DescriptionItem term={t("client:admin.account.actions.lifted_at")}>
            <DateTime value={action.lifted_at} />
          </DescriptionItem>
        )}
        <DescriptionItem term={t("client:admin.account_actions.category")}>{t(reportCategoryLabels[action.reason_category])}</DescriptionItem>
        <DescriptionItem term={t("client:admin.account.actions.issuer")}>
          {action.issued_by_account_id === undefined ? t(issuerLabels[action.issuer]) : <AssigneeName id={action.issued_by_account_id} />}
        </DescriptionItem>
        {local && (
          <DescriptionItem term={t("client:admin.account.actions.notified")}>
            {action.notify_target ? t("client:admin.account.actions.notified.yes") : t("client:admin.account.actions.notified.no")}
          </DescriptionItem>
        )}
        {action.status_ids !== undefined && (
          <DescriptionItem term={t("client:admin.report.posts")}>
            <span className="tabular-nums">{t("client:admin.report.posts.count", { count: action.status_ids.length })}</span>
          </DescriptionItem>
        )}
        {action.report_id !== undefined && (
          <DescriptionItem term={t("client:admin.account_actions.report")}>
            <Link href={`/moderation/reports/${encodeURIComponent(action.report_id)}`} variant="inline">
              {t("client:admin.account_actions.report.other")}
            </Link>
          </DescriptionItem>
        )}
      </DescriptionList>
      {(action.public_reason !== undefined || action.private_note !== undefined || action.lift_reason !== undefined) && (
        <DescriptionList>
          {action.public_reason !== undefined && (
            <DescriptionItem term={t("client:admin.account_actions.public_reason")}>
              {/* Written by a moderator, in any language. */}
              <span dir="auto" className="block text-start wrap-anywhere whitespace-pre-wrap">
                {action.public_reason}
              </span>
            </DescriptionItem>
          )}
          {action.private_note !== undefined && (
            <DescriptionItem term={t("client:admin.account_actions.private_note")}>
              <span dir="auto" className="block text-start wrap-anywhere whitespace-pre-wrap">
                {action.private_note}
              </span>
            </DescriptionItem>
          )}
          {action.lift_reason !== undefined && (
            <DescriptionItem term={t("client:admin.account.actions.lift_reason")}>
              <span dir="auto" className="block text-start wrap-anywhere whitespace-pre-wrap">
                {action.lift_reason}
              </span>
            </DescriptionItem>
          )}
        </DescriptionList>
      )}
    </div>
  );
}

const reportStateLabels: Readonly<Record<Report["state"], ClientMessageKey>> = {
  open: "client:admin.reports.filter.open",
  resolved: "client:admin.reports.filter.resolved",
};

function AccountReports({ reports }: { reports: readonly Report[] }) {
  const t = useT();
  const intl = useIntl();
  const canHandle = useHasPermission("manage_reports");
  if (reports.length === 0) return <EmptyState headingLevel={3} heading={t("client:admin.account.reports.empty")} />;
  return (
    <ul className="flex flex-col divide-y divide-border">
      {reports.map((report) => {
        const title = t("client:admin.account.reports.item", { date: intl.formatDate(report.filed_at, { dateStyle: "medium", timeStyle: "short" }) });
        return (
          <li key={report.id} className="@container flex flex-col gap-2 py-3">
            <h3 className="font-semibold">
              {canHandle ? (
                <Link href={`/moderation/reports/${encodeURIComponent(report.id)}`} variant="standalone">
                  {title}
                </Link>
              ) : (
                title
              )}
            </h3>
            <DescriptionList columns={3}>
              <DescriptionItem term={t("client:admin.report.state")}>
                {report.outcome === undefined ? t(reportStateLabels[report.state]) : t(reportOutcomeLabels[report.outcome])}
              </DescriptionItem>
              <DescriptionItem term={t("client:admin.report.category")}>{t(reportCategoryLabels[report.category])}</DescriptionItem>
              <DescriptionItem term={t("client:admin.report.reporter")}>
                <AccountLabel account={report.reporter_account} />
              </DescriptionItem>
            </DescriptionList>
            {report.comment !== "" && (
              // Written by the reporter, in any language.
              <p dir="auto" className="line-clamp-2 text-start wrap-anywhere whitespace-pre-wrap">
                {report.comment}
              </p>
            )}
          </li>
        );
      })}
    </ul>
  );
}
