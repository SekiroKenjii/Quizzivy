import { useMemo, type RefObject } from "react";
import { useTranslation } from "react-i18next";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { KeyRound, Pencil } from "lucide-react";
import { FormDialog } from "@/components/shared/form/FormDialog";
import {
  createRole,
  ROLES_KEY,
  updateRole,
  withRoles,
  type RoleEdit,
  type RoleList,
  type RoleSummary,
} from "@/features/roles/api";
import {
  blankAsAbsent,
  editRoleFields,
  nameKeyOf,
  newRoleFields,
  NO_SOURCE,
  saveError,
  takenNames,
  type EditRoleValues,
  type NewRoleValues,
} from "@/features/roles/roleFields";
import { ApiError } from "@/lib/api/errors";
import { reloadAfterRefusal } from "@/features/roles/refusal";
import { roleDescription, roleName } from "@/lib/roleName";
import { notify } from "@/lib/toast";

const EMPTY: NewRoleValues = { name: "", from: NO_SOURCE, description: "" };

function isCode(cause: unknown, ...codes: string[]): boolean {
  return cause instanceof ApiError && codes.includes(cause.code);
}

/**
 * NewRoleDialog is the deck's "New role": a name, the role whose permissions
 * it starts with (none by default) and a description. A name another role
 * has, typed in any letter case, is refused as "this role already exists",
 * before the request when the roles loaded already show it and after it when
 * the server does. The created role joins the list and `onCreated` gets its
 * id, so the carousel can scroll to its card.
 */
export function NewRoleDialog({
  open,
  roles,
  onOpenChange,
  onCreated,
  returnFocus,
}: Readonly<{
  open: boolean;
  roles: readonly RoleSummary[];
  onOpenChange: (open: boolean) => void;
  onCreated: (role: RoleSummary) => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const fields = useMemo(() => newRoleFields(t, roles), [t, roles]);
  const taken = useMemo(() => takenNames(roles, t), [roles, t]);

  const create = useMutation({
    mutationFn: (values: NewRoleValues) =>
      createRole({
        name: values.name.trim(),
        ...(blankAsAbsent(values.description) !== undefined && {
          description: values.description.trim(),
        }),
        ...(values.from !== NO_SOURCE && { copyFromRoleId: values.from }),
      }),
    onSuccess: (created) => {
      queryClient.setQueryData<RoleList>(
        ROLES_KEY,
        (old) => old && withRoles(old, [created]),
      );
      onOpenChange(false);
      notify.success(t("roles.newRole.created", { name: created.name }));
      onCreated(created);
    },
    onError: (cause) => reloadAfterRefusal(cause, queryClient),
  });

  let error: string | null = null;
  if (create.isError) {
    error = isCode(create.error, "ROLE_NAME_TAKEN")
      ? t("roles.newRole.taken")
      : saveError(create.error, t("roles.newRole.failed"));
  }

  return (
    <FormDialog
      open={open}
      onOpenChange={(next) => {
        if (!next) create.reset();
        onOpenChange(next);
      }}
      title={t("roles.newRole.title")}
      description={t("roles.newRole.hint")}
      icon={KeyRound}
      initial={EMPTY}
      fields={fields}
      validate={(values) =>
        taken.has(nameKeyOf(values.name)) ? { name: t("roles.newRole.taken") } : null
      }
      submitLabel={t("roles.newRole.submit")}
      pending={create.isPending}
      error={error}
      returnFocus={returnFocus}
      onSubmit={(values) => {
        if (!create.isPending) create.mutate(values);
      }}
    />
  );
}

/**
 * EditRoleDialog is the deck's "Edit role": the twelve icons, the name, the
 * description and the seven colours of the role it is given, open while
 * `role` is set. `role` is the role as the card showed it, and the dialog
 * sends only the fields the Admin changed against it, with `revision`, the
 * latest one loaded, so a built-in role's name is stored only when it is
 * renamed and a request that changes nothing is not sent. A stale revision
 * reloads the roles and keeps what was typed. The Admin's dialog says it
 * always keeps every permission, which is also why it offers no way to
 * change them.
 */
export function EditRoleDialog({
  role,
  revision,
  roles,
  onClose,
  returnFocus,
}: Readonly<{
  role: RoleSummary | null;
  revision: number;
  roles: readonly RoleSummary[];
  onClose: () => void;
  returnFocus?: RefObject<HTMLElement | null> | undefined;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const fields = useMemo(() => editRoleFields(t), [t]);
  const taken = useMemo(
    () => takenNames(roles, t, role?.id ?? null),
    [roles, t, role?.id],
  );
  const initial = useMemo<EditRoleValues>(
    () => ({
      icon: role?.icon ?? "key-round",
      name: role === null ? "" : roleName(role, t),
      description: role === null ? "" : roleDescription(role, t),
      color: role?.color ?? "gray",
    }),
    [role, t],
  );

  const save = useMutation({
    mutationFn: async (values: EditRoleValues) => {
      if (role === null) throw new Error("no role to edit");
      const body: RoleEdit = { expectedRevision: revision };
      if (values.icon !== initial.icon) body.icon = values.icon;
      if (values.color !== initial.color) body.color = values.color;
      if (values.name.trim() !== initial.name) body.name = values.name.trim();
      if (values.description.trim() !== initial.description.trim())
        body.description = values.description.trim();
      if (Object.keys(body).length === 1) return null;
      return updateRole(role.id, body);
    },
    onSuccess: (updated) => {
      onClose();
      if (updated === null) return;
      queryClient.setQueryData<RoleList>(
        ROLES_KEY,
        (old) => old && withRoles(old, [updated]),
      );
      notify.success(t("roles.editRole.saved"));
    },
    onError: (cause) => reloadAfterRefusal(cause, queryClient),
  });

  let error: string | null = null;
  if (save.isError) {
    if (isCode(save.error, "ROLE_NAME_TAKEN")) error = t("roles.editRole.taken");
    else if (isCode(save.error, "STALE_WRITE")) error = t("roles.editRole.stale");
    else error = saveError(save.error, t("roles.editRole.failed"));
  }

  return (
    <FormDialog
      open={role !== null}
      onOpenChange={(next) => {
        if (next) return;
        save.reset();
        onClose();
      }}
      title={t("roles.editRole.title")}
      description={t(
        role?.builtinKey === "admin"
          ? "roles.editRole.adminHint"
          : "roles.editRole.hint",
      )}
      icon={Pencil}
      width={500}
      initial={initial}
      fields={fields}
      validate={(values) =>
        taken.has(nameKeyOf(values.name)) ? { name: t("roles.editRole.taken") } : null
      }
      submitLabel={t("roles.editRole.submit")}
      pending={save.isPending}
      error={error}
      returnFocus={returnFocus}
      onSubmit={(values) => {
        if (!save.isPending) save.mutate(values);
      }}
    />
  );
}
