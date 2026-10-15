import type { TFunction } from "i18next";
import type { components } from "@/lib/api/schema";

type Role = Pick<components["schemas"]["CurrentUserRole"], "builtinKey" | "name">;
type Described = Pick<
  components["schemas"]["RoleSummary"],
  "builtinKey" | "description"
>;

/**
 * roleName is how a role reads on screen: a built-in role by its localised
 * `role.builtin.<key>` string while the stored name is still the seeded one,
 * the Vietnamese string being the seed, and the stored name once an Admin has
 * renamed it, as for every custom role.
 */
export function roleName(role: Role, t: TFunction): string {
  const key = role.builtinKey;
  if (key === null) return role.name;
  const path = `role.builtin.${key}`;
  return role.name === t(path, { lng: "vi" }) ? t(path) : role.name;
}

/**
 * roleDescription is how a role's description reads on screen, by the rule
 * roleName follows: a built-in role by its localised
 * `role.builtinDescription.<key>` string while the stored description is
 * still the seeded one, the Vietnamese string being the seed, and the stored
 * text once an Admin has edited it, as for every custom role.
 */
export function roleDescription(role: Described, t: TFunction): string {
  const key = role.builtinKey;
  if (key === null) return role.description;
  const path = `role.builtinDescription.${key}`;
  return role.description === t(path, { lng: "vi" }) ? t(path) : role.description;
}
