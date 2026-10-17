// SPDX-License-Identifier: AGPL-3.0-only
// The domain actions screen below its heading (FR-702): the form of a new action, then the actions, in effect
// or all, newest first. The form follows the browser's constraint validation (React Aria's native validation);
// a suspension asks first in an alert dialog (ConfirmDialog, NFR-MOD-7), the other kinds take effect at once.
// What an action or a lift did stays on the page in a StatusMessage (role="status") next to what caused it.
// The tabs follow the WAI-ARIA APG tabs pattern with automatic activation (React Aria Tabs), kept in the URL by
// the route (web-client.md S3); only the selected panel is mounted.
import {
  Button,
  Checkbox,
  ConfirmDialog,
  DescriptionItem,
  DescriptionList,
  DurationSelect,
  EmptyState,
  Link,
  Radio,
  RadioGroup,
  ReasonField,
  StatusMessage,
  Tab,
  TabList,
  TabPanel,
  Tabs,
  TextField,
} from "@my/ui";
import { type RefObject, useEffect, useRef, useState } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import {
  type DomainAction,
  type DomainActionFilter,
  domainActionFilters,
  type DomainActionKind,
  domainActionKinds,
  type DomainActionState,
  domainActionTextLimit,
} from "../api/admin";
import {
  domainActionKindLabels,
  domainInText,
  type LiftDomainAction,
  useDomainActionForm,
  useDomainActions,
  useLiftDomainAction,
} from "../hooks/useDomainActions";
import { DateTime, DetailSection, DomainName, RecordList, RequirePermission } from "./parts";

const kindDescriptions: Readonly<Record<DomainActionKind, ClientMessageKey>> = {
  limit: "client:admin.domain_actions.kind.limit.description",
  suspend: "client:admin.domain_actions.kind.suspend.description",
  reject_media: "client:admin.domain_actions.kind.reject_media.description",
  reject_reports: "client:admin.domain_actions.kind.reject_reports.description",
  force_sensitive: "client:admin.domain_actions.kind.force_sensitive.description",
};

const stateLabels: Readonly<Record<DomainActionState, ClientMessageKey>> = {
  active: "client:admin.domain_actions.state.active",
  expired: "client:admin.domain_actions.state.expired",
  lifted: "client:admin.domain_actions.state.lifted",
};

const filterLabels: Readonly<Record<DomainActionFilter, ClientMessageKey>> = {
  active: "client:admin.domain_actions.filter.active",
  all: "client:admin.domain_actions.filter.all",
};

/** The durations an action can last, in seconds, from the shortest. */
const day = 86_400;
const durations = [day, 7 * day, 30 * day, 90 * day, 180 * day, 365 * day] as const;

function isFilter(key: unknown): key is DomainActionFilter {
  return domainActionFilters.some((filter) => filter === key);
}

export type DomainActionsProps = {
  /** The selected filter, from the URL. */
  filter: DomainActionFilter;
  /** The domain whose actions to list and to fill in the form, from the URL; undefined lists every domain. */
  domain: string | undefined;
  /** Selects another filter: the route puts it into the URL. */
  onFilterChange: (filter: DomainActionFilter) => void;
  /** The address of the list of every domain under the same filter. */
  everyDomainHref: string;
};

export function DomainActions(props: DomainActionsProps) {
  const t = useT();
  return (
    <RequirePermission permission="manage_domains">
      <div className="flex flex-col gap-8">
        <DetailSection title={t("client:admin.domain_actions.new.title")}>
          {/* Another domain from the URL starts another form (web-client.md R10). */}
          <NewDomainActionForm key={props.domain ?? ""} initialDomain={props.domain ?? ""} />
        </DetailSection>
        <DetailSection title={t("client:admin.domain_actions.list.title")}>
          <DomainActionTabs {...props} />
        </DetailSection>
      </div>
    </RequirePermission>
  );
}

