// SPDX-License-Identifier: AGPL-3.0-only
// Integration tests of going offline and back (web-client.md G17, T4): the app's one offline bar, what a page and a
// region show when they cannot load offline, what loads again when the app is back online, and the navigation's
// timelines on a start without the network. jsdom has no network to lose, so `goOffline` makes navigator.onLine
// false, fires the `offline` event and lets every request fail as a browser offline does; `goOnline` undoes it.
import { act, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { instanceFixture } from "./test/fixtures";
import { resumableSession } from "./test/msw/http";
import { server } from "./test/msw/server";
import { renderApp } from "./test/render";

const offlineMessage = "You are offline. Anything that failed to load loads again when you are back online.";
const offlineReason = "You are offline, so the server cannot be reached. Connect to the internet and try again.";

let offline = false;
/** Whether the server is out of reach while the browser still says it is online. */
let unreachable = false;

beforeEach(() => {
  offline = false;
  unreachable = false;
  vi.spyOn(navigator, "onLine", "get").mockImplementation(() => !offline);
  // Before every other handler: offline, no request gets a response.
  server.use(
    http.all("*", () => {
      if (offline || unreachable) return HttpResponse.error();
      return undefined;
    }),
  );
});

afterEach(() => {
  server.events.removeAllListeners("request:start");
});

function goOffline() {
  act(() => {
    offline = true;
    window.dispatchEvent(new Event("offline"));
  });
}

function goOnline() {
  act(() => {
    offline = false;
    window.dispatchEvent(new Event("online"));
  });
}

/** Asserts whether the app's offline bar shows its message, in its polite live region. */
function expectOfflineBar(shown: boolean) {
  if (shown) expect(screen.getAllByRole("status").filter((region) => within(region).queryByText(offlineMessage) !== null)).toHaveLength(1);
  else expect(screen.queryByText(offlineMessage)).not.toBeInTheDocument();
}

/** Counts the requests to `path` that got through, offline or not. */
function countRequests(path: string): { readonly count: number } {
  const counter = { count: 0 };
  server.events.on("request:start", ({ request }) => {
    if (new URL(request.url).pathname === `/api/v1${path}`) counter.count++;
  });
  return counter;
}

describe("the offline bar", () => {
  test("says so while the browser is offline", async () => {
    await renderApp({ path: "/local" });
    await screen.findByRole("feed", { name: "Local timeline" });
    expectOfflineBar(false);

    goOffline();
    expectOfflineBar(true);

    goOnline();
    expectOfflineBar(false);
  });

  test("appears when a request gets no response, and goes when one gets an answer again", async () => {
    unreachable = true;
    const { user } = await renderApp({ path: "/local" });

    // The browser still says it is online: the server is unreachable.
    expect(await screen.findByText("Could not reach the server. Check your connection and try again.")).toBeInTheDocument();
    expectOfflineBar(true);

    unreachable = false;
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("feed", { name: "Local timeline" })).toBeInTheDocument();
    expectOfflineBar(false);
  });
});

describe("offline", () => {
  test("the notifications page keeps its heading and says why the list cannot load, then loads it back online", async () => {
    server.use(resumableSession());
    const { router } = await renderApp({ path: "/home" });
    await screen.findByRole("feed", { name: "Home" });

    goOffline();
    await act(() => router.navigate({ href: "/notifications" }));

    expect(await screen.findByRole("heading", { level: 1, name: "Notifications" })).toBeInTheDocument();
    expect(await screen.findByText(offlineReason)).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();

    goOnline();
    expect(await screen.findByRole("feed", { name: "Notifications" })).toBeInTheDocument();
  });

  test("the local timeline says it cannot load instead of loading forever, and loads by itself back online", async () => {
    const { router } = await renderApp({ path: "/" });
    await screen.findByRole("heading", { level: 1 });

    goOffline();
    await act(() => router.navigate({ href: "/local" }));

    expect(await screen.findByText(offlineReason)).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();

    goOnline();
    expect(await screen.findByRole("feed", { name: "Local timeline" })).toBeInTheDocument();
  });

  test("back online, only what failed loads again: a timeline already shown is not refetched", async () => {
    server.use(resumableSession());
    const local = countRequests("/timelines/local");
    const notifications = countRequests("/notifications");
    const { router } = await renderApp({ path: "/local" });
    await screen.findByRole("feed", { name: "Local timeline" });
    expect(local.count).toBe(1);

    goOffline();
    await act(() => router.navigate({ href: "/notifications" }));
    await screen.findByText(offlineReason);
    await act(() => router.navigate({ href: "/local" }));
    await screen.findByRole("feed", { name: "Local timeline" });
    const failedTries = notifications.count;

    goOnline();
    // The failed list loads when its page opens again.
    await act(() => router.navigate({ href: "/notifications" }));
    expect(await screen.findByRole("feed", { name: "Notifications" })).toBeInTheDocument();
    expect(notifications.count).toBeGreaterThan(failedTries);
    expect(local.count).toBe(1);
  });

  test("a start without the network offers the timelines the server offered last time", async () => {
    const view = await renderApp({ path: "/local" });
    const navigation = await screen.findByRole("navigation", { name: "Main" });
    expect(await within(navigation).findByRole("link", { name: "Federated" })).toBeInTheDocument();
    view.unmount();

    // A reload offline: GET /instance gets no response.
    offline = true;
    await renderApp({ path: "/local" });
    const offlineNavigation = await screen.findByRole("navigation", { name: "Main" });
    expectOfflineBar(true);
    expect(within(offlineNavigation).getByRole("link", { name: "Federated" })).toBeInTheDocument();
  });

  test("a start without the network or a stored answer does not offer the federated timeline", async () => {
    offline = true;
    await renderApp({ path: "/local" });
    const navigation = await screen.findByRole("navigation", { name: "Main" });
    await waitFor(() => {
      expectOfflineBar(true);
    });
    expect(within(navigation).queryByRole("link", { name: "Federated" })).not.toBeInTheDocument();
    expect(within(navigation).getByRole("link", { name: "Local" })).toBeInTheDocument();
    // The instance fixture serves both; the navigation does not guess.
    expect(instanceFixture.timelines.federated).toBe(true);
  });
});
