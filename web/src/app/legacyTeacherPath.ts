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

/** assignmentStudentsLocation translates canonical and legacy paper addresses without losing roster filters or unrelated URL values. */
export function assignmentStudentsLocation(
  location: Readonly<{ pathname: string; search: string; hash: string }>,
) {
  const path = legacyTeacherPath(location.pathname) ?? location.pathname;
  const match = /^\/teacher\/assignments\/([^/]+)\/attempts\/?$/i.exec(path);
  if (!match) return null;
  const params = new URLSearchParams(location.search);
  const filter = params.get("tab");
  if (
    filter === "all" ||
    filter === "submitted" ||
    filter === "pending" ||
    filter === "flagged" ||
    filter === "notStarted"
  )
    params.set("roster", filter);
  params.set("tab", "students");
  return {
    pathname: `/teacher/assignments/${match[1]}`,
    search: `?${params.toString()}`,
    hash: location.hash,
  };
}
