// SPDX-License-Identifier: AGPL-3.0-only
// The user's page of their custom emoji (`/settings/custom-emojis`, ADR-0070 §1–§4, create_custom_emojis), below its
// heading: three regions. "Request a custom emoji" says how many emoji the user has against their limit and holds the
// form of a request (an image file and a shortcode); an administrator approves it before the emoji exists. "Your
// requests" lists the requests in every state, newest first, with the reason of a rejection and withdrawing a pending
// one. "Your custom emoji" lists the emoji the user made: their settings change in a dialog, a new image is requested
// by choosing another file, and deleting one asks first in an alert dialog (it cannot be undone). What a change did
// stays in a StatusMessage (role="status") of its region, and a failure stays there too (role="alert").
import {
  Button,
  Checkbox,
  ConfirmDialog,
  CustomEmoji,
  DescriptionItem,
  DescriptionList,
  Dialog,
  DialogTrigger,
  EmptyState,
  FileTrigger,
  StatusMessage,
  TextField,
} from "@my/ui";
import { type SubmitEvent, useId, useRef, useState } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import { acceptedEmojiTypes, categoryLimit, federatedEmojiBytes, maxAliases, parseAliases, requestStateLabels, shortcodePattern } from "../../admin";
import { DateTime, DetailSection, RecordList, RequirePermission } from "../../moderation";
import type { CustomEmojiRequest, OwnCustomEmoji } from "../api/ownEmojis";
import {
  type CustomEmojiRequestActions,
  emojiInText,
  type OwnCustomEmojiActions,
  useCustomEmojiRequestActions,
  useCustomEmojiRequestForm,
  useOwnCustomEmojiActions,
  useOwnCustomEmojiRequests,
  useOwnCustomEmojis,
} from "../hooks/useOwnCustomEmojis";

/** What each kind of request asks for. */
const kindLabels: Readonly<Record<CustomEmojiRequest["kind"], ClientMessageKey>> = {
  create: "client:custom_emoji_requests.kind.create",
  image: "client:custom_emoji_requests.kind.image",
};

export function OwnCustomEmojis() {
  const t = useT();
  return (
    <RequirePermission permission="create_custom_emojis">
      <div className="flex flex-col gap-8">
        <p className="text-fg-muted">{t("client:custom_emojis.description")}</p>
        <DetailSection title={t("client:custom_emojis.new.title")}>
          <Usage />
          <RequestForm />
        </DetailSection>
        <DetailSection title={t("client:custom_emojis.requests.title")}>
          <RequestList />
        </DetailSection>
        <DetailSection title={t("client:custom_emojis.list.title")}>
          <EmojiList />
        </DetailSection>
      </div>
    </RequirePermission>
  );
}

/** How many emoji the user has (with their pending requests to create one) against their limit. */
function Usage() {
  const t = useT();
  const { used, limit } = useOwnCustomEmojis();
  return (
    <p className="text-sm text-fg-muted">
      {limit === undefined ? t("client:custom_emojis.usage.unlimited", { used }) : t("client:custom_emojis.usage", { used, limit: String(limit) })}
    </p>
  );
}

