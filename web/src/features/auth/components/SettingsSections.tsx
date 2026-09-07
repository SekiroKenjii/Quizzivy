import { useState, type ReactNode } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { CircleCheck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Segmented } from "@/components/ui/segmented";
import { toast } from "@/components/ui/sonner";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { changePassword, fetchCurrentUser, updateProfile } from "@/features/auth/api";
import {
  changePasswordSchema,
  type ChangePasswordValues,
} from "@/features/auth/changePasswordSchema";
import { profileSchema, type ProfileValues } from "@/features/auth/profileSchema";
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
}: Readonly<{
  title: string;
  labelledBy: string;
  children: ReactNode;
}>) {
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

/**
 * The "Hồ sơ" card both settings boards draw (S-10, S-17): a name the account
 * owns and may change, and the email it signs in with, which it may not. The
 * hint says who the address came from, because that is the answer to "why can
 * I not edit this?" -- Google for a linked account, the teacher otherwise.
 */
export function ProfileSection() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);
  const [error, setError] = useState<string | null>(null);

  const form = useForm<ProfileValues>({
    resolver: zodResolver(profileSchema),
    values: { fullName: user?.fullName ?? "" },
    mode: "onTouched",
  });
  const nameError = form.formState.errors.fullName;

  const onSubmit = form.handleSubmit(async (values) => {
    setError(null);
    try {
      setUser(await updateProfile(values.fullName));
      // F-08: a completed action is confirmed by a toast, not by a sentence
      // that stays on the screen and pushes the button under the pointer.
      toast(t("settings.profileSaved"));
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("error.body"));
    }
  });

  if (!user) return null;
  const fromGoogle = user.linkedProviders.includes("google");

  return (
    <Section title={t("settings.profile")} labelledBy="settings-profile">
      <form
        onSubmit={(e) => void onSubmit(e)}
        className="max-w-md space-y-3"
        noValidate
      >
        <div className="space-y-1.5">
          <Label htmlFor="settings-name">{t("settings.fullName")}</Label>
          <Input
            id="settings-name"
            className="h-11"
            autoComplete="name"
            aria-invalid={nameError ? true : undefined}
            aria-describedby={nameError ? "settings-name-error" : undefined}
            {...form.register("fullName")}
          />
          {nameError ? (
            <p id="settings-name-error" className="text-destructive text-xs">
              {t(nameError.message ?? "settings.errors.nameRequired")}
            </p>
          ) : null}
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="settings-email">{t("settings.email")}</Label>
          <Input
            id="settings-email"
            className="h-11"
            value={user.email}
            disabled
            readOnly
            aria-describedby="settings-email-hint"
          />
          <p id="settings-email-hint" className="text-muted-foreground text-xs">
            {t(fromGoogle ? "settings.emailFromGoogle" : "settings.emailFromTeacher")}
          </p>
        </div>
        {error !== null ? (
          // S-02's rule, which holds anywhere a form can fail: an error is a
          // line under the button, never a toast that leaves before it is read.
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        ) : null}
        <Button
          type="submit"
          size="sm"
          disabled={form.formState.isSubmitting || !form.formState.isDirty}
        >
          {form.formState.isSubmitting ? t("common.loading") : t("common.saveChanges")}
        </Button>
      </form>
    </Section>
  );
}

