import { useTranslation } from "react-i18next";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Segmented } from "@/components/ui/segmented";
import type { AssignableRole } from "@/features/users/api";
import { roleName } from "@/lib/roleName";

const BUILTIN_ORDER = ["student", "teacher", "assistant", "admin"] as const;

function builtinRank(role: AssignableRole): number {
  return BUILTIN_ORDER.findIndex((key) => key === role.builtinKey);
}

/**
 * RolePicker chooses one of `roles`, the roles the caller may give: the
 * deck's segments, in the order Student, Teacher, Assistant, Admin, while
 * every role offered is a built-in one, and a select of them by name once a
 * custom role exists. `id` names the control for a label outside it.
 */
export function RolePicker({
  id,
  label,
  roles,
  value,
  onChange,
  disabled = false,
}: Readonly<{
  id?: string;
  label: string;
  roles: readonly AssignableRole[];
  value: string;
  onChange: (roleId: string) => void;
  disabled?: boolean;
}>) {
  const { t } = useTranslation();
  if (roles.every((role) => role.builtinKey !== null)) {
    const ordered = [...roles].sort((a, b) => builtinRank(a) - builtinRank(b));
    return (
      <div id={id}>
        <Segmented
          label={label}
          value={value}
          onChange={onChange}
          fill
          options={ordered.map((role) => ({
            value: role.id,
            label: roleName(role, t),
            disabled,
          }))}
        />
      </div>
    );
  }
  return (
    <Select value={value} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger id={id} aria-label={label} className="w-full">
        <SelectValue placeholder={t("users.role.choose")} />
      </SelectTrigger>
      <SelectContent>
        {roles.map((role) => (
          <SelectItem key={role.id} value={role.id}>
            {roleName(role, t)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
