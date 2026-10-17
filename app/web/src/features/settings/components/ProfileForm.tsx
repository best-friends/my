// SPDX-License-Identifier: AGPL-3.0-only
import { Avatar, Button, Checkbox, ConfirmDialog, FileTrigger, TextArea, TextField } from "@my/ui";
import { type ReactNode, useId } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { acceptedImageTypes } from "../api/media";
import { characterCount, type FieldDraft, type ImageControl, profileLimits, useProfileForm } from "../hooks/useProfileForm";
import { useUnsavedChangesGuard } from "../hooks/useUnsavedChangesGuard";

/**
 * The profile editor (FR-006). React Aria's native validation shows each error under its field, links it with
 * aria-describedby and marks the field invalid; the browser then refuses to submit, and focus moves to the
 * first invalid field, so a screen reader reads the error with the field. A text over its limit is invalid as
 * soon as it is typed, like the count of the bio: an error that appeared only when the field lost focus would
 * push the save button away under the pointer that is pressing it.
 */
export function ProfileForm() {
  const t = useT();
  const form = useProfileForm();
  // Only an invalid text sets isInvalid: React Aria ignores `validate` once isInvalid is given at all.
  const lengthCheck: LengthCheck = (value, max) =>
    characterCount(value) > max ? { isInvalid: true, errorMessage: t("client:settings.profile.too_long", { max: String(max) }) } : {};
  return (
    <>
      <form onSubmit={form.submit} className="flex flex-col gap-6">
        <ImageSection
          titleKey="client:settings.profile.avatar"
          control={form.avatar}
          preview={<Avatar src={form.avatar.url} name={form.displayName.value || form.account.username} decorative size="xl" />}
        />
        <ImageSection
          titleKey="client:settings.profile.header"
          control={form.header}
          preview={
            form.header.url === undefined ? null : (
              <img src={form.header.url} alt="" className="aspect-3/1 w-full rounded-md bg-surface-sunken object-cover" />
            )
          }
        />
        <TextField
          label={t("client:settings.profile.display_name")}
          description={t("client:settings.profile.display_name.description", { max: String(profileLimits.displayName) })}
          value={form.displayName.value}
          onChange={form.displayName.onChange}
          {...lengthCheck(form.displayName.value, profileLimits.displayName)}
          autoComplete="nickname"
        />
        <TextArea
          label={t("client:settings.profile.note")}
          value={form.note.value}
          onChange={form.note.onChange}
          characterLimit={profileLimits.note}
          formatRemaining={(remaining) =>
            remaining < 0 ? t("client:settings.profile.characters_over", { count: -remaining }) : t("client:settings.profile.characters_left", { count: remaining })
          }
          errorMessage={t("client:settings.profile.too_long", { max: String(profileLimits.note) })}
          rows={4}
        />
        <ProfileFields fields={form.fields} lengthCheck={lengthCheck} />
        <div className="flex flex-col gap-2">
          <Checkbox {...form.locked} description={t("client:settings.profile.locked.description")}>
            {t("client:settings.profile.locked")}
          </Checkbox>
          <Checkbox {...form.bot} description={t("client:settings.profile.bot.description")}>
            {t("client:settings.profile.bot")}
          </Checkbox>
        </div>
        {form.saveError !== undefined && (
          <p role="alert" className="text-danger">
            {form.saveError}
          </p>
        )}
        <div>
          <Button type="submit" variant="primary" isPending={form.isSaving} isDisabled={form.isBusy && !form.isSaving}>
            {t("client:settings.profile.save")}
          </Button>
        </div>
      </form>
      {/* Outside the form: React bubbles the dialog's own submit event through its portal to an enclosing form. */}
      <UnsavedChangesDialog hasChanges={form.hasChanges} />
    </>
  );
}

