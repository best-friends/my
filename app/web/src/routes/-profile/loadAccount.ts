// SPDX-License-Identifier: AGPL-3.0-only
// What the profile routes share: the handle in their address and the account it names.
import { notFound } from "@tanstack/react-router";
import { z } from "zod";
import { accountQueryOptions } from "../../features/profile";
import { ApiError } from "../../lib/errors";
import type { RouterContext } from "../../lib/query";

/**
 * The handle after the `@` of a profile address, `username` or `username@domain` (api/openapi.yaml,
 * lookupAccount), as the server's HTML shell routes have it (internal/interface/web, ADR-0028).
 */
export const profileParamsSchema = z.object({ acct: z.string().min(1).max(320) });

/**
 * Loads the account whose handle is `acct`, unless it is cached. An account that the server does not
 * find (unknown, gone, or blocking the viewer) is the route's "not found".
 */
export async function loadAccount({ queryClient, api }: Pick<RouterContext, "queryClient" | "api">, acct: string) {
  try {
    return await queryClient.query({ ...accountQueryOptions(api, acct), staleTime: "static" });
  } catch (error) {
    // eslint-disable-next-line @typescript-eslint/only-throw-error -- TanStack Router's "not found" is a thrown object.
    if (error instanceof ApiError && error.status === 404) throw notFound();
    throw error;
  }
}
