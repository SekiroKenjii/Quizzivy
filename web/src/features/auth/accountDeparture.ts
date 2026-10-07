import type { QueryClient } from "@tanstack/react-query";
import { authStore, useAuthStore, type ActorLease } from "@/stores/auth";
import { useAppState } from "@/stores/appState";
import { clearAuthoringDrafts } from "@/lib/drafts/store";
import { clearAnswerDrafts } from "@/features/take-test/draft";
import { clearGroupPlayDrafts } from "@/features/take-test/groupPlaybackDraft";
import {
  finishTransition,
  isTransitionCurrent,
  setTransitionPhase,
  waitForTransition,
  type AccountTransition,
} from "@/lib/api/authTransition";

let recover: (() => Promise<void>) | null = null;

/** beginAccountDeparture invalidates mounted account data and retains cleanup under its original admission owner. */
export function beginAccountDeparture(
  ticket: AccountTransition,
  client: QueryClient,
  signedOut: boolean,
  completed?: (actor: ActorLease, interested: boolean) => Promise<void>,
) {
  if (signedOut) useAuthStore.getState().signOut();
  else useAuthStore.getState().clearSession();
  const actor = authStore.captureActor();
  void client.cancelQueries();
  client.clear();
  clearAnswerDrafts();
  clearGroupPlayDrafts();
  if (useAppState.getState().overlay.kind === "expired")
    useAppState.getState().closeOverlay();
  let cleanup: Promise<void> | null = null;
  const retry = () => {
    if (!authStore.isCurrent(actor)) return Promise.resolve();
    recover = null;
    ticket.interested = true;
    return waitForTransition(ticket, clean(true));
  };
  const clean = (recovery = false): Promise<void> => {
    if (cleanup) return cleanup;
    setTransitionPhase(ticket, "cleanup");
    cleanup = clearAuthoringDrafts().then(
      async () => {
        if (recover === retry) recover = null;
        const interested = isTransitionCurrent(ticket) && authStore.isCurrent(actor);
        if (completed || recovery || !interested) finishTransition(ticket);
        else setTransitionPhase(ticket, "account");
        await completed?.(actor, interested);
      },
      (error: unknown) => {
        cleanup = null;
        recover = retry;
        throw error;
      },
    );
    return cleanup;
  };
  return { actor, clean };
}

/** checkAccountDeparture retries only naturally failed cleanup without replaying discarded credentials. */
export function checkAccountDeparture() {
  return recover?.() ?? Promise.resolve();
}

/** accountDepartureNeedsRetry distinguishes naturally failed cleanup from an active owned operation. */
export function accountDepartureNeedsRetry() {
  return recover !== null;
}
