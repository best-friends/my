// SPDX-License-Identifier: AGPL-3.0-only
import { describe, expect, test } from "vitest";
import { handleOf, isRemoteAcct } from "./handle";

describe("handleOf", () => {
  test("shows a local account without the domain", () => {
    expect(handleOf("alice")).toBe("@alice");
    expect(isRemoteAcct("alice")).toBe(false);
  });

  test("shows a remote account with its domain", () => {
    expect(handleOf("bob@remote.example")).toBe("@bob@remote.example");
    expect(isRemoteAcct("bob@remote.example")).toBe(true);
  });
});
