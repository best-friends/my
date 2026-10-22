// SPDX-License-Identifier: AGPL-3.0-only
// The supporters screen below its heading (ADR-0072, manage_supporters): the supporter program (whether it is enabled,
// its payment provider, what it needs, and enabling or disabling it), the form that creates a plan, the plans by rank
// with changing and retiring them, the retired plans, and the subscriptions narrowed to a plan and a state with ending
// one at once. The plans' names and descriptions are the administrators' text, shown with dir="auto". What a change did
// stays in a StatusMessage (role="status"), and a refusal is said in the server's words next to its field or under the
// form (role="alert"). Retiring a plan and ending a subscription ask first in an alert dialog: neither can be undone.
import {
  Button,
  ConfirmDialog,
  DescriptionItem,
  DescriptionList,
  Dialog,
  DialogTrigger,
  EmptyState,
  Link,
  Radio,
  RadioGroup,
  Select,
  SelectItem,
  StatusMessage,
  Switch,
  TextArea,
  TextField,
} from "@my/ui";
import { type SubmitEvent, useState } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFormatMoney } from "../../costs";
import { DateTime, DetailSection, type Permission, RecordList, RequirePermission } from "../../moderation";
import {
  type SupporterPlan,
  supporterPlanDescriptionLimit,
  type SupporterPlanVisibility,
  supporterPlanVisibilities,
  type SupporterProgramWarning,
  type SupporterSubscription,
  type SupporterSubscriptionFilter,
  type SupporterSubscriptionState,
  supporterSubscriptionStates,
} from "../api/supporters";
import {
  draftOfPlan,
  type PlanActions,
  type PlanDraft,
  type PlanFieldError,
  planName,
  type SubscriptionActions,
  useNewPlanForm,
  usePlanActions,
  useSubscriptionActions,
  useSupporterPlans,
  useSupporterProgram,
  useSupporterSubscriptions,
} from "../hooks/useSupporters";

const warningLabels: Readonly<Record<SupporterProgramWarning, ClientMessageKey>> = {
  no_provider: "client:admin.supporters.warning.no_provider",
  no_email: "client:admin.supporters.warning.no_email",
  provider_mismatch: "client:admin.supporters.warning.provider_mismatch",
  no_commerce_disclosure: "client:admin.supporters.warning.no_commerce_disclosure",
};

const visibilityLabels: Readonly<Record<SupporterPlanVisibility, ClientMessageKey>> = {
  public: "client:admin.supporters.plan.visibility.public",
  hidden: "client:admin.supporters.plan.visibility.hidden",
};

const stateLabels: Readonly<Record<SupporterSubscriptionState, ClientMessageKey>> = {
  pending: "client:admin.supporters.state.pending",
  active: "client:admin.supporters.state.active",
  canceling: "client:admin.supporters.state.canceling",
  past_due: "client:admin.supporters.state.past_due",
  lapsed: "client:admin.supporters.state.lapsed",
  expired: "client:admin.supporters.state.expired",
  revoked: "client:admin.supporters.state.revoked",
  abandoned: "client:admin.supporters.state.abandoned",
};

/** Whether a subscription goes on, so that it can be ended at once. */
function isLive(state: SupporterSubscriptionState): boolean {
  return state !== "expired" && state !== "revoked" && state !== "abandoned";
}

type SupporterSettingsProps = {
  /** The subscriptions shown, from the URL (web-client.md S3). */
  filter: SupporterSubscriptionFilter;
  onFilterChange: (filter: SupporterSubscriptionFilter) => void;
};

export function SupporterSettings({ filter, onFilterChange }: SupporterSettingsProps) {
  const t = useT();
  return (
    <RequirePermission permission="manage_supporters">
      <div className="flex flex-col gap-8">
        <DetailSection title={t("client:admin.supporters.program.title")}>
          <Program />
        </DetailSection>
        <DetailSection title={t("client:admin.supporters.new.title")}>
          <NewPlanForm />
        </DetailSection>
        <Plans />
        <DetailSection title={t("client:admin.supporters.subscriptions.title")}>
          <Subscriptions filter={filter} onFilterChange={onFilterChange} />
        </DetailSection>
      </div>
    </RequirePermission>
  );
}

