// SPDX-License-Identifier: AGPL-3.0-only
// The server's policies (FR-001, FR-202, FR-408, FR-409, FR-718, ADR-0009): who may sign up, the default
// language, the public timelines, the federation, and who may read the list of the domain actions. They are
// instance settings that used to change only from the CLI (`my instance set`); the form sends the ones that
// changed together, all or none, and each change goes to the audit log (FR-704).
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { type SyntheticEvent, useState } from "react";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { useLocale, useT } from "../../../lib/i18n";
import { cacheSavedSettings, type InstanceSettingKey, instanceSettingsQueryOptions, updateInstanceSettingsMutation } from "../api/settings";
import { languageName, selectableLocales } from "./languages";

/** The settings of the form, in the order of its sections. */
const policyKeys = [
  "registration_mode",
  "default_locale",
  "local_timeline_enabled",
  "federated_timeline_enabled",
  "authorized_fetch",
  "remote_media_policy",
  "remote_media_cache_ttl",
  "domain_actions_visibility",
  "domain_actions_rationale_visibility",
] as const satisfies readonly InstanceSettingKey[];

export type PolicyKey = (typeof policyKeys)[number];

/** The lengths of time offered for how long remote media may be kept, in seconds: an hour to a year. */
const cacheTtlChoices: readonly number[] = [3600, 86_400, 7 * 86_400, 30 * 86_400, 365 * 86_400];

/** A choice of a setting: the value the server stores and its words. */
export type PolicyChoice = { value: string; label: string };

export function useServerPolicies() {
  const api = useApi();
  const queryClient = useQueryClient();
  const t = useT();
  const displayLocale = useLocale();
  const { data } = useSuspenseQuery(instanceSettingsQueryOptions(api));
  const saved = new Map(data.items.map((setting) => [setting.key, setting]));
  const [drafts, setDrafts] = useState<Readonly<Partial<Record<PolicyKey, string>>>>({});
  const [status, setStatus] = useState<string | undefined>(undefined);

  const savedValue = (key: PolicyKey) => saved.get(key)?.value ?? "";
  const valueOf = (key: PolicyKey) => drafts[key] ?? savedValue(key);
  const changes = Object.fromEntries(policyKeys.filter((key) => valueOf(key) !== savedValue(key)).map((key) => [key, valueOf(key)]));

  const save = useMutation({
    ...updateInstanceSettingsMutation(api),
    onMutate: () => {
      setStatus(undefined);
    },
    onSuccess: async (settings) => {
      // The navigation offers the timelines and the sign-up page says who may sign up from GET /instance, which is
      // fetched again before the form says it is done (the button stays pending until then).
      await cacheSavedSettings(queryClient, api, settings);
      setDrafts({});
      setStatus(t("client:admin.settings.policies.saved"));
    },
  });

  const durationLabel = (seconds: number): string => {
    if (seconds % 86_400 === 0) return t("client:admin.settings.duration.days", { count: seconds / 86_400 });
    if (seconds % 3600 === 0) return t("client:admin.settings.duration.hours", { count: seconds / 3600 });
    return t("client:admin.settings.duration.seconds", { count: seconds });
  };
  const currentTtl = Number(savedValue("remote_media_cache_ttl"));
  const ttlChoices = [...new Set([...cacheTtlChoices, ...(Number.isInteger(currentTtl) && currentTtl > 0 ? [currentTtl] : [])])].toSorted((a, b) => a - b);
  const localeChoices = [...new Set([...selectableLocales(), savedValue("default_locale")])].filter((locale) => locale !== "");

  return {
    /** The value of a setting in the form: the draft, else what is saved. */
    value: valueOf,
    /** Changes a setting in the form; nothing is sent until the form is saved. */
    change: (key: PolicyKey, value: string) => {
      setDrafts({ ...drafts, [key]: value });
    },
    /** The languages people may pick as the default one, named in the app's language. */
    localeChoices: localeChoices.map((locale): PolicyChoice => ({ value: locale, label: languageName(locale, displayLocale) })),
    /** The lengths of time remote media may be kept, with the saved one when it is not among them. */
    cacheTtlChoices: ttlChoices.map((seconds): PolicyChoice => ({ value: String(seconds), label: durationLabel(seconds) })),
    submit: (event: SyntheticEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (save.isPending) return;
      if (Object.keys(changes).length === 0) {
        setStatus(t("client:admin.settings.policies.unchanged"));
        return;
      }
      save.mutate(changes);
    },
    isSaving: save.isPending,
    /** Why the last save failed, localized; nothing was changed and the form keeps the drafts. */
    saveError: save.isError ? errorMessage(save.error) : undefined,
    /** What the last save did, for the region that reports it. */
    status,
  };
}
