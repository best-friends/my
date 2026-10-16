// SPDX-License-Identifier: AGPL-3.0-only
import { expect, test } from "bun:test";
import { loadCatalog } from "./catalog";
import { createTranslator, fallbackChain } from "./translator";

const catalog = loadCatalog();

test("typed translator formats the same strings as the Go accessors", () => {
  const ja = createTranslator(catalog, "ja-JP");
  const en = createTranslator(catalog, "en-US");
  expect(ja("demo:notifications.unread", { count: 3 })).toBe("未読の通知が3件あります");
  expect(en("demo:notifications.unread", { count: 1 })).toBe("1 unread notification");
  expect(ja("demo:follow.rank", { rank: 2 })).toBe("あなたは2番目のフォロワーです");
  expect(en("demo:follow.rank", { rank: 22 })).toBe("You are the 22nd follower");
  expect(en("demo:profile.followers", { count: 1234.5 })).toBe("1,234.5 followers");
  expect(en("demo:post.boosted_own", { gender: "female", name: "Alice" })).toBe("Alice boosted her own post");
  // ja has no translation: falls back to en.
  expect(ja("demo:profile.moved", { name: "Alice", account: "@alice@example.com" })).toBe(
    "Alice has moved to @alice@example.com.",
  );
});

test("fallback chain", () => {
  expect(fallbackChain("zh-Hant-TW")).toEqual(["zh-Hant-TW", "zh-Hant", "zh", "en"]);
  expect(fallbackChain("en")).toEqual(["en"]);
});
