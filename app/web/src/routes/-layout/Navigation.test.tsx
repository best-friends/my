// SPDX-License-Identifier: AGPL-3.0-only
// The main navigation through the whole app (web-client.md T4, ADR-0039): the phones' tab bar with Timelines and
// Menu, the menu's sheet, and the Timelines screen's tabs. jsdom applies no styles, so the items that only the
// tab bar or only the rail shows are all in the document; e2e/shell.spec.ts checks which ones each width shows.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, test } from "vitest";
import { instanceFixture, preferencesFixture } from "../../test/fixtures";
import { apiBase, problem, signedInHandler } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";

/** GET /instance with the given public timelines turned on or off. */
function instanceWith(timelines: { local: boolean; federated: boolean }) {
  return http.get(`${apiBase}/instance`, () => HttpResponse.json({ ...instanceFixture, timelines }));
}

function navigation() {
  return screen.getByRole("navigation", { name: "Main" });
}

describe("the tab bar's Timelines", () => {
  test("opens the local timeline, whose page has the federated one as its other tab", async () => {
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    const timelines = within(navigation()).getByRole("link", { name: "Timelines" });
    expect(timelines).toHaveAttribute("href", "/local");
    expect(timelines).not.toHaveAttribute("aria-current");
    await user.click(timelines);

    const heading = await screen.findByRole("heading", { level: 1, name: "Local timeline" });
    await waitFor(() => {
      expect(heading).toHaveFocus();
    });
    expect(within(navigation()).getByRole("link", { name: "Timelines" })).toHaveAttribute("aria-current", "page");
    const tabs = screen.getByRole("tablist", { name: "Timelines" });
    const local = within(tabs).getByRole("tab", { name: "Local" });
    expect(local).toHaveAttribute("aria-selected", "true");
    expect(local).toHaveAttribute("href", "/local");
    expect(screen.getByRole("tabpanel", { name: "Local" })).toContainElement(screen.getByRole("feed", { name: "Local timeline" }));

    const federated = await within(tabs).findByRole("tab", { name: "Federated" });
    expect(federated).toHaveAttribute("href", "/public");
    await user.click(federated);

    expect(await screen.findByRole("feed", { name: "Federated timeline" })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Federated" })).toHaveAttribute("aria-selected", "true");
    // Timelines stays the current item of the tab bar on either tab.
    expect(within(navigation()).getByRole("link", { name: "Timelines" })).toHaveAttribute("aria-current", "page");
  });

  test("moves between the tabs with the arrow keys and opens one with Enter", async () => {
    const { user, router } = await renderApp({ path: "/local" });
    const local = await screen.findByRole("tab", { name: "Local" });
    const federated = await screen.findByRole("tab", { name: "Federated" });

    local.focus();
    await user.keyboard("{ArrowRight}");
    expect(federated).toHaveFocus();
    // Manual activation: focus alone opens nothing.
    expect(router.state.location.pathname).toBe("/local");

    await user.keyboard("{Enter}");
    expect(await screen.findByRole("feed", { name: "Federated timeline" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/public");
  });

  test("keeps its place when the server serves no federated timeline, which leaves the local tab alone", async () => {
    let answered = false;
    server.use(
      http.get(`${apiBase}/instance`, () => {
        answered = true;
        return HttpResponse.json({ ...instanceFixture, timelines: { local: true, federated: false } });
      }),
    );
    await renderApp({ path: "/local" });
    await screen.findByRole("feed", { name: "Local timeline" });
    await waitFor(() => {
      expect(answered).toBe(true);
    });

    expect(within(navigation()).getByRole("link", { name: "Timelines" })).toHaveAttribute("href", "/local");
    expect(within(screen.getByRole("tablist", { name: "Timelines" })).getAllByRole("tab").map((tab) => tab.textContent)).toEqual(["Local"]);
  });

  test("opens the federated timeline when the server serves only that one", async () => {
    server.use(instanceWith({ local: false, federated: true }));
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    await waitFor(() => {
      expect(within(navigation()).getByRole("link", { name: "Timelines" })).toHaveAttribute("href", "/public");
    });
  });

  test("stays in the tab bar when the server serves neither, and its page says the local timeline is off", async () => {
    server.use(
      instanceWith({ local: false, federated: false }),
      http.get(`${apiBase}/timelines/local`, () => problem(404, "about:blank", "Not Found")),
    );
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    await waitFor(() => {
      expect(within(navigation()).queryByRole("link", { name: "Local" })).not.toBeInTheDocument();
    });

    await user.click(within(navigation()).getByRole("link", { name: "Timelines" }));

    expect(await screen.findByRole("heading", { level: 1, name: "The local timeline is off" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Go to the home timeline" })).toHaveAttribute("href", "/home");
    expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
  });
});

describe("the tab bar's Menu", () => {
  test("is a button, never the current item, that opens a sheet with the profile and the settings", async () => {
    server.use(signedInHandler);
    const { user } = await renderApp({ path: "/settings" });
    await screen.findByRole("heading", { level: 1, name: "Settings" });

    const menu = within(navigation()).getByRole("button", { name: "Menu" });
    expect(menu).not.toHaveAttribute("aria-current");
    await user.click(menu);

    const sheet = await screen.findByRole("dialog", { name: "Menu" });
    expect(within(sheet).getAllByRole("link").map((link) => link.textContent)).toEqual(["Profile", "Settings"]);
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
    expect(localStorage.getItem("my:display:theme")).toBe("dark");
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
    expect(within(nav).getByRole("link", { name: "タイムライン" })).toBeInTheDocument();

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
    const viewer = await within(nav).findByRole("link", { name: "Alice @alice@my.example" });
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
      "/local",
      "/public",
      "/notifications",
      "/search",
      "/profile",
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
    expect(await within(nav).findByRole("link", { name: "Alice @alice@my.example" })).toHaveAccessibleDescription(
      "サインインしているアカウント",
    );
  });
});
