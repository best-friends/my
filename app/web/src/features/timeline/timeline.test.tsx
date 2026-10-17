// SPDX-License-Identifier: AGPL-3.0-only
// The timelines through the whole app (web-client.md T4): the real router and QueryClient, MSW for the
// client API. Time is fixed so that relative times are known.
import { isInaccessible, screen, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import type { Status } from "./api/timelines";
import { apiBase, oauthPaths, problem, tokenResponse } from "../../test/msw/http";
import { statusPage, timelineStatusFixture } from "../../test/fixtures";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";

/** A status like the fixture, with its own ID and text. */
function status(n: number, overrides: Partial<Status> = {}): Status {
  return { ...timelineStatusFixture, id: `0192f0c4-7a8e-7000-8000-${String(n).padStart(12, "0")}`, content: `<p>Post ${String(n)}</p>`, ...overrides };
}

beforeEach(() => {
  // Only Date is faked: timers, MSW and user-event run as usual.
  vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-10-16T12:00:00Z") });
  sessionStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("the local timeline", () => {
  test("shows the posts as the articles of a feed named by the page heading", async () => {
    await renderApp({ path: "/local" });

    const feed = await screen.findByRole("feed", { name: "Local timeline" });
    const article = within(feed).getByRole("article", { name: "Bob @bob@my.example 3 hr. ago" });
    expect(article).toHaveAccessibleDescription("Hello from the local timeline!");
    expect(article).toHaveAttribute("aria-posinset", "1");
    // The last page: the feed knows its size and offers nothing more.
    expect(article).toHaveAttribute("aria-setsize", "1");
    expect(feed).toHaveAttribute("aria-busy", "false");
    expect(screen.queryByRole("button", { name: "Load more posts" })).not.toBeInTheDocument();
    expect(within(article).getByText("3 hr. ago")).toHaveAttribute("datetime", "2026-10-16T09:00:00Z");
  });

  test("loads the older pages by their cursor when asked", async () => {
    const cursors: (string | null)[] = [];
    server.use(
      http.get(`${apiBase}/timelines/local`, ({ request }) => {
        const cursor = new URL(request.url).searchParams.get("cursor");
        cursors.push(cursor);
        return HttpResponse.json(cursor === null ? statusPage([status(1), status(2)], "page-2") : statusPage([status(3)]));
      }),
    );
    const { user } = await renderApp({ path: "/local" });

    const feed = await screen.findByRole("feed", { name: "Local timeline" });
    expect(within(feed).getAllByRole("article")).toHaveLength(2);
    // More to come: the size is unknown.
    expect(within(feed).getAllByRole("article")[0]).toHaveAttribute("aria-setsize", "-1");

    await user.click(screen.getByRole("button", { name: "Load more posts" }));

    expect(await within(feed).findByRole("article", { description: "Post 3" })).toHaveAttribute("aria-posinset", "3");
    expect(cursors).toEqual([null, "page-2"]);
    expect(within(feed).getAllByRole("article").map((a) => a.getAttribute("aria-setsize"))).toEqual(["3", "3", "3"]);
    expect(screen.queryByRole("button", { name: "Load more posts" })).not.toBeInTheDocument();
  });

  test("says so when there are no posts", async () => {
    server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([]))));
    await renderApp({ path: "/local" });

    expect(await screen.findByRole("heading", { level: 2, name: "No posts yet" })).toBeInTheDocument();
    expect(screen.queryByRole("feed")).not.toBeInTheDocument();
  });

  test("shows the page's error when the first page fails, and loads it again on request", async () => {
    let fail = true;
    server.use(
      http.get(`${apiBase}/timelines/local`, () =>
        fail ? problem(503, "about:blank", "Service Unavailable") : HttpResponse.json(statusPage([status(1)])),
      ),
    );
    const { user } = await renderApp({ path: "/local" });

    expect(await screen.findByRole("heading", { level: 2, name: "Could not load this page" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Local timeline" })).toBeInTheDocument();
    fail = false;
    await user.click(screen.getByRole("button", { name: "Try again" }));
    // The router reruns the loader after its pending delay: about a second in jsdom.
    expect(await screen.findByRole("feed", { name: "Local timeline" })).toBeInTheDocument();
  });

  test("keeps the posts when an older page fails, announces it and tries again on request", async () => {
    let failures = 1;
    server.use(
      http.get(`${apiBase}/timelines/local`, ({ request }) => {
        if (new URL(request.url).searchParams.get("cursor") === null) return HttpResponse.json(statusPage([status(1)], "page-2"));
        if (failures-- > 0) return problem(503, "about:blank", "Service Unavailable");
        return HttpResponse.json(statusPage([status(2)]));
      }),
    );
    const { user } = await renderApp({ path: "/local" });
    const feed = await screen.findByRole("feed", { name: "Local timeline" });

    await user.click(screen.getByRole("button", { name: "Load more posts" }));

    expect(await within(screen.getByRole("alert")).findByText("Could not load more posts: Service Unavailable")).toBeInTheDocument();
    expect(within(feed).getAllByRole("article")).toHaveLength(1);
    // The feed stops loading by itself; only the retry button asks again.
    expect(screen.queryByRole("button", { name: "Load more posts" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(await within(feed).findByRole("article", { description: "Post 2" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
  });
});

describe("a status card", () => {
  test("shows a boost as the boosted post under the booster's name", async () => {
    const original = status(1, { created_at: "2026-10-16T11:55:00Z" });
    const boost = status(2, {
      account: { ...timelineStatusFixture.account, id: "0192f0c4-7a8e-7000-8000-000000000003", username: "carol", acct: "carol@remote.example", display_name: "" },
      content: "",
      created_at: "2026-10-16T11:59:30Z",
      boost: original,
    });
    server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([boost]))));
    await renderApp({ path: "/local" });

    // A booster without a display name is named by the username; the time is the post's.
    const article = await screen.findByRole("article", { name: "carol boosted Bob @bob@my.example 5 min. ago" });
    expect(article).toHaveAccessibleDescription("Post 1");
  });

  test("shows the author's avatar, and the first letter of the name without one", async () => {
    const pictured = status(1, { account: { ...timelineStatusFixture.account, avatar_url: "https://my.example/media/avatar/full.png" } });
    const plain = status(2);
    server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([pictured, plain]))));
    await renderApp({ path: "/local" });

    const [withAvatar, withoutAvatar] = await screen.findAllByRole("article");
    if (withAvatar === undefined || withoutAvatar === undefined) throw new Error("want two articles");
    // The avatar is decorative (alt=""), and its link is hidden: the name is next to it.
    expect(within(withAvatar).getByRole("presentation", { hidden: true })).toHaveAttribute("src", "https://my.example/media/avatar/full.png");
    expect(within(withoutAvatar).queryByRole("presentation", { hidden: true })).not.toBeInTheDocument();
    expect(within(withoutAvatar).getByText("B")).toBeInTheDocument();
  });

  test("links the avatar for the pointer only: the name is the one tab stop to the profile", async () => {
    server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([status(1)]))));
    const { user } = await renderApp({ path: "/local" });

    const article = await screen.findByRole("article");
    // Screen readers meet the profile once, on the name, and then the time.
    const links = within(article).getAllByRole("link");
    expect(links).toHaveLength(2);
    expect(links[0]).toHaveAccessibleName("Bob @bob@my.example");
    expect(links[1]).toHaveAccessibleName("3 hr. ago");
    // The avatar's link, to the same profile, is out of the tab order inside an element hidden from them.
    const name = within(article).getByRole("link", { name: "Bob @bob@my.example" });
    const [avatar] = within(article).getAllByRole("link", { hidden: true });
    if (avatar === undefined) throw new Error("want the avatar's link");
    expect(isInaccessible(avatar)).toBe(true);
    expect(avatar).toHaveAttribute("href", name.getAttribute("href"));
    expect(avatar).toHaveAttribute("tabindex", "-1");

    // The keyboard goes from the name straight to the time, and back from the name past the avatar.
    name.focus();
    await user.tab();
    expect(within(article).getByRole("link", { name: "3 hr. ago" })).toHaveFocus();
    await user.tab({ shift: true });
    expect(name).toHaveFocus();
    await user.tab({ shift: true });
    expect(avatar).not.toHaveFocus();
  });

  test("hides a post behind its content warning, and its sensitive media behind their own toggle", async () => {
    const warned = status(1, {
      spoiler_text: "Cake",
      sensitive: true,
      created_at: "2026-10-01T09:00:00Z",
      media_attachments: [
        {
          id: "0192f0c4-7a8e-7000-8000-0000000000m1",
          type: "image",
          state: "ready",
          meta: { focus: { x: 0, y: 0 } },
          variants: {
            full: { url: "https://my.example/media/full.webp", content_type: "image/webp", width: 1600, height: 900, size: 1000 },
            thumbnail: { url: "https://my.example/media/thumb.webp", content_type: "image/webp", width: 640, height: 360, size: 100 },
          },
          url: "https://my.example/media/full.webp",
          preview_url: "https://my.example/media/thumb.webp",
        },
      ],
    });
    server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([warned]))));
    const { user } = await renderApp({ path: "/local" });

    // Two weeks ago: the date instead of a relative time.
    const article = await screen.findByRole("article", { name: "Bob @bob@my.example Oct 1, 2026" });
    expect(within(article).getByText("Cake")).toBeVisible();
    expect(within(article).getByText("Post 1")).not.toBeVisible();

    await user.click(within(article).getByRole("button", { name: "Show content", expanded: false }));

    expect(within(article).getByText("Post 1")).toBeVisible();
    const reveal = within(article).getByRole("button", { name: "Show sensitive media", pressed: false });
    expect(within(article).queryByRole("img")).not.toBeInTheDocument();
    await user.click(reveal);
    // No description: the picture is named so, and not skipped.
    expect(within(article).getByRole("img", { name: "No description" })).toHaveAttribute("src", "https://my.example/media/thumb.webp");
  });

  test("formats the relative time in the page's locale", async () => {
    await renderApp({ path: "/local", locale: "ja" });

    const feed = await screen.findByRole("feed", { name: "ローカルタイムライン" });
    expect(within(feed).getByRole("article", { name: /3 時間前$/ })).toBeInTheDocument();
  });
});