function Program() {
  const t = useT();
  const { program, set, isPending, error, status } = useSupporterProgram();
  const none = t("client:admin.supporters.program.none");
  return (
    <div className="flex flex-col gap-4">
      <p className="text-fg-muted">{t("client:admin.supporters.program.description")}</p>
      <DescriptionList columns={2}>
        <DescriptionItem term={t("client:admin.supporters.program.provider")}>
          <span dir="ltr">{program.provider ?? none}</span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.program.configured_provider")}>
          <span dir="ltr">{program.configured_provider ?? none}</span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.program.live_subscriptions")}>
          {t("client:admin.supporters.program.subscription_count", { count: program.live_subscriptions })}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.program.changed_at")}>
          {program.changed_at === undefined ? t("client:admin.supporters.program.never") : <DateTime value={program.changed_at} />}
        </DescriptionItem>
      </DescriptionList>
      {program.warnings.length > 0 && (
        <ul className="flex flex-col gap-1">
          {program.warnings.map((warning) => (
            <li key={warning} className="text-warning">
              {t(warningLabels[warning])}
              {warning === "no_commerce_disclosure" && (
                <>
                  {" "}
                  <Link href="/admin/documents" variant="standalone">
                    {t("client:admin.supporters.warning.no_commerce_disclosure.link")}
                  </Link>
                </>
              )}
            </li>
          ))}
        </ul>
      )}
      <Switch
        isSelected={program.enabled}
        isDisabled={isPending || (!program.enabled && !program.can_enable)}
        onChange={set}
        description={t("client:admin.supporters.program.switch.description")}
      >
        {t("client:admin.supporters.program.switch")}
      </Switch>
      {error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.supporters.program.failed", { reason: error })}
        </p>
      )}
      <StatusMessage>{status}</StatusMessage>
    </div>
  );
}

/** The error props of a field, when the refusal is about it. */
function fieldError(error: PlanFieldError | undefined, field: PlanFieldError["field"]) {
  return error?.field === field ? { isInvalid: true, errorMessage: error.message } : {};
}

type PlanFieldsProps = {
  draft: PlanDraft;
  change: (change: Partial<PlanDraft>) => void;
  error: PlanFieldError | undefined;
  /** A new plan picks its role and currency, which never change; an existing one sets its role's position. */
  isNew: boolean;
  autoFocus?: boolean;
};

