// SPDX-License-Identifier: AGPL-3.0-only
// The server's descriptions in each language (FR-705): the short one, a paragraph that the about page and other
// servers' directories show, and the full one. A language without a description shows the one of the server's
// default language. Saving sends only the texts that changed, each an audited change.
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { type SyntheticEvent, useState } from "react";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { useLocale, useT } from "../../../lib/i18n";
import { instanceTextsQueryOptions } from "../api/documents";
import { cacheSavedTexts, descriptionLimit, type InstanceTextChange, setInstanceTextsMutation, shortDescriptionLimit } from "../api/settings";
import { characterCount, editedLocales, languageName } from "./languages";

type DescriptionKey = Extract<InstanceTextChange["key"], "short_description" | "description">;

/** The most characters of each description. */
export const descriptionLimits: Readonly<Record<DescriptionKey, number>> = {
  short_description: shortDescriptionLimit,
  description: descriptionLimit,
};

/** One description in one language: what the field holds and how to change it. */
export type DescriptionField = {
  key: DescriptionKey;
  value: string;
  onChange: (value: string) => void;
};

/** The descriptions of one language. */
type DescriptionLanguage = {
  locale: string;
  /** The language's name in the app's language. */
  language: string;
  short: DescriptionField;
  full: DescriptionField;
};

function draftKey(key: DescriptionKey, locale: string): string {
  return `${key}:${locale}`;
}

export function useServerDescriptions() {
  const api = useApi();
  const queryClient = useQueryClient();
  const t = useT();
  const displayLocale = useLocale();
  const { data } = useSuspenseQuery(instanceTextsQueryOptions(api));
  const texts = data.items.flatMap((text) => (text.key === "short_description" || text.key === "description" ? [{ ...text, key: text.key }] : []));
  const saved = new Map(texts.map((text) => [draftKey(text.key, text.locale), text.body]));
  // The languages of the app, and any other language a description was saved in (from the CLI).
  const locales = editedLocales(texts.map((text) => text.locale));
  const [drafts, setDrafts] = useState<Readonly<Record<string, string>>>({});
  const [status, setStatus] = useState<string | undefined>(undefined);

  const valueOf = (key: DescriptionKey, locale: string) => drafts[draftKey(key, locale)] ?? saved.get(draftKey(key, locale)) ?? "";
  const changes: (InstanceTextChange & { key: DescriptionKey })[] = locales.flatMap((locale) =>
    (["short_description", "description"] as const)
      .filter((key) => valueOf(key, locale) !== (saved.get(draftKey(key, locale)) ?? ""))
      .map((key) => ({ key, locale, body: valueOf(key, locale) })),
  );
  const tooLong = changes.some((change) => characterCount(change.body) > descriptionLimits[change.key]);

  const save = useMutation({
    ...setInstanceTextsMutation(api),
    onMutate: () => {
      setStatus(undefined);
    },
    onSuccess: async () => {
      // The about page shows the new descriptions once GET /instance is fetched again (the button stays pending
      // until then); the audit log has the change when it is opened next (cacheSavedTexts).
      await cacheSavedTexts(queryClient);
      setDrafts({});
      setStatus(t("client:admin.settings.descriptions.saved"));
    },
    // Some texts may have been saved before one failed: the form compares with what is saved now, and keeps the
    // rest of the drafts to send again (web-client.md E2).
    onError: () => cacheSavedTexts(queryClient),
  });

  const field = (key: DescriptionKey, locale: string): DescriptionField => ({
    key,
    value: valueOf(key, locale),
    onChange: (next) => {
      const id = draftKey(key, locale);
      setDrafts({ ...drafts, [id]: next });
    },
  });

  const languages: DescriptionLanguage[] = locales.map((locale) => ({
    locale,
    language: languageName(locale, displayLocale),
    short: field("short_description", locale),
    full: field("description", locale),
  }));

  return {
    languages,
    /** Whether a field holds a description that is not saved. */
    hasChanges: changes.length > 0,
    submit: (event: SyntheticEvent<HTMLFormElement>) => {
      event.preventDefault();
      // A description over its limit shows why under its field (its count); the server would refuse it.
      if (tooLong || save.isPending) return;
      if (changes.length === 0) {
        setStatus(t("client:admin.settings.descriptions.unchanged"));
        return;
      }
      save.mutate(changes);
    },
    isSaving: save.isPending,
    /** Why the last save failed, localized; the server names the invalid property in it. */
    saveError: save.isError ? errorMessage(save.error) : undefined,
    /** What the last save did, for the region that reports it. */
    status,
  };
}
