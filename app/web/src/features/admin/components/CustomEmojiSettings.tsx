// SPDX-License-Identifier: AGPL-3.0-only
// The custom emoji screen below its heading (ADR-0051, manage_custom_emojis): the form that registers a local emoji
// with an image file, then the emoji of this server or of other servers (tabs kept in the URL by the route,
// web-client.md S3), newest first, with a search. A local emoji's settings change in a dialog, its image is replaced
// by choosing another file, and deleting it asks first in an alert dialog (it cannot be undone); a remote emoji is
// only enabled or disabled. What a change did stays above the list in a StatusMessage (role="status"), and a
// failure stays there too (role="alert").
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
  Tab,
  TabList,
  TabPanel,
  Tabs,
  TextField,
} from "@my/ui";
import { type SubmitEvent, useId, useRef, useState } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import { DateTime, DetailSection, DomainName, RecordList, RequirePermission } from "../../moderation";
import {
  acceptedEmojiTypes,
  type AdminCustomEmoji,
  categoryLimit,
  type CustomEmojiFilter,
  type CustomEmojiOrigin,
  customEmojiOrigins,
  federatedEmojiBytes,
  maxAliases,
  shortcodePattern,
} from "../api/customEmojis";
import { type CustomEmojiActions, emojiInText, parseAliases, useAdminCustomEmojis, useCustomEmojiActions, useCustomEmojiForm } from "../hooks/useCustomEmojis";

const originLabels: Readonly<Record<CustomEmojiOrigin, ClientMessageKey>> = {
  local: "client:admin.custom_emojis.filter.local",
  remote: "client:admin.custom_emojis.filter.remote",
};

function isOrigin(key: unknown): key is CustomEmojiOrigin {
  return customEmojiOrigins.some((origin) => origin === key);
}

export type CustomEmojiSettingsProps = {
  /** The list's filter, from the URL. */
  filter: CustomEmojiFilter;
  /** Shows another filter: the route puts it into the URL. */
  onFilterChange: (filter: CustomEmojiFilter) => void;
};

export function CustomEmojiSettings(props: CustomEmojiSettingsProps) {
  const t = useT();
  return (
    <RequirePermission permission="manage_custom_emojis">
      <div className="flex flex-col gap-8">
        <DetailSection title={t("client:admin.custom_emojis.new.title")}>
          <NewCustomEmojiForm />
        </DetailSection>
        <DetailSection title={t("client:admin.custom_emojis.list.title")}>
          <CustomEmojiTabs {...props} />
        </DetailSection>
      </div>
    </RequirePermission>
  );
}

function NewCustomEmojiForm() {
  const t = useT();
  const form = useCustomEmojiForm();
  const imageId = useId();
  const imageErrorId = useId();
  const { draft } = form;
  const aliasCount = parseAliases(draft.aliases).length;
  return (
    <form onSubmit={form.submit} className="flex flex-col gap-6">
      <div role="group" aria-labelledby={imageId} className="flex flex-col gap-2">
        <span id={imageId} className="text-sm font-medium text-fg">
          {t("client:admin.custom_emojis.image")}
        </span>
        <p className="text-sm text-fg-muted">{t("client:admin.custom_emojis.image.description")}</p>
        <div>
          <FileTrigger
            acceptedFileTypes={acceptedEmojiTypes}
            onSelect={(files) => {
              const file = files?.[0];
              if (file !== undefined) form.change({ file });
            }}
          >
            <Button size="sm" aria-describedby={form.missingImage ? `${imageId} ${imageErrorId}` : imageId}>
              {t("client:admin.custom_emojis.image.choose")}
            </Button>
          </FileTrigger>
        </div>
        {draft.file !== null && (
          <p className="text-sm wrap-anywhere">
            {t("client:admin.custom_emojis.image.chosen", { name: draft.file.name })}
          </p>
        )}
        {draft.file !== null && draft.file.size > federatedEmojiBytes && <p className="text-sm text-warning">{t("client:admin.custom_emojis.image.large_hint")}</p>}
        {form.missingImage && (
          <p id={imageErrorId} role="alert" className="text-sm text-danger">
            {t("client:admin.custom_emojis.image.required")}
          </p>
        )}
      </div>
      <TextField
        label={t("client:admin.custom_emojis.shortcode")}
        description={t("client:admin.custom_emojis.shortcode.description")}
        value={draft.shortcode}
        onChange={(shortcode) => {
          form.change({ shortcode });
        }}
        isRequired
        pattern={shortcodePattern}
        maxLength={30}
        autoComplete="off"
        spellCheck="false"
        errorMessage={t("client:admin.custom_emojis.shortcode.invalid")}
      />
      <TextField
        label={t("client:admin.custom_emojis.category")}
        description={t("client:admin.custom_emojis.category.description", { max: String(categoryLimit) })}
        value={draft.category}
        onChange={(category) => {
          form.change({ category });
        }}
        maxLength={categoryLimit}
        autoComplete="off"
      />
      <TextField
        label={t("client:admin.custom_emojis.aliases")}
        description={t("client:admin.custom_emojis.aliases.description", { max: String(maxAliases) })}
        value={draft.aliases}
        onChange={(aliases) => {
          form.change({ aliases });
        }}
        {...(aliasCount > maxAliases && { isInvalid: true, errorMessage: t("client:admin.custom_emojis.aliases.too_many", { max: String(maxAliases) }) })}
        autoComplete="off"
        spellCheck="false"
      />
      <Checkbox
        isSelected={draft.visibleInPicker}
        onChange={(visibleInPicker) => {
          form.change({ visibleInPicker });
        }}
        description={t("client:admin.custom_emojis.visible_in_picker.description")}
      >
        {t("client:admin.custom_emojis.visible_in_picker")}
      </Checkbox>
      {form.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.custom_emojis.create_failed", { reason: form.error })}
        </p>
      )}
      <div>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:admin.custom_emojis.submit")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
    </form>
  );
}

