// SPDX-License-Identifier: AGPL-3.0-only
// A post's page and the actions on posts through the whole app (web-client.md T4): the real router and
// QueryClient, MSW for the client API. Time is fixed so that relative times are known.
import { screen, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { apiBase, oauthPaths, problem, tokenResponse } from "../../test/msw/http";
import { remoteStatusFixture, statusFixture, statusPage, timelineStatusFixture, viewerFixture, viewerStatusFixture } from "../../test/fixtures";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import type { Status } from "./api/status";

const postId = "0192f0c4-7a8e-7000-8000-0000000000c1";
const post: Status = { ...statusFixture(postId), content: "<p>The post</p>", favourites_count: 2, boosts_count: 1, replies_count: 1 };
const ancestor: Status = { ...statusFixture("0192f0c4-7a8e-7000-8000-0000000000c0"), content: "<p>The question</p>" };
const reply: Status = {
  ...viewerStatusFixture({ id: "0192f0c4-7a8e-7000-8000-0000000000c2", content: "<p>The answer</p>", created_at: "2026-10-16T11:00:00Z" }),
  in_reply_to_id: postId,
};

/** Resumes a session with the token the default handlers accept. */
function signedIn() {
  server.use(http.post(oauthPaths.refresh, () => tokenResponse("test-access-token")));
}

/** Answers GET /statuses/{id} with `statuses` by ID, and the thread of `post`. */
function serveThread(...statuses: Status[]) {
  server.use(
    http.get(`${apiBase}/statuses/:id/context`, ({ params }) =>
      params["id"] === postId ? HttpResponse.json({ ancestors: [ancestor], descendants: [reply] }) : HttpResponse.json({ ancestors: [], descendants: [] }),
    ),
    http.get(`${apiBase}/statuses/:id`, ({ params }) => {
      const found = [post, ancestor, reply, ...statuses].find((s) => s.id === params["id"]);
      return found === undefined ? problem(404, "/api/v1/problems/not-found", "Not Found") : HttpResponse.json(found);
    }),
  );
}

/** A request the test answers when it chooses, to look at the page while the request is on its way. */
function deferred() {
  let release: (response: Response) => void = () => undefined;
  const response = new Promise<Response>((resolve) => {
    release = resolve;
  });
  return {
    response,
    release: (answer: Response) => {
      release(answer);
    },
  };
}

/** Finds a visible toast by its text (the announcer's live regions say it too). */
async function findToast(text: string) {
  return within(await screen.findByRole("region", { name: "Messages" })).findByText(text);
}

/** The article of the post the page is about. */
async function findFocusedPost() {
  return screen.findByRole("article", { description: "The post" });
}

beforeEach(() => {
  // Only Date is faked: timers, MSW and user-event run as usual.
  vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-10-16T12:00:00Z") });
  sessionStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("a post's page", () => {
  test("shows the post between the posts it replies to and its replies, each under a heading", async () => {
    serveThread();
    await renderApp({ path: `/@bob/${postId}` });

    expect(await screen.findByRole("heading", { level: 1, name: "Post" })).toBeInTheDocument();
    const article = await findFocusedPost();
    // The full date instead of the relative time, and no link to the page it is on.
    expect(within(article).getByText(/October 16, 2026/)).toHaveAttribute("datetime", post.created_at);
    expect(within(article).queryByRole("link", { name: /ago/ })).not.toBeInTheDocument();

    const earlier = await screen.findByRole("region", { name: "Earlier in the thread" });
    expect(within(earlier).getByRole("article", { description: "The question" })).toBeInTheDocument();
    const replies = screen.getByRole("region", { name: "Replies" });
    expect(await within(replies).findByRole("article", { description: "The answer" })).toBeInTheDocument();
    // The thread reads top to bottom: question, post, answer.
    expect(screen.getAllByRole("article").map((a) => a.textContent)).toEqual([
      expect.stringContaining("The question"),
      expect.stringContaining("The post"),
      expect.stringContaining("The answer"),
    ]);
    expect(document.title).toBe("Post – my example");
  });

  test("is where the time of a post in a timeline leads", async () => {
    serveThread();
    server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([post]))));
    const { user, router } = await renderApp({ path: "/local" });

    const feed = await screen.findByRole("feed", { name: "Local timeline" });
    const time = within(feed).getByRole("link", { name: "3 hr. ago" });
    expect(time).toHaveAttribute("href", `/@bob/${postId}`);
    await user.click(time);

    expect(await screen.findByRole("heading", { level: 1, name: "Post" })).toHaveFocus();
    expect(router.state.location.pathname).toBe(`/@bob/${postId}`);
    expect(await findFocusedPost()).toBeInTheDocument();
  });

  test("keeps the post on screen when its thread fails, and loads the thread again on request", async () => {
    let fail = true;
    serveThread();
    server.use(
      http.get(`${apiBase}/statuses/:id/context`, () =>
        fail ? problem(503, "about:blank", "Service Unavailable") : HttpResponse.json({ ancestors: [], descendants: [reply] }),
      ),
    );
    const { user } = await renderApp({ path: `/@bob/${postId}` });

    expect(await screen.findByText("Could not load the replies: Service Unavailable")).toBeInTheDocument();
    expect(await findFocusedPost()).toBeInTheDocument();
    fail = false;
    await user.click(screen.getByRole("button", { name: "Try again" }));

    expect(await screen.findByRole("article", { description: "The answer" })).toBeInTheDocument();
  });

  test("says so when there are no replies", async () => {
    await renderApp({ path: `/@bob/${postId}` });

    expect(await screen.findByText("No replies yet.")).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Earlier in the thread" })).not.toBeInTheDocument();
  });

  test("says that a post that does not exist, or that the viewer may not see, is not found", async () => {
    serveThread();
    await renderApp({ path: "/@bob/0192f0c4-7a8e-7000-8000-0000000000ff" });

    expect(await screen.findByRole("heading", { level: 1, name: "Post not found" })).toBeInTheDocument();
    expect(screen.getByText("This post does not exist, or you cannot see it.")).toBeInTheDocument();
  });

  test("offers no actions while signed out", async () => {
    serveThread();
    await renderApp({ path: `/@bob/${postId}` });

    const article = await findFocusedPost();
    expect(within(article).queryByRole("button", { name: "Favourite" })).not.toBeInTheDocument();
    expect(within(article).queryByRole("button", { name: "Reply" })).not.toBeInTheDocument();
  });
});

