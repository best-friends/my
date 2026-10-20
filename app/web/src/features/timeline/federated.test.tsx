// SPDX-License-Identifier: AGPL-3.0-only
// The federated timeline through the whole app (web-client.md T4): its page with the loading, empty and
// error states, and its navigation item, which the instance setting federated_timeline_enabled turns off
// (FR-202). The real router and QueryClient, MSW for the client API.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { instanceFixture, statusPage, timelineStatusFixture } from "../../test/fixtures";
import { findLoadingRegion } from "../../test/loading";
import { apiBase, problem } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";

/** GET /instance with the given public timelines turned on or off. */
function instanceWith(timelines: { local: boolean; federated: boolean }) {
  return http.get(`${apiBase}/instance`, () => HttpResponse.json({ ...instanceFixture, timelines }));
}

/** The main navigation. */
function navigation() {
  return screen.getByRole("navigation", { name: "Main" });
}

beforeEach(() => {
  // Only Date is faked: timers, MSW and user-event run as usual.
  vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-10-16T12:00:00Z") });
  sessionStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("the federated timeline", () => {
  test("shows the posts of this server and of other servers as the articles of a feed named by the page heading", async () => {
    await renderApp({ path: "/public" });

    const feed = await screen.findByRole("feed", { name: "Federated timeline" });
    expect(document.title).toBe("Federated timeline – my example");
    expect(within(feed).getAllByRole("article")).toHaveLength(2);
    expect(within(feed).getByRole("article", { name: "Bob @bob 3 hr. ago" })).toHaveAttribute("aria-posinset", "1");
    const remote = within(feed).getByRole("article", { name: /^Carol @carol@remote\.example/ });
    expect(remote).toHaveAttribute("aria-posinset", "2");
    expect(remote).toHaveAttribute("aria-setsize", "2");
    // The post of another server links to its page there.
    expect(within(remote).getByRole("link", { name: "Open the original post on remote.example (opens in a new tab)" })).toHaveAttribute("href", "https://remote.example/@carol/1");
  });

  test("shows the skeleton while the first page loads after a navigation", async () => {
    let release = () => {};
    const released = new Promise<void>((resolve) => {
      release = resolve;
    });
    server.use(
      http.get(`${apiBase}/timelines/public`, async () => {
        await released;
        return HttpResponse.json(statusPage([timelineStatusFixture]));
      }),
    );
    // The router measures its pending delay with Date, so the clock has to run here.
    vi.useRealTimers();
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    await user.click(await within(navigation()).findByRole("link", { name: "Federated" }));

    // After the router's pending delay, the page has its heading and a skeleton named once as loading posts.
    await findLoadingRegion("Loading posts");
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Federated timeline" })).toBeInTheDocument();
    expect(screen.queryByRole("feed")).not.toBeInTheDocument();
    release();
    expect(await screen.findByRole("feed", { name: "Federated timeline" })).toBeInTheDocument();
  });

  test("says so when there are no posts", async () => {
    server.use(http.get(`${apiBase}/timelines/public`, () => HttpResponse.json(statusPage([]))));
    await renderApp({ path: "/public" });

    expect(await screen.findByRole("heading", { level: 2, name: "No posts yet" })).toBeInTheDocument();
    expect(screen.getByText("Public posts of people here and on the servers this one knows will appear here.")).toBeInTheDocument();
    expect(screen.queryByRole("feed")).not.toBeInTheDocument();
  });

  test("shows the page's error when the first page fails, and loads it again on request", async () => {
    let fail = true;
    server.use(
      http.get(`${apiBase}/timelines/public`, () =>
        fail ? problem(503, "about:blank", "Service Unavailable") : HttpResponse.json(statusPage([timelineStatusFixture])),
      ),
    );
    const { user } = await renderApp({ path: "/public" });

    expect(await screen.findByRole("heading", { level: 2, name: "Could not load this page" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Federated timeline" })).toBeInTheDocument();
    fail = false;
    await user.click(screen.getByRole("button", { name: "Try again" }));
    // The router reruns the loader after its pending delay: about a second in jsdom.
    expect(await screen.findByRole("feed", { name: "Federated timeline" })).toBeInTheDocument();
  });

  test("says that the server has turned it off when its address is a 404", async () => {
    server.use(
      instanceWith({ local: true, federated: false }),
      http.get(`${apiBase}/timelines/public`, () => problem(404, "about:blank", "Not Found")),
    );
    await renderApp({ path: "/public" });

    expect(await screen.findByRole("heading", { level: 1, name: "The federated timeline is off" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Go to the home timeline" })).toHaveAttribute("href", "/home");
    expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
  });

  test("is in Japanese in the ja locale", async () => {
    await renderApp({ path: "/public", locale: "ja" });

    expect(await screen.findByRole("feed", { name: "連合タイムライン" })).toBeInTheDocument();
    expect(await within(screen.getByRole("navigation", { name: "メイン" })).findByRole("link", { name: "連合" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });
});

describe("the navigation's timelines", () => {
  test("offer the federated timeline after the local one while the server serves it, and open it", async () => {
    const { user } = await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    const federated = await within(navigation()).findByRole("link", { name: "Federated" });
    // jsdom applies no styles, so the items of the rail only are here too. Local and Federated are items of
    // their own at every width.
    expect(within(navigation()).getAllByRole("link").map((link) => link.getAttribute("href"))).toEqual([
      "/home",
      "/local",
      "/public",
      "/notifications",
      "/search",
      "/profile",
      "/bookmarks",
      "/settings",
    ]);

    await user.click(federated);

    const heading = await screen.findByRole("heading", { level: 1, name: "Federated timeline" });
    await waitFor(() => {
      expect(heading).toHaveFocus();
    });
    expect(within(navigation()).getByRole("link", { name: "Federated" })).toHaveAttribute("aria-current", "page");
  });

  test("leave out the federated timeline when the server has turned it off", async () => {
    let answered = false;
    server.use(
      http.get(`${apiBase}/instance`, () => {
        answered = true;
        return HttpResponse.json({ ...instanceFixture, timelines: { local: true, federated: false } });
      }),
    );
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });
    await waitFor(() => {
      expect(answered).toBe(true);
    });

    expect(within(navigation()).getByRole("link", { name: "Local" })).toBeInTheDocument();
    expect(within(navigation()).queryByRole("link", { name: "Federated" })).not.toBeInTheDocument();
  });

  test("leave out the local timeline when the server has turned it off", async () => {
    server.use(instanceWith({ local: false, federated: true }));
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    await within(navigation()).findByRole("link", { name: "Federated" });
    expect(within(navigation()).queryByRole("link", { name: "Local" })).not.toBeInTheDocument();
  });

  test("do not offer the federated timeline while the server's answer is missing", async () => {
    server.use(http.get(`${apiBase}/instance`, () => problem(503, "about:blank", "Service Unavailable")));
    await renderApp({ path: "/home" });
    await screen.findByRole("heading", { level: 1, name: "Home" });

    expect(within(navigation()).getByRole("link", { name: "Local" })).toBeInTheDocument();
    expect(within(navigation()).queryByRole("link", { name: "Federated" })).not.toBeInTheDocument();
  });
});
