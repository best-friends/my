// SPDX-License-Identifier: AGPL-3.0-only
// An in-memory stand-in for the Web Locks API, which jsdom lacks. Like navigator.locks it grants each
// named lock to one holder at a time, in request order, and releases it when the callback's promise
// settles (MDN, LockManager.request()). Sessions given the same instance act as tabs of one origin.
import type { Locks } from "../lib/auth/session";

export function createLockManager(): Locks {
  const tails = new Map<string, Promise<unknown>>();
  return {
    request: ((name: string, callback: (lock: Lock | null) => unknown) => {
      const previous = tails.get(name) ?? Promise.resolve();
      const run = previous.then(() => callback({ name, mode: "exclusive" }));
      tails.set(
        name,
        run.catch(() => undefined),
      );
      return run;
    }) as Locks["request"],
  };
}
