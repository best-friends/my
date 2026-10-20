// SPDX-License-Identifier: AGPL-3.0-only
import { describe, expect, test } from "vitest";
import { contentLinkPath } from "./contentLinks";

describe("contentLinkPath", () => {
  test.each([
    ["https://localhost:8080/tags/cats", "http://localhost:8080", "/tags/cats"],
    ["https://localhost:8080/@alice", "http://localhost:8080", "/@alice"],
    ["https://my.example/@alice/status/0192f0c4", "https://my.example", "/@alice/status/0192f0c4"],
    ["http://MY.example/@bob@remote.example", "https://my.example", "/@bob@remote.example"],
    ["https://my.example/tags/%E7%8C%AB", "https://my.example", "/tags/%E7%8C%AB"],
  ])("opens %s of %s at %s", (href, apiOrigin, path) => {
    expect(contentLinkPath(href, apiOrigin)).toBe(path);
  });

  test.each([
    ["https://remote.example/tags/cats", "https://my.example"],
    ["https://my.example:8443/tags/cats", "https://my.example"],
    ["https://my.example/media/1/full.jpg", "https://my.example"],
    ["https://my.example/@alice/status/1/extra", "https://my.example"],
    ["https://my.example/@alice/0192f0c4", "https://my.example"],
    ["https://my.example/ap/actors/1", "https://my.example"],
    ["mailto:alice@my.example", "https://my.example"],
    ["not a url", "https://my.example"],
  ])("leaves %s of %s to the browser", (href, apiOrigin) => {
    expect(contentLinkPath(href, apiOrigin)).toBeNull();
  });
});