/** The fields of a plan, shared by the form of a new plan and the dialog that changes one. */
function PlanFields({ draft, change, error, isNew, autoFocus = false }: PlanFieldsProps) {
  const t = useT();
  const formatRemaining = (remaining: number) =>
    remaining < 0 ? t("client:admin.supporters.plan.characters_over", { count: -remaining }) : t("client:admin.supporters.plan.characters_left", { count: remaining });
  return (
    <>
      {isNew && (
        <TextField
          label={t("client:admin.supporters.plan.role")}
          description={t("client:admin.supporters.plan.role.description")}
          value={draft.role}
          onChange={(role) => {
            change({ role });
          }}
          dir="ltr"
          isRequired
          autoComplete="off"
          spellCheck="false"
          {...fieldError(error, "role")}
        />
      )}
      <TextField
        label={t("client:admin.supporters.plan.name")}
        value={draft.name}
        onChange={(name) => {
          change({ name });
        }}
        isRequired
        autoComplete="off"
        autoFocus={autoFocus}
        {...fieldError(error, "name")}
      />
      <TextArea
        label={t("client:admin.supporters.plan.description")}
        description={t("client:admin.supporters.plan.description.description")}
        value={draft.description}
        onChange={(description) => {
          change({ description });
        }}
        rows={3}
        characterLimit={supporterPlanDescriptionLimit}
        formatRemaining={formatRemaining}
        {...fieldError(error, "description")}
      />
      <div className="flex flex-wrap gap-4">
        <TextField
          label={t("client:admin.supporters.plan.amount")}
          description={t("client:admin.supporters.plan.amount.description")}
          value={draft.amount}
          onChange={(amount) => {
            change({ amount });
          }}
          inputMode="decimal"
          isRequired
          autoComplete="off"
          {...fieldError(error, "amount")}
        />
        {isNew && (
          <TextField
            label={t("client:admin.supporters.plan.currency")}
            description={t("client:admin.supporters.plan.currency.description")}
            value={draft.currency}
            onChange={(currency) => {
              change({ currency: currency.toUpperCase() });
            }}
            isRequired
            autoComplete="off"
            maxLength={3}
            {...fieldError(error, "currency")}
          />
        )}
      </div>
      <div className="flex flex-wrap gap-4">
        <TextField
          label={t("client:admin.supporters.plan.discount")}
          description={t("client:admin.supporters.plan.discount.description")}
          value={draft.discount}
          onChange={(discount) => {
            change({ discount });
          }}
          inputMode="numeric"
          isRequired
          autoComplete="off"
          {...fieldError(error, "discount")}
        />
        <TextField
          label={t("client:admin.supporters.plan.emoji_limit")}
          description={t("client:admin.supporters.plan.emoji_limit.description")}
          value={draft.emojiLimit}
          onChange={(emojiLimit) => {
            change({ emojiLimit });
          }}
          inputMode="numeric"
          isRequired
          autoComplete="off"
          {...fieldError(error, "emojiLimit")}
        />
        <TextField
          label={t("client:admin.supporters.plan.rank")}
          description={t("client:admin.supporters.plan.rank.description")}
          value={draft.rank}
          onChange={(rank) => {
            change({ rank });
          }}
          inputMode="numeric"
          isRequired
          autoComplete="off"
          {...fieldError(error, "rank")}
        />
        {!isNew && (
          <TextField
            label={t("client:admin.supporters.plan.role_position")}
            description={t("client:admin.supporters.plan.role_position.description")}
            value={draft.rolePosition}
            onChange={(rolePosition) => {
              change({ rolePosition });
            }}
            inputMode="numeric"
            isRequired
            autoComplete="off"
            {...fieldError(error, "rolePosition")}
          />
        )}
      </div>
      <RadioGroup
        label={t("client:admin.supporters.plan.visibility")}
        orientation="horizontal"
        value={draft.visibility}
        onChange={(value) => {
          const visibility = supporterPlanVisibilities.find((v) => v === value);
          if (visibility !== undefined) change({ visibility });
        }}
        {...fieldError(error, "visibility")}
      >
        {supporterPlanVisibilities.map((visibility) => (
          <Radio key={visibility} value={visibility}>
            {t(visibilityLabels[visibility])}
          </Radio>
        ))}
      </RadioGroup>
      {error?.field === "form" && (
        <p role="alert" className="text-danger">
          {t("client:admin.supporters.plan.save_failed", { reason: error.message })}
        </p>
      )}
    </>
  );
}

function NewPlanForm() {
  const t = useT();
  const form = useNewPlanForm();
  return (
    <form onSubmit={form.submit} className="@container flex flex-col gap-4" noValidate>
      <p className="text-fg-muted">{t("client:admin.supporters.new.description")}</p>
      <PlanFields draft={form.draft} change={form.change} error={form.error} isNew />
      <div>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:admin.supporters.new.submit")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
    </form>
  );
}

