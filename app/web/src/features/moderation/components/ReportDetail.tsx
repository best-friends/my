// SPDX-License-Identifier: AGPL-3.0-only
// A report's page below its heading: everything a moderator needs to handle one report on one page (NFR-MOD-4,
// FR-710). The report itself, the posts it points at, its target with the target's history (its other reports
// and its moderation actions), the moderator who handles it, the moderators' notes, and handling it: a
// moderation action on the target that resolves it, resolving it without one, or reopening it. What a change
// did stays on the page in a StatusMessage (role="status") next to what caused it; failures are toasts, and a
// failed note stays under its field (web-client.md E2).
import {
  AccountItem,
  Button,
  DescriptionItem,
  DescriptionList,
  EmptyState,
  Link,
  ReasonField,
  RegionError,
  Spinner,
  StatusMessage,
} from "@my/ui";
import { type ReactNode, type RefObject, useId, useRef, useState } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { StatusCard } from "../../status";
import { type AccountAction, type ReportDetail, reportNoteLimit } from "../api/reports";
import { useHasPermission } from "../hooks/usePermissions";
import { useRefetch } from "../hooks/useRefetch";
import {
  accountActionKindLabels,
  accountName,
  accountActionStateLabels,
  handleOf,
  isRemote,
  type ReportedStatus,
  reportCategoryLabels,
  reportedStatuses,
  reportOutcomeLabels,
  useCitedRules,
  useReport,
  useReportChanges,
  useReportNoteForm,
  useTargetActions,
} from "../hooks/useReports";
import { AccountActionForm } from "./AccountActionForm";
import { DateTime, RecordList, RequirePermission, useFocusAfterChange } from "./parts";
import { AccountLabel, AssigneeName } from "./reportParts";
import { imageSources } from "../../../lib/images";

export function ReportDetailView({ id }: { id: string }) {
  return (
    <RequirePermission permission="manage_reports">
      <ReportPage id={id} />
    </RequirePermission>
  );
}

/** The region of the page that a message belongs to: the one whose button caused it. */
type Message = { region: "assignee" | "resolution"; text: string };

