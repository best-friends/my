// SPDX-License-Identifier: AGPL-3.0-only
import { RegionError, Spinner } from "@my/ui";
import { useT } from "../../../lib/i18n";
import { DetailSection, MissingPermission } from "../../moderation";
import type { FinanceViewerSettings } from "../api/costs";
import { useFinanceAccess, useFinanceCosts } from "../hooks/useFinanceCosts";
import { CostTotalsPanel, type CostTotalsPanelProps, CostTotalsView } from "./CostTotals";

/** What the instance shows of the items to the holders of view_finance, for the sentence that says it. */
function shownOfItems(viewer: FinanceViewerSettings): "both" | "names" | "memos" | "none" {
  if (viewer.item_names && viewer.item_memos) return "both";
  if (viewer.item_names) return "names";
  if (viewer.item_memos) return "memos";
  return "none";
}

export type FinanceCostsProps = Omit<CostTotalsPanelProps, "children"> & {
  /** Today's date on the viewer's device, which ends the shown periods (ADR-0067 §5). */
  today: string;
};

/**
 * The body of the finances page (`/finance`, ADR-0070 §5), under its heading, for the holders of view_finance: the
 * totals of the instance's costs by week, month or year, amortized or in cash, always with the breakdown of the
 * current period by provider and category, and the lines of its items while the instance shows their names or memos
 * (a holder of manage_finance reads them all). It says what the instance does not show. Recording and changing the
 * costs is the costs screen's (`/admin/finance`). The income is not shown yet. A viewer without the permission sees why.
 */
export function FinanceCosts({ today, ...panel }: FinanceCostsProps) {
  const t = useT();
  const access = useFinanceAccess();
  const costs = useFinanceCosts(panel.granularity, panel.basis, today);
  if (!access.allowed) return <MissingPermission permission="view_finance" />;
  return (
    <div className="flex flex-col gap-6">
      <p>{t("client:finance.description")}</p>
      {costs.status === "ready" && !access.manage && (
        <p className="text-fg-muted">
          {t("client:finance.items_shown", { shown: shownOfItems(costs.totals.viewer) })}
        </p>
      )}
      <DetailSection title={t("client:finance.costs.title")}>
        <CostTotalsPanel {...panel}>
          {costs.status === "loading" && (
            <div aria-busy="true">
              <Spinner label={t("client:finance.loading")} />
            </div>
          )}
          {costs.status === "error" && (
            <RegionError
              heading={t("client:finance.failed")}
              reason={costs.reason}
              retryLabel={t("client:finance.retry")}
              onRetry={costs.retry}
              isRetrying={costs.isRetrying}
            />
          )}
          {costs.status === "ready" && <CostTotalsView totals={costs.totals} breakdown items={costs.items} />}
        </CostTotalsPanel>
      </DetailSection>
    </div>
  );
}
