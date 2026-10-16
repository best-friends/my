// SPDX-License-Identifier: AGPL-3.0-only
import { Button } from "@my/ui";
import { useConfig } from "../../../lib/config";
import { useT } from "../../../lib/i18n";
import { useSignIn } from "../hooks/useSignIn";

export type SignInPanelProps = {
  /** The path of this app to open after signing in. */
  returnTo: string;
};

/** The body of the sign-in page: what signing in gives, and the button to the server's sign-in screen. */
export function SignInPanel({ returnTo }: SignInPanelProps) {
  const t = useT();
  const { instanceName } = useConfig();
  const { signIn, isPending } = useSignIn(returnTo);
  return (
    <div className="flex flex-col items-start gap-4">
      <p>{t("client:login.body", { instance: instanceName })}</p>
      <Button variant="primary" onPress={signIn} isPending={isPending}>
        {t("client:login.submit")}
      </Button>
    </div>
  );
}
