// SPDX-License-Identifier: AGPL-3.0-only
import { Avatar, Button, EmptyState, ExternalLink, Link, RegionError, Spinner } from "@my/ui";
import { type ReactNode, useId } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { handleOf } from "../../../lib/handle";
import { domainToUnicode } from "../../../lib/idn";
import { imageSources } from "../../../lib/images";
import { profilePath } from "../../../lib/paths";
import { DocumentsSection } from "../../documents";
import type { Instance, PublicDomainAction } from "../api/about";
import { type ModeratedDomains, useAbout, useModeratedDomains } from "../hooks/useAbout";

/**
 * The body of the instance's information page (`/about`), under the page's heading: the thumbnail, the summary, the
 * description, the rules followed by its terms of service and privacy policy, the remote domains it moderates as far
 * as the instance lists them, what it states to search engines and AI crawlers, how to reach the operators, and where
 * to get its source code. Everyone may read it, signed in or not.
 */
export function About() {
  const about = useAbout();
  return (
    <div className="flex flex-col gap-8">
      {about.thumbnail !== undefined && <ServerThumbnail thumbnail={about.thumbnail} />}
      {about.summary !== "" && (
        // The administrator's text, in any language (web-client.md Y10).
        <p dir="auto" className="text-lg">
          {about.summary}
        </p>
      )}
      {about.description.length > 0 && <Description paragraphs={about.description} />}
      {/* What users agree to stands together: the rules, then the terms and policies. */}
      {about.rules.length > 0 && <Rules rules={about.rules} />}
      <DocumentsSection />
      {/* How the server treats other servers and crawlers. */}
      <ModeratedDomainsSection />
      {about.crawlers !== undefined && <Crawlers searchIndexing={about.crawlers.searchIndexing} aiUsage={about.crawlers.aiUsage} />}
      {/* Who runs the server and what it runs. */}
      {about.contact !== undefined && <Contact contact={about.contact} />}
      {about.sourceUrl !== undefined && <SourceCode url={about.sourceUrl} />}
    </div>
  );
}

/** A section of the page: an `h2` that names a `<section>`. */
function Section({ title, children }: { title: string; children: ReactNode }) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {title}
      </h2>
      {children}
    </section>
  );
}

function Description({ paragraphs }: { paragraphs: readonly string[] }) {
  const t = useT();
  return (
    <Section title={t("client:about.description.title")}>
      {paragraphs.map((paragraph) => (
        // Paragraphs of one text have no IDs; the text itself is the key (two equal paragraphs are not expected).
        <p key={paragraph} dir="auto" className="whitespace-pre-line">
          {paragraph}
        </p>
      ))}
    </Section>
  );
}

function Rules({ rules }: { rules: ReturnType<typeof useAbout>["rules"] }) {
  const t = useT();
  return (
    <Section title={t("client:about.rules.title")}>
      <ol className="flex list-decimal flex-col gap-2 ps-6">
        {rules.map((rule) => (
          <li key={rule.id} lang={rule.lang} dir="auto">
            <p className="font-medium">{rule.text}</p>
            {rule.hint !== undefined && <p className="text-sm text-fg-muted">{rule.hint}</p>}
          </li>
        ))}
      </ol>
    </Section>
  );
}

/**
 * The image that represents the instance, at the width of the page. Its alt text is the administrator's; without
 * one, the image is decorative, as the page names the server in its heading.
 */
export function ServerThumbnail({ thumbnail }: { thumbnail: NonNullable<Instance["thumbnail"]> }) {
  return (
    <img
      src={thumbnail.url}
      alt={thumbnail.alt ?? ""}
      width={thumbnail.width}
      height={thumbnail.height}
      decoding="async"
      className="h-auto max-h-80 w-full rounded-lg bg-surface-sunken object-cover"
    />
  );
}

/** How to reach the operators: their account on the server and their email address, as far as the server has them. */
function Contact({ contact }: { contact: NonNullable<Instance["contact"]> }) {
  const t = useT();
  const { account, email } = contact;
  return (
    <Section title={t("client:about.contact.title")}>
      <dl className="flex flex-col gap-3">
        {account !== undefined && (
          <div className="flex flex-col gap-1">
            <dt className="text-sm text-fg-muted">{t("client:about.contact.account")}</dt>
            <dd>
              <Link
                href={profilePath(account.acct)}
                variant="standalone"
                className="inline-grid max-w-full min-w-0 grid-cols-[auto_minmax(0,1fr)] items-center gap-x-3"
              >
                <>
                  <Avatar sources={imageSources(account.avatar)} name={account.display_name || account.username} decorative className="row-span-2" />
                  {/* The user's own text, in any language (web-client.md Y10). */}
                  <span dir="auto" className="font-semibold wrap-anywhere">
                    {account.display_name || account.username}
                  </span>
                  {/* The space keeps the name and the handle apart in the link's name. */}{" "}
                  <span dir="ltr" className="text-start text-sm font-normal wrap-anywhere text-fg-muted">
                    {handleOf(account.acct)}
                  </span>
                </>
              </Link>
            </dd>
          </div>
        )}
        {email !== undefined && (
          <div className="flex flex-col gap-1">
            <dt className="text-sm text-fg-muted">{t("client:about.contact.email")}</dt>
            <dd>
              {/* An address reads left to right in every locale. */}
              <Link href={`mailto:${email}`} variant="standalone" className="wrap-anywhere">
                <span dir="ltr">{email}</span>
              </Link>
            </dd>
          </div>
        )}
      </dl>
    </Section>
  );
}

/**
 * What the instance states to search engines and AI crawlers (FR-725, ADR-0057), and what such a statement is: a
 * request that crawlers may or may not follow, with the legal standing it may have, never a promise of one.
 */
