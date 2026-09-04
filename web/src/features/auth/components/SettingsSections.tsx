import { useState, type FormEvent, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { CircleCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
import { SUPPORTED_LOCALES, setLocale, type Locale } from "@/lib/i18n";
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
    <Card asChild className="gap-0 py-0">
      <section aria-labelledby={labelledBy}>
        <div className="px-5 pt-4 pb-3">
          <h2
            id={labelledBy}
            className="text-[0.9375rem] font-semibold tracking-[-0.01em]"
          >
            {title}
          </h2>
        </div>
        <div className="px-5 pb-4">{children}</div>
      </section>
    </Card>
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
      setStatus({
        kind: "error",
        message: cause instanceof ApiError ? cause.message : t("error.body"),
      });
    } finally {
      setPending(false);
    }
  }

  if (user && !user.hasPassword) {
    return (
      <Section title={t("settings.password")} labelledBy="settings-password">
        <p className="text-muted-foreground text-sm">{t("settings.noPassword")}</p>
      </Section>
    );
  }

  return (
    <Section title={t("settings.password")} labelledBy="settings-password">
      <form onSubmit={onSubmit} className="max-w-sm space-y-3" noValidate>
        <div className="space-y-1.5">
          <Label htmlFor="settings-current">{t("changePassword.current")}</Label>
          <Input
            id="settings-current"
            type="password"
            className="h-11"
            autoComplete="current-password"
            value={current}
            onChange={(e) => setCurrent(e.target.value)}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="settings-new">{t("changePassword.new")}</Label>
          <Input
            id="settings-new"
            type="password"
            className="h-11"
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
      {linked ? (
        <div className="flex items-center gap-2">
          <CircleCheck className="text-success size-4" aria-hidden="true" />
          <p className="text-sm">{t("settings.googleLinked")}</p>
        </div>
      ) : (
        <p className="text-muted-foreground text-sm">{t("settings.googleNotLinked")}</p>
      )}

      {(error ?? google.error) ? (
        <p role="alert" className="text-destructive mt-3 text-sm">
          {error ?? google.error}
        </p>
      ) : null}

      {wouldLockOut ? (
        <p className="text-muted-foreground mt-2 text-xs leading-relaxed">
          {t("settings.googleOnlyExplainer")}
        </p>
      ) : null}

      <div className="mt-3">
        {linked ? (
          <Button
            variant="outline"
            size="sm"
            onClick={() => void unlink()}
            disabled={pending || wouldLockOut}
          >
            {t("settings.unlinkGoogle")}
          </Button>
        ) : googleSignInAvailable() ? (
          <Button
            variant="outline"
            size="sm"
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
      <Tabs
        value={i18n.language}
        onValueChange={(locale) => setLocale(locale as Locale)}
      >
        <TabsList aria-label={t("common.language")}>
          {SUPPORTED_LOCALES.map((locale: Locale) => (
            <TabsTrigger key={locale} value={locale}>
              {t(`settings.locale.${locale}`)}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
    </Section>
  );
}
