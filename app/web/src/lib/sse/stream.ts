// SPDX-License-Identifier: AGPL-3.0-only
// The connection to GET /api/v1/streaming (client-api.md §5): the viewer's new home entries (`update`),
// notifications (`notification`) and the server's announcements (`announcement`, `announcement_removed`) as
// Server-Sent Events. It is read with the API client's fetch, so
// the request carries the session's Bearer token and a 401 refreshes the token once through the
// session's serialized refresh (lib/api/client.ts), which EventSource could not do.
//
// The server replays nothing, so the connection reconnects after every end or failure, with an
// exponential backoff and jitter after failures, and tells its caller when a stream opens again: what
// happened in between must be read from the timelines and notifications again.
import type { Api } from "../api/client";
import type { components } from "../api/schema.gen";
import { readEventStream } from "./parse";

type Status = components["schemas"]["Status"];
type Notification = components["schemas"]["Notification"];
type Announcement = components["schemas"]["Announcement"];

/** An event of the stream, with its data parsed. */
export type StreamEvent =
  | { type: "update"; status: Status }
  | { type: "notification"; notification: Notification }
  | { type: "announcement"; announcement: Announcement }
  | { type: "announcement_removed"; id: string };

export type StreamCallbacks = {
  /** A stream opened; `reconnected` is false for the first one and true for every later one. */
  onOpen: (reconnected: boolean) => void;
  onEvent: (event: StreamEvent) => void;
};

/** How long to wait before connecting again, in ms. */
export type RetryDelay = (failures: number) => number;

/** The wait after a stream that ended, before the first retry. */
const baseDelay = 1_000;
/** The longest wait between failed attempts. */
const maxDelay = 60_000;

/**
 * The wait before the next connection: `baseDelay` after a stream that ended normally, doubling with each
 * failure in a row up to `maxDelay`, and spread over its upper half at random so that the clients a
 * server restart disconnected do not all come back at the same moment.
 */
export const defaultRetryDelay: RetryDelay = (failures) => {
  const delay = Math.min(maxDelay, baseDelay * 2 ** failures);
  return delay / 2 + Math.random() * (delay / 2);
};

/** Resolves after `ms`, or as soon as `signal` aborts. */
function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    const done = () => {
      clearTimeout(timer);
      signal.removeEventListener("abort", done);
      resolve();
    };
    const timer = setTimeout(done, ms);
    signal.addEventListener("abort", done, { once: true });
  });
}

function isObjectWithId(value: unknown): value is { id: string } {
  return typeof value === "object" && value !== null && typeof (value as Record<string, unknown>)["id"] === "string";
}

/** The events this client reads. */
const knownEvents: ReadonlySet<string> = new Set(["update", "notification", "announcement", "announcement_removed"]);

/**
 * The event `name` with JSON `data`, or null for an event this client does not know or data it cannot
 * read: a newer server may send more events, and a broken one is skipped rather than ending the stream.
 * The data is the server's Status, Notification or Announcement, or an announcement's ID (api/openapi.yaml);
 * the types are the generated ones (S9).
 */
export function parseStreamEvent(name: string, data: string): StreamEvent | null {
  if (!knownEvents.has(name)) return null;
  let value: unknown;
  try {
    value = JSON.parse(data);
  } catch {
    return null;
  }
  if (!isObjectWithId(value)) return null;
  switch (name) {
    case "update":
      return { type: "update", status: value as Status };
    case "notification":
      return { type: "notification", notification: value as Notification };
    case "announcement":
      return { type: "announcement", announcement: value as Announcement };
    default:
      return { type: "announcement_removed", id: value.id };
  }
}

/** Whether `signal` has aborted by now: it may abort while the stream is awaited, which narrowing cannot see. */
function isAborted(signal: AbortSignal): boolean {
  return signal.aborted;
}

/** Whether `response` is an event stream; another body (a proxy's page) is a failure to connect. */
function isEventStream(response: Response): boolean {
  return response.headers.get("Content-Type")?.split(";")[0]?.trim().toLowerCase() === "text/event-stream";
}

/**
 * Keeps a stream open until `signal` aborts: connects, delivers its events, and connects again whenever it
 * ends or fails. Resolves once `signal` has aborted; it never rejects.
 */
export async function runEventStream(api: Api, signal: AbortSignal, callbacks: StreamCallbacks, retryDelay: RetryDelay = defaultRetryDelay): Promise<void> {
  let opened = false;
  let failures = 0;
  while (!signal.aborted) {
    try {
      // With parseAs "stream", the data is the response's body itself, which is read from `response`.
      const { response } = await api.fetch.GET("/streaming", { parseAs: "stream", signal });
      const body = response.body;
      // The middleware throws for every error status (lib/api/client.ts); a 200 must also be a stream.
      if (body === null || !isEventStream(response)) {
        await body?.cancel();
        throw new Error(`GET /streaming: ${response.headers.get("Content-Type") ?? "no content type"}, not an event stream`);
      }
      callbacks.onOpen(opened);
      opened = true;
      failures = 0;
      for await (const { event, data: text } of readEventStream(body)) {
        const parsed = parseStreamEvent(event, text);
        if (parsed !== null) callbacks.onEvent(parsed);
      }
    } catch (error) {
      if (isAborted(signal)) return;
      // Offline, a 429 (too many streams), a server that went away: try again later, waiting longer.
      console.warn(error);
      failures += 1;
    }
    await sleep(retryDelay(failures), signal);
  }
}
