import { useCallback, useMemo, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useSearchParams } from "react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Callout } from "@/components/shared/Callout";
import { DirtyBar } from "@/components/shared/DirtyBar";
import type { PermissionKey } from "@/features/auth/permissions";
import {
  ROLES_KEY,
  saveRolePermissions,
  withRoles,
  type RoleList,
  type RoleSummary,
} from "@/features/roles/api";
import {
  changesOf,
  NO_DRAFT,
  revisionsOf,
  takingNow,
  toggled,
  type Caller,
  type Draft,
} from "@/features/roles/cells";
import { PermissionMatrix } from "@/features/roles/components/PermissionMatrix";
import { RoleCarousel } from "@/features/roles/components/RoleCarousel";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { notify } from "@/lib/toast";
import { useAuthStore } from "@/stores/auth";

const REFRESHING = new Set(["STALE_WRITE", "TARGET_ROLE_NOT_ALLOWED"]);

/**
 * RolesBoard is the loaded Roles & permissions page: the carousel of roles,
 * the matrix and the unsaved-changes bar. The cells the Admin has turned are
 * a draft held here, by cell, so a change turned back leaves it and the count
 * is of cells that differ from the server. Save sends them as one
 * `setRolePermissions`; a stale revision reloads the roles and keeps the
 * draft. `?role=<id>` names the column the matrix frames.
 */
export function RolesBoard({
  list,
  reveal = null,
  action,
}: Readonly<{
  list: RoleList;
  reveal?: string | null;
  action?: ((role: RoleSummary) => ReactNode) | undefined;
}>) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const roleId = useAuthStore((s) => s.user?.role?.id ?? null);
  const permissions = useAuthStore((s) => s.user?.permissions);
  const reduced = useMediaQuery("(prefers-reduced-motion: reduce)");
  const matrix = useRef<HTMLDivElement>(null);
  const [draft, setDraft] = useState<Draft>(NO_DRAFT);
  const [failure, setFailure] = useState<string | null>(null);

  const caller = useMemo<Caller>(
    () => ({ roleId, holds: new Set(permissions ?? []) }),
    [roleId, permissions],
  );
  const changes = useMemo(
    () => changesOf(draft, list.roles, list.catalogue),
    [draft, list.roles, list.catalogue],
  );
  const wanted = params.get("role");
  const highlight =
    wanted !== null && list.roles.some((role) => role.id === wanted) ? wanted : null;
  const behavior = reduced ? "auto" : "smooth";

  const save = useMutation({
    mutationFn: () => saveRolePermissions(changes, revisionsOf(changes, list.roles)),
    onSuccess: (result) => {
      queryClient.setQueryData<RoleList>(
        ROLES_KEY,
        (old) => old && withRoles(old, result.roles),
      );
      setDraft(NO_DRAFT);
      setFailure(null);
      notify.success(t("roles.saved"));
    },
    onError: (cause) => {
      const stale = cause instanceof ApiError && cause.code === "STALE_WRITE";
      setFailure(
        stale ? t("roles.stale") : failureMessage(cause, t("roles.saveFailed")),
      );
      if (cause instanceof ApiError && REFRESHING.has(cause.code))
        void queryClient.invalidateQueries({ queryKey: ROLES_KEY });
    },
  });

  function frame(role: RoleSummary, toggle: boolean) {
    setParams(
      (previous) => {
        const next = new URLSearchParams(previous);
        if (toggle && next.get("role") === role.id) next.delete("role");
        else next.set("role", role.id);
        return next;
      },
      { replace: true },
    );
  }

  function editPermissions(role: RoleSummary) {
    frame(role, false);
    const table = matrix.current;
    if (table === null) return;
    table.scrollIntoView({ block: "start", behavior });
    Array.from(table.querySelectorAll<HTMLElement>("[data-pos]"))
      .find((cell) => cell.dataset["pos"] === `0:${list.roles.indexOf(role)}`)
      ?.focus({ preventScroll: true });
  }

  function toggle(role: RoleSummary, key: PermissionKey) {
    if (save.isPending) return;
    setFailure(null);
    setDraft((previous) => toggled(previous, role, key));
  }

  const returnFocus = useCallback(
    () =>
      matrix.current?.querySelector<HTMLElement>('[data-pos][tabindex="0"]') ?? null,
    [],
  );
  const taking = takingNow(changes, list.roles);
  const dirty = changes.length > 0;

  return (
    <>
      <RoleCarousel
        roles={list.roles}
        reveal={reveal}
        action={action}
        onEditPermissions={editPermissions}
      />
      <PermissionMatrix
        ref={matrix}
        roles={list.roles}
        catalogue={list.catalogue}
        draft={draft}
        caller={caller}
        highlight={highlight}
        onToggle={toggle}
        onHighlight={(role) => {
          frame(role, true);
          matrix.current?.scrollIntoView({ block: "nearest", behavior });
        }}
      />
      <div className="sticky bottom-3 z-10 flex flex-col gap-2 empty:hidden">
        {dirty && failure !== null && (
          <Callout tone="danger" size="md" announce onDismiss={() => setFailure(null)}>
            {failure}
          </Callout>
        )}
        <DirtyBar
          dirty={dirty}
          saving={save.isPending}
          message={
            <>
              {t("roles.changed", { count: changes.length })}
              {taking > 0 && (
                <span className="text-warning-ink mt-0.5 block">
                  {t("roles.takingNow", { count: taking })}
                </span>
              )}
            </>
          }
          returnFocus={returnFocus}
          onDiscard={() => {
            setDraft(NO_DRAFT);
            setFailure(null);
          }}
          onSave={() => save.mutate()}
        />
      </div>
    </>
  );
}
