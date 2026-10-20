// SPDX-License-Identifier: AGPL-3.0-only
// The roles screen below its heading (FR-720, manage_roles): the form that creates a role, then the roles, highest
// position first. Each role shows its position and its permissions by area (ADR-0058); one at or below the viewer's
// highest role is changed in a dialog (position and permissions; never admin) and deleted after an alert dialog
// (never a built-in role, and never one that someone holds). A role above the viewer's says why it has no buttons.
// Roles are granted on an account's admin page. What a change did stays above the list in a StatusMessage; a refusal
// stays in the form or the dialog.
import { Button, Checkbox, ConfirmDialog, DescriptionItem, DescriptionList, Dialog, DialogTrigger, Link, StatusMessage, TextField } from "@my/ui";
import { type SubmitEvent, useId, useRef, useState } from "react";
import { type ClientMessageKey, useT } from "../../../lib/i18n";
import { useFocusAfterRemoval } from "../../../lib/useFocusAfterRemoval";
import { DetailSection, type Permission, RecordList, RequirePermission } from "../../moderation";
import { adminRoleKey, type AdminRole, moderatorRoleKey, permissionCategories, roleKeyLimit } from "../api/roles";
import {
  draftOfRole,
  isolatedRoleKey,
  type RoleActions,
  type RoleDraft,
  type RoleFieldError,
  type RoleRights,
  type RoleRow,
  togglePermission,
  useNewRoleForm,
  useRoleActions,
  useRoleSettings,
} from "../hooks/useRoleSettings";

const permissionLabels: Readonly<Record<Permission, ClientMessageKey>> = {
  manage_reports: "client:admin.roles.permission.manage_reports",
  manage_accounts: "client:admin.roles.permission.manage_accounts",
  manage_domains: "client:admin.roles.permission.manage_domains",
  manage_rules: "client:admin.roles.permission.manage_rules",
  manage_settings: "client:admin.roles.permission.manage_settings",
  manage_roles: "client:admin.roles.permission.manage_roles",
  manage_invites: "client:admin.roles.permission.manage_invites",
  view_audit: "client:admin.roles.permission.view_audit",
  manage_jobs: "client:admin.roles.permission.manage_jobs",
  manage_two_factor: "client:admin.roles.permission.manage_two_factor",
  manage_custom_emojis: "client:admin.roles.permission.manage_custom_emojis",
  manage_webhooks: "client:admin.roles.permission.manage_webhooks",
  manage_announcements: "client:admin.roles.permission.manage_announcements",
};

const categoryLabels: Readonly<Record<"moderation" | "server", ClientMessageKey>> = {
  moderation: "client:admin.roles.category.moderation",
  server: "client:admin.roles.category.server",
};

/** A role's key, an identifier that reads left to right in every locale. */
function RoleKey({ value }: { value: string }) {
  return (
    <span dir="ltr" className="wrap-anywhere">
      {value}
    </span>
  );
}

export function RoleSettings() {
  return (
    <RequirePermission permission="manage_roles">
      <RoleScreen />
    </RequirePermission>
  );
}

function RoleScreen() {
  const t = useT();
  const { rows, rights } = useRoleSettings();
  return (
    <div className="flex flex-col gap-8">
      <DetailSection title={t("client:admin.roles.new.title")}>
        <NewRoleForm rights={rights} />
      </DetailSection>
      <DetailSection title={t("client:admin.roles.list.title")}>
        <RoleList rows={rows} rights={rights} />
      </DetailSection>
    </div>
  );
}

function NewRoleForm({ rights }: { rights: RoleRights }) {
  const t = useT();
  const form = useNewRoleForm(rights);
  if (rights.top === undefined) return <p className="text-fg-muted">{t("client:admin.roles.new.no_role")}</p>;
  return (
    <form onSubmit={form.submit} className="@container flex flex-col gap-6" noValidate>
      <div className="flex flex-col items-start gap-1">
        <p className="text-sm text-fg-muted">{t("client:admin.roles.new.description")}</p>
        <Link href="/moderation/accounts" variant="standalone">
          {t("client:admin.roles.new.accounts")}
        </Link>
      </div>
      <TextField
        label={t("client:admin.roles.key")}
        description={t("client:admin.roles.key.description", { max: String(roleKeyLimit) })}
        value={form.draft.key}
        onChange={(key) => {
          form.change({ key });
        }}
        isRequired
        maxLength={roleKeyLimit}
        autoComplete="off"
        spellCheck="false"
        {...(form.error?.field === "key" ? { isInvalid: true, errorMessage: form.error.message } : {})}
      />
      <RoleFields draft={form.draft} change={form.change} error={form.error} rights={rights} />
      <div>
        <Button type="submit" variant="primary" isPending={form.isPending}>
          {t("client:admin.roles.submit")}
        </Button>
      </div>
      <StatusMessage>{form.status}</StatusMessage>
    </form>
  );
}