describe("the hashtag timeline", () => {
  test("is titled after the hashtag and asks for its posts", async () => {
    const hashtags: string[] = [];
    server.use(
      http.get(`${apiBase}/timelines/tag/:hashtag`, ({ params }) => {
        hashtags.push(String(params["hashtag"]));
        return HttpResponse.json(statusPage([]));
      }),
    );
    await renderApp({ path: "/tags/cats" });

    expect(await screen.findByRole("heading", { level: 1, name: "#cats" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 2, name: "No posts with #cats" })).toBeInTheDocument();
    expect(document.title).toBe("#cats – my example");
    expect(hashtags).toEqual(["cats"]);
  });
});

describe("the home timeline", () => {
  test("is where the root address leads, and offers to sign in without a session", async () => {
    const { router } = await renderApp({ path: "/" });

    expect(await screen.findByRole("heading", { level: 2, name: "You are not signed in" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/home");
    expect(screen.getByRole("link", { name: "Sign in" })).toHaveAttribute("href", "/login?return_to=%2Fhome");
  });

  test("shows the signed-in user's timeline", async () => {
    server.use(http.post(oauthPaths.refresh, () => tokenResponse("test-access-token")));
    await renderApp({ path: "/home" });

    const feed = await screen.findByRole("feed", { name: "Home" });
    expect(within(feed).getByRole("article", { description: "Hello from the local timeline!" })).toBeInTheDocument();
  });

  test("says how to fill an empty timeline", async () => {
    server.use(
      http.post(oauthPaths.refresh, () => tokenResponse("test-access-token")),
      http.get(`${apiBase}/timelines/home`, () => HttpResponse.json(statusPage([]))),
    );
    await renderApp({ path: "/home" });

    expect(await screen.findByRole("heading", { level: 2, name: "Your home timeline is empty" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Browse the local timeline" })).toHaveAttribute("href", "/local");
  });
});
