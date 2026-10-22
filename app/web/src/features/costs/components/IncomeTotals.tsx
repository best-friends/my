// SPDX-License-Identifier: AGPL-3.0-only
// The totals of the instance's income as the administrators' costs screen, the public costs page and the finances
// page show them (ADR-0073 §5–§7, FR-916, FR-917): the table of the periods, newest first, with the net income of
// each currency, the gross and the fees on the costs screen only, and the balance against the costs where the answer
// carries it; the members of each supporter plan now where they are shown; and the rules that date and place the
// payments. Nothing here names a payment or a subscriber. Plan names are the operators' words, shown with dir="auto".
import { RegionError, Spinner } from "@my/ui";
import { useId } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import type { CostBasis, CostGranularity, Money } from "../api/costs";
import type { IncomePeriodTotal, IncomePlanCount, IncomeTotals } from "../api/income";
import { useFormatTotals, usePeriodName } from "../hooks/useCostFormat";

const granularityLabels: Readonly<Record<CostGranularity, ClientMessageKey>> = {
  week: "client:costs.granularity.week",
  month: "client:costs.granularity.month",
  year: "client:costs.granularity.year",
};

const basisLabels: Readonly<Record<CostBasis, ClientMessageKey>> = {
  amortized: "client:costs.basis.amortized",
  cash: "client:costs.basis.cash",
};

/** A period of the income totals, with the gross and the fees on the administrators' screen. */
type ShownIncomePeriod = IncomePeriodTotal & { gross?: readonly Money[]; fee?: readonly Money[] };

/** The income totals of any of the screens: the administrators' periods also have the gross and the fees. */
type ShownIncomeTotals = Omit<IncomeTotals, "periods"> & { periods: readonly ShownIncomePeriod[] };

export type IncomeTotalsViewProps = {
  totals: ShownIncomeTotals;
  /** Shows the gross and the provider's fees of each period: the costs screen only (ADR-0073 §6). */
  detailed?: boolean;
  /** The level of the headings of the members of each plan and of the rules: 3 below an h2, 4 below an h3. */
  headingLevel?: 3 | 4;
};

/** Formats a balance of each currency with its sign, never converted: "+¥1,200, −$3.40". */
function useFormatBalance(): (balance: readonly Money[]) => string {
  const intl = useIntl();
  const t = useT();
  return (balance) => {
    if (balance.length === 0) return t("client:costs.totals.none");
    const amounts = balance.map((money) =>
      intl.formatNumber(Number(money.amount), { style: "currency", currency: money.currency, signDisplay: "exceptZero" }),
    );
    return intl.formatList(amounts, { type: "unit", style: "short" });
  };
}