function RequestForm() {
  const t = useT();
  const form = useCustomEmojiRequestForm();
  const imageId = useId();
  const imageErrorId = useId();
  const { draft } = form;
  return (
    <form onSubmit={form.submit} className="flex flex-col gap-6">
      <div role="group" aria-labelledby={imageId} className="flex flex-col gap-2">
        <span id={imageId} className="text-sm font-medium text-fg">
          {t("client:custom_emojis.image")}
        </span>
        <p className="text-sm text-fg-muted">{t("client:custom_emojis.image.description")}</p>
        <div>
          <FileTrigger
            acceptedFileTypes={acceptedEmojiTypes}
            onSelect={(files) => {
              const file = files?.[0];
              if (file !== undefined) form.change({ file });
            }}
          >
            <Button size="sm" aria-describedby={form.missingImage ? `${imageId} ${imageErrorId}` : imageId}>
              {t("client:custom_emojis.image.choose")}
            </Button>
          </FileTrigger>
        </div>
        {draft.file !== null && <p className="text-sm wrap-anywhere">{t("client:custom_emojis.image.chosen", { name: draft.file.name })}</p>}
        {draft.file !== null && draft.file.size > federatedEmojiBytes && <p className="text-sm text-warning">{t("client:custom_emojis.image.large_hint")}</p>}
        {form.missingImage && (
          <p id={imageErrorId} role="alert" className="text-sm text-danger">
            {t("client:custom_emojis.image.required")}
          </p>
        )}
      </div>
      <TextField
        label={t("client:custom_emojis.shortcode")}
        description={t("client:custom_emojis.shortcode.description")}
        value={draft.shortcode}
        onChange={(shortcode) => {
          form.change({ shortcode });
        }}
        isRequired
        pattern={shortcodePattern}
        maxLength={30}
        autoComplete="off"
        spellCheck="false"
        errorMessage={t("client:custom_emojis.shortcode.invalid")}
      />
      {form.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:custom_emojis.request_failed", { reason: form.error })}
        </p>
      )}
      <div>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:custom_emojis.submit")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
    </form>
  );
}

function RequestList() {
  const t = useT();
  const list = useOwnCustomEmojiRequests();
  const actions = useCustomEmojiRequestActions();
  return (
    <div className="flex flex-col gap-3">
      <StatusMessage>{actions.status}</StatusMessage>
      {actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:custom_emojis.withdraw_failed", { reason: actions.error })}
        </p>
      )}
      <RecordList list={list} getId={(request) => request.id} empty={<EmptyState headingLevel={3} heading={t("client:custom_emojis.requests.empty")} />}>
        {(request) => <RequestRow request={request} actions={actions} />}
      </RecordList>
    </div>
  );
}

/** The image of an emoji or a request beside its heading, which names it. */
function EmojiImage({ shortcode, url, staticUrl }: { shortcode: string; url: string | undefined; staticUrl: string | undefined }) {
  return (
    <span aria-hidden="true" className="flex size-12 shrink-0 items-center justify-center rounded-md bg-surface-sunken text-3xl">
      {url !== undefined && staticUrl !== undefined && <CustomEmoji emoji={{ shortcode, url, static_url: staticUrl }} />}
    </span>
  );
}

function RequestRow({ request, actions }: { request: CustomEmojiRequest; actions: CustomEmojiRequestActions }) {
  const t = useT();
  const heading = `:${request.shortcode}:`;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-3">
        <EmojiImage shortcode={request.shortcode} url={request.url} staticUrl={request.static_url} />
        <h3 className="min-w-0 font-mono font-semibold wrap-anywhere">
          <span dir="ltr">{heading}</span>
        </h3>
      </div>
      <DescriptionList columns={2}>
        <DescriptionItem term={t("client:custom_emoji_requests.kind")}>{t(kindLabels[request.kind])}</DescriptionItem>
        <DescriptionItem term={t("client:custom_emoji_requests.state")}>{t(requestStateLabels[request.state])}</DescriptionItem>
        <DescriptionItem term={t("client:custom_emoji_requests.created_at")}>
          <DateTime value={request.created_at} />
        </DescriptionItem>
        {request.decided_at !== undefined && (
          <DescriptionItem term={t("client:custom_emoji_requests.decided_at")}>
            <DateTime value={request.decided_at} />
          </DescriptionItem>
        )}
        {request.reason !== undefined && (
          <DescriptionItem term={t("client:custom_emoji_requests.reason")}>
            <span dir="auto" className="whitespace-pre-line">
              {request.reason}
            </span>
          </DescriptionItem>
        )}
      </DescriptionList>
      {request.state === "pending" && (
        <div>
          <Button
            size="sm"
            isPending={actions.pendingId === request.id}
            aria-label={t("client:custom_emojis.withdraw_request", { emoji: emojiInText(request) })}
            onPress={() => {
              actions.withdraw(request);
            }}
          >
            {t("client:custom_emojis.withdraw")}
          </Button>
        </div>
      )}
    </div>
  );
}

