// SPDX-License-Identifier: AGPL-3.0-only
// The bar at the top of the app that says why it cannot load from the server right now: it is offline (web-client.md
// G17), or the server's rate limit refuses its requests until the time Retry-After names (lib/rateLimit). Either way,
// what failed loads again by itself, and the bar says when. Being offline comes first: no request gets through then.
// It is the one OfflineBanner (a role="status" region in the page from the start), so that the change is announced;
// on a short screen it scrolls away with the page instead of sticking to the top edge (OfflineBanner).
import { OfflineBanner } from "@my/ui";
import { useIntl } from "react-intl";
import { useT } from "../../lib/i18n";
import { useIsOffline } from "../../lib/network";
import { useRateLimitRetry } from "../../lib/rateLimit";

export function ConnectionBanner() {
  const t = useT();
  const intl = useIntl();
  const offline = useIsOffline();
  const rateLimitedUntil = useRateLimitRetry();
  const message = offline
    ? t("client:shell.offline")
    : rateLimitedUntil === null
      ? null
      : t("client:shell.rate_limited", { time: intl.formatTime(rateLimitedUntil, { timeStyle: "medium" }) });
  return <OfflineBanner isOffline={message !== null}>{message}</OfflineBanner>;
}
