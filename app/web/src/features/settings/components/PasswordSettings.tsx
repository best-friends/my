// SPDX-License-Identifier: AGPL-3.0-only
// The password page below its heading (FR-002): what a change does (the other browsers and apps are signed out, the
// passkeys and two-factor authentication stay) and the form that changes the password with the current one. What the
// change did stays on the page in a StatusMessage (role="status"). A wrong current password is that field's error and
// any other failure an alert next to the form (web-client.md E2, G16).
import { Button, StatusMessage, TextField } from "@my/ui";
import { useT } from "../../../lib/i18n";
import { passwordMaxLength, passwordMinLength } from "../api/password";
import { usePasswordChangeForm } from "../hooks/usePasswordChange";

export function PasswordSettings() {
  const t = useT();
  const form = usePasswordChangeForm();
  const lengths = { min: passwordMinLength, max: passwordMaxLength };
  return (
    <div className="@container flex flex-col gap-8">
      <p>{t("client:settings.password.description")}</p>
      <form onSubmit={form.submit} className="flex flex-col items-start gap-4">
        <TextField
          className="w-full max-w-sm"
          label={t("client:settings.password.current")}
          type="password"
          autoComplete="current-password"
          value={form.currentPassword}
          onChange={form.changeCurrentPassword}
          isRequired
          isInvalid={form.currentPasswordError !== undefined}
          errorMessage={form.currentPasswordError ?? t("client:settings.password.current.required")}
        />
        <TextField
          className="w-full max-w-sm"
          label={t("client:settings.password.new")}
          description={t("client:settings.password.new.description", lengths)}
          type="password"
          autoComplete="new-password"
          value={form.newPassword}
          onChange={form.changeNewPassword}
          isRequired
          minLength={passwordMinLength}
          maxLength={passwordMaxLength}
          errorMessage={t("client:settings.password.new.invalid", lengths)}
        />
        {form.error !== undefined && (
          <p role="alert" className="text-danger">
            {form.error}
          </p>
        )}
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:settings.password.submit")}
        </Button>
        <StatusMessage>{form.status}</StatusMessage>
      </form>
    </div>
  );
}