function ReportPage({ id }: { id: string }) {
  const t = useT();
  const report = useReport(id);
  const canAct = useHasPermission("manage_accounts");
  const canAudit = useHasPermission("view_audit");
  const posts = reportedStatuses(report);
  // Deleted posts stay to be reviewed, but no action can delete them or mark them sensitive.
  const actionable = posts.flatMap((post) => (post.status === "found" && post.deletedAt === undefined ? [post.value] : []));
  const [message, setMessage] = useState<Message | undefined>(undefined);
  const resolutionHeadingRef = useRef<HTMLHeadingElement>(null);
  const focusResolution = useFocusAfterChange(resolutionHeadingRef, report.state);
  const changes = useReportChanges(id, (change, text) => {
    const isResolution = change.operation === "resolve" || change.operation === "reopen";
    setMessage({ region: isResolution ? "resolution" : "assignee", text });
    if (isResolution) focusResolution();
  });
  const target = report.target_account;
  return (
    <div className="flex flex-col gap-8">
      <Section title={t("client:admin.report.summary.title")}>
        <ReportSummary report={report} />
      </Section>
      <Section title={t("client:admin.report.posts.title")}>
        <ReportedPosts posts={posts} />
      </Section>
      <Section title={t("client:admin.report.target.title")}>
        {target === undefined ? (
          <p>{t("client:admin.report.target.deleted")}</p>
        ) : (
          <div className="flex flex-col gap-3">
            <AccountItem
              name={accountName(target)}
              handle={target.display_name === "" ? undefined : handleOf(target)}
              avatar={imageSources(target.avatar)}
            />
            <ul className="flex flex-wrap gap-x-4 gap-y-1">
              <li>
                <Link href={`/@${target.acct}`} variant="standalone">
                  {t("client:admin.report.target.profile")}
                </Link>
              </li>
              {canAct && (
                <li>
                  <Link href={`/moderation/accounts/${encodeURIComponent(target.id)}`} variant="standalone">
                    {t("client:admin.report.target.admin")}
                  </Link>
                </li>
              )}
              {canAudit && (
                <li>
                  <Link href={`/moderation/audit-log?target_kind=account&target=${encodeURIComponent(target.id)}`} variant="standalone">
                    {t("client:admin.report.target.audit")}
                  </Link>
                </li>
              )}
            </ul>
          </div>
        )}
        <DescriptionList columns={2}>
          <DescriptionItem term={t("client:admin.report.target.reports")}>
            <span className="tabular-nums">{t("client:admin.report.target.reports.count", { count: report.target_history.reports_count })}</span>
          </DescriptionItem>
          <DescriptionItem term={t("client:admin.report.target.actions")}>
            <span className="tabular-nums">{t("client:admin.report.target.actions.count", { count: report.target_history.actions_count })}</span>
          </DescriptionItem>
        </DescriptionList>
      </Section>
      {canAct && target !== undefined && (
        <Section title={t("client:admin.report.history.title")}>
          <TargetActions accountId={target.id} reportId={report.id} />
        </Section>
      )}
      <Section title={t("client:admin.report.assignee.title")}>
        <Assignee
          report={report}
          pending={changes.pending !== undefined}
          onAssign={changes.assignToSelf}
          onUnassign={changes.unassign}
          isAssigning={changes.pending?.operation === "assign_to_self"}
          isUnassigning={changes.pending?.operation === "unassign"}
        />
        <StatusMessage>{message?.region === "assignee" ? message.text : undefined}</StatusMessage>
      </Section>
      <Section title={t("client:admin.report.notes.title")}>
        <Notes report={report} />
      </Section>
      <Section title={t("client:admin.report.resolution.title")} headingRef={resolutionHeadingRef}>
        <StatusMessage>{message?.region === "resolution" ? message.text : undefined}</StatusMessage>
        {report.state === "resolved" ? (
          <div className="flex flex-col items-start gap-3">
            <DescriptionList columns={2}>
              <DescriptionItem term={t("client:admin.report.outcome")}>
                {report.outcome === undefined ? t("client:admin.reports.filter.resolved") : t(reportOutcomeLabels[report.outcome])}
              </DescriptionItem>
              <DescriptionItem term={t("client:admin.report.resolved_at")}>
                <DateTime value={report.state_changed_at} />
              </DescriptionItem>
            </DescriptionList>
            <p className="text-sm text-fg-muted">{t("client:admin.report.reopen.description")}</p>
            <Button onPress={changes.reopen} isPending={changes.pending?.operation === "reopen"}>
              {t("client:admin.report.reopen")}
            </Button>
          </div>
        ) : (
          <div className="flex flex-col gap-8">
            <div className="flex flex-col gap-3">
              <h3 className="font-semibold">{t("client:admin.report.action.title")}</h3>
              {target === undefined ? (
                <p>{t("client:admin.report.action.deleted")}</p>
              ) : canAct ? (
                <AccountActionForm
                  target={{
                    accountId: target.id,
                    remote: isRemote(target),
                    reportId: report.id,
                    statusIds: actionable.map((post) => post.id),
                    category: report.category,
                  }}
                  handle={handleOf(target)}
                  posts={actionable}
                  onIssued={(action) => {
                    setMessage({ region: "resolution", text: t("client:admin.report.action.issued", { kind: t(accountActionKindLabels[action.kind]) }) });
                    focusResolution();
                  }}
                />
              ) : (
                <p>{t("client:admin.report.action.missing_permission")}</p>
              )}
            </div>
            <div className="flex flex-col items-start gap-3">
              <h3 className="font-semibold">{t("client:admin.report.resolve.title")}</h3>
              <p className="text-sm text-fg-muted">{t("client:admin.report.resolve.description")}</p>
              <div className="flex flex-wrap gap-2">
                <Button
                  onPress={() => {
                    changes.resolve("dismissed");
                  }}
                  isPending={changes.pending?.operation === "resolve" && changes.pending.outcome === "dismissed"}
                >
                  {t("client:admin.report.resolve.dismissed")}
                </Button>
                <Button
                  onPress={() => {
                    changes.resolve("actioned");
                  }}
                  isPending={changes.pending?.operation === "resolve" && changes.pending.outcome === "actioned"}
                >
                  {t("client:admin.report.resolve.actioned")}
                </Button>
              </div>
            </div>
          </div>
        )}
      </Section>
    </div>
  );
}

/** A titled region of the page: an `h2` that names a `<section>`, which takes focus when the region changes under it. */
function Section({ title, headingRef, children }: { title: ReactNode; headingRef?: RefObject<HTMLHeadingElement | null>; children: ReactNode }) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 ref={headingRef} id={headingId} tabIndex={-1} className="text-lg font-semibold outline-none">
        {title}
      </h2>
      {children}
    </section>
  );
}

const stateLabels: Readonly<Record<ReportDetail["state"], ClientMessageKey>> = {
  open: "client:admin.reports.filter.open",
  resolved: "client:admin.reports.filter.resolved",
};

