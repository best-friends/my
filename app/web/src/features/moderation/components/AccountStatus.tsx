// SPDX-License-Identifier: AGPL-3.0-only
// The account status page below its heading (FR-715, FR-707): the moderation actions in effect on the user's
// account and the past ones, each with what it does, its reason, when it was taken and when it ends, the
// moderators' explanation, the posts it acted on, and its appeal. An action that can still be appealed has a
// button that opens the appeal in an alert dialog (ConfirmDialog: an action is appealed once, and the appeal
// cannot be changed); one appealed already shows the appeal's state instead, and one whose window closed
// says so. Dates are in the user's locale and time zone (react-intl's, from the preferences).
import { Button, ConfirmDialog, DescriptionItem, DescriptionList, EmptyState, ReasonField, Spinner, StatusMessage } from "@my/ui";
import { type ReactNode, useId } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, type Translate, useT } from "../../../lib/i18n";
import { StatusCard } from "../../status";
import {
  appealTextLimit,
  type ModerationAction,
  type ModerationActionKind,
  type ModerationActionState,
  type ModerationAppeal,
  type ModerationAppealState,
} from "../api/actions";
import { type ActionList, type AppealForm, appealHeadingId, useActionStatuses, useActiveActions, useAppealForm, usePastActions } from "../hooks/useAccountStatus";

const stateLabels: Readonly<Record<ModerationActionState, ClientMessageKey>> = {
  active: "client:account_status.state.active",
  expired: "client:account_status.state.expired",
  lifted: "client:account_status.state.lifted",
};

const appealStateLabels: Readonly<Record<ModerationAppealState, ClientMessageKey>> = {
  pending: "client:account_status.appeal.state.pending",
  upheld: "client:account_status.appeal.state.upheld",
  overturned: "client:account_status.appeal.state.overturned",
};

/**
 * The kinds that can last for a time; the others (a notice, the actions on posts, a ban) have no end. One of
 * these in effect without an end lasts until a moderator lifts it.
 */
const timedKinds: ReadonlySet<ModerationActionKind> = new Set(["force_sensitive", "limit", "restrict_posting", "restrict_interaction", "disable_login", "suspend"]);

/** The format of every date and time on the page. */
const dateTimeFormat = { dateStyle: "medium", timeStyle: "short" } as const;

/** A moment in the user's locale and time zone, as a `<time>`. */
function DateTime({ value }: { value: string }) {
  const intl = useIntl();
  return <time dateTime={value}>{intl.formatDate(value, dateTimeFormat)}</time>;
}

export function AccountStatus() {
  const t = useT();
  const active = useActiveActions();
  const past = usePastActions();
  const form = useAppealForm();
  return (
    <div className="flex flex-col gap-8">
      <StatusMessage tone="success">{form.status}</StatusMessage>
      <Section title={t("client:account_status.active.title")}>
        <ActionListView
          list={active}
          form={form}
          empty={<EmptyState headingLevel={3} heading={t("client:account_status.active.empty")} description={t("client:account_status.active.empty.description")} />}
        />
      </Section>
      <Section title={t("client:account_status.past.title")}>
        <ActionListView list={past} form={form} empty={<EmptyState headingLevel={3} heading={t("client:account_status.past.empty")} />} />
      </Section>
      <AppealDialog form={form} />
    </div>
  );
}

/** A titled region of the page below its `h1`: an `h2` that names a `<section>`. */
function Section({ title, children }: { title: ReactNode; children: ReactNode }) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {title}
      </h2>
      {children}
    </section>
  );
}

/** The actions of a list, divided by lines (web-client.md G3), then "show more" while there are more. */
function ActionListView({ list, form, empty }: { list: ActionList; form: AppealForm; empty: ReactNode }) {
  const t = useT();
  // The past actions are the whole list without those in effect, so a page can have none of them.
  if (list.items.length === 0 && !list.hasMore) return empty;
  return (
    <div className="flex flex-col gap-4">
      {list.items.length > 0 && (
        <ul className="flex flex-col divide-y divide-border">
          {list.items.map((action) => (
            <li key={action.id} className="@container py-3">
              <ActionRow action={action} form={form} />
            </li>
          ))}
        </ul>
      )}
      {list.hasMore && (
        <div>
          <Button onPress={list.loadMore} isPending={list.isLoadingMore}>
            {t("client:account_status.more")}
          </Button>
        </div>
      )}
    </div>
  );
}

