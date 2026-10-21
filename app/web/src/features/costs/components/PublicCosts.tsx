// SPDX-License-Identifier: AGPL-3.0-only
import { EmptyState, Link, RegionError, Spinner } from "@my/ui";
import { useQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { useT } from "../../../lib/i18n";
import { shellQueryReader } from "../../../lib/query";
import { instanceQueryOptions } from "../../about";
import { usePublicCosts } from "../hooks/usePublicCosts";
import { CostTotalsPanel, type CostTotalsPanelProps, CostTotalsView } from "./CostTotals";

export type PublicCostsProps = Omit<CostTotalsPanelProps, "children"> & {
  /** Today's date on the viewer's device, which ends the shown periods (ADR-0067 §5). */
  today: string;
};

/**
 * The body of the public costs page (`/about/costs`, FR-903, ADR-0067 §8), under its heading: what the instance
 * publishes of its running costs, by week, month or year, amortized or in cash, with the breakdown of the current
 * period by provider and category when the instance publishes it. The names and memos of the items and the individual
 * payments are never published. When the instance publishes nothing, the page says so.
 */
export function PublicCosts({ today, ...panel }: PublicCostsProps) {
  const t = useT();
  const api = useApi();
  const { data: instance } = useQuery({ ...instanceQueryOptions(api), ...shellQueryReader });
  const breakdown = instance?.finance_visibility === "breakdown";
  const costs = usePublicCosts(panel.granularity, panel.basis, today, breakdown);
  if (costs.status === "hidden") {
    return (
      <EmptyState
        headingLevel={2}
        heading={t("client:costs.public.hidden")}
        action={
          <Link href="/about" variant="standalone">
            {t("client:costs.public.about")}
          </Link>
        }
      />
    );
  }
  return (
    <div className="flex flex-col gap-6">
      <p>{t("client:costs.public.description")}</p>
      {instance?.finance_visibility === "totals" && <p className="text-fg-muted">{t("client:costs.public.totals_only")}</p>}
      <CostTotalsPanel {...panel}>
        {costs.status === "loading" && (
          <div aria-busy="true">
            <Spinner label={t("client:costs.public.loading")} />
          </div>
        )}
        {costs.status === "error" && (
          <RegionError
            heading={t("client:costs.public.failed")}
            reason={costs.reason}
            retryLabel={t("client:costs.public.retry")}
            onRetry={costs.retry}
            isRetrying={costs.isRetrying}
          />
        )}
        {costs.status === "ready" && <CostTotalsView totals={costs.totals} breakdown={costs.breakdown} />}
      </CostTotalsPanel>
    </div>
  );
}
