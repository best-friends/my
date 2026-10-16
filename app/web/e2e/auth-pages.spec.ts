// SPDX-License-Identifier: AGPL-3.0-only
// The pages the Go server renders without JavaScript (ADR-0030): sign-in, sign-up and OAuth consent,
// against a real `my serve` (e2e/server.ts) in every width × locale project. Fields are found by their
// autocomplete tokens, which are part of what these pages promise (password managers), and stay the same
// in every locale.
import { createHash, randomBytes } from "node:crypto";
import AxeBuilder from "@axe-core/playwright";
import type { Browser, BrowserContext, Page } from "@playwright/test";
import { axeTags, expect, expectNoAxeViolations, expectNoHorizontalScroll, test } from "./fixtures";
import { admin, serverURL, thirdPartyClient } from "./server-origin.ts";

test.use({ baseURL: serverURL });

const emailField = (page: Page) => page.locator('input[type="email"][autocomplete="username"]');
const passwordField = (page: Page, autocomplete: "current-password" | "new-password") =>
  page.locator(`input[type="password"][autocomplete="${autocomplete}"]`);
const submitButton = (page: Page) => page.locator('main button[type="submit"]').first();

/** Asserts what every server-rendered page shares: a language and direction, a main landmark with one h1. */
async function expectPageBasics(page: Page): Promise<void> {
  await expect(page.locator("html")).toHaveAttribute("lang", /^[a-z]{2,3}(?:-[A-Za-z0-9]+)*$/);
  await expect(page.locator("html")).toHaveAttribute("dir", /^(?:ltr|rtl)$/);
  await expect(page.locator("main h1")).toHaveCount(1);
  await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeVisible();
}

/** Asserts that the error message element `id` describes the field and marks it invalid. */
async function expectDescribedError(page: Page, field: ReturnType<Page["locator"]>, id: string): Promise<void> {
  await expect(page.locator(`[id="${id}"]`)).toBeVisible();
  await expect(field).toHaveAttribute("aria-invalid", "true");
  await expect(field).toHaveAttribute("aria-describedby", new RegExp(`(?:^| )${id}(?: |$)`));
}

/**
 * Checks the current page in both themes. Each theme gets a fresh load, so that axe does not sample
 * colors in the middle of a transition.
 */
async function expectNoAxeViolationsInBothThemes(page: Page, makeAxeBuilder: () => AxeBuilder): Promise<void> {
  for (const colorScheme of ["light", "dark"] as const) {
    await page.emulateMedia({ colorScheme });
    await page.reload();
    await expect(page.getByRole("main").getByRole("heading", { level: 1 })).toBeVisible();
    await expectNoAxeViolations(makeAxeBuilder);
  }
}

/**
 * Opens the page that a JavaScript-less browser is on in a browser with JavaScript and the same cookies,
 * for what needs a script in the page: axe. Contexts made in a test inherit the project's options (device,
 * locale), so only JavaScript and the cookies are set here.
 */
async function withJavaScript(
  browser: Browser,
  context: BrowserContext,
  page: Page,
  check: (page: Page, makeAxeBuilder: () => AxeBuilder) => Promise<void>,
): Promise<void> {
  const enabled = await browser.newContext({ javaScriptEnabled: true, storageState: await context.storageState() });
  try {
    const other = await enabled.newPage();
    await other.goto(page.url());
    await check(other, () => new AxeBuilder({ page: other }).withTags([...axeTags]));
  } finally {
    await enabled.close();
  }
}

async function signIn(page: Page): Promise<void> {
  await emailField(page).fill(admin.email);
  await passwordField(page, "current-password").fill(admin.password);
  await submitButton(page).click();
}

