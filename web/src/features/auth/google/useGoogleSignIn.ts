import { authStore, useAuthStore, type ActorLease } from "@/stores/auth";
import { transitionStatus } from "@/lib/api/authTransition";
import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { buildAuthorizationRequest, rememberPending } from "./pkce";

/** Public config (§5.3). Absent in a deployment without Google sign-in. */
const CLIENT_ID = import.meta.env["VITE_GOOGLE_CLIENT_ID"] as string | undefined;

export function googleSignInAvailable(): boolean {
  return typeof CLIENT_ID === "string" && CLIENT_ID.length > 0;
}

/** useGoogleSignIn starts an actor-bound PKCE flow and ignores departed UI outcomes. */
export function useGoogleSignIn() {
  const { t } = useTranslation();
  const mounted = useRef(true);
  const attempt = useRef(0);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const generation = useAuthStore((state) => state.actorGeneration);
  const userId = useAuthStore((state) => state.user?.id ?? null);
  const [failure, setFailure] = useState<{
    actor: ActorLease;
    message: string;
  } | null>(null);
  const [pendingActor, setPendingActor] = useState<ActorLease | null>(null);
  const error =
    failure?.actor.generation === generation && failure.actor.userId === userId
      ? failure.message
      : null;
  const pending =
    pendingActor?.generation === generation && pendingActor.userId === userId;

  async function start(
    options: {
      mode?: "signin" | "link";
      next?: string | undefined;
      joinCode?: string | undefined;
    } = {},
  ) {
    if (!CLIENT_ID) {
      setFailure({
        actor: authStore.captureActor(),
        message: t("login.googleUnavailable"),
      });
      return;
    }
    if (transitionStatus().kind !== "idle") {
      setFailure({
        actor: authStore.captureActor(),
        message: t("auth.transition.accountPending"),
      });
      return;
    }
    const lease = authStore.captureActor();
    const ownAttempt = ++attempt.current;
    setFailure(null);
    setPendingActor(lease);
    try {
      const request = await buildAuthorizationRequest({
        clientId: CLIENT_ID,
        mode: options.mode ?? "signin",
        next: options.next,
        joinCode: options.joinCode,
      });
      if (
        !mounted.current ||
        !authStore.isCurrent(lease) ||
        attempt.current !== ownAttempt
      )
        return;
      if (transitionStatus().kind !== "idle") {
        setFailure({ actor: lease, message: t("auth.transition.accountPending") });
        setPendingActor(null);
        return;
      }
      rememberPending(request.pending);
      window.location.assign(request.url);
    } catch {
      if (
        !mounted.current ||
        !authStore.isCurrent(lease) ||
        attempt.current !== ownAttempt
      )
        return;
      setFailure({
        actor: authStore.captureActor(),
        message: t("login.googleUnavailable"),
      });
      setPendingActor(null);
    }
  }

  return { start, error, pending };
}