function ReportSummary({ report }: { report: ReportDetail }) {
  const t = useT();
  const rules = useCitedRules(report.rule_ids);
  return (
    <div className="flex flex-col gap-3">
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.report.state")}>
          <span className="font-semibold">{t(stateLabels[report.state])}</span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.report.category")}>{t(reportCategoryLabels[report.category])}</DescriptionItem>
        <DescriptionItem term={t("client:admin.report.filed_at")}>
          <DateTime value={report.filed_at} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.report.reporter")}>
          <AccountLabel account={report.reporter_account} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.report.origin")}>
          {report.uri === undefined ? t("client:admin.report.origin.local") : t("client:admin.report.origin.remote")}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.report.forwarded")}>
          {report.forwarded ? t("client:admin.report.forwarded.yes") : t("client:admin.report.forwarded.no")}
        </DescriptionItem>
      </DescriptionList>
      {report.rule_ids.length > 0 && (
        <DescriptionList>
          <DescriptionItem term={t("client:admin.report.rules")}>
            {rules === null ? (
              <Spinner size="sm" label={t("client:admin.report.rules.loading")} />
            ) : (
              <ul className="flex list-inside list-disc flex-col gap-1">
                {rules.map((rule) =>
                  rule.text === null ? (
                    <li key={rule.id} className="text-fg-muted">
                      {t("client:admin.report.rules.retired")}
                    </li>
                  ) : (
                    <li key={rule.id} lang="" dir="auto">
                      {rule.text}
                    </li>
                  ),
                )}
              </ul>
            )}
          </DescriptionItem>
        </DescriptionList>
      )}
      <DescriptionList>
        <DescriptionItem term={t("client:admin.report.comment")}>
          {report.comment === "" ? (
            <span className="text-fg-muted">{t("client:admin.report.comment.none")}</span>
          ) : (
            // Written by the reporter, in any language (Y10).
            <span dir="auto" className="block text-start wrap-anywhere whitespace-pre-wrap">
              {report.comment}
            </span>
          )}
        </DescriptionItem>
      </DescriptionList>
    </div>
  );
}

function ReportedPosts({ posts }: { posts: readonly ReportedStatus[] }) {
  const t = useT();
  const intl = useIntl();
  // Refetching a remote post from its server (FR-407) is for a viewer with manage_domains.
  const canRefetch = useHasPermission("manage_domains");
  const refetch = useRefetch();
  if (posts.length === 0) return <p>{t("client:admin.report.posts.none")}</p>;
  return (
    <div className="flex flex-col gap-3">
      {canRefetch && <StatusMessage tone="success">{refetch.status}</StatusMessage>}
      <ul className="flex flex-col divide-y divide-border rounded-lg border border-border">
        {posts.map((post) => (
          <li key={post.id}>
            {post.status === "found" ? (
              <>
                {post.deletedAt !== undefined && (
                  <p className="px-4 pt-3 text-sm font-semibold text-fg-muted">
                    {t("client:admin.report.posts.deleted", { date: intl.formatDate(post.deletedAt, { dateStyle: "medium", timeStyle: "short" }) })}
                  </p>
                )}
                <StatusCard status={post.value} withActions={false} />
                {canRefetch && post.deletedAt === undefined && isRemote(post.value.account) && (
                  <div className="px-4 pb-3">
                    <Button
                      isPending={refetch.isPending({ kind: "status", id: post.value.id })}
                      onPress={() => {
                        refetch.request({ kind: "status", id: post.value.id });
                      }}
                    >
                      {t("client:admin.refetch.post.button", {
                        date: intl.formatDate(post.value.created_at, { dateStyle: "medium", timeStyle: "short" }),
                      })}
                    </Button>
                  </div>
                )}
              </>
            ) : (
              <p className="px-4 py-3 text-fg-muted">{t("client:admin.report.posts.missing")}</p>
            )}
          </li>
        ))}
      </ul>
    </div>
  );
}

function TargetActions({ accountId, reportId }: { accountId: string; reportId: string }) {
  const t = useT();
  const actions = useTargetActions(accountId);
  if (actions.isLoading) return <Spinner size="sm" label={t("client:admin.report.history.loading")} />;
  if (actions.items.length === 0 && actions.error !== undefined) return <RegionError heading={t("client:admin.report.history.failed")} headingLevel={3} reason={actions.error} />;
  return (
    <RecordList
      list={actions}
      getId={(action) => action.id}
      empty={<EmptyState headingLevel={3} heading={t("client:admin.report.history.empty")} />}
      moreError={actions.error !== undefined && <RegionError heading={t("client:admin.report.history.failed")} headingLevel={3} reason={actions.error} />}
    >
      {(action) => <TargetActionRow action={action} fromThisReport={action.report_id === reportId} />}
    </RecordList>
  );
}

