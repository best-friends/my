// SPDX-License-Identifier: AGPL-3.0-only
// The server's rules (FR-705, manage_rules): the rules in effect in order, each in every language it is written
// in; the texts of a new or changed rule in each language of the app; moving a rule up or down among the others;
// and retiring one, which reports keep citing. Every change goes to the audit log.
import { useMutation, useMutationState, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { type SyntheticEvent, useState } from "react";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { useLocale, useT } from "../../../lib/i18n";
import {
  createRuleMutation,
  moveRuleMutation,
  refreshRules,
  retireRuleMutation,
  type Rule,
  ruleHintLimit,
  ruleTextLimit,
  type RuleTextInput,
  rulesQueryOptions,
  updateRuleMutation,
} from "../api/rules";
import { characterCount, editedLocales, languageName } from "./languages";

/** A rule's text for a reader of `locale`: in that locale, else in its language, else the first one. */
export function ruleText(rule: Rule, locale: string): { text: string; locale: string } {
  const language = locale.split("-")[0];
  const text =
    rule.texts.find((item) => item.locale === locale) ?? rule.texts.find((item) => item.locale.split("-")[0] === language) ?? rule.texts[0];
  return { text: text?.text ?? "", locale: text?.locale ?? locale };
}

/** The rules in effect, in order. */
export function useRuleList(): Rule[] {
  const api = useApi();
  return useSuspenseQuery(rulesQueryOptions(api)).data.items;
}

/** The text and hint of a rule in one language, as typed. */
type DraftText = { text: string; hint: string };

/** The fields of a rule in one language. */
export type RuleLanguageFields = {
  locale: string;
  /** The language's name in the app's language. */
  language: string;
  text: string;
  hint: string;
  changeText: (text: string) => void;
  changeHint: (hint: string) => void;
  /** Why the text cannot be saved, localized; undefined when it can. */
  textError: string | undefined;
  /** Why the hint cannot be saved, localized; undefined when it can. */
  hintError: string | undefined;
};

/**
 * The texts of a rule in each language of the app and each language `rule` is written in. A language whose text
 * and hint are both empty is left out of the rule; one with a hint needs a text.
 */
export function useRuleDraft(rule?: Rule) {
  const t = useT();
  const displayLocale = useLocale();
  const initial = Object.fromEntries((rule?.texts ?? []).map((item): [string, DraftText] => [item.locale, { text: item.text, hint: item.hint ?? "" }]));
  const [drafts, setDrafts] = useState<Readonly<Record<string, DraftText>>>(initial);
  const [noText, setNoText] = useState(false);
  const locales = editedLocales(Object.keys(initial));
  const draftOf = (locale: string): DraftText => drafts[locale] ?? { text: "", hint: "" };

  const textErrorOf = ({ text, hint }: DraftText): string | undefined => {
    if (characterCount(text) > ruleTextLimit) return t("client:admin.rules.too_long", { max: String(ruleTextLimit) });
    if (text.trim() === "" && (text !== "" || hint !== "")) return t("client:admin.rules.text_required");
    return undefined;
  };
  const hintErrorOf = ({ hint }: DraftText): string | undefined =>
    characterCount(hint) > ruleHintLimit ? t("client:admin.rules.too_long", { max: String(ruleHintLimit) }) : undefined;

  const fields: RuleLanguageFields[] = locales.map((locale) => {
    const draft = draftOf(locale);
    return {
      locale,
      language: languageName(locale, displayLocale),
      text: draft.text,
      hint: draft.hint,
      changeText: (text) => {
        setNoText(false);
        setDrafts({ ...drafts, [locale]: { ...draft, text } });
      },
      changeHint: (hint) => {
        setDrafts({ ...drafts, [locale]: { ...draft, hint } });
      },
      textError: textErrorOf(draft),
      hintError: hintErrorOf(draft),
    };
  });

  return {
    fields,
    /** Why the rule cannot be saved as a whole: it has no text in any language. */
    error: noText ? t("client:admin.rules.no_text") : undefined,
    /** The texts to send, or null when a field is invalid (it shows why) or no language has a text. */
    texts: (): RuleTextInput[] | null => {
      if (fields.some((field) => field.textError !== undefined || field.hintError !== undefined)) return null;
      const texts = fields.filter((field) => field.text.trim() !== "").map((field) => ({ locale: field.locale, text: field.text.trim(), ...(field.hint.trim() === "" ? {} : { hint: field.hint.trim() }) }));
      setNoText(texts.length === 0);
      return texts.length === 0 ? null : texts;
    },
    /** Empties every field, for the next rule. */
    reset: () => {
      setDrafts({});
      setNoText(false);
    },
  };
}

/** The form of a new rule: it is added after the others, and the form empties once the list shows it. */
export function useNewRuleForm() {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const draft = useRuleDraft();
  const [status, setStatus] = useState<string | undefined>(undefined);
  const create = useMutation({
    ...createRuleMutation(api),
    onMutate: () => {
      setStatus(undefined);
    },
    onSuccess: async () => {
      await refreshRules(queryClient);
      draft.reset();
      setStatus(t("client:admin.rules.added"));
    },
  });
  return {
    draft,
    submit: (event: SyntheticEvent<HTMLFormElement>) => {
      event.preventDefault();
      if (create.isPending) return;
      const texts = draft.texts();
      if (texts !== null) create.mutate(texts);
    },
    isPending: create.isPending,
    /** Why adding failed, localized; the draft stays. */
    error: create.isError ? errorMessage(create.error) : undefined,
    status,
  };
}

/** What the list of rules does to a rule. */
export type RuleActions = {
  /** Replaces the rule's texts; resolves whether that worked (the dialog stays open with the error if not). */
  update: (rule: Rule, texts: RuleTextInput[]) => Promise<boolean>;
  /** Moves the rule to a place among the rules in effect, from 1. */
  move: (rule: Rule, position: number) => void;
  /** Retires the rule; resolves whether that worked. */
  retire: (rule: Rule) => Promise<boolean>;
  /** The IDs of the rules being moved. */
  movingIds: readonly string[];
  /** Why the last change failed, localized. */
  error: string | undefined;
};

/**
 * The changes of the rules in the list. Each says what it did through `onDone(rule, status)` once the list shows
 * it (the rules are fetched again first), so that the screen can move focus and keep the status next to the list.
 */
export function useRuleActions(onDone: (rule: Rule, status: string, change: "updated" | "moved" | "retired") => void): RuleActions {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const displayLocale = useLocale();
  const [error, setError] = useState<string | undefined>(undefined);
  const nameOf = (rule: Rule) => ruleText(rule, displayLocale).text;
  const update = useMutation({ ...updateRuleMutation(api) });
  const retire = useMutation({ ...retireRuleMutation(api) });
  const moveOptions = moveRuleMutation(api);
  const move = useMutation({
    ...moveOptions,
    onMutate: () => {
      setError(undefined);
    },
    onSuccess: async (rule, { position }) => {
      await refreshRules(queryClient);
      onDone(rule, t("client:admin.rules.moved", { rule: nameOf(rule), position: String(position) }), "moved");
    },
    onError: (failure) => {
      setError(errorMessage(failure));
    },
  });
  const movingIds = useMutationState({
    filters: { mutationKey: moveOptions.mutationKey, status: "pending" },
    select: (pending) => (pending.state.variables as { id: string }).id,
  });
  return {
    update: async (rule, texts) => {
      setError(undefined);
      try {
        const saved = await update.mutateAsync({ id: rule.id, texts });
        await refreshRules(queryClient);
        onDone(saved, t("client:admin.rules.updated", { rule: nameOf(saved) }), "updated");
        return true;
      } catch (failure) {
        setError(errorMessage(failure));
        return false;
      }
    },
    move: (rule, position) => {
      move.mutate({ id: rule.id, position });
    },
    retire: async (rule) => {
      setError(undefined);
      try {
        const retired = await retire.mutateAsync(rule.id);
        // The rule leaves the list once it is fetched again: focus moves before that (useFocusAfterRemoval).
        onDone(retired, t("client:admin.rules.retired", { rule: nameOf(retired) }), "retired");
        await refreshRules(queryClient);
        return true;
      } catch (failure) {
        setError(errorMessage(failure));
        return false;
      }
    },
    movingIds,
    error,
  };
}
