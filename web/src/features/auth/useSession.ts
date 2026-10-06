import "./accountPreferences";
import i18n from "@/lib/i18n";
import { toast } from "@/components/ui/sonner";
import {
  transitionStatus,
  reserveTransition,
  waitForTransition,
  TransitionPendingError,
  type AccountTransition,
} from "@/lib/api/authTransition";
import { beginAccountDeparture, checkAccountDeparture } from "./accountDeparture";
import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "react-router";
import { fetchCurrentUser, logout as logoutRequest } from "./api";
import { ApiError, maintenanceWindow } from "@/lib/api/errors";
import { useAppState } from "@/stores/appState";
import { authStore, useAuthStore, type ActorLease } from "@/stores/auth";

/**
 * useBootstrapSession restores the session on app load (§5.4), and again each
 * time `retryBoot` asks. A 401 means signed out, unless a user is in the store
 * when it is handled: a sign-in that finished while the request was out is
 * kept. A request that got no answer leaves the session as it was and marks
 * boot `offline`; a 503 `MAINTENANCE` raises the maintenance overlay; anything
 * else marks boot `failed` with the error, for the unexpected-error page.
 */
export function useBootstrapSession() {
  const setSessionUser = useAuthStore((s) => s.setUser);
  const finishBootstrap = useAuthStore((s) => s.finishBootstrap);
  const bootAttempt = useAppState((s) => s.bootAttempt);
  const setBootPhase = useAppState((s) => s.setBootPhase);

  useEffect(() => {
    const controller = new AbortController();
    const lease = authStore.captureActor();
    void (async () => {
      try {
        const user = await fetchCurrentUser(controller.signal);
        if (
          !bootstrapCurrent(lease, controller.signal) ||
          (lease.userId !== null && user.id !== lease.userId)
        )
          return;
        setSessionUser(user);
        finishBootstrap();
        setBootPhase("ready");
      } catch (cause) {
        if (!bootstrapCurrent(lease, controller.signal)) return;
        settleBootFailure(cause);
      }
    })();
    return () => controller.abort();
  }, [bootAttempt, setSessionUser, finishBootstrap, setBootPhase]);
}

function settleBootFailure(cause: unknown) {
  const state = useAppState.getState();
  const window = maintenanceWindow(cause);
  if (window) {
    state.showOverlay({ kind: "maintenance", window });
    return;
  }
  if (isSignedOut(cause)) {
    if (useAuthStore.getState().user === null) useAuthStore.getState().clearSession();
    state.setBootPhase("ready");
    return;
  }
  if (isUnanswered(cause)) {
    state.setBootPhase("offline");
    return;
  }
  state.setBootPhase(
    "failed",
    cause instanceof Error ? cause : new Error(String(cause)),
  );
}

function bootstrapCurrent(lease: ActorLease, signal: AbortSignal) {
  return (
    !signal.aborted &&
    authStore.isCurrent(lease) &&
    ["idle", "refresh"].includes(transitionStatus().kind)
  );
}

function isSignedOut(cause: unknown): boolean {
  return cause instanceof ApiError && (cause.status === 401 || cause.status === 403);
}

function isUnanswered(cause: unknown): boolean {
  return (
    cause instanceof TypeError || (cause instanceof ApiError && cause.status === 0)
  );
}

/** checkLogoutCleanup retries only a naturally failed cleanup under its original departure owner. */
export function checkLogoutCleanup() {
  return checkAccountDeparture();
}

function reportTransition(
  error: unknown,
  lease: ActorLease = authStore.captureActor(),
) {
  if (!authStore.isCurrent(lease)) return;
  const pending = error instanceof TransitionPendingError;
  let message = "auth.transition.cleanupFailed";
  if (pending)
    message =
      error.phase === "cleanup"
        ? "auth.transition.cleanupPending"
        : "auth.transition.accountPending";
  toast(i18n.t(message), {
    id: `account-transition-${lease.generation}`,
    duration: Infinity,
    action: {
      label: i18n.t(pending ? "auth.transition.checkStatus" : "common.retry"),
      onClick: () => {
        if (authStore.isCurrent(lease))
          void checkLogoutCleanup().catch((cause: unknown) =>
            reportTransition(cause, lease),
          );
      },
    },
  });
}

/** useLogout invalidates local actor work before draining its cookie operation and global draft cleanup. */
export function useLogout() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  return async function logout() {
    let ticket: AccountTransition;
    try {
      ticket = reserveTransition("logout", authStore.getGeneration());
    } catch (error) {
      reportTransition(error);
      return;
    }
    const departure = beginAccountDeparture(
      ticket,
      queryClient,
      true,
      async (lease, interested) => {
        toast.dismiss(`account-transition-${lease.generation}`);
        if (interested) await navigate("/login", { replace: true });
      },
    );
    const lease = departure.actor;
    const raw = logoutRequest(ticket).catch(() => undefined);
    const completion = raw.then(() => departure.clean());
    void completion.catch((error: unknown) => {
      reportTransition(error, lease);
    });
    try {
      await waitForTransition(ticket, raw);
      await waitForTransition(ticket, completion);
    } catch (error) {
      reportTransition(error, lease);
    }
  };
}
