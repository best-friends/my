// SPDX-License-Identifier: AGPL-3.0-only
import { createFileRoute, redirect } from "@tanstack/react-router";

/** The root address opens the home timeline; the timelines are named by their own paths (web-client.md S3). */
export const Route = createFileRoute("/")({
  beforeLoad: () => {
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- TanStack Router's redirects are thrown objects.
    throw redirect({ to: "/home", replace: true });
  },
});