type RoleFieldsProps = {
  draft: RoleDraft;
  change: (change: Partial<RoleDraft>) => void;
  error: RoleFieldError | undefined;
  rights: RoleRights;
  autoFocus?: boolean;
};

/** The position and the permissions of a role, the permissions in a group per area. */
function RoleFields({ draft, change, error, rights, autoFocus = false }: RoleFieldsProps) {
  const t = useT();
  const hintId = useId();
  const errorId = useId();
  return (
    <>
      <TextField
        label={t("client:admin.roles.position")}
        description={t("client:admin.roles.position.description", { max: String(rights.maxPosition) })}
        value={draft.position}
        onChange={(position) => {
          change({ position });
        }}
        isRequired
        inputMode="numeric"
        autoComplete="off"
        autoFocus={autoFocus}
        {...(error?.field === "position" ? { isInvalid: true, errorMessage: error.message } : {})}
      />
      <p id={hintId} className="text-sm text-fg-muted">
        {t("client:admin.roles.permissions.description")}
      </p>
      {permissionCategories.map(({ category, permissions }) => (
        <PermissionGroup
          key={category}
          label={t(categoryLabels[category])}
          permissions={permissions}
          draft={draft}
          change={change}
          rights={rights}
          describedBy={error?.field === "permissions" ? `${hintId} ${errorId}` : hintId}
        />
      ))}
      {error?.field === "permissions" && (
        <div id={errorId} role="alert" className="flex flex-col gap-1 text-sm text-danger">
          <p>{error.message}</p>
          {error.missing.length > 0 && <p>{t("client:admin.roles.missing_permissions", { permissions: `⁦${error.missing.join(", ")}⁩` })}</p>}
        </div>
      )}
      {error?.field === "form" && (
        <p role="alert" className="text-danger">
          {t("client:admin.roles.save_failed", { reason: error.message })}
        </p>
      )}
    </>
  );
}

type PermissionGroupProps = {
  label: string;
  permissions: readonly Permission[];
  draft: RoleDraft;
  change: (change: Partial<RoleDraft>) => void;
  rights: RoleRights;
  describedBy: string;
};

/** The permissions of one area; those the viewer's roles do not grant cannot be given or taken. */
function PermissionGroup({ label, permissions, draft, change, rights, describedBy }: PermissionGroupProps) {
  const t = useT();
  const labelId = useId();
  return (
    <div role="group" aria-labelledby={labelId} aria-describedby={describedBy} className="flex flex-col gap-2">
      <span id={labelId} className="text-sm font-medium text-fg">
        {label}
      </span>
      <div className="grid gap-2 @md:grid-cols-2">
        {permissions.map((permission) => (
          <Checkbox
            key={permission}
            isSelected={draft.permissions.includes(permission)}
            isDisabled={!rights.permissions.includes(permission)}
            onChange={(selected) => {
              change({ permissions: togglePermission(draft.permissions, permission, selected) });
            }}
          >
            {t(permissionLabels[permission])}
          </Checkbox>
        ))}
      </div>
    </div>
  );
}

function RoleList({ rows, rights }: { rows: RoleRow[]; rights: RoleRights }) {
  const listRef = useRef<HTMLUListElement>(null);
  const removed = useFocusAfterRemoval(
    listRef,
    rows.map((row) => row.role.id),
  );
  const actions = useRoleActions(rights, removed);
  const list = { items: rows, hasMore: false, isLoadingMore: false, loadMore: () => undefined };
  return (
    <div className="flex flex-col gap-3">
      <StatusMessage>{actions.status}</StatusMessage>
      <RecordList list={list} getId={(row) => row.role.id} listRef={listRef} empty={null}>
        {(row) => <RoleItem row={row} rights={rights} actions={actions} />}
      </RecordList>
    </div>
  );
}

/** The permissions of a role in one area, as a sentence. */
function permissionNames(role: AdminRole, permissions: readonly Permission[], t: ReturnType<typeof useT>): string {
  const names = permissions.filter((permission) => role.permissions.includes(permission)).map((permission) => t(permissionLabels[permission]));
  return names.length === 0 ? t("client:admin.roles.permissions.none") : names.join(t("client:admin.roles.permissions.separator"));
}

