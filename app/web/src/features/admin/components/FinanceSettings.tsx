// SPDX-License-Identifier: AGPL-3.0-only
// The costs screen below its heading (ADR-0067 §7, ADR-0068 §9, manage_finance): the totals by week, month or year
// under tabs, amortized (the default) or in cash, with the rules that cut their periods next to them; the form that
// records a cost paid once or every N weeks, months or years; the recorded items, newest first, each with its
// schedule, dates and prices and the dialogs that change its label, revise its price and set its end, and canceling
// it after an alert dialog; and the state of the billing imports with importing now. The granularity and the basis
// are in the URL (web-client.md S3). Amounts and dates are formatted with Intl (ADR-0067 §10); names, providers and
// memos are the operator's words, shown with dir="auto". What a change did stays in a StatusMessage above the list;
// a failure stays there too (role="alert", web-client.md E2).
import {
  Button,
  ConfirmDialog,
  DescriptionItem,
  DescriptionList,
  Dialog,
  DialogTrigger,
  EmptyState,
  Radio,
  RadioGroup,
  Select,
  SelectItem,
  StatusMessage,
  Tab,
  TabList,
  TabPanel,
  Tabs,
  TextArea,
  TextField,
} from "@my/ui";
import { type ReactNode, useId, useRef, useState } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFocusAfterRemoval, useFocusHeadingAfterMove } from "../../../lib/useFocusAfterRemoval";
import { DateTime, DetailSection, RecordList, RequirePermission } from "../../moderation";
import {
  type BillingImport,
  type CostBasis,
  costBases,
  type CostCategory,
  costCategories,
  type CostGranularity,
  costGranularities,
  type CostItem,
  costMemoLimit,
  costNameLimit,
  type CostPeriodTotal,
  type CostSchedule,
  costSchedules,
  type CostTotals,
  maxCostInterval,
  priceOn,
  utcDate,
} from "../api/finance";
import {
  type CostItemActions,
  type LabelDraft,
  type PriceDraft,
  useBillingImports,
  useCostItemActions,
  useCostItems,
  useCostTotals,
  useFormatDay,
  useFormatMoney,
  useFormatTotals,
  useNewCostItemForm,
  useRunBillingImport,
} from "../hooks/useFinance";

const granularityLabels: Readonly<Record<CostGranularity, ClientMessageKey>> = {
  week: "client:admin.finance.granularity.week",
  month: "client:admin.finance.granularity.month",
  year: "client:admin.finance.granularity.year",
};

const basisLabels: Readonly<Record<CostBasis, { label: ClientMessageKey; description: ClientMessageKey }>> = {
  amortized: { label: "client:admin.finance.basis.amortized", description: "client:admin.finance.basis.amortized.description" },
  cash: { label: "client:admin.finance.basis.cash", description: "client:admin.finance.basis.cash.description" },
};

const scheduleLabels: Readonly<Record<CostSchedule, ClientMessageKey>> = {
  once: "client:admin.finance.schedule.once",
  weeks: "client:admin.finance.schedule.weeks",
  months: "client:admin.finance.schedule.months",
  years: "client:admin.finance.schedule.years",
};

const categoryLabels: Readonly<Record<CostCategory, ClientMessageKey>> = {
  compute: "client:admin.finance.category.compute",
  database: "client:admin.finance.category.database",
  storage: "client:admin.finance.category.storage",
  network: "client:admin.finance.category.network",
  domain: "client:admin.finance.category.domain",
  email: "client:admin.finance.category.email",
  software: "client:admin.finance.category.software",
  other: "client:admin.finance.category.other",
};

const monthKindLabels: Readonly<Record<BillingImport["months"][number]["kind"], ClientMessageKey>> = {
  estimate: "client:admin.finance.imports.kind.estimate",
  final: "client:admin.finance.imports.kind.final",
  correction: "client:admin.finance.imports.kind.correction",
};

