// SPDX-License-Identifier: AGPL-3.0-only
// The client API client (ADR-0029): openapi-fetch typed by the types that openapi-typescript generates
// from api/openapi.yaml (schema.gen.ts, `make generate`), and openapi-react-query on top of it for the
// `queryOptions` of features/<x>/api (web-client/README.md S1). Its middleware
//
// - sends the access token as a Bearer token, and after a 401 refreshes it once and sends the request again;
// - turns every response with an error status into an ApiError and every network failure into a
//   NetworkError, with a localized message (errors.ts);
// - tells lib/network whether the server answered, which decides whether the app is offline (web-client/16-look-states-and-wording.md G17).
//
// The client sends the locale the app is displayed in as Accept-Language, so that the server localizes its
// problems too; it reads the locale for every request, as the user can change it while the page is open.
import createFetchClient, { type Client, type Middleware } from "openapi-fetch";
import createQueryClient, { type OpenapiQueryClient } from "openapi-react-query";
import type { Session } from "../auth/session";
import type { AppLocale } from "../locale";
import { apiErrorFromResponse, NetworkError } from "../errors";
import { isBrowserOffline, reportNetworkFailure, reportResponse } from "../network";
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
  /** The locale the app is displayed in, sent as Accept-Language, and its messages for errors the server did not describe. */
  locale: Pick<AppLocale, "locale" | "t">;
};

export function createApi({ apiOrigin, session, locale }: CreateApiOptions): Api {
  const fetch = createFetchClient<paths>({ baseUrl: new URL(apiBasePath, apiOrigin).href });
  // onResponse runs in the reverse order of registration: the retry after a refresh comes first, and
  // only its final response becomes an error.
  fetch.use(localeMiddleware(locale), errorMiddleware(locale), authMiddleware(session, locale));
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
function authMiddleware(session: Session, locale: Pick<AppLocale, "t">): Middleware {
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
        throw new NetworkError(locale.t("client:api.error.network"), { cause: error });
      }
    },
    onError({ id }) {
      copies.delete(id);
      return undefined;
    },
  };
}

/** Sends the locale the app is displayed in at the time of the request. */
function localeMiddleware(locale: Pick<AppLocale, "locale">): Middleware {
  return {
    onRequest({ request }) {
      request.headers.set("Accept-Language", locale.locale);
      return request;
    },
  };
}

/**
 * Throws an ApiError for every response with an error status, and turns network failures into a NetworkError, whose
 * message says that the app is offline when the browser has no network.
 */
function errorMiddleware(locale: Pick<AppLocale, "t">): Middleware {
  return {
    async onResponse({ response }) {
      reportResponse();
      if (response.ok) return undefined;
      throw await apiErrorFromResponse(response, locale.t);
    },
    onError({ error }) {
      // An aborted request (TanStack Query cancelling a query) stays an AbortError.
      if (error instanceof DOMException && error.name === "AbortError") return undefined;
      reportNetworkFailure();
      return new NetworkError(locale.t(isBrowserOffline() ? "client:api.error.offline" : "client:api.error.network"), { cause: error });
    },
  };
}
