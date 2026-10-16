// SPDX-License-Identifier: AGPL-3.0-only
import { describe, expect, test } from "vitest";
import { statusFixture, statusPage, viewerStatusFixture } from "../../../test/fixtures";
import { forPost, updateStatuses, withRelation, withRelationFrom } from "./cache";

const post = statusFixture("0192f0c4-7a8e-7000-8000-0000000000c1");
const other = statusFixture("0192f0c4-7a8e-7000-8000-0000000000c2");
const boost = viewerStatusFixture({ id: "0192f0c4-7a8e-7000-8000-0000000000c3", content: "", boost: post });
const favourite = forPost(post.id, (status) => withRelation(status, "favourite", true));

describe("withRelation", () => {
  test("turns a relation on and off with its count, and leaves a post that is already so as it is", () => {
    const on = withRelation({ ...post, favourites_count: 2 }, "favourite", true);
    expect(on).toMatchObject({ favourited: true, favourites_count: 3 });
    expect(withRelation(on, "favourite", true)).toBe(on);
    expect(withRelation(on, "favourite", false)).toMatchObject({ favourited: false, favourites_count: 2 });
    // A bookmark has no public count.
    expect(withRelation(post, "bookmark", true)).toEqual({ ...post, bookmarked: true });
  });

  test("copies only the relation and its count from the server's answer", () => {
    const answer = { ...post, boosted: true, boosts_count: 7, favourited: true };
    expect(withRelationFrom(post, "boost", answer)).toEqual({ ...post, boosted: true, boosts_count: 7 });
  });
});

describe("updateStatuses", () => {
  test("updates a post's own answer", () => {
    expect(updateStatuses(post, favourite)).toMatchObject({ id: post.id, favourited: true });
  });

  test("updates the post and the boosts of it in the pages of a timeline, and keeps the rest", () => {
    const data = { pages: [statusPage([other, boost]), statusPage([post], undefined)], pageParams: [undefined, "page-2"] };
    const next = updateStatuses(data, favourite) as typeof data;
    expect(next.pages[0]?.items[0]).toBe(other);
    expect(next.pages[0]?.items[1]?.boost).toMatchObject({ favourited: true, favourites_count: 1 });
    expect(next.pages[1]?.items[0]).toMatchObject({ favourited: true });
    expect(next.pageParams).toBe(data.pageParams);
  });

  test("updates the ancestors and descendants of a thread", () => {
    const data = { ancestors: [post], descendants: [other] };
    const next = updateStatuses(data, favourite) as typeof data;
    expect(next.ancestors[0]).toMatchObject({ favourited: true });
    expect(next.descendants).toBe(data.descendants);
  });

  test("drops a post and the boosts of it from lists", () => {
    const data = { pages: [statusPage([other, boost, post])], pageParams: [undefined] };
    const next = updateStatuses(
      data,
      forPost(post.id, () => null),
    ) as typeof data;
    expect(next.pages[0]?.items).toEqual([other]);
  });

  test("returns an answer that does not hold the post as it is", () => {
    const page = statusPage([other]);
    expect(updateStatuses(page, favourite)).toBe(page);
    const notifications = { items: [{ id: "n1", type: "follow" }] };
    expect(updateStatuses(notifications, favourite)).toBe(notifications);
    expect(updateStatuses(undefined, favourite)).toBeUndefined();
  });
});