function EmojiList() {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const list = useOwnCustomEmojis();
  const removed = useFocusAfterRemoval(
    listRef,
    list.items.map((emoji) => emoji.id),
  );
  const actions = useOwnCustomEmojiActions(removed);
  return (
    <div className="flex flex-col gap-3">
      <StatusMessage>{actions.status}</StatusMessage>
      {actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:custom_emojis.change_failed", { reason: actions.error })}
        </p>
      )}
      <RecordList
        list={list}
        getId={(emoji) => emoji.id}
        listRef={listRef}
        empty={<EmptyState headingLevel={3} heading={t("client:custom_emojis.empty")} description={t("client:custom_emojis.empty.description")} />}
      >
        {(emoji) => <EmojiRow emoji={emoji} actions={actions} />}
      </RecordList>
    </div>
  );
}

function EmojiRow({ emoji, actions }: { emoji: OwnCustomEmoji; actions: OwnCustomEmojiActions }) {
  const t = useT();
  const name = emojiInText(emoji);
  const heading = `:${emoji.shortcode}:`;
  const isPending = actions.pendingId === emoji.id;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-3">
        <EmojiImage shortcode={emoji.shortcode} url={emoji.url} staticUrl={emoji.static_url} />
        <h3 className="min-w-0 font-mono font-semibold wrap-anywhere">
          <span dir="ltr">{heading}</span>
        </h3>
      </div>
      <DescriptionList columns={2}>
        <DescriptionItem term={t("client:custom_emojis.state")}>{t(emoji.enabled ? "client:custom_emojis.state.enabled" : "client:custom_emojis.state.disabled")}</DescriptionItem>
        <DescriptionItem term={t("client:custom_emojis.picker")}>
          {t(emoji.visible_in_picker ? "client:custom_emojis.picker.shown" : "client:custom_emojis.picker.hidden")}
        </DescriptionItem>
        <DescriptionItem term={t("client:custom_emojis.category")}>
          {emoji.category === undefined ? t("client:custom_emojis.none") : <span dir="auto">{emoji.category}</span>}
        </DescriptionItem>
        <DescriptionItem term={t("client:custom_emojis.aliases")}>
          {emoji.aliases.length === 0 ? t("client:custom_emojis.none") : <span dir="ltr">{emoji.aliases.join(", ")}</span>}
        </DescriptionItem>
        <DescriptionItem term={t("client:custom_emojis.updated_at")}>
          <DateTime value={emoji.updated_at} />
        </DescriptionItem>
      </DescriptionList>
      <div className="flex flex-wrap gap-2">
        <EditEmoji emoji={emoji} actions={actions} isPending={isPending} />
        <FileTrigger
          acceptedFileTypes={acceptedEmojiTypes}
          onSelect={(files) => {
            const file = files?.[0];
            if (file !== undefined) actions.requestImage(emoji, file);
          }}
        >
          <Button size="sm" isPending={isPending} aria-label={t("client:custom_emojis.request_image_of", { emoji: name })}>
            {t("client:custom_emojis.request_image")}
          </Button>
        </FileTrigger>
        <DeleteEmoji emoji={emoji} actions={actions} isPending={isPending} />
      </div>
    </div>
  );
}

type RowActionProps = { emoji: OwnCustomEmoji; actions: OwnCustomEmojiActions; isPending: boolean };

