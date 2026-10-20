// SPDX-License-Identifier: AGPL-3.0-only
// The theme screen below its heading (manage_settings, ADR-0059): the color tokens in each theme, the contrast of
// their pairs as they are typed, the other tokens (type, spacing, radius, elevation, motion), the custom CSS with
// what the server does not check, and a preview of the overrides. An empty input keeps the default, which its
// description names. The reasons the server refuses a theme stay above the button (role="alert", web-client.md
// E2); what a save did stays on the page in a StatusMessage.
import { Button, StatusMessage, TextArea, TextField } from "@my/ui";
import { useT } from "../../../lib/i18n";
import { type ColorField, useThemeSettings, type ValueField } from "../hooks/useThemeSettings";
import { DetailSection, RequirePermission } from "./parts";

export function ThemeSettings() {
  return (
    <RequirePermission permission="manage_settings">
      <ThemeSettingsForm />
    </RequirePermission>
  );
}

function ThemeSettingsForm() {
  const t = useT();
  const form = useThemeSettings();
  return (
    <form onSubmit={form.submit} className="flex flex-col gap-6">
      <p className="text-fg-muted">{t("client:admin.theme.description")}</p>
      <DetailSection title={t("client:admin.theme.colors.title")}>
        <p className="text-fg-muted">{t("client:admin.theme.colors.description")}</p>
        <ul className="flex flex-col gap-4">
          {form.colorFields.map((field) => (
            <ColorInputs key={field.token} field={field} />
          ))}
        </ul>
      </DetailSection>
      <DetailSection title={t("client:admin.theme.contrast.title")}>
        {form.failingPairs.length === 0 && form.malformedColors.length === 0 ? (
          <p>{t("client:admin.theme.contrast.ok")}</p>
        ) : (
          <ul className="flex list-disc flex-col gap-1 ps-6 text-danger">
            {form.malformedColors.map((text) => (
              <li key={text}>{text}</li>
            ))}
            {form.failingPairs.map((pair) => (
              <li key={pair.key}>{pair.text}</li>
            ))}
          </ul>
        )}
      </DetailSection>
      <DetailSection title={t("client:admin.theme.tokens.title")}>
        <p className="text-fg-muted">{t("client:admin.theme.tokens.description")}</p>
        {form.valueFields.map((field) => (
          <ValueInput key={field.token.name} field={field} />
        ))}
      </DetailSection>
      <DetailSection title={t("client:admin.theme.css.title")}>
        <p className="text-fg-muted">{t("client:admin.theme.css.description")}</p>
        <TextArea
          label={t("client:admin.theme.css.label")}
          value={form.customCss.value}
          onChange={form.customCss.onChange}
          rows={8}
          {...(form.customCss.error === undefined ? {} : { isInvalid: true, errorMessage: form.customCss.error })}
        />
      </DetailSection>
      <DetailSection title={t("client:admin.theme.preview.title")}>
        <p className="text-fg-muted">{t("client:admin.theme.preview.description")}</p>
        <Preview style={form.previewStyle} />
      </DetailSection>
      {form.saveErrors !== undefined && (
        <div role="alert" className="flex flex-col gap-1 text-danger">
          <p>{t("client:admin.theme.save_refused")}</p>
          <ul className="flex list-disc flex-col gap-1 ps-6">
            {form.saveErrors.map((text) => (
              <li key={text}>{text}</li>
            ))}
          </ul>
        </div>
      )}
      {form.saveError !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.theme.save_failed", { reason: form.saveError })}
        </p>
      )}
      <div>
        <Button type="submit" variant="primary" isPending={form.isSaving}>
          {t("client:admin.theme.save")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
    </form>
  );
}

function ColorInputs({ field }: { field: ColorField }) {
  const t = useT();
  const format = field.alpha ? "#rrggbbaa" : "#rrggbb";
  return (
    <li className="flex flex-col gap-2">
      <code dir="ltr" className="font-mono text-sm font-semibold">
        {field.token}
      </code>
      <div className="grid gap-2 @md:grid-cols-2">
        <TextField
          label={t("client:admin.theme.color.light", { token: field.token })}
          description={t("client:admin.theme.color.default", { value: field.defaults.light, format })}
          value={field.light.value}
          onChange={field.light.onChange}
          placeholder={field.defaults.light}
          autoComplete="off"
        />
        <TextField
          label={t("client:admin.theme.color.dark", { token: field.token })}
          description={t("client:admin.theme.color.default", { value: field.defaults.dark, format })}
          value={field.dark.value}
          onChange={field.dark.onChange}
          placeholder={field.defaults.dark}
          autoComplete="off"
        />
      </div>
    </li>
  );
}

function ValueInput({ field }: { field: ValueField }) {
  const t = useT();
  const { token } = field;
  const value = token.value ?? "";
  let description: string;
  if (token.kind === "length" && token.min !== undefined && token.max !== undefined) {
    description = t("client:admin.theme.bounds.length", { value, min: String(token.min), max: String(token.max) });
  } else if (token.kind === "duration" && token.min !== undefined && token.max !== undefined) {
    description = t("client:admin.theme.bounds.duration", { value, min: String(token.min), max: String(token.max) });
  } else if (token.kind === "number" && token.min !== undefined && token.max !== undefined) {
    description = t("client:admin.theme.bounds.number", { value, min: String(token.min), max: String(token.max) });
  } else {
    description = t("client:admin.theme.bounds.default", { value });
  }
  return (
    <TextField
      label={token.name}
      description={description}
      value={field.input.value}
      onChange={field.input.onChange}
      autoComplete="off"
    />
  );
}

/** A few parts of the app with the overrides applied to the region's custom properties. */
function Preview({ style }: { style: Record<string, string> }) {
  const t = useT();
  return (
    // The overrides are custom properties (D10 allows React's style); the utilities inside read them.
    <div style={style} className="flex flex-col gap-3 rounded-lg border border-border bg-surface p-4 font-sans text-fg">
      <div className="flex flex-col gap-2 rounded-md bg-surface-raised p-4 shadow-md">
        <p className="text-lg font-semibold">{t("client:admin.theme.preview.heading")}</p>
        <p>{t("client:admin.theme.preview.text")}</p>
        <p className="text-sm text-fg-muted">{t("client:admin.theme.preview.muted")}</p>
        {/* Look-alikes of a link and a button: the preview has nothing to operate. */}
        <p className="text-accent underline underline-offset-2">{t("client:admin.theme.preview.link")}</p>
        <p className="self-start rounded-md bg-accent px-4 py-2 font-medium text-accent-fg">{t("client:admin.theme.preview.button")}</p>
      </div>
      <p className="text-danger">{t("client:admin.theme.preview.danger")}</p>
    </div>
  );
}
