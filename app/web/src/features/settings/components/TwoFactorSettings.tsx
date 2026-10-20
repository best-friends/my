// SPDX-License-Identifier: AGPL-3.0-only
// The two-factor authentication page below its heading (FR-005): where it stands; while it is off, enrolling an
// authenticator app (the secret as a QR code and as text to type, then the app's first code); while it is on,
// replacing the recovery codes and turning it off, each confirmed by a code. The recovery codes that turning it
// on or replacing them answers are listed once. What a change did stays on the page in a StatusMessage
// (role="status"); a wrong code is the code field's error and any other failure an alert next to its form
// (web-client.md E2, G16).
import { Button, DescriptionItem, DescriptionList, QrCode, StatusMessage, TextField } from "@my/ui";
import { type ReactNode, useId } from "react";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import { twoFactorCodeLimit, type TotpSetup } from "../api/twoFactor";
import { type CodeForm, useTwoFactor } from "../hooks/useTwoFactor";

export function TwoFactorSettings() {
  const t = useT();
  const intl = useIntl();
  const settings = useTwoFactor();
  const { status } = settings;
  return (
    <div className="@container flex flex-col gap-8">
      <StatusMessage>{settings.result}</StatusMessage>
      <Section title={t("client:settings.two_factor.status.title")}>
        {status.enabled ? (
          <DescriptionList columns={2}>
            <DescriptionItem term={t("client:settings.two_factor.status")}>{t("client:settings.two_factor.status.on")}</DescriptionItem>
            {status.enrolled_at !== undefined && (
              <DescriptionItem term={t("client:settings.two_factor.enrolled_at")}>
                <time dateTime={status.enrolled_at}>{intl.formatDate(status.enrolled_at, { dateStyle: "medium", timeStyle: "short" })}</time>
              </DescriptionItem>
            )}
            {status.recovery_codes_remaining !== undefined && (
              <DescriptionItem term={t("client:settings.two_factor.recovery_codes.remaining")}>
                <span className="tabular-nums">{intl.formatNumber(status.recovery_codes_remaining)}</span>
              </DescriptionItem>
            )}
          </DescriptionList>
        ) : (
          <p>{t("client:settings.two_factor.status.off")}</p>
        )}
      </Section>
      {settings.recoveryCodes !== undefined && <RecoveryCodeList codes={settings.recoveryCodes} />}
      {status.enabled ? (
        <>
          <Section title={t("client:settings.two_factor.recovery_codes.title")}>
            <p className="text-sm text-fg-muted">{t("client:settings.two_factor.recovery_codes.description")}</p>
            <CodeFormView form={settings.regenerate} submitLabel={t("client:settings.two_factor.recovery_codes.replace")} />
          </Section>
          <Section title={t("client:settings.two_factor.disable.title")}>
            <p className="text-sm text-fg-muted">{t("client:settings.two_factor.disable.description")}</p>
            <CodeFormView form={settings.disable} submitLabel={t("client:settings.two_factor.disable.submit")} />
          </Section>
        </>
      ) : (
        <Section title={t("client:settings.two_factor.setup.title")}>
          {settings.setup === null ? (
            <div className="flex flex-col items-start gap-3">
              <p className="text-sm text-fg-muted">{t("client:settings.two_factor.setup.description")}</p>
              {settings.setupError !== undefined && (
                <p role="alert" className="text-danger">
                  {settings.setupError}
                </p>
              )}
              <Button variant="primary" onPress={settings.setUp} isPending={settings.isSettingUp}>
                {t("client:settings.two_factor.setup.start")}
              </Button>
            </div>
          ) : (
            <SetupSteps setup={settings.setup} form={settings.enable} />
          )}
        </Section>
      )}
    </div>
  );
}

/** A titled region of the page below its `h1`: an `h2` that names a `<section>`. */
function Section({ title, children }: { title: ReactNode; children: ReactNode }) {
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-lg font-semibold">
        {title}
      </h2>
      {children}
    </section>
  );
}

/** Enrolling the app: scan the code or type the secret, then confirm with the app's first code. */
function SetupSteps({ setup, form }: { setup: TotpSetup; form: CodeForm }) {
  const t = useT();
  const secretId = useId();
  return (
    <div className="flex flex-col gap-6">
      <ol className="flex list-inside list-decimal flex-col gap-1">
        <li>{t("client:settings.two_factor.setup.step.scan")}</li>
        <li>{t("client:settings.two_factor.setup.step.confirm")}</li>
      </ol>
      <QrCode size={setup.qr_code.size} path={setup.qr_code.path} label={t("client:settings.two_factor.setup.qr_code")} />
      <div className="flex flex-col gap-1">
        <span id={secretId} className="text-sm font-medium">
          {t("client:settings.two_factor.setup.secret")}
        </span>
        {/* Base32, typed into the app as it is: left to right in every locale, in groups that read easily. */}
        <code aria-labelledby={secretId} dir="ltr" className="font-mono text-base wrap-anywhere">
          {setup.secret.match(/.{1,4}/g)?.join(" ") ?? setup.secret}
        </code>
      </div>
      <CodeFormView form={form} submitLabel={t("client:settings.two_factor.setup.submit")} description={t("client:settings.two_factor.setup.code.description")} />
    </div>
  );
}

/** A form that sends a code of the app (or a recovery code) to confirm what its button says. */
function CodeFormView({ form, submitLabel, description }: { form: CodeForm; submitLabel: string; description?: string }) {
  const t = useT();
  return (
    <form onSubmit={form.submit} className="flex flex-col items-start gap-3">
      <TextField
        className="w-full max-w-xs"
        label={t("client:settings.two_factor.code")}
        description={description ?? t("client:settings.two_factor.code.description")}
        value={form.code}
        onChange={form.changeCode}
        isRequired
        isInvalid={form.codeError !== undefined}
        maxLength={twoFactorCodeLimit}
        autoComplete="one-time-code"
        spellCheck="false"
        errorMessage={form.codeError ?? t("client:settings.two_factor.code.required")}
      />
      {form.error !== undefined && (
        <p role="alert" className="text-danger">
          {form.error}
        </p>
      )}
      <Button type="submit" variant="primary" isPending={form.isPending}>
        {submitLabel}
      </Button>
    </form>
  );
}

/** The recovery codes as the server answered them, shown this once. */
function RecoveryCodeList({ codes }: { codes: readonly string[] }) {
  const t = useT();
  return (
    <Section title={t("client:settings.two_factor.recovery_codes.new.title")}>
      <p>{t("client:settings.two_factor.recovery_codes.new.description")}</p>
      <ul className="grid grid-cols-1 gap-2 rounded-lg border border-border bg-surface-raised px-4 py-3 font-mono @md:grid-cols-2" dir="ltr">
        {codes.map((code) => (
          <li key={code}>{code}</li>
        ))}
      </ul>
    </Section>
  );
}