/** The settings of an emoji in a dialog; it closes once they are saved, and keeps a failure inside. */
function EditEmoji({ emoji, actions, isPending }: RowActionProps) {
  const t = useT();
  const name = emojiInText(emoji);
  return (
    <DialogTrigger>
      <Button size="sm" isDisabled={isPending} aria-label={t("client:custom_emojis.edit_emoji", { emoji: name })}>
        {t("client:custom_emojis.edit")}
      </Button>
      <Dialog title={t("client:custom_emojis.edit.title", { emoji: name })} closeLabel={t("client:custom_emojis.edit.close")} isDismissable={false} size="md">
        {({ close }) => <EditEmojiForm emoji={emoji} actions={actions} close={close} />}
      </Dialog>
    </DialogTrigger>
  );
}

function EditEmojiForm({ emoji, actions, close }: { emoji: OwnCustomEmoji; actions: OwnCustomEmojiActions; close: () => void }) {
  const t = useT();
  const [category, setCategory] = useState(emoji.category ?? "");
  const [aliases, setAliases] = useState(emoji.aliases.join(" "));
  const [visibleInPicker, setVisibleInPicker] = useState(emoji.visible_in_picker);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState(false);
  const aliasCount = parseAliases(aliases).length;
  const onSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSaving(true);
    setError(false);
    void actions.update(emoji, { category: category.trim(), aliases: parseAliases(aliases), visible_in_picker: visibleInPicker }).then((saved) => {
      setIsSaving(false);
      if (saved) close();
      else setError(true);
    });
  };
  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-4">
      <TextField
        label={t("client:custom_emojis.category")}
        description={t("client:custom_emojis.category.description", { max: String(categoryLimit) })}
        value={category}
        onChange={setCategory}
        maxLength={categoryLimit}
        autoComplete="off"
        autoFocus
      />
      <TextField
        label={t("client:custom_emojis.aliases")}
        description={t("client:custom_emojis.aliases.description", { max: String(maxAliases) })}
        value={aliases}
        onChange={setAliases}
        {...(aliasCount > maxAliases && { isInvalid: true, errorMessage: t("client:custom_emojis.aliases.too_many", { max: String(maxAliases) }) })}
        autoComplete="off"
        spellCheck="false"
      />
      <Checkbox isSelected={visibleInPicker} onChange={setVisibleInPicker} description={t("client:custom_emojis.visible_in_picker.description")}>
        {t("client:custom_emojis.visible_in_picker")}
      </Checkbox>
      {error && actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:custom_emojis.change_failed", { reason: actions.error })}
        </p>
      )}
      <div className="flex flex-wrap justify-end gap-2">
        <Button onPress={close}>{t("client:custom_emojis.cancel")}</Button>
        <Button type="submit" variant="primary" isPending={isSaving}>
          {t("client:custom_emojis.save")}
        </Button>
      </div>
    </form>
  );
}

/** Deleting an emoji, after an alert dialog: it cannot be undone. */
function DeleteEmoji({ emoji, actions, isPending }: RowActionProps) {
  const t = useT();
  const name = emojiInText(emoji);
  const [isDeleting, setIsDeleting] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <DialogTrigger>
      <Button size="sm" variant="danger" isDisabled={isPending} aria-label={t("client:custom_emojis.delete_emoji", { emoji: name })}>
        {t("client:custom_emojis.delete")}
      </Button>
      <ConfirmDialog
        title={t("client:custom_emojis.delete.title", { emoji: name })}
        description={t("client:custom_emojis.delete.description", { emoji: name })}
        confirmLabel={t("client:custom_emojis.delete")}
        cancelLabel={t("client:custom_emojis.cancel")}
        isPending={isDeleting}
        {...(failed && actions.error !== undefined && { errorMessage: t("client:custom_emojis.change_failed", { reason: actions.error }) })}
        onConfirm={(close) => {
          setIsDeleting(true);
          setFailed(false);
          void actions.remove(emoji).then((deleted) => {
            setIsDeleting(false);
            if (deleted) close();
            else setFailed(true);
          });
        }}
      />
    </DialogTrigger>
  );
}
