// SPDX-License-Identifier: AGPL-3.0-only
// The main navigation through the whole app (web-client.md T4, ADR-0039): the local and the federated timeline as
// items of their own, the phones' tab bar with Menu, and the menu's sheet. jsdom applies no styles, so the items
// that only the tab bar or only the rail shows are all in the document; e2e/shell.spec.ts checks which ones each
// width shows.
import { act, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, test } from "vitest";
import { instanceFixture, preferencesFixture } from "../../test/fixtures";
import { apiBase, problem, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { navigationKey, themeKey } from "../../lib/storage";
import { renderApp } from "../../test/render";

/** GET /instance with the given public timelines turned on or off. */
function instanceWith(timelines: { local: boolean; federated: boolean }) {
  return http.get(`${apiBase}/instance`, () => HttpResponse.json({ ...instanceFixture, timelines }));
}

function navigation() {
  return screen.getByRole("navigation", { name: "Main" });
}

describe("the timelines' items", () => {
  test("open the local and the federated timeline as two screens, each the current item on its own, with no tabs", async () => {
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    const local = within(navigation()).getByRole("link", { name: "Local" });
    expect(local).toHaveAttribute("href", "/local");
    expect(local).not.toHaveAttribute("aria-current");
    await user.click(local);

    const heading = await screen.findByRole("heading", { level: 1, name: "Local timeline" });
    await waitFor(() => {
      expect(heading).toHaveFocus();
    });
    expect(within(navigation()).getByRole("link", { name: "Local" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("feed", { name: "Local timeline" })).toBeInTheDocument();
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();

    const federated = await within(navigation()).findByRole("link", { name: "Federated" });
    expect(federated).toHaveAttribute("href", "/public");
    expect(federated).not.toHaveAttribute("aria-current");
    await user.click(federated);

    expect(await screen.findByRole("feed", { name: "Federated timeline" })).toBeInTheDocument();
    expect(within(navigation()).getByRole("link", { name: "Federated" })).toHaveAttribute("aria-current", "page");
    expect(within(navigation()).getByRole("link", { name: "Local" })).not.toHaveAttribute("aria-current");
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
  });

  test("leave out both timelines when the server serves neither", async () => {
    server.use(instanceWith({ local: false, federated: false }));
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    await waitFor(() => {
      expect(within(navigation()).queryByRole("link", { name: "Local" })).not.toBeInTheDocument();
    });
    expect(within(navigation()).queryByRole("link", { name: "Federated" })).not.toBeInTheDocument();
    expect(within(navigation()).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual([
      "/home",
      "/notifications",
      "/search",
      "/profile",
      "/bookmarks",
      "/settings",
    ]);
  });

  test("say that the local timeline is off when its address is a 404", async () => {
    server.use(
      instanceWith({ local: false, federated: false }),
      http.get(`${apiBase}/timelines/local`, () => problem(404, "about:blank", "Not Found")),
    );
    await renderApp({ path: "/local" });

    expect(await screen.findByRole("heading", { level: 1, name: "The local timeline is off" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Go to the home timeline" })).toHaveAttribute("href", "/home");
    expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
  });

  test("are in Japanese in the ja locale", async () => {
    await renderApp({ path: "/home", locale: "ja" });
    await screen.findByRole("heading", { level: 1, name: "ホーム" });
    const nav = screen.getByRole("navigation", { name: "メイン" });
    expect(within(nav).getByRole("link", { name: "ローカル" })).toHaveAttribute("href", "/local");
    expect(await within(nav).findByRole("link", { name: "連合" })).toHaveAttribute("href", "/public");
  });
});

describe("the user's arrangement", () => {
  test("orders and hides the items as kept on this browser, with the shown items after the first five in the menu's sheet", async () => {
    navigationKey.write({
      order: ["search", "notifications", "home", "profile", "public", "local", "bookmarks", "settings"],
      hidden: ["local"],
    });
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    const list = within(navigation()).getByRole("list");
    await within(list).findByRole("link", { name: "Federated" });
    expect(within(list).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual([
      "/search",
      "/notifications",
      "/home",
      "/profile",
      "/public",
      "/bookmarks",
      "/settings",
    ]);

    await user.click(within(navigation()).getByRole("button", { name: "Menu" }));
    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    expect(within(sheet).getAllByRole("link").map((link) => link.textContent)).toEqual(["Bookmarks", "Settings"]);
  });

  test("keeps the place of a timeline the server does not offer among the first five", async () => {
    server.use(instanceWith({ local: false, federated: true }));
    navigationKey.write({ order: ["local", "home", "public", "notifications", "search", "profile", "bookmarks", "settings"], hidden: [] });
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    await waitFor(() => {
      expect(within(navigation()).queryByRole("link", { name: "Local" })).not.toBeInTheDocument();
    });

    await user.click(within(navigation()).getByRole("button", { name: "Menu" }));
    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    expect(within(sheet).getAllByRole("link").map((link) => link.textContent)).toEqual(["Profile", "Bookmarks", "Settings"]);
  });

  test("follows a change made in another tab", async () => {
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    expect(within(navigation()).getByRole("link", { name: "Search" })).toBeInTheDocument();

    const value = JSON.stringify({ v: 1, value: { order: [], hidden: ["search"] } });
    localStorage.setItem("my:display:navigation", value);
    act(() => {
      window.dispatchEvent(new StorageEvent("storage", { key: "my:display:navigation", newValue: value }));
    });
    expect(within(navigation()).queryByRole("link", { name: "Search" })).not.toBeInTheDocument();
  });
});

describe("the tab bar's Menu", () => {
  test("is a button, never the current item, that opens a sheet with the profile, the bookmarks and the settings", async () => {
    server.use(signedInHandler);
    const { user } = await renderApp({ path: "/settings" });
    await screen.findByRole("heading", { level: 1, name: "Settings" });

    const menu = within(navigation()).getByRole("button", { name: "Menu" });
    expect(menu).not.toHaveAttribute("aria-current");
    await user.click(menu);

    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    expect(within(sheet).getAllByRole("link").map((link) => link.textContent)).toEqual(["Profile", "Bookmarks", "Settings"]);
    expect(within(sheet).getByRole("link", { name: "Settings" })).toHaveAttribute("aria-current", "page");
    expect(within(sheet).getByRole("link", { name: "Profile" })).not.toHaveAttribute("aria-current");

    await user.keyboard("{Escape}");
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(menu).toHaveFocus();
    });
  });

  test("switches the color theme in place, and the sheet stays open", async () => {
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    await user.click(within(navigation()).getByRole("button", { name: "Menu" }));
    const sheet = await screen.findByRole("dialog", { name: "Menu" });

    const theme = within(sheet).getByRole("radiogroup", { name: "Theme" });
    expect(theme).toHaveAttribute("aria-orientation", "horizontal");
    await user.click(within(theme).getByRole("radio", { name: "Dark" }));

    expect(document.documentElement).toHaveAttribute("data-theme", "dark");
    expect(themeKey.read()).toBe("dark");
    expect(screen.getByRole("dialog", { name: "Menu" })).toBeInTheDocument();
  });

  test("closes when one of its links is followed, and focus moves to the new page's heading", async () => {
    server.use(signedInHandler);
    const { user, router } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    await user.click(within(navigation()).getByRole("button", { name: "Menu" }));
    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    await user.click(within(sheet).getByRole("link", { name: "Settings" }));

    const heading = await screen.findByRole("heading", { level: 1, name: "Settings" });
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(heading).toHaveFocus();
    });
    expect(router.state.location.pathname).toBe("/settings");

    // Back on the page it was opened from, the sheet stays closed.
    await user.click(within(navigation()).getByRole("link", { name: "Home" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Home" })).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  test("stays open while leaving unsaved changes is asked about, and closes once they are discarded", async () => {
    server.use(signedInHandler);
    const { user, router } = await renderApp({ path: "/settings/profile" });
    const displayName = await screen.findByRole("textbox", { name: "Display name" });
    await user.type(displayName, " Liddell");

    await user.click(within(navigation()).getByRole("button", { name: "Menu" }));
    await user.click(within(await screen.findByRole("dialog", { name: "Menu" })).getByRole("link", { name: "Settings" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Keep editing" }));
    expect(router.state.location.pathname).toBe("/settings/profile");
    expect(screen.getByRole("dialog", { name: "Menu" })).toBeInTheDocument();

    await user.click(within(screen.getByRole("dialog", { name: "Menu" })).getByRole("link", { name: "Settings" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Discard changes" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Settings" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  test("marks the viewer's own profile as current in the sheet", async () => {
    server.use(signedInHandler);
    const { user } = await renderApp({ path: "/profile" });
    await screen.findByRole("heading", { level: 1, name: "Alice" });

    await user.click(within(navigation()).getByRole("button", { name: "Menu" }));
    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    expect(within(sheet).getByRole("link", { name: "Profile" })).toHaveAttribute("aria-current", "page");
  });

  test("is in Japanese in the ja locale", async () => {
    const { user } = await renderApp({ path: "/home", locale: "ja" });
    await screen.findByRole("heading", { level: 1, name: "ホーム" });
    const nav = screen.getByRole("navigation", { name: "メイン" });
    await user.click(within(nav).getByRole("button", { name: "メニュー" }));
    const sheet = await screen.findByRole("dialog", { name: "メニュー" });
    expect(within(sheet).getByRole("button", { name: "閉じる" })).toBeInTheDocument();
  });
});

describe("the sidebar's server and viewer", () => {
  test("names the server above the list and links the signed-in account below it, without changing the list", async () => {
    server.use(signedInHandler);
    await renderApp({ path: "/profile" });
    await screen.findByRole("heading", { level: 1, name: "Alice" });

    const nav = navigation();
    const viewer = await within(nav).findByRole("link", { name: "Alice @alice" });
    expect(viewer).toHaveAttribute("href", "/@alice");
    expect(viewer).toHaveAccessibleDescription("The account you are signed in with");
    // The list's Profile item marks the place; the viewer's link is a second way there.
    expect(viewer).not.toHaveAttribute("aria-current");
    expect(within(nav).getByRole("link", { name: "Profile" })).toHaveAttribute("aria-current", "page");

    const name = within(nav).getByText("my example");
    const list = within(nav).getByRole("list");
    expect(name.compareDocumentPosition(list) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(list.compareDocumentPosition(viewer) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(list).not.toContainElement(viewer);
    // The federated timeline joins the list once the instance says it serves it.
    await within(list).findByRole("link", { name: "Federated" });
    expect(within(list).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual([
      "/home",
      "/local",
      "/public",
      "/notifications",
      "/search",
      "/profile",
      "/bookmarks",
      "/settings",
    ]);
  });

  test("shows no account while signed out", async () => {
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    expect(within(navigation()).getByText("my example")).toBeInTheDocument();
    expect(within(navigation()).queryByRole("link", { name: /@alice/ })).not.toBeInTheDocument();
  });

  test("is in Japanese in the ja locale", async () => {
    server.use(signedInHandler, http.get(`${apiBase}/preferences`, () => HttpResponse.json({ ...preferencesFixture, locale: "ja" })));
    await renderApp({ path: "/home", locale: "ja" });
    const nav = await screen.findByRole("navigation", { name: "メイン" });
    expect(await within(nav).findByRole("link", { name: "Alice @alice" })).toHaveAccessibleDescription(
      "サインインしているアカウント",
    );
  });
});
