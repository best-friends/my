// SPDX-License-Identifier: AGPL-3.0-only
import { useToast } from "@my/ui";
import { useMutation, useQueryClient, useSuspenseQuery } from "@tanstack/react-query";
import { type SyntheticEvent, useState } from "react";
import { useApi } from "../../../lib/auth/SessionProvider";
import { errorMessage } from "../../../lib/errors";
import { useT } from "../../../lib/i18n";
import { type CredentialAccount, credentialsQueryOptions, type ProfileChange } from "../api/me";
import { type MediaAttachment, MediaProcessingError, uploadImage } from "../api/media";

/** The limits of a profile (UpdateCredentialsRequest in api/openapi.yaml), counted in code points as the server does. */
export const profileLimits = { displayName: 30, note: 500, fields: 4, fieldName: 255, fieldValue: 255 } as const;

/** The number of characters of `text` as the server counts them: Unicode code points. */
export function characterCount(text: string): number {
  return Array.from(text).length;
}

/** One metadata field being edited; `key` identifies its row while rows are added and removed. */
export type FieldDraft = { key: number; name: string; value: string };

/** The avatar or the header image being edited. */
export type ImageControl = {
  /** The image to show now: the saved one, the new upload, or none. */
  url: string | undefined;
  /** Whether there is an image that can be removed. */
  hasImage: boolean;
  /** Uploads `file` to replace the image when the profile is saved. */
  choose: (file: File) => void;
  /** Removes the image when the profile is saved. */
  remove: () => void;
  isUploading: boolean;
  /** Why the upload failed, localized. */
  error: string | undefined;
};

type ImageChange = { kind: "keep" } | { kind: "new"; media: MediaAttachment } | { kind: "remove" };

/**
 * The profile form (FR-006): its fields, and saving them in one change of the profile. The browser checks the
 * limits before submitting (React Aria's native validation), so `submit` runs only with valid values.
 * Mount it again (with a `key`) to start over from a newer saved profile.
 */
export function useProfileForm() {
  const api = useApi();
  const queryClient = useQueryClient();
  const toast = useToast();
  const t = useT();
  const { data: account } = useSuspenseQuery(credentialsQueryOptions(api));

  const [displayName, setDisplayName] = useState(account.display_name);
  const [note, setNote] = useState(account.source.note);
  const [fields, setFields] = useState<FieldDraft[]>(() => account.source.fields.map((field, key) => ({ key, ...field })));
  const [nextFieldKey, setNextFieldKey] = useState(account.source.fields.length);
  const [locked, setLocked] = useState(account.locked);
  const [bot, setBot] = useState(account.bot);
  const avatar = useImageControl(account.avatar, "thumbnail");
  const header = useImageControl(account.header, "full");

  const save = api.query.useMutation("patch", "/accounts/update_credentials", {
    onSuccess: (saved) => {
      queryClient.setQueryData(credentialsQueryOptions(api).queryKey, saved);
      toast.show({ message: t("client:settings.profile.saved"), tone: "success" });
    },
  });

  const change = (): ProfileChange => ({
    display_name: displayName,
    note,
    // A row left empty is not a field.
    fields: fields.filter((field) => field.name !== "" || field.value !== "").map(({ name, value }) => ({ name, value })),
    locked,
    bot,
    ...avatar.change("avatar_media_id"),
    ...header.change("header_media_id"),
  });

  return {
    account,
    displayName: { value: displayName, onChange: setDisplayName },
    note: { value: note, onChange: setNote },
    fields: {
      rows: fields,
      canAdd: fields.length < profileLimits.fields,
      add: () => {
        setFields([...fields, { key: nextFieldKey, name: "", value: "" }]);
        setNextFieldKey(nextFieldKey + 1);
      },
      update: (key: number, change: Partial<Omit<FieldDraft, "key">>) => {
        setFields(fields.map((field) => (field.key === key ? { ...field, ...change } : field)));
      },
      remove: (key: number) => {
        setFields(fields.filter((field) => field.key !== key));
      },
    },
    locked: { isSelected: locked, onChange: setLocked },
    bot: { isSelected: bot, onChange: setBot },
    avatar: avatar.control,
    header: header.control,
    submit: (event: SyntheticEvent<HTMLFormElement>) => {
      event.preventDefault();
      save.mutate({ body: change() });
    },
    isSaving: save.isPending,
    isBusy: save.isPending || avatar.control.isUploading || header.control.isUploading,
    /** Why the last save failed, localized; the server names the invalid property in it. */
    saveError: save.isError ? errorMessage(save.error) : undefined,
  };
}

/** The state of one image of the profile, and its upload. */
function useImageControl(savedUrl: CredentialAccount["avatar"], rendition: "thumbnail" | "full") {
  const api = useApi();
  const t = useT();
  const [image, setImage] = useState<ImageChange>({ kind: "keep" });
  const upload = useMutation({
    mutationFn: (file: File) => uploadImage(api, file),
    onSuccess: (media) => {
      setImage({ kind: "new", media });
    },
  });
  const url = image.kind === "new" ? image.media.variants?.[rendition].url : image.kind === "keep" ? savedUrl : undefined;
  const uploadError = upload.error instanceof MediaProcessingError ? t("client:settings.profile.image.failed") : errorMessage(upload.error);
  const control: ImageControl = {
    url,
    hasImage: url !== undefined,
    choose: (file) => {
      upload.mutate(file);
    },
    remove: () => {
      upload.reset();
      setImage({ kind: "remove" });
    },
    isUploading: upload.isPending,
    error: upload.isError ? uploadError : undefined,
  };
  return {
    control,
    /** The property of the profile change for this image: a new media ID, "" to remove it, or nothing. */
    change: <K extends "avatar_media_id" | "header_media_id">(property: K): Partial<Record<K, string>> => {
      if (image.kind === "new") return { [property]: image.media.id } as Partial<Record<K, string>>;
      if (image.kind === "remove") return { [property]: "" } as Partial<Record<K, string>>;
      return {};
    },
  };
}
