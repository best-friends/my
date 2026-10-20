// SPDX-License-Identifier: AGPL-3.0-only
// The account search below its heading (FR-714): a form of the conditions, then the accounts that match, newest
// first. The selects are React Aria Selects (the WAI-ARIA APG select-only combobox) and the fields React Aria
// TextFields, checked by the browser's constraint validation before the search runs. The route keeps the search
// in the address (web-client.md S3). A local account can be searched by its email address, its email domain or
// an IP address it used, one of them at a time; the results never show them (the account's page does, when asked).
import { Avatar, Button, DescriptionItem, DescriptionList, EmptyState, Link, RegionError, Select, SelectItem, Spinner, TextField } from "@my/ui";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import {
  type AdminAccount,
  adminAccountContactFields,
  adminAccountOrigins,
  type AdminAccountSearch,
  adminAccountSearchLimit,
  adminAccountStates,
} from "../api/accounts";
import {
  adminAccountContactLabels,
  adminAccountOriginLabels,
  adminAccountRegistrationStateLabels,
  adminAccountRestrictionLabels,
  adminAccountStateLabels,
  anyOption,
  isAccountState,
  isContactField,
  isOrigin,
  useAccountSearchForm,
  useAccountSearchResults,
} from "../hooks/useAdminAccounts";
import { accountName } from "../hooks/useReports";
import { DateTime, DetailSection, DomainName, RecordList, RequirePermission } from "./parts";
import { AccountLabel } from "./reportParts";
import { imageSources } from "../../../lib/images";

export type AccountSearchProps = {
  /** The search, from the URL. */
  search: AdminAccountSearch;
  /** Runs another search: the route puts it into the URL. */
  onSearch: (search: AdminAccountSearch) => void;
};

export function AccountSearch(props: AccountSearchProps) {
  return (
    <RequirePermission permission="manage_accounts">
      <AccountSearchView {...props} />
    </RequirePermission>
  );
}

/** Whether a search has any condition. */
function isFiltered(search: AdminAccountSearch): boolean {
  return Object.values(search).some((value) => value !== undefined);
}

function AccountSearchView({ search, onSearch }: AccountSearchProps) {
  const t = useT();
  return (
    <div className="flex flex-col gap-8">
      <DetailSection title={t("client:admin.accounts.search.title")}>
        {/* Another search in the address (back, forward, a link) starts the form from it (web-client.md R10). */}
        <SearchForm key={JSON.stringify(search)} search={search} onSearch={onSearch} />
      </DetailSection>
      <DetailSection title={t("client:admin.accounts.results.title")}>
        <Results search={search} />
      </DetailSection>
    </div>
  );
}

function SearchForm({ search, onSearch }: AccountSearchProps) {
  const t = useT();
  const form = useAccountSearchForm(search, onSearch);
  const { draft } = form;
  return (
    <form onSubmit={form.submit} className="@container flex flex-col gap-4">
      <div className="grid grid-cols-1 gap-4 @md:grid-cols-2">
        <TextField
          label={t("client:admin.accounts.search.username")}
          description={t("client:admin.accounts.search.username.description")}
          value={draft.username}
          onChange={(username) => {
            form.change({ username });
          }}
          maxLength={adminAccountSearchLimit}
          autoComplete="off"
          spellCheck="false"
        />
        <TextField
          label={t("client:admin.accounts.search.domain")}
          description={t("client:admin.accounts.search.domain.description")}
          value={draft.domain}
          onChange={(domain) => {
            form.change({ domain });
          }}
          maxLength={adminAccountSearchLimit}
          autoComplete="off"
          spellCheck="false"
        />
        <Select
          label={t("client:admin.accounts.search.origin")}
          value={draft.origin}
          onChange={(key) => {
            form.change({ origin: isOrigin(key) ? key : anyOption });
          }}
        >
          <SelectItem id={anyOption}>{t("client:admin.accounts.search.origin.any")}</SelectItem>
          {adminAccountOrigins.map((origin) => (
            <SelectItem key={origin} id={origin}>
              {t(adminAccountOriginLabels[origin])}
            </SelectItem>
          ))}
        </Select>
        <Select
          label={t("client:admin.accounts.search.state")}
          value={draft.state}
          onChange={(key) => {
            form.change({ state: isAccountState(key) ? key : anyOption });
          }}
        >
          <SelectItem id={anyOption}>{t("client:admin.accounts.search.state.any")}</SelectItem>
          {adminAccountStates.map((state) => (
            <SelectItem key={state} id={state}>
              {t(adminAccountStateLabels[state])}
            </SelectItem>
          ))}
        </Select>
        <Select
          label={t("client:admin.accounts.search.contact")}
          description={t("client:admin.accounts.search.contact.description")}
          value={draft.contact}
          onChange={(key) => {
            form.change({ contact: isContactField(key) ? key : anyOption });
          }}
        >
          <SelectItem id={anyOption}>{t("client:admin.accounts.search.contact.none")}</SelectItem>
          {adminAccountContactFields.map((field) => (
            <SelectItem key={field} id={field}>
              {t(adminAccountContactLabels[field])}
            </SelectItem>
          ))}
        </Select>
        {draft.contact !== anyOption && (
          <TextField
            label={t(adminAccountContactLabels[draft.contact])}
            value={draft.contactValue}
            onChange={(contactValue) => {
              form.change({ contactValue });
            }}
            isRequired
            maxLength={adminAccountSearchLimit}
            {...(draft.contact === "email" && { type: "email" })}
            autoComplete="off"
            spellCheck="false"
            errorMessage={(validation) =>
              validation.validationDetails.valueMissing ? t("client:admin.accounts.search.contact.required") : t("client:admin.accounts.search.contact.invalid")
            }
          />
        )}
      </div>
      <div>
        <Button type="submit" variant="primary">
          {t("client:admin.accounts.search.submit")}
        </Button>
      </div>
    </form>
  );
}