describe("the actions on a post", () => {
  test("favourite at once, announce it when the server agrees, and take it back the same way", async () => {
    signedIn();
    serveThread();
    const pending = deferred();
    const requests: string[] = [];
    server.use(
      http.post(`${apiBase}/statuses/:id/favourite`, async ({ request }) => {
        requests.push(`${request.method} ${new URL(request.url).pathname}`);
        return pending.response;
      }),
      http.delete(`${apiBase}/statuses/:id/favourite`, ({ request }) => {
        requests.push(`${request.method} ${new URL(request.url).pathname}`);
        return HttpResponse.json({ ...post, favourited: false, favourites_count: 2 });
      }),
    );
    const { user } = await renderApp({ path: `/@bob/${postId}` });
    const article = await findFocusedPost();
    const favourite = await within(article).findByRole("button", { name: "Favourite", pressed: false });
    expect(favourite).toHaveAccessibleDescription("2");

    await user.click(favourite);

    // Optimistic: pressed with the count moved before the server answers.
    expect(favourite).toHaveAttribute("aria-pressed", "true");
    expect(favourite).toHaveAccessibleDescription("3");
    pending.release(HttpResponse.json({ ...post, favourited: true, favourites_count: 5 }));
    expect(await within(screen.getByRole("status")).findByText("Added to favourites")).toBeInTheDocument();
    // The server's count, which includes the others' favourites.
    expect(favourite).toHaveAccessibleDescription("5");

    await user.click(favourite);

    expect(favourite).toHaveAttribute("aria-pressed", "false");
    expect(await within(screen.getByRole("status")).findByText("Removed from favourites")).toBeInTheDocument();
    expect(requests).toEqual([`POST ${apiBase}/statuses/${postId}/favourite`, `DELETE ${apiBase}/statuses/${postId}/favourite`]);
  });

  test("set a failed boost back, and say why", async () => {
    signedIn();
    serveThread();
    server.use(http.post(`${apiBase}/statuses/:id/boost`, () => problem(503, "about:blank", "Service Unavailable")));
    const { user } = await renderApp({ path: `/@bob/${postId}` });
    const boost = await within(await findFocusedPost()).findByRole("button", { name: "Boost", pressed: false });

    await user.click(boost);

    expect(await findToast("The change was not saved: Service Unavailable")).toBeInTheDocument();
    expect(within(screen.getByRole("alert")).getByText("The change was not saved: Service Unavailable")).toBeInTheDocument();
    expect(boost).toHaveAttribute("aria-pressed", "false");
    expect(boost).toHaveAccessibleDescription("1");
  });

  test("send presses one after another, so that the last one wins, and set back only the one that failed", async () => {
    signedIn();
    serveThread();
    const first = deferred();
    const sent: string[] = [];
    server.use(
      http.post(`${apiBase}/statuses/:id/bookmark`, () => {
        sent.push("on");
        return first.response;
      }),
      http.delete(`${apiBase}/statuses/:id/bookmark`, () => {
        sent.push("off");
        return problem(503, "about:blank", "Service Unavailable");
      }),
    );
    const { user } = await renderApp({ path: `/@bob/${postId}` });
    const bookmark = await within(await findFocusedPost()).findByRole("button", { name: "Bookmark", pressed: false });

    await user.click(bookmark);
    await user.click(bookmark);

    // The second waits for the first.
    expect(bookmark).toHaveAttribute("aria-pressed", "false");
    expect(sent).toEqual(["on"]);
    first.release(HttpResponse.json({ ...post, bookmarked: true }));

    expect(await findToast("The change was not saved: Service Unavailable")).toBeInTheDocument();
    expect(sent).toEqual(["on", "off"]);
    // The removal failed, so the bookmark the server kept shows.
    expect(bookmark).toHaveAttribute("aria-pressed", "true");
  });

  test("show on every copy of the post: the boost of it in the same timeline, and its page", async () => {
    signedIn();
    serveThread();
    const booster = { ...viewerFixture, id: "0192f0c4-7a8e-7000-8000-000000000003", username: "carol", acct: "carol", display_name: "Carol" };
    const boostOfPost: Status = { ...timelineStatusFixture, id: "0192f0c4-7a8e-7000-8000-0000000000d1", account: booster, content: "", boost: post };
    server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([boostOfPost, post]))));
    const { user } = await renderApp({ path: "/local" });
    const feed = await screen.findByRole("feed", { name: "Local timeline" });
    const [boosted, original] = await within(feed).findAllByRole("article", { description: "The post" });
    if (boosted === undefined || original === undefined) throw new Error("two copies expected");

    await user.click(await within(original).findByRole("button", { name: "Favourite", pressed: false }));

    expect(within(boosted).getByRole("button", { name: "Favourite" })).toHaveAttribute("aria-pressed", "true");
    expect(await within(screen.getByRole("status")).findByText("Added to favourites")).toBeInTheDocument();

    // The page's own answer comes from the server, which now knows the favourite.
    server.use(http.get(`${apiBase}/statuses/:id`, () => HttpResponse.json({ ...post, favourited: true, favourites_count: 3 })));
    await user.click(within(original).getByRole("link", { name: "3 hr. ago" }));

    expect(await within(await findFocusedPost()).findByRole("button", { name: "Favourite", pressed: true })).toBeInTheDocument();
  });

  test("copy the post's address", async () => {
    signedIn();
    serveThread();
    const { user } = await renderApp({ path: `/@bob/${postId}` });

    await user.click(await within(await findFocusedPost()).findByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Copy link" }));

    expect(await findToast("Link copied")).toBeInTheDocument();
    // user-event's clipboard stands in for the browser's.
    expect(await navigator.clipboard.readText()).toBe(post.url);
  });

  test("reply: the form names the post, in the sheet even though the page floats no new-post button", async () => {
    signedIn();
    serveThread();
    const { user } = await renderApp({ path: `/@bob/${postId}` });

    const replyButton = await within(await findFocusedPost()).findByRole("button", { name: "Reply" });
    expect(screen.queryByRole("button", { name: "Write a post" })).not.toBeInTheDocument();
    await user.click(replyButton);

    const sheet = await screen.findByRole("dialog", { name: "New post" });
    expect(within(sheet).getByText("Replying to Bob")).toBeInTheDocument();
  });
});