function NewDomainActionForm({ initialDomain }: { initialDomain: string }) {
  const t = useT();
  const form = useDomainActionForm(initialDomain);
  const { draft } = form;
  const formatRemaining = (remaining: number) =>
    remaining < 0 ? t("client:admin.domain_actions.characters_over", { count: -remaining }) : t("client:admin.domain_actions.characters_left", { count: remaining });
  const tooLong = t("client:admin.domain_actions.too_long", { max: domainActionTextLimit });
  return (
    <>
      <form onSubmit={form.submit} className="flex flex-col gap-6">
      <TextField
        label={t("client:admin.domain_actions.domain")}
        description={t("client:admin.domain_actions.domain.description")}
        value={draft.domain}
        onChange={(domain) => {
          form.change({ domain });
        }}
        isRequired
        maxLength={253}
        autoComplete="off"
        errorMessage={t("client:admin.domain_actions.domain.required")}
      />
      <RadioGroup
        label={t("client:admin.domain_actions.kind")}
        value={draft.kind}
        onChange={(kind) => {
          const chosen = domainActionKinds.find((item) => item === kind);
          if (chosen !== undefined) form.change({ kind: chosen });
        }}
        isRequired
        errorMessage={t("client:admin.domain_actions.kind.required")}
      >
        {domainActionKinds.map((kind) => (
          <Radio key={kind} value={kind} description={t(kindDescriptions[kind])}>
            {t(domainActionKindLabels[kind])}
          </Radio>
        ))}
      </RadioGroup>
      <DurationSelect
        label={t("client:admin.domain_actions.duration")}
        description={t("client:admin.domain_actions.duration.description")}
        durations={durations}
        formatDuration={(seconds) => t("client:admin.domain_actions.duration.days", { count: seconds / day })}
        indefiniteLabel={t("client:admin.domain_actions.duration.indefinite")}
        value={draft.expiresIn}
        onChange={(expiresIn) => {
          form.change({ expiresIn });
        }}
      />
      <ReasonField
        label={t("client:admin.domain_actions.public_comment")}
        description={t("client:admin.domain_actions.public_comment.description")}
        value={draft.publicComment}
        onChange={(publicComment) => {
          form.change({ publicComment });
        }}
        characterLimit={domainActionTextLimit}
        formatRemaining={formatRemaining}
        errorMessage={tooLong}
      />
      <ReasonField
        label={t("client:admin.domain_actions.private_note")}
        description={t("client:admin.domain_actions.private_note.description")}
        value={draft.privateNote}
        onChange={(privateNote) => {
          form.change({ privateNote });
        }}
        characterLimit={domainActionTextLimit}
        formatRemaining={formatRemaining}
        errorMessage={tooLong}
      />
      <Checkbox
        isSelected={draft.obfuscate}
        onChange={(obfuscate) => {
          form.change({ obfuscate });
        }}
        description={t("client:admin.domain_actions.obfuscate.description")}
      >
        {t("client:admin.domain_actions.obfuscate")}
      </Checkbox>
      {form.confirmation === null && form.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.domain_actions.issue_failed", { reason: form.error })}
        </p>
      )}
      <div>
        <Button type="submit" variant="primary" isPending={form.isPending && form.confirmation === null}>
          {t("client:admin.domain_actions.submit")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
      </form>
      {/* Outside the form: React sends the dialog's own submit event up its component tree, through a portal too. */}
      <ConfirmDialog
        isOpen={form.confirmation !== null}
        onOpenChange={(isOpen) => {
          if (!isOpen) form.confirmation?.cancel();
        }}
        title={t("client:admin.domain_actions.confirm_suspend.title", { domain: domainInText(t, form.confirmation?.domain ?? draft.domain) })}
        description={t("client:admin.domain_actions.confirm_suspend.description")}
        confirmLabel={t("client:admin.domain_actions.confirm_suspend.confirm")}
        cancelLabel={t("client:admin.domain_actions.confirm_suspend.cancel")}
        onConfirm={() => {
          form.confirmation?.confirm();
        }}
        isPending={form.isPending}
        {...(form.confirmation !== null && form.error !== undefined ? { errorMessage: t("client:admin.domain_actions.issue_failed", { reason: form.error }) } : {})}
      />
    </>
  );
}

function DomainActionTabs({ filter, domain, onFilterChange, everyDomainHref }: DomainActionsProps) {
  const t = useT();
  const [status, setStatus] = useState<string | undefined>(undefined);
  return (
    <div className="flex flex-col gap-3">
      {domain !== undefined && (
        <p className="flex flex-wrap items-baseline gap-x-2">
          <span className="wrap-anywhere">{t("client:admin.domain_actions.list.of_domain", { domain: domainInText(t, domain) })}</span>
          <Link href={everyDomainHref} variant="standalone">
            {t("client:admin.domain_actions.list.every_domain")}
          </Link>
        </p>
      )}
      <StatusMessage>{status}</StatusMessage>
      <Tabs
        selectedKey={filter}
        onSelectionChange={(key) => {
          if (isFilter(key)) onFilterChange(key);
        }}
      >
        <TabList aria-label={t("client:admin.domain_actions.filter.label")}>
          {domainActionFilters.map((item) => (
            <Tab key={item} id={item}>
              {t(filterLabels[item])}
            </Tab>
          ))}
        </TabList>
        <TabPanel id={filter}>
          <DomainActionList
            filter={filter}
            domain={domain}
            onLifted={(action) => {
              setStatus(t("client:admin.domain_actions.lifted", { kind: t(domainActionKindLabels[action.kind]), domain: domainInText(t, action.domain) }));
            }}
          />
        </TabPanel>
      </Tabs>
    </div>
  );
}

