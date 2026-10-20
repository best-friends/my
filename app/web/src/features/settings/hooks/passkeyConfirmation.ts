// SPDX-License-Identifier: AGPL-3.0-only
// What the operations confirmed with the password or a passkey say when the confirmation fails (web-client.md E2, G16):
// a dismissed or failed prompt of the browser, a passkey that the server did not take, or a confirmation that took too
// long. The other failures are each operation's own.
import { errorMessage } from "../../../lib/errors";
import type { Translate } from "../../../lib/i18n";
import type { ConfirmationFailure } from "../api/password";

/** The localized message of a failed confirmation with a passkey, or undefined for a failure of another kind. */
function passkeyConfirmationMessage(t: Translate, failure: ConfirmationFailure): string | undefined {
  switch (failure) {
    case "cancelled":
      return t("client:settings.passkey_confirmation.cancelled");
    case "unavailable":
      return t("client:settings.passkey_confirmation.unavailable");
    case "invalid-passkey":
      return t("client:settings.passkey_confirmation.invalid");
    case "expired":
      return t("client:settings.passkey_confirmation.expired");
    default:
      return undefined;
  }
}

/** The localized message of a failed operation: that of its confirmation, or `fallback` with the reason. */
export function confirmationErrorMessage(
  t: Translate,
  failure: ConfirmationFailure,
  error: unknown,
  fallback: (reason: string) => string,
): string {
  return passkeyConfirmationMessage(t, failure) ?? fallback(errorMessage(error));
}
