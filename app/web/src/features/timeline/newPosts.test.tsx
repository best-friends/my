// SPDX-License-Identifier: AGPL-3.0-only
// Real-time updates of the home timeline through the whole app (web-client.md S10, Y7, T4; FR-207): the
// layout's event stream, MSW as the server's GET /streaming, and the "N new posts" button.
import { act, screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { describe, expect, test } from "vitest";
import type { Status } from "./api/timelines";
import { apiBase, oauthPaths, problem, resumableSession, tokenResponse } from "../../test/msw/http";
import { eventStreamResponse, streamingServer, type TestEventStream } from "../../test/msw/streaming";
import { preferencesFixture, statusPage, timelineStatusFixture, viewerFixture } from "../../test/fixtures";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";

/** A post newer than the fixture of the home timeline. */
function newStatus(n: number): Status {
  const id = `0192f0c4-7a8e-7000-8000-0000000001${String(n).padStart(2, "0")}`;
  return { ...timelineStatusFixture, id, uri: `https://my.example/ap/statuses/${id}`, url: `https://my.example/@bob/${id}`, content: `<p>New post ${String(n)}</p>` };
}

/** The reconnect waits about a second after a stream ends (lib/sse/stream.ts). */
const reconnectTimeout = { timeout: 3_000 };

async function openHome() {
  const view = await renderApp({ path: "/home" });
  const feed = await screen.findByRole("feed", { name: "Home" });
  return { ...view, feed };
}

describe("new posts in the home timeline", () => {
  test("wait behind the button, are not inserted on their own, and show at the top when it is pressed", async () => {
    const streams = streamingServer();
    server.use(resumableSession(), streams.handler);
    const { user, feed } = await openHome();
    await waitFor(() => {
      expect(streams.connections()).toBe(1);
    });

    streams.current().send("update", newStatus(1));
    expect(await screen.findByRole("button", { name: "1 new post" })).toBeInTheDocument();
    streams.current().send("update", newStatus(2));
    // The same entry twice counts once.
    streams.current().send("update", newStatus(2));
    const button = await screen.findByRole("button", { name: "2 new posts" });

    // The feed did not change, and the count is in a polite status region.
    expect(within(feed).getAllByRole("article")).toHaveLength(1);
    expect(screen.getAllByRole("status").filter((region) => within(region).queryByRole("button", { name: "2 new posts" }) !== null)).toHaveLength(1);

    await user.click(button);

    const articles = await within(feed).findAllByRole("article", {}, { timeout: 2_000 });
    expect(articles).toHaveLength(3);
    expect(articles[0]).toHaveAccessibleDescription("New post 2");
    expect(articles[1]).toHaveAccessibleDescription("New post 1");
    expect(articles[2]).toHaveAccessibleDescription("Hello from the local timeline!");
    // The button went away; focus is on the heading above the new posts.
    expect(screen.queryByRole("button", { name: /new post/ })).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1, name: "Home" })).toHaveFocus();
  });

  test("do not count an entry that the timeline has loaded already", async () => {
    const streams = streamingServer();
    server.use(resumableSession(), streams.handler);
    const { feed } = await openHome();
    await waitFor(() => {
      expect(streams.connections()).toBe(1);
    });

    streams.current().send("update", timelineStatusFixture);
    streams.current().send("update", newStatus(1));

    expect(await screen.findByRole("button", { name: "1 new post" })).toBeInTheDocument();
    expect(within(feed).getAllByRole("article")).toHaveLength(1);
  });

  test("that arrive while another page is open are on the timeline when it opens again, without the button", async () => {
    let homeFetches = 0;
    const streams = streamingServer();
    server.use(
      resumableSession(),
      streams.handler,
      http.get(`${apiBase}/timelines/home`, () => {
        homeFetches += 1;
        return HttpResponse.json(statusPage([timelineStatusFixture]));
      }),
    );
    const { router, queryClient } = await openHome();
    await waitFor(() => {
      expect(streams.connections()).toBe(1);
    });
    await act(() => router.navigate({ to: "/local" }));
    await screen.findByRole("feed", { name: "Local timeline" });

    // The cached home timeline is not stale yet, so opening it again does not fetch it.
    streams.current().send("update", newStatus(1));
    await waitFor(() => {
      expect(queryClient.getQueryData(["get", "/timelines/home", "pages"])).toMatchObject({ pages: [{ items: [{ id: newStatus(1).id }, {}] }] });
    }, { timeout: 3_000 });
    await act(() => router.navigate({ to: "/home" }));

    const feed = await screen.findByRole("feed", { name: "Home" });
    const articles = await within(feed).findAllByRole("article");
    expect(articles).toHaveLength(2);
    expect(articles[0]).toHaveAccessibleDescription("New post 1");
    expect(screen.queryByRole("button", { name: /new post/ })).not.toBeInTheDocument();
    expect(homeFetches).toBe(1);
  });
});