function Crawlers({ searchIndexing, aiUsage }: { searchIndexing: Instance["search_indexing"]; aiUsage: Instance["ai_usage"] }) {
  const t = useT();
  return (
    <Section title={t("client:about.crawlers.title")}>
      <p>{t("client:about.crawlers.search", { setting: searchIndexing })}</p>
      <p>{t("client:about.crawlers.ai", { setting: aiUsage })}</p>
      <p className="text-sm text-fg-muted">{t("client:about.crawlers.how")}</p>
      <p className="text-sm text-fg-muted">{t("client:about.crawlers.accounts")}</p>
      <p className="text-sm text-fg-muted">{t("client:about.crawlers.legal")}</p>
    </Section>
  );
}

/** The address of a mailto: URI (without its query), or undefined when the URI is not one. */
function mailtoAddress(url: string): string | undefined {
  if (!url.startsWith("mailto:")) return undefined;
  const address = url.slice("mailto:".length).split("?")[0] ?? "";
  try {
    return decodeURIComponent(address);
  } catch {
    return address;
  }
}

/**
 * The link to the source code the server runs (AGPL-3.0 section 13, ADR-0011): to the page that publishes it, which
 * leaves the app, or, for a mailto: URI, to an email that requests it.
 */
export function SourceCodeLink({ url }: { url: string }) {
  const t = useT();
  if (mailtoAddress(url) !== undefined) {
    return (
      <Link href={url} variant="standalone">
        {t("client:about.source.request")}
      </Link>
    );
  }
  return (
    <ExternalLink href={url} variant="standalone">
      {t("client:about.source.view")}
    </ExternalLink>
  );
}

/** Where users get the source code the server runs, and why they may (AGPL-3.0 section 13). */
function SourceCode({ url }: { url: string }) {
  const t = useT();
  const address = mailtoAddress(url);
  return (
    <Section title={t("client:about.source.title")}>
      <p>{t("client:about.source.license")}</p>
      <p>
        <SourceCodeLink url={url} />
      </p>
      {address !== undefined && (
        <dl className="flex flex-col gap-1">
          <dt className="text-sm text-fg-muted">{t("client:about.source.address")}</dt>
          {/* An address reads left to right in every locale. */}
          <dd dir="ltr" className="text-start wrap-anywhere">
            {address}
          </dd>
        </dl>
      )}
    </Section>
  );
}

/** The section of the moderated domains; absent when the instance lists them to nobody. */
function ModeratedDomainsSection() {
  const t = useT();
  const domains = useModeratedDomains();
  if (domains.status === "hidden") return null;
  return (
    <Section title={t("client:about.domains.title")}>
      <p className="text-sm text-fg-muted">{t("client:about.domains.intro")}</p>
      <ModeratedDomainsBody domains={domains} />
    </Section>
  );
}

function ModeratedDomainsBody({ domains }: { domains: Exclude<ModeratedDomains, { status: "hidden" }> }) {
  const t = useT();
  switch (domains.status) {
    case "signed-in-only":
      return (
        <p className="flex flex-wrap gap-x-1">
          <span>{t("client:about.domains.signed_in_only")}</span>
          <Link href={`/login?${new URLSearchParams({ return_to: "/about" }).toString()}`} variant="standalone">
            {t("client:about.domains.sign_in")}
          </Link>
        </p>
      );
    case "loading":
      return (
        <div aria-busy="true">
          <Spinner label={t("client:about.domains.loading")} />
        </div>
      );
    case "error":
      return (
        <RegionError
          heading={t("client:about.domains.failed")}
          headingLevel={3}
          reason={domains.reason}
          retryLabel={t("client:about.domains.retry")}
          onRetry={domains.retry}
          isRetrying={domains.isRetrying}
        />
      );
    case "ready":
      if (domains.items.length === 0) return <EmptyState headingLevel={3} heading={t("client:about.domains.empty")} />;
      return (
        <div className="flex flex-col gap-4">
          {domains.items.some(({ action }) => action.domain.includes("*")) && (
            <p className="text-sm text-fg-muted">{t("client:about.domains.obfuscated")}</p>
          )}
          <ul className="flex flex-col divide-y divide-border">
            {domains.items.map(({ key, action }) => (
              <li key={key} className="flex flex-col gap-1 py-3">
                <ModeratedDomain action={action} />
              </li>
            ))}
          </ul>
          {domains.moreError !== undefined && <RegionError heading={t("client:about.domains.more_failed")} reason={domains.moreError} />}
          {domains.hasMore && (
            <div>
              <Button onPress={domains.loadMore} isPending={domains.isLoadingMore}>
                {t("client:about.domains.more")}
              </Button>
            </div>
          )}
        </div>
      );
  }
}

const kindLabels: Readonly<Record<PublicDomainAction["kind"], ClientMessageKey>> = {
  suspend: "client:about.domains.kind.suspend",
  limit: "client:about.domains.kind.limit",
};

function ModeratedDomain({ action }: { action: PublicDomainAction }) {
  const t = useT();
  return (
    <>
      <p className="flex flex-wrap items-baseline gap-x-3">
        {/* A domain reads left to right in every locale; an obfuscated one is shown as the server sent it. */}
        <span dir="ltr" className="font-medium wrap-anywhere">
          {domainToUnicode(action.domain)}
        </span>
        <span className="text-sm text-fg-muted">{t(kindLabels[action.kind])}</span>
      </p>
      {action.comment !== undefined && action.comment !== "" && (
        // The moderators' public comment, in any language (web-client.md Y10).
        <p dir="auto" className="text-sm wrap-anywhere">
          {action.comment}
        </p>
      )}
    </>
  );
}
