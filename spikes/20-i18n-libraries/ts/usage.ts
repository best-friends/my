// SPDX-License-Identifier: AGPL-3.0-only
// Type-level test for the generated accessors: `bunx tsc --noEmit` must pass,
// which also proves every @ts-expect-error line really is an error.
import { loadCatalog } from "./catalog";
import { createTranslator } from "./translator";

const t = createTranslator(loadCatalog(), "ja-JP");

export const ok = [
  t("demo:greeting", { name: "Alice" }),
  t("demo:notifications.unread", { count: 3 }),
  t("demo:post.boosted_own", { gender: "female", name: "Alice" }),
  t("demo:post.boosted_own", { gender: "any-other-value", name: "Sam" }),
  t("demo:profile.moved", { name: "Alice", account: "@alice@example.com" }),
];

export const ng = [
  // @ts-expect-error unknown key
  t("demo:does.not.exist", {}),
  // @ts-expect-error missing argument `name`
  t("demo:greeting", {}),
  // @ts-expect-error `count` must be a number
  t("demo:notifications.unread", { count: "3" }),
  // @ts-expect-error arguments are required
  t("demo:greeting"),
];
