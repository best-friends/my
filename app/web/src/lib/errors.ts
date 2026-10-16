// SPDX-License-Identifier: AGPL-3.0-only
// The errors of the API client. Every failed response becomes an ApiError that carries its RFC 9457
// problem document; a request that never got a response becomes a NetworkError. Both have a localized
// `message` that the UI can show as it is (web-client.md E2): the server localizes `title` and `detail`
// from the Accept-Language the client sends, and the client's own catalog covers the rest.
import type { Translate } from "./i18n";
import type { components } from "./api/schema.gen";

/** An RFC 9457 problem document (the `Problem` schema of api/openapi.yaml). */
export type Problem = components["schemas"]["Problem"];

/**
 * A response with an error status. It is a Problem itself, so it matches the error type that the
 * generated types give every operation; branch on `type` and `status`, never on the text.
 */
export class ApiError extends Error implements Problem {
  override name = "ApiError";
  readonly type: string;
  readonly title: string;
  readonly status: number;
  readonly detail?: string;
  readonly request_id?: string;

  constructor(problem: Problem) {
    super(problem.detail ?? problem.title);
    this.type = problem.type;
    this.title = problem.title;
    this.status = problem.status;
    if (problem.detail !== undefined) this.detail = problem.detail;
    if (problem.request_id !== undefined) this.request_id = problem.request_id;
  }
}

/**
 * The message to show for an error of the API client. The generated types call every error a Problem, but
 * the middleware throws an ApiError (a Problem with a message) or a NetworkError.
 */
export function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (isProblem(error)) return error.detail ?? error.title;
  return String(error);
}

/** A request that got no response: the network is down, or the server is unreachable. */
export class NetworkError extends Error {
  override name = "NetworkError";
}

/** Whether `value` has the required members of a Problem. */
function isProblem(value: unknown): value is Problem {
  if (typeof value !== "object" || value === null) return false;
  const { type, title, status, detail } = value as Record<string, unknown>;
  return (
    typeof type === "string" &&
    typeof title === "string" &&
    typeof status === "number" &&
    (detail === undefined || typeof detail === "string")
  );
}

/**
 * The ApiError of a response with an error status. A body that is not a problem document (a proxy's
 * HTML page, say) becomes an `about:blank` problem titled from the client's catalog.
 */
export async function apiErrorFromResponse(response: Response, t: Translate): Promise<ApiError> {
  const mediaType = response.headers.get("Content-Type")?.split(";")[0]?.trim().toLowerCase();
  if (mediaType === "application/problem+json") {
    try {
      const body: unknown = await response.json();
      if (isProblem(body)) return new ApiError(body);
    } catch {
      // Not JSON after all; described by its status below.
    }
  }
  return new ApiError({ type: "about:blank", title: t("client:api.error.status", { status: String(response.status) }), status: response.status });
}
