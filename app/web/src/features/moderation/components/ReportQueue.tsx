// SPDX-License-Identifier: AGPL-3.0-only
// The report queue below its heading (FR-701, FR-710): the reports that await the moderators and those handled,
// newest first. The tabs follow the WAI-ARIA APG tabs pattern with automatic activation (React Aria Tabs), kept
// in the URL by the route (web-client.md S3); only the selected panel is mounted. Each row's heading links to the
// report's page, where it is handled.
import { DescriptionItem, DescriptionList, EmptyState, Heading, Link, Stack, Tab, TabList, TabPanel, Tabs, Text } from "@my/ui";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { type Report, type ReportFilter, reportFilters } from "../api/reports";
import { handleOf, isolate, reportCategoryLabels, reportOutcomeLabels, useReports } from "../hooks/useReports";
import { DateTime, RecordList, RequirePermission } from "./parts";
import { AccountLabel, AssigneeName } from "./reportParts";

const filterLabels: Readonly<Record<ReportFilter, ClientMessageKey>> = {
  open: "client:admin.reports.filter.open",
  resolved: "client:admin.reports.filter.resolved",
};

function isFilter(key: unknown): key is ReportFilter {
  return reportFilters.some((filter) => filter === key);
}

export type ReportQueueProps = {
  /** The selected tab, from the URL. */
  filter: ReportFilter;
  /** Selects another tab: the route puts it into the URL. */
  onFilterChange: (filter: ReportFilter) => void;
};

export function ReportQueue(props: ReportQueueProps) {
  return (
    <RequirePermission permission="manage_reports">
      <ReportTabs {...props} />
    </RequirePermission>
  );
}

function ReportTabs({ filter, onFilterChange }: ReportQueueProps) {
  const t = useT();
  return (
    <Tabs
      selectedKey={filter}
      onSelectionChange={(key) => {
        if (isFilter(key)) onFilterChange(key);
      }}
    >
      <TabList aria-label={t("client:admin.reports.filter.label")}>
        {reportFilters.map((item) => (
          <Tab key={item} id={item}>
            {t(filterLabels[item])}
          </Tab>
        ))}
      </TabList>
      <TabPanel id={filter}>
        <ReportList filter={filter} />
      </TabPanel>
    </Tabs>
  );
}

function ReportList({ filter }: { filter: ReportFilter }) {
  const t = useT();
  const list = useReports(filter);
  const empty =
    filter === "open" ? (
      <EmptyState headingLevel={2} heading={t("client:admin.reports.empty.open")} description={t("client:admin.reports.empty.open.description")} />
    ) : (
      <EmptyState headingLevel={2} heading={t("client:admin.reports.empty.resolved")} />
    );
  return (
    <RecordList list={list} getId={(report) => report.id} empty={empty}>
      {(report) => <ReportRow report={report} />}
    </RecordList>
  );
}

function ReportRow({ report }: { report: Report }) {
  const t = useT();
  return (
    <Stack gap="2">
      <Heading level={2} size="subsection">
        <Link href={`/moderation/reports/${encodeURIComponent(report.id)}`} variant="standalone" wrap="anywhere">
          {report.target_account === undefined
            ? t("client:admin.report.of_deleted")
            : t("client:admin.report.of", { target: isolate(handleOf(report.target_account)) })}
        </Link>
      </Heading>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.report.category")}>{t(reportCategoryLabels[report.category])}</DescriptionItem>
        <DescriptionItem term={t("client:admin.report.reporter")}>
          <AccountLabel account={report.reporter_account} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.report.filed_at")}>
          <DateTime value={report.filed_at} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.report.assignee")}>
          {report.assignee_account_id === undefined ? t("client:admin.report.assignee.none") : <AssigneeName id={report.assignee_account_id} />}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.report.posts")}>
          <Text numeric>{t("client:admin.report.posts.count", { count: report.status_ids.length })}</Text>
        </DescriptionItem>
        {report.state === "resolved" && (
          <>
            <DescriptionItem term={t("client:admin.report.outcome")}>
              {report.outcome === undefined ? t("client:admin.reports.filter.resolved") : t(reportOutcomeLabels[report.outcome])}
            </DescriptionItem>
            <DescriptionItem term={t("client:admin.report.resolved_at")}>
              <DateTime value={report.state_changed_at} />
            </DescriptionItem>
          </>
        )}
      </DescriptionList>
      {report.comment !== "" && (
        // Written by the reporter, in any language; the report's page shows it whole.
        <Text as="p" dir="auto" lineClamp={2} align="start" wrap="anywhere" whitespace="preWrap">
          {report.comment}
        </Text>
      )}
    </Stack>
  );
}
