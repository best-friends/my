// SPDX-License-Identifier: AGPL-3.0-only
// The requests tab of the custom emoji screen (ADR-0070 §2, manage_custom_emojis): the requests of the holders of
// create_custom_emojis in one state (the pending ones by default, the state kept in the URL by the route,
// web-client.md S3), newest first. Each shows the requested image, its shortcode, what it asks for and who asked; a
// pending one is approved at once, or rejected after an alert dialog with an optional reason that the requester
// sees. What a decision did stays above the list in a StatusMessage, and a failure (a request that cannot be decided
// says why) stays there too (role="alert").
import { Button, ConfirmDialog, CustomEmoji, DescriptionItem, DescriptionList, DialogTrigger, EmptyState, Link, ReasonField, Select, SelectItem, StatusMessage } from "@my/ui";
import { useRef, useState } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { profilePath } from "../../../lib/paths";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import { DateTime, RecordList } from "../../moderation";
import { type AdminCustomEmojiRequest, type CustomEmojiRequestState, customEmojiRequestStates, rejectReasonLimit } from "../api/customEmojiRequests";
import { type CustomEmojiRequestActions, requestInText, useAdminCustomEmojiRequests, useCustomEmojiRequestActions } from "../hooks/useCustomEmojiRequests";

/** The name of each state of a request, shared with the user's page of their requests. */
export const requestStateLabels: Readonly<Record<CustomEmojiRequestState, ClientMessageKey>> = {
  pending: "client:custom_emoji_requests.state.pending",
  approved: "client:custom_emoji_requests.state.approved",
  rejected: "client:custom_emoji_requests.state.rejected",
  withdrawn: "client:custom_emoji_requests.state.withdrawn",
  lapsed: "client:custom_emoji_requests.state.lapsed",
};

/** What each kind of request asks for. */
const kindLabels: Readonly<Record<AdminCustomEmojiRequest["kind"], ClientMessageKey>> = {
  create: "client:custom_emoji_requests.kind.create",
  image: "client:custom_emoji_requests.kind.image",
};

function isState(key: unknown): key is CustomEmojiRequestState {
  return customEmojiRequestStates.some((state) => state === key);
}

export type CustomEmojiRequestsProps = {
  /** The state of the listed requests, from the URL. */
  state: CustomEmojiRequestState;
  /** Shows another state: the route puts it into the URL. */
  onStateChange: (state: CustomEmojiRequestState) => void;
};

export function CustomEmojiRequests({ state, onStateChange }: CustomEmojiRequestsProps) {
  const t = useT();
  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-fg-muted">{t("client:admin.custom_emoji_requests.description")}</p>
      <div>
        <Select
          label={t("client:admin.custom_emoji_requests.state")}
          value={state}
          onChange={(key) => {
            if (isState(key)) onStateChange(key);
          }}
          items={customEmojiRequestStates.map((id) => ({ id, name: t(requestStateLabels[id]) }))}
        >
          {(choice) => (
            <SelectItem id={choice.id} textValue={choice.name}>
              {choice.name}
            </SelectItem>
          )}
        </Select>
      </div>
      <RequestList key={state} state={state} />
    </div>
  );
}

function RequestList({ state }: { state: CustomEmojiRequestState }) {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const list = useAdminCustomEmojiRequests(state);
  const removed = useFocusAfterRemoval(
    listRef,
    list.items.map((request) => request.id),
  );
  const actions = useCustomEmojiRequestActions(removed);
  const empty =
    state === "pending" ? (
      <EmptyState headingLevel={3} heading={t("client:admin.custom_emoji_requests.empty.pending")} />
    ) : (
      <EmptyState headingLevel={3} heading={t("client:admin.custom_emoji_requests.empty")} />
    );
  return (
    <div className="flex flex-col gap-3">
      <StatusMessage>{actions.status}</StatusMessage>
      {actions.error !== undefined && (
        <p role="alert" className="text-danger">
          {t("client:admin.custom_emoji_requests.decide_failed", { reason: actions.error })}
        </p>
      )}
      <RecordList list={list} getId={(request) => request.id} listRef={listRef} empty={empty}>
        {(request) => <RequestRow request={request} actions={actions} />}
      </RecordList>
    </div>
  );
}

