import type { QueryClient } from "@tanstack/react-query";

/** Everything a membership change makes stale. */
export function invalidateClassMembership(client: QueryClient, classId: string) {
  return Promise.all([
    client.invalidateQueries({ queryKey: ["admin-class-members", classId] }),
    client.invalidateQueries({ queryKey: ["admin-class", classId] }),
    client.invalidateQueries({ queryKey: ["admin-classes"] }),
  ]);
}

/** A class's own row and every list or picker that carries it. */
export function invalidateClass(client: QueryClient, classId: string) {
  return Promise.all([
    client.invalidateQueries({ queryKey: ["admin-class", classId] }),
    client.invalidateQueries({ queryKey: ["admin-classes"] }),
  ]);
}
