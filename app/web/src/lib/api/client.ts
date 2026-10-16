// SPDX-License-Identifier: AGPL-3.0-only
// The client API client (ADR-0029): openapi-fetch typed by the types that openapi-typescript generates
// from api/openapi.yaml (schema.gen.ts, `make generate`), and openapi-react-query on top of it for the
// `queryOptions` of features/<x>/api (web-client.md S1). Its middleware
//
// - sends the access token as a Bearer token, and after a 401 refreshes it once and sends the request again;
// - turns every response with an error status into an ApiError and every network failure into a
//   NetworkError, with a localized message (errors.ts).
//
// The client sends the page's locale in Accept-Language, so that the server localizes its problems too.
import createFetchClient, { type Client, type Middleware } from "openapi-fetch";
import createQueryClient, { type OpenapiQueryClient } from "openapi-react-query";
import type { Session } from "../auth/session";
import type { Translate } from "../i18n";
import { apiErrorFromResponse, NetworkError } from "../errors";
import type { paths } from "./schema.gen";

/** The base path of the client API, `servers[0].url` in api/openapi.yaml. */
const apiBasePath = "/api/v1";

export type Api = {
  /** The openapi-fetch client, for loaders and mutations. */
  fetch: Client<paths>;
  /** The openapi-react-query client, for `queryOptions` and `useMutation`. */
  query: OpenapiQueryClient<paths>;
};

export type CreateApiOptions = {
  /** The origin of the client API (the runtime config's apiOrigin). */
  apiOrigin: string;
  session: Session;
  /** The locale the page is rendered in, sent as Accept-Language. */
  locale: string;
  /** Formats the client's messages, for errors the server did not describe. */
  t: Translate;
};

export function createApi({ apiOrigin, session, locale, t }: CreateApiOptions): Api {
  const fetch = createFetchClient<paths>({ baseUrl: new URL(apiBasePath, apiOrigin).href, headers: { "Accept-Language": locale } });
  // onResponse runs in the reverse order of registration: the retry after a refresh comes first, and
  // only its final response becomes an error.
  fetch.use(errorMiddleware(t), authMiddleware(session, t));
  return { fetch, query: createQueryClient(fetch) };
}

/** The Bearer token of a request, or null. */
function bearerToken(request: Request): string | null {
  const authorization = request.headers.get("Authorization");
  return authorization?.startsWith("Bearer ") === true ? authorization.slice("Bearer ".length) : null;
}

/**
 * Authenticates requests with the session's access token. A request waits for the session to resume
 * first, so that the first requests of a page carry the token too. After a 401 to a request that carried
 * a token, it refreshes the token once (the session serializes refreshes across tabs) and sends a copy of
 * the request with the new one; a second 401 stands.
 */
function authMiddleware(session: Session, t: Translate): Middleware {
  /** Unsent copies of the authenticated requests in flight, by request ID: a body can be read only once. */
  const copies = new Map<string, Request>();
  return {
    async onRequest({ request, id }) {
      if (session.status === "unknown") await session.restore();
      const token = session.accessToken;
      if (token === null) return undefined;
      request.headers.set("Authorization", `Bearer ${token}`);
      copies.set(id, request.clone());
      return request;
    },
    async onResponse({ request, response, id, options }) {
      const copy = copies.get(id);
      copies.delete(id);
      if (response.status !== 401 || copy === undefined) return undefined;
      try {
        const token = await session.refresh(bearerToken(request));
        if (token === null) return undefined;
        copy.headers.set("Authorization", `Bearer ${token}`);
        return await options.fetch(copy);
      } catch (error) {
        // An error thrown here skips the onError of every middleware, so it is localized here.
        throw new NetworkError(t("client:api.error.network"), { cause: error });
      }
    },
    onError({ id }) {
      copies.delete(id);
      return undefined;
    },
  };
}

/** Throws an ApiError for every response with an error status, and turns network failures into a NetworkError. */
function errorMiddleware(t: Translate): Middleware {
  return {
    async onResponse({ response }) {
      if (response.ok) return undefined;
      throw await apiErrorFromResponse(response, t);
    },
    onError({ error }) {
      // An aborted request (TanStack Query cancelling a query) stays an AbortError.
      if (error instanceof DOMException && error.name === "AbortError") return undefined;
      return new NetworkError(t("client:api.error.network"), { cause: error });
    },
  };
}
