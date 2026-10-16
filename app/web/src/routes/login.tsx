// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";
import { SignInPanel } from "../features/session";
import { isReturnPath } from "../lib/auth/signIn";
import { useT } from "../lib/i18n";
import { PageHeading } from "./-layout/PageHeading";

/** `return_to`: the path of this app to open after signing in. Anything else is dropped (web-client.md S3). */
const searchSchema = z.object({
  return_to: z.string().refine(isReturnPath).optional().catch(undefined),
});

/** The sign-in page: it sends the browser to the server's sign-in screen (ADR-0030). */
export const Route = createFileRoute("/login")({
  validateSearch: searchSchema,
  component: LoginPage,
});

function LoginPage() {
  const t = useT();
  const { return_to: returnTo = "/" } = Route.useSearch();
  return (
    <>
      <PageHeading>{t("client:login.title")}</PageHeading>
      <SignInPanel returnTo={returnTo} />
    </>
  );
}