function RequestRow({ request, actions }: { request: AdminCustomEmojiRequest; actions: CustomEmojiRequestActions }) {
  const t = useT();
  const name = requestInText(request);
  const heading = `:${request.shortcode}:`;
  const isPending = actions.pendingId === request.id;
  const { requester } = request;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-3">
        {/* The heading names the emoji; the image beside it shows what was requested. */}
        <span aria-hidden="true" className="flex size-12 shrink-0 items-center justify-center rounded-md bg-surface-sunken text-3xl">
          {request.url !== undefined && request.static_url !== undefined && (
            <CustomEmoji emoji={{ shortcode: request.shortcode, url: request.url, static_url: request.static_url }} />
          )}
        </span>
        <h3 className="min-w-0 font-mono font-semibold wrap-anywhere">
          <span dir="ltr">{heading}</span>
        </h3>
      </div>
      <DescriptionList columns={2}>
        <DescriptionItem term={t("client:custom_emoji_requests.kind")}>{t(kindLabels[request.kind])}</DescriptionItem>
        <DescriptionItem term={t("client:custom_emoji_requests.state")}>{t(requestStateLabels[request.state])}</DescriptionItem>
        <DescriptionItem term={t("client:admin.custom_emoji_requests.requester")}>
          {requester === undefined ? (
            t("client:admin.custom_emoji_requests.requester.deleted")
          ) : (
            <AccountLink acct={requester.acct} />
          )}
        </DescriptionItem>
        <DescriptionItem term={t("client:custom_emoji_requests.created_at")}>
          <DateTime value={request.created_at} />
        </DescriptionItem>
        {request.url === undefined && <DescriptionItem term={t("client:custom_emoji_requests.image")}>{t("client:custom_emoji_requests.no_image")}</DescriptionItem>}
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
        <div className="flex flex-wrap gap-2">
          <Button
            size="sm"
            variant="primary"
            isPending={isPending}
            aria-label={t("client:admin.custom_emoji_requests.approve_request", { emoji: name })}
            onPress={() => {
              void actions.approve(request);
            }}
          >
            {t("client:admin.custom_emoji_requests.approve")}
          </Button>
          <RejectRequest request={request} actions={actions} isPending={isPending} />
        </div>
      )}
    </div>
  );
}

/** A link to an account's profile, named by its handle, which reads left to right. */
export function AccountLink({ acct }: { acct: string }) {
  const handle = `@${acct}`;
  return (
    <Link href={profilePath(acct)}>
      <span dir="ltr">{handle}</span>
    </Link>
  );
}

/** Rejecting a request, after an alert dialog with an optional reason that the requester sees. */
function RejectRequest({ request, actions, isPending }: { request: AdminCustomEmojiRequest; actions: CustomEmojiRequestActions; isPending: boolean }) {
  const t = useT();
  const name = requestInText(request);
  const [reason, setReason] = useState("");
  const [isRejecting, setIsRejecting] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <DialogTrigger>
      <Button size="sm" variant="danger" isDisabled={isPending} aria-label={t("client:admin.custom_emoji_requests.reject_request", { emoji: name })}>
        {t("client:admin.custom_emoji_requests.reject")}
      </Button>
      <ConfirmDialog
        title={t("client:admin.custom_emoji_requests.reject.title", { emoji: name })}
        description={t("client:admin.custom_emoji_requests.reject.description")}
        confirmLabel={t("client:admin.custom_emoji_requests.reject")}
        cancelLabel={t("client:admin.custom_emoji_requests.cancel")}
        isPending={isRejecting}
        {...(failed && actions.error !== undefined && { errorMessage: t("client:admin.custom_emoji_requests.decide_failed", { reason: actions.error }) })}
        onConfirm={(close) => {
          setIsRejecting(true);
          setFailed(false);
          void actions.reject(request, reason).then((rejected) => {
            setIsRejecting(false);
            if (rejected) close();
            else setFailed(true);
          });
        }}
      >
        <ReasonField
          label={t("client:admin.custom_emoji_requests.reason")}
          description={t("client:admin.custom_emoji_requests.reason.description")}
          value={reason}
          onChange={setReason}
          characterLimit={rejectReasonLimit}
          formatRemaining={(remaining) =>
            remaining < 0 ? t("client:admin.custom_emoji_requests.characters_over", { count: -remaining }) : t("client:admin.custom_emoji_requests.characters_left", { count: remaining })
          }
          errorMessage={t("client:admin.custom_emoji_requests.reason.too_long", { max: String(rejectReasonLimit) })}
        />
      </ConfirmDialog>
    </DialogTrigger>
  );
}
