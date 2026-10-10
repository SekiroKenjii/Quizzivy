import { runAccountMutation } from "@/features/auth/accountPreferences";
import { PasswordInput } from "@/components/shared/PasswordInput";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { useEffect, useRef, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { CircleCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/sonner";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import { Label } from "@/components/ui/label";
import { changePassword, openDocsSession } from "@/features/auth/api";
import {
  changePasswordSchema,
  type ChangePasswordValues,
} from "@/features/auth/changePasswordSchema";
import {
  googleSignInAvailable,
  useGoogleSignIn,
} from "@/features/auth/google/useGoogleSignIn";
import { api, BASE_URL } from "@/lib/api/client";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { authStore, useAuthStore } from "@/stores/auth";

const BODY = "flex flex-col gap-3.5 p-4.5";
const GRID = "grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3.5";
const FIELD = "flex min-w-0 flex-col gap-1.5";
const LABEL = "text-meta in-data-[scale=deck]:text-meta leading-normal font-medium";
const INPUT = "bg-bg h-9.5";
const ACTION = "h-8.5 self-start";

/**
 * PasswordSection is the Password card of the teacher's Sign-in & security
 * settings: the current and the new password, changed by its own button
 * (DG-157). A Google-only account has no password to change and gets no card.
 */
export function PasswordSection() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const mounted = useMounted();

  const [error, setError] = useState<string | null>(null);
  const form = useForm<ChangePasswordValues>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: { currentPassword: "", newPassword: "" },
    mode: "onTouched",
  });
  const newPasswordError = form.formState.errors.newPassword;

  const onSubmit = form.handleSubmit(async (values) => {
    const lease = authStore.captureActor();
    setError(null);
    try {
      await runAccountMutation(() =>
        changePassword(values.currentPassword, values.newPassword),
      );
      if (!mounted.current || !authStore.isCurrent(lease)) return;
      form.reset();
      toast(t("settings.passwordChanged"));
    } catch (cause) {
      if (!mounted.current || !authStore.isCurrent(lease)) return;
      setError(
        cause instanceof ApiError && cause.code === "PASSWORD_UNCHANGED"
          ? t("changePassword.errors.unchanged")
          : failureMessage(cause, t("api.failed")),
      );
    }
  });

  if (user && !user.hasPassword) return null;

  return (
    <SettingsCard title={t("settings.password")}>
      <form onSubmit={(e) => void onSubmit(e)} className={BODY} noValidate>
        <div className={GRID}>
          <div className={FIELD}>
            <Label htmlFor="settings-current" className={LABEL}>
              {t("changePassword.current")}
            </Label>
            <PasswordInput
              id="settings-current"
              className={INPUT}
              autoComplete="current-password"
              {...form.register("currentPassword")}
            />
          </div>
          <div className={FIELD}>
            <Label htmlFor="settings-new" className={LABEL}>
              {t("changePassword.new")}
            </Label>
            <PasswordInput
              id="settings-new"
              className={INPUT}
              autoComplete="new-password"
              aria-invalid={newPasswordError ? true : undefined}
              aria-describedby={
                newPasswordError ? "settings-new-error" : "settings-new-hint"
              }
              {...form.register("newPassword")}
            />
            {newPasswordError ? (
              <p id="settings-new-error" className="text-danger-ink text-xs">
                {t(newPasswordError.message ?? "changePassword.errors.tooShort")}
              </p>
            ) : (
              <p id="settings-new-hint" className="text-muted-fg text-xs">
                {t("changePassword.hint")}
              </p>
            )}
          </div>
        </div>
        {error === null ? null : (
          <p role="alert" className="text-danger-ink text-sm">
            {error}
          </p>
        )}
        <Button
          type="submit"
          variant="outline"
          className={ACTION}
          disabled={form.formState.isSubmitting}
        >
          {form.formState.isSubmitting
            ? t("common.saving")
            : t("changePassword.submit")}
        </Button>
      </form>
    </SettingsCard>
  );
}

/**
 * GoogleSection is the Google account card the deck does not draw (DG-157):
 * whether Google is linked, what unlinking would leave, and the control that
 * links or unlinks it.
 */