const errorKindLabels: Readonly<Record<NonNullable<BillingImport["last_run"]>["error_kind"], ClientMessageKey>> = {
  none: "client:admin.finance.imports.error.none",
  permission: "client:admin.finance.imports.error.permission",
  not_enabled: "client:admin.finance.imports.error.not_enabled",
  not_found: "client:admin.finance.imports.error.not_found",
  throttled: "client:admin.finance.imports.error.throttled",
  request_cap: "client:admin.finance.imports.error.request_cap",
  bytes_cap: "client:admin.finance.imports.error.bytes_cap",
  invalid_response: "client:admin.finance.imports.error.invalid_response",
  unavailable: "client:admin.finance.imports.error.unavailable",
};

function isGranularity(key: unknown): key is CostGranularity {
  return costGranularities.some((item) => item === key);
}

function isBasis(key: unknown): key is CostBasis {
  return costBases.some((item) => item === key);
}

export type FinanceSettingsProps = {
  /** The granularity and the basis of the totals, from the URL. */
  granularity: CostGranularity;
  basis: CostBasis;
  /** Today's date on the viewer's device, which ends the shown periods (ADR-0067 §5). */
  today: string;
  /** Selects another granularity or basis: the route puts it into the URL. */
  onTotalsChange: (next: { granularity: CostGranularity; basis: CostBasis }) => void;
};

export function FinanceSettings(props: FinanceSettingsProps) {
  const t = useT();
  return (
    <RequirePermission permission="manage_finance">
      <div className="flex flex-col gap-8">
        <p className="text-fg-muted">{t("client:admin.finance.description")}</p>
        <DetailSection title={t("client:admin.finance.totals.title")}>
          <TotalsSection {...props} />
        </DetailSection>
        <DetailSection title={t("client:admin.finance.new.title")}>
          <NewCostItemForm />
        </DetailSection>
        <DetailSection title={t("client:admin.finance.list.title")}>
          <CostItemList today={props.today} />
        </DetailSection>
        <DetailSection title={t("client:admin.finance.imports.title")}>
          <BillingImports />
        </DetailSection>
      </div>
    </RequirePermission>
  );
}

function TotalsSection({ granularity, basis, today, onTotalsChange }: FinanceSettingsProps) {
  const t = useT();
  return (
    <div className="flex flex-col gap-4">
      <RadioGroup
        label={t("client:admin.finance.basis")}
        orientation="horizontal"
        value={basis}
        onChange={(value) => {
          if (isBasis(value)) onTotalsChange({ granularity, basis: value });
        }}
      >
        {costBases.map((item) => (
          <Radio key={item} value={item} description={t(basisLabels[item].description)}>
            {t(basisLabels[item].label)}
          </Radio>
        ))}
      </RadioGroup>
      <Tabs
        selectedKey={granularity}
        onSelectionChange={(key) => {
          if (isGranularity(key)) onTotalsChange({ granularity: key, basis });
        }}
      >
        <TabList aria-label={t("client:admin.finance.granularity")}>
          {costGranularities.map((item) => (
            <Tab key={item} id={item}>
              {t(granularityLabels[item])}
            </Tab>
          ))}
        </TabList>
        <TabPanel id={granularity}>
          <Totals granularity={granularity} basis={basis} today={today} />
        </TabPanel>
      </Tabs>
    </div>
  );
}

/** The name of a period in the viewer's locale: the month and year, the year, or the dates of an ISO week with its number. */
function usePeriodName() {
  const intl = useIntl();
  const t = useT();
  return (granularity: CostGranularity, period: CostPeriodTotal): string => {
    const start = utcDate(period.starts_on);
    if (granularity === "year") return intl.formatDate(start, { year: "numeric", timeZone: "UTC" });
    if (granularity === "month") return intl.formatDate(start, { year: "numeric", month: "long", timeZone: "UTC" });
    const range = intl.formatDateTimeRange(start, utcDate(period.ends_on), { year: "numeric", month: "short", day: "numeric", timeZone: "UTC" });
    return t("client:admin.finance.period.week", { range, week: period.label });
  };
}

