// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, type ErrorComponentProps, redirect } from "@tanstack/react-router";
import { z } from "zod";
import { SignInFailure } from "../features/session";
import { completeSignIn } from "../lib/auth/signIn";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/** The authorization response (RFC 6749 §4.1.2, §4.1.2.1); completeSignIn checks what is missing. */
const searchSchema = z.object({
  code: z.string().optional().catch(undefined),
  state: z.string().optional().catch(undefined),
  error: z.string().optional().catch(undefined),
});

/**
 * The redirect URI of the web client (platform.WebClientCallbackPath). It completes the sign-in before
 * anything renders and replaces itself in the history with the page the sign-in started from, so that
 * the code never stays in the address bar or the history.
 */
export const Route = createFileRoute("/login_/callback")({
  validateSearch: searchSchema,
  beforeLoad: async ({ context, search }) => {
    const returnTo = await completeSignIn(context.session, search, window.location.origin);
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- TanStack Router's redirects are thrown objects.
    throw redirect({ href: returnTo, replace: true });
  },
  errorComponent: SignInCallbackError,
});

function SignInCallbackError({ error }: ErrorComponentProps) {
  const t = useT();
  return (
    <>
      <PageHeading>{t("client:login.failed.title")}</PageHeading>
      <SignInFailure error={error} />
    </>
  );
}
