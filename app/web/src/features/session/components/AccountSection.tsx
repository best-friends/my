// SPDX-License-Identifier: AGPL-3.0-only
import { Button, Link, RegionError, Spinner } from "@my/ui";
import { useId, useState } from "react";
import { useConfig } from "../../../lib/config";
import { useT } from "../../../lib/i18n";
import { useSignOut } from "../hooks/useSignOut";
import { useViewer } from "../hooks/useViewer";

/** The account section of the settings page: who is signed in and the sign-out button, or a link to sign in. */
export function AccountSection() {
  const t = useT();
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {t("client:session.account.title")}
      </h2>
      <AccountStatus />
    </section>
  );
}

function AccountStatus() {
  const t = useT();
  const { accountDomain } = useConfig();
  const viewer = useViewer();
  const { signOut, isPending } = useSignOut();
  const [retried, setRetried] = useState(false);
  switch (viewer.status) {
    case "loading":
      return <Spinner label={t("client:shell.loading")} />;
    case "signed-out":
      return (
        <div className="flex flex-col items-start gap-2">
          <p className="text-fg-muted">{t("client:session.signed_out")}</p>
          <Link href="/login">{t("client:session.sign_in")}</Link>
        </div>
      );
    case "error":
      return (
        <div className="flex flex-col items-start gap-2">
          {/* Read with the page, not announced; a retry that fails again is the user's action, and an alert (G16). */}
          <RegionError
            headingLevel={3}
            heading={t("client:session.account.failed")}
            reason={viewer.message}
            retryLabel={t("client:session.account.retry")}
            onRetry={() => {
              setRetried(true);
              viewer.retry();
            }}
            isRetrying={viewer.isRetrying}
            alert={retried && !viewer.isRetrying}
          />
          <Button onPress={signOut} isPending={isPending}>
            {t("client:session.sign_out")}
          </Button>
        </div>
      );
    case "signed-in": {
      const handle = `@${viewer.account.acct}@${accountDomain}`;
      return (
        <div className="flex flex-col items-start gap-2">
          {/* The display name and handle are the user's text, in any language (web-client.md Y10). */}
          <p className="flex flex-col">
            <span dir="auto" className="font-semibold align-match-parent">
              {viewer.account.display_name || viewer.account.username}
            </span>
            <span dir="ltr" className="text-sm text-fg-muted">
              {handle}
            </span>
          </p>
          <Button onPress={signOut} isPending={isPending}>
            {t("client:session.sign_out")}
          </Button>
        </div>
      );
    }
  }
}
