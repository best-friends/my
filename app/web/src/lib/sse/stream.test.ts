// SPDX-License-Identifier: AGPL-3.0-only
import { afterEach, describe, expect, test, vi } from "vitest";
import { defaultRetryDelay, parseStreamEvent } from "./stream";

describe("parseStreamEvent", () => {
  test("reads the update and notification events of the client API", () => {
    expect(parseStreamEvent("update", '{"id":"s1"}')).toEqual({ type: "update", status: { id: "s1" } });
    expect(parseStreamEvent("notification", '{"id":"n1","type":"follow"}')).toEqual({ type: "notification", notification: { id: "n1", type: "follow" } });
  });

  test("skips events it does not know and data it cannot read", () => {
    expect(parseStreamEvent("message", '{"id":"s1"}')).toBeNull();
    expect(parseStreamEvent("update", "{not json")).toBeNull();
    expect(parseStreamEvent("update", '"a string"')).toBeNull();
    expect(parseStreamEvent("update", '{"no":"id"}')).toBeNull();
  });
});

describe("defaultRetryDelay", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  test("waits about a second after a stream ended and doubles with each failure, up to a minute", () => {
    vi.spyOn(Math, "random").mockReturnValue(1);
    expect([0, 1, 2, 3, 10].map(defaultRetryDelay)).toEqual([1_000, 2_000, 4_000, 8_000, 60_000]);
  });

  test("spreads each wait over its upper half", () => {
    vi.spyOn(Math, "random").mockReturnValue(0);
    expect([0, 1, 10].map(defaultRetryDelay)).toEqual([500, 1_000, 30_000]);
  });
});
