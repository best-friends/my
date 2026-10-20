// SPDX-License-Identifier: AGPL-3.0-only
// The links to the server's terms of service and privacy policy through the whole app (web-client.md T4, #489):
// on the sign-in page, in the settings, and in the phones' menu sheet, only for the documents the server
// publishes (GET /instance); a link loads the server's own page in place of the app.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, test } from "vitest";
import { instanceFixture } from "../../test/fixtures";
import { apiBase, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import { newTabName, visibleText } from "../../test/links";

const termsUrl = "http://localhost:3000/auth/terms";
const privacyUrl = "http://localhost:3000/auth/privacy";

function publish(documents: { terms_url?: string; privacy_policy_url?: string }) {
  server.use(http.get(`${apiBase}/instance`, () => HttpResponse.json({ ...instanceFixture, ...documents })));
}

describe("the links to the server's documents", () => {
  test("are on the sign-in page and open the server's page in a new tab", async () => {
    publish({ terms_url: termsUrl, privacy_policy_url: privacyUrl });
    await renderApp({ path: "/login" });

    const links = await screen.findByRole("navigation", { name: "Terms and policies" });
    expect(within(links).getAllByRole("link").map((link) => [visibleText(link), link.getAttribute("href")])).toEqual([
      ["Terms of service", termsUrl],
      ["Privacy policy", privacyUrl],
    ]);
    const privacy = within(links).getByRole("link", { name: newTabName("Privacy policy") });
    expect(privacy).toHaveAttribute("target", "_blank");
    expect(privacy).toHaveAttribute("rel", "noopener noreferrer");
  });

  test("leave out a document the server does not publish, and are not there without any", async () => {
    publish({ privacy_policy_url: privacyUrl });
    const view = await renderApp({ path: "/login" });
    const links = await screen.findByRole("navigation", { name: "Terms and policies" });
    expect(within(links).getAllByRole("link").map(visibleText)).toEqual(["Privacy policy"]);
    view.unmount();

    publish({});
    const { queryClient } = await renderApp({ path: "/login" });
    // The links would be there once the instance has loaded.
    await waitFor(() => {
      expect(queryClient.getQueryCache().find({ queryKey: ["get", "/instance"] })?.state.status).toBe("success");
    });
    expect(screen.queryByRole("navigation", { name: "Terms and policies" })).not.toBeInTheDocument();
  });

  test("are a section of the settings", async () => {
    server.use(signedInHandler);
    publish({ terms_url: termsUrl });
    await renderApp({ path: "/settings" });

    const section = await screen.findByRole("region", { name: "Terms and policies" });
    const links = within(section).getByRole("navigation", { name: "Terms and policies" });
    expect(within(links).getByRole("link", { name: newTabName("Terms of service") })).toHaveAttribute("href", termsUrl);
  });

  test("are in the menu sheet under its items", async () => {
    server.use(signedInHandler);
    publish({ terms_url: termsUrl, privacy_policy_url: privacyUrl });
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    await user.click(within(screen.getByRole("navigation", { name: "Main" })).getByRole("button", { name: "Menu" }));
    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    const links = within(sheet).getByRole("navigation", { name: "Terms and policies" });
    expect(within(links).getAllByRole("link").map(visibleText)).toEqual(["Terms of service", "Privacy policy"]);
  });
});