function RoleItem({ row, rights, actions }: { row: RoleRow; rights: RoleRights; actions: RoleActions }) {
  const t = useT();
  const { role, outrankedBy } = row;
  const builtin =
    role.key === adminRoleKey
      ? t("client:admin.roles.builtin.admin")
      : role.key === moderatorRoleKey
        ? t("client:admin.roles.builtin.moderator")
        : undefined;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col items-start gap-2 @md:flex-row @md:items-center @md:justify-between">
        <h3 className="font-semibold">
          <RoleKey value={role.key} />
        </h3>
        {(row.canChange || row.canDelete) && (
          <div className="flex flex-wrap gap-2">
            {row.canChange && <EditRole role={role} rights={rights} actions={actions} />}
            {row.canDelete && <DeleteRole role={role} actions={actions} />}
          </div>
        )}
      </div>
      <DescriptionList columns={2}>
        <DescriptionItem term={t("client:admin.roles.position")}>{role.position}</DescriptionItem>
        {permissionCategories.map(({ category, permissions }) => (
          <DescriptionItem key={category} term={t(categoryLabels[category])}>
            {permissionNames(role, permissions, t)}
          </DescriptionItem>
        ))}
      </DescriptionList>
      {builtin !== undefined && <p className="text-sm text-fg-muted">{builtin}</p>}
      {outrankedBy !== undefined && (
        <p className="text-sm text-fg-muted">{t("client:admin.roles.outranked", { role: isolatedRoleKey(outrankedBy.key) })}</p>
      )}
    </div>
  );
}

/** The position and permissions of a role in a dialog; it closes once they are saved, and keeps a refusal on its field. */
function EditRole({ role, rights, actions }: { role: AdminRole; rights: RoleRights; actions: RoleActions }) {
  const t = useT();
  const key = isolatedRoleKey(role.key);
  return (
    <DialogTrigger>
      <Button size="sm" aria-label={t("client:admin.roles.edit_role", { role: key })}>
        {t("client:admin.roles.edit")}
      </Button>
      <Dialog title={t("client:admin.roles.edit.title", { role: key })} closeLabel={t("client:admin.roles.edit.close")} size="md">
        {({ close }) => <EditRoleForm role={role} rights={rights} actions={actions} close={close} />}
      </Dialog>
    </DialogTrigger>
  );
}

function EditRoleForm({ role, rights, actions, close }: { role: AdminRole; rights: RoleRights; actions: RoleActions; close: () => void }) {
  const t = useT();
  const [draft, setDraft] = useState<RoleDraft>(() => draftOfRole(role));
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState<RoleFieldError | undefined>(undefined);
  const onSubmit = (event: SubmitEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSaving(true);
    setError(undefined);
    void actions.update(role, draft).then((refused) => {
      setIsSaving(false);
      if (refused === undefined) close();
      else setError(refused);
    });
  };
  return (
    <form onSubmit={onSubmit} className="@container flex flex-col gap-4" noValidate>
      <RoleFields
        draft={draft}
        change={(change) => {
          setDraft((previous) => ({ ...previous, ...change }));
        }}
        error={error}
        rights={rights}
        autoFocus
      />
      <div className="flex flex-wrap justify-end gap-2">
        <Button onPress={close}>{t("client:admin.roles.cancel")}</Button>
        <Button type="submit" variant="primary" isPending={isSaving}>
          {t("client:admin.roles.save")}
        </Button>
      </div>
    </form>
  );
}

/** Deleting a role, after an alert dialog: it cannot be undone, and the server refuses a role that someone holds. */
function DeleteRole({ role, actions }: { role: AdminRole; actions: RoleActions }) {
  const t = useT();
  const key = isolatedRoleKey(role.key);
  const [isDeleting, setIsDeleting] = useState(false);
  const [error, setError] = useState<string | undefined>(undefined);
  return (
    <DialogTrigger
      onOpenChange={() => {
        setError(undefined);
      }}
    >
      <Button size="sm" variant="danger" aria-label={t("client:admin.roles.delete_role", { role: key })}>
        {t("client:admin.roles.delete")}
      </Button>
      <ConfirmDialog
        title={t("client:admin.roles.delete.title", { role: key })}
        description={t("client:admin.roles.delete.description")}
        confirmLabel={t("client:admin.roles.delete")}
        cancelLabel={t("client:admin.roles.cancel")}
        isPending={isDeleting}
        {...(error !== undefined && { errorMessage: t("client:admin.roles.delete_failed", { reason: error }) })}
        onConfirm={(close) => {
          setIsDeleting(true);
          setError(undefined);
          void actions.remove(role).then((failed) => {
            setIsDeleting(false);
            if (failed === undefined) close();
            else setError(failed);
          });
        }}
      />
    </DialogTrigger>
  );
}
