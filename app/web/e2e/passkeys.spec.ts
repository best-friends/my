// SPDX-License-Identifier: AGPL-3.0-only
// Passkeys (FR-004) against a real `my serve` (e2e/server.ts) with Chromium's virtual authenticator (the
// WebAuthn domain of the DevTools protocol): a user adds a passkey on the settings page of the app, then signs in
// with it on the server-rendered sign-in page, without the email address and the password. The authenticator
// stores discoverable credentials and verifies the user, as a phone or a password manager does.
//
// Every project creates a user of its own (a random name), so a retry and the other projects never meet its
// passkey. The server's host is localhost, which browsers treat as a secure context, so WebAuthn runs over plain
// HTTP with the relying party ID "localhost".
import { randomBytes } from "node:crypto";
import { readFileSync } from "node:fs";
import type { CDPSession, Page } from "@playwright/test";
import { expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";
import { ownsServer, runMy } from "./server-cli.ts";
import { serverURL } from "./server-origin.ts";

test.use({ baseURL: serverURL });

type Catalog = Record<string, { defaultMessage: string }>;

/** The message `key` of the catalog `namespace` in the project's locale, with its `{name}` arguments replaced. */
function message(namespace: "client" | "server", key: string, values: Record<string, string> = {}): string {
  const locale = test.info().project.use.locale ?? "en";
  const catalog = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/${namespace}.json`, import.meta.url), "utf8")) as Catalog;
  const text = catalog[key]?.defaultMessage;
  if (text === undefined) throw new Error(`no message ${namespace}:${key} in ${locale}`);
  return text.replaceAll(/\{(\w+)\}/g, (_, name: string) => values[name] ?? `{${name}}`);
}

/** A virtual authenticator of the page's browser: a platform authenticator with discoverable credentials. */
async function addAuthenticator(page: Page): Promise<{ cdp: CDPSession; authenticatorId: string }> {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send("WebAuthn.enable");
  const { authenticatorId } = await cdp.send("WebAuthn.addVirtualAuthenticator", {
    options: {
      protocol: "ctap2",
      transport: "internal",
      hasResidentKey: true,
      hasUserVerification: true,
      isUserVerified: true,
      automaticPresenceSimulation: true,
    },
  });
  return { cdp, authenticatorId };
}

test.describe("passkeys", () => {
  test.skip(!ownsServer, "the test creates a user with bin/my, which needs the database of the server e2e/server.ts starts");

  test("adds a passkey in the settings and signs in with it", async ({ page, makeAxeBuilder }) => {
    const username = `e2e_passkey_${randomBytes(4).toString("hex")}`;
    const user = { email: `${username}@example.com`, password: `e2e-password-${username}` };
    runMy(["admin", "create", "-username", username, "-email", user.email], `${user.password}\n`);
    const { cdp, authenticatorId } = await addAuthenticator(page);

    // Into the app through the server's sign-in page, with the password.
    await page.goto("/login");
    await page.getByRole("main").getByRole("button").click();
    await page.locator('input[type="email"][autocomplete="username"]').fill(user.email);
    await page.locator('input[type="password"][autocomplete="current-password"]').fill(user.password);
    await page.locator('main button[type="submit"]').first().click();
    await expect(page).toHaveURL(`${serverURL}/home`);

    // The settings page adds a passkey after the password.
    await page.goto("/settings/passkeys");
    const main = page.getByRole("main");
    const form = main.getByRole("region", { name: message("client", "settings.passkeys.add.title") });
    await form.getByRole("textbox", { name: message("client", "settings.passkeys.name") }).fill("E2E authenticator");
    await form.locator('input[type="password"][autocomplete="current-password"]').fill(user.password);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
    await form.getByRole("button", { name: message("client", "settings.passkeys.add.submit"), exact: true }).click();
    await expect(main.getByText(message("client", "settings.passkeys.added", { name: "E2E authenticator" }))).toBeVisible();
    const list = main.getByRole("region", { name: message("client", "settings.passkeys.list.title") });
    await expect(list.getByText("E2E authenticator", { exact: true })).toBeVisible();
    const { credentials } = await cdp.send("WebAuthn.getCredentials", { authenticatorId });
    expect(credentials).toHaveLength(1);
    expect(credentials[0]?.isResidentCredential).toBe(true);
    expect(credentials[0]?.rpId).toBe("localhost");

    // Signed out, the server's sign-in page signs in with the passkey alone.
    await page.context().clearCookies();
    await page.goto("/auth/sign_in");
    const passkeyButton = page.getByRole("main").getByRole("button", { name: message("server", "auth.sign_in.passkey"), exact: true });
    await expect(passkeyButton).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
    await passkeyButton.click();
    await expect(page).toHaveURL(`${serverURL}/`);
    await page.goto("/auth/sign_in");
    await expect(page.locator('form[action="/auth/sign_out"]')).toBeVisible();
    const { credentials: used } = await cdp.send("WebAuthn.getCredentials", { authenticatorId });
    expect(used[0]?.signCount).toBeGreaterThan(credentials[0]?.signCount ?? 0);
  });
});
