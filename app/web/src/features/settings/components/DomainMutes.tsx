// SPDX-License-Identifier: AGPL-3.0-only
// The muted domains page below its heading (FR-304): what muting a domain does, the form that mutes one, and the muted
// domains in the order of the domains, each with its button that unmutes it. The form follows the browser's constraint
// validation (React Aria's native validation); muting is undone as easily, so neither asks first. What muting did stays
// on the page in a StatusMessage (role="status") under the form; unmuting is announced and moves focus to the next row.
import { Button, EmptyState, StatusMessage, TextField } from "@my/ui";
import { useId, useRef } from "react";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import { domainToUnicode } from "../../../lib/idn";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import { domainLimit, type DomainMute } from "../api/domainMutes";
import { type UnmuteDomain, useDomainMuteList, useMuteDomainForm, useUnmuteDomain } from "../hooks/useDomainMutes";

export function DomainMutes() {
  const t = useT();
  const formHeadingId = useId();
  const listHeadingId = useId();
  return (
    <div className="flex flex-col gap-8">
      <p>{t("client:settings.domain_mutes.description")}</p>
      <section aria-labelledby={formHeadingId} className="flex flex-col gap-3">
        <h2 id={formHeadingId} className="text-lg font-semibold">
          {t("client:settings.domain_mutes.new.title")}
        </h2>
        <MuteDomainForm />
      </section>
      <section aria-labelledby={listHeadingId} className="flex flex-col gap-3">
        <h2 id={listHeadingId} className="text-lg font-semibold">
          {t("client:settings.domain_mutes.list.title")}
        </h2>
        <DomainMuteList />
      </section>
    </div>
  );
}

function MuteDomainForm() {
  const t = useT();
  const form = useMuteDomainForm();
  return (
    <form onSubmit={form.submit} className="flex flex-col gap-4">
      <TextField
        label={t("client:settings.domain_mutes.domain")}
        description={t("client:settings.domain_mutes.domain.description")}
        value={form.domain}
        onChange={form.setDomain}
        isRequired
        maxLength={domainLimit}
        autoComplete="off"
        autoCorrect="off"
        spellCheck="false"
        inputMode="url"
        errorMessage={t("client:settings.domain_mutes.domain.required")}
      />
      {form.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:settings.domain_mutes.mute_failed", { reason: form.error })}
        </p>
      )}
      <div>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:settings.domain_mutes.mute")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
    </form>
  );
}

function DomainMuteList() {
  const t = useT();
  const list = useDomainMuteList();
  const unmute = useUnmuteDomain();
  const listRef = useRef<HTMLUListElement>(null);
  const removed = useFocusAfterRemoval(
    listRef,
    list.items.map((mute) => mute.domain),
  );
  if (list.items.length === 0) return <EmptyState heading={t("client:settings.domain_mutes.empty")} />;
  return (
    <div className="flex flex-col gap-4">
      <ul ref={listRef} className="flex flex-col divide-y divide-border">
        {list.items.map((mute) => (
          <li key={mute.domain} data-row-id={mute.domain} className="@container py-3">
            <DomainMuteRow
              mute={mute}
              unmute={unmute}
              onUnmuted={() => {
                removed(mute.domain);
              }}
            />
          </li>
        ))}
      </ul>
      {list.hasMore && (
        <div>
          <Button onPress={list.loadMore} isPending={list.isLoadingMore}>
            {t("client:settings.list.more")}
          </Button>
        </div>
      )}
    </div>
  );
}

type DomainMuteRowProps = { mute: DomainMute; unmute: UnmuteDomain; onUnmuted: () => void };

/** A muted domain: its U-labels (and its A-labels when they differ) and when it was muted, with the button that unmutes it. */
function DomainMuteRow({ mute, unmute, onUnmuted }: DomainMuteRowProps) {
  const t = useT();
  const intl = useIntl();
  const unicode = domainToUnicode(mute.domain);
  const label = t("client:settings.domain_mutes.unmute");
  return (
    <div className="flex flex-col gap-3 @md:flex-row @md:items-center">
      <p className="flex min-w-0 flex-1 flex-col">
        {/* A domain reads left to right in every locale. */}
        <span dir="ltr" className="truncate text-start font-semibold">
          {unicode}
        </span>
        {unicode !== mute.domain && (
          <span dir="ltr" className="truncate text-start text-sm text-fg-muted">
            {mute.domain}
          </span>
        )}
        <span className="text-sm text-fg-muted">
          {t("client:settings.domain_mutes.since", {
            date: intl.formatDate(mute.created_at, { dateStyle: "medium", timeStyle: "short" }),
          })}
        </span>
      </p>
      <div className="flex flex-wrap gap-2">
        <Button
          size="sm"
          isPending={unmute.pendingDomain === mute.domain}
          aria-label={t("client:settings.list.action", { action: label, name: unicode })}
          onPress={() => {
            unmute.run(mute.domain, onUnmuted);
          }}
        >
          {label}
        </Button>
      </div>
    </div>
  );
}
