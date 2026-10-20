// SPDX-License-Identifier: AGPL-3.0-only
// The passkeys page below its heading (FR-004): what a passkey does, the form that adds one (its name and the
// password that confirms it, or for a user without a password another passkey, then the browser's prompt), and the
// passkeys, oldest first, each with when it was added
// and its button that removes it after an alert dialog (ConfirmDialog: it can no longer sign in). A browser that
// cannot create passkeys gets a note instead of the form. What adding did stays on the page in a StatusMessage
// (role="status"); removing is announced and moves focus to the next row. A wrong password is the password field's
// error and any other failure an alert next to the form (web-client.md E2, G16).
import { Button, ConfirmDialog, EmptyState, StatusMessage, TextField } from "@my/ui";
import { type ReactNode, useId, useRef, useState } from "react";
import { useIntl } from "react-intl";
import { useT } from "../../../lib/i18n";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import { type Passkey, passkeyNameLimit } from "../api/passkeys";
import { type RemovePasskey, usePasskeyForm, usePasskeyList, useRemovePasskey } from "../hooks/usePasskeys";

export function PasskeySettings() {
  const t = useT();
  return (
    <div className="@container flex flex-col gap-8">
      <p>{t("client:settings.passkeys.description")}</p>
      <Section title={t("client:settings.passkeys.add.title")}>
        <AddPasskeyForm />
      </Section>
      <Section title={t("client:settings.passkeys.list.title")}>
        <PasskeyList />
      </Section>
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

function AddPasskeyForm() {
  const t = useT();
  const form = usePasskeyForm();
  if (!form.isSupported) return <p>{t("client:settings.passkeys.unsupported")}</p>;
  return (
    <form onSubmit={form.submit} className="flex flex-col items-start gap-4">
      <p className="text-sm text-fg-muted">
        {t(form.hasPassword ? "client:settings.passkeys.add.description" : "client:settings.passkeys.add.description_passkey")}
      </p>
      <TextField
        className="w-full max-w-sm"
        label={t("client:settings.passkeys.name")}
        description={t("client:settings.passkeys.name.description")}
        value={form.name}
        onChange={form.changeName}
        isRequired
        maxLength={passkeyNameLimit}
        autoComplete="off"
        errorMessage={t("client:settings.passkeys.name.required")}
      />
      {form.hasPassword && (
        <TextField
          className="w-full max-w-sm"
          label={t("client:settings.passkeys.password")}
          description={t("client:settings.passkeys.password.description")}
          type="password"
          autoComplete="current-password"
          value={form.password}
          onChange={form.changePassword}
          isRequired
          isInvalid={form.passwordError !== undefined}
          errorMessage={form.passwordError ?? t("client:settings.passkeys.password.required")}
        />
      )}
      {form.error !== undefined && (
        <p role="alert" className="text-danger">
          {form.error}
        </p>
      )}
      <Button type="submit" variant="primary" isPending={form.isPending}>
        {t("client:settings.passkeys.add.submit")}
      </Button>
      <StatusMessage>{form.status}</StatusMessage>
    </form>
  );
}

function PasskeyList() {
  const t = useT();
  const passkeys = usePasskeyList();
  const remove = useRemovePasskey();
  const listRef = useRef<HTMLUListElement>(null);
  const removed = useFocusAfterRemoval(
    listRef,
    passkeys.map((passkey) => passkey.id),
  );
  const [confirming, setConfirming] = useState<Passkey | null>(null);
  if (passkeys.length === 0) return <EmptyState headingLevel={3} heading={t("client:settings.passkeys.empty")} />;
  return (
    <>
      <ul ref={listRef} className="flex flex-col divide-y divide-border">
        {passkeys.map((passkey) => (
          <li key={passkey.id} data-row-id={passkey.id} className="@container py-3">
            <PasskeyRow
              passkey={passkey}
              remove={remove}
              onRemove={() => {
                remove.reset();
                setConfirming(passkey);
              }}
            />
          </li>
        ))}
      </ul>
      <ConfirmDialog
        isOpen={confirming !== null}
        onOpenChange={(isOpen) => {
          if (!isOpen) setConfirming(null);
        }}
        title={t("client:settings.passkeys.remove.title", { name: confirming?.name ?? "" })}
        description={t("client:settings.passkeys.remove.description")}
        confirmLabel={t("client:settings.passkeys.remove")}
        cancelLabel={t("client:settings.passkeys.remove.cancel")}
        tone="danger"
        isPending={confirming !== null && remove.pendingId === confirming.id}
        {...(remove.error === undefined ? {} : { errorMessage: remove.error })}
        onConfirm={(close) => {
          if (confirming === null) return;
          remove.run(confirming, () => {
            removed(confirming.id);
            close();
          });
        }}
      />
    </>
  );
}

/** A passkey: its name and when it was added, with the button that removes it. */
function PasskeyRow({ passkey, remove, onRemove }: { passkey: Passkey; remove: RemovePasskey; onRemove: () => void }) {
  const t = useT();
  const intl = useIntl();
  const label = t("client:settings.passkeys.remove");
  return (
    <div className="flex flex-col gap-3 @md:flex-row @md:items-center">
      <p className="flex min-w-0 flex-1 flex-col">
        <span dir="auto" className="truncate text-start font-semibold">
          {passkey.name}
        </span>
        <span className="text-sm text-fg-muted">
          {t("client:settings.passkeys.added_at", {
            date: intl.formatDate(passkey.created_at, { dateStyle: "medium", timeStyle: "short" }),
          })}
        </span>
      </p>
      <div className="flex flex-wrap gap-2">
        <Button
          size="sm"
          variant="danger"
          isPending={remove.pendingId === passkey.id}
          aria-label={t("client:settings.list.action", { action: label, name: passkey.name })}
          onPress={onRemove}
        >
          {label}
        </Button>
      </div>
    </div>
  );
}
