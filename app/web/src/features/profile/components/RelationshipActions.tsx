// SPDX-License-Identifier: AGPL-3.0-only
// The buttons of a profile that change the viewer's relationship with its account (FR-301 to FR-303):
// the follow button, whose label is where the follow stands (Follow, Requested, Following, Blocked), and
// a menu button (WAI-ARIA APG, menu button pattern) with mute and block. Undoing a follow or a request,
// blocking and unblocking ask first in an alert dialog (WAI-ARIA APG, alert dialog pattern).
import { Button, Dialog, Link, Menu, MenuItem, MenuTrigger } from "@my/ui";
import { useState } from "react";
import { useT } from "../../../lib/i18n";
import type { Account, RelationshipAction } from "../api/accounts";
import type { RelationshipState } from "../hooks/useRelationship";

/** The changes that ask the viewer first. */
type Confirmable = "unfollow" | "withdraw" | "block" | "unblock";

export type RelationshipActionsProps = {
  account: Account;
  /** The account's name in the UI. */
  name: string;
  state: RelationshipState;
};

export function RelationshipActions({ account, name, state }: RelationshipActionsProps) {
  const t = useT();
  switch (state.kind) {
    case "signed-out":
      return (
        <Link href={state.signInHref} variant="standalone">
          {t("client:profile.sign_in_to_follow")}
        </Link>
      );
    case "self":
      return (
        <Link href="/settings" variant="standalone">
          {t("client:profile.edit")}
        </Link>
      );
    case "loading":
      return null;
    case "error":
      return <p className="text-sm text-fg-muted">{t("client:profile.relationship_failed", { reason: state.message })}</p>;
    case "other":
      return <OtherActions account={account} name={name} state={state} />;
  }
}

function OtherActions({ account, name, state }: { account: Account; name: string; state: Extract<RelationshipState, { kind: "other" }> }) {
  const t = useT();
  // The kind stays while the dialog closes, so that its exit transition still shows what it asked.
  const [dialog, setDialog] = useState<{ isOpen: boolean; kind: Confirmable }>({ isOpen: false, kind: "unfollow" });
  const confirm = (kind: Confirmable) => {
    setDialog({ isOpen: true, kind });
  };
  const { follow, relationship, pending, change } = state;
  const isPending = pending !== null;

  const followButton = (() => {
    switch (follow) {
      case "none":
        return (
          <Button variant="primary" isPending={isPending} onPress={() => { change("follow"); }}>
            {account.locked ? t("client:profile.follow.request") : relationship.followed_by ? t("client:profile.follow.back") : t("client:profile.follow")}
          </Button>
        );
      case "requested":
        return (
          <Button isPending={isPending} onPress={() => { confirm("withdraw"); }}>
            {t("client:profile.follow.requested")}
          </Button>
        );
      case "following":
        return (
          <Button isPending={isPending} onPress={() => { confirm("unfollow"); }}>
            {t("client:profile.follow.following")}
          </Button>
        );
      case "blocking":
        return (
          <Button isPending={isPending} onPress={() => { confirm("unblock"); }}>
            {t("client:profile.follow.blocked")}
          </Button>
        );
    }
  })();

  return (
    <div className="flex flex-wrap items-center gap-2">
      {followButton}
      <MenuTrigger>
        <Button variant="ghost" isPending={isPending} aria-label={t("client:profile.more", { name: `⁨${name}⁩` })} className="px-2">
          <MoreIcon />
        </Button>
        <Menu
          aria-label={t("client:profile.more", { name: `⁨${name}⁩` })}
          placement="bottom end"
          onAction={(key) => {
            if (key === "mute") change("mute");
            if (key === "unmute") change("unmute");
            if (key === "block") confirm("block");
            if (key === "unblock") confirm("unblock");
          }}
        >
          {relationship.muting ? (
            <MenuItem id="unmute">{t("client:profile.menu.unmute")}</MenuItem>
          ) : (
            <MenuItem id="mute">{t("client:profile.menu.mute")}</MenuItem>
          )}
          {relationship.blocking ? (
            <MenuItem id="unblock">{t("client:profile.menu.unblock")}</MenuItem>
          ) : (
            <MenuItem id="block" intent="danger">
              {t("client:profile.menu.block")}
            </MenuItem>
          )}
        </Menu>
      </MenuTrigger>
      <ConfirmDialog
        isOpen={dialog.isOpen}
        kind={dialog.kind}
        name={name}
        onCancel={() => {
          setDialog({ ...dialog, isOpen: false });
        }}
        onConfirm={(action) => {
          setDialog({ ...dialog, isOpen: false });
          change(action);
        }}
      />
    </div>
  );
}

type ConfirmDialogProps = {
  isOpen: boolean;
  kind: Confirmable;
  name: string;
  onCancel: () => void;
  onConfirm: (action: RelationshipAction) => void;
};

/** Asks before a change that the viewer may not mean: one alert dialog, whose text is the change's. */
function ConfirmDialog({ isOpen, kind, name, onCancel, onConfirm }: ConfirmDialogProps) {
  const t = useT();
  const values = { name: `⁨${name}⁩` };
  const text = (() => {
    switch (kind) {
      case "unfollow":
        return { title: t("client:profile.confirm.unfollow.title", values), body: t("client:profile.confirm.unfollow.body"), action: t("client:profile.confirm.unfollow.action"), run: "unfollow" as const };
      case "withdraw":
        return { title: t("client:profile.confirm.withdraw.title", values), body: t("client:profile.confirm.withdraw.body"), action: t("client:profile.confirm.withdraw.action"), run: "unfollow" as const };
      case "block":
        return { title: t("client:profile.confirm.block.title", values), body: t("client:profile.confirm.block.body"), action: t("client:profile.confirm.block.action"), run: "block" as const };
      case "unblock":
        return { title: t("client:profile.confirm.unblock.title", values), body: t("client:profile.confirm.unblock.body"), action: t("client:profile.confirm.unblock.action"), run: "unblock" as const };
    }
  })();
  return (
    <Dialog
      role="alertdialog"
      size="sm"
      title={text.title}
      isOpen={isOpen}
      onOpenChange={(isOpen) => {
        if (!isOpen) onCancel();
      }}
    >
      <p>{text.body}</p>
      <div className="mt-4 flex flex-wrap justify-end gap-2">
        <Button onPress={onCancel}>{t("client:profile.confirm.cancel")}</Button>
        <Button variant={text.run === "block" ? "danger" : "primary"} onPress={() => { onConfirm(text.run); }}>
          {text.action}
        </Button>
      </div>
    </Dialog>
  );
}

function MoreIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" fill="currentColor" className="size-5">
      <circle cx="5" cy="12" r="2" />
      <circle cx="12" cy="12" r="2" />
      <circle cx="19" cy="12" r="2" />
    </svg>
  );
}
