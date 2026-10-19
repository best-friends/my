// SPDX-License-Identifier: AGPL-3.0-only
// A change of a relationship is in the lists that no page shows when it is made (web-client.md S1): their
// routes read them from the cache as they are (S7), so the change fetches them again before it is shown done.
// The server answers every list slowly, as over a real network, so that a list shown from a stale cache would
// be seen.
import { screen, within } from "@testing-library/react";
import { delay, http, HttpResponse } from "msw";
import { describe, expect, test } from "vitest";
import { otherAccountFixture, relationshipFixture, statusPage, timelineStatusFixture } from "../../test/fixtures";
import { apiBase, resumableSession } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";

/** How long the server takes to answer a list, in ms. */
const latency = 300;

/** A server where the viewer does not follow Bob until they follow him, and the home timeline then has his post. */
function followServer() {
  let following = false;
  server.use(
    resumableSession(),
    http.get(`${apiBase}/accounts/lookup`, () => HttpResponse.json({ ...otherAccountFixture, followers_count: following ? 1 : 0, following_count: 0 })),
    http.get(`${apiBase}/accounts/relationships`, () => HttpResponse.json([relationshipFixture(otherAccountFixture.id, { following })])),
    http.post(`${apiBase}/accounts/:id/follow`, () => {
      following = true;
      return HttpResponse.json(relationshipFixture(otherAccountFixture.id, { following }));
    }),
    http.get(`${apiBase}/timelines/home`, async () => {
      await delay(latency);
      return HttpResponse.json(statusPage(following ? [timelineStatusFixture] : []));
    }),
  );
}

describe("a follow", () => {
  test("is on the home timeline at once when it opens again after following from a profile", async () => {
    followServer();
    const { user, router } = await renderApp({ path: "/home" });
    expect(await screen.findByRole("heading", { level: 2, name: "Your home timeline is empty" }, { timeout: latency * 5 })).toBeInTheDocument();

    await router.navigate({ to: "/@{$acct}", params: { acct: otherAccountFixture.acct } });
    await user.click(await screen.findByRole("button", { name: "Follow" }));
    // The follow is shown done once the lists it changes have been fetched again.
    expect(await screen.findByText(/^You follow ⁨Bob⁩ now\.$/, {}, { timeout: latency * 5 })).toBeInTheDocument();

    await user.click(within(screen.getByRole("navigation", { name: "Main" })).getByRole("link", { name: "Home" }));
    expect(await screen.findByRole("heading", { level: 1, name: "Home" })).toBeInTheDocument();
    // The home timeline shows Bob's post from its first render, not the list from before the follow.
    expect(within(screen.getByRole("feed", { name: "Home" })).getByRole("article", { description: "Hello from the local timeline!" })).toBeInTheDocument();
  });
});
