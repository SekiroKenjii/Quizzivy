import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { fetchCurrentUser, logout as logoutRequest } from "./api";
import { useAuthStore } from "@/stores/auth";

/** Restores the session on app load (§5.4). */
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
        // An ABORT says nothing about the session -- it says this component went away.
        if (controller.signal.aborted) return;
        clearSession();
      } finally {
        if (!controller.signal.aborted) finishBootstrap();
      }
    })();
    return () => controller.abort();
  }, [setSessionUser, clearSession, finishBootstrap]);
}

/** §5.4's logout: revoke server-side, then forget everything client-side. */
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
