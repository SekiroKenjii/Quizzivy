import { useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { changePassword, fetchCurrentUser } from "@/features/auth/api";
import {
  googleSignInAvailable,
  useGoogleSignIn,
} from "@/features/auth/google/useGoogleSignIn";
import { api } from "@/lib/api/client";
import { ApiError } from "@/lib/api/errors";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";

function Section({
  title,
  labelledBy,
  children,
}: {
  title: string;
  labelledBy: string;
  children: ReactNode;
}) {
  return (
    <section className="rounded-lg border p-6" aria-labelledby={labelledBy}>
      <h2 id={labelledBy} className="text-base font-semibold">
        {title}
      </h2>
      <div className="mt-4">{children}</div>
    </section>
  );
}

/** §8's profile block. Read-only: there is no endpoint to change it yet. */
export function ProfileSection() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  if (!user) return null;

  return (
    <Section title={t("settings.profile")} labelledBy="settings-profile">
      <dl className="grid grid-cols-[8rem_1fr] gap-y-2 text-sm">
        <dt className="text-muted-foreground">{t("settings.fullName")}</dt>
        <dd>{user.fullName}</dd>
        <dt className="text-muted-foreground">{t("settings.email")}</dt>
        <dd>{user.email}</dd>
      </dl>
    </Section>
  );
}

export function PasswordSection() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);

  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [status, setStatus] = useState<{
    kind: "ok" | "error";
    message: string;
  } | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setStatus(null);
    setPending(true);
    try {
      await changePassword(current, next);
      setUser(await fetchCurrentUser());
      setCurrent("");
      setNext("");
      setStatus({ kind: "ok", message: t("settings.passwordChanged") });
    } catch (cause) {
      // A wrong current password is a 400, not a 401 -- see the server handler.
      // A 401 would make the client refresh, retry, and sign the user out over
      // a typo.
      setStatus({
        kind: "error",
        message: cause instanceof ApiError ? cause.message : t("error.body"),
      });
    } finally {
      setPending(false);
    }
  }

  if (user && !user.hasPassword) {
    // §6.3: a self-join account signs in with Google and has no password to
    // change. Offering the form would be offering a flow that cannot succeed.
    return (
      <Section title={t("settings.password")} labelledBy="settings-password">
        <p className="text-muted-foreground text-sm">{t("settings.noPassword")}</p>
      </Section>
    );
  }

  return (
    <Section title={t("settings.password")} labelledBy="settings-password">
      <form onSubmit={onSubmit} className="max-w-sm space-y-4" noValidate>
        <div className="space-y-2">
          <Label htmlFor="settings-current">{t("changePassword.current")}</Label>
          <Input
            id="settings-current"
            type="password"
            autoComplete="current-password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="settings-new">{t("changePassword.new")}</Label>
          <Input
            id="settings-new"
            type="password"
            autoComplete="new-password"
            minLength={8}
            value={next}
            onChange={(e) => setNext(e.target.value)}
          />
        </div>
        {status ? (
          <p
            role={status.kind === "error" ? "alert" : "status"}
            className={status.kind === "error" ? "text-destructive text-sm" : "text-sm"}
          >
            {status.message}
          </p>
        ) : null}
        <Button type="submit" disabled={pending}>
          {pending ? t("common.loading") : t("changePassword.submit")}
        </Button>
      </form>
    </Section>
  );
}

export function GoogleSection() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);
  const google = useGoogleSignIn();
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  const linked = user?.linkedProviders.includes("google") ?? false;
  // §15: unlinking would leave no way in at all. The account would still
  // exist, still hold its work, and nobody -- including its owner -- could sign
  // into it. The server refuses this too; the control explains it beforehand
  // rather than letting them press it and read a 409.
  const wouldLockOut = linked && !(user?.hasPassword ?? false);

  async function unlink() {
    setError(null);
    setPending(true);
    try {
      await api("delete", "/auth/google/link");
      setUser(await fetchCurrentUser());
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("error.body"));
    } finally {
      setPending(false);
    }
  }

  if (!user) return null;

  return (
    <Section title={t("settings.google")} labelledBy="settings-google">
      <p className="text-muted-foreground text-sm">
        {linked ? t("settings.googleLinked") : t("settings.googleNotLinked")}
      </p>

      {(error ?? google.error) ? (
        <p role="alert" className="text-destructive mt-3 text-sm">
          {error ?? google.error}
        </p>
      ) : null}

      {wouldLockOut ? (
        <p className="text-muted-foreground mt-3 text-sm">
          {t("settings.googleOnlyExplainer")}
        </p>
      ) : null}

      <div className="mt-4">
        {linked ? (
          <Button
            variant="outline"
            onClick={() => void unlink()}
            disabled={pending || wouldLockOut}
          >
            {t("settings.unlinkGoogle")}
          </Button>
        ) : googleSignInAvailable() ? (
          <Button
            variant="outline"
            disabled={google.pending}
            onClick={() =>
              void google.start({ mode: "link", next: window.location.pathname })
            }
          >
            <GoogleMark />
            {t("settings.linkGoogle")}
          </Button>
        ) : (
          <p className="text-muted-foreground text-sm">
            {t("login.googleUnavailable")}
          </p>
        )}
      </div>
    </Section>
  );
}

export function LanguageSection() {
  const { t, i18n } = useTranslation();

  return (
    <Section title={t("common.language")} labelledBy="settings-language">
      <div className="flex gap-2">
        {SUPPORTED_LOCALES.map((locale: Locale) => (
          <Button
            key={locale}
            variant={i18n.language === locale ? "default" : "outline"}
            size="sm"
            aria-pressed={i18n.language === locale}
            onClick={() => void i18n.changeLanguage(locale)}
          >
            {t(`settings.locale.${locale}`)}
          </Button>
        ))}
      </div>
    </Section>
  );
}
