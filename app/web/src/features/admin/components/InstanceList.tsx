// SPDX-License-Identifier: AGPL-3.0-only
// The remote servers screen below its heading (federation.md §7.3): tabs that filter by state, kept in the
// URL by the route (web-client.md S3), and the servers of the selected filter, each with its state, since when,
// and its failures, linking to the server's page with the reasons. The tabs follow the WAI-ARIA APG tabs
// pattern with automatic activation (React Aria Tabs); only the selected panel is mounted.
import { DescriptionItem, DescriptionList, EmptyState, Link, Tab, TabList, TabPanel, Tabs } from "@my/ui";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { type InstanceFilter, instanceFilters, type InstanceHealth } from "../api/admin";
import { useInstances } from "../hooks/useInstances";
import { Day, DateTime, DomainName, InstanceStateLabel, RecordList, RequirePermission } from "./parts";

const filterLabels: Readonly<Record<InstanceFilter, ClientMessageKey>> = {
  all: "client:admin.instances.filter.all",
  unavailable: "client:admin.instances.filter.unavailable",
  available: "client:admin.instances.filter.available",
};

function isFilter(key: unknown): key is InstanceFilter {
  return instanceFilters.some((filter) => filter === key);
}

/** The address of a server's page. */
function instancePath(domain: string): string {
  return `/admin/instances/${encodeURIComponent(domain)}`;
}

export type InstanceListProps = {
  /** The selected filter, from the URL. */
  filter: InstanceFilter;
  /** Selects another filter: the route puts it into the URL. */
  onFilterChange: (filter: InstanceFilter) => void;
};

export function InstanceList(props: InstanceListProps) {
  return (
    <RequirePermission permission="manage_domains">
      <InstanceTabs {...props} />
    </RequirePermission>
  );
}

function InstanceTabs({ filter, onFilterChange }: InstanceListProps) {
  const t = useT();
  return (
    <Tabs
      selectedKey={filter}
      onSelectionChange={(key) => {
        if (isFilter(key)) onFilterChange(key);
      }}
    >
      <TabList aria-label={t("client:admin.instances.filter.label")}>
        {instanceFilters.map((item) => (
          <Tab key={item} id={item}>
            {t(filterLabels[item])}
          </Tab>
        ))}
      </TabList>
      <TabPanel id={filter}>
        <Instances filter={filter} />
      </TabPanel>
    </Tabs>
  );
}

function Instances({ filter }: { filter: InstanceFilter }) {
  const t = useT();
  const list = useInstances(filter);
  return (
    <RecordList
      list={list}
      getId={(health) => health.domain}
      empty={<EmptyState heading={t("client:admin.instances.empty")} description={t("client:admin.instances.empty.description")} />}
    >
      {(health) => <InstanceRow health={health} />}
    </RecordList>
  );
}

function InstanceRow({ health }: { health: InstanceHealth }) {
  const t = useT();
  return (
    <div className="flex flex-col gap-2">
      <h2 className="font-semibold">
        <Link href={instancePath(health.domain)} variant="standalone" className="wrap-anywhere">
          <DomainName domain={health.domain} />
        </Link>
      </h2>
      <DescriptionList columns={4}>
        <DescriptionItem term={t("client:admin.instance.state")}>
          <InstanceStateLabel state={health.state} />
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.instance.since")}>{health.since === undefined ? t("client:admin.instance.none") : <DateTime value={health.since} />}</DescriptionItem>
        <DescriptionItem term={t("client:admin.instance.failure_days")}>{t("client:admin.instance.failure_days.value", { count: health.failure_days })}</DescriptionItem>
        <DescriptionItem term={t("client:admin.instance.last_failure")}>
          {health.last_failure_day === undefined ? t("client:admin.instance.none") : <Day value={health.last_failure_day} />}
        </DescriptionItem>
      </DescriptionList>
    </div>
  );
}