function Totals({ granularity, basis, today }: { granularity: CostGranularity; basis: CostBasis; today: string }) {
  const t = useT();
  const totals = useCostTotals(granularity, basis, today);
  const periodName = usePeriodName();
  const formatTotals = useFormatTotals();
  const captionId = useId();
  const newestFirst = totals.periods.toReversed();
  const current = newestFirst[0];
  return (
    <div className="flex flex-col gap-4">
      <table aria-labelledby={captionId} className="w-full text-start">
        <caption id={captionId} className="sr-only">
          {t("client:admin.finance.totals.caption", { granularity: t(granularityLabels[granularity]), basis: t(basisLabels[basis].label) })}
        </caption>
        <thead>
          <tr className="border-b border-border">
            <th scope="col" className="py-2 pe-3 text-start font-semibold">
              {t("client:admin.finance.totals.period")}
            </th>
            <th scope="col" className="py-2 text-start font-semibold">
              {t("client:admin.finance.totals.total")}
            </th>
          </tr>
        </thead>
        <tbody>
          {newestFirst.map((period) => (
            <tr key={period.label} className="border-b border-border align-top">
              <th scope="row" className="py-2 pe-3 text-start font-normal">
                {periodName(granularity, period)}
              </th>
              <td className="py-2 tabular-nums wrap-anywhere">
                <span className="flex flex-wrap items-center gap-2">
                  {formatTotals(period.totals)}
                  {period.estimated && <span className="rounded-sm border border-border px-1 text-sm text-fg-muted">{t("client:admin.finance.totals.estimated")}</span>}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {current !== undefined && <Breakdown name={periodName(granularity, current)} period={current} />}
      <PeriodRules totals={totals} />
    </div>
  );
}

/** The breakdown of the current period by provider and category. */
function Breakdown({ name, period }: { name: string; period: CostPeriodTotal }) {
  const t = useT();
  const format = useFormatMoney();
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-2">
      <h3 id={headingId} className="font-semibold">
        {t("client:admin.finance.breakdown.title", { period: name })}
      </h3>
      {period.breakdown.length === 0 ? (
        <p className="text-fg-muted">{t("client:admin.finance.breakdown.empty")}</p>
      ) : (
        <ul className="flex flex-col gap-1">
          {period.breakdown.map((line) => (
            <li key={`${line.provider} ${line.category} ${line.amount.currency}`} className="flex flex-wrap gap-x-2">
              <span dir="auto" className="wrap-anywhere">
                {line.provider}
              </span>
              <span className="text-fg-muted">{t(categoryLabels[line.category])}</span>
              <span className="tabular-nums">{format(line.amount)}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/** How the totals cut their periods and place the payments (ADR-0067 §5, FR-902). */
function PeriodRules({ totals }: { totals: CostTotals }) {
  const t = useT();
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-2 text-fg-muted">
      <h3 id={headingId} className="font-semibold text-fg">
        {t("client:admin.finance.rules.title")}
      </h3>
      <ul className="flex list-disc flex-col gap-1 ps-6">
        <li>{t("client:admin.finance.rules.dates")}</li>
        <li>{t("client:admin.finance.rules.week_start")}</li>
        <li>{t(totals.basis === "amortized" ? "client:admin.finance.rules.amortized" : "client:admin.finance.rules.cash")}</li>
        <li>{t("client:admin.finance.rules.currencies")}</li>
        {totals.rules.sources.map((source) => (
          <li key={source.source}>{t("client:admin.finance.rules.source", { provider: source.provider, timeZone: source.time_zone })}</li>
        ))}
        {totals.rules.sources.length > 0 && <li>{t("client:admin.finance.rules.estimated")}</li>}
      </ul>
    </section>
  );
}

/** The fields of a label: name, provider, category and memo. */
function LabelFields({
  label,
  onChange,
  errors,
  autoFocus = false,
}: {
  label: LabelDraft;
  onChange: (patch: Partial<LabelDraft>) => void;
  errors: { name?: string; provider?: string; memo?: string };
  autoFocus?: boolean;
}) {
  const t = useT();
  const categories = costCategories.map((id) => ({ id, name: t(categoryLabels[id]) }));
  return (
    <>
      <TextField
        label={t("client:admin.finance.name")}
        description={t("client:admin.finance.name.description", { max: String(costNameLimit) })}
        value={label.name}
        onChange={(name) => {
          onChange({ name });
        }}
        isRequired
        autoFocus={autoFocus}
        {...(errors.name !== undefined && { isInvalid: true, errorMessage: errors.name })}
      />
      <TextField
        label={t("client:admin.finance.provider")}
        description={t("client:admin.finance.provider.description")}
        value={label.provider}
        onChange={(provider) => {
          onChange({ provider });
        }}
        isRequired
        {...(errors.provider !== undefined && { isInvalid: true, errorMessage: errors.provider })}
      />
      <Select
        label={t("client:admin.finance.category")}
        value={label.category}
        onChange={(key) => {
          const choice = categories.find((item) => item.id === key);
          if (choice !== undefined) onChange({ category: choice.id });
        }}
        items={categories}
      >
        {(choice) => (
          <SelectItem id={choice.id} textValue={choice.name}>
            {choice.name}
          </SelectItem>
        )}
      </Select>
      <TextArea
        label={t("client:admin.finance.memo")}
        description={t("client:admin.finance.memo.description")}
        value={label.memo}
        onChange={(memo) => {
          onChange({ memo });
        }}
        rows={3}
        characterLimit={costMemoLimit}
        formatRemaining={(remaining) =>
          remaining < 0 ? t("client:admin.settings.characters_over", { count: -remaining }) : t("client:admin.settings.characters_left", { count: remaining })
        }
        {...(errors.memo !== undefined && { isInvalid: true, errorMessage: errors.memo })}
      />
    </>
  );
}

/** The fields of a price: the amount in the major unit and the ISO 4217 code of its currency. */
function PriceFields({
  price,
  onChange,
  errors,
}: {
  price: PriceDraft;
  onChange: (patch: Partial<PriceDraft>) => void;
  errors: { amount?: string; currency?: string };
}) {
  const t = useT();
  return (
    <div className="flex flex-wrap gap-4">
      <TextField
        label={t("client:admin.finance.amount")}
        description={t("client:admin.finance.amount.description")}
        value={price.amount}
        onChange={(amount) => {
          onChange({ amount });
        }}
        inputMode="decimal"
        isRequired
        className="grow"
        {...(errors.amount !== undefined && { isInvalid: true, errorMessage: errors.amount })}
      />
      <TextField
        label={t("client:admin.finance.currency")}
        description={t("client:admin.finance.currency.description")}
        value={price.currency}
        onChange={(currency) => {
          onChange({ currency: currency.toUpperCase() });
        }}
        maxLength={3}
        spellCheck="false"
        isRequired
        {...(errors.currency !== undefined && { isInvalid: true, errorMessage: errors.currency })}
      />
    </div>
  );
}

function NewCostItemForm() {
  const t = useT();
  const items = useCostItems();
  const form = useNewCostItemForm(items.items[0]?.prices[0]?.price.currency ?? "USD");
  const { draft, errors } = form;
  return (
    <form onSubmit={form.submit} className="flex flex-col gap-6" noValidate>
      <LabelFields
        label={draft}
        onChange={form.change}
        errors={errors}
      />
      <RadioGroup
        label={t("client:admin.finance.schedule")}
        description={t("client:admin.finance.schedule.description")}
        value={draft.schedule}
        onChange={(value) => {
          const schedule = costSchedules.find((item) => item === value);
          if (schedule !== undefined) form.change({ schedule });
        }}
      >
        {costSchedules.map((item) => (
          <Radio key={item} value={item}>
            {t(scheduleLabels[item])}
          </Radio>
        ))}
      </RadioGroup>
      {draft.schedule !== "once" && (
        <TextField
          label={t("client:admin.finance.interval")}
          description={t("client:admin.finance.interval.description", { max: String(maxCostInterval) })}
          value={draft.interval}
          onChange={(interval) => {
            form.change({ interval });
          }}
          inputMode="numeric"
          isRequired
          {...(errors.interval !== undefined && { isInvalid: true, errorMessage: errors.interval })}
        />
      )}
      <TextField
        type="date"
        label={t(draft.schedule === "once" ? "client:admin.finance.paid_on" : "client:admin.finance.starts_on")}
        description={t(draft.schedule === "once" ? "client:admin.finance.paid_on.description" : "client:admin.finance.starts_on.description")}
        value={draft.startsOn}
        onChange={(startsOn) => {
          form.change({ startsOn });
        }}
        isRequired
        {...(errors.startsOn !== undefined && { isInvalid: true, errorMessage: errors.startsOn })}
      />
      {draft.schedule !== "once" && (
        <TextField
          type="date"
          label={t("client:admin.finance.ends_on")}
          description={t("client:admin.finance.ends_on.description")}
          value={draft.endsOn}
          onChange={(endsOn) => {
            form.change({ endsOn });
          }}
          {...(errors.endsOn !== undefined && { isInvalid: true, errorMessage: errors.endsOn })}
        />
      )}
      <PriceFields price={draft} onChange={form.change} errors={errors} />
      {form.failure !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.finance.create_failed", { reason: form.failure })}
        </p>
      )}
      <div>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:admin.finance.create")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
    </form>
  );
}

function CostItemList({ today }: { today: string }) {
  const t = useT();
  const list = useCostItems();
  const listRef = useRef<HTMLUListElement>(null);
  const [status, setStatus] = useState<string | undefined>(undefined);
  // A change keeps the row and may close its dialog: focus moves to the row's heading. A canceled item leaves the
  // list: focus moves to the next row's button.
  const changed = useFocusHeadingAfterMove(listRef);
  const removed = useFocusAfterRemoval(
    listRef,
    list.items.map((item) => item.id),
  );
  const actions = useCostItemActions((item, done, gone) => {
    if (gone) removed(item.id);
    else changed(item.id);
    setStatus(done);
  });
  return (
    <div className="flex flex-col gap-3">
      <StatusMessage>{status}</StatusMessage>
      {actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.finance.change_failed", { reason: actions.error })}
        </p>
      )}
      <RecordList
        list={list}
        getId={(item) => item.id}
        listRef={listRef}
        empty={<EmptyState headingLevel={3} heading={t("client:admin.finance.empty")} description={t("client:admin.finance.empty.description")} />}
      >
        {(item) => <CostItemRow item={item} today={today} actions={actions} />}
      </RecordList>
    </div>
  );
}

/** How often an item is paid, in words: "Once", "Every 3 months". */
function useScheduleText() {
  const t = useT();
  return (item: Pick<CostItem, "schedule" | "interval">): string => {
    switch (item.schedule) {
      case "once":
        return t("client:admin.finance.every.once");
      case "weeks":
        return t("client:admin.finance.every.weeks", { count: item.interval });
      case "months":
        return t("client:admin.finance.every.months", { count: item.interval });
      case "years":
        return t("client:admin.finance.every.years", { count: item.interval });
    }
  };
}

function CostItemRow({ item, today, actions }: { item: CostItem; today: string; actions: CostItemActions }) {
  const t = useT();
  const formatDay = useFormatDay();
  const format = useFormatMoney();
  const scheduleText = useScheduleText();
  // Before the start date, the price of the first payment.
  const price = priceOn(item, today) ?? item.prices[0]?.price;
  return (
    <div className="flex flex-col gap-3">
      <h3 tabIndex={-1} dir="auto" className="font-semibold wrap-anywhere outline-none">
        {item.name}
      </h3>
      <DescriptionList>
        <DescriptionItem term={t("client:admin.finance.provider")}>
          <span dir="auto">{item.provider}</span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.finance.category")}>{t(categoryLabels[item.category])}</DescriptionItem>
        <DescriptionItem term={t("client:admin.finance.schedule")}>{scheduleText(item)}</DescriptionItem>
        <DescriptionItem term={t(item.schedule === "once" ? "client:admin.finance.paid_on" : "client:admin.finance.starts_on")}>
          {formatDay(item.starts_on)}
        </DescriptionItem>
        {item.schedule !== "once" && (
          <DescriptionItem term={t("client:admin.finance.ends_on")}>
            {item.ends_on === undefined ? t("client:admin.finance.no_end") : formatDay(item.ends_on)}
          </DescriptionItem>
        )}
        {price !== undefined && (
          <DescriptionItem term={t("client:admin.finance.price")}>
            <span className="tabular-nums">{format(price)}</span>
          </DescriptionItem>
        )}
        {item.prices.length > 1 && (
          <DescriptionItem term={t("client:admin.finance.prices")}>
            <ul className="flex flex-col">
              {item.prices.map((revision) => (
                <li key={`${revision.effective_on} ${revision.revised_at}`} className="tabular-nums">
                  {t("client:admin.finance.price_from", { date: formatDay(revision.effective_on), amount: format(revision.price) })}
                </li>
              ))}
            </ul>
          </DescriptionItem>
        )}
        {item.memo !== "" && (
          <DescriptionItem term={t("client:admin.finance.memo")}>
            <span dir="auto" lang="" className="whitespace-pre-wrap wrap-anywhere">
              {item.memo}
            </span>
          </DescriptionItem>
        )}
        <DescriptionItem term={t("client:admin.finance.created_at")}>
          <DateTime value={item.created_at} />
        </DescriptionItem>
      </DescriptionList>
      <div className="flex flex-wrap gap-2">
        <EditLabel item={item} actions={actions} />
        <RevisePrice item={item} actions={actions} />
        {item.schedule !== "once" && <SetEnd item={item} actions={actions} />}
        <CancelItem item={item} actions={actions} />
      </div>
    </div>
  );
}

type ItemActionProps = { item: CostItem; actions: CostItemActions };

/** A dialog's form that closes once `save` worked and keeps a failure inside. */
function DialogForm({
  save,
  close,
  actions,
  children,
}: {
  save: () => Promise<boolean> | undefined;
  close: () => void;
  actions: CostItemActions;
  children: ReactNode;
}) {
  const t = useT();
  const [isSaving, setIsSaving] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <form
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        if (isSaving) return;
        const saving = save();
        if (saving === undefined) return;
        setIsSaving(true);
        setFailed(false);
        void saving.then((saved) => {
          setIsSaving(false);
          if (saved) close();
          else setFailed(true);
        });
      }}
      className="flex flex-col gap-4"
    >
      {children}
      {failed && actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.finance.change_failed", { reason: actions.error })}
        </p>
      )}
      <div className="flex flex-wrap justify-end gap-2">
        <Button onPress={close}>{t("client:admin.finance.cancel_dialog")}</Button>
        <Button type="submit" variant="primary" isPending={isSaving}>
          {t("client:admin.finance.save")}
        </Button>
      </div>
    </form>
  );
}

function EditLabel({ item, actions }: ItemActionProps) {
  const t = useT();
  return (
    <DialogTrigger>
      <Button size="sm" aria-label={t("client:admin.finance.edit_one", { name: item.name })}>
        {t("client:admin.finance.edit")}
      </Button>
      <Dialog title={t("client:admin.finance.edit.title")} closeLabel={t("client:admin.finance.close")} isDismissable={false} size="md">
        {({ close }) => <EditLabelForm item={item} actions={actions} close={close} />}
      </Dialog>
    </DialogTrigger>
  );
}

function EditLabelForm({ item, actions, close }: ItemActionProps & { close: () => void }) {
  const [label, setLabel] = useState<LabelDraft>({ name: item.name, provider: item.provider, category: item.category, memo: item.memo });
  const [submitted, setSubmitted] = useState(false);
  const errors = submitted ? actions.labelErrors(label) : {};
  return (
    <DialogForm
      close={close}
      actions={actions}
      save={() => {
        setSubmitted(true);
        if (Object.keys(actions.labelErrors(label)).length > 0) return undefined;
        return actions.relabel(item, label);
      }}
    >
      <LabelFields
        label={label}
        onChange={(patch) => {
          setLabel((current) => ({ ...current, ...patch }));
        }}
        errors={errors}
        autoFocus
      />
    </DialogForm>
  );
}

function RevisePrice({ item, actions }: ItemActionProps) {
  const t = useT();
  return (
    <DialogTrigger>
      <Button size="sm" aria-label={t("client:admin.finance.revise_one", { name: item.name })}>
        {t("client:admin.finance.revise")}
      </Button>
      <Dialog title={t("client:admin.finance.revise.title", { name: item.name })} closeLabel={t("client:admin.finance.close")} isDismissable={false} size="md">
        {({ close }) => <RevisePriceForm item={item} actions={actions} close={close} />}
      </Dialog>
    </DialogTrigger>
  );
}

function RevisePriceForm({ item, actions, close }: ItemActionProps & { close: () => void }) {
  const t = useT();
  const last = item.prices.at(-1)?.price;
  const [effectiveOn, setEffectiveOn] = useState("");
  const [price, setPrice] = useState<PriceDraft>({ amount: "", currency: last?.currency ?? "USD" });
  const [submitted, setSubmitted] = useState(false);
  const dateError = (value: string) => {
    if (value === "") return t("client:admin.finance.required");
    if (value < item.starts_on) return t("client:admin.finance.effective_on.before_start");
    return undefined;
  };
  const priceErrors = submitted ? actions.priceErrors(price) : {};
  const effectiveError = submitted ? dateError(effectiveOn) : undefined;
  return (
    <DialogForm
      close={close}
      actions={actions}
      save={() => {
        setSubmitted(true);
        if (dateError(effectiveOn) !== undefined || Object.keys(actions.priceErrors(price)).length > 0) return undefined;
        return actions.revise(item, effectiveOn, price);
      }}
    >
      <TextField
        type="date"
        label={t("client:admin.finance.effective_on")}
        description={t("client:admin.finance.effective_on.description")}
        value={effectiveOn}
        onChange={setEffectiveOn}
        isRequired
        autoFocus
        {...(effectiveError !== undefined && { isInvalid: true, errorMessage: effectiveError })}
      />
      <PriceFields
        price={price}
        onChange={(patch) => {
          setPrice((current) => ({ ...current, ...patch }));
        }}
        errors={priceErrors}
      />
    </DialogForm>
  );
}

function SetEnd({ item, actions }: ItemActionProps) {
  const t = useT();
  return (
    <DialogTrigger>
      <Button size="sm" aria-label={t("client:admin.finance.end_one", { name: item.name })}>
        {t("client:admin.finance.end")}
      </Button>
      <Dialog title={t("client:admin.finance.end.title", { name: item.name })} closeLabel={t("client:admin.finance.close")} isDismissable={false} size="md">
        {({ close }) => <SetEndForm item={item} actions={actions} close={close} />}
      </Dialog>
    </DialogTrigger>
  );
}

function SetEndForm({ item, actions, close }: ItemActionProps & { close: () => void }) {
  const t = useT();
  const [endsOn, setEndsOn] = useState(item.ends_on ?? "");
  const [submitted, setSubmitted] = useState(false);
  const error = (value: string) => (value !== "" && value < item.starts_on ? t("client:admin.finance.ends_on.before_start") : undefined);
  const endError = submitted ? error(endsOn) : undefined;
  return (
    <DialogForm
      close={close}
      actions={actions}
      save={() => {
        setSubmitted(true);
        if (error(endsOn) !== undefined) return undefined;
        return actions.setEnd(item, endsOn === "" ? undefined : endsOn);
      }}
    >
      <TextField
        type="date"
        label={t("client:admin.finance.ends_on")}
        description={t("client:admin.finance.end.description")}
        value={endsOn}
        onChange={setEndsOn}
        autoFocus
        {...(endError !== undefined && { isInvalid: true, errorMessage: endError })}
      />
    </DialogForm>
  );
}

/** Canceling an item, after an alert dialog: it leaves every period, past ones included, and cannot be undone. */
function CancelItem({ item, actions }: ItemActionProps) {
  const t = useT();
  const [isCanceling, setIsCanceling] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <DialogTrigger>
      <Button size="sm" variant="danger" aria-label={t("client:admin.finance.cancel_one", { name: item.name })}>
        {t("client:admin.finance.cancel_item")}
      </Button>
      <ConfirmDialog
        title={t("client:admin.finance.cancel.title", { name: item.name })}
        description={t("client:admin.finance.cancel.description")}
        confirmLabel={t("client:admin.finance.cancel_item")}
        cancelLabel={t("client:admin.finance.cancel_dialog")}
        isPending={isCanceling}
        {...(failed && actions.error !== undefined && { errorMessage: t("client:admin.finance.change_failed", { reason: actions.error }) })}
        onConfirm={(close) => {
          setIsCanceling(true);
          setFailed(false);
          void actions.cancel(item).then((canceled) => {
            setIsCanceling(false);
            if (canceled) close();
            else setFailed(true);
          });
        }}
      />
    </DialogTrigger>
  );
}

function BillingImports() {
  const t = useT();
  const sources = useBillingImports();
  const run = useRunBillingImport();
  if (sources.length === 0) {
    return <EmptyState headingLevel={3} heading={t("client:admin.finance.imports.none")} description={t("client:admin.finance.imports.none.description")} />;
  }
  return (
    <div className="flex flex-col gap-3">
      <StatusMessage>{run.status}</StatusMessage>
      {run.error !== undefined && (
        <p role="alert" className="text-danger">
          {run.error}
        </p>
      )}
      <ul className="flex flex-col divide-y divide-border">
        {sources.map((source) => (
          <li key={source.source} className="@container py-3">
            <BillingImportRow source={source} isPending={run.pendingSource === source.source} onRun={run.run} />
          </li>
        ))}
      </ul>
    </div>
  );
}

function BillingImportRow({ source, isPending, onRun }: { source: BillingImport; isPending: boolean; onRun: (source: BillingImport) => void }) {
  const t = useT();
  const intl = useIntl();
  const formatTotals = useFormatTotals();
  const count = (used: number, max: number | undefined) =>
    max === undefined ? intl.formatNumber(used) : t("client:admin.finance.imports.used_of", { used: intl.formatNumber(used), max: intl.formatNumber(max) });
  return (
    <div className="flex flex-col gap-3">
      <h3 className="font-semibold">{source.provider}</h3>
      <DescriptionList>
        <DescriptionItem term={t("client:admin.finance.imports.time_zone")}>{source.time_zone}</DescriptionItem>
        <DescriptionItem term={t("client:admin.finance.imports.last_success")}>
          {source.last_success_at === undefined ? t("client:admin.finance.imports.never") : <DateTime value={source.last_success_at} />}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.finance.imports.last_run")}>
          {source.last_run === undefined ? (
            t("client:admin.finance.imports.never")
          ) : (
            <span className="flex flex-col">
              <DateTime value={source.last_run.occurred_at} />
              <span>{t(errorKindLabels[source.last_run.error_kind])}</span>
            </span>
          )}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.finance.imports.requests_today")}>{count(source.requests_today, source.max_requests_per_day)}</DescriptionItem>
        <DescriptionItem term={t("client:admin.finance.imports.requests_month")}>{count(source.requests_this_month, source.max_requests_per_month)}</DescriptionItem>
        {source.source === "gcp" && (
          <DescriptionItem term={t("client:admin.finance.imports.bytes")}>
            {intl.formatNumber(source.bytes_billed_this_month, { style: "unit", unit: "byte", notation: "compact", unitDisplay: "narrow" })}
          </DescriptionItem>
        )}
      </DescriptionList>
      {source.months.length > 0 && (
        <ul aria-label={t("client:admin.finance.imports.months", { provider: source.provider })} className="flex flex-col gap-1">
          {source.months.map((month) => (
            <li key={month.month} className="flex flex-wrap gap-x-2">
              <span>{intl.formatDate(utcDate(`${month.month}-01`), { year: "numeric", month: "long", timeZone: "UTC" })}</span>
              <span className="text-fg-muted">{t(monthKindLabels[month.kind])}</span>
              <span className="tabular-nums">{formatTotals(month.totals)}</span>
            </li>
          ))}
        </ul>
      )}
      <div>
        <Button
          size="sm"
          isPending={isPending}
          aria-label={t("client:admin.finance.imports.run_one", { provider: source.provider })}
          onPress={() => {
            onRun(source);
          }}
        >
          {t("client:admin.finance.imports.run")}
        </Button>
      </div>
    </div>
  );
}