/**
 * Asks before another page replaces a profile with unsaved changes (the APG alert dialog pattern): focus
 * starts on "Keep editing", which stays on the form like Esc does, and returns to the link that was pressed.
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
      title={t("client:settings.profile.unsaved.title")}
      description={t("client:settings.profile.unsaved.description")}
      confirmLabel={t("client:settings.profile.unsaved.leave")}
      cancelLabel={t("client:settings.profile.unsaved.stay")}
      onConfirm={guard.leave}
    />
  );
}

type ImageSectionProps = { titleKey: ClientMessageKey; control: ImageControl; preview: ReactNode };

/** An image of the profile: its preview, and buttons to choose a new file or remove it. */
function ImageSection({ titleKey, control, preview }: ImageSectionProps) {
  const t = useT();
  const headingId = useId();
  const errorId = useId();
  return (
    <section aria-labelledby={headingId} aria-busy={control.isUploading} className="flex flex-col gap-2">
      <h2 id={headingId} className="text-sm font-medium text-fg">
        {t(titleKey)}
      </h2>
      {preview}
      <div className="flex flex-wrap gap-2">
        <FileTrigger
          acceptedFileTypes={acceptedImageTypes}
          onSelect={(files) => {
            const file = files?.[0];
            if (file !== undefined) control.choose(file);
          }}
        >
          <Button size="sm" isPending={control.isUploading} aria-describedby={control.error === undefined ? headingId : `${headingId} ${errorId}`}>
            {t("client:settings.profile.image.choose")}
          </Button>
        </FileTrigger>
        {control.hasImage && (
          <Button size="sm" variant="ghost" onPress={control.remove} isDisabled={control.isUploading} aria-describedby={headingId}>
            {t("client:settings.profile.image.remove")}
          </Button>
        )}
      </div>
      {control.error !== undefined && (
        <p id={errorId} role="alert" className="text-sm text-danger">
          {control.error}
        </p>
      )}
    </section>
  );
}

/** The props that mark a text over its limit invalid, with the error that says so; none for a text within it. */
type LengthCheck = (value: string, max: number) => { isInvalid?: true; errorMessage?: string };

type ProfileFieldsProps = {
  fields: ReturnType<typeof useProfileForm>["fields"];
  lengthCheck: LengthCheck;
};

/** The metadata fields: up to four label and content pairs, each its own group. */
function ProfileFields({ fields, lengthCheck }: ProfileFieldsProps) {
  const t = useT();
  const headingId = useId();
  return (
    <section aria-labelledby={headingId} className="flex flex-col gap-3">
      <h2 id={headingId} className="text-sm font-medium text-fg">
        {t("client:settings.profile.fields")}
      </h2>
      <p className="text-sm text-fg-muted">{t("client:settings.profile.fields.description", { max: String(profileLimits.fields) })}</p>
      {fields.rows.map((field, index) => (
        <FieldRow
          key={field.key}
          field={field}
          number={index + 1}
          onChange={(change) => {
            fields.update(field.key, change);
          }}
          onRemove={() => {
            fields.remove(field.key);
          }}
          lengthCheck={lengthCheck}
        />
      ))}
      <div>
        <Button size="sm" onPress={fields.add} isDisabled={!fields.canAdd}>
          {t("client:settings.profile.fields.add")}
        </Button>
      </div>
    </section>
  );
}

type FieldRowProps = {
  field: FieldDraft;
  number: number;
  onChange: (change: Partial<Omit<FieldDraft, "key">>) => void;
  onRemove: () => void;
  lengthCheck: LengthCheck;
};

function FieldRow({ field, number, onChange, onRemove, lengthCheck }: FieldRowProps) {
  const t = useT();
  return (
    <fieldset className="flex min-w-0 flex-col gap-2 rounded-md border border-border p-3">
      <legend className="px-1 text-sm font-medium text-fg">{t("client:settings.profile.field", { number: String(number) })}</legend>
      <TextField
        label={t("client:settings.profile.field.name")}
        value={field.name}
        onChange={(name) => {
          onChange({ name });
        }}
        {...lengthCheck(field.name, profileLimits.fieldName)}
        // A missing label is reported when the form is submitted, as the content is often typed first.
        validate={(name) => (name === "" && field.value !== "" ? t("client:settings.profile.field.name_required") : null)}
      />
      <TextField
        label={t("client:settings.profile.field.value")}
        value={field.value}
        onChange={(value) => {
          onChange({ value });
        }}
        {...lengthCheck(field.value, profileLimits.fieldValue)}
      />
      <div>
        <Button size="sm" variant="ghost" onPress={onRemove}>
          {t("client:settings.profile.field.remove", { number: String(number) })}
        </Button>
      </div>
    </fieldset>
  );
}
