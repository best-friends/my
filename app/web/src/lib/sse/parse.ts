// SPDX-License-Identifier: AGPL-3.0-only
// The parser of a text/event-stream body, as the HTML Standard interprets one ("Server-sent events",
// §9.2.6 Interpreting an event stream: https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation).
// The client reads the stream with fetch, since EventSource cannot send the Authorization header
// (client-api.md §5), so it parses the body itself. The client API sends no `id` and no `retry`
// (it does not resume with Last-Event-ID), so those fields are ignored like any unknown field.

/** One dispatched event: its type (`message` when the stream named none) and its data lines joined by LF. */
export type ServerSentEvent = { event: string; data: string };

/** The end of a line: CRLF, LF or CR. */
const lineEnd = /\r\n|\n|\r/;

/** Parses an event stream that arrives in pieces; feed it text with `push`, then call `end`. */
export class EventStreamParser {
  #buffer = "";
  #started = false;
  #event = "";
  #data = "";

  /** Parses the next piece of text and returns the events it completed. */
  push(text: string): ServerSentEvent[] {
    let input = this.#buffer + text;
    if (!this.#started && input.length > 0) {
      this.#started = true;
      // A leading byte order mark is not part of the stream.
      if (input.startsWith("﻿")) input = input.slice(1);
    }
    const events: ServerSentEvent[] = [];
    for (;;) {
      const match = lineEnd.exec(input);
      if (match === null) break;
      // A CR at the very end may be the first half of a CRLF that the next piece completes.
      if (match[0] === "\r" && match.index === input.length - 1) break;
      const event = this.#line(input.slice(0, match.index));
      if (event !== null) events.push(event);
      input = input.slice(match.index + match[0].length);
    }
    this.#buffer = input;
    return events;
  }

  /** Ends the stream. An event that no blank line completed is discarded, as the standard says. */
  end(): ServerSentEvent[] {
    const events = this.#buffer.endsWith("\r") ? this.push("\n") : [];
    this.#buffer = "";
    this.#event = "";
    this.#data = "";
    return events;
  }

  /** Processes one line; a blank line dispatches the event collected so far. */
  #line(line: string): ServerSentEvent | null {
    if (line === "") return this.#dispatch();
    // A comment (": heartbeat").
    if (line.startsWith(":")) return null;
    const colon = line.indexOf(":");
    const field = colon === -1 ? line : line.slice(0, colon);
    let value = colon === -1 ? "" : line.slice(colon + 1);
    if (value.startsWith(" ")) value = value.slice(1);
    if (field === "event") this.#event = value;
    else if (field === "data") this.#data += `${value}\n`;
    return null;
  }

  #dispatch(): ServerSentEvent | null {
    const data = this.#data;
    const event = this.#event === "" ? "message" : this.#event;
    this.#event = "";
    this.#data = "";
    // An event without data is not dispatched.
    if (data === "") return null;
    return { event, data: data.slice(0, -1) };
  }
}

/** Reads the events of a text/event-stream body as they arrive. Throws when reading fails or is aborted. */
export async function* readEventStream(body: ReadableStream<Uint8Array>): AsyncGenerator<ServerSentEvent, void, undefined> {
  const reader = body.getReader();
  const decoder = new TextDecoder("utf-8");
  const parser = new EventStreamParser();
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      yield* parser.push(decoder.decode(value, { stream: true }));
    }
    yield* parser.push(decoder.decode());
    yield* parser.end();
  } finally {
    // Stops the download when the reader leaves early; the stream may already have failed.
    await reader.cancel().catch(() => undefined);
  }
}