function CustomEmojiTabs({ filter, onFilterChange }: CustomEmojiSettingsProps) {
  const t = useT();
  return (
    <Tabs
      selectedKey={filter.origin}
      onSelectionChange={(key) => {
        if (isOrigin(key)) onFilterChange({ origin: key });
      }}
    >
      <TabList aria-label={t("client:admin.custom_emojis.filter.label")}>
        {customEmojiOrigins.map((origin) => (
          <Tab key={origin} id={origin}>
            {t(originLabels[origin])}
          </Tab>
        ))}
      </TabList>
      <TabPanel id={filter.origin} className="flex flex-col gap-4 pt-3">
        <SearchForm key={`${filter.origin}:${filter.shortcode ?? ""}:${filter.domain ?? ""}`} filter={filter} onFilterChange={onFilterChange} />
        <CustomEmojiList filter={filter} />
      </TabPanel>
    </Tabs>
  );
}

/** The search of the list: the shortcode, and for other servers the domain. Submitting puts it into the URL. */
function SearchForm({ filter, onFilterChange }: CustomEmojiSettingsProps) {
  const t = useT();
  const [shortcode, setShortcode] = useState(filter.shortcode ?? "");
  const [domain, setDomain] = useState(filter.domain ?? "");
  const searching = filter.shortcode !== undefined || filter.domain !== undefined;
  const onSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    const code = shortcode.trim().replace(/^:+|:+$/gu, "");
    const host = domain.trim();
    onFilterChange({
      origin: filter.origin,
      ...(code === "" ? {} : { shortcode: code }),
      ...(filter.origin === "remote" && host !== "" ? { domain: host } : {}),
    });
  };
  return (
    <form role="search" aria-label={t("client:admin.custom_emojis.search.label")} onSubmit={onSubmit} className="flex flex-wrap items-end gap-2">
      <TextField
        type="search"
        label={t("client:admin.custom_emojis.search.shortcode")}
        value={shortcode}
        onChange={setShortcode}
        maxLength={128}
        autoComplete="off"
        spellCheck="false"
      />
      {filter.origin === "remote" && (
        <TextField type="search" label={t("client:admin.custom_emojis.search.domain")} value={domain} onChange={setDomain} maxLength={253} autoComplete="off" spellCheck="false" />
      )}
      <Button type="submit">{t("client:admin.custom_emojis.search.submit")}</Button>
      {searching && (
        <Button
          variant="ghost"
          onPress={() => {
            onFilterChange({ origin: filter.origin });
          }}
        >
          {t("client:admin.custom_emojis.search.clear")}
        </Button>
      )}
    </form>
  );
}

