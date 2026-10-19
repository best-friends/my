// SPDX-License-Identifier: AGPL-3.0-only
// The appeals screen below its heading (FR-707): the appeals that await a decision and those decided, newest
// first. The tabs follow the WAI-ARIA APG tabs pattern with automatic activation (React Aria Tabs), kept in the
// URL by the route (web-client.md S3); only the selected panel is mounted. Each row's heading links to the
// appeal's page, where it is decided.
import { DescriptionItem, DescriptionList, EmptyState, Link, Tab, TabList, TabPanel, Tabs } from "@my/ui";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { type Appeal, type AppealFilter, appealFilters } from "../api/appeals";
import { appealStateLabels, appealTitle, useAppeals } from "../hooks/useAppeals";
import { accountActionStateLabels } from "../hooks/useReports";
import { DateTime, RecordList, RequirePermission } from "./parts";
import { AccountByIdLabel } from "./reportParts";

const filterLabels: Readonly<Record<AppealFilter, ClientMessageKey>> = {
  pending: "client:admin.appeals.filter.pending",
  decided: "client:admin.appeals.filter.decided",
};

function isFilter(key: unknown): key is AppealFilter {
  return appealFilters.some((filter) => filter === key);
}

/** The address of an appeal's page. */
function appealPath(id: string): string {
  return `/admin/appeals/${encodeURIComponent(id)}`;
}

export type AppealQueueProps = {
  /** The selected tab, from the URL. */
  filter: AppealFilter;
  /** Selects another tab: the route puts it into the URL. */
  onFilterChange: (filter: AppealFilter) => void;
};

export function AppealQueue(props: AppealQueueProps) {
  return (
    <RequirePermission permission="manage_accounts">
      <AppealTabs {...props} />
    </RequirePermission>
  );
}

function AppealTabs({ filter, onFilterChange }: AppealQueueProps) {
  const t = useT();
  return (
    <Tabs
      selectedKey={filter}
      onSelectionChange={(key) => {
        if (isFilter(key)) onFilterChange(key);
      }}
    >
      <TabList aria-label={t("client:admin.appeals.filter.label")}>
        {appealFilters.map((item) => (
          <Tab key={item} id={item}>
            {t(filterLabels[item])}
          </Tab>
        ))}
      </TabList>
      <TabPanel id={filter}>
        <AppealList filter={filter} />
      </TabPanel>
    </Tabs>
  );
}

function AppealList({ filter }: { filter: AppealFilter }) {
  const t = useT();
  const list = useAppeals(filter);
  const empty =
    filter === "pending" ? (
      <EmptyState headingLevel={2} heading={t("client:admin.appeals.empty.pending")} description={t("client:admin.appeals.empty.pending.description")} />
    ) : (
      <EmptyState headingLevel={2} heading={t("client:admin.appeals.empty.decided")} />
    );
  return (
    <RecordList list={list} getId={(appeal) => appeal.id} empty={empty}>
      {(appeal) => <AppealRow appeal={appeal} />}
    </RecordList>
  );
}

function AppealRow({ appeal }: { appeal: Appeal }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-2">
      <h2 className="font-semibold">
        <Link href={appealPath(appeal.id)} variant="standalone" className="wrap-anywhere">
          {appealTitle(t, appeal)}
        </Link>
      </h2>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.appeal.account")}>
          <AccountByIdLabel id={appeal.account_id} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.appeal.submitted_at")}>
          <DateTime value={appeal.submitted_at} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.appeal.action_state")}>{t(accountActionStateLabels[appeal.action.state])}</DescriptionItem>
        {appeal.state !== "pending" && (
          <>
            <DescriptionItem term={t("client:admin.appeal.decision")}>
              <span className="font-semibold">{t(appealStateLabels[appeal.state])}</span>
            </DescriptionItem>
            {appeal.decided_at !== undefined && (
              <DescriptionItem term={t("client:admin.appeal.decided_at")}>
                <DateTime value={appeal.decided_at} />
              </DescriptionItem>
            )}
          </>
        )}
      </DescriptionList>
      {/* Written by the user who appealed, in any language; the appeal's page shows it whole (Y10). */}
      <p dir="auto" className="line-clamp-2 text-start wrap-anywhere whitespace-pre-wrap">
        {appeal.text}
      </p>
    </div>
  );
}