test.describe("sign-in page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/auth/sign_in");
    await expectPageBasics(page);
  });

  test("has no axe violations in the light and the dark theme", async ({ page, makeAxeBuilder }) => {
    await expectNoAxeViolationsInBothThemes(page, makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });

  test("labels its fields for password managers", async ({ page }) => {
    await expect(emailField(page)).toHaveAccessibleName(/\S/);
    await expect(emailField(page)).toHaveAttribute("required", "");
    await expect(passwordField(page, "current-password")).toHaveAccessibleName(/\S/);
    await expect(page.getByRole("main")).toMatchAriaSnapshot(`
      - main:
        - heading [level=1]
        - textbox
        - button
    `);
  });

  test("shows a focus ring on keyboard focus", async ({ page }) => {
    await page.keyboard.press("Tab");
    await expect(emailField(page)).toBeFocused();
    const outline = await emailField(page).evaluate((el) => getComputedStyle(el).outlineStyle);
    expect(outline).toBe("solid");
  });

  test("has 44 px targets on touch screens", async ({ page }) => {
    const coarse = await page.evaluate(() => matchMedia("(pointer: coarse)").matches);
    test.skip(!coarse, "only coarse pointers get the larger targets (web-client.md D4)");
    for (const target of [emailField(page), passwordField(page, "current-password"), submitButton(page)]) {
      const box = await target.boundingBox();
      expect(box?.height ?? 0).toBeGreaterThanOrEqual(44);
    }
  });

  test("associates the error with the fields when the form expired", async ({ page, context, makeAxeBuilder }) => {
    // Without the anti-forgery cookie the server rejects the form before checking the password, so the
    // test uses up no sign-in rate limit.
    await context.clearCookies({ name: "__Host-my_csrf" });
    await signIn(page);
    await expect(page.getByRole("alert")).toBeVisible();
    await expectDescribedError(page, emailField(page), "sign-in-error");
    await expectDescribedError(page, passwordField(page, "current-password"), "sign-in-error");
    await expect(emailField(page)).toHaveValue(admin.email);
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});

test.describe("without JavaScript", () => {
  test.use({ javaScriptEnabled: false });

  test("signs in and shows the signed-in page", async ({ browser, context, page }) => {
    await page.goto("/auth/sign_in");
    await signIn(page);
    await expect(page).toHaveURL(`${serverURL}/`);

    await page.goto("/auth/sign_in");
    await expectPageBasics(page);
    await expect(page.locator('form[action="/auth/sign_out"] button[type="submit"]')).toBeVisible();
    await withJavaScript(browser, context, page, async (signedIn, makeAxeBuilder) => {
      await expect(signedIn.locator('form[action="/auth/sign_out"]')).toBeVisible();
      await expectNoAxeViolationsInBothThemes(signedIn, makeAxeBuilder);
      await expectNoHorizontalScroll(signedIn);
    });
  });

  test("creates an account and shows that it waits for approval", async ({ browser, context, page }, testInfo) => {
    // Every sign-up counts against a per-address limit (5 in 30 minutes), so one project creates the account.
    test.skip(testInfo.project.name !== "en-320", "the sign-up rate limit allows only a few accounts per run");
    const username = `e2e_${randomBytes(4).toString("hex")}`;
    await page.goto("/auth/sign_up");
    await page.locator('input[autocomplete="nickname"]').fill(username);
    await emailField(page).fill(`${username}@example.com`);
    await passwordField(page, "new-password").fill("e2e-password-new-user");
    await page.locator("textarea").fill("E2E");
    await submitButton(page).click();

    await expect(page).toHaveURL(/\/auth\/sign_in$/);
    await expectPageBasics(page);
    await expect(page.locator('form[action="/auth/sign_out"]')).toBeVisible();
    await withJavaScript(browser, context, page, async (status, makeAxeBuilder) => {
      await expect(status.locator('form[action="/auth/sign_out"]')).toBeVisible();
      await expectNoAxeViolationsInBothThemes(status, makeAxeBuilder);
      await expectNoHorizontalScroll(status);
    });
  });
});

test.describe("sign-up page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/auth/sign_up");
    await expectPageBasics(page);
  });

  test("has no axe violations in the light and the dark theme", async ({ page, makeAxeBuilder }) => {
    await expectNoAxeViolationsInBothThemes(page, makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });

  test("describes every field by its hint", async ({ page }) => {
    for (const field of [page.locator('input[autocomplete="nickname"]'), emailField(page), passwordField(page, "new-password")]) {
      await expect(field).toHaveAccessibleName(/\S/);
      await expect(field).toHaveAccessibleDescription(/\S/);
    }
  });

  test("associates each field error with its field", async ({ page, makeAxeBuilder }) => {
    // The server checks the fields before the sign-up counts against its rate limit.
    const username = page.locator('input[autocomplete="nickname"]');
    await username.fill("not a username!");
    await emailField(page).fill("someone@example.com");
    await passwordField(page, "new-password").fill("e2e-password-new-user");
    await submitButton(page).click();

    await expect(page.getByRole("alert")).toBeVisible();
    await expectDescribedError(page, username, "sign-up-username-error");
    await expect(username).toHaveAttribute("aria-describedby", /sign-up-username-hint/);
    await expect(emailField(page)).not.toHaveAttribute("aria-invalid", "true");
    await expectNoAxeViolations(makeAxeBuilder);
    await expectNoHorizontalScroll(page);
  });
});

test.describe("OAuth consent page", () => {
  /** An authorization request of the third-party client, with a PKCE challenge (RFC 7636). */
  function authorizeURL(state: string): string {
    const challenge = createHash("sha256").update(randomBytes(32).toString("base64url")).digest("base64url");
    const query = new URLSearchParams({
      response_type: "code",
      client_id: thirdPartyClient.clientID,
      redirect_uri: thirdPartyClient.redirectURI,
      scope: "read",
      state,
      code_challenge: challenge,
      code_challenge_method: "S256",
    });
    return `/oauth/authorize?${query.toString()}`;
  }

  test.beforeEach(async ({ page }) => {
    await page.goto(authorizeURL("e2e-state"));
    // Without a session the endpoint asks for a sign-in and comes back.
    await expect(page).toHaveURL(/\/auth\/sign_in\?return_to=/);
    await signIn(page);
    await expect(page).toHaveURL(/\/oauth\/authorize\?/);
    await expectPageBasics(page);
    await expect(page.getByRole("main")).toContainText(thirdPartyClient.name);
  });

  test("has no axe violations in the light and the dark theme", async ({ page, makeAxeBuilder }) => {
    await expectNoAxeViolationsInBothThemes(page, makeAxeBuilder);
  });

  test("does not scroll horizontally", async ({ page }) => {
    await expectNoHorizontalScroll(page);
  });

  test("lists the scopes and offers to approve or deny", async ({ page }) => {
    await expect(page.getByRole("main")).toMatchAriaSnapshot(`
      - main:
        - heading [level=1]
        - list:
          - listitem
        - button
        - button
    `);
  });

  test.describe("without JavaScript", () => {
    test.use({ javaScriptEnabled: false });

    test("approving redirects to the client with a code", async ({ page }) => {
      await page.locator('button[name="decision"][value="approve"]').click();
      await expect(page).toHaveURL((url) => `${url.origin}${url.pathname}` === thirdPartyClient.redirectURI);
      const query = new URL(page.url()).searchParams;
      expect(query.get("code")).toMatch(/\S/);
      expect(query.get("state")).toBe("e2e-state");
      expect(query.get("iss")).toBe(serverURL.replace(/^http:/, "https:"));
    });
  });
});