function Plans() {
  const t = useT();
  const { active, retired } = useSupporterPlans();
  const actions = usePlanActions();
  const list = (items: SupporterPlan[]) => ({ items, hasMore: false, isLoadingMore: false, loadMore: () => undefined });
  return (
    <>
      <DetailSection title={t("client:admin.supporters.plans.title")}>
        <div className="flex flex-col gap-3">
          <StatusMessage>{actions.status}</StatusMessage>
          {actions.error !== undefined && (
            <p role="alert" className="text-danger">
              {t("client:admin.supporters.plan.change_failed", { reason: actions.error })}
            </p>
          )}
          <RecordList
            list={list(active)}
            getId={(plan) => plan.id}
            empty={<EmptyState headingLevel={3} heading={t("client:admin.supporters.plans.empty")} description={t("client:admin.supporters.plans.empty.description")} />}
          >
            {(plan) => <PlanRow plan={plan} actions={actions} />}
          </RecordList>
        </div>
      </DetailSection>
      {retired.length > 0 && (
        <DetailSection title={t("client:admin.supporters.retired.title")}>
          <RecordList list={list(retired)} getId={(plan) => plan.id} empty={null}>
            {(plan) => <PlanRow plan={plan} actions={actions} />}
          </RecordList>
        </DetailSection>
      )}
    </>
  );
}

/** The permissions of a plan's role, by name, or that it has none. */
function RolePermissions({ permissions }: { permissions: readonly Permission[] }) {
  const t = useT();
  if (permissions.length === 0) return t("client:admin.supporters.plan.role.no_permissions");
  return <span dir="ltr">{permissions.join(", ")}</span>;
}

function PlanRow({ plan, actions }: { plan: SupporterPlan; actions: PlanActions }) {
  const t = useT();
  const format = useFormatMoney();
  return (
    <div className="flex flex-col gap-3">
      <h3 className="min-w-0 font-semibold wrap-anywhere">
        <span dir="auto" lang="">
          {plan.name}
        </span>
      </h3>
      {!plan.retired && !plan.prices_ready && <p className="text-warning">{t("client:admin.supporters.plan.preparing")}</p>}
      {!plan.retired && plan.visibility === "hidden" && <p className="text-fg-muted">{t("client:admin.supporters.plan.visibility.hidden")}</p>}
      {plan.description !== "" && (
        <p dir="auto" lang="" className="whitespace-pre-line">
          {plan.description}
        </p>
      )}
      <DescriptionList columns={2}>
        <DescriptionItem term={t("client:admin.supporters.plan.monthly")}>{format(plan.monthly_price)}</DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.plan.yearly")}>
          {t("client:admin.supporters.plan.yearly_value", { amount: format(plan.yearly_price), discount: String(plan.annual_discount_percent) })}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.plan.rank")}>{plan.rank}</DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.plan.emoji_limit")}>{plan.emoji_limit}</DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.plan.role")}>
          {t("client:admin.supporters.plan.role_value", { role: `⁦${plan.role.key}⁩`, position: String(plan.role.position) })}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.plan.permissions")}>
          <RolePermissions permissions={plan.role.permissions} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.plan.members")}>
          {t("client:admin.supporters.program.subscription_count", { count: plan.members })}
        </DescriptionItem>
        {plan.retired_at !== undefined && (
          <DescriptionItem term={t("client:admin.supporters.plan.retired_at")}>
            <DateTime value={plan.retired_at} />
          </DescriptionItem>
        )}
      </DescriptionList>
      {!plan.retired && (
        <div className="flex flex-wrap items-center gap-2">
          <EditPlan plan={plan} actions={actions} />
          <RetirePlan plan={plan} actions={actions} />
        </div>
      )}
    </div>
  );
}

/** The terms of a plan and its role's position in a dialog; it closes once they are saved, and keeps a refusal on its field. */
function EditPlan({ plan, actions }: { plan: SupporterPlan; actions: PlanActions }) {
  const t = useT();
  const name = planName(plan);
  return (
    <DialogTrigger>
      <Button size="sm" aria-label={t("client:admin.supporters.plan.edit_plan", { name })}>
        {t("client:admin.supporters.plan.edit")}
      </Button>
      <Dialog title={t("client:admin.supporters.plan.edit.title", { name })} closeLabel={t("client:admin.supporters.close")} isDismissable={false} size="md">
        {({ close }) => <EditPlanForm plan={plan} actions={actions} close={close} />}
      </Dialog>
    </DialogTrigger>
  );
}

