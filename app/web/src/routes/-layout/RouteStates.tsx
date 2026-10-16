// SPDX-License-Identifier: AGPL-3.0-only
// What a route shows instead of its page: while it loads, when it fails, and for an unknown address
// (web-client.md E1, E3). Each has the page's h1, so the layout moves focus to it and titles the document.
import { Button, Center, Link, Spinner } from "@my/ui";
import { useQueryErrorResetBoundary } from "@tanstack/react-query";
import { type ErrorComponentProps, useRouter } from "@tanstack/react-router";
import { useT } from "../../lib/i18n";
import { PageHeading } from "./PageHeading";

/** The router's defaultPendingComponent: shown only when a page takes longer than defaultPendingMs to load. */
export function RoutePending() {
  const t = useT();
  return (
    <div className="grid place-items-center py-10">
      <Spinner size="lg" label={t("client:shell.loading")} />
    </div>
  );
}

/** The error of one page, inside the layout so that the navigation stays usable (the router's defaultErrorComponent). */
export function RouteError({ reset }: ErrorComponentProps) {
  const t = useT();
  const router = useRouter();
  const queryErrorResetBoundary = useQueryErrorResetBoundary();
  const retry = () => {
    // Queries that failed into this boundary refetch instead of throwing their cached error again, and
    // invalidate() runs the loaders again (TanStack Router docs, "Data Loading" with TanStack Query).
    queryErrorResetBoundary.reset();
    reset();
    void router.invalidate();
  };
  return (
    <>
      <PageHeading>{t("client:shell.error.title")}</PageHeading>
      <p>{t("client:shell.error.body")}</p>
      <div>
        <Button variant="primary" onPress={retry}>
          {t("client:shell.error.retry")}
        </Button>
      </div>
    </>
  );
}

/** The error of the layout itself: the whole page, since the layout cannot be shown. */
export function RootError(props: ErrorComponentProps) {
  return (
    <main className="px-4">
      <Center>
        <div className="flex max-w-prose flex-col gap-4">
          <RouteError {...props} />
        </div>
      </Center>
    </main>
  );
}

/** An address that no route matches. */
export function NotFound() {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:shell.not_found.title")}</PageHeading>
      <p>{t("client:shell.not_found.body")}</p>
      <p>
        <Link href="/" variant="standalone">
          {t("client:shell.not_found.home")}
        </Link>
      </p>
    </>
  );
}
