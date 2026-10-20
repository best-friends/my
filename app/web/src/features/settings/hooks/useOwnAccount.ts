// SPDX-License-Identifier: AGPL-3.0-only
// The user's own account (FR-008): asking for archives of the user's data and downloading them, and deleting the
// account with the password. A pending archive is read again every few seconds until it is built.
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";
import { useApi, useSession } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { useT } from "../../../lib/i18n";
import {
  cacheRequestedExport,
  type DataExport,
  dataExportsQueryOptions,
  deleteOwnAccountMutation,
  fetchExportArchive,
  isWrongPasswordError,
  requestDataExportMutation,
} from "../api/account";

/** How often a pending archive is checked, in ms. */
const pendingPollInterval = 5000;
/** How long the URL of a downloaded archive stays usable, in ms: long enough for the browser to start saving it. */
const downloadUrlLifetime = 60_000;

/** What the archives section needs. */
export type DataExports = {
  items: DataExport[];
  /** Asks for a new archive; only while none is pending. */
  request: () => void;
  canRequest: boolean;
  isRequesting: boolean;
  /** Saves the archive of a ready export as a file. */
  download: (item: DataExport) => void;
  /** The export whose archive is being downloaded, if any. */
  downloadingId: string | undefined;
  /** Why the last request or download failed, localized; undefined when it did not. */
  error: string | undefined;
  /** What the last request did, for the section's StatusMessage. */
  status: string | undefined;
};

/** Saves `blob` as a file named `name` through a temporary link, as the browser's download does. */
function saveFile(blob: Blob, name: string): void {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  setTimeout(() => {
    URL.revokeObjectURL(url);
  }, downloadUrlLifetime);
}

export function useDataExports(): DataExports {
  const api = useApi();
  const t = useT();
  const queryClient = useQueryClient();
  const query = useSuspenseQuery({
    ...dataExportsQueryOptions(api),
    refetchInterval: (q) => (q.state.data?.items.some((item) => item.state === "pending") === true ? pendingPollInterval : false),
  });
  const [status, setStatus] = useState<string | undefined>(undefined);
  const request = useMutation({
    ...requestDataExportMutation(api),
    onSuccess: (requested) => {
      cacheRequestedExport(api, queryClient, requested);
      setStatus(t("client:settings.account.export.requested"));
    },
  });
  const download = useMutation({
    mutationKey: ["get", "/exports/{id}/archive"] as const,
    mutationFn: async (item: DataExport) => {
      saveFile(await fetchExportArchive(api, item.id), `export-${item.id}.zip`);
    },
  });
  const items = query.data.items;
  const failed = request.isError ? request.error : download.isError ? download.error : null;
  return {
    items,
    request: () => {
      if (!request.isPending) request.mutate();
    },
    canRequest: !items.some((item) => item.state === "pending"),
    isRequesting: request.isPending,
    download: (item) => {
      download.mutate(item);
    },
    downloadingId: download.isPending ? download.variables.id : undefined,
    error: failed === null ? undefined : errorMessage(failed),
    status,
  };
}

/** What the deletion of the account needs: the dialog with the password, and deleting. */
export type AccountDeletion = {
  isOpen: boolean;
  open: () => void;
  cancel: () => void;
  password: string;
  changePassword: (password: string) => void;
  /** Deletes the account; on success the user is signed out and sent to the sign-in page. */
  submit: () => void;
  isPending: boolean;
  /** Why the deletion failed, localized, for the dialog (which stays open). */
  error: string | undefined;
};

export function useAccountDeletion(): AccountDeletion {
  const api = useApi();
  const session = useSession();
  const navigate = useNavigate();
  const t = useT();
  const [isOpen, setIsOpen] = useState(false);
  const [password, setPassword] = useState("");
  const mutation = useMutation({
    ...deleteOwnAccountMutation(api),
    onSuccess: async () => {
      setIsOpen(false);
      setPassword("");
      // The server has ended every session and token of the account; this forgets them in every tab.
      await session.signOut().catch(() => undefined);
      await navigate({ to: "/login" });
    },
  });
  return {
    isOpen,
    open: () => {
      mutation.reset();
      setPassword("");
      setIsOpen(true);
    },
    cancel: () => {
      setIsOpen(false);
      mutation.reset();
    },
    password,
    changePassword: setPassword,
    submit: () => {
      if (!mutation.isPending && password !== "") mutation.mutate(password);
    },
    isPending: mutation.isPending,
    error: !mutation.isError
      ? undefined
      : isWrongPasswordError(mutation.error)
        ? t("client:settings.account.delete.wrong_password")
        : errorMessage(mutation.error),
  };
}
