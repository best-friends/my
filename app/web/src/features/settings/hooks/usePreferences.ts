// SPDX-License-Identifier: AGPL-3.0-only
import { useAnnouncer } from "@my/ui";
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { useT } from "../../../lib/i18n";
import { type Preferences, preferencesQueryOptions, updatePreferencesMutation } from "../api/me";

/** The preferences that the settings page changes, by their API name. */
type PreferenceKey = "locale" | "time_zone" | "default_visibility" | "timeline_languages" | "email_moderator_notifications";

/** One preference as a control shows it: it is saved as soon as it changes. */
export type PreferenceControl<T> = {
  /** The value being saved while `isPending`, the saved one otherwise (web-client.md S8). */
  value: T;
  onChange: (value: T) => void;
  isPending: boolean;
  /** Why the last change was not saved, localized; undefined when it was. */
  error: string | undefined;
};

export type PreferencesControls = {
  locale: PreferenceControl<string>;
  timeZone: PreferenceControl<string>;
  defaultVisibility: PreferenceControl<Preferences["default_visibility"]>;
  timelineLanguages: PreferenceControl<string[]>;
  emailModeratorNotifications: PreferenceControl<boolean>;
};

/**
 * Returns a control for each preference of the settings page. Each saves itself at once (FR-007): there is
 * no form to submit, and a change of the locale displays the app in it as soon as it is saved
 * (useUserDisplaySettings reads the cached preferences).
 */
export function usePreferences(): PreferencesControls {
  const api = useApi();
  const { data } = useSuspenseQuery(preferencesQueryOptions(api));
  return {
    locale: usePreferenceControl("locale", data.locale),
    timeZone: usePreferenceControl("time_zone", data.time_zone),
    defaultVisibility: usePreferenceControl("default_visibility", data.default_visibility),
    timelineLanguages: usePreferenceControl("timeline_languages", data.timeline_languages),
    emailModeratorNotifications: usePreferenceControl("email_moderator_notifications", data.email_moderator_notifications),
  };
}

function usePreferenceControl<K extends PreferenceKey>(key: K, saved: Preferences[K]): PreferenceControl<Preferences[K]> {
  const api = useApi();
  const queryClient = useQueryClient();
  const announce = useAnnouncer();
  const t = useT();
  const mutation = useMutation({
    ...updatePreferencesMutation(api),
    onSuccess: (preferences) => {
      queryClient.setQueryData(preferencesQueryOptions(api).queryKey, preferences);
      announce(t("client:settings.saved"));
    },
  });
  const pending = mutation.isPending ? mutation.variables[key] : undefined;
  return {
    value: (pending ?? saved) as Preferences[K],
    onChange: (value) => {
      mutation.mutate({ [key]: value });
    },
    isPending: mutation.isPending,
    error: mutation.isError ? errorMessage(mutation.error) : undefined,
  };
}
