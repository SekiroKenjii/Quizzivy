import { useTranslation } from "react-i18next";
import { Pencil } from "lucide-react";
import type { RoleSummary } from "@/features/roles/api";
import { LOOKS } from "@/features/roles/roleLook";
import { roleName } from "@/lib/roleName";
import { cn } from "@/lib/utils";

/**
 * EditRoleButton is the pencil on a role's card. A role the subset rule keeps
 * the caller from answers `aria-disabled` with the reason as its title, stays
 * focusable and ignores a press; its colours fade, never the button.
 */
export function EditRoleButton({
  role,
  onEdit,
}: Readonly<{
  role: RoleSummary;
  onEdit: (role: RoleSummary, trigger: HTMLElement) => void;
}>) {
  const { t } = useTranslation();
  return (
    <button
      type="button"
      data-edit-role={role.id}
      aria-label={t("roles.editRole.pencil", { name: roleName(role, t) })}
      aria-disabled={role.editable ? undefined : true}
      title={role.editable ? t("roles.editRole.title") : t("roles.lock.subset")}
      className={cn(
        "grid size-8.5 place-items-center rounded-[0.5625rem] border text-inherit transition-opacity",
        "aria-disabled:cursor-not-allowed aria-disabled:border-current/30 aria-disabled:text-current/50",
        role.editable && "hover:opacity-85",
        LOOKS[role.color].button,
      )}
      onClick={(event) => {
        if (role.editable) onEdit(role, event.currentTarget);
      }}
    >
      <Pencil aria-hidden="true" className="size-3.75" />
    </button>
  );
}