function TargetActionRow({ action, fromThisReport }: { action: AccountAction; fromThisReport: boolean }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-2">
      <h3 className="font-semibold">{t(accountActionKindLabels[action.kind])}</h3>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.account_actions.state")}>{t(accountActionStateLabels[action.state])}</DescriptionItem>
        <DescriptionItem term={t("client:admin.account_actions.issued_at")}>
          <DateTime value={action.issued_at} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.account_actions.expires_at")}>
          {action.expires_at === undefined ? t("client:admin.account_actions.duration.indefinite") : <DateTime value={action.expires_at} />}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.account_actions.category")}>{t(reportCategoryLabels[action.reason_category])}</DescriptionItem>
        {action.report_id !== undefined && (
          <DescriptionItem term={t("client:admin.account_actions.report")}>
            {fromThisReport ? (
              t("client:admin.account_actions.report.this")
            ) : (
              <Link href={`/moderation/reports/${encodeURIComponent(action.report_id)}`} variant="inline">
                {t("client:admin.account_actions.report.other")}
              </Link>
            )}
          </DescriptionItem>
        )}
      </DescriptionList>
      {action.public_reason !== undefined && (
        <DescriptionList>
          <DescriptionItem term={t("client:admin.account_actions.public_reason")}>
            {/* Written by a moderator, in any language. */}
            <span dir="auto" className="block text-start wrap-anywhere whitespace-pre-wrap">
              {action.public_reason}
            </span>
          </DescriptionItem>
        </DescriptionList>
      )}
    </div>
  );
}

type AssigneeProps = {
  report: ReportDetail;
  pending: boolean;
  onAssign: () => void;
  onUnassign: () => void;
  isAssigning: boolean;
  isUnassigning: boolean;
};

function Assignee({ report, pending, onAssign, onUnassign, isAssigning, isUnassigning }: AssigneeProps) {
  const t = useT();
  const assignee = report.assignee_account_id;
  return (
    <div className="flex flex-col items-start gap-3">
      <DescriptionList>
        <DescriptionItem term={t("client:admin.report.assignee")}>
          {assignee === undefined ? t("client:admin.report.assignee.none") : <AssigneeName id={assignee} />}
        </DescriptionItem>
      </DescriptionList>
      {/*
        The buttons keep their places as the assignee changes, so that focus stays on the one pressed: taking it
        from nobody or from another moderator, and leaving it to nobody.
      */}
      <div className="flex flex-wrap gap-2">
        <Button onPress={onAssign} isPending={isAssigning} isDisabled={pending && !isAssigning}>
          {assignee === undefined ? t("client:admin.report.assign") : t("client:admin.report.assign.take_over")}
        </Button>
        {assignee !== undefined && (
          <Button onPress={onUnassign} isPending={isUnassigning} isDisabled={pending && !isUnassigning}>
            {t("client:admin.report.unassign")}
          </Button>
        )}
      </div>
    </div>
  );
}

function Notes({ report }: { report: ReportDetail }) {
  const t = useT();
  const form = useReportNoteForm(report.id);
  return (
    <div className="flex flex-col gap-4">
      {report.notes.length === 0 ? (
        <p className="text-fg-muted">{t("client:admin.report.notes.empty")}</p>
      ) : (
        <ul className="flex flex-col divide-y divide-border">
          {report.notes.map((note) => (
            <li key={note.id} className="flex flex-col gap-1 py-3">
              <span className="flex flex-wrap items-baseline gap-x-3 text-sm text-fg-muted">
                <span>{note.author_account_id === undefined ? t("client:admin.report.notes.cli") : <AssigneeName id={note.author_account_id} />}</span>
                <DateTime value={note.created_at} />
              </span>
              {/* Written by a moderator, in any language. */}
              <p dir="auto" className="text-start wrap-anywhere whitespace-pre-wrap">
                {note.body}
              </p>
            </li>
          ))}
        </ul>
      )}
      <form onSubmit={form.submit} className="flex flex-col gap-3">
        <ReasonField
          label={t("client:admin.report.notes.new")}
          description={t("client:admin.report.notes.new.description")}
          value={form.body}
          onChange={form.change}
          isRequired
          characterLimit={reportNoteLimit}
          formatRemaining={(remaining) =>
            remaining < 0 ? t("client:admin.account_actions.characters_over", { count: -remaining }) : t("client:admin.account_actions.characters_left", { count: remaining })
          }
          isInvalid={form.error !== undefined}
          errorMessage={(validation) =>
            form.error ?? (validation.validationDetails.valueMissing ? t("client:admin.report.notes.blank") : t("client:admin.report.notes.too_long", { max: reportNoteLimit }))
          }
        />
        <div>
          <Button type="submit" isPending={form.isPending}>
            {t("client:admin.report.notes.submit")}
          </Button>
        </div>
        <StatusMessage>{form.status}</StatusMessage>
      </form>
    </div>
  );
}
