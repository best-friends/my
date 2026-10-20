// SPDX-License-Identifier: AGPL-3.0-only
// The prompt for a new version of the app (ADR-0028), in the whole app (web-client.md T4). jsdom has no
// Service Workers, so the container is a stand-in whose registration already has a waiting version.
import { screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import { registerServiceWorker } from "./lib/pwa";
import { skipWaitingMessage } from "./lib/sw-protocol";
import { renderApp } from "./test/render";

function fakeContainer() {
  const waiting = { postMessage: vi.fn() };
  const registration = Object.assign(new EventTarget(), { waiting, installing: null, update: vi.fn() });
  const container = Object.assign(new EventTarget(), {
    controller: {},
    register: vi.fn(() => Promise.resolve(registration)),
  });
  return { container: container as unknown as ServiceWorkerContainer, register: container.register, waiting };
}

test("offers a waiting version and applies it only when the user presses Update", async () => {
  const { container, register, waiting } = fakeContainer();
  const reload = vi.fn();
  await registerServiceWorker(container, reload);
  expect(register).toHaveBeenCalledWith("/sw.js", { scope: "/" });

  const { user } = await renderApp({ path: "/local" });
  await screen.findByRole("heading", { level: 1, name: "Local timeline" });
  const region = await screen.findByRole("region", { name: "Messages" });
  expect(region).toHaveTextContent("A new version of the app is available.");
  expect(waiting.postMessage).not.toHaveBeenCalled();

  await user.click(screen.getByRole("button", { name: "Update" }));
  expect(waiting.postMessage).toHaveBeenCalledWith(skipWaitingMessage);
  expect(reload).not.toHaveBeenCalled();

  // The page reloads once the new version controls it.
  container.dispatchEvent(new Event("controllerchange"));
  expect(reload).toHaveBeenCalledOnce();
});
