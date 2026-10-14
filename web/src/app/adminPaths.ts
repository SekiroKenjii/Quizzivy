/** ADMIN_HOME is the Admin console's Overview, the console's own address. */
export const ADMIN_HOME = "/admin";

/**
 * ADMIN_PATHS are the console's addresses in the sidebar's order. Settings
 * takes a section beneath it (`SETTINGS_SECTIONS`); an export takes its id.
 */
export const ADMIN_PATHS = {
  overview: ADMIN_HOME,
  users: "/admin/users",
  roles: "/admin/roles",
  classes: "/admin/classes",
  ai: "/admin/ai",
  audit: "/admin/audit",
  api: "/admin/api",
  settings: "/admin/settings",
} as const;

/** SETTINGS_SECTIONS are the sections the System settings page takes after its path. */
export const SETTINGS_SECTIONS: ReadonlySet<string> = new Set([
  "organization",
  "sign-in",
  "data",
  "plan",
]);

const FLAT = new Set<string>(
  Object.values(ADMIN_PATHS).map((path) => path.toLowerCase()),
);

function normalised(pathname: string): string {
  let end = pathname.length;
  while (end > 1 && pathname[end - 1] === "/") end -= 1;
  return pathname.slice(0, end).toLowerCase();
}

/**
 * consolePath reports whether pathname is an address of the Admin console, as
 * the router matches it: in any letter case, with or without a trailing slash.
 * Any other address beneath the old prefix is a teacher address from before R4.
 */
export function consolePath(pathname: string): boolean {
  const path = normalised(pathname);
  if (FLAT.has(path)) return true;
  const rest = path.slice(ADMIN_PATHS.settings.length + 1);
  if (path.startsWith(`${ADMIN_PATHS.settings}/`)) return SETTINGS_SECTIONS.has(rest);
  return /^\/admin\/exports\/[^/]+$/.test(path);
}