function CustomEmojiList({ filter }: { filter: CustomEmojiFilter }) {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const list = useAdminCustomEmojis(filter);
  const removed = useFocusAfterRemoval(
    listRef,
    list.items.map((emoji) => emoji.id),
  );
  const actions = useCustomEmojiActions(removed);
  const searching = filter.shortcode !== undefined || filter.domain !== undefined;
  const empty = searching ? (
    <EmptyState headingLevel={3} heading={t("client:admin.custom_emojis.empty.search")} />
  ) : filter.origin === "local" ? (
    <EmptyState headingLevel={3} heading={t("client:admin.custom_emojis.empty.local")} description={t("client:admin.custom_emojis.empty.local.description")} />
  ) : (
    <EmptyState headingLevel={3} heading={t("client:admin.custom_emojis.empty.remote")} />
  );
  return (
    <div className="flex flex-col gap-3">
      <StatusMessage>{actions.status}</StatusMessage>
      {actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.custom_emojis.change_failed", { reason: actions.error })}
        </p>
      )}
      <RecordList list={list} getId={(emoji) => emoji.id} listRef={listRef} empty={empty}>
        {(emoji) => <CustomEmojiRow emoji={emoji} actions={actions} />}
      </RecordList>
    </div>
  );
}

function CustomEmojiRow({ emoji, actions }: { emoji: AdminCustomEmoji; actions: CustomEmojiActions }) {
  const t = useT();
  const name = emojiInText(emoji);
  const heading = `:${emoji.shortcode}:`;
  const isPending = actions.pendingId === emoji.id;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-3">
        {/* The heading names the emoji; the image beside it repeats it. */}
        <span aria-hidden="true" className="flex size-12 shrink-0 items-center justify-center rounded-md bg-surface-sunken text-3xl">
          {emoji.url !== undefined && emoji.static_url !== undefined && <CustomEmoji emoji={{ shortcode: emoji.shortcode, url: emoji.url, static_url: emoji.static_url }} />}
        </span>
        <h3 className="min-w-0 font-mono font-semibold wrap-anywhere">
          <span dir="ltr">{heading}</span>
        </h3>
      </div>
      <DescriptionList columns={2}>
        {emoji.domain !== undefined && (
          <DescriptionItem term={t("client:admin.custom_emojis.domain")}>
            <DomainName domain={emoji.domain} />
          </DescriptionItem>
        )}
        <DescriptionItem term={t("client:admin.custom_emojis.state")}>
          {t(emoji.enabled ? "client:admin.custom_emojis.state.enabled" : "client:admin.custom_emojis.state.disabled")}
        </DescriptionItem>
        {emoji.local && (
          <>
            <DescriptionItem term={t("client:admin.custom_emojis.picker")}>
              {t(emoji.visible_in_picker ? "client:admin.custom_emojis.picker.shown" : "client:admin.custom_emojis.picker.hidden")}
            </DescriptionItem>
            <DescriptionItem term={t("client:admin.custom_emojis.category")}>
              {emoji.category === undefined ? t("client:admin.custom_emojis.none") : <span dir="auto">{emoji.category}</span>}
            </DescriptionItem>
            <DescriptionItem term={t("client:admin.custom_emojis.aliases")}>
              {emoji.aliases.length === 0 ? t("client:admin.custom_emojis.none") : <span dir="ltr">{emoji.aliases.join(", ")}</span>}
            </DescriptionItem>
          </>
        )}
        {emoji.url === undefined && <DescriptionItem term={t("client:admin.custom_emojis.image")}>{t("client:admin.custom_emojis.no_image")}</DescriptionItem>}
        <DescriptionItem term={t("client:admin.custom_emojis.updated_at")}>
          <DateTime value={emoji.updated_at} />
        </DescriptionItem>
      </DescriptionList>
      <div className="flex flex-wrap gap-2">
        {emoji.local ? (
          <>
            <EditCustomEmoji emoji={emoji} actions={actions} isPending={isPending} />
            <FileTrigger
              acceptedFileTypes={acceptedEmojiTypes}
              onSelect={(files) => {
                const file = files?.[0];
                if (file !== undefined) actions.replaceImage(emoji, file);
              }}
            >
              <Button size="sm" isPending={isPending} aria-label={t("client:admin.custom_emojis.replace_image_of", { emoji: name })}>
                {t("client:admin.custom_emojis.replace_image")}
              </Button>
            </FileTrigger>
            <DeleteCustomEmoji emoji={emoji} actions={actions} isPending={isPending} />
          </>
        ) : (
          <Button
            size="sm"
            isPending={isPending}
            aria-label={t(emoji.enabled ? "client:admin.custom_emojis.disable_emoji" : "client:admin.custom_emojis.enable_emoji", { emoji: name })}
            onPress={() => {
              actions.setEnabled(emoji, !emoji.enabled);
            }}
          >
            {t(emoji.enabled ? "client:admin.custom_emojis.disable" : "client:admin.custom_emojis.enable")}
          </Button>
        )}
      </div>
    </div>
  );
}

