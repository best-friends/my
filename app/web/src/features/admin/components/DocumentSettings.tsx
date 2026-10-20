// SPDX-License-Identifier: AGPL-3.0-only
// The terms and policies screen (#489), for manage_settings: a language, and the terms of service and the
// privacy policy in it, each a form of its own (an `h2` section) with a Markdown text area, what is saved, and
// a link to its public page. Removing a saved text asks first (the APG alert dialog pattern, web-client.md G22).
import { Button, ConfirmDialog, DescriptionItem, DescriptionList, ExternalLink, Select, SelectItem, StatusMessage, TextArea } from "@my/ui";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import { useUnsavedChangesGuard } from "../../settings";
import { documentTextLimit } from "../api/documents";
import { type DocumentEditor, useDocumentSettings } from "../hooks/useDocumentSettings";
import { DateTime, DetailSection, RequirePermission } from "../../moderation";

export type DocumentSettingsProps = {
  /** The language from the URL; undefined chooses the display language. */
  locale: string | undefined;
  /** Chooses another language: the route puts it into the URL. */
  onLocaleChange: (locale: string) => void;
};

export function DocumentSettings(props: DocumentSettingsProps) {
  return (
    <RequirePermission permission="manage_settings">
      <DocumentSettingsForms {...props} />
    </RequirePermission>
  );
}

function DocumentSettingsForms({ locale, onLocaleChange }: DocumentSettingsProps) {
  const t = useT();
  const settings = useDocumentSettings(locale);
  return (
    <div className="flex flex-col gap-8">
      <Select
        label={t("client:admin.documents.locale")}
        description={t("client:admin.documents.locale.description")}
        value={settings.locale}
        onChange={(key) => {
          if (typeof key === "string") onLocaleChange(key);
        }}
        items={settings.languages}
      >
        {(language) => (
          <SelectItem id={language.id} textValue={language.name}>
            {language.name}
          </SelectItem>
        )}
      </Select>
      <DetailSection title={t("client:admin.documents.terms")}>
        <DocumentForm
          editor={settings.terms}
          languageName={settings.languageName}
          viewLabel={t("client:admin.documents.view_terms")}
          confirmTitle={t("client:admin.documents.confirm_remove.terms", { language: settings.languageName })}
        />
      </DetailSection>
      <DetailSection title={t("client:admin.documents.privacy_policy")}>
        <DocumentForm
          editor={settings.privacyPolicy}
          languageName={settings.languageName}
          viewLabel={t("client:admin.documents.view_privacy_policy")}
          confirmTitle={t("client:admin.documents.confirm_remove.privacy_policy", { language: settings.languageName })}
        />
      </DetailSection>
      <UnsavedChangesDialog hasChanges={settings.hasChanges} />
    </div>
  );
}

type DocumentFormProps = {
  editor: DocumentEditor;
  /** The name of the selected language. */
  languageName: string;
  /** The name of the link to the document's public page. */
  viewLabel: string;
  /** The question before removing the text in the selected language. */
  confirmTitle: string;
};

/** One document: what is saved, the text in the selected language, saving it, and its public page. */
function DocumentForm({ editor, languageName, viewLabel, confirmTitle }: DocumentFormProps) {
  const t = useT();
  const intl = useIntl();
  const formatRemaining = (remaining: number) =>
    remaining < 0 ? t("client:admin.documents.characters_over", { count: -remaining }) : t("client:admin.documents.characters_left", { count: remaining });
  const failed = editor.error === undefined ? undefined : t("client:admin.documents.save_failed", { reason: editor.error });
  return (
    <>
      <DescriptionList>
        <DescriptionItem term={t("client:admin.documents.languages")}>
          {editor.languages.length === 0 ? t("client:admin.documents.languages.none") : intl.formatList(editor.languages, { type: "conjunction" })}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.documents.saved_at", { language: languageName })}>
          {editor.savedAt === undefined ? t("client:admin.documents.saved_at.never") : <DateTime value={editor.savedAt} />}
        </DescriptionItem>
      </DescriptionList>
      {editor.pageUrl !== undefined && (
        <p>
          <ExternalLink href={editor.pageUrl} variant="standalone">
            {viewLabel}
          </ExternalLink>
        </p>
      )}
      <form onSubmit={editor.submit} className="flex flex-col gap-4">
        <TextArea
          label={t("client:admin.documents.body", { language: languageName })}
          description={t("client:admin.documents.body.description")}
          value={editor.value}
          onChange={editor.change}
          rows={12}
          characterLimit={documentTextLimit}
          formatRemaining={formatRemaining}
          errorMessage={t("client:admin.documents.too_long", { max: documentTextLimit })}
        />
        {editor.confirmation === null && failed !== undefined && (
          <p role="alert" className="text-danger">
            {failed}
          </p>
        )}
        <div>
          <Button type="submit" variant="primary" isPending={editor.isPending && editor.confirmation === null}>
            {t("client:admin.documents.save")}
          </Button>
        </div>
        <StatusMessage>{editor.status}</StatusMessage>
      </form>
      {/* Outside the form: React sends the dialog's own submit event up its component tree, through a portal too. */}
      <ConfirmDialog
        isOpen={editor.confirmation !== null}
        onOpenChange={(isOpen) => {
          if (!isOpen) editor.confirmation?.cancel();
        }}
        title={confirmTitle}
        description={t("client:admin.documents.confirm_remove.description")}
        confirmLabel={t("client:admin.documents.confirm_remove.confirm")}
        cancelLabel={t("client:admin.documents.confirm_remove.cancel")}
        tone="danger"
        onConfirm={() => {
          editor.confirmation?.confirm();
        }}
        isPending={editor.isPending}
        {...(editor.confirmation !== null && failed !== undefined ? { errorMessage: failed } : {})}
      />
    </>
  );
}

/**
 * Asks before another page replaces the screen with unsaved texts (the APG alert dialog pattern): focus starts
 * on "Keep editing", which stays on the screen like Esc does.
 */
function UnsavedChangesDialog({ hasChanges }: { hasChanges: boolean }) {
  const t = useT();
  const guard = useUnsavedChangesGuard(hasChanges);
  return (
    <ConfirmDialog
      isOpen={guard.isAsking}
      onOpenChange={(isOpen) => {
        if (!isOpen) guard.stay();
      }}
      title={t("client:admin.documents.unsaved.title")}
      description={t("client:admin.documents.unsaved.description")}
      confirmLabel={t("client:admin.documents.unsaved.leave")}
      cancelLabel={t("client:admin.documents.unsaved.stay")}
      onConfirm={guard.leave}
    />
  );
}
