// SPDX-License-Identifier: AGPL-3.0-only
// The root route: the app layout around every page, and the error screen for a failure of the layout
// itself. Pages fail inside the layout instead, through the router's defaultErrorComponent (router.ts).
import { createRootRouteWithContext } from "@tanstack/react-router";
import type { RouterContext } from "../lib/query";
import { AppLayout } from "./-layout/AppLayout";
import { NotFound, RootError } from "./-layout/RouteStates";

export const Route = createRootRouteWithContext<RouterContext>()({
  component: AppLayout,
  errorComponent: RootError,
  notFoundComponent: NotFound,
});
