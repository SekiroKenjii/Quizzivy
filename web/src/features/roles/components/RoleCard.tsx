import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { ArrowDown } from "lucide-react";
import type { RoleSummary } from "@/features/roles/api";
import { iconOf, LOOKS } from "@/features/roles/roleLook";
import { IconTile } from "@/components/shared/IconTile";
import { roleName } from "@/lib/roleName";
import { cn } from "@/lib/utils";

function Figure({ value, label }: Readonly<{ value: number; label: string }>) {
  return (
    <div>
      <span className="block text-[22px] leading-[1.1] font-semibold tabular-nums">
        {value}
      </span>
      <span className="text-xs opacity-75">{label}</span>
    </div>
  );
}

/**
 * RoleCard is one role as the deck draws it: its icon tile in the role's
 * colour, a "Custom" pill for a role that is not built in, its name,
 * description, head count and number of permissions, and the button that
 * takes the Admin to its column of the matrix. `action` is the control at the
 * end of the tile row, the pencil that edits the role. A built-in role reads
 * by its localised name until an Admin renames it.
 */
export function RoleCard({
  role,
  className,
  action,
  onEditPermissions,
}: Readonly<{
  role: RoleSummary;
  className?: string | undefined;
  action?: ReactNode;
  onEditPermissions: (role: RoleSummary) => void;
}>) {
  const { t } = useTranslation();
  const look = LOOKS[role.color];
  const name = roleName(role, t);
  return (
    <article
      aria-label={name}
      data-role={role.id}
      className={cn(
        "relative flex min-h-49 flex-none snap-start flex-col gap-3 overflow-hidden rounded-[1rem] border p-5.5",
        look.card,
        className,
      )}
    >
      <div className="flex items-center gap-2.5">
        <IconTile
          icon={iconOf(role.icon)}
          size={40}
          className={cn("flex-none rounded-[0.6875rem]", look.tile)}
        />
        {role.custom && (
          <span className="rounded-full border border-current px-2 py-px text-[0.71875rem] font-semibold opacity-80">
            {t("roles.custom")}
          </span>
        )}
        {action !== undefined && <span className="ml-auto">{action}</span>}
      </div>
      <div className="flex flex-col gap-1">
        <h2 className="min-w-0 text-[22px] leading-[1.2] font-semibold tracking-[-0.02em] break-words">
          {name}
        </h2>
        {role.description !== "" && (
          <p className="text-[0.84375rem] leading-normal text-pretty opacity-80">
            {role.description}
          </p>
        )}
      </div>
      <div className="mt-auto flex flex-wrap items-end justify-between gap-2.5">
        <div className="flex gap-4.5">
          <Figure
            value={role.userCount}
            label={t("roles.peopleLabel", { count: role.userCount })}
          />
          <Figure
            value={role.permissions.length}
            label={t("roles.permissionsLabel", { count: role.permissions.length })}
          />
        </div>
        <button
          type="button"
          aria-label={t("roles.editPermissionsOf", { name })}
          className={cn(
            "inline-flex h-8.5 items-center gap-1.5 rounded-lg border px-3 text-[0.8125rem] font-medium whitespace-nowrap",
            look.button,
          )}
          onClick={() => onEditPermissions(role)}
        >
          {t("roles.editPermissions")}
          <ArrowDown aria-hidden="true" className="size-3.5" />
        </button>
      </div>
    </article>
  );
}
