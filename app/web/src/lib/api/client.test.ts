// SPDX-License-Identifier: AGPL-3.0-only
// The API client's middleware against MSW: the Bearer token, the one refresh and retry after a 401, and
// errors turned into localized ApiError / NetworkError objects.
import { http, HttpResponse } from "msw";
import { afterEach, expect, test } from "vitest";
import { apiBase, instanceFixture, oauthError, oauthPaths, problem, tokenResponse, viewerFixture } from "../../test/msw/handlers";
import { server } from "../../test/msw/server";
import { Session } from "../auth/session";
import { ApiError, NetworkError } from "../errors";
import { createTranslate, loadI18n } from "../i18n";
import { createApi } from "./client";

const apiOrigin = window.location.origin;
const sessions: Session[] = [];

afterEach(() => {
  for (const session of sessions.splice(0)) session.dispose();
});

async function setUp(locale = "en") {
  const session = new Session({ apiOrigin });
  sessions.push(session);
  const i18n = await loadI18n(locale);
  return { session, api: createApi({ apiOrigin, session, locale: i18n.locale, t: createTranslate(i18n) }) };
}

test("resumes the session before the first request and sends its token as a Bearer token", async () => {
  server.use(
    http.post(oauthPaths.refresh, () => tokenResponse("access-1")),
    http.get(`${apiBase}/accounts/verify_credentials`, ({ request }) =>
      request.headers.get("Authorization") === "Bearer access-1" ? HttpResponse.json(viewerFixture) : problem(401, "/api/v1/problems/invalid-token", "Invalid token"),
    ),
  );
  const { api, session } = await setUp();

  const { data } = await api.fetch.GET("/accounts/verify_credentials");

  expect(data).toEqual(viewerFixture);
  expect(session.status).toBe("signed-in");
});

test("after a 401 refreshes the token once and sends the request again with the new one", async () => {
  let refreshes = 0;
  const authorizations: (string | null)[] = [];
  server.use(
    http.post(oauthPaths.refresh, () => {
      refreshes++;
      return tokenResponse(`access-${refreshes}`);
    }),
    http.get(`${apiBase}/accounts/verify_credentials`, ({ request }) => {
      authorizations.push(request.headers.get("Authorization"));
      return request.headers.get("Authorization") === "Bearer access-2" ? HttpResponse.json(viewerFixture) : problem(401, "/api/v1/problems/invalid-token", "Invalid token");
    }),
  );
  const { api } = await setUp();

  const { data } = await api.fetch.GET("/accounts/verify_credentials");

  expect(data).toEqual(viewerFixture);
  expect(authorizations).toEqual(["Bearer access-1", "Bearer access-2"]);
  expect(refreshes).toBe(2);
});

test("a second 401 stands as an ApiError, and a refused refresh signs out", async () => {
  let refreshes = 0;
  server.use(
    http.post(oauthPaths.refresh, () => {
      refreshes++;
      return refreshes === 1 ? tokenResponse("access-1") : oauthError("invalid_grant");
    }),
    http.get(`${apiBase}/accounts/verify_credentials`, () => problem(401, "/api/v1/problems/invalid-token", "The access token is invalid.")),
  );
  const { api, session } = await setUp();

  const failure = api.fetch.GET("/accounts/verify_credentials");

  await expect(failure).rejects.toBeInstanceOf(ApiError);
  await expect(failure).rejects.toMatchObject({ status: 401, type: "/api/v1/problems/invalid-token", message: "The access token is invalid." });
  expect(session.status).toBe("signed-out");
  expect(refreshes).toBe(2);
});

test("sends the page's locale and keeps the server's localized problem as the message", async () => {
  server.use(
    http.get(`${apiBase}/instance`, ({ request }) =>
      HttpResponse.json(
        { type: "about:blank", title: "サービスを利用できません", detail: `${request.headers.get("Accept-Language") ?? ""} で説明しています`, status: 503 },
        { status: 503, headers: { "Content-Type": "application/problem+json" } },
      ),
    ),
  );
  const { api } = await setUp("ja");

  await expect(api.fetch.GET("/instance")).rejects.toMatchObject({ name: "ApiError", status: 503, title: "サービスを利用できません", message: "ja で説明しています" });
});

test("describes an error without a problem document from the client's catalog", async () => {
  server.use(http.get(`${apiBase}/instance`, () => new HttpResponse("<h1>Bad Gateway</h1>", { status: 502, headers: { "Content-Type": "text/html" } })));
  const { api } = await setUp();

  await expect(api.fetch.GET("/instance")).rejects.toMatchObject({
    status: 502,
    type: "about:blank",
    message: "The server could not handle the request (error 502). Try again later.",
  });
});

test("turns a network failure into a localized NetworkError", async () => {
  server.use(http.get(`${apiBase}/instance`, () => HttpResponse.error()));
  const { api } = await setUp("ja");

  const failure = api.fetch.GET("/instance");

  await expect(failure).rejects.toBeInstanceOf(NetworkError);
  await expect(failure).rejects.toThrow("サーバーに接続できませんでした。接続を確かめて、もう一度試してください。");
});

test("answers without a token when signed out", async () => {
  let authorization: string | null = "unset";
  server.use(
    http.get(`${apiBase}/instance`, ({ request }) => {
      authorization = request.headers.get("Authorization");
      return HttpResponse.json(instanceFixture);
    }),
  );
  const { api, session } = await setUp();

  const { data } = await api.fetch.GET("/instance");

  expect(data).toEqual(instanceFixture);
  expect(authorization).toBeNull();
  expect(session.status).toBe("signed-out");
});
