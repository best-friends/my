// SPDX-License-Identifier: AGPL-3.0-only
// The server settings screen below its heading (manage_settings): the instance's name in each language of the
// app. The name stands in for the domain where the instance is named to people; the domain stays in handles and
// links. The browser keeps the form from being sent while a field is invalid (React Aria's native validation:
// the error is under its field and linked to it, and focus moves to the first invalid field). What a save did
// stays on the page in a StatusMessage; a failure stays above the button (web-client.md E2).
import { Button, StatusMessage, TextField } from "@my/ui";
import { useT } from "../../../lib/i18n";
import { useServerSettings } from "../hooks/useServerSettings";
import { DetailSection, RequirePermission } from "../../moderation";

export function ServerSettings() {
  return (
    <RequirePermission permission="manage_settings">
      <ServerSettingsForm />
    </RequirePermission>
  );
}

function ServerSettingsForm() {
  const t = useT();
  const form = useServerSettings();
  return (
    <DetailSection title={t("client:admin.settings.name.title")}>
      <p className="text-fg-muted">{t("client:admin.settings.name.description", { domain: form.domain })}</p>
      <form onSubmit={form.submit} className="flex flex-col gap-4">
        {form.fields.map((field) => (
          <TextField
            key={field.locale}
            label={t("client:admin.settings.name.field", { language: field.language })}
            value={field.value}
            onChange={field.onChange}
            // Only an invalid name sets isInvalid: React Aria ignores `validate` once isInvalid is given at all.
            {...(field.error === undefined ? {} : { isInvalid: true, errorMessage: field.error })}
          />
        ))}
        {form.saveError !== undefined && (
          <p role="alert" className="text-danger">
            {t("client:admin.settings.name.save_failed", { reason: form.saveError })}
          </p>
        )}
        <div>
          <Button type="submit" variant="primary" isPending={form.isSaving}>
            {t("client:admin.settings.name.save")}
          </Button>
        </div>
        <StatusMessage>{form.status}</StatusMessage>
      </form>
    </DetailSection>
  );
}
