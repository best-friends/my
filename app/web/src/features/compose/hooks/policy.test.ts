// SPDX-License-Identifier: AGPL-3.0-only
import { describe, expect, test } from "vitest";
import { statusFixture, viewerFixture } from "../../../test/msw/handlers";
import { fitWithin } from "./prepareImage";
import { countCharacters, defaultVisibility, narrower, replyMentions } from "./policy";

describe("narrower", () => {
  test.each([
    ["public", "local", "local"],
    ["local", "unlisted", "unlisted"],
    ["private", "public", "private"],
    ["direct", "private", "direct"],
    ["unlisted", "unlisted", "unlisted"],
  ] as const)("of %s and %s is %s", (a, b, expected) => {
    expect(narrower(a, b)).toBe(expected);
  });
});

describe("defaultVisibility", () => {
  test("is the preference for a new post", () => {
    expect(defaultVisibility("unlisted", undefined)).toBe("unlisted");
  });

  test("of a reply is the narrower of the preference and the replied-to post's", () => {
    expect(defaultVisibility("public", statusFixture({ visibility: "local" }))).toBe("local");
    expect(defaultVisibility("private", statusFixture({ visibility: "public" }))).toBe("private");
  });
});

describe("replyMentions", () => {
  const bob = { ...viewerFixture, id: "bob-id", username: "bob", acct: "bob" };

  test("mentions the author and the mentioned accounts once, without the viewer", () => {
    const post = statusFixture({
      account: bob,
      mentions: [
        { id: viewerFixture.id, username: "alice", acct: "alice", url: "https://my.example/@alice" },
        { id: "bob-id", username: "bob", acct: "bob", url: "https://my.example/@bob" },
        { id: "carol-id", username: "carol", acct: "carol@remote.example", url: "https://remote.example/@carol" },
      ],
    });
    expect(replyMentions(post, viewerFixture.id)).toBe("@bob @carol@remote.example ");
  });

  test("is empty for a reply to the viewer's own post that mentions nobody else", () => {
    expect(replyMentions(statusFixture(), viewerFixture.id)).toBe("");
  });
});

test("countCharacters counts code points, as the server does", () => {
  expect(countCharacters("👍🏽 é")).toBe(4);
});

describe("fitWithin", () => {
  test("keeps an image that is small enough", () => {
    expect(fitWithin(1920, 1080)).toEqual({ width: 1920, height: 1080 });
  });

  test("scales a larger image down to 3840 × 2160 pixels of area, keeping the aspect ratio", () => {
    const { width, height } = fitWithin(8000, 2000);
    expect(width * height).toBeLessThanOrEqual(3840 * 2160);
    expect(width / height).toBeCloseTo(4, 2);
  });
});
