import type { QueryClient } from "@tanstack/react-query";

/**
 * Everything a membership change makes stale.
 *
 * The list query is the one that gets forgotten: it is not on screen when a
 * member is added, so its `studentCount` stays at the old number and reappears
 * later somewhere else -- G-01's roster estimate read "tối đa 1" for a class
 * that had just gained its second student.
 */
export function invalidateClassMembership(client: QueryClient, classId: string) {
  return Promise.all([
    client.invalidateQueries({ queryKey: ["admin-class-members", classId] }),
    client.invalidateQueries({ queryKey: ["admin-class", classId] }),
    client.invalidateQueries({ queryKey: ["admin-classes"] }),
  ]);
}
