// SPDX-License-Identifier: AGPL-3.0-only
// The rules screen below its heading (manage_rules, FR-705): the form of a new rule, then the rules in effect in
// order. A rule has a text (one line) and an optional hint in each language; the about page and the report form
// show it in the reader's language. "Move up" and "Move down" reorder the rules with the keyboard as well as the
// pointer (no dragging); focus then moves to the moved rule's heading. Editing opens a dialog with the texts of
// every language; retiring cannot be undone, so it asks first (an alert dialog). What a change did stays on the
// page in a StatusMessage above the list; a failure stays there too (role="alert", web-client.md E2).
import { Button, ConfirmDialog, DescriptionItem, DescriptionList, Dialog, DialogTrigger, EmptyState, StatusMessage, TextArea, TextField } from "@my/ui";
import { useRef, useState } from "react";
import { useLocale, useT } from "../../../lib/i18n";
import { useFocusAfterRemoval, useFocusHeadingAfterMove } from "../../../lib/useFocusAfterRemoval";
import { DetailSection, RequirePermission } from "../../moderation";
import { type Rule, ruleHintLimit, ruleTextLimit } from "../api/rules";
import { languageName } from "../hooks/languages";
import { type RuleActions, type RuleLanguageFields, ruleText, useNewRuleForm, useRuleActions, useRuleDraft, useRuleList } from "../hooks/useRules";

export function RuleSettings() {
  const t = useT();
  return (
    <RequirePermission permission="manage_rules">
      <div className="flex flex-col gap-8">
        <p className="text-fg-muted">{t("client:admin.rules.description")}</p>
        <DetailSection title={t("client:admin.rules.new.title")}>
          <NewRuleForm />
        </DetailSection>
        <DetailSection title={t("client:admin.rules.list.title")}>
          <RuleList />
        </DetailSection>
      </div>
    </RequirePermission>
  );
}

/** The text and hint fields of a rule in each language. */
function RuleTextFields({ fields, autoFocus = false }: { fields: readonly RuleLanguageFields[]; autoFocus?: boolean }) {
  const t = useT();
  return (
    <>
      {fields.map((field, index) => (
        <div key={field.locale} className="flex flex-col gap-3">
          <TextField
            label={t("client:admin.rules.text", { language: field.language })}
            description={t("client:admin.rules.text.description", { max: String(ruleTextLimit) })}
            value={field.text}
            onChange={field.changeText}
            autoComplete="off"
            autoFocus={autoFocus && index === 0}
            {...(field.textError === undefined ? {} : { isInvalid: true, errorMessage: field.textError })}
          />
          <TextArea
            label={t("client:admin.rules.hint", { language: field.language })}
            value={field.hint}
            onChange={field.changeHint}
            rows={2}
            characterLimit={ruleHintLimit}
            formatRemaining={(remaining) =>
              remaining < 0 ? t("client:admin.settings.characters_over", { count: -remaining }) : t("client:admin.settings.characters_left", { count: remaining })
            }
            {...(field.hintError === undefined ? {} : { isInvalid: true, errorMessage: field.hintError })}
          />
        </div>
      ))}
    </>
  );
}

function NewRuleForm() {
  const t = useT();
  const form = useNewRuleForm();
  return (
    <form onSubmit={form.submit} className="flex flex-col gap-6">
      <RuleTextFields fields={form.draft.fields} />
      {form.draft.error !== undefined && (
        <p role="alert" className="text-danger">
          {form.draft.error}
        </p>
      )}
      {form.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.rules.add_failed", { reason: form.error })}
        </p>
      )}
      <div>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:admin.rules.add")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
    </form>
  );
}

