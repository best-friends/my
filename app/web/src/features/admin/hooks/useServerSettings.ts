// SPDX-License-Identifier: AGPL-3.0-only
// The server settings screen: the instance's display name in each language of the app, which names the
// instance to people in place of its domain (the navigation, the tab title, the sign-in pages, emails, link
// previews). A language without a name shows the name of the instance's default language, and with no name at
// all the domain; handles and links keep the domain.
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { type SyntheticEvent, useState } from "react";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { supportedLocales, useLocale, useT } from "../../../lib/i18n";
import { instanceQueryOptions, instanceTextsQueryOptions } from "../api/documents";
import { cacheSavedName, type InstanceNameChange, instanceNameLimit, setInstanceNamesMutation } from "../api/settings";

/** Pseudo-locales are for testing; nobody reads the instance's name in them. */
const pseudoLocales: ReadonlySet<string> = new Set(["en-XA", "ar-XB"]);

/** The number of characters of `text` as the server counts them: Unicode code points. */
function characterCount(text: string): number {
  return Array.from(text).length;
}

/** `tag`'s name in `inLocale`, or the tag itself when the browser does not know it. */
function languageName(tag: string, inLocale: string): string {
  try {
    return new Intl.DisplayNames([inLocale], { type: "language", fallback: "code" }).of(tag) ?? tag;
  } catch {
    // Not a valid tag for Intl; the tag is its own name.
    return tag;
  }
}

/** The name in one language: what is saved, what the field holds, and why that cannot be saved. */
type NameField = {
  locale: string;
  /** The language's name in the app's language. */
  language: string;
  value: string;
  onChange: (value: string) => void;
  /** Why the field cannot be saved, localized; undefined when it can. */
  error: string | undefined;
};

export function useServerSettings() {
  const api = useApi();
  const queryClient = useQueryClient();
  const t = useT();
  const displayLocale = useLocale();
  const { data } = useSuspenseQuery(instanceTextsQueryOptions(api));
  const { data: instance } = useSuspenseQuery(instanceQueryOptions(api));
  const saved = new Map(data.items.filter((text) => text.key === "name").map((text) => [text.locale, text.body]));
  // The languages of the app, and any other language a name was saved in (from the CLI).
  const locales = [...new Set([...supportedLocales.filter((locale) => !pseudoLocales.has(locale)), ...saved.keys()])].toSorted();
  const [drafts, setDrafts] = useState<Readonly<Record<string, string>>>({});
  const [status, setStatus] = useState<string | undefined>(undefined);

  const valueOf = (locale: string) => drafts[locale] ?? saved.get(locale) ?? "";
  const changes: InstanceNameChange[] = locales.filter((locale) => valueOf(locale) !== (saved.get(locale) ?? "")).map((locale) => ({ locale, body: valueOf(locale) }));
  const errorOf = (value: string): string | undefined => {
    if (characterCount(value) > instanceNameLimit) return t("client:admin.settings.name.too_long", { max: String(instanceNameLimit) });
    if (value !== "" && value.trim() === "") return t("client:admin.settings.name.blank");
    return undefined;
  };

  const save = useMutation({
    ...setInstanceNamesMutation(api),
    onMutate: () => {
      setStatus(undefined);
    },
    onSuccess: async () => {
      // The navigation and the tab title show the new name once GET /instance is fetched again (the button
      // stays pending until then); the audit log has the change when it is opened next (cacheSavedName).
      await cacheSavedName(queryClient);
      setDrafts({});
      setStatus(t("client:admin.settings.name.saved"));
    },
    // Some languages may have been saved before one failed: the form compares with what is saved now, and keeps
    // the rest of the drafts to send again (web-client.md E2).
    onError: () => cacheSavedName(queryClient),
  });

  const fields: NameField[] = locales.map((locale) => {
    const value = valueOf(locale);
    return {
      locale,
      language: languageName(locale, displayLocale),
      value,
      onChange: (next: string) => {
        setDrafts({ ...drafts, [locale]: next });
      },
      error: errorOf(value),
    };
  });

  return {
    /** The domain that names the instance where no language has a name, and that handles and links use. */
    domain: instance.domain,
    fields,
    /** Whether a field holds a name that is not saved. */
    hasChanges: changes.length > 0,
    submit: (event: SyntheticEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (fields.some((field) => field.error !== undefined)) return;
      if (changes.length === 0) {
        setStatus(t("client:admin.settings.name.unchanged"));
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