export function PasswordSection() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const setUser = useAuthStore((s) => s.setUser);

  const [error, setError] = useState<string | null>(null);
  const form = useForm<ChangePasswordValues>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: { currentPassword: "", newPassword: "" },
    mode: "onTouched",
  });
  const newPasswordError = form.formState.errors.newPassword;

  const onSubmit = form.handleSubmit(async (values) => {
    setError(null);
    try {
      await changePassword(values.currentPassword, values.newPassword);
      setUser(await fetchCurrentUser());
      form.reset();
      toast(t("settings.passwordChanged"));
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : t("error.body"));
    }
  });

  // S-10 draws no password card for a Google-only account: there is nothing to
  // change and no way to set one, and the Google card already says so. A card
  // whose whole body is "you have no password" is a hole in the grid (S-17).
  if (user && !user.hasPassword) return null;

  return (
    <Section title={t("settings.password")} labelledBy="settings-password">
      <form
        onSubmit={(e) => void onSubmit(e)}
        className="max-w-md space-y-3"
        noValidate
      >
        <div className="space-y-1.5">
          <Label htmlFor="settings-current">{t("changePassword.current")}</Label>
          <Input
            id="settings-current"
            type="password"
            className="h-11"
            autoComplete="current-password"
            {...form.register("currentPassword")}
          />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="settings-new">{t("changePassword.new")}</Label>
          <Input
            id="settings-new"
            type="password"
            className="h-11"
            autoComplete="new-password"
            aria-invalid={newPasswordError ? true : undefined}
            aria-describedby={
              newPasswordError ? "settings-new-error" : "settings-new-hint"
            }
            {...form.register("newPassword")}
          />
          {newPasswordError ? (
            // F-06 swaps the hint's colour, not its size: the card must not
            // grow the moment a field goes invalid.
            <p id="settings-new-error" className="text-destructive text-xs">
              {t(newPasswordError.message ?? "changePassword.errors.tooShort")}
            </p>
          ) : (
            <p id="settings-new-hint" className="text-muted-foreground text-xs">
              {t("changePassword.hint")}
            </p>
          )}
        </div>
        {error !== null ? (
          // S-02's rule, which holds anywhere a form can fail: an error is a
          // line under the button, never a toast that leaves before it is read.
          <p role="alert" className="text-destructive text-sm">
            {error}
          </p>
        ) : null}
        <Button type="submit" size="sm" disabled={form.formState.isSubmitting}>
          {form.formState.isSubmitting
            ? t("common.loading")
            : t("changePassword.submit")}
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

      {linked ? (
        <p
          id="settings-google-explainer"
          className="text-muted-foreground mt-2 text-xs leading-relaxed"
        >
          {t(
            wouldLockOut
              ? "settings.googleOnlyExplainer"
              : "settings.googleBothExplainer",
          )}
        </p>
      ) : null}

      <div className="mt-3">
        {linked ? (
          <Button
            variant="outline"
            size="sm"
            aria-disabled={pending || wouldLockOut}
            aria-describedby="settings-google-explainer"
            className={wouldLockOut ? "opacity-50" : undefined}
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
    </Section>
  );
}

export function LanguageSection() {
  const { t, i18n } = useTranslation();

  return (
    <Section title={t("common.language")} labelledBy="settings-language">
      <Segmented
        label={t("common.language")}
        value={i18n.language}
        options={SUPPORTED_LOCALES.map((locale: Locale) => ({
          value: locale,
          label: t(`settings.locale.${locale}`),
        }))}
        onChange={(locale) => setLocale(locale as Locale)}
      />
      {/* S-17 writes this under the switch; S-10's phone card is the tabs and
          nothing else, so the sentence arrives with the room for it. */}
      <p className="text-muted-foreground mt-3 hidden text-xs leading-relaxed lg:block">
        {t("settings.languageExplainer")}
      </p>
    </Section>
  );
}

/** The link button, or the reason there is none: GIS is a public config value that can be absent. */
function LinkGoogleControl({
  pending,
  onStart,
}: Readonly<{ pending: boolean; onStart: () => void }>) {
  const { t } = useTranslation();
  if (!googleSignInAvailable()) {
    return (
      <p className="text-muted-foreground text-sm">{t("login.googleUnavailable")}</p>
    );
  }
  return (
    <Button variant="outline" size="sm" disabled={pending} onClick={onStart}>
      <GoogleMark />
      {t("settings.linkGoogle")}
    </Button>
  );
}
