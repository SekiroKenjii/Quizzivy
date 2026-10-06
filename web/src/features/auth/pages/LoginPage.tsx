import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useForm, type FieldErrors } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { CircleAlert, LoaderCircle } from "lucide-react";
import { Link, useNavigate, useSearchParams } from "react-router";

import { PasswordInput } from "@/components/shared/PasswordInput";
import { Alert } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { AuthLayout } from "@/features/auth/AuthLayout";
import { login } from "@/features/auth/api";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import {
  googleSignInAvailable,
  useGoogleSignIn,
} from "@/features/auth/google/useGoogleSignIn";
import { destinationAfterSignIn } from "@/features/auth/home";
import { loginSchema, type LoginValues } from "@/features/auth/loginSchema";
import { readJoinContext } from "@/features/join/context";
import { ApiError } from "@/lib/api/errors";
import { authStore } from "@/stores/auth";
import { useAppState } from "@/stores/appState";
import { signInAccount } from "@/features/auth/accountPreferences";
import { TransitionPendingError, transitionStatus } from "@/lib/api/authTransition";
import { accountDepartureNeedsRetry } from "@/features/auth/accountDeparture";
import { checkLogoutCleanup } from "@/features/auth/useSession";

/**
 * LoginPage is §5.1's password sign-in and §5.3's Google entry point. When a
 * visitor arrives from joining a class, it names the class and hands the
 * signed-in user back to `/join/:code` to finish.
 */
