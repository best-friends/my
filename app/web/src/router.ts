// SPDX-License-Identifier: AGPL-3.0-only
// The router (ADR-0025): file-based routes from src/routes, compiled into routeTree.gen.ts by the
// TanStack Router Vite plugin. Navigations commit inside React's startTransition (the router's
// Transitioner), so the current page stays on screen while the next one loads (web-client.md E4); the
// pending component appears only when a load takes longer than defaultPendingMs.
import { createRouter, type RouterHistory } from "@tanstack/react-router";
import type { RouterContext } from "./lib/query";
import { routeTree } from "./routeTree.gen";
import { RouteError, RoutePending } from "./routes/-layout/RouteStates";

/** The options of the app's router other than its routes, history and context. */
export const routerOptions = {
  // Every page that fails renders this inside the layout (web-client.md E1); the root route has its own
  // error component for a failure of the layout (routes/__root.tsx).
  defaultErrorComponent: RouteError,
  defaultPendingComponent: RoutePending,
  // Start loading a page's code and data when the pointer rests on or touches a link to it.
  defaultPreload: "intent",
  // TanStack Query caches the data (web-client.md S1), so the router keeps no preloaded data of its own.
  defaultPreloadStaleTime: 0,
  scrollRestoration: true,
  // A remote account's profile is /@alice@example.com, as the server's shell routes have it (ADR-0028);
  // the `@` of a path param stays as it is instead of becoming %40.
  pathParamsAllowedCharacters: ["@"],
} as const;

export type AppRouterOptions = RouterContext & {
  /** The browser's history by default; tests pass a memory history. */
  history?: RouterHistory;
};

export function createAppRouter({ queryClient, api, session, history }: AppRouterOptions) {
  return createRouter({
    ...routerOptions,
    routeTree,
    context: { queryClient, api, session },
    ...(history === undefined ? {} : { history }),
  });
}

declare module "@tanstack/react-router" {
  interface Register {
    router: ReturnType<typeof createAppRouter>;
  }
}
