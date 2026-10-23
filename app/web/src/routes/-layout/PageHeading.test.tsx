// SPDX-License-Identifier: AGPL-3.0-only
// The page's h1 (web-client/README.md Y5): on a page whose heading only says where the user is, which the navigation
// already shows, it is visually hidden (sr-only) and stays in the accessibility tree, on the page and on its
// error; a page that says more than where the user is shows it.
import { screen } from "@testing-library/react";
import { http, HttpResponse } from "msw";
import { expect, test } from "vitest";
import { apiBase, problem, resumableSession } from "../../test/msw/http";
import { server } from "../../test/msw/server";
import { renderApp } from "../../test/render";

test("the local timeline's heading is for assistive technology only", async () => {
  server.use(http.get(`${apiBase}/timelines/local`, () => HttpResponse.json([])));
  await renderApp({ path: "/local" });

  expect(await screen.findByRole("heading", { level: 1, name: "Local timeline" })).toHaveClass("sr-only");
});

test("the notifications' heading is for assistive technology only", async () => {
  server.use(resumableSession());
  await renderApp({ path: "/notifications" });

  expect(await screen.findByRole("heading", { level: 1, name: "Notifications" })).toHaveClass("sr-only");
});

test("a page that failed keeps its hidden heading and shows the error's", async () => {
  server.use(http.get(`${apiBase}/timelines/local`, () => problem(500, "about:blank", "Internal Server Error")));
  await renderApp({ path: "/local" });

  const error = await screen.findByRole("heading", { level: 2, name: "Could not load this page" });
  expect(error).not.toHaveClass("sr-only");
  expect(screen.getByRole("heading", { level: 1, name: "Local timeline" })).toHaveClass("sr-only");
});

test("the heading that says the timeline is off is shown", async () => {
  server.use(http.get(`${apiBase}/timelines/local`, () => problem(404, "about:blank", "Not Found")));
  await renderApp({ path: "/local" });

  const heading = await screen.findByRole("heading", { level: 1, name: "The local timeline is off" });
  expect(heading).not.toHaveClass("sr-only");
});
