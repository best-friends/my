// SPDX-License-Identifier: AGPL-3.0-only
// The form that changes the user's password (FR-002): the current password that confirms it and the new one. A wrong
// current password is that field's error, a new password of the wrong length the new field's (the browser's
// constraint validation catches it first), and any other failure an alert next to the form (web-client.md E2, G16).
import { useMutation } from "@tanstack/react-query";
import { type SyntheticEvent, useState } from "react";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { useT } from "../../../lib/i18n";
import { changePasswordMutation, passwordChangeFailure } from "../api/password";

/** What the form that changes the password needs. */
export type PasswordChangeForm = {
  currentPassword: string;
  changeCurrentPassword: (password: string) => void;
  newPassword: string;
  changeNewPassword: (password: string) => void;
  submit: (event: SyntheticEvent<HTMLFormElement>) => void;
  isPending: boolean;
  /** The current password field's error when the server refused it, localized; undefined otherwise. */
  currentPasswordError: string | undefined;
  /** Why the change failed otherwise, localized, for an alert next to the form; undefined when it did not. */
  error: string | undefined;
  /** What the last change did, for the form's StatusMessage; undefined before the first one. */
  status: string | undefined;
};

/** The form that changes the password. A success empties the form and says that the other sessions ended. */
export function usePasswordChangeForm(): PasswordChangeForm {
  const api = useApi();
  const t = useT();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [status, setStatus] = useState<string | undefined>(undefined);
  const change = useMutation({
    ...changePasswordMutation(api),
    onSuccess: () => {
      setCurrentPassword("");
      setNewPassword("");
      setStatus(t("client:settings.password.changed"));
    },
  });
  const failure = change.isError ? passwordChangeFailure(change.error) : undefined;
  let error: string | undefined;
  if (failure === "rate-limited") error = t("client:settings.password.rate_limited", { reason: errorMessage(change.error) });
  else if (failure === "other") error = t("client:settings.password.failed", { reason: errorMessage(change.error) });
  return {
    currentPassword,
    changeCurrentPassword: (next) => {
      setCurrentPassword(next);
      // A refused password marks the field invalid, which stops the form (native validation) until it changes.
      if (change.isError) change.reset();
    },
    newPassword,
    changeNewPassword: (next) => {
      setNewPassword(next);
      if (change.isError) change.reset();
    },
    submit: (event) => {
      event.preventDefault();
      if (change.isPending || currentPassword === "" || newPassword === "") return;
      change.reset();
      setStatus(undefined);
      change.mutate({ current_password: currentPassword, new_password: newPassword });
    },
    isPending: change.isPending,
    currentPasswordError: failure === "wrong-password" ? t("client:settings.password.wrong_password") : undefined,
    error,
    status,
  };
}