describe("deleting a post", () => {
  const own: Status = { ...viewerStatusFixture({ id: "0192f0c4-7a8e-7000-8000-0000000000e1", content: "<p>My post</p>" }), in_reply_to_id: postId };

  test("is offered for the viewer's own posts only", async () => {
    signedIn();
    serveThread();
    const { user } = await renderApp({ path: `/@bob/${postId}` });

    await user.click(await within(await findFocusedPost()).findByRole("button", { name: "More actions" }));

    expect(await screen.findByRole("menuitem", { name: "Copy link" })).toBeInTheDocument();
    expect(screen.queryByRole("menuitem", { name: "Delete" })).not.toBeInTheDocument();
  });

  test("asks first, then deletes the post and leaves its page", async () => {
    signedIn();
    serveThread(own);
    const deleted: string[] = [];
    server.use(
      http.delete(`${apiBase}/statuses/:id`, ({ params }) => {
        deleted.push(String(params["id"]));
        return new HttpResponse(null, { status: 204 });
      }),
    );
    const { user, router } = await renderApp({ path: `/@alice/${own.id}` });
    const article = await screen.findByRole("article", { description: "My post" });

    await user.click(await within(article).findByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const dialog = await screen.findByRole("alertdialog", { name: "Delete this post?" });
    // The least destructive choice has the focus.
    expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    expect(await findToast("Post deleted")).toBeInTheDocument();
    expect(deleted).toEqual([own.id]);
    // Opened directly, the page has nowhere to go back to: it leaves for the home timeline.
    expect(await screen.findByRole("heading", { level: 1, name: "Home" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/home");
  });

  test("keeps the post and the dialog when the server refuses", async () => {
    signedIn();
    serveThread(own);
    server.use(http.delete(`${apiBase}/statuses/:id`, () => problem(503, "about:blank", "Service Unavailable")));
    const { user } = await renderApp({ path: `/@alice/${own.id}` });

    await user.click(await within(await screen.findByRole("article", { description: "My post" })).findByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    const dialog = await screen.findByRole("alertdialog", { name: "Delete this post?" });
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    expect(await findToast("Could not delete the post: Service Unavailable")).toBeInTheDocument();
    expect(dialog).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(screen.getByRole("article", { description: "My post" })).toBeInTheDocument();
  });

  test("drops it from the timeline", async () => {
    signedIn();
    server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([own, post]))));
    const { user } = await renderApp({ path: "/local" });
    const feed = await screen.findByRole("feed", { name: "Local timeline" });
    const article = within(feed).getByRole("article", { description: "My post" });

    await user.click(await within(article).findByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Delete" }));

    expect(await findToast("Post deleted")).toBeInTheDocument();
    expect(within(feed).getAllByRole("article")).toHaveLength(1);
    expect(within(feed).getByRole("article", { description: "The post" })).toBeInTheDocument();
  });
});

describe("a post of another server", () => {
  const remote = remoteStatusFixture;

  test("shows the author's full handle, its content warning, its proxied pictures, and a link to its page there", async () => {
    signedIn();
    serveThread(remote);
    const { user } = await renderApp({ path: `/@${remote.account.acct}/${remote.id}` });

    const article = await screen.findByRole("article", { name: /^Carol @carol@remote\.example / });
    expect(within(article).getByText("@carol@remote.example")).toHaveAttribute("dir", "ltr");
    // The link shows the host and is named by where it leads; the actions stay next to it.
    const original = within(article).getByRole("link", { name: "Open the original post on remote.example" });
    expect(original).toHaveAttribute("href", "https://remote.example/@carol/1");
    expect(original).toHaveTextContent("remote.example");
    expect(await within(article).findByRole("button", { name: "Reply" })).toBeInTheDocument();

    expect(within(article).getByText("Cats")).toBeVisible();
    expect(within(article).getByText("Hello from another server!")).not.toBeVisible();
    await user.click(within(article).getByRole("button", { name: "Show content", expanded: false }));

    expect(within(article).getByText("Hello from another server!")).toBeVisible();
    // Remote files have no renditions here: the card loads the media proxy's preview_url.
    expect(within(article).getByRole("img", { name: "A cat on a sofa" })).toHaveAttribute(
      "src",
      "https://my.example/media/proxy/0192f0c4-7a8e-7000-8000-0000000003d1",
    );
    expect(within(article).getByRole("img", { name: "No description" })).toHaveAttribute(
      "src",
      "https://my.example/media/proxy/0192f0c4-7a8e-7000-8000-0000000003d2",
    );
    // The badge says so to the eye (web-client.md Y9); to a screen reader it is the picture's name.
    expect(within(article).getByText("No description", { selector: "span" })).toBeVisible();
  });

  test("is the only kind of post with a link to another server", async () => {
    serveThread();
    await renderApp({ path: `/@bob/${postId}` });

    const article = await findFocusedPost();
    expect(within(article).queryByRole("link", { name: /^Open the original/ })).not.toBeInTheDocument();
  });

  test("links to its ActivityPub ID when it has no page", async () => {
    const withoutPage: Status = { ...remote };
    delete withoutPage.url;
    server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json(statusPage([withoutPage]))));
    await renderApp({ path: "/local" });

    const article = await screen.findByRole("article", { name: "Carol @carol@remote.example 3 hr. ago" });
    expect(within(article).getByRole("link", { name: "Open the original post on remote.example" })).toHaveAttribute("href", remote.uri);
  });
});
