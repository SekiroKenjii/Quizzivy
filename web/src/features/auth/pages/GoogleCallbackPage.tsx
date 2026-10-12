import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import type { TFunction } from "i18next";
import type { components } from "@/lib/api/schema";
import type { PendingAuthorization } from "@/features/auth/google/pkce";
import { useTranslation } from "react-i18next";
import { ArrowLeft, CircleAlert, LoaderCircle } from "lucide-react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { callbackUrl, statesMatch, takePending } from "@/features/auth/google/pkce";
import { destinationAfterSignIn, preloadStudentHome } from "@/features/auth/home";
import { authStore } from "@/stores/auth";
import { useAppState } from "@/stores/appState";
import { runAccountMutation, signInAccount } from "@/features/auth/accountPreferences";
import { TransitionPendingError } from "@/lib/api/authTransition";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { AuthLayout } from "@/features/auth/AuthLayout";
import {
  clearJoinContext,
  joinOutcomeState,
  readJoinContext,
} from "@/features/join/context";

/** Where Google sends the browser back (§5.3 step 2). */
export default function GoogleCallbackPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [params] = useSearchParams();
  const mounted = useRef(true);
  const [error, setError] = useState<string | null>(null);
  // Effects run twice under StrictMode, and this one redeems a single-use code.
  const started = useRef(false);

  useEffect(() => {
    mounted.current = true;
    if (started.current)
      return () => {
        mounted.current = false;
      };
    started.current = true;

    let lease = authStore.captureActor();
    void (async () => {
      const pending = takePending();
      const code = params.get("code");
      const state = params.get("state");
      if (params.get("error")) {
        await navigate("/login", { replace: true });
        return;
      }
      if (!pending || !code || !state || !statesMatch(pending.state, state)) {
        setError(t("login.googleFailed"));
        return;
      }
      try {
        if (pending.mode === "link") {
          const linked = await runAccountMutation(() =>
            api("post", "/auth/google/link", {
              body: {
                code,
                codeVerifier: pending.verifier,
                redirectUri: callbackUrl(),
              },
            }),
          );
          if (!linked || !mounted.current || !authStore.isCurrent(lease)) return;
          await navigate(destinationAfterSignIn(pending.next, linked), {
            replace: true,
          });
          return;
        }

        // A visitor holding a class code is a student, whatever the exchange says next.
        if (pending.joinCode) preloadStudentHome();

        const operation = signInAccount(
          "google",
          (ticket) =>
            api("post", "/auth/google", {
              transition: ticket,
              body: {
                code,
                codeVerifier: pending.verifier,
                redirectUri: callbackUrl(),
                ...(pending.joinCode ? { joinCode: pending.joinCode } : {}),
              },
            }),
          queryClient,
          () => mounted.current,
        );
        lease = authStore.captureActor();
        const result = await operation;
        if (!mounted.current || !authStore.isCurrent(result.actor)) return;
        useAppState.getState().setBootPhase("ready");
        const destination = googleDestination(pending, result);
        await navigate(destination.to, {
          replace: true,
          ...(destination.state ? { state: destination.state } : {}),
        });
      } catch (cause) {
        if (!mounted.current || !authStore.isCurrent(lease)) return;
        setError(callbackError(cause, t));
      }
    })();
    return () => {
      mounted.current = false;
    };
  }, [params, navigate, queryClient, t]);

  if (error) {
    return (
      <AuthLayout>
        <div>
          <h1 className="text-h1">{t("login.googleFailedTitle")}</h1>
        </div>
        <Alert variant="danger" className="text-ui flex gap-2.5">
          <CircleAlert aria-hidden="true" className="mt-px size-4 shrink-0" />
          <span>{error}</span>
        </Alert>
        <Button
          asChild
          variant="outline"
          size="xl"
          className="bg-card hover:bg-muted w-full font-medium"
        >
          <Link to="/login" replace>
            <ArrowLeft aria-hidden="true" className="size-[17px]" />
            {t("login.backToSignIn")}
          </Link>
        </Button>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout>
      <p role="status" className="text-muted-fg flex items-center gap-2.5">
        <LoaderCircle aria-hidden="true" className="size-[17px] animate-spin" />
        {t("login.completingGoogle")}
      </p>
    </AuthLayout>
  );
}

function callbackError(cause: unknown, t: TFunction) {
  if (cause instanceof TransitionPendingError)
    return t("auth.transition.accountPending");
  if (cause instanceof ApiError) return cause.message;
  return t("login.googleFailed");
}

function googleDestination(
  pending: PendingAuthorization,
  result: components["schemas"]["GoogleSignInSuccess"],
) {
  if (pending.joinCode && result.enrolledClass) {
    const teacherName = readJoinContext()?.teacherName ?? "";
    clearJoinContext();
    return {
      to: `/join/${pending.joinCode}`,
      state: joinOutcomeState({
        kind: "joined",
        className: result.enrolledClass.name,
        teacherName,
      }),
    };
  }
  return {
    to: pending.joinCode
      ? `/join/${pending.joinCode}`
      : destinationAfterSignIn(pending.next, result.user),
  };
}