export function GoogleSection() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const mounted = useMounted();
  const google = useGoogleSignIn();
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const linked = user?.linkedProviders.includes("google") ?? false;
  const wouldLockOut = linked && !(user?.hasPassword ?? false);

  async function unlink() {
    setError(null);
    setPending(true);
    const lease = authStore.captureActor();
    try {
      await runAccountMutation(() => api("delete", "/auth/google/link"));
      if (!mounted.current || !authStore.isCurrent(lease)) return;
    } catch (cause) {
      if (!mounted.current || !authStore.isCurrent(lease)) return;
      setError(cause instanceof ApiError ? cause.message : t("api.failed"));
    } finally {
      if (mounted.current && authStore.isCurrent(lease)) setPending(false);
    }
  }

  if (!user) return null;

  return (
    <SettingsCard title={t("settings.google")}>
      <div className={BODY}>
        {linked ? (
          <p className="text-ui flex items-center gap-2">
            <CircleCheck className="text-success-ink size-4" aria-hidden="true" />
            {t("settings.googleLinked")}
          </p>
        ) : (
          <p className="text-muted-fg text-ui">{t("settings.googleNotLinked")}</p>
        )}
        {linked ? (
          <p
            id="settings-google-explainer"
            className="text-muted-fg text-meta leading-normal"
          >
            {t(
              wouldLockOut
                ? "settings.googleOnlyExplainer"
                : "settings.googleBothExplainer",
            )}
          </p>
        ) : null}
        {(error ?? google.error) ? (
          <p role="alert" className="text-danger-ink text-sm">
            {error ?? google.error}
          </p>
        ) : null}
        {linked ? (
          <Button
            variant="outline"
            aria-disabled={pending || wouldLockOut}
            aria-describedby="settings-google-explainer"
            className={wouldLockOut ? `${ACTION} opacity-50` : ACTION}
            onClick={() => {
              if (pending || wouldLockOut) return;
              void unlink();
            }}
          >
            {t("settings.unlinkGoogle")}
          </Button>
        ) : (
          <LinkGoogleControl
            pending={google.pending}
            onStart={() =>
              void google.start({ mode: "link", next: window.location.pathname })
            }
          />
        )}
      </div>
    </SettingsCard>
  );
}

function LinkGoogleControl({
  pending,
  onStart,
}: Readonly<{ pending: boolean; onStart: () => void }>) {
  const { t } = useTranslation();
  if (!googleSignInAvailable()) {
    return <p className="text-muted-fg text-ui">{t("login.googleUnavailable")}</p>;
  }
  return (
    <Button variant="outline" className={ACTION} disabled={pending} onClick={onStart}>
      <GoogleMark />
      {t("settings.linkGoogle")}
    </Button>
  );
}

/**
 * ApiDocsSection opens the API reference in a new tab: the tab is opened with its
 * opener cleared, then pointed at /docs once the docs session exists, and closed
 * if it cannot be opened.
 */
export function ApiDocsSection() {
  const { t } = useTranslation();
  const [problem, setProblem] = useState<"blocked" | "failed" | null>(null);
  const [pending, setPending] = useState(false);
  const mounted = useMounted();
  const open = () => {
    const lease = authStore.captureActor();
    const tab = window.open("about:blank", "_blank");
    if (!tab) {
      setProblem("blocked");
      return;
    }
    tab.opener = null;
    setProblem(null);
    setPending(true);
    openDocsSession()
      .then(
        () => {
          if (!mounted.current || !authStore.isCurrent(lease)) {
            tab.close();
            return;
          }
          tab.location.href = `${BASE_URL}/docs`;
        },
        () => {
          tab.close();
          if (mounted.current && authStore.isCurrent(lease)) setProblem("failed");
        },
      )
      .finally(() => {
        if (mounted.current && authStore.isCurrent(lease)) setPending(false);
      });
  };
  return (
    <SettingsCard title={t("settings.apiDocs.title")}>
      <div className={BODY}>
        <p className="text-muted-fg text-ui leading-relaxed">
          {t("settings.apiDocs.body")}
        </p>
        <Button
          type="button"
          variant="outline"
          className={ACTION}
          disabled={pending}
          onClick={open}
        >
          {t("settings.apiDocs.open")}
        </Button>
        {problem ? (
          <p role="alert" className="text-sm">
            {t(`settings.apiDocs.${problem}`)}
          </p>
        ) : null}
      </div>
    </SettingsCard>
  );
}

function useMounted() {
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  return mounted;
}