function EditPlanForm({ plan, actions, close }: { plan: SupporterPlan; actions: PlanActions; close: () => void }) {
  const t = useT();
  const [draft, setDraft] = useState<PlanDraft>(() => draftOfPlan(plan));
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<PlanFieldError | undefined>(undefined);
  const onSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSaving(true);
    setError(undefined);
    void actions.update(plan, draft).then((refused) => {
      setIsSaving(false);
      if (refused === undefined) close();
      else setError(refused);
    });
  };
  return (
    <form onSubmit={onSubmit} className="@container flex flex-col gap-4" noValidate>
      <p className="text-sm text-fg-muted">{t("client:admin.supporters.plan.edit.description")}</p>
      <PlanFields
        draft={draft}
        change={(change) => {
          setDraft((previous) => ({ ...previous, ...change }));
        }}
        error={error}
        isNew={false}
        autoFocus
      />
      <div className="flex flex-wrap justify-end gap-2">
        <Button onPress={close}>{t("client:admin.supporters.cancel")}</Button>
        <Button type="submit" variant="primary" isPending={isSaving}>
          {t("client:admin.supporters.save")}
        </Button>
      </div>
    </form>
  );
}

/** Retiring a plan, after an alert dialog: it cannot be undone, and every subscription of it ends. */
function RetirePlan({ plan, actions }: { plan: SupporterPlan; actions: PlanActions }) {
  const t = useT();
  const name = planName(plan);
  const [isRetiring, setIsRetiring] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <DialogTrigger>
      <Button size="sm" variant="danger" aria-label={t("client:admin.supporters.plan.retire_plan", { name })}>
        {t("client:admin.supporters.plan.retire")}
      </Button>
      <ConfirmDialog
        title={t("client:admin.supporters.plan.retire.title", { name })}
        description={t("client:admin.supporters.plan.retire.description")}
        confirmLabel={t("client:admin.supporters.plan.retire")}
        cancelLabel={t("client:admin.supporters.cancel")}
        tone="danger"
        isPending={isRetiring}
        {...(failed && actions.error !== undefined && { errorMessage: t("client:admin.supporters.plan.change_failed", { reason: actions.error }) })}
        onConfirm={(close) => {
          setIsRetiring(true);
          setFailed(false);
          void actions.retire(plan).then((retired) => {
            setIsRetiring(false);
            if (retired) close();
            else setFailed(true);
          });
        }}
      />
    </DialogTrigger>
  );
}

/** The value of the "every" choice of the filters. */
const anyChoice = "any";

function Subscriptions({ filter, onFilterChange }: SupporterSettingsProps) {
  const t = useT();
  const { active, retired } = useSupporterPlans();
  const list = useSupporterSubscriptions(filter);
  const actions = useSubscriptionActions();
  const plans = [{ id: anyChoice, name: t("client:admin.supporters.filter.any_plan") }, ...[...active, ...retired].map((plan) => ({ id: plan.id, name: plan.name }))];
  const states = [
    { id: anyChoice, name: t("client:admin.supporters.filter.any_state") },
    ...supporterSubscriptionStates.map((state) => ({ id: state, name: t(stateLabels[state]) })),
  ];
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap gap-4">
        <Select
          label={t("client:admin.supporters.filter.plan")}
          value={filter.planId ?? anyChoice}
          onChange={(key) => {
            onFilterChange({ ...filter, planId: key === anyChoice || typeof key !== "string" ? undefined : key });
          }}
          items={plans}
        >
          {(item) => (
            <SelectItem id={item.id} textValue={item.name}>
              <span dir="auto">{item.name}</span>
            </SelectItem>
          )}
        </Select>
        <Select
          label={t("client:admin.supporters.filter.state")}
          value={filter.state ?? anyChoice}
          onChange={(key) => {
            const state = supporterSubscriptionStates.find((s) => s === key);
            onFilterChange({ ...filter, state });
          }}
          items={states}
        >
          {(item) => (
            <SelectItem id={item.id} textValue={item.name}>
              {item.name}
            </SelectItem>
          )}
        </Select>
      </div>
      <StatusMessage>{actions.status}</StatusMessage>
      <RecordList
        list={list}
        getId={(subscription) => subscription.id}
        empty={<EmptyState headingLevel={3} heading={t("client:admin.supporters.subscriptions.empty")} />}
      >
        {(subscription) => <SubscriptionRow subscription={subscription} actions={actions} />}
      </RecordList>
    </div>
  );
}