type RowActionProps = { emoji: AdminCustomEmoji; actions: CustomEmojiActions; isPending: boolean };

/** The settings of a local emoji in a dialog; it closes once they are saved, and keeps a failure inside. */
function EditCustomEmoji({ emoji, actions, isPending }: RowActionProps) {
  const t = useT();
  const name = emojiInText(emoji);
  return (
    <DialogTrigger>
      <Button size="sm" isDisabled={isPending} aria-label={t("client:admin.custom_emojis.edit_emoji", { emoji: name })}>
        {t("client:admin.custom_emojis.edit")}
      </Button>
      <Dialog title={t("client:admin.custom_emojis.edit.title", { emoji: name })} closeLabel={t("client:admin.custom_emojis.edit.close")} size="md">
        {({ close }) => <EditCustomEmojiForm emoji={emoji} actions={actions} close={close} />}
      </Dialog>
    </DialogTrigger>
  );
}

function EditCustomEmojiForm({ emoji, actions, close }: { emoji: AdminCustomEmoji; actions: CustomEmojiActions; close: () => void }) {
  const t = useT();
  const [category, setCategory] = useState(emoji.category ?? "");
  const [aliases, setAliases] = useState(emoji.aliases.join(" "));
  const [visibleInPicker, setVisibleInPicker] = useState(emoji.visible_in_picker);
  const [enabled, setEnabled] = useState(emoji.enabled);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState(false);
  const aliasCount = parseAliases(aliases).length;
  const onSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSaving(true);
    setError(false);
    void actions.update(emoji, { category: category.trim(), aliases: parseAliases(aliases), visible_in_picker: visibleInPicker, enabled }).then((saved) => {
      setIsSaving(false);
      if (saved) close();
      else setError(true);
    });
  };
  return (
    <form onSubmit={onSubmit} className="flex flex-col gap-4">
      <TextField
        label={t("client:admin.custom_emojis.category")}
        description={t("client:admin.custom_emojis.category.description", { max: String(categoryLimit) })}
        value={category}
        onChange={setCategory}
        maxLength={categoryLimit}
        autoComplete="off"
        autoFocus
      />
      <TextField
        label={t("client:admin.custom_emojis.aliases")}
        description={t("client:admin.custom_emojis.aliases.description", { max: String(maxAliases) })}
        value={aliases}
        onChange={setAliases}
        {...(aliasCount > maxAliases && { isInvalid: true, errorMessage: t("client:admin.custom_emojis.aliases.too_many", { max: String(maxAliases) }) })}
        autoComplete="off"
        spellCheck="false"
      />
      <Checkbox isSelected={visibleInPicker} onChange={setVisibleInPicker} description={t("client:admin.custom_emojis.visible_in_picker.description")}>
        {t("client:admin.custom_emojis.visible_in_picker")}
      </Checkbox>
      <Checkbox isSelected={enabled} onChange={setEnabled} description={t("client:admin.custom_emojis.enabled.description")}>
        {t("client:admin.custom_emojis.enabled")}
      </Checkbox>
      {error && actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.custom_emojis.change_failed", { reason: actions.error })}
        </p>
      )}
      <div className="flex flex-wrap justify-end gap-2">
        <Button onPress={close}>{t("client:admin.custom_emojis.cancel")}</Button>
        <Button type="submit" variant="primary" isPending={isSaving}>
          {t("client:admin.custom_emojis.save")}
        </Button>
      </div>
    </form>
  );
}

/** Deleting a local emoji, after an alert dialog: it cannot be undone. */
function DeleteCustomEmoji({ emoji, actions, isPending }: RowActionProps) {
  const t = useT();
  const name = emojiInText(emoji);
  const [isDeleting, setIsDeleting] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <DialogTrigger>
      <Button size="sm" variant="danger" isDisabled={isPending} aria-label={t("client:admin.custom_emojis.delete_emoji", { emoji: name })}>
        {t("client:admin.custom_emojis.delete")}
      </Button>
      <ConfirmDialog
        title={t("client:admin.custom_emojis.delete.title", { emoji: name })}
        description={t("client:admin.custom_emojis.delete.description", { emoji: name })}
        confirmLabel={t("client:admin.custom_emojis.delete")}
        cancelLabel={t("client:admin.custom_emojis.cancel")}
        isPending={isDeleting}
        {...(failed && actions.error !== undefined && { errorMessage: t("client:admin.custom_emojis.change_failed", { reason: actions.error }) })}
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
