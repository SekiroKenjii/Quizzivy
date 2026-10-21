import type { QueryClient } from "@tanstack/react-query";
import { fetchCurrentUser } from "@/features/auth/api";
import { ROLES_KEY } from "@/features/roles/api";
import { ApiError } from "@/lib/api/errors";
import { authStore, useAuthStore } from "@/stores/auth";

/**
 * reloadAfterRefusal brings the screen back in step with the server after a
 * write it refused for being out of date. A stale revision reloads the roles;
 * `TARGET_ROLE_NOT_ALLOWED` reloads them and the caller's own permissions too,
 * since the matrix disables a cell by what the caller holds and the caller has
 * just learned they no longer hold it. Any other error changes nothing.
 */
export function reloadAfterRefusal(cause: unknown, queryClient: QueryClient) {
  if (!(cause instanceof ApiError)) return;
  if (cause.code !== "STALE_WRITE" && cause.code !== "TARGET_ROLE_NOT_ALLOWED") return;
  void queryClient.invalidateQueries({ queryKey: ROLES_KEY });
  if (cause.code === "TARGET_ROLE_NOT_ALLOWED") void reloadCaller();
}

async function reloadCaller() {
  const lease = authStore.captureActor();
  try {
    const user = await fetchCurrentUser();
    if (authStore.isCurrent(lease)) useAuthStore.getState().setUser(user);
  } catch {
    return;
  }
}
