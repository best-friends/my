// SPDX-License-Identifier: AGPL-3.0-only
// The announcements screen below its heading (manage_announcements, FR-726): the form of a new announcement, then
// every announcement, newest first. The text is Markdown, which the server renders to sanitized HTML; the list
// shows each as the users see it, with its state in words. A new announcement shows for a chosen time from when it
// is sent, or until it is withdrawn, and is published at once or kept as a draft. Editing opens a dialog with the
// text; withdrawing cannot be undone, so it asks first (an alert dialog). What a change did stays on the page in a
// StatusMessage above the list; a failure stays there too (role="alert", web-client.md E2).
import {
  Button,
  Checkbox,
  ConfirmDialog,
  DescriptionItem,
  DescriptionList,
  Dialog,
  DialogTrigger,
  DurationSelect,
  EmptyState,
  PostContent,
  serverSanitizedHtml,
  StatusMessage,
  TextArea,
} from "@my/ui";
import { useRef, useState } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFocusHeadingAfterMove } from "../../../lib/useFocusAfterRemoval";
import { DateTime, DetailSection, RequirePermission } from "../../moderation";
import { type AdminAnnouncement, type AnnouncementState, announcementTextLimit } from "../api/announcements";
import {
  type AnnouncementActions,
  announcementDurations,
  announcementName,
  useAdminAnnouncements,
  useAnnouncementActions,
  useNewAnnouncementForm,
} from "../hooks/useAnnouncementSettings";

const stateLabels: Readonly<Record<AnnouncementState, ClientMessageKey>> = {
  draft: "client:admin.announcements.state.draft",
  published: "client:admin.announcements.state.published",
  withdrawn: "client:admin.announcements.state.withdrawn",
};

export function AnnouncementSettings() {
  const t = useT();
  return (
    <RequirePermission permission="manage_announcements">
      <div className="flex flex-col gap-8">
        <p className="text-fg-muted">{t("client:admin.announcements.description")}</p>
        <DetailSection title={t("client:admin.announcements.new.title")}>
          <NewAnnouncementForm />
        </DetailSection>
        <DetailSection title={t("client:admin.announcements.list.title")}>
          <AnnouncementList />
        </DetailSection>
      </div>
    </RequirePermission>
  );
}

/** The counter of a text field, in the viewer's language. */
function useFormatRemaining() {
  const t = useT();
  return (remaining: number) =>
    remaining < 0 ? t("client:admin.settings.characters_over", { count: -remaining }) : t("client:admin.settings.characters_left", { count: remaining });
}

function NewAnnouncementForm() {
  const t = useT();
  const form = useNewAnnouncementForm();
  const formatRemaining = useFormatRemaining();
  return (
    <form onSubmit={form.submit} className="flex flex-col gap-6">
      <TextArea
        label={t("client:admin.announcements.text")}
        description={t("client:admin.announcements.text.description")}
        value={form.text}
        onChange={form.changeText}
        rows={6}
        characterLimit={announcementTextLimit}
        formatRemaining={formatRemaining}
        {...(form.textError === undefined ? {} : { isInvalid: true, errorMessage: form.textError })}
      />
      <DurationSelect
        label={t("client:admin.announcements.duration")}
        description={t("client:admin.announcements.duration.description")}
        durations={announcementDurations}
        formatDuration={(seconds) => t("client:admin.settings.duration.days", { count: seconds / 86_400 })}
        indefiniteLabel={t("client:admin.announcements.until_withdrawn")}
        value={form.duration}
        onChange={form.changeDuration}
      />
      <Checkbox isSelected={form.publish} onChange={form.changePublish} description={t("client:admin.announcements.publish_now.description")}>
        {t("client:admin.announcements.publish_now")}
      </Checkbox>
      {form.failure !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.announcements.create_failed", { reason: form.failure })}
        </p>
      )}
      <div>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t(form.publish ? "client:admin.announcements.create_published" : "client:admin.announcements.create_draft")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
    </form>
  );
}

