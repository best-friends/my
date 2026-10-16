// SPDX-License-Identifier: AGPL-3.0-only
import { describe, expect, test } from "vitest";
import { EventStreamParser, readEventStream, type ServerSentEvent } from "./parse";

function parseAll(...pieces: string[]): ServerSentEvent[] {
  const parser = new EventStreamParser();
  return [...pieces.flatMap((piece) => parser.push(piece)), ...parser.end()];
}

describe("EventStreamParser", () => {
  test("dispatches an event at a blank line, with its type and data", () => {
    expect(parseAll('event: update\ndata: {"id":"1"}\n\n')).toEqual([{ event: "update", data: '{"id":"1"}' }]);
  });

  test("skips comments, such as the server's heartbeat", () => {
    expect(parseAll(": connected\n\n: heartbeat\n\nevent: notification\ndata: x\n\n")).toEqual([{ event: "notification", data: "x" }]);
  });

  test("joins data lines with LF and names an event without a type message", () => {
    expect(parseAll("data: one\ndata: two\n\n")).toEqual([{ event: "message", data: "one\ntwo" }]);
  });

  test("removes only one space after the colon, and takes a field without a colon as empty", () => {
    expect(parseAll("data:no space\ndata:  two spaces\ndata\n\n")).toEqual([{ event: "message", data: "no space\n two spaces\n" }]);
  });

  test("does not dispatch an event without data, and resets the type after each event", () => {
    expect(parseAll("event: update\n\ndata: x\n\n")).toEqual([{ event: "message", data: "x" }]);
  });

  test("accepts CRLF, CR and LF line ends, also when a CRLF is split between pieces", () => {
    expect(parseAll("event: a\r\ndata: 1\r", "\n\r\nevent: b\rdata: 2\r\r")).toEqual([
      { event: "a", data: "1" },
      { event: "b", data: "2" },
    ]);
  });

  test("assembles an event that arrives in many pieces", () => {
    expect(parseAll("ev", "ent: up", "date\nda", "ta: {}", "\n", "\n")).toEqual([{ event: "update", data: "{}" }]);
  });

  test("ignores a leading byte order mark, and fields it does not use", () => {
    expect(parseAll("﻿id: 7\nretry: 10\ndata: x\n\n")).toEqual([{ event: "message", data: "x" }]);
  });

  test("discards an event that the stream ended before completing", () => {
    expect(parseAll("data: complete\n\ndata: cut off\n")).toEqual([{ event: "message", data: "complete" }]);
  });
});

describe("readEventStream", () => {
  test("decodes UTF-8 split between chunks", async () => {
    const bytes = new TextEncoder().encode("data: こんにちは\n\n");
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(bytes.slice(0, 8));
        controller.enqueue(bytes.slice(8));
        controller.close();
      },
    });

    const events: ServerSentEvent[] = [];
    for await (const event of readEventStream(body)) events.push(event);

    expect(events).toEqual([{ event: "message", data: "こんにちは" }]);
  });
});
