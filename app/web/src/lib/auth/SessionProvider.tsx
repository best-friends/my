// SPDX-License-Identifier: AGPL-3.0-only
// The session and the API client for components (web-client/README.md S6: values that rarely change). Server
// data is not in this context: the viewer's account is a query (features/session).
import { createContext, type ReactNode, use, useSyncExternalStore } from "react";
import type { Api } from "../api/client";
import type { Session, SessionStatus } from "./session";

/** Leaves the app for `url` with a full page load (the server's authorization screen). */
export type LeaveApp = (url: URL) => void;

type SessionContextValue = { session: Session; api: Api; leaveApp: LeaveApp };

const SessionContext = createContext<SessionContextValue | null>(null);

export type SessionProviderProps = SessionContextValue & { children: ReactNode };

export function SessionProvider({ session, api, leaveApp, children }: SessionProviderProps) {
  return <SessionContext value={{ session, api, leaveApp }}>{children}</SessionContext>;
}

function useSessionContext(): SessionContextValue {
  const value = use(SessionContext);
  if (value === null) throw new Error("the session hooks need a <SessionProvider> above them");
  return value;
}

/** Returns the session, for its actions (sign-out). Read the status with useSessionStatus. */
export function useSession(): Session {
  return useSessionContext().session;
}

/** Returns the session's status, and renders again when it changes. */
export function useSessionStatus(): SessionStatus {
  const { session } = useSessionContext();
  return useSyncExternalStore(session.subscribe, () => session.status);
}

/** Returns whether resuming the session failed because the server could not be reached (Session.unreachable). */
export function useSessionUnreachable(): boolean {
  const { session } = useSessionContext();
  return useSyncExternalStore(session.subscribe, () => session.unreachable);
}

/** Returns the API client. */
export function useApi(): Api {
  return useSessionContext().api;
}

/** Returns the function that leaves the app for a URL outside it. */
export function useLeaveApp(): LeaveApp {
  return useSessionContext().leaveApp;
}
