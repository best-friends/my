// SPDX-License-Identifier: AGPL-3.0-only
// The emoji reactions of a post through the whole app (web-client.md T4, FR-116, ADR-0052): the counts under the
// post, joining and leaving one, the picker, the Like slot shared with the favourite, and the accounts that reacted.
import { screen, waitFor, within } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { apiBase, oauthPaths, problem, tokenResponse } from "../../test/msw/http";
import { type Schemas, statusFixture } from "../../test/fixtures";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";
import type { Status } from "./api/status";

const postId = "0192f0c4-7a8e-7000-8000-0000000000d1";
const blobcat: Schemas["CustomEmoji"] = { shortcode: "blobcat", url: "https://remote.example/blobcat.png", static_url: "https://remote.example/blobcat.png" };
const post: Status = {
  ...statusFixture(postId),
  content: "<p>React to me</p>",
  favourited: true,
  favourites_count: 2,
  reactions: [
    { name: "👍", count: 2, me: false },
    { name: "blobcat@remote.example", shortcode: "blobcat", domain: "remote.example", emoji: blobcat, count: 1, me: false },
  ],
};

function signedIn() {
  server.use(http.post(oauthPaths.refresh, () => tokenResponse("test-access-token")));
}

/** Serves the post, and records the emoji of each PUT of a reaction in `sent` (null for a DELETE). */
function servePost(sent: (string | null)[] = []) {
  server.use(
    http.get(`${apiBase}/statuses/:id`, () => HttpResponse.json(post)),
    http.put(`${apiBase}/statuses/:id/reaction`, async ({ request }) => {
      const { emoji } = (await request.json()) as Schemas["ReactToStatusRequest"];
      sent.push(emoji);
      const thumbs = emoji === "👍";
      return HttpResponse.json({
        ...post,
        favourited: false,
        favourites_count: 1,
        reactions: [
          { name: "👍", count: thumbs ? 3 : 2, me: thumbs },
          post.reactions[1] as Schemas["StatusReaction"],
          ...(thumbs ? [] : [{ name: emoji, count: 1, me: true }]),
        ],
      } satisfies Status);
    }),
    http.delete(`${apiBase}/statuses/:id/reaction`, () => {
      sent.push(null);
      return HttpResponse.json({ ...post, favourited: false, favourites_count: 1 });
    }),
  );
}

/** The article of the post the page is about. */
async function findPost() {
  return screen.findByRole("article", { description: "React to me" });
}

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"], now: new Date("2026-10-16T12:00:00Z") });
  sessionStorage.clear();
});

afterEach(() => {
  vi.useRealTimers();
});

describe("the reactions of a post", () => {
  test("show each emoji with its count, and pressing one reacts with it in place of the favourite", async () => {
    signedIn();
    const sent: (string | null)[] = [];
    servePost(sent);
    const { user } = await renderApp({ path: `/@bob/status/${postId}` });
    const article = await findPost();
    const reactions = within(article).getByRole("group", { name: "Reactions" });
    const thumbs = await within(reactions).findByRole("button", { name: "React with 👍" });
    expect(thumbs).toHaveAttribute("aria-pressed", "false");
    expect(thumbs).toHaveAccessibleDescription("2");
    // Another server's custom emoji is shown with its count, but nobody here can react with it (ADR-0052 §2).
    expect(within(reactions).getByRole("img", { name: ":blobcat:" })).toBeInTheDocument();
    expect(within(reactions).queryByRole("button", { name: /blobcat/ })).not.toBeInTheDocument();
    const favourite = within(article).getByRole("button", { name: "Favourite" });
    expect(favourite).toHaveAttribute("aria-pressed", "true");

    await user.click(thumbs);

    // The reaction takes the favourite's place at once.
    expect(within(article).getByRole("button", { name: "React with 👍" })).toHaveAttribute("aria-pressed", "true");
    expect(within(article).getByRole("button", { name: "Favourite" })).toHaveAttribute("aria-pressed", "false");
    await waitFor(() => {
      expect(sent).toEqual(["👍"]);
    });
    expect(within(article).getByRole("button", { name: "React with 👍" })).toHaveAccessibleDescription("3");

    // Pressing it again takes the reaction back.
    await user.click(within(article).getByRole("button", { name: "React with 👍" }));
    await waitFor(() => {
      expect(sent).toEqual(["👍", null]);
    });
    expect(within(article).getByRole("button", { name: "React with 👍" })).toHaveAttribute("aria-pressed", "false");
  });

  test("react with an emoji picked in the picker", async () => {
    signedIn();
    const sent: (string | null)[] = [];
    servePost(sent);
    const { user } = await renderApp({ path: `/@bob/status/${postId}` });
    const article = await findPost();

    await user.click(await within(article).findByRole("button", { name: "React" }));
    const picker = await screen.findByRole("dialog", { name: "Emoji to react with" });
    await waitFor(() => {
      expect(within(picker).getByRole("searchbox", { name: "Search emoji" })).toHaveFocus();
    });
    await user.click(within(picker).getByRole("row", { name: "🎉" }));

    await waitFor(() => {
      expect(screen.queryByRole("dialog", { name: "Emoji to react with" })).not.toBeInTheDocument();
    });
    expect(await within(article).findByRole("button", { name: "React with 🎉" })).toHaveAttribute("aria-pressed", "true");
    await waitFor(() => {
      expect(sent).toEqual(["🎉"]);
    });
  });

  test("put the favourite back and say why when the server refuses the reaction", async () => {
    signedIn();
    servePost();
    server.use(http.put(`${apiBase}/statuses/:id/reaction`, () => problem(422, "about:blank", "Not an emoji you can react with.")));
    const { user } = await renderApp({ path: `/@bob/status/${postId}` });
    const article = await findPost();

    await user.click(await within(article).findByRole("button", { name: "React with 👍" }));

    expect(await within(await screen.findByRole("region", { name: "Messages" })).findByText(/Not an emoji you can react with/)).toBeInTheDocument();
    expect(within(article).getByRole("button", { name: "React with 👍" })).toHaveAttribute("aria-pressed", "false");
    expect(within(article).getByRole("button", { name: "Favourite" })).toHaveAttribute("aria-pressed", "true");
  });

  test("list the accounts that reacted", async () => {
    signedIn();
    servePost();
    const { user } = await renderApp({ path: `/@bob/status/${postId}` });
    const article = await findPost();

    await user.click(within(article).getByRole("button", { name: "Who reacted" }));

    const dialog = await screen.findByRole("dialog", { name: "Who reacted" });
    const [row] = await within(dialog).findAllByRole("listitem");
    expect(within(row as HTMLElement).getByRole("link", { name: /Bob/ })).toBeInTheDocument();
    expect(row).toHaveTextContent("👍");

    // The × button closes it by click or tap alone.
    await user.click(within(dialog).getByRole("button", { name: "Close" }));
    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  test("are shown without buttons while nobody is signed in", async () => {
    servePost();
    await renderApp({ path: `/@bob/status/${postId}` });
    const article = await findPost();

    expect(within(article).getByRole("group", { name: "Reactions" })).toHaveTextContent("👍");
    expect(within(article).queryByRole("button", { name: "React with 👍" })).not.toBeInTheDocument();
  });
});
