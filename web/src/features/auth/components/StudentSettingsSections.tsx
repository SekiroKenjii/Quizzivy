import {
  chooseAccountPreference,
  retryAccountPreference,
  refreshAccount,
  runAccountMutation,
  saveProfilePatch,
  useAccountPreferenceStatus,
} from "@/features/auth/accountPreferences";
import { useEffect, useRef, useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import { KeyRound } from "lucide-react";
import { PasswordInput } from "@/components/shared/PasswordInput";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { FieldError } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { changePassword } from "@/features/auth/api";
import {
  changePasswordSchema,
  type ChangePasswordValues,
} from "@/features/auth/changePasswordSchema";
import { GoogleMark } from "@/features/auth/components/GoogleMark";
import {
  googleSignInAvailable,
  useGoogleSignIn,
} from "@/features/auth/google/useGoogleSignIn";
import { profileSchema, type ProfileValues } from "@/features/auth/profileSchema";
import { api } from "@/lib/api/client";
import { ApiError, failureMessage } from "@/lib/api/errors";
import { SUPPORTED_LOCALES, type Locale } from "@/lib/i18n";
import { passwordRules, passwordStrength } from "@/lib/password";
import { useLargerTestText } from "@/lib/testText";
import { useThemePreference, type ThemePreference } from "@/lib/theme";
import { notify } from "@/lib/toast";
import { cn } from "@/lib/utils";
import { authStore, useAuthStore } from "@/stores/auth";

const CARD = "bg-card shadow-card rounded-xl border";
const FIELD = "flex flex-col gap-1.5";
const LABEL = "in-data-[scale=deck]:text-meta";
const HINT = "text-muted-fg text-xs leading-normal";
const NOTE = "text-meta leading-normal";
const ROW = "flex items-center gap-3 px-4 py-3.5";
const TILE = "bg-muted rounded-ctl grid size-9 flex-none place-items-center";
const ROW_TITLE = "block text-base leading-normal font-medium";
const ROW_SUB = "text-muted-fg text-meta block leading-normal";
const ROW_BUTTON =
  "h-8.5 flex-none rounded-md shadow-none in-data-[scale=deck]:px-3 in-data-[scale=deck]:text-sm";
const BAR_BUTTON = "h-8.5 flex-none rounded-md";

const NAME = "student-settings-name";
const EMAIL = "student-settings-email";
const LANGUAGE = "student-settings-language";
const KEY_ROW = "student-settings-key";
const KEY_TOGGLE = "student-settings-key-toggle";
const CURRENT = "student-settings-current";
const NEW = "student-settings-new";
const GOOGLE = "student-settings-google";
const THEME = "student-settings-theme";
const LARGER = "student-settings-larger";

const BARS = [0, 1, 2, 3] as const;
const STRENGTH = ["okay", "strong", "veryStrong"] as const;
const THEMES: readonly ThemePreference[] = ["light", "dark", "system"];

/**
 * StudentProfileSection is the Profile card of the student's settings, as the
 * design deck draws it: the initials, the name the account may change, the
 * email it may not, and the language. The language applies at once. A changed
 * name brings up the bar that saves or discards it; the saved name is the one
 * the teacher sees.
 */
export function StudentProfileSection() {
  const { t, i18n } = useTranslation();
  const fullName = useAuthStore((s) => s.user?.fullName);
  const email = useAuthStore((s) => s.user?.email);
  const displayName = useAuthStore((s) => s.user?.displayName);
  const preferenceStatus = useAccountPreferenceStatus();
  const mounted = useMounted();
  const [failure, setFailure] = useState<string | null>(null);
  const form = useForm<ProfileValues>({
    resolver: zodResolver(profileSchema),
    defaultValues: { fullName: fullName ?? "" },
    mode: "onTouched",
  });
  const nameError = form.formState.errors.fullName;
  const { isDirty, isSubmitting } = form.formState;

  const { reset } = form;
  useEffect(() => {
    if (!isDirty) reset({ fullName: fullName ?? "" });
  }, [fullName, isDirty, reset]);

  const save = form.handleSubmit(async (values) => {
    const lease = authStore.captureActor();
    const submittedName = form.getValues("fullName");
    setFailure(null);
    try {
      const saved = await saveProfilePatch({ fullName: values.fullName });
      if (!mounted.current || !authStore.isCurrent(lease)) return;
      form.reset(
        { fullName: saved.fullName },
        { keepValues: form.getValues("fullName") !== submittedName },
      );
      notify.success(t("settings.profileSaved"));
    } catch (cause) {
      if (!mounted.current || !authStore.isCurrent(lease)) return;
      setFailure(failureMessage(cause, t("api.failed")));
    }
  });

  const discard = () => {
    setFailure(null);
    form.reset();
  };

  if (fullName === undefined || email === undefined) return null;

  return (
    <form
      noValidate
      className="flex flex-col gap-4"
      onSubmit={(event) => {
        if (!isDirty || isSubmitting) {
          event.preventDefault();
          return;
        }
        void save(event);
      }}
    >
      <section
        aria-label={t("student.settings.sections.profile")}
        className={cn(CARD, "flex flex-col gap-4 p-4.5")}
      >
        <div className="flex items-center gap-3.5">
          <Avatar
            name={displayName ?? fullName}
            size="56"
            tone="self"
            className="text-stat-sm"
          />
        </div>
        <div className={FIELD}>
          <Label htmlFor={NAME} className={LABEL}>
            {t("settings.fullName")}
          </Label>
          <Input
            id={NAME}
            size="lg"
            autoComplete="name"
            readOnly={isSubmitting}
            aria-invalid={nameError ? true : undefined}
            aria-describedby={`${NAME}-note`}
            {...form.register("fullName", { onChange: () => setFailure(null) })}
          />
          {nameError ? (
            <FieldError
              id={`${NAME}-note`}
              className="leading-normal in-data-[scale=deck]:text-xs"
            >
              {t(nameError.message ?? "settings.errors.nameRequired")}
            </FieldError>
          ) : (
            <p id={`${NAME}-note`} className={HINT}>
              {t("student.settings.nameHint")}
            </p>
          )}
        </div>
        <div className={FIELD}>
          <Label htmlFor={EMAIL} className={LABEL}>
            {t("settings.email")}
          </Label>
          <Input
            id={EMAIL}
            size="lg"
            value={email}
            disabled
            readOnly
            aria-describedby={`${EMAIL}-note`}
            className="disabled:bg-muted disabled:text-muted-fg disabled:opacity-100"
          />
          <p id={`${EMAIL}-note`} className={HINT}>
            {t("student.settings.emailHint")}
          </p>
        </div>
        <div className={FIELD}>
          <Label htmlFor={LANGUAGE} className={LABEL}>
            {t("common.language")}
          </Label>
          <Select
            value={i18n.language}
            disabled={preferenceStatus.phase === "saving"}
            onValueChange={(locale) =>
              void chooseAccountPreference({ locale: locale as Locale })
            }
          >
            <SelectTrigger id={LANGUAGE} size="lg" className="w-full pr-[11px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {SUPPORTED_LOCALES.map((locale) => (
                <SelectItem key={locale} value={locale}>
                  {t(`settings.locale.${locale}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <PreferenceNotice />
      </section>
      {isDirty && (
        <div className="bg-card shadow-float sticky bottom-3 flex flex-wrap items-center gap-2.5 rounded-xl border py-2.5 pr-3 pl-4">
          {failure === null ? (
            <p className="text-muted-fg min-w-40 flex-1 text-sm">
              {t("student.settings.unsaved")}
            </p>
          ) : (
            <p role="alert" className="text-danger-ink min-w-40 flex-1 text-sm">
              {failure}
            </p>
          )}
          <div className="ml-auto flex items-center gap-2.5">
            <Button
              type="button"
              variant="ghost"
              disabled={isSubmitting}
              className={cn(
                BAR_BUTTON,
                "hover:bg-muted dark:hover:bg-muted in-data-[scale=deck]:px-3 in-data-[scale=deck]:text-sm",
              )}
              onClick={discard}
            >
              {t("student.settings.discard")}
            </Button>
            <Button type="submit" disabled={isSubmitting} className={BAR_BUTTON}>
              {isSubmitting ? t("common.saving") : t("common.saveChanges")}
            </Button>
          </div>
        </div>
      )}
    </form>
  );
}

/**
 * StudentSignInSection is the Sign-in card of the student's settings, as the
 * design deck draws it: a password row that opens the change form, and the
 * Google row. An account with no password has no password row, and its Unlink
 * stays in the tab order, switched off, beside the reason: Google is then its
 * only way in. The strength meter only advises; the form's rules and the
 * server decide.
 */
export function StudentSignInSection() {
  const { t } = useTranslation();
  const signedIn = useAuthStore((s) => s.user !== null);
  const hasPassword = useAuthStore((s) => s.user?.hasPassword ?? false);
  if (!signedIn) return null;
  return (
    <section
      aria-label={t("student.settings.sections.signIn")}
      className={cn(CARD, "overflow-hidden")}
    >
      {hasPassword && <PasswordRow />}
      <GoogleRow />
    </section>
  );
}

function meterScore(password: string): number {
  const rules = passwordRules(password);
  const score = passwordStrength(password);
  return rules.length && rules.numberOrSymbol ? score : Math.min(score, 2);
}

function meterFill(score: number): string {
  if (score <= 1) return "bg-danger";
  return score === 2 ? "bg-warning" : "bg-success";
}

function meterHint(password: string, t: TFunction): string {
  const rules = passwordRules(password);
  if (!rules.length) return t("student.settings.passwordMin");
  const strength = t(`student.settings.strength.${STRENGTH[meterScore(password) - 2]}`);
  return rules.numberOrSymbol
    ? t("student.settings.strengthOnly", { strength })
    : t("student.settings.strengthAdvice", { strength });
}

function PasswordRow() {
  const { t } = useTranslation();
  const mounted = useMounted();
  const [open, setOpen] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const form = useForm<ChangePasswordValues>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: { currentPassword: "", newPassword: "" },
  });
  const typed = useWatch({ control: form.control, name: "newPassword" });
  const score = meterScore(typed);
  const newError = form.formState.errors.newPassword;
  const blocked = !passwordRules(typed).length || form.formState.isSubmitting;

  const toggle = () => {
    form.reset();
    setFailure(null);
    setOpen(!open);
  };

  const update = form.handleSubmit(async (values) => {
    const lease = authStore.captureActor();
    setFailure(null);
    try {
      await runAccountMutation(
        () => changePassword(values.currentPassword, values.newPassword),
        true,
      );
      if (!mounted.current || !authStore.isCurrent(lease)) return;
      form.reset();
      setOpen(false);
      document.getElementById(KEY_TOGGLE)?.focus();
      notify.success(t("student.settings.passwordUpdated"));
    } catch (cause) {
      if (!mounted.current || !authStore.isCurrent(lease)) return;
      setFailure(
        cause instanceof ApiError && cause.code === "PASSWORD_UNCHANGED"
          ? t("changePassword.errors.unchanged")
          : failureMessage(cause, t("api.failed")),
      );
    }
  });

  return (
    <>
      <div className={ROW}>
        <span className={TILE}>
          <KeyRound aria-hidden="true" className="size-[17px]" />
        </span>
        <span className="min-w-0 flex-1">
          <span id={KEY_ROW} className={ROW_TITLE}>
            {t("settings.password")}
          </span>
        </span>
        <Button
          id={KEY_TOGGLE}
          type="button"
          variant="outline"
          aria-expanded={open}
          aria-describedby={KEY_ROW}
          className={ROW_BUTTON}
          onClick={toggle}
        >
          {open ? t("common.cancel") : t("student.settings.change")}
        </Button>
      </div>
      {open && (
        <form
          noValidate
          className="flex flex-col gap-3 px-4 pt-1 pb-4"
          onSubmit={(event) => {
            if (form.formState.isSubmitting) {
              event.preventDefault();
              return;
            }
            void update(event);
          }}
        >
          <div className={FIELD}>
            <Label htmlFor={CURRENT} className={LABEL}>
              {t("changePassword.current")}
            </Label>
            <PasswordInput
              id={CURRENT}
              size="lg"
              autoComplete="current-password"
              {...form.register("currentPassword")}
            />
          </div>
          <div className={FIELD}>
            <Label htmlFor={NEW} className={LABEL}>
              {t("changePassword.new")}
            </Label>
            <PasswordInput
              id={NEW}
              size="lg"
              autoComplete="new-password"
              aria-invalid={newError ? true : undefined}
              aria-describedby={`${NEW}-note`}
              {...form.register("newPassword")}
            />
          </div>
          <div aria-hidden="true" data-testid="password-meter" className="flex gap-1">
            {BARS.map((bar) => (
              <span
                key={bar}
                data-filled={bar < score || undefined}
                className={cn(
                  "h-1 flex-1 rounded-[4px]",
                  bar < score ? meterFill(score) : "bg-muted",
                )}
              />
            ))}
          </div>
          {newError ? (
            <p id={`${NEW}-note`} role="alert" className={cn(NOTE, "text-danger-ink")}>
              {t(newError.message ?? "changePassword.errors.tooShort")}
            </p>
          ) : (
            <p id={`${NEW}-note`} className={cn(NOTE, "text-muted-fg")}>
              {meterHint(typed, t)}
            </p>
          )}
          {failure !== null && (
            <p role="alert" className={cn(NOTE, "text-danger-ink")}>
              {failure}
            </p>
          )}
          <Button
            type="submit"
            size="md"
            aria-disabled={blocked}
            className="self-start px-4 aria-disabled:opacity-50"
            onClick={(event) => {
              if (blocked) event.preventDefault();
            }}
          >
            {form.formState.isSubmitting
              ? t("common.saving")
              : t("student.settings.updatePassword")}
          </Button>
        </form>
      )}
    </>
  );
}

function GoogleRow() {
  const { t } = useTranslation();
  const linked = useAuthStore(
    (s) => s.user?.linkedProviders.includes("google") ?? false,
  );
  const hasPassword = useAuthStore((s) => s.user?.hasPassword ?? false);
  const mounted = useMounted();
  const google = useGoogleSignIn();
  const [failure, setFailure] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const available = googleSignInAvailable();
  const problem = failure ?? google.error;
  const locked = pending || !hasPassword;
  const sentence = !linked || !hasPassword;

  async function unlink() {
    setFailure(null);
    setPending(true);
    const lease = authStore.captureActor();
    try {
      await runAccountMutation(() => api("delete", "/auth/google/link"));
      if (!mounted.current || !authStore.isCurrent(lease)) return;
      notify.success(t("student.settings.googleUnlinked"));
    } catch (cause) {
      if (!mounted.current || !authStore.isCurrent(lease)) return;
      setFailure(failureMessage(cause, t("api.failed")));
    } finally {
      if (mounted.current && authStore.isCurrent(lease)) setPending(false);
    }
  }

  return (
    <div className="border-t first:border-t-0">
      <div className={cn(ROW, sentence && "flex-wrap")}>
        <span className={TILE}>
          <GoogleMark className="size-4.5" />
        </span>
        <span className={cn("min-w-0 flex-1", sentence && "basis-44")}>
          <span id={GOOGLE} className={ROW_TITLE}>
            {t("student.settings.google")}
          </span>
          <span id={`${GOOGLE}-status`} className={cn(ROW_SUB, "break-words")}>
            {googleStatus({ linked, hasPassword, available }, t)}
          </span>
        </span>
        {linked && (
          <Button
            type="button"
            variant="outline"
            aria-disabled={locked}
            aria-describedby={`${GOOGLE} ${GOOGLE}-status`}
            className={cn(
              ROW_BUTTON,
              "aria-disabled:opacity-50",
              sentence && "ml-auto",
            )}
            onClick={() => {
              if (!locked) void unlink();
            }}
          >
            {t("student.settings.unlink")}
          </Button>
        )}
        {!linked && available && (
          <Button
            type="button"
            variant="outline"
            disabled={google.pending}
            className={cn(ROW_BUTTON, "ml-auto")}
            onClick={() =>
              void google.start({ mode: "link", next: window.location.pathname })
            }
          >
            {t("settings.linkGoogle")}
          </Button>
        )}
      </div>
      {problem !== null && (
        <p role="alert" className={cn(NOTE, "text-danger-ink px-4 pb-3.5")}>
          {problem}
        </p>
      )}
    </div>
  );
}

function googleStatus(
  account: Readonly<{ linked: boolean; hasPassword: boolean; available: boolean }>,
  t: TFunction,
): string {
  if (account.linked)
    return account.hasPassword
      ? t("student.settings.googleLinked")
      : t("settings.googleOnlyExplainer");
  return account.available
    ? t("student.settings.googleNotLinked")
    : t("login.googleUnavailable");
}

/** StudentAppearanceSection previews and saves the account theme and larger-test-text choices. */
export function StudentAppearanceSection() {
  const { t } = useTranslation();
  const preference = useThemePreference();
  const larger = useLargerTestText();
  const preferenceStatus = useAccountPreferenceStatus();
  return (
    <section
      aria-label={t("student.settings.sections.appearance")}
      className={cn(CARD, "flex flex-col gap-3.5 p-4")}
    >
      <span id={THEME} className={ROW_TITLE}>
        {t("student.settings.theme")}
      </span>
      <div role="group" aria-labelledby={THEME} className="grid grid-cols-3 gap-2.5">
        {THEMES.map((theme) => (
          <button
            key={theme}
            type="button"
            aria-pressed={preference === theme}
            className={cn(
              "bg-card flex flex-col gap-2 rounded-lg border p-2 text-left",
              preference === theme && "border-primary ring-primary ring-1",
            )}
            disabled={preferenceStatus.phase === "saving"}
            onClick={() => void chooseAccountPreference({ theme })}
          >
            <span
              data-swatch={theme}
              className="rounded-seg flex h-14 overflow-hidden border"
            >
              {theme !== "dark" && <span className="bg-swatch-light flex-1" />}
              {theme !== "light" && <span className="bg-swatch-dark flex-1" />}
            </span>
            <span className="text-sm leading-[1.265] font-medium break-words">
              {t(`student.settings.themes.${theme}`)}
            </span>
          </button>
        ))}
      </div>
      <div className="flex items-center gap-3.5 border-t pt-1.5">
        <span className="min-w-0 flex-1">
          <span id={LARGER} className={ROW_TITLE}>
            {t("student.settings.largerText")}
          </span>
          <span id={`${LARGER}-hint`} className={ROW_SUB}>
            {t("student.settings.largerTextHint")}
          </span>
        </span>
        <Switch
          size="lg"
          checked={larger}
          aria-labelledby={LARGER}
          aria-describedby={`${LARGER}-hint`}
          disabled={preferenceStatus.phase === "saving"}
          onCheckedChange={(largerTestText) =>
            void chooseAccountPreference({ largerTestText })
          }
        />
      </div>
      <PreferenceNotice />
    </section>
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

function PreferenceNotice() {
  const { t } = useTranslation();
  const status = useAccountPreferenceStatus();
  const message = {
    idle: "settings.preferenceSaved",
    saving: "settings.preferenceSaving",
    saved: "settings.preferenceSaved",
    failed: "settings.preferenceFailed",
  }[status.phase];
  return (
    <>
      {status.phase !== "idle" && (
        <p
          role={status.phase === "failed" ? "alert" : "status"}
          className="text-muted-fg mt-3 text-sm"
        >
          {t(message)}
          {status.phase === "failed" && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => void retryAccountPreference()}
            >
              {t("common.retry")}
            </Button>
          )}
        </p>
      )}
      {status.unsupportedZone && (
        <p role="alert" className="text-muted-fg mt-3 text-sm">
          {t("settings.accountZoneUnsupported")}
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => void refreshAccount().catch(() => undefined)}
          >
            {t("auth.transition.checkStatus")}
          </Button>
        </p>
      )}
    </>
  );
}