function AnnouncementList() {
  const t = useT();
  const announcements = useAdminAnnouncements();
  const listRef = useRef<HTMLUListElement>(null);
  const [status, setStatus] = useState<string | undefined>(undefined);
  // A change may take the pressed button off the row (publishing, withdrawing): focus moves to the row's heading.
  const changed = useFocusHeadingAfterMove(listRef);
  const actions = useAnnouncementActions((announcement, done) => {
    changed(announcement.id);
    setStatus(done);
  });
  return (
    <div className="flex flex-col gap-3">
      <StatusMessage>{status}</StatusMessage>
      {actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.announcements.change_failed", { reason: actions.error })}
        </p>
      )}
      {announcements.length === 0 ? (
        <EmptyState headingLevel={3} heading={t("client:admin.announcements.empty")} description={t("client:admin.announcements.empty.description")} />
      ) : (
        <ul ref={listRef} className="flex flex-col divide-y divide-border">
          {announcements.map((announcement) => (
            <li key={announcement.id} data-row-id={announcement.id} className="@container py-3">
              <AnnouncementRow announcement={announcement} actions={actions} />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

type AnnouncementRowProps = { announcement: AdminAnnouncement; actions: AnnouncementActions };

function AnnouncementRow({ announcement, actions }: AnnouncementRowProps) {
  const t = useT();
  const name = announcementName(announcement.text);
  return (
    <div className="flex flex-col gap-3">
      <h3 tabIndex={-1} className="font-semibold wrap-anywhere outline-none">
        {/* Written by an administrator in any language: U+2068 FIRST STRONG ISOLATE … U+2069 POP DIRECTIONAL ISOLATE. */}
        {t("client:admin.announcements.heading", { announcement: `⁨${name}⁩`, state: t(stateLabels[announcement.state]) })}
      </h3>
      <PostContent html={serverSanitizedHtml(announcement.content)} className="rounded-md border border-border p-3" />
      <DescriptionList>
        <DescriptionItem term={t("client:admin.announcements.state")}>{t(stateLabels[announcement.state])}</DescriptionItem>
        {announcement.published_at !== undefined && (
          <DescriptionItem term={t("client:admin.announcements.published_at")}>
            <DateTime value={announcement.published_at} />
          </DescriptionItem>
        )}
        {announcement.starts_at !== undefined && (
          <DescriptionItem term={t("client:admin.announcements.starts_at")}>
            <DateTime value={announcement.starts_at} />
          </DescriptionItem>
        )}
        <DescriptionItem term={t("client:admin.announcements.ends_at")}>
          {announcement.ends_at === undefined ? t("client:admin.announcements.until_withdrawn") : <DateTime value={announcement.ends_at} />}
        </DescriptionItem>
      </DescriptionList>
      {announcement.state !== "withdrawn" && (
        <div className="flex flex-wrap gap-2">
          {announcement.state === "draft" && <PublishAnnouncement announcement={announcement} name={name} actions={actions} />}
          <EditAnnouncement announcement={announcement} name={name} actions={actions} />
          <WithdrawAnnouncement announcement={announcement} name={name} actions={actions} />
        </div>
      )}
    </div>
  );
}

type AnnouncementActionProps = { announcement: AdminAnnouncement; name: string; actions: AnnouncementActions };

function PublishAnnouncement({ announcement, name, actions }: AnnouncementActionProps) {
  const t = useT();
  const [isPublishing, setIsPublishing] = useState(false);
  return (
    <Button
      size="sm"
      variant="primary"
      isPending={isPublishing}
      aria-label={t("client:admin.announcements.publish_one", { announcement: name })}
      onPress={() => {
        setIsPublishing(true);
        void actions.publish(announcement).then(() => {
          setIsPublishing(false);
        });
      }}
    >
      {t("client:admin.announcements.publish")}
    </Button>
  );
}

/** The text of an announcement in a dialog; it closes once it is saved, and keeps a failure inside. */
function EditAnnouncement({ announcement, name, actions }: AnnouncementActionProps) {
  const t = useT();
  return (
    <DialogTrigger>
      <Button size="sm" aria-label={t("client:admin.announcements.edit_one", { announcement: name })}>
        {t("client:admin.announcements.edit")}
      </Button>
      <Dialog title={t("client:admin.announcements.edit.title")} closeLabel={t("client:admin.announcements.edit.close")} size="md">
        {({ close }) => <EditAnnouncementForm announcement={announcement} actions={actions} close={close} />}
      </Dialog>
    </DialogTrigger>
  );
}

function EditAnnouncementForm({ announcement, actions, close }: { announcement: AdminAnnouncement; actions: AnnouncementActions; close: () => void }) {
  const t = useT();
  const formatRemaining = useFormatRemaining();
  const [text, setText] = useState(announcement.text);
  const [submitted, setSubmitted] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [failed, setFailed] = useState(false);
  const textError = submitted ? actions.textError(text) : undefined;
  return (
    <form
      onSubmit={(event) => {
        event.preventDefault();
        setSubmitted(true);
        if (isSaving || actions.textError(text) !== undefined) return;
        setIsSaving(true);
        setFailed(false);
        void actions.update(announcement, text).then((saved) => {
          setIsSaving(false);
          if (saved) close();
          else setFailed(true);
        });
      }}
      className="flex flex-col gap-4"
    >
      <TextArea
        label={t("client:admin.announcements.text")}
        description={t("client:admin.announcements.text.description")}
        value={text}
        onChange={setText}
        rows={8}
        autoFocus
        characterLimit={announcementTextLimit}
        formatRemaining={formatRemaining}
        {...(textError === undefined ? {} : { isInvalid: true, errorMessage: textError })}
      />
      {failed && actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.announcements.change_failed", { reason: actions.error })}
        </p>
      )}
      <div className="flex flex-wrap justify-end gap-2">
        <Button onPress={close}>{t("client:admin.announcements.cancel")}</Button>
        <Button type="submit" variant="primary" isPending={isSaving}>
          {t("client:admin.announcements.save")}
        </Button>
      </div>
    </form>
  );
}

/** Withdrawing an announcement, after an alert dialog: it cannot be undone. */
function WithdrawAnnouncement({ announcement, name, actions }: AnnouncementActionProps) {
  const t = useT();
  const [isWithdrawing, setIsWithdrawing] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <DialogTrigger>
      <Button size="sm" variant="danger" aria-label={t("client:admin.announcements.withdraw_one", { announcement: name })}>
        {t("client:admin.announcements.withdraw")}
      </Button>
      <ConfirmDialog
        title={t("client:admin.announcements.withdraw.title", { announcement: name })}
        description={t("client:admin.announcements.withdraw.description")}
        confirmLabel={t("client:admin.announcements.withdraw")}
        cancelLabel={t("client:admin.announcements.cancel")}
        isPending={isWithdrawing}
        {...(failed && actions.error !== undefined && { errorMessage: t("client:admin.announcements.change_failed", { reason: actions.error }) })}
        onConfirm={(close) => {
          setIsWithdrawing(true);
          setFailed(false);
          void actions.withdraw(announcement).then((withdrawn) => {
            setIsWithdrawing(false);
            if (withdrawn) close();
            else setFailed(true);
          });
        }}
      />
    </DialogTrigger>
  );
}
