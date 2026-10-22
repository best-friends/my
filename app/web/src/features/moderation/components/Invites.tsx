// SPDX-License-Identifier: AGPL-3.0-only
// The invites screen below its heading (FR-001, manage_invites): the form of a new invite, then the invites, newest
// first. An invite lets people sign up only while the registration mode is `invite` (a server setting, /admin/settings),
// which the screen says when it is not. A new invite's code and link stay under the form in a StatusMessage; each
// usable invite can be copied as its sign-up link or revoked, which takes effect at once and asks nothing first.
import { Box, Button, DescriptionItem, DescriptionList, DurationSelect, EmptyState, Heading, InlineError, SectionHeader, Select, SelectItem, Stack, StatusMessage, Text } from "@my/ui";
import { useRef, useState } from "react";
import { useIntl } from "react-intl";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFocusHeadingAfterLift } from "../../../lib/useFocusAfterRemoval";
import type { Invite, InviteState } from "../api/invites";
import { inviteExpiries, inviteMaxUses, type RevokeInvite, useCopyInviteLink, useInviteForm, useInvites, useInvitesWork, useRevokeInvite } from "../hooks/useInvites";
import { DateTime, DetailSection, RecordList, RequirePermission } from "./parts";

const stateLabels: Readonly<Record<InviteState, ClientMessageKey>> = {
  usable: "client:admin.invites.state.usable",
  revoked: "client:admin.invites.state.revoked",
  expired: "client:admin.invites.state.expired",
  exhausted: "client:admin.invites.state.exhausted",
};

/** The Select's key of "no limit", which is not a number of uses. */
const unlimited = "unlimited";

export function Invites() {
  const t = useT();
  return (
    <RequirePermission permission="manage_invites">
      <Stack gap="8">
        <ModeNotice />
        <DetailSection title={t("client:admin.invites.new.title")}>
          <NewInviteForm />
        </DetailSection>
        <DetailSection title={t("client:admin.invites.list.title")}>
          <InviteList />
        </DetailSection>
      </Stack>
    </RequirePermission>
  );
}

/** Why the invites do not let anyone sign up now, unless the registration mode is `invite`. */
function ModeNotice() {
  const t = useT();
  if (useInvitesWork()) return null;
  return (
    <Box as="p" border="md" padding="3">
      {t("client:admin.invites.mode_notice")}
    </Box>
  );
}

function NewInviteForm() {
  const t = useT();
  const form = useInviteForm();
  const formatExpiry = (seconds: number) => {
    if (seconds % 86_400 === 0) return t("client:admin.settings.duration.days", { count: seconds / 86_400 });
    if (seconds % 3600 === 0) return t("client:admin.settings.duration.hours", { count: seconds / 3600 });
    return t("client:admin.invites.duration.minutes", { count: seconds / 60 });
  };
  return (
    <Stack as="form" onSubmit={form.submit} gap="6">
      <Select
        label={t("client:admin.invites.max_uses")}
        description={t("client:admin.invites.max_uses.description")}
        value={form.draft.maxUses === null ? unlimited : String(form.draft.maxUses)}
        onChange={(key) => {
          if (key === unlimited) form.change({ maxUses: null });
          else if (typeof key === "string") form.change({ maxUses: Number(key) });
        }}
        items={[...inviteMaxUses.map((uses) => ({ id: String(uses), name: t("client:admin.invites.uses", { count: uses }) })), { id: unlimited, name: t("client:admin.invites.unlimited") }]}
      >
        {(item) => (
          <SelectItem id={item.id} textValue={item.name}>
            {item.name}
          </SelectItem>
        )}
      </Select>
      <DurationSelect
        label={t("client:admin.invites.expires_in")}
        durations={inviteExpiries}
        formatDuration={formatExpiry}
        indefiniteLabel={t("client:admin.invites.never")}
        value={form.draft.expiresIn}
        onChange={(expiresIn) => {
          form.change({ expiresIn });
        }}
      />
      {form.error !== undefined && (
        <InlineError alert size="inherit">
          {t("client:admin.invites.create_failed", { reason: form.error })}
        </InlineError>
      )}
      <Box>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:admin.invites.create")}
        </Button>
      </Box>
      <StatusMessage>{form.status}</StatusMessage>
      {form.created !== undefined && (
        <Stack as="p" gap="1">
          <span>{t("client:admin.invites.link")}</span>
          <Text dir="ltr" mono wrap="anywhere" selectAll>
            {form.created.url}
          </Text>
        </Stack>
      )}
    </Stack>
  );
}

function InviteList() {
  const t = useT();
  const listRef = useRef<HTMLUListElement>(null);
  const list = useInvites();
  const [status, setStatus] = useState<string | undefined>(undefined);
  const keepFocus = useFocusHeadingAfterLift(listRef);
  const revoke = useRevokeInvite((invite) => {
    // The buttons go away from the row, which stays.
    keepFocus(invite.id);
    setStatus(t("client:admin.invites.revoked", { code: invite.code }));
  });
  return (
    <Stack gap="3">
      <StatusMessage>{status}</StatusMessage>
      <RecordList
        list={list}
        getId={(invite) => invite.id}
        listRef={listRef}
        empty={<EmptyState headingLevel={3} heading={t("client:admin.invites.empty")} description={t("client:admin.invites.empty.description")} />}
      >
        {(invite) => <InviteRow invite={invite} revoke={revoke} />}
      </RecordList>
    </Stack>
  );
}

function InviteRow({ invite, revoke }: { invite: Invite; revoke: RevokeInvite }) {
  const t = useT();
  const intl = useIntl();
  const copy = useCopyInviteLink();
  const created = intl.formatDate(invite.created_at, { dateStyle: "medium", timeStyle: "short" });
  return (
    <Stack gap="3">
      <SectionHeader
        actions={
          invite.state === "usable" && (
            <>
            <Button
              size="sm"
              aria-label={t("client:admin.invites.copy_invite", { code: invite.code })}
              onPress={() => {
                copy(invite);
              }}
            >
              {t("client:admin.invites.copy")}
            </Button>
            <Button
              size="sm"
              variant="danger"
              isPending={revoke.pendingIds.includes(invite.id)}
              aria-label={t("client:admin.invites.revoke_invite", { code: invite.code, date: created })}
              onPress={() => {
                revoke.revoke(invite);
              }}
            >
              {t("client:admin.invites.revoke")}
            </Button>
            </>
          )
        }
      >
        <Heading level={3} dir="ltr" mono wrap="anywhere" focusable>
          {invite.code}
        </Heading>
      </SectionHeader>
      <DescriptionList columns={3}>
        <DescriptionItem term={t("client:admin.invites.state")}>
          <Text {...(invite.state === "usable" ? { weight: "semibold" as const } : {})}>{t(stateLabels[invite.state])}</Text>
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.invites.used")}>
          {invite.max_uses === undefined
            ? t("client:admin.invites.used.unlimited", { uses: invite.uses })
            : t("client:admin.invites.used.limited", { uses: invite.uses, max: invite.max_uses })}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.invites.expires_at")}>
          {invite.expires_at === undefined ? t("client:admin.invites.never") : <DateTime value={invite.expires_at} />}
        </DescriptionItem>
        <DescriptionItem term={t("client:admin.invites.created_at")}>
          <DateTime value={invite.created_at} />
        </DescriptionItem>
      </DescriptionList>
    </Stack>
  );
}