function ActionRow({ action, form }: { action: ModerationAction; form: AppealForm }) {
  const t = useT();
  return (
    <article aria-labelledby={`action-${action.id}`} className="flex flex-col gap-3">
      {/* The kind in the user's language, as the server names it. */}
      <h3 id={`action-${action.id}`} className="font-semibold wrap-anywhere">
        {action.kind_label}
      </h3>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:account_status.state")}>{t(stateLabels[action.state])}</DescriptionItem>
        <DescriptionItem term={t("client:account_status.reason_category")}>{action.reason_category_label}</DescriptionItem>
        <DescriptionItem term={t("client:account_status.issued_at")}>
          <DateTime value={action.issued_at} />
        </DescriptionItem>
        {action.expires_at !== undefined ? (
          <DescriptionItem term={t("client:account_status.expires_at")}>
            <DateTime value={action.expires_at} />
          </DescriptionItem>
        ) : (
          action.state === "active" &&
          timedKinds.has(action.kind) && <DescriptionItem term={t("client:account_status.expires_at")}>{t("client:account_status.expires_at.none")}</DescriptionItem>
        )}
        {action.lifted_at !== undefined && (
          <DescriptionItem term={t("client:account_status.lifted_at")}>
            <DateTime value={action.lifted_at} />
          </DescriptionItem>
        )}
      </DescriptionList>
      <DescriptionList>
        <DescriptionItem term={t("client:account_status.public_reason")}>
          {action.public_reason === undefined ? (
            t("client:account_status.public_reason.none")
          ) : (
            // Written by a moderator, in any language and direction (web-client.md Y10).
            <span dir="auto" className="whitespace-pre-wrap wrap-anywhere">
              {action.public_reason}
            </span>
          )}
        </DescriptionItem>
      </DescriptionList>
      {action.status_ids.length > 0 && <ActionPosts statusIds={action.status_ids} />}
      <AppealPart action={action} form={form} />
    </article>
  );
}

