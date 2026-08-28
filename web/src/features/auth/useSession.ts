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
        // Any failure means "no session": a 401 after the single refresh
        // attempt, or the API being unreachable. Both send the user to /login,
        // which is the only screen that works without one.
        clearSession();
      } finally {
        finishBootstrap();
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
      // The server call failing must not strand the user in a session they
      // asked to leave. The refresh token may outlive this, which is a smaller
      // problem than a logout button that does nothing.
    }
    // Leave the guarded route BEFORE forgetting the session. Clearing first
    // re-renders the page the user is still on, RequireSession sees no user,
    // and it redirects to `/login?next=<that page>` -- so the next person to
    // sign in on this device inherits the previous user's destination. Seen
    // for real: a student signed in after a teacher signed out of
    // /admin/classes and was sent straight to a 403.
    await navigate("/login", { replace: true });
    clearSession();
    queryClient.clear();
  };
}
