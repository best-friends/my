// SPDX-License-Identifier: AGPL-3.0-only
import { http, HttpResponse } from "msw";
import { expect, test } from "vitest";
import { apiBase, instanceFixture } from "./handlers";
import { server } from "./server";

const instanceURL = new URL(`${apiBase}/instance`, window.location.href);

test("the default handler answers GET /api/v1/instance", async () => {
  const response = await fetch(instanceURL);

  expect(response.status).toBe(200);
  expect(await response.json()).toEqual(instanceFixture);
});

test("a test overrides a handler with server.use", async () => {
  server.use(
    http.get(`${apiBase}/instance`, () =>
      HttpResponse.json(
        { type: "about:blank", title: "Service Unavailable", status: 503 },
        { status: 503, headers: { "Content-Type": "application/problem+json" } },
      ),
    ),
  );

  const response = await fetch(instanceURL);

  expect(response.status).toBe(503);
});
