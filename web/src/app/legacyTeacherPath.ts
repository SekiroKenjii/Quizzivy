const LEGACY_PREFIX = /^\/admin(?=\/|$)/i;

/**
 * legacyTeacherPath maps a teacher address from before R4 to the address that
 * replaced it: `/admin` to `/teacher` and `/admin/<rest>` to `/teacher/<rest>`,
 * in any letter case, as the router matches it. Any other path gives null.
 */
export function legacyTeacherPath(pathname: string): string | null {
  if (!LEGACY_PREFIX.test(pathname)) return null;
  return pathname.replace(LEGACY_PREFIX, "/teacher");
}
