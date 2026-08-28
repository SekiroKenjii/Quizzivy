import { useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { useNavigate, useSearchParams } from "react-router";
import { AuthLayout } from "@/features/auth/AuthLayout";
import { Button } from "@/components/ui/button";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { login } from "@/features/auth/api";
import { loginSchema, type LoginValues } from "@/features/auth/loginSchema";
import {
  googleSignInAvailable,
  useGoogleSignIn,
} from "@/features/auth/google/useGoogleSignIn";
import { ApiError } from "@/lib/api/errors";
import { destinationAfterSignIn } from "@/features/auth/home";
import { useAuthStore } from "@/stores/auth";

/**
 * §5.1's password sign-in, plus the §5.3 Google entry point.
 *
 * There is deliberately no "create an account" here. Password accounts are
 * created by the teacher, and self-signup is Google-only and requires a class
 * code (§6.3) -- so a signup form on this page would be an invitation to a
 * flow that does not exist. That is the one substantive departure from the
 * shadcn authentication example this layout is modelled on.
 */
export default function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const setSession = useAuthStore((s) => s.setSession);
  const google = useGoogleSignIn();

  const [error, setError] = useState<string | null>(null);

  // Where the guard was sending them before it bounced them here.
  const next = params.get("next") ?? undefined;

  const form = useForm<LoginValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
    // Validate on blur rather than on every keystroke: complaining that an
    // email is invalid while it is still being typed is noise.
    mode: "onTouched",
  });

  const onSubmit = form.handleSubmit(async (values) => {
    setError(null);
    try {
      const result = await login(values.email, values.password);
      setSession(result.accessToken, result.user);
      await navigate(destinationAfterSignIn(next, result.user), { replace: true });
    } catch (cause) {
      // The server's message is already localised and deliberately identical
      // for every failure -- unknown email, wrong password, disabled account
      // (§5.1). Rendering it verbatim is what keeps that property; a
      // client-side lookup keyed on the code would be free to say more than
      // the server chose to.
      setError(cause instanceof ApiError ? cause.message : t("login.failed"));
    }
  });

  return (
    <AuthLayout>
      <h1 className="text-2xl font-semibold tracking-tight">{t("login.title")}</h1>
      <p className="text-muted-foreground mt-2 text-sm">{t("login.subtitle")}</p>

      {/* noValidate hands validation to zod, so the messages are Vietnamese
          and styled rather than the browser's own. */}
      <form onSubmit={(e) => void onSubmit(e)} className="mt-6 space-y-4" noValidate>
        <div className="space-y-2">
          <Label htmlFor="email">{t("login.email")}</Label>
          <Input
            id="email"
            type="email"
            inputMode="email"
            autoComplete="email"
            autoCapitalize="none"
            spellCheck={false}
            aria-invalid={form.formState.errors.email ? true : undefined}
            aria-describedby={form.formState.errors.email ? "email-error" : undefined}
            {...form.register("email")}
          />
          {form.formState.errors.email ? (
            <p id="email-error" className="text-destructive text-sm">
              {t(form.formState.errors.email.message ?? "login.failed")}
            </p>
          ) : null}
        </div>

        <div className="space-y-2">
          <Label htmlFor="password">{t("login.password")}</Label>
          <Input
            id="password"
            type="password"
            autoComplete="current-password"
            aria-invalid={form.formState.errors.password ? true : undefined}
            aria-describedby={
              form.formState.errors.password ? "password-error" : undefined
            }
            {...form.register("password")}
          />
          {form.formState.errors.password ? (
            <p id="password-error" className="text-destructive text-sm">
              {t(form.formState.errors.password.message ?? "login.failed")}
            </p>
          ) : null}
        </div>

        {(error ?? google.error) ? (
          <p role="alert" className="text-destructive text-sm">
            {error ?? google.error}
          </p>
        ) : null}

        <Button type="submit" className="w-full" disabled={form.formState.isSubmitting}>
          {form.formState.isSubmitting ? t("common.loading") : t("login.submit")}
        </Button>
      </form>

      {googleSignInAvailable() ? (
        <>
          <div className="my-6 flex items-center gap-3">
            <span className="bg-border h-px flex-1" aria-hidden="true" />
            <span className="text-muted-foreground text-xs">{t("login.or")}</span>
            <span className="bg-border h-px flex-1" aria-hidden="true" />
          </div>

          <Button
            type="button"
            variant="outline"
            className="w-full"
            disabled={google.pending}
            onClick={() => void google.start({ next })}
          >
            <GoogleMark />
            {t("login.continueWithGoogle")}
          </Button>
        </>
      ) : null}

      <p className="text-muted-foreground mt-6 text-xs leading-relaxed">
        {t("login.noSignup")}
      </p>
    </AuthLayout>
  );
}
