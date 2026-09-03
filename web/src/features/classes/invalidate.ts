import type { QueryClient } from "@tanstack/react-query";

/** Everything a membership change makes stale. */
export function invalidateClassMembership(client: QueryClient, classId: string) {
  return Promise.all([
    client.invalidateQueries({ queryKey: ["admin-class-members", classId] }),
    client.invalidateQueries({ queryKey: ["admin-class", classId] }),
    client.invalidateQueries({ queryKey: ["admin-classes"] }),
  ]);
}
