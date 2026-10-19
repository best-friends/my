// SPDX-License-Identifier: AGPL-3.0-only
import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { useApi, useSessionStatus } from "../../../lib/auth/SessionProvider";
import { shellQueryRetry } from "../../../lib/query";
import { useAppLocale } from "../../../lib/locale";
import { preferencesQueryOptions } from "../api/me";

/**
 * Displays the app in the signed-in user's locale and time zone (FR-007): at start-up, and at once when the
 * preferences change, in this tab or after a refetch. The page starts in the instance's locale, and keeps the
 * last one it showed when nobody is signed in.
 */
export function useUserDisplaySettings(): void {
  const api = useApi();
  const status = useSessionStatus();
  const appLocale = useAppLocale();
  const { data } = useQuery({ ...preferencesQueryOptions(api), enabled: status === "signed-in", retry: shellQueryRetry });
  const locale = data?.locale;
  const timeZone = data?.time_zone;

  // The locale store is outside React (lib/locale): loading a catalog and setting <html lang dir>.
  useEffect(() => {
    if (locale !== undefined) void appLocale.setLocale(locale);
  }, [appLocale, locale]);
  useEffect(() => {
    if (timeZone !== undefined) appLocale.setTimeZone(timeZone);
  }, [appLocale, timeZone]);
}
