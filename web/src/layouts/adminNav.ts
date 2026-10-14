import {
  BookOpenText,
  GraduationCap,
  KeyRound,
  LayoutDashboard,
  ScrollText,
  Settings,
  Sparkles,
  Users,
} from "lucide-react";
import { ADMIN_PATHS } from "@/app/adminPaths";
import { holds, type Requirement } from "@/features/auth/permissions";
import type { NavDestination, NavGroup } from "@/layouts/shell/nav";
import type { components } from "@/lib/api/schema";

type NavUser = Pick<components["schemas"]["CurrentUser"], "permissions" | "workspaces">;

/**
 * AdminNavItem is one destination of the Admin console's sidebar.
 * `operation` is the `operationId` of the read the destination's page opens
 * with, and `requires` that operation's `x-permission`.
 */
export interface AdminNavItem extends NavDestination {
  operation: string;
  requires: Requirement;
}

/** AdminNavGroup is a run of destinations under one label; the first has none. */
export interface AdminNavGroup extends NavGroup {
  items: readonly AdminNavItem[];
}

/**
 * ADMIN_NAV is the Admin console's sidebar in the deck's order. The Users
 * badge counts the people who must change their password and the AI credits
 * badge the requests waiting for an Admin; both are hidden at zero and while
 * the sidebar is collapsed.
 */
export const ADMIN_NAV: readonly AdminNavGroup[] = [
  {
    id: "home",
    items: [
      {
        id: "overview",
        to: ADMIN_PATHS.overview,
        label: "adminShell.nav.overview",
        icon: LayoutDashboard,
        operation: "getAdminOverview",
        requires: "workspace.admin",
      },
    ],
  },
  {
    id: "people",
    label: "adminShell.group.people",
    items: [
      {
        id: "users",
        to: ADMIN_PATHS.users,
        label: "adminShell.nav.users",
        icon: Users,
        operation: "listUsers",
        requires: "people.users.manage",
        count: {
          figure: "mustChangePassword",
          tone: "warning",
          sentence: "adminShell.count.mustChangePassword",
        },
      },
      {
        id: "roles",
        to: ADMIN_PATHS.roles,
        label: "adminShell.nav.roles",
        icon: KeyRound,
        operation: "listRoles",
        requires: "people.roles.manage",
      },
    ],
  },
  {
    id: "teaching",
    label: "adminShell.group.teaching",
    items: [
      {
        id: "classes",
        to: ADMIN_PATHS.classes,
        label: "adminShell.nav.classes",
        icon: GraduationCap,
        operation: "listAllClasses",
        requires: "scope.all",
      },
    ],
  },
  {
    id: "system",
    label: "adminShell.group.system",
    items: [
      {
        id: "ai",
        to: ADMIN_PATHS.ai,
        label: "adminShell.nav.ai",
        icon: Sparkles,
        operation: "getAiCredits",
        requires: "ai.credits.manage",
        count: {
          figure: "creditRequests",
          tone: "urgent",
          sentence: "adminShell.count.creditRequests",
        },
      },
      {
        id: "audit",
        to: ADMIN_PATHS.audit,
        label: "adminShell.nav.audit",
        icon: ScrollText,
        operation: "listAuditLog",
        requires: "system.audit.read",
      },
      {
        id: "api",
        to: ADMIN_PATHS.api,
        label: "adminShell.nav.api",
        icon: BookOpenText,
        operation: "getApiReferenceSummary",
        requires: "system.api_reference",
      },
      {
        id: "settings",
        to: ADMIN_PATHS.settings,
        label: "adminShell.nav.settings",
        icon: Settings,
        operation: "getOrganizationSettings",
        requires: "system.settings.write",
      },
    ],
  },
];

/**
 * adminNavFor returns the sidebar for `user`: the destinations whose
 * requirement the user holds, in order. A group left with no destination is
 * dropped.
 */
export function adminNavFor(user: NavUser | null | undefined): AdminNavGroup[] {
  return ADMIN_NAV.map((group) => ({
    ...group,
    items: group.items.filter((item) => holds(user, item.requires)),
  })).filter((group) => group.items.length > 0);
}

/**
 * activeAdminNavId returns the id of the destination `pathname` belongs to,
 * or null when it belongs to none, as an export page does. The bare console
 * address is the Overview; any other destination owns its own path and every path
 * beneath it.
 */
export function activeAdminNavId(pathname: string): string | null {
  let end = pathname.length;
  while (end > 1 && pathname[end - 1] === "/") end -= 1;
  const path = pathname.slice(0, end).toLowerCase();
  if (path === ADMIN_PATHS.overview) return "overview";
  for (const group of ADMIN_NAV) {
    for (const item of group.items) {
      if (
        item.id !== "overview" &&
        (path === item.to || path.startsWith(`${item.to}/`))
      )
        return item.id;
    }
  }
  return null;
}
