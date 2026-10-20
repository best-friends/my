// SPDX-License-Identifier: AGPL-3.0-only
// The navigation's admin entry and its counts through the whole app (web-client.md T4, FR-712): shown only to a
// viewer with an admin permission, the same in the rail, the sidebar and the phones' menu sheet, with what awaits
// the moderators in the names of the links (and drawn as digits); the administration page it opens; and the
// counts read again when the stream tells of a new registration. jsdom applies no styles, so the rail's and the
// tab bar's items are all in the document; e2e/admin-entry.spec.ts checks which ones each width shows.
import { screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, test } from "vitest";
import { otherAccountFixture, type Schemas } from "../../test/fixtures";
import { apiBase, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { streamingServer } from "../../test/msw/streaming";
import { renderApp } from "../../test/render";
import { countsHandler, operatorHandler } from "./test/handlers";

beforeEach(() => {
  server.use(signedInHandler);
});

function navigation() {
  return screen.getByRole("navigation", { name: "Main" });
}

/** The requests for the counts that the app sent. */
function recordCountRequests() {
  const requests: string[] = [];
  server.events.on("request:start", ({ request }) => {
    if (new URL(request.url).pathname === `${apiBase}/admin/counts`) requests.push(request.method);
  });
  return requests;
}

describe("the navigation's admin entry", () => {
  test("is not there for a user without admin permissions, who is not asked for counts", async () => {
    const requests = recordCountRequests();
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    // The sidebar's account loads with the viewer's permissions; the entry would be shown by then.
    expect(await within(navigation()).findByRole("link", { name: /Alice/ })).toBeInTheDocument();

    expect(within(navigation()).queryByRole("link", { name: /^Administration/ })).not.toBeInTheDocument();
    await user.click(within(navigation()).getByRole("button", { name: "Menu" }));
    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    expect(within(sheet).getAllByRole("link").map((link) => link.textContent)).toEqual(["Profile", "Bookmarks", "Settings"]);
    expect(requests).toEqual([]);
  });

  test("names the pending count in the rail, the Menu button and the sheet, and opens the administration page", async () => {
    server.use(operatorHandler(["manage_accounts", "manage_reports"]), countsHandler({ open_reports: 1, pending_registrations: 1, pending_appeals: 1 }));
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    const rail = await within(navigation()).findByRole("link", { name: "Administration, 3 pending" });
    expect(rail).toHaveAttribute("href", "/admin");
    // The digits are drawn (not color alone) and hidden from the tree, which reads the link's name instead.
    expect(within(rail).getByText("3")).toHaveAttribute("aria-hidden", "true");
    const menu = within(navigation()).getByRole("button", { name: "Menu, 3 pending in administration" });
    expect(within(menu).getByText("3")).toHaveAttribute("aria-hidden", "true");

    await user.click(menu);
    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    expect(within(sheet).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual(["/profile", "/bookmarks", "/settings", "/admin"]);
    await user.click(within(sheet).getByRole("link", { name: "Administration, 3 pending" }));

    const heading = await screen.findByRole("heading", { level: 1, name: "Administration" });
    await waitFor(() => {
      expect(heading).toHaveFocus();
    });
    expect(within(navigation()).getByRole("link", { name: "Administration, 3 pending" })).toHaveAttribute("aria-current", "page");
    // The page links to the screens of the viewer's roles, each with its own count in its name.
    const main = within(screen.getByRole("main"));
    expect(main.getByRole("link", { name: "Registrations 1 awaits approval" })).toHaveAttribute("href", "/admin/registrations");
    expect(main.getByRole("link", { name: "Registrations 1 awaits approval" })).toHaveAccessibleDescription("Sign-ups that await approval, and approving or rejecting them.");
  });

  test("stays the current section on the admin screens", async () => {
    server.use(operatorHandler(["manage_accounts"]));
    await renderApp({ path: "/admin/registrations" });
    await screen.findByRole("heading", { level: 1, name: "Registrations" });

    expect(await within(navigation()).findByRole("link", { name: "Administration" })).toHaveAttribute("aria-current", "page");
    expect(within(navigation()).getByRole("link", { name: "Settings" })).not.toHaveAttribute("aria-current");
  });

  test("shows no badge without anything pending, and reads no counts for permissions that have none", async () => {
    server.use(operatorHandler(["manage_domains"]));
    const requests = recordCountRequests();
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    expect(await within(navigation()).findByRole("link", { name: "Administration" })).toHaveAttribute("href", "/admin");
    expect(within(navigation()).getByRole("button", { name: "Menu" })).toBeInTheDocument();
    expect(requests).toEqual([]);
  });

  test("reads the counts again when the stream tells of a new registration", async () => {
    const stream = streamingServer();
    server.use(operatorHandler(["manage_accounts"]), stream.handler, countsHandler({ pending_registrations: 1 }));
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    await within(navigation()).findByRole("link", { name: "Administration, 1 pending" });
    await waitFor(() => {
      expect(stream.connections()).toBe(1);
    });

    server.use(countsHandler({ pending_registrations: 2 }));
    stream.current().send("notification", {
      id: "0192f0c4-7a8e-7000-8000-0000000000ff",
      type: "admin_sign_up",
      created_at: "2026-10-17T09:00:00Z",
      account: otherAccountFixture,
    } satisfies Schemas["Notification"]);

    expect(await within(navigation()).findByRole("link", { name: "Administration, 2 pending" })).toBeInTheDocument();
  });
});

describe("the administration page", () => {
  test("says so to a user whose roles allow no admin screen", async () => {
    await renderApp({ path: "/admin" });

    expect(await screen.findByRole("heading", { level: 2, name: "No admin screens for your roles" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Administration" })).toBeInTheDocument();
  });
});
