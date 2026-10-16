// SPDX-License-Identifier: AGPL-3.0-only
// Integration tests of the error boundaries (web-client.md E1): the app's layout, error components and
// router options over a route tree of the test's own, with a page whose loader fails.
import { createRootRouteWithContext, createRoute, createRouter, type RouterHistory } from "@tanstack/react-router";
import { act, screen, waitFor } from "@testing-library/react";
import { expect, test } from "vitest";
import type { RouterContext } from "./lib/query";
import { routerOptions } from "./router";
import { AppLayout } from "./routes/-layout/AppLayout";
import { PlaceholderPage } from "./routes/-layout/PlaceholderPage";
import { NotFound, RootError } from "./routes/-layout/RouteStates";
import { renderApp } from "./test/render";

function HomePage() {
  return <PlaceholderPage titleKey="client:home.title" />;
}

function SettingsPage() {
  return <PlaceholderPage titleKey="client:settings.title" />;
}

/** A router whose "/broken" page fails to load `failures` times, then shows the settings placeholder. */
function routerWithBrokenPage(failures: number) {
  let remaining = failures;
  const rootRoute = createRootRouteWithContext<RouterContext>()({
    component: AppLayout,
    errorComponent: RootError,
    notFoundComponent: NotFound,
  });
  const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: "/", component: HomePage });
  const brokenRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/broken",
    loader: () => {
      if (remaining > 0) {
        remaining--;
        throw new Error("the page failed to load");
      }
    },
    component: SettingsPage,
  });
  const routeTree = rootRoute.addChildren([indexRoute, brokenRoute]);
  return ({ history, ...context }: RouterContext & { history: RouterHistory }) => createRouter({ ...routerOptions, routeTree, context, history });
}

test("a page that fails shows the error inside the layout, focused and titled, and recovers on retry", async () => {
  const { user, router } = await renderApp({ createRouter: routerWithBrokenPage(1) });
  await screen.findByRole("heading", { level: 1, name: "Home" });

  await act(() => router.navigate({ href: "/broken" }));

  const heading = await screen.findByRole("heading", { level: 1, name: "Something went wrong" });
  // Focus moves when the router reports the page rendered, after the heading is in the document.
  await waitFor(() => {
    expect(heading).toHaveFocus();
  });
  expect(document.title).toBe("Something went wrong – my example");
  // The layout stays, so the user can go elsewhere.
  expect(screen.getByRole("navigation", { name: "Main" })).toBeInTheDocument();

  await user.click(screen.getByRole("button", { name: "Try again" }));

  expect(await screen.findByRole("heading", { level: 1, name: "Settings" })).toBeInTheDocument();
  expect(screen.queryByRole("heading", { name: "Something went wrong" })).not.toBeInTheDocument();
});

test("a page that fails on the first load shows the error without moving focus", async () => {
  await renderApp({ path: "/broken", createRouter: routerWithBrokenPage(1) });

  const heading = await screen.findByRole("heading", { level: 1, name: "Something went wrong" });
  expect(heading).not.toHaveFocus();
  expect(screen.getByRole("navigation", { name: "Main" })).toBeInTheDocument();
});