type DomainActionListProps = { filter: DomainActionFilter; domain: string | undefined; onLifted: (action: DomainAction) => void };

function DomainActionList({ filter, domain, onLifted }: DomainActionListProps) {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const list = useDomainActions({ filter, domain });
  const removed = useFocusAfterRemoval(
    listRef,
    list.items.map((action) => action.id),
  );
  const keepFocus = useFocusHeadingAfterLift(listRef);
  const lift = useLiftDomainAction((action) => {
    // The lift button goes away: with the row from a list of actions in effect, or from the lifted row.
    if (filter === "active") removed(action.id);
    else keepFocus(action.id);
    onLifted(action);
  });
  const empty =
    filter === "active" ? (
      <EmptyState headingLevel={3} heading={t("client:admin.domain_actions.empty.active")} description={t("client:admin.domain_actions.empty.active.description")} />
    ) : (
      <EmptyState headingLevel={3} heading={t("client:admin.domain_actions.empty.all")} />
    );
  return (
    <RecordList list={list} getId={(action) => action.id} listRef={listRef} empty={empty}>
      {(action) => <DomainActionRow action={action} lift={lift} />}
    </RecordList>
  );
}

/**
 * Keeps focus when a lifted action stays in the list without its lift button: once the row has rendered
 * without the button, focus moves to the row's heading. Returns `lifted(id)`, to call when the action is lifted.
 */
function useFocusHeadingAfterLift(list: RefObject<HTMLElement | null>): (id: string) => void {
  const pendingRef = useRef<string | null>(null);
  // Runs after every render: focus is moved in the DOM, outside React, once the button is gone.
  useEffect(() => {
    const id = pendingRef.current;
    if (id === null) return;
    const row = list.current?.querySelector<HTMLElement>(`[data-row-id="${id}"]`);
    if (row?.querySelector("button") != null) return;
    pendingRef.current = null;
    row?.querySelector<HTMLElement>("h3")?.focus();
  });
  return (id) => {
    pendingRef.current = id;
  };
}

function DomainActionRow({ action, lift }: { action: DomainAction; lift: LiftDomainAction }) {
  const t = useT();
  const intl = useIntl();
  const kind = t(domainActionKindLabels[action.kind]);
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col items-start gap-2 @md:flex-row @md:items-center @md:justify-between">
        <h3 tabIndex={-1} className="font-semibold wrap-anywhere outline-none">
          <DomainName domain={action.domain} />
        </h3>
        {action.state === "active" && (
          <Button
            size="sm"
            isPending={lift.pendingIds.includes(action.id)}
            aria-label={t("client:admin.domain_actions.lift_action", {
              kind,
              domain: domainInText(t, action.domain),
              date: intl.formatDate(action.issued_at, { dateStyle: "medium", timeStyle: "short" }),
            })}
            onPress={() => {
              lift.lift(action);
            }}
          >
            {t("client:admin.domain_actions.lift")}
          </Button>
        )}
      </div>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.domain_actions.kind")}>{kind}</DescriptionItem>
        <DescriptionItem term={t("client:admin.domain_actions.state")}>
          <span className={action.state === "active" ? "font-semibold" : undefined}>{t(stateLabels[action.state])}</span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.domain_actions.issued_at")}>
          <DateTime value={action.issued_at} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.domain_actions.expires_at")}>
          {action.expires_at === undefined ? t("client:admin.domain_actions.duration.indefinite") : <DateTime value={action.expires_at} />}
        </DescriptionItem>
        {action.lifted_at !== undefined && (
          <DescriptionItem term={t("client:admin.domain_actions.lifted_at")}>
            <DateTime value={action.lifted_at} />
          </DescriptionItem>
        )}
        <DescriptionItem term={t("client:admin.domain_actions.obfuscate.term")}>
          {action.obfuscate ? t("client:admin.domain_actions.obfuscate.yes") : t("client:admin.domain_actions.obfuscate.no")}
        </DescriptionItem>
      </DescriptionList>
      {(action.public_comment !== undefined || action.private_note !== undefined) && (
        <DescriptionList>
          {action.public_comment !== undefined && (
            <DescriptionItem term={t("client:admin.domain_actions.public_comment")}>
              {/* Written by a moderator, in any language. */}
              <span dir="auto" className="whitespace-pre-wrap">
                {action.public_comment}
              </span>
            </DescriptionItem>
          )}
          {action.private_note !== undefined && (
            <DescriptionItem term={t("client:admin.domain_actions.private_note")}>
              <span dir="auto" className="whitespace-pre-wrap">
                {action.private_note}
              </span>
            </DescriptionItem>
          )}
        </DescriptionList>
      )}
    </div>
  );
}