export default function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [params] = useSearchParams();
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const [indeterminate, setIndeterminate] = useState(false);
  const google = useGoogleSignIn();
  const [error, setError] = useState<string | null>(null);
  const [joining] = useState(readJoinContext);

  const next = params.get("next") ?? undefined;

  const form = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
  });
  const submitting = form.formState.isSubmitting;
  const clear = { onChange: () => setError(null) };

  const onValid = async (values: LoginValues) => {
    setError(null);
    try {
      const operation = signInAccount(
        "login",
        (ticket) => login(values.email, values.password, ticket),
        queryClient,
        () => mounted.current,
      );
      const failedLease = authStore.captureActor();
      const result = await operation.catch((cause: unknown) => {
        if (!authStore.isCurrent(failedLease)) {
          throw new DOMException("Superseded", "AbortError");
        }
        throw cause;
      });
      if (!mounted.current || !authStore.isCurrent(result.actor)) return;
      useAppState.getState().setBootPhase("ready");
      await navigate(
        joining ? `/join/${joining.code}` : destinationAfterSignIn(next, result.user),
        { replace: true },
      );
    } catch (cause) {
      if (
        !mounted.current ||
        (cause instanceof DOMException && cause.name === "AbortError")
      )
        return;
      setIndeterminate(
        cause instanceof TransitionPendingError ||
          transitionStatus().phase === "cleanup",
      );
      setError(messageFor(cause));
    }
  };
  const onInvalid = (errors: FieldErrors<LoginValues>) => {
    const missing =
      errors.email?.message === "login.errors.emailRequired" || errors.password;
    setError(t(missing ? "login.missingFields" : "login.errors.emailInvalid"));
  };

  function messageFor(cause: unknown) {
    if (cause instanceof TransitionPendingError) {
      if (cause.phase === "cleanup" && accountDepartureNeedsRetry())
        return t("auth.transition.cleanupFailed");
      return t(
        cause.phase === "cleanup"
          ? "auth.transition.cleanupPending"
          : "auth.transition.accountPending",
      );
    }
    if (transitionStatus().phase === "cleanup")
      return t("auth.transition.cleanupFailed");
    if (!(cause instanceof ApiError)) return t("login.failed");
    return cause.code === "INVALID_CREDENTIALS"
      ? t("login.invalidCredentials")
      : cause.message;
  }

  const shown = error ?? google.error;

  return (
    <AuthLayout
      footer={
        <p>
          {t("login.haveCode")}{" "}
          <Link
            to="/join"
            className="text-fg font-medium underline underline-offset-[3px]"
          >
            {t("login.joinClass")}
          </Link>
        </p>
      }
    >
      <div>
        <h1 className="text-h1">{t("login.title")}</h1>
        <p className="text-muted-fg mt-1">
          {joining
            ? t("login.joinSubtitle", { className: joining.className })
            : t("login.subtitle")}
        </p>
      </div>

      {googleSignInAvailable() && (
        <>
          <Button
            type="button"
            variant="outline"
            size="xl"
            className="group/login-busy bg-card shadow-card hover:bg-muted text-body w-full gap-2.5 font-medium"
            aria-busy={google.pending || undefined}
            onClick={() => {
              if (!google.pending) void google.start({ next, joinCode: joining?.code });
            }}
          >
            {google.pending ? (
              <LoaderCircle
                aria-hidden="true"
                className="size-[18px] animate-spin group-focus-within/login-busy:[animation-play-state:paused] group-hover/login-busy:[animation-play-state:paused] motion-reduce:animate-none"
              />
            ) : (
              <GoogleMark className="size-[18px]" />
            )}
            {t("login.continueWithGoogle")}
          </Button>
          <div className="text-muted-fg text-meta flex items-center gap-3">
            <span aria-hidden="true" className="bg-border h-px flex-1" />
            {t("login.orWithEmail")}
            <span aria-hidden="true" className="bg-border h-px flex-1" />
          </div>
        </>
      )}

      {shown && (
        <Alert variant="danger" className="text-ui flex gap-2.5">
          <CircleAlert aria-hidden="true" className="mt-px size-4 shrink-0" />
          <span>{shown}</span>
          {indeterminate && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                const lease = authStore.captureActor();
                const cleanup = checkLogoutCleanup();
                if (
                  mounted.current &&
                  authStore.isCurrent(lease) &&
                  transitionStatus().phase === "cleanup"
                )
                  setError(t("auth.transition.cleanupPending"));
                void cleanup
                  .then(() => {
                    if (!mounted.current || !authStore.isCurrent(lease)) {
                      return;
                    }
                    if (transitionStatus().kind === "idle") {
                      setIndeterminate(false);
                      setError(null);
                    }
                  })
                  .catch((cause: unknown) => {
                    if (!mounted.current || !authStore.isCurrent(lease)) return;
                    setIndeterminate(true);
                    setError(messageFor(cause));
                  });
              }}
            >
              {t(
                accountDepartureNeedsRetry()
                  ? "common.retry"
                  : "auth.transition.checkStatus",
              )}
            </Button>
          )}
        </Alert>
      )}

      <form
        onSubmit={(e) => {
          if (submitting) {
            e.preventDefault();
            return;
          }
          void form.handleSubmit(onValid, onInvalid)(e);
        }}
        className="flex flex-col gap-5"
        noValidate
      >
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="email">{t("login.email")}</Label>
          <Input
            id="email"
            type="email"
            size="xl"
            inputMode="email"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            placeholder={t("login.emailPlaceholder")}
            {...form.register("email", clear)}
          />
        </div>

        <div className="flex flex-col gap-1.5">
          <div className="flex items-baseline justify-between gap-3">
            <Label htmlFor="password">{t("login.password")}</Label>
            <Link
              to="/forgot-password"
              className="text-muted-fg hover:text-fg text-sm no-underline"
            >
              {t("login.forgot")}
            </Link>
          </div>
          <PasswordInput
            id="password"
            size="xl"
            autoComplete="current-password"
            {...form.register("password", clear)}
          />
        </div>

        <Button
          type="submit"
          size="xl"
          className="group/login-busy w-full"
          aria-busy={submitting || undefined}
        >
          {submitting && (
            <LoaderCircle
              aria-hidden="true"
              className="size-[17px] animate-spin group-focus-within/login-busy:[animation-play-state:paused] group-hover/login-busy:[animation-play-state:paused] motion-reduce:animate-none"
            />
          )}
          {t(submitting ? "login.submitting" : "login.submit")}
        </Button>
      </form>
    </AuthLayout>
  );
}
