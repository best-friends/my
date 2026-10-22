// SPDX-License-Identifier: AGPL-3.0-only
import { act, render, screen } from "@testing-library/react";
import { useRef, useState } from "react";
import { describe, expect, test } from "vitest";
import { useFocusAfterRemoval } from "./useFocusAfterRemoval";

const all = ["a", "b", "c", "d", "e", "f"];

/** A list of six ids that shows `pageSize` of them from `start`, as a paged table does. */
function Paged({ start, pageSize }: { start: number; pageSize: number }) {
  const [ids, setIds] = useState(all);
  const list = useRef<HTMLUListElement>(null);
  const removed = useFocusAfterRemoval(list, ids);
  return (
    <>
      <h1>Title</h1>
      <ul ref={list}>
        {ids.slice(start, start + pageSize).map((id) => (
          <li key={id} data-row-id={id}>
            <button
              type="button"
              onClick={() => {
                removed(id);
                setIds(ids.filter((other) => other !== id));
              }}
            >
              {id}
            </button>
          </li>
        ))}
      </ul>
    </>
  );
}

describe("useFocusAfterRemoval", () => {
  test("focus goes to the row that took the place of the removed one within the shown page", () => {
    render(<Paged start={3} pageSize={3} />);
    act(() => screen.getByRole("button", { name: "d" }).click());
    // The page now shows e, f, and nothing from the next page: "e" took the place of "d".
    expect(screen.getByRole("button", { name: "e" })).toHaveFocus();
  });

  test("the last row of a page hands focus to the row before it", () => {
    render(<Paged start={3} pageSize={3} />);
    act(() => screen.getByRole("button", { name: "f" }).click());
    expect(screen.getByRole("button", { name: "e" })).toHaveFocus();
  });
});
