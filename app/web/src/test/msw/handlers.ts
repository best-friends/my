// SPDX-License-Identifier: AGPL-3.0-only
// Default MSW handlers: one successful response per client API operation in api/openapi.yaml, at the
// spec's path under its server URL. The fixtures take their types from the TypeScript types generated
// from the spec (web-client.md S9) once the web API client generates them (#127); API types are never
// written by hand here.
import { http, HttpResponse } from "msw";

/** The client API base path, `servers[0].url` in api/openapi.yaml. */
export const apiBase = "/api/v1";

/** A valid `Instance` for GET /instance (operationId getInstance). */
export const instanceFixture = {
  domain: "my.example",
  name: "my example",
  version: "0.0.0-test",
  source_url: "https://github.com/best-friends/my",
  limits: { max_post_characters: 500 },
};

export const handlers = [http.get(`${apiBase}/instance`, () => HttpResponse.json(instanceFixture))];
