// SPDX-License-Identifier: AGPL-3.0-only
// The navigation's staff entries and their counts through the whole app (web-client.md T4, FR-712, ADR-0058):
// Moderation shown only to a viewer with a permission of a moderation screen and Administration only to one with
// a permission of a server administration screen (a moderator never sees it), the same in the rail, the sidebar
// and the phones' menu sheet, with what awaits the moderators in the names of Moderation's links (and drawn as
// digits); the pages they open; and the counts read again when the stream tells of a new registration. jsdom applies no styles, so the rail's and the
// tab bar's items are all in the document; e2e/admin-entry.spec.ts checks which ones each width shows.
import { screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, test } from "vitest";
import { otherAccountFixture, type Schemas } from "../../test/fixtures";
import { apiBase, operatorHandler, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { streamingServer } from "../../test/msw/streaming";
import { renderApp } from "../../test/render";
import { countsHandler } from "./test/handlers";

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

describe("the navigation's staff entries", () => {
  test("are not there for a user without staff permissions, who is not asked for counts", async () => {
    const requests = recordCountRequests();
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    // The sidebar's account loads with the viewer's permissions; the entry would be shown by then.
    expect(await within(navigation()).findByRole("link", { name: /Alice/ })).toBeInTheDocument();

    expect(within(navigation()).queryByRole("link", { name: /^Moderation/ })).not.toBeInTheDocument();
    expect(within(navigation()).queryByRole("link", { name: /^Administration/ })).not.toBeInTheDocument();
    await user.click(within(navigation()).getByRole("button", { name: "Menu" }));
    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    expect(within(sheet).getAllByRole("link").map((link) => link.textContent)).toEqual(["Profile", "Bookmarks", "Settings"]);
    expect(requests).toEqual([]);
  });

  test("show a moderator Moderation alone, with the pending count in the rail, the Menu button and the sheet, and open the moderation page", async () => {
    server.use(operatorHandler(["manage_accounts", "manage_reports"]), countsHandler({ open_reports: 1, pending_registrations: 1, pending_appeals: 1 }));
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    const rail = await within(navigation()).findByRole("link", { name: "Moderation, 3 pending" });
    expect(rail).toHaveAttribute("href", "/moderation");
    expect(within(navigation()).queryByRole("link", { name: /^Administration/ })).not.toBeInTheDocument();
    // The digits are drawn (not color alone) and hidden from the tree, which reads the link's name instead.
    expect(within(rail).getByText("3")).toHaveAttribute("aria-hidden", "true");
    const menu = within(navigation()).getByRole("button", { name: "Menu, 3 pending in moderation" });
    expect(within(menu).getByText("3")).toHaveAttribute("aria-hidden", "true");

    await user.click(menu);
    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    expect(within(sheet).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual(["/profile", "/bookmarks", "/settings", "/moderation"]);
    await user.click(within(sheet).getByRole("link", { name: "Moderation, 3 pending" }));

    const heading = await screen.findByRole("heading", { level: 1, name: "Moderation" });
    await waitFor(() => {
      expect(heading).toHaveFocus();
    });
    expect(within(navigation()).getByRole("link", { name: "Moderation, 3 pending" })).toHaveAttribute("aria-current", "page");
    // The page links to the screens of the viewer's roles, each with its own count in its name.
    const main = within(screen.getByRole("main"));
    expect(main.getByRole("link", { name: "Registrations 1 awaits approval" })).toHaveAttribute("href", "/moderation/registrations");
    expect(main.getByRole("link", { name: "Registrations 1 awaits approval" })).toHaveAccessibleDescription("Sign-ups that await approval, and approving or rejecting them.");
  });

  test("Moderation stays the current section on the moderation screens", async () => {
    server.use(operatorHandler(["manage_accounts"]));
    await renderApp({ path: "/moderation/registrations" });
    await screen.findByRole("heading", { level: 1, name: "Registrations" });

    expect(await within(navigation()).findByRole("link", { name: "Moderation" })).toHaveAttribute("aria-current", "page");
    expect(within(navigation()).getByRole("link", { name: "Settings" })).not.toHaveAttribute("aria-current");
  });

  test("shows no badge without anything pending, and reads no counts for permissions that have none", async () => {
    server.use(operatorHandler(["manage_domains"]));
    const requests = recordCountRequests();
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    expect(await within(navigation()).findByRole("link", { name: "Moderation" })).toHaveAttribute("href", "/moderation");
    expect(within(navigation()).getByRole("button", { name: "Menu" })).toBeInTheDocument();
    expect(requests).toEqual([]);
  });

  test("reads the counts again when the stream tells of a new registration", async () => {
    const stream = streamingServer();
    server.use(operatorHandler(["manage_accounts"]), stream.handler, countsHandler({ pending_registrations: 1 }));
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    await within(navigation()).findByRole("link", { name: "Moderation, 1 pending" });
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

    expect(await within(navigation()).findByRole("link", { name: "Moderation, 2 pending" })).toBeInTheDocument();
  });
});

describe("the navigation's Administration", () => {
  test("follows Moderation for a user with both areas, has no count, and opens the server administration page", async () => {
    server.use(operatorHandler(["manage_reports", "manage_settings", "manage_jobs"]), countsHandler({ open_reports: 2 }));
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    await within(navigation()).findByRole("link", { name: "Moderation, 2 pending" });
    const links = within(navigation()).getAllByRole("link").map((link) => link.getAttribute("href"));
    expect(links.indexOf("/admin")).toBe(links.indexOf("/moderation") + 1);
    const admin = within(navigation()).getByRole("link", { name: "Administration" });
    expect(admin).toHaveAttribute("href", "/admin");

    await user.click(admin);
    expect(await screen.findByRole("heading", { level: 1, name: "Administration" })).toBeInTheDocument();
    expect(admin).toHaveAttribute("aria-current", "page");
    expect(within(navigation()).getByRole("link", { name: "Moderation, 2 pending" })).not.toHaveAttribute("aria-current");
    const main = within(screen.getByRole("main"));
    expect(main.getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual(["/admin/settings", "/admin/theme", "/admin/documents", "/admin/dead-jobs"]);
  });

  test("is the only staff entry of a user with server administration permissions alone", async () => {
    server.use(operatorHandler(["manage_jobs"]));
    const requests = recordCountRequests();
    await renderApp({ path: "/admin/dead-jobs" });
    await screen.findByRole("heading", { level: 1, name: "Failed jobs" });

    expect(await within(navigation()).findByRole("link", { name: "Administration" })).toHaveAttribute("aria-current", "page");
    expect(within(navigation()).queryByRole("link", { name: /^Moderation/ })).not.toBeInTheDocument();
    expect(requests).toEqual([]);
  });
});

describe("the pages of the staff's areas", () => {
  test("the moderation page says so to a user whose roles allow no moderation screen", async () => {
    await renderApp({ path: "/moderation" });

    expect(await screen.findByRole("heading", { level: 2, name: "No moderation screens for your roles" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Moderation" })).toBeInTheDocument();
  });

  test("the administration page says so to a moderator, whose roles allow no server administration screen", async () => {
    server.use(operatorHandler(["manage_reports", "manage_accounts", "view_audit"]));
    await renderApp({ path: "/admin" });

    expect(await screen.findByRole("heading", { level: 2, name: "No server administration screens for your roles" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Administration" })).toBeInTheDocument();
  });
});