/** The posts an action on posts acted on; a deleted one says so. */
function ActionPosts({ statusIds }: { statusIds: readonly string[] }) {
  const t = useT();
  const headingId = useId();
  const posts = useActionStatuses(statusIds);
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-2">
      <h4 id={headingId} className="font-semibold">
        {t("client:account_status.posts.title")}
      </h4>
      <ul className="flex flex-col divide-y divide-border rounded-lg border border-border">
        {posts.map((post) => (
          <li key={post.id}>
            {post.status === "found" ? (
              <StatusCard status={post.value} withActions={false} />
            ) : post.status === "loading" ? (
              <div className="px-4 py-3">
                <Spinner size="sm" label={t("client:account_status.posts.loading")} />
              </div>
            ) : (
              <p className="px-4 py-3 text-fg-muted">{t("client:account_status.posts.missing")}</p>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

/** An action's appeal: the appeal and its decision, the button that opens one, or why there can be none. */
function AppealPart({ action, form }: { action: ModerationAction; form: AppealForm }) {
  const t = useT();
  const intl = useIntl();
  const headingId = appealHeadingId(action.id);
  let body: ReactNode;
  if (action.appeal !== undefined) {
    body = <AppealFacts appeal={action.appeal} />;
  } else if (action.appealable) {
    const deadline = action.appeal_deadline === undefined ? undefined : intl.formatDate(action.appeal_deadline, dateTimeFormat);
    body = (
      <div className="flex flex-col items-start gap-2">
        {deadline !== undefined && <p>{t("client:account_status.appeal.until", { date: deadline })}</p>}
        <Button
          size="sm"
          aria-label={t("client:account_status.appeal.open_of", { date: intl.formatDate(action.issued_at, dateTimeFormat) })}
          onPress={() => {
            form.open(action);
          }}
        >
          {t("client:account_status.appeal.open")}
        </Button>
      </div>
    );
  } else {
    body = (
      <p>
        {action.appeal_deadline === undefined
          ? t("client:account_status.appeal.unavailable")
          : t("client:account_status.appeal.closed", { date: intl.formatDate(action.appeal_deadline, dateTimeFormat) })}
      </p>
    );
  }
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-2">
      {/* Focus moves here once an appeal is sent, as the button that opened it is gone. */}
      <h4 id={headingId} tabIndex={-1} className="font-semibold">
        {t("client:account_status.appeal.title")}
      </h4>
      {body}
    </section>
  );
}

function AppealFacts({ appeal }: { appeal: ModerationAppeal }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-3">
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:account_status.appeal.state")}>{t(appealStateLabels[appeal.state])}</DescriptionItem>
        <DescriptionItem term={t("client:account_status.appeal.submitted_at")}>
          <DateTime value={appeal.submitted_at} />
        </DescriptionItem>
        {appeal.decided_at !== undefined && (
          <DescriptionItem term={t("client:account_status.appeal.decided_at")}>
            <DateTime value={appeal.decided_at} />
          </DescriptionItem>
        )}
      </DescriptionList>
      <DescriptionList>
        <DescriptionItem term={t("client:account_status.appeal.text")}>
          {/* The user's own text (web-client.md Y10). */}
          <span dir="auto" className="whitespace-pre-wrap wrap-anywhere">
            {appeal.text}
          </span>
        </DescriptionItem>
        {appeal.decision_reason !== undefined && (
          <DescriptionItem term={t("client:account_status.appeal.decision_reason")}>
            <span dir="auto" className="whitespace-pre-wrap wrap-anywhere">
              {appeal.decision_reason}
            </span>
          </DescriptionItem>
        )}
      </DescriptionList>
    </div>
  );
}

/** Why an appeal's text cannot be sent, or null when it can. */
function validateAppealText(t: Translate, text: string): string | null {
  if (text.trim() === "") return t("client:account_status.appeal.text.required");
  if (Array.from(text).length > appealTextLimit) return t("client:account_status.appeal.text.too_long", { max: appealTextLimit });
  return null;
}

function AppealDialog({ form }: { form: AppealForm }) {
  const t = useT();
  const intl = useIntl();
  const { action } = form;
  // The last action stays while the dialog fades out; before the first one the dialog is closed and empty.
  const date = action === null ? "" : intl.formatDate(action.issued_at, dateTimeFormat);
  return (
    <ConfirmDialog
      isOpen={form.isOpen}
      onOpenChange={(isOpen) => {
        if (!isOpen) form.cancel();
      }}
      title={t("client:account_status.appeal.confirm.title")}
      description={t("client:account_status.appeal.confirm.description", { kind: action?.kind_label ?? "", date })}
      confirmLabel={t("client:account_status.appeal.send")}
      cancelLabel={t("client:account_status.appeal.cancel")}
      tone="primary"
      onConfirm={() => {
        form.submit();
      }}
      isPending={form.isPending}
      {...(form.error === undefined ? {} : { errorMessage: t("client:account_status.appeal.failed", { reason: form.error }) })}
    >
      <ReasonField
        label={t("client:account_status.appeal.text.label")}
        description={t("client:account_status.appeal.text.description")}
        value={form.text}
        onChange={form.changeText}
        isRequired
        // Text of white space only is empty to the server too, and too long a text is refused; either one
        // stops the dialog's form before it sends (the browser's constraint validation).
        validate={(value) => validateAppealText(t, value)}
        characterLimit={appealTextLimit}
        formatRemaining={(remaining) =>
          remaining < 0 ? t("client:account_status.appeal.characters_over", { count: -remaining }) : t("client:account_status.appeal.characters_left", { count: remaining })
        }
        errorMessage={validateAppealText(t, form.text) ?? t("client:account_status.appeal.text.required")}
      />
    </ConfirmDialog>
  );
}