function SubscriptionRow({ subscription, actions }: { subscription: SupporterSubscription; actions: SubscriptionActions }) {
  const t = useT();
  const format = useFormatMoney();
  const who = subscription.username === undefined ? t("client:admin.supporters.subscription.deleted_account") : `@${subscription.username}`;
  return (
    <div className="flex flex-col gap-3">
      <h3 className="min-w-0 font-semibold wrap-anywhere">
        <span dir="ltr">{who}</span>
      </h3>
      <DescriptionList columns={2}>
        <DescriptionItem term={t("client:admin.supporters.subscription.state")}>{t(stateLabels[subscription.state])}</DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.subscription.plan")}>
          <span dir="auto" lang="">
            {subscription.plan_name}
          </span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.subscription.price")}>
          {t(subscription.interval === "year" ? "client:admin.supporters.subscription.per_year" : "client:admin.supporters.subscription.per_month", {
            amount: format(subscription.price),
          })}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.supporters.subscription.started_at")}>
          <DateTime value={subscription.started_at} />
        </DescriptionItem>
        {subscription.period_ends_at !== undefined && (
          <DescriptionItem term={t("client:admin.supporters.subscription.period_ends_at")}>
            <DateTime value={subscription.period_ends_at} />
          </DescriptionItem>
        )}
        {subscription.grace_ends_at !== undefined && (
          <DescriptionItem term={t("client:admin.supporters.subscription.grace_ends_at")}>
            <DateTime value={subscription.grace_ends_at} />
          </DescriptionItem>
        )}
        {subscription.scheduled_interval !== undefined && (
          <DescriptionItem term={t("client:admin.supporters.subscription.scheduled")}>
            {t(subscription.scheduled_interval === "year" ? "client:admin.supporters.subscription.scheduled_year" : "client:admin.supporters.subscription.scheduled_month")}
          </DescriptionItem>
        )}
      </DescriptionList>
      {isLive(subscription.state) && <CancelSubscription subscription={subscription} who={who} actions={actions} />}
    </div>
  );
}

/** Ending a subscription at once, after an alert dialog: the role goes now, without a refund. */
function CancelSubscription({ subscription, who, actions }: { subscription: SupporterSubscription; who: string; actions: SubscriptionActions }) {
  const t = useT();
  const name = `⁦${who}⁩`;
  const [isCanceling, setIsCanceling] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <div>
      <DialogTrigger>
        <Button size="sm" variant="danger" aria-label={t("client:admin.supporters.subscription.cancel_of", { name })}>
          {t("client:admin.supporters.subscription.cancel")}
        </Button>
        <ConfirmDialog
          title={t("client:admin.supporters.subscription.cancel.title", { name })}
          description={t("client:admin.supporters.subscription.cancel.description")}
          confirmLabel={t("client:admin.supporters.subscription.cancel")}
          cancelLabel={t("client:admin.supporters.cancel")}
          tone="danger"
          isPending={isCanceling}
          {...(failed && actions.error !== undefined && { errorMessage: t("client:admin.supporters.subscription.cancel_failed", { reason: actions.error }) })}
          onConfirm={(close) => {
            setIsCanceling(true);
            setFailed(false);
            void actions.cancel(subscription).then((canceled) => {
              setIsCanceling(false);
              if (canceled) close();
              else setFailed(true);
            });
          }}
        />
      </DialogTrigger>
    </div>
  );
}