/** The income totals of the shown periods, newest first, the members of each plan, and the rules that place them. */
export function IncomeTotalsView({ totals, detailed = false, headingLevel = 3 }: IncomeTotalsViewProps) {
  const t = useT();
  const periodName = usePeriodName();
  const formatTotals = useFormatTotals();
  const formatBalance = useFormatBalance();
  const captionId = useId();
  const { granularity, basis } = totals;
  const newestFirst = totals.periods.toReversed();
  // The answer carries the balance only where it is shown: on the public page, while the costs are public too.
  const balance = totals.periods.some((period) => period.balance !== undefined);
  return (
    <div className="flex flex-col gap-4">
      <table aria-labelledby={captionId} className="w-full text-start">
        <caption id={captionId} className="sr-only">
          {t("client:income.totals.caption", { granularity: t(granularityLabels[granularity]), basis: t(basisLabels[basis]) })}
        </caption>
        <thead>
          <tr className="border-b border-border">
            <th scope="col" className="py-2 pe-3 text-start font-semibold">
              {t("client:costs.totals.period")}
            </th>
            {detailed && (
              <>
                <th scope="col" className="py-2 pe-3 text-start font-semibold">
                  {t("client:income.totals.gross")}
                </th>
                <th scope="col" className="py-2 pe-3 text-start font-semibold">
                  {t("client:income.totals.fee")}
                </th>
              </>
            )}
            <th scope="col" className="py-2 pe-3 text-start font-semibold">
              {t("client:income.totals.net")}
            </th>
            {balance && (
              <th scope="col" className="py-2 text-start font-semibold">
                {t("client:income.totals.balance")}
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {newestFirst.map((period) => (
            <tr key={period.label} className="border-b border-border align-top">
              <th scope="row" className="py-2 pe-3 text-start font-normal">
                {periodName(granularity, period)}
              </th>
              {detailed && (
                <>
                  <td className="py-2 pe-3 tabular-nums wrap-anywhere">{formatTotals(period.gross ?? [])}</td>
                  <td className="py-2 pe-3 tabular-nums wrap-anywhere">{formatTotals(period.fee ?? [])}</td>
                </>
              )}
              <td className="py-2 pe-3 tabular-nums wrap-anywhere">{formatTotals(period.net)}</td>
              {balance && <td className="py-2 tabular-nums wrap-anywhere">{formatBalance(period.balance ?? [])}</td>}
            </tr>
          ))}
        </tbody>
      </table>
      {totals.plans !== undefined && <PlanMembers plans={totals.plans} headingLevel={headingLevel} />}
      <IncomeRules totals={totals} balance={balance} headingLevel={headingLevel} />
    </div>
  );
}

/** The members of each supporter plan now (ADR-0073 §7): never per period, never the plans' amounts. */
function PlanMembers({ plans, headingLevel }: { plans: readonly IncomePlanCount[]; headingLevel: 3 | 4 }) {
  const t = useT();
  const headingId = useId();
  const Heading = headingLevel === 3 ? "h3" : "h4";
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-2">
      <Heading id={headingId} className="font-semibold">
        {t("client:income.plans.title")}
      </Heading>
      <p className="text-fg-muted">{t("client:income.plans.description")}</p>
      {plans.length === 0 ? (
        <p className="text-fg-muted">{t("client:income.plans.empty")}</p>
      ) : (
        <ul className="flex flex-col gap-1">
          {plans.map((plan) => (
            <li key={plan.plan_id} className="flex flex-wrap gap-x-2">
              {/* The administrator's words, in a language the app does not know (web-client.md Y10). */}
              <span dir="auto" lang="" className="wrap-anywhere">
                {plan.name}
              </span>
              <span className="tabular-nums">{t("client:income.plans.members", { count: plan.members })}</span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/** How the income totals date and place the payments (ADR-0073 §3, §5), and what the balance is. */
function IncomeRules({ totals, balance, headingLevel }: { totals: ShownIncomeTotals; balance: boolean; headingLevel: 3 | 4 }) {
  const t = useT();
  const intl = useIntl();
  const headingId = useId();
  const Heading = headingLevel === 3 ? "h3" : "h4";
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-2 text-fg-muted">
      <Heading id={headingId} className="font-semibold text-fg">
        {t("client:income.rules.title")}
      </Heading>
      <ul className="flex list-disc flex-col gap-1 ps-6">
        <li>{t("client:income.rules.net")}</li>
        {totals.rules.providers.map((provider) => (
          <li key={provider.provider}>{t("client:income.rules.provider", { provider: provider.provider, timeZone: provider.time_zone })}</li>
        ))}
        <li>
          {totals.basis === "amortized"
            ? t("client:income.rules.amortized", { days: intl.formatNumber(totals.rules.amortize_after_days) })
            : t("client:income.rules.cash")}
        </li>
        {balance && <li>{t("client:income.rules.balance")}</li>}
      </ul>
    </section>
  );
}

/** The income of a page as its hook gives it, once the instance is known to show it. */
type ShownIncome =
  | { status: "loading" }
  | { status: "error"; reason: string; retry: () => void; isRetrying: boolean }
  | { status: "ready"; totals: ShownIncomeTotals };

/** The income totals of the public costs page and the finances page while they load, when they fail, and once read. */
export function IncomeState({ income }: { income: ShownIncome }) {
  const t = useT();
  if (income.status === "loading") {
    return (
      <div aria-busy="true">
        <Spinner label={t("client:income.loading")} />
      </div>
    );
  }
  if (income.status === "error") {
    return (
      <RegionError
        heading={t("client:income.failed")}
        reason={income.reason}
        retryLabel={t("client:income.retry")}
        onRetry={income.retry}
        isRetrying={income.isRetrying}
      />
    );
  }
  return <IncomeTotalsView totals={income.totals} />;
}