function Results({ search }: { search: AdminAccountSearch }) {
  const t = useT();
  const results = useAccountSearchResults(search);
  switch (results.status) {
    case "loading":
      return <Spinner size="sm" label={t("client:admin.accounts.results.loading")} />;
    case "error":
      return (
        <RegionError
          heading={t("client:admin.accounts.results.failed")}
          headingLevel={3}
          reason={results.reason}
          retryLabel={t("client:admin.accounts.results.retry")}
          onRetry={results.retry}
          isRetrying={results.isRetrying}
        />
      );
    case "ready": {
      const { list } = results;
      // A page reads a part of the accounts, so it may find none and still go on.
      const empty = list.hasMore ? (
        <EmptyState
          headingLevel={3}
          heading={t("client:admin.accounts.empty.so_far")}
          description={t("client:admin.accounts.empty.so_far.description")}
          action={
            <Button onPress={list.loadMore} isPending={list.isLoadingMore}>
              {t("client:admin.accounts.empty.so_far.more")}
            </Button>
          }
        />
      ) : isFiltered(search) ? (
        <EmptyState headingLevel={3} heading={t("client:admin.accounts.empty.filtered")} description={t("client:admin.accounts.empty.filtered.description")} />
      ) : (
        <EmptyState headingLevel={3} heading={t("client:admin.accounts.empty")} />
      );
      return (
        <RecordList
          list={list}
          getId={(account) => account.id}
          empty={
            <div className="flex flex-col gap-4">
              {empty}
              {results.moreError !== undefined && <RegionError heading={t("client:admin.accounts.results.more_failed")} reason={results.moreError} />}
            </div>
          }
          moreError={results.moreError !== undefined && <RegionError heading={t("client:admin.accounts.results.more_failed")} reason={results.moreError} />}
        >
          {(account) => <AccountRow account={account} />}
        </RecordList>
      );
    }
  }
}

/** The place of an account: this server, or the domain of its own. */
export function AccountOrigin({ account }: { account: AdminAccount }) {
  const t = useT();
  return account.local ? t("client:admin.accounts.origin.local") : <DomainName domain={account.account.domain} />;
}

function AccountRow({ account }: { account: AdminAccount }) {
  const t = useT();
  const intl = useIntl();
  return (
    <div className="flex flex-col gap-2">
      <div className="flex min-w-0 items-center gap-3">
        <Avatar sources={imageSources(account.account.avatar)} name={accountName(account.account)} decorative />
        <h3 className="min-w-0 font-semibold">
          <Link href={`/moderation/accounts/${encodeURIComponent(account.id)}`} variant="standalone">
            <AccountLabel account={account.account} />
          </Link>
        </h3>
      </div>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.accounts.state")}>
          <span className="font-semibold">{t(adminAccountStateLabels[account.state])}</span>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.accounts.origin")}>
          <AccountOrigin account={account} />
        </DescriptionItem>
        {account.registration_state !== undefined && (
          <DescriptionItem term={t("client:admin.accounts.registration")}>{t(adminAccountRegistrationStateLabels[account.registration_state])}</DescriptionItem>
        )}
        <DescriptionItem term={t("client:admin.accounts.created_at")}>
          <DateTime value={account.account.created_at} />
        </DescriptionItem>
        {account.restrictions.length > 0 && (
          <DescriptionItem term={t("client:admin.accounts.restrictions")}>
            {intl.formatList(
              account.restrictions.map((restriction) => t(adminAccountRestrictionLabels[restriction])),
              { type: "conjunction" },
            )}
          </DescriptionItem>
        )}
      </DescriptionList>
    </div>
  );
}