describe("the event stream", () => {
  test("reconnects after the server ends it, and counts what the home timeline got meanwhile", async () => {
    let home = [timelineStatusFixture];
    const streams = streamingServer();
    server.use(
      resumableSession(),
      streams.handler,
      http.get(`${apiBase}/timelines/home`, () => HttpResponse.json(statusPage(home))),
    );
    const { feed } = await openHome();
    await waitFor(() => {
      expect(streams.connections()).toBe(1);
    });

    // A post arrives while the stream is down; nothing replays it.
    home = [newStatus(1), timelineStatusFixture];
    streams.current().close();

    await waitFor(() => {
      expect(streams.connections()).toBe(2);
    }, reconnectTimeout);
    expect(await screen.findByRole("button", { name: "1 new post" })).toBeInTheDocument();
    expect(within(feed).getAllByRole("article")).toHaveLength(1);

    // The new stream delivers again.
    streams.current().send("update", newStatus(2));
    expect(await screen.findByRole("button", { name: "2 new posts" })).toBeInTheDocument();
  });

  test("loads the timeline again when what it missed may not join what is loaded", async () => {
    let page = statusPage([timelineStatusFixture]);
    const streams = streamingServer();
    server.use(
      resumableSession(),
      streams.handler,
      http.get(`${apiBase}/timelines/home`, () => HttpResponse.json(page)),
    );
    const { user, feed } = await openHome();
    await waitFor(() => {
      expect(streams.connections()).toBe(1);
    });

    // A whole page of news, with more behind it, arrives while the stream is down.
    page = statusPage([newStatus(3), newStatus(2)], "older");
    streams.current().close();
    await user.click(await screen.findByRole("button", { name: "2 new posts" }, reconnectTimeout));

    // The feed starts again from the new first page; the older posts follow it on scrolling.
    await waitFor(() => {
      expect(within(feed).getAllByRole("article").map((article) => article.getAttribute("aria-posinset"))).toEqual(["1", "2"]);
    });
    expect(within(feed).getAllByRole("article")[0]).toHaveAccessibleDescription("New post 3");
    expect(screen.getByRole("button", { name: "Load more posts" })).toBeInTheDocument();
  });

  test("tries again after a failed connection", async () => {
    let failures = 1;
    const streams = streamingServer();
    server.use(
      resumableSession(),
      http.get(`${apiBase}/streaming`, () => {
        if (failures === 0) return undefined;
        failures -= 1;
        return problem(503, "about:blank", "Service Unavailable");
      }),
      streams.handler,
    );
    await openHome();

    await waitFor(() => {
      expect(streams.connections()).toBe(1);
    }, reconnectTimeout);
    streams.current().send("update", newStatus(1));
    expect(await screen.findByRole("button", { name: "1 new post" })).toBeInTheDocument();
  });

  test("refreshes the access token when the server ended the stream because the token stopped being valid", async () => {
    let issued = 0;
    const tokens: string[] = [];
    const streams: TestEventStream[] = [];
    server.use(
      http.post(oauthPaths.refresh, () => {
        issued += 1;
        return tokenResponse(`token-${String(issued)}`);
      }),
      // The other operations of the page take any token, so that only the stream refreshes it.
      http.get(`${apiBase}/timelines/home`, () => HttpResponse.json(statusPage([timelineStatusFixture]))),
      http.get(`${apiBase}/accounts/verify_credentials`, () => HttpResponse.json(viewerFixture)),
      http.get(`${apiBase}/notifications/marker`, () => HttpResponse.json({ unread_count: 0 })),
      http.get(`${apiBase}/preferences`, () => HttpResponse.json(preferencesFixture)),
      http.get(`${apiBase}/streaming`, ({ request }) => {
        const token = request.headers.get("Authorization");
        tokens.push(token ?? "");
        // Only the newest token is valid.
        if (token !== `Bearer token-${String(issued)}`) return problem(401, "/api/v1/problems/invalid-token", "Invalid token");
        const { response, stream } = eventStreamResponse();
        streams.push(stream);
        return response;
      }),
    );
    await openHome();
    await waitFor(() => {
      expect(streams).toHaveLength(1);
    });

    // The token expires: the next heartbeat ends the stream, and the old token is refused.
    issued += 1;
    streams[0]?.close();

    await waitFor(() => {
      expect(streams).toHaveLength(2);
    }, reconnectTimeout);
    expect(tokens).toEqual(["Bearer token-1", "Bearer token-1", "Bearer token-3"]);
    streams[1]?.send("update", newStatus(1));
    expect(await screen.findByRole("button", { name: "1 new post" })).toBeInTheDocument();
  });

  test("closes when the session ends", async () => {
    const streams = streamingServer();
    server.use(resumableSession(), streams.handler);
    const { session } = await openHome();
    await waitFor(() => {
      expect(streams.connections()).toBe(1);
    });

    await session.signOut();

    expect(await screen.findByRole("heading", { level: 2, name: "You are not signed in" })).toBeInTheDocument();
    // The aborted stream does not come back.
    await new Promise((resolve) => setTimeout(resolve, 1_500));
    expect(streams.connections()).toBe(1);
  });
});
