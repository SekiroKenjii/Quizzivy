import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { fetchCurrentUser, logout as logoutRequest } from "./api";
import { useAuthStore } from "@/stores/auth";

/**
 * Restores the session on app load (§5.4).
 *
 * `GET /auth/me` is the whole mechanism: the access token lives in memory and
 * is therefore gone after a reload, but the refresh cookie is not, so the
 * request 401s, the client refreshes once, and retries. A success means the
 * session survived; a 401 after the retry means it did not.
 *
 * `isBootstrapping` exists so guards can WAIT rather than bounce. Without it
 * every reload would flash /login for the duration of one round trip, and a
 * deep link would be lost on the way.
 */
export function useBootstrapSession() {
  const setSessionUser = useAuthStore((s) => s.setUser);
  const finishBootstrap = useAuthStore((s) => s.finishBootstrap);
  const clearSession = useAuthStore((s) => s.clearSession);

  useEffect(() => {
    const controller = new AbortController();
    void (async () => {
      try {
        const user = await fetchCurrentUser(controller.signal);
        setSessionUser(user);
      } catch {
        // An ABORT says nothing about the session -- it says this component
        // went away. Treating it as "signed out" is what made a reload land on
        // /login while the refresh cookie was still perfectly good: React's
        // double-invoked effect aborts the first call, and the bounce happened
        // before the second one could answer.
        if (controller.signal.aborted) return;
        clearSession();
      } finally {
        if (!controller.signal.aborted) finishBootstrap();
      }
    })();
    return () => controller.abort();
  }, [setSessionUser, clearSession, finishBootstrap]);
}

/**
 * §5.4's logout: revoke server-side, then forget everything client-side.
 *
 * `queryClient.clear()` is not housekeeping. React Query's cache would
 * otherwise still hold the previous user's assignments and attempts, and the
 * next person to sign in on this device would see them for as long as it took
 * the refetch to land.
 */
export function useLogout() {
  const queryClient = useQueryClient();
  const clearSession = useAuthStore((s) => s.clearSession);
  const navigate = useNavigate();

  return async function logout() {
    try {
      await logoutRequest();
    } catch {
      // A failed server logout must not strand the user in a signed-in shell.
    }
    await navigate("/login", { replace: true });
    clearSession();
    queryClient.clear();
  };
}
