// SPDX-License-Identifier: AGPL-3.0-only
// The terms and policies screen (#489): the terms of service and the privacy policy in one language at a time,
// each written in Markdown and saved on its own (FR-705). The server renders and sanitizes them on their public
// pages, keeps every revision, and records each change in the audit log.
import { useMutation, useQuery, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { type SyntheticEvent, useState } from "react";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { resolveLocale, supportedLocales, useLocale, useT } from "../../../lib/i18n";
import { shellQueryReader } from "../../../lib/query";
import {
  type DocumentKey,
  instanceQueryOptions,
  type InstanceText,
  instanceTextsQueryOptions,
  cacheSavedDocument,
  setDocumentMutation,
} from "../api/documents";

/** Pseudo-locales are for testing; nobody writes a document in them. */
const pseudoLocales: ReadonlySet<string> = new Set(["en-XA", "ar-XB"]);

/** A language to write the documents in: its tag and its name in the display language. */
type LanguageChoice = { id: string; name: string };

/** `tag`'s name in `inLocale`, or the tag itself when the browser does not know it. */
function languageName(tag: string, inLocale: string): string {
  try {
    return new Intl.DisplayNames([inLocale], { type: "language", fallback: "code" }).of(tag) ?? tag;
  } catch {
    // Not a valid tag for Intl; the tag is its own name.
    return tag;
  }
}

/**
 * The languages to choose from: those the app has a catalog for (the server reads a text in the reader's catalog
 * language, then the default language, then English), those a document is already written in, and `requested`
 * (a language from the address), sorted by tag.
 */
function languageChoices(texts: readonly InstanceText[], requested: string | undefined, displayLocale: string): LanguageChoice[] {
  const tags = new Set(supportedLocales.filter((locale) => !pseudoLocales.has(locale)));
  for (const text of texts) tags.add(text.locale);
  if (requested !== undefined) tags.add(requested);
  return [...tags].toSorted().map((id) => ({ id, name: languageName(id, displayLocale) }));
}

/** The key of a draft: a document in a language. */
function draftKey(key: DocumentKey, locale: string): string {
  return `${key} ${locale}`;
}

/** The drafts that differ from what is saved, by draftKey. */
type Drafts = Readonly<Record<string, string>>;

/** One document's form in the selected language. */
export type DocumentEditor = {
  key: DocumentKey;
  /** The text in the field: the draft, or else what is saved. */
  value: string;
  change: (value: string) => void;
  /** The names of the languages the document is written in; none hides its page and links. */
  languages: readonly string[];
  /** When the text in the selected language was saved; undefined when it has none. */
  savedAt: string | undefined;
  /** The public page of the document as the viewer reads it; undefined when the viewer finds none. */
  pageUrl: string | undefined;
  /** Saves the text once the browser has checked the field; removing a saved text asks first. */
  submit: (event: SyntheticEvent<HTMLFormElement>) => void;
  isPending: boolean;
  /** The question before removing the text, or null while it is not asked. */
  confirmation: { confirm: () => void; cancel: () => void } | null;
  /** Why saving failed, localized: in the question while it is open, else under the form. */
  error: string | undefined;
  /** What the last save did, for the form's StatusMessage. */
  status: string | undefined;
};

export type DocumentSettings = {
  /** The selected language and its name. */
  locale: string;
  languageName: string;
  languages: readonly LanguageChoice[];
  terms: DocumentEditor;
  privacyPolicy: DocumentEditor;
  /** Whether a document has a draft that is not saved, in any language. */
  hasChanges: boolean;
};

type EditorContext = {
  locale: string;
  displayLocale: string;
  texts: readonly InstanceText[];
  drafts: Drafts;
  setDraft: (key: DocumentKey, locale: string, value: string | undefined) => void;
};

/** One document's form; the screen has two, so this is called twice (never in a loop, web-client.md R3). */
function useDocumentEditor(key: DocumentKey, { locale, displayLocale, texts, drafts, setDraft }: EditorContext, pageUrl: string | undefined): DocumentEditor {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const [confirming, setConfirming] = useState(false);
  const [status, setStatus] = useState<string | undefined>(undefined);
  const saved = texts.find((text) => text.key === key && text.locale === locale);
  const savedBody = saved?.body ?? "";
  const value = drafts[draftKey(key, locale)] ?? savedBody;
  const save = useMutation({
    ...setDocumentMutation(api),
    onSuccess: async (text) => {
      // The texts, the links to the pages, and the audit log change: the text is saved once those shown now are
      // fetched again (the form stays pending until then); the others load again when opened (cacheSavedDocument).
      await cacheSavedDocument(queryClient);
      setConfirming(false);
      setDraft(key, text.locale, undefined);
      const language = languageName(text.locale, displayLocale);
      setStatus(text.body === "" ? t("client:admin.documents.removed", { language }) : t("client:admin.documents.saved", { language }));
    },
  });
  const run = () => {
    save.mutate({ key, locale, body: value });
  };
  return {
    key,
    value,
    change: (next) => {
      setDraft(key, locale, next === savedBody ? undefined : next);
    },
    languages: texts.filter((text) => text.key === key).map((text) => languageName(text.locale, displayLocale)),
    savedAt: saved?.revised_at,
    pageUrl,
    submit: (event) => {
      event.preventDefault();
      if (save.isPending) return;
      save.reset();
      if (value === "" && saved !== undefined) setConfirming(true);
      else run();
    },
    isPending: save.isPending,
    confirmation: confirming
      ? {
          confirm: run,
          cancel: () => {
            setConfirming(false);
            save.reset();
          },
        }
      : null,
    error: save.isError ? errorMessage(save.error) : undefined,
    status,
  };
}

/**
 * The documents in `requestedLocale`, or else in the display language (the closest one with a catalog). Drafts
 * are kept per document and language, so choosing another language keeps them; leaving the screen with any asks
 * first (useUnsavedChangesGuard).
 */
export function useDocumentSettings(requestedLocale: string | undefined): DocumentSettings {
  const api = useApi();
  const displayLocale = useLocale();
  const texts = useSuspenseQuery(instanceTextsQueryOptions(api)).data.items;
  const instance = useQuery({ ...instanceQueryOptions(api), ...shellQueryReader }).data;
  const [drafts, setDrafts] = useState<Drafts>({});
  const languages = languageChoices(texts, requestedLocale, displayLocale);
  const fallback = resolveLocale(displayLocale);
  const locale = requestedLocale ?? (languages.some((language) => language.id === fallback) ? fallback : "en");
  const context: EditorContext = {
    locale,
    displayLocale,
    texts,
    drafts,
    setDraft: (key, draftLocale, value) => {
      const id = draftKey(key, draftLocale);
      setDrafts((current) => {
        const others = Object.fromEntries(Object.entries(current).filter(([other]) => other !== id));
        return value === undefined ? others : { ...others, [id]: value };
      });
    },
  };
  return {
    locale,
    languageName: languageName(locale, displayLocale),
    languages,
    terms: useDocumentEditor("terms", context, instance?.terms_url),
    privacyPolicy: useDocumentEditor("privacy_policy", context, instance?.privacy_policy_url),
    hasChanges: Object.keys(drafts).length > 0,
  };
}
