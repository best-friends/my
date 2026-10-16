// SPDX-License-Identifier: AGPL-3.0-only
// The app's long-lived objects, made once per page: the session (the in-memory access token), the API
// client on it, and the QueryClient, which forgets the user's data when the user changes. main.tsx makes
// them for the page and the tests make them for each render.
import type { QueryClient } from "@tanstack/react-query";
import { type Api, createApi } from "./api/client";
import { Session } from "./auth/session";
import type { LeaveApp } from "./auth/SessionProvider";
import { forgetUserDataOnUserChange } from "./auth/userData";
import type { RuntimeConfig } from "./config";
import { createTranslate, type I18n } from "./i18n";
import { createQueryClient } from "./query";

export type AppServices = { queryClient: QueryClient; api: Api; session: Session; leaveApp: LeaveApp };

/** Loads `url` in place of the app. */
function assignLocation(url: URL): void {
  window.location.assign(url);
}

/** Makes the services; tests pass their own `leaveApp`, as jsdom cannot load another document. */
export function createAppServices(config: RuntimeConfig, i18n: I18n, leaveApp: LeaveApp = assignLocation): AppServices {
  const queryClient = createQueryClient();
  const session = new Session({ apiOrigin: config.apiOrigin });
  forgetUserDataOnUserChange(session, queryClient);
  const api = createApi({ apiOrigin: config.apiOrigin, session, locale: i18n.locale, t: createTranslate(i18n) });
  return { queryClient, api, session, leaveApp };
}
