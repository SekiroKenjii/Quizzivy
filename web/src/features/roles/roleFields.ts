import type { TFunction } from "i18next";
import type { FormField, FormOption } from "@/components/shared/form/FormDialog";
import type { RoleColor, RoleIcon, RoleSummary } from "@/features/roles/api";
import { iconOf, LOOKS, ROLE_COLORS, ROLE_ICONS } from "@/features/roles/roleLook";
import { ApiError, fieldMessages } from "@/lib/api/errors";
import { roleName } from "@/lib/roleName";

/** NewRoleValues are the words and the source a New role dialog collects. */
export type NewRoleValues = { name: string; from: string; description: string };

/** EditRoleValues are what the Edit role dialog changes: look, name and description. */
export type EditRoleValues = {
  icon: RoleIcon;
  name: string;
  description: string;
  color: RoleColor;
};

/** NO_SOURCE is the "copy from" choice that starts a role with no permissions. */
export const NO_SOURCE = "";

const NAME_MAX = 60;
const DESCRIPTION_MAX = 300;
const SEGMENT_MAX = 4;

/**
 * sourcesOf returns the roles a new role may copy its permissions from: the
 * Teacher, the Student and the custom roles the caller may act on. The Admin
 * holds its permissions by wildcard and the Assistant is refused as a source.
 */
export function sourcesOf(roles: readonly RoleSummary[]): RoleSummary[] {
  return roles.filter(
    (role) =>
      role.editable && role.builtinKey !== "admin" && role.builtinKey !== "assistant",
  );
}

/**
 * newRoleFields are the New role dialog's fields: a name, the role to copy
 * from (segments while there are few to choose from, else a list) and a
 * description.
 */
export function newRoleFields(
  t: TFunction,
  roles: readonly RoleSummary[],
): readonly FormField<NewRoleValues>[] {
  const options: FormOption[] = [
    { value: NO_SOURCE, label: t("roles.newRole.copyNone") },
    ...sourcesOf(roles).map((role) => ({ value: role.id, label: roleName(role, t) })),
  ];
  return [
    {
      kind: "text",
      name: "name",
      label: t("roles.newRole.name"),
      placeholder: t("roles.newRole.namePlaceholder"),
      required: true,
      maxLength: NAME_MAX,
    },
    {
      kind: options.length <= SEGMENT_MAX ? "seg" : "select",
      name: "from",
      label: t("roles.newRole.copyFrom"),
      options,
    },
    {
      kind: "text",
      name: "description",
      label: t("roles.newRole.description"),
      placeholder: t("roles.newRole.descriptionPlaceholder"),
      maxLength: DESCRIPTION_MAX,
    },
  ];
}

/** editRoleFields are the Edit role dialog's fields: the twelve icons, a name, a description and the seven colours. */
export function editRoleFields(t: TFunction): readonly FormField<EditRoleValues>[] {
  return [
    {
      kind: "icons",
      name: "icon",
      label: t("roles.editRole.icon"),
      options: ROLE_ICONS.map((icon) => ({
        value: icon,
        label: t(`roles.icons.${icon}`),
        icon: iconOf(icon),
      })),
    },
    {
      kind: "text",
      name: "name",
      label: t("roles.editRole.name"),
      required: true,
      maxLength: NAME_MAX,
    },
    {
      kind: "area",
      name: "description",
      label: t("roles.editRole.description"),
      maxLength: DESCRIPTION_MAX,
    },
    {
      kind: "colors",
      name: "color",
      label: t("roles.editRole.color"),
      options: ROLE_COLORS.map((color) => ({
        value: color,
        label: t(`roles.colors.${color}`),
        swatch: LOOKS[color].swatch,
      })),
    },
  ];
}

/**
 * nameKeyOf is how two role names compare: composed, trimmed and without
 * letter case, as the server compares them.
 */
export function nameKeyOf(name: string): string {
  return name.normalize("NFC").trim().toLocaleLowerCase();
}

/**
 * takenNames returns the names `roles` go by, stored and as shown, so a name
 * typed in either language is caught before the server is asked. `except` is
 * the role being edited, whose own name is not a clash.
 */
export function takenNames(
  roles: readonly RoleSummary[],
  t: TFunction,
  except: string | null = null,
): ReadonlySet<string> {
  const names = new Set<string>();
  for (const role of roles) {
    if (role.id === except) continue;
    names.add(nameKeyOf(role.name));
    names.add(nameKeyOf(roleName(role, t)));
  }
  return names;
}

/**
 * saveError is what a role dialog says when its save fails: the first field
 * the server named, else the server's message, else `fallback`.
 */
export function saveError(cause: unknown, fallback: string): string {
  const field = fieldMessages(cause)[0];
  if (field !== undefined) return field;
  return cause instanceof ApiError ? cause.message : fallback;
}

/** blankAsAbsent trims a field and leaves a blank one out of the request. */
export function blankAsAbsent(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed === "" ? undefined : trimmed;
}
