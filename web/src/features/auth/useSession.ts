import { clearAuthoringDrafts } from "@/lib/drafts/store";
import { useEffect } from "react";
import { clearGroupPlayDrafts } from "@/features/take-test/groupPlaybackDraft";
import { clearAnswerDrafts } from "@/features/take-test/draft";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { fetchCurrentUser, logout as logoutRequest } from "./api";
import { ApiError, maintenanceWindow } from "@/lib/api/errors";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";

/**
 * useBootstrapSession restores the session on app load (§5.4), and again each
 * time `retryBoot` asks. A 401 means signed out. A request that got no answer
 * leaves the session as it was and marks boot `offline`; a 503 `MAINTENANCE`
 * raises the maintenance overlay; anything else marks boot `failed` with the
 * error, for the unexpected-error page.
 */
export function useBootstrapSession() {
  const setSessionUser = useAuthStore((s) => s.setUser);
  const finishBootstrap = useAuthStore((s) => s.finishBootstrap);
  const clearSession = useAuthStore((s) => s.clearSession);
  const bootAttempt = useAppState((s) => s.bootAttempt);
  const setBootPhase = useAppState((s) => s.setBootPhase);
  const showOverlay = useAppState((s) => s.showOverlay);

  useEffect(() => {
    const controller = new AbortController();
    void (async () => {
      try {
        const user = await fetchCurrentUser(controller.signal);
        setSessionUser(user);
        finishBootstrap();
        setBootPhase("ready");
      } catch (cause) {
        if (controller.signal.aborted) return;
        const window = maintenanceWindow(cause);
        if (window) {
          showOverlay({ kind: "maintenance", window });
        } else if (isSignedOut(cause)) {
          clearSession();
          setBootPhase("ready");
        } else if (isUnanswered(cause)) {
          setBootPhase("offline");
        } else {
          setBootPhase(
            "failed",
            cause instanceof Error ? cause : new Error(String(cause)),
          );
        }
      }
    })();
    return () => controller.abort();
  }, [
    bootAttempt,
    setSessionUser,
    clearSession,
    finishBootstrap,
    setBootPhase,
    showOverlay,
  ]);
}

function isSignedOut(cause: unknown): boolean {
  return cause instanceof ApiError && (cause.status === 401 || cause.status === 403);
}

function isUnanswered(cause: unknown): boolean {
  return (
    cause instanceof TypeError || (cause instanceof ApiError && cause.status === 0)
  );
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
    clearAnswerDrafts();
    clearGroupPlayDrafts();
    await clearAuthoringDrafts().catch(() => undefined);
    queryClient.clear();
  };
}
