// SPDX-License-Identifier: AGPL-3.0-only
// The buttons of a profile that change the viewer's relationship with its account (FR-301 to FR-303):
// the follow button, whose label is where the follow stands (Follow, Requested, Following, Blocked), and
// a menu button (WAI-ARIA APG, menu button pattern) with mute, block and report (features/report). Undoing a follow or a request,
// blocking and unblocking ask first in an alert dialog (WAI-ARIA APG, alert dialog pattern). A follow
// request to an account of another server says which server it waits for.
import { Button, Dialog, Link, Menu, MenuItem, MenuTrigger, MoreIcon } from "@my/ui";
import { useId, useState } from "react";
import { type Translate, useT } from "../../../lib/i18n";
import { ReportDialog } from "../../report";
import type { Account, RelationshipAction } from "../api/accounts";
import { isolate } from "../hooks/useProfile";
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
        <Link href="/settings/profile" variant="standalone">
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

type OtherActionsProps = { account: Account; name: string; state: Extract<RelationshipState, { kind: "other" }> };

/** The follow button's label and variant, and the change it asks for, for where the follow stands. */
function followButtonOf(t: Translate, account: Account, state: OtherActionsProps["state"]) {
  switch (state.follow) {
    case "none": {
      const label = account.locked
        ? t("client:profile.follow.request")
        : state.relationship.followed_by
          ? t("client:profile.follow.back")
          : t("client:profile.follow");
      return { label, variant: "primary", press: "follow" } as const;
    }
    case "requested":
      return { label: t("client:profile.follow.requested"), variant: "secondary", press: "withdraw" } as const;
    case "following":
      return { label: t("client:profile.follow.following"), variant: "secondary", press: "unfollow" } as const;
    case "blocking":
      return { label: t("client:profile.follow.blocked"), variant: "secondary", press: "unblock" } as const;
  }
}

function OtherActions({ account, name, state }: OtherActionsProps) {
  const t = useT();
  // The kind stays while the dialog closes, so that it shows what it asked until it is gone.
  const [dialog, setDialog] = useState<{ isOpen: boolean; kind: Confirmable }>({ isOpen: false, kind: "unfollow" });
  const [isReporting, setIsReporting] = useState(false);
  const confirm = (kind: Confirmable) => {
    setDialog({ isOpen: true, kind });
  };
  const { relationship, pending, change } = state;
  const isPending = pending !== null;
  const followButton = followButtonOf(t, account, state);
  const moreLabel = t("client:profile.more", { name: isolate(name) });
  // Another server answers a follow request on its own, later (federation.md): until it accepts, the
  // request is pending, and the button's description names the server it waits for.
  const pendingId = useId();
  const pendingRemote = state.follow === "requested" && account.acct.includes("@");

  return (
    <>
      <div className="flex flex-wrap items-center gap-2">
        {/* One button for every state, so that it keeps the focus when its label changes. */}
        <Button
          variant={followButton.variant}
          isPending={isPending}
          {...(pendingRemote ? { "aria-describedby": pendingId } : {})}
          onPress={() => {
            if (followButton.press === "follow") change("follow");
            else confirm(followButton.press);
          }}
        >
          {followButton.label}
        </Button>
        <MenuTrigger>
          <Button variant="ghost" isPending={isPending} aria-label={moreLabel} className="px-2">
            <MoreIcon />
          </Button>
          <Menu
            aria-label={moreLabel}
            placement="bottom end"
            onAction={(key) => {
              if (key === "mute") change(relationship.muting ? "unmute" : "mute");
              if (key === "block") confirm(relationship.blocking ? "unblock" : "block");
              if (key === "report") setIsReporting(true);
            }}
          >
            {/* The items keep their ids when their action flips: React Aria's collections cannot change an
                item's id while the menu is still mounted while it closes. */}
            <MenuItem id="mute">{relationship.muting ? t("client:profile.menu.unmute") : t("client:profile.menu.mute")}</MenuItem>
            <MenuItem id="block" intent={relationship.blocking ? "neutral" : "danger"}>
              {relationship.blocking ? t("client:profile.menu.unblock") : t("client:profile.menu.block")}
            </MenuItem>
            <MenuItem id="report">{t("client:report.open")}</MenuItem>
          </Menu>
        </MenuTrigger>
        <ReportDialog account={account} isOpen={isReporting} onOpenChange={setIsReporting} />
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
      {/* A line of its own in the header's wrapping row, below the picture and the buttons. */}
      {pendingRemote && (
        <p id={pendingId} className="basis-full text-sm wrap-anywhere text-fg-muted">
          {t("client:profile.follow.pending_remote", { host: isolate(account.domain) })}
        </p>
      )}
    </>
  );
}

type ConfirmDialogProps = {
  isOpen: boolean;
  kind: Confirmable;
  name: string;
  onCancel: () => void;
  onConfirm: (action: RelationshipAction) => void;
};

/** The texts of the confirmation of `kind`, and the change it confirms. */
function confirmationOf(t: Translate, kind: Confirmable, name: string) {
  const values = { name: isolate(name) };
  switch (kind) {
    case "unfollow":
      return {
        title: t("client:profile.confirm.unfollow.title", values),
        body: t("client:profile.confirm.unfollow.body"),
        action: t("client:profile.confirm.unfollow.action"),
        run: "unfollow",
      } as const;
    case "withdraw":
      return {
        title: t("client:profile.confirm.withdraw.title", values),
        body: t("client:profile.confirm.withdraw.body"),
        action: t("client:profile.confirm.withdraw.action"),
        run: "unfollow",
      } as const;
    case "block":
      return {
        title: t("client:profile.confirm.block.title", values),
        body: t("client:profile.confirm.block.body"),
        action: t("client:profile.confirm.block.action"),
        run: "block",
      } as const;
    case "unblock":
      return {
        title: t("client:profile.confirm.unblock.title", values),
        body: t("client:profile.confirm.unblock.body"),
        action: t("client:profile.confirm.unblock.action"),
        run: "unblock",
      } as const;
  }
}

/** Asks before a change that the viewer may not mean: one alert dialog, whose texts are the change's. */
function ConfirmDialog({ isOpen, kind, name, onCancel, onConfirm }: ConfirmDialogProps) {
  const t = useT();
  const text = confirmationOf(t, kind, name);
  return (
    <Dialog
      role="alertdialog"
      size="sm"
      title={text.title}
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) onCancel();
      }}
    >
      <p>{text.body}</p>
      <div className="mt-4 flex flex-wrap justify-end gap-2">
        <Button onPress={onCancel}>{t("client:profile.confirm.cancel")}</Button>
        <Button
          variant={text.run === "block" ? "danger" : "primary"}
          onPress={() => {
            onConfirm(text.run);
          }}
        >
          {text.action}
        </Button>
      </div>
    </Dialog>
  );
}
