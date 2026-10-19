// SPDX-License-Identifier: AGPL-3.0-only
// A new post is in the lists that no page shows when it is posted (web-client.md S1): their routes read them
// from the cache as they are (S7), so the post fetches them again before it is shown done. The server answers
// every list slowly, as over a real network, so that a list shown from a stale cache would be seen.
import { screen, within } from "@testing-library/react";
import { delay, http, HttpResponse } from "msw";
import { describe, expect, test } from "vitest";
import { type Schemas, statusFixture, statusPage, viewerFixture, viewerStatusFixture } from "../../test/fixtures";
import { apiBase, resumableSession } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";

/** How long the server takes to answer a list, in ms. */
const latency = 300;

/** A server where the viewer's posts start empty and every POST /statuses adds one, newest first. */
function postingServer() {
  const posts: Schemas["Status"][] = [];
  server.use(
    resumableSession(),
    http.get(`${apiBase}/accounts/:id/statuses`, async () => {
      await delay(latency);
      return HttpResponse.json(statusPage(posts));
    }),
    http.get(`${apiBase}/timelines/home`, async () => {
      await delay(latency);
      return HttpResponse.json(statusPage(posts));
    }),
    http.post(`${apiBase}/statuses`, async ({ request }) => {
      const body = (await request.json()) as Schemas["CreateStatusRequest"];
      const post = viewerStatusFixture({ id: `0192f0c4-7a8e-7000-8000-${String(posts.length + 1).padStart(12, "0")}`, content: `<p>${body.text}</p>` });
      posts.unshift(post);
      return HttpResponse.json(post);
    }),
  );
}

function navigation() {
  return screen.getByRole("navigation", { name: "Main" });
}

describe("a new post", () => {
  test("is on the viewer's profile at once when it opens again after posting from the home timeline", async () => {
    postingServer();
    const { user } = await renderApp({ path: "/profile" });
    expect(await screen.findByRole("heading", { level: 1, name: viewerFixture.display_name })).toBeInTheDocument();
    expect(await screen.findByText("No posts yet")).toBeInTheDocument();

    await user.click(within(navigation()).getByRole("link", { name: "Home" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Home" })).toBeInTheDocument();
    await user.click(await screen.findByRole("button", { name: "Write a post" }));
    const sheet = await screen.findByRole("dialog", { name: "New post" });
    await user.type(await within(sheet).findByRole("textbox", { name: "Your post" }), "Fresh from the oven");
    await user.click(within(sheet).getByRole("button", { name: "Post" }));
    // The post is shown done once the lists it is in have been fetched again.
    const messages = await screen.findByRole("region", { name: "Messages" }, { timeout: latency * 5 });
    expect(within(messages).getByText("Posted.")).toBeInTheDocument();

    await user.click(within(navigation()).getByRole("link", { name: "Profile" }));
    expect(await screen.findByRole("heading", { level: 1, name: viewerFixture.display_name })).toBeInTheDocument();
    // The profile shows the new post from its first render, not the list from before it.
    expect(within(screen.getByRole("feed", { name: "Posts" })).getByText("Fresh from the oven")).toBeInTheDocument();
  });
});

describe("a reply", () => {
  test("is counted on the post it answers at once when the post's page opens again", async () => {
    const parent: Schemas["Status"] = { ...statusFixture("0192f0c4-7a8e-7000-8000-0000000000c1"), content: "<p>The question</p>" };
    const replies: Schemas["Status"][] = [];
    server.use(
      resumableSession(),
      http.get(`${apiBase}/statuses/:id`, async () => {
        await delay(latency);
        return HttpResponse.json({ ...parent, replies_count: replies.length });
      }),
      http.get(`${apiBase}/statuses/:id/context`, async () => {
        await delay(latency);
        return HttpResponse.json({ ancestors: [], descendants: replies });
      }),
      http.post(`${apiBase}/statuses`, async ({ request }) => {
        const body = (await request.json()) as Schemas["CreateStatusRequest"];
        const reply = { ...viewerStatusFixture({ id: "0192f0c4-7a8e-7000-8000-0000000000c2", content: `<p>${body.text}</p>` }), in_reply_to_id: parent.id };
        replies.push(reply);
        return HttpResponse.json(reply);
      }),
    );
    const { user, router } = await renderApp({ path: `/@bob/${parent.id}` });
    const article = await screen.findByRole("article", { description: "The question" }, { timeout: latency * 5 });
    await user.click(await within(article).findByRole("button", { name: "Reply" }));
    const sheet = await screen.findByRole("dialog", { name: "New post" });
    await user.type(within(sheet).getByRole("textbox", { name: "Your post" }), "An answer");
    await user.click(within(sheet).getByRole("button", { name: "Reply" }));
    const messages = await screen.findByRole("region", { name: "Messages" }, { timeout: latency * 5 });
    expect(within(messages).getByText("Posted.")).toBeInTheDocument();

    await user.click(within(navigation()).getByRole("link", { name: "Home" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Home" })).toBeInTheDocument();
    await router.navigate({ to: "/@{$acct}/$id", params: { acct: "bob", id: parent.id } });
    // The post counts the reply from the page's first render, not as it was before the reply.
    const again = await screen.findByRole("article", { description: "The question" });
    expect(within(again).getByRole("button", { name: "Reply" })).toHaveAccessibleDescription("1");
  });
});