function RuleList() {
  const t = useT();
  const rules = useRuleList();
  const listRef = useRef<HTMLOListElement>(null);
  const [status, setStatus] = useState<string | undefined>(undefined);
  const removed = useFocusAfterRemoval(
    listRef,
    rules.map((rule) => rule.id),
  );
  const focusHeading = useFocusHeadingAfterMove(listRef);
  const actions = useRuleActions((rule, done, change) => {
    if (change === "retired") removed(rule.id);
    else if (change === "moved") focusHeading(rule.id);
    setStatus(done);
  });
  return (
    <div className="flex flex-col gap-3">
      <StatusMessage>{status}</StatusMessage>
      {actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.rules.change_failed", { reason: actions.error })}
        </p>
      )}
      {rules.length === 0 ? (
        <EmptyState headingLevel={3} heading={t("client:admin.rules.empty")} description={t("client:admin.rules.empty.description")} />
      ) : (
        <ol ref={listRef} className="flex flex-col divide-y divide-border">
          {rules.map((rule, index) => (
            <li key={rule.id} data-row-id={rule.id} className="@container py-3">
              <RuleRow rule={rule} place={index + 1} count={rules.length} actions={actions} />
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}

function RuleRow({ rule, place, count, actions }: { rule: Rule; place: number; count: number; actions: RuleActions }) {
  const t = useT();
  const displayLocale = useLocale();
  const name = ruleText(rule, displayLocale).text;
  const isMoving = actions.movingIds.includes(rule.id);
  return (
    <div className="flex flex-col gap-3">
      <h3 tabIndex={-1} className="font-semibold wrap-anywhere outline-none">
        {/* The rule is written by an administrator in any language: U+2068 FIRST STRONG ISOLATE … U+2069 POP DIRECTIONAL ISOLATE. */}
        {t("client:admin.rules.heading", { place: String(place), rule: `\u2068${name}\u2069` })}
      </h3>
      <DescriptionList>
        {rule.texts.map((text) => (
          <DescriptionItem key={text.locale} term={languageName(text.locale, displayLocale)}>
            {/* Written by an administrator, in the language of its locale. */}
            <span dir="auto" lang={text.locale} className="block wrap-anywhere">
              {text.text}
            </span>
            {text.hint !== undefined && (
              <span dir="auto" lang={text.locale} className="block text-fg-muted wrap-anywhere whitespace-pre-wrap">
                {text.hint}
              </span>
            )}
          </DescriptionItem>
        ))}
      </DescriptionList>
      <div className="flex flex-wrap gap-2">
        {/* The first rule cannot move up nor the last down: those buttons stay out rather than doing nothing. */}
        {place > 1 && (
          <Button
            size="sm"
            isPending={isMoving}
            aria-label={t("client:admin.rules.move_up_rule", { rule: name })}
            onPress={() => {
              actions.move(rule, place - 1);
            }}
          >
            {t("client:admin.rules.move_up")}
          </Button>
        )}
        {place < count && (
          <Button
            size="sm"
            isPending={isMoving}
            aria-label={t("client:admin.rules.move_down_rule", { rule: name })}
            onPress={() => {
              actions.move(rule, place + 1);
            }}
          >
            {t("client:admin.rules.move_down")}
          </Button>
        )}
        <EditRule rule={rule} name={name} actions={actions} />
        <RetireRule rule={rule} name={name} actions={actions} />
      </div>
    </div>
  );
}

type RuleActionProps = { rule: Rule; name: string; actions: RuleActions };

/** The texts of a rule in a dialog; it closes once they are saved, and keeps a failure inside. */
function EditRule({ rule, name, actions }: RuleActionProps) {
  const t = useT();
  return (
    <DialogTrigger>
      <Button size="sm" aria-label={t("client:admin.rules.edit_rule", { rule: name })}>
        {t("client:admin.rules.edit")}
      </Button>
      <Dialog title={t("client:admin.rules.edit.title")} closeLabel={t("client:admin.rules.edit.close")} size="md">
        {({ close }) => <EditRuleForm rule={rule} actions={actions} close={close} />}
      </Dialog>
    </DialogTrigger>
  );
}

function EditRuleForm({ rule, actions, close }: { rule: Rule; actions: RuleActions; close: () => void }) {
  const t = useT();
  const draft = useRuleDraft(rule);
  const [isSaving, setIsSaving] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        const texts = draft.texts();
        if (texts === null || isSaving) return;
        setIsSaving(true);
        setFailed(false);
        void actions.update(rule, texts).then((saved) => {
          setIsSaving(false);
          if (saved) close();
          else setFailed(true);
        });
      }}
      className="flex flex-col gap-4"
    >
      <RuleTextFields fields={draft.fields} autoFocus />
      {draft.error !== undefined && (
        <p role="alert" className="text-danger">
          {draft.error}
        </p>
      )}
      {failed && actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.rules.change_failed", { reason: actions.error })}
        </p>
      )}
      <div className="flex flex-wrap justify-end gap-2">
        <Button onPress={close}>{t("client:admin.rules.cancel")}</Button>
        <Button type="submit" variant="primary" isPending={isSaving}>
          {t("client:admin.rules.save")}
        </Button>
      </div>
    </form>
  );
}

/** Retiring a rule, after an alert dialog: it cannot be undone. */
function RetireRule({ rule, name, actions }: RuleActionProps) {
  const t = useT();
  const [isRetiring, setIsRetiring] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <DialogTrigger>
      <Button size="sm" variant="danger" aria-label={t("client:admin.rules.retire_rule", { rule: name })}>
        {t("client:admin.rules.retire")}
      </Button>
      <ConfirmDialog
        title={t("client:admin.rules.retire.title", { rule: name })}
        description={t("client:admin.rules.retire.description")}
        confirmLabel={t("client:admin.rules.retire")}
        cancelLabel={t("client:admin.rules.cancel")}
        isPending={isRetiring}
        {...(failed && actions.error !== undefined && { errorMessage: t("client:admin.rules.change_failed", { reason: actions.error }) })}
        onConfirm={(close) => {
          setIsRetiring(true);
          setFailed(false);
          void actions.retire(rule).then((retired) => {
            setIsRetiring(false);
            if (retired) close();
            else setFailed(true);
          });
        }}
      />
    </DialogTrigger>
  );
}
