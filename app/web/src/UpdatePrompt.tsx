// SPDX-License-Identifier: AGPL-3.0-only
// "A new version is available" (ADR-0028): a toast with an Update action that stays until it is used or
// dismissed. The new version takes over only when the user presses Update, so a draft is never lost to a
// reload the user did not ask for. The toast is announced politely and never takes focus (web-client.md Y7).
import { useToast } from "@my/ui";
import { useEffect, useEffectEvent } from "react";
import { type PendingUpdate, usePendingUpdate } from "./lib/pwa";
import { useT } from "./lib/i18n";

export function UpdatePrompt() {
  const update = usePendingUpdate();
  const toast = useToast();
  const t = useT();
  // The texts and the toast API are read when the toast opens or closes, so that neither a change of
  // locale nor a new toast API object reopens it.
  const show = useEffectEvent((pending: PendingUpdate) =>
    toast.show({
      message: t("client:shell.update.available"),
      action: { label: t("client:shell.update.apply"), onAction: pending.apply },
      timeout: null,
    }),
  );
  const dismiss = useEffectEvent((id: number) => {
    toast.dismiss(id);
  });
  // Syncs the toast with the worker's state, an external system (web-client.md R8).
  useEffect(() => {
    if (update === null) return;
    const id = show(update);
    return () => {
      dismiss(id);
    };
  }, [update]);
  return null;
}
