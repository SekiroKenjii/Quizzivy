import { useEffect, useMemo, useState, type ReactNode } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import { useTranslation } from "react-i18next";
import { DirtyBar } from "@/components/shared/DirtyBar";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { SelectField } from "@/components/shared/form/fields/SelectField";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { toast } from "@/components/ui/sonner";
import {
  saveProfilePatch,
  useAccountPreferenceStatus,
} from "@/features/auth/accountPreferences";
import type { ProfilePatch, User } from "@/features/auth/api";
import {
  profileFormSchema,
  type ProfileFormOutput,
  type ProfileFormValues,
} from "@/features/settings/profileFormSchema";
import { ProfilePhoto } from "@/features/settings/sections/ProfilePhoto";
import { timeZoneOptions } from "@/features/settings/timeZones";
import { failureMessage } from "@/lib/api/errors";
import i18n, { SUPPORTED_LOCALES, chosenLocale, type Locale } from "@/lib/i18n";
import { APP_TIME_ZONE } from "@/lib/i18n/datetime";
import { authStore, useAuthStore } from "@/stores/auth";

const GRID = "grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3.5";
const INPUT = "bg-bg h-9.5";

function savedProfile(user: User): ProfileFormOutput {
  return {
    fullName: user.fullName,
    displayName: user.displayName ?? null,
    phone: user.phone ?? null,
    locale: user.locale ?? chosenLocale(),
    timeZone: user.timeZone ?? APP_TIME_ZONE,
  };
}

function formValues(saved: ProfileFormOutput): ProfileFormValues {
  return {
    ...saved,
    displayName: saved.displayName ?? "",
    phone: saved.phone ?? "",
  };
}

function changes(next: ProfileFormOutput, saved: ProfileFormOutput): ProfilePatch {
  return {
    ...(next.fullName === saved.fullName ? {} : { fullName: next.fullName }),
    ...(next.displayName === saved.displayName
      ? {}
      : { displayName: next.displayName }),
    ...(next.phone === saved.phone ? {} : { phone: next.phone }),
    ...(next.locale === saved.locale ? {} : { locale: next.locale }),
    ...(next.timeZone === saved.timeZone ? {} : { timeZone: next.timeZone }),
  };
}

function Field({
  id,
  label,
  hint,
  error,
  children,
}: Readonly<{
  id: string;
  label: string;
  hint?: string | undefined;
  error?: string | undefined;
  children: ReactNode;
}>) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <Label
        htmlFor={id}
        className="text-meta in-data-[scale=deck]:text-meta leading-normal font-medium"
      >
        {label}
      </Label>
      {children}
      {error === undefined ? null : (
        <p id={`${id}-error`} className="text-danger-ink text-xs leading-normal">
          {error}
        </p>
      )}
      {error === undefined && hint !== undefined ? (
        <p id={`${id}-hint`} className="text-muted-fg text-xs leading-normal">
          {hint}
        </p>
      ) : null}
    </div>
  );
}

function described(id: string, hint: boolean, error: boolean) {
  if (error) return `${id}-error`;
  return hint ? `${id}-hint` : undefined;
}

/**
 * ProfileSection is the Profile card of the teacher's settings: the photo,
 * which saves on its own, and a form of the full name, the name students see,
 * the email the account signs in with (read only), the phone, the language
 * and the time zone. An edit raises the DirtyBar; Save changes sends only the
 * changed fields in one `PATCH /auth/me`, and the language and the zone apply
 * once they are saved (DG-158). Discard restores what was saved.
 */
export function ProfileSection() {
  const user = useAuthStore((s) => s.user);
  if (!user) return null;
  return <ProfileForm key={user.id} user={user} />;
}

function ProfileForm({ user }: Readonly<{ user: User }>) {
  const { t } = useTranslation();
  const saved = useMemo(() => savedProfile(user), [user]);
  const [error, setError] = useState<string | null>(null);
  const unsupportedZone = useAccountPreferenceStatus().unsupportedZone;
  const form = useForm<ProfileFormValues, unknown, ProfileFormOutput>({
    resolver: zodResolver(profileFormSchema),
    defaultValues: formValues(saved),
    mode: "onTouched",
  });
  const { reset, formState } = form;
  const dirty = formState.isDirty;
  const zone = useWatch({ control: form.control, name: "timeZone" });
  const zones = useMemo(() => timeZoneOptions(t, zone, new Date()), [t, zone]);
  const languages = useMemo(
    () =>
      SUPPORTED_LOCALES.map((locale: Locale) => ({
        value: locale,
        label: t(`settings.locale.${locale}`),
      })),
    [t],
  );

  useEffect(() => {
    if (!dirty) reset(formValues(saved));
  }, [dirty, saved, reset]);

  const onSubmit = form.handleSubmit(async (values) => {
    const patch = changes(values, saved);
    setError(null);
    if (Object.keys(patch).length === 0) {
      reset(formValues(saved));
      return;
    }
    const lease = authStore.captureActor();
    try {
      const next = await saveProfilePatch(patch);
      if (!authStore.isCurrent(lease)) return;
      reset(formValues(savedProfile(next)));
      toast(i18n.t("settings.saved"));
    } catch (cause) {
      if (!authStore.isCurrent(lease)) return;
      setError(failureMessage(cause, t("api.failed")));
    }
  });

  const errors = formState.errors;
  const message = (key: string | undefined) => (key === undefined ? undefined : t(key));

  return (
    <form onSubmit={(event) => void onSubmit(event)} noValidate className="contents">
      <SettingsCard
        title={t("settings.profile")}
        description={t("settings.profileHint")}
      >
        <div className="flex flex-col gap-4 p-4.5">
          <ProfilePhoto />
          <div className={GRID}>
            <Field
              id="settings-name"
              label={t("settings.fullName")}
              error={message(errors.fullName?.message)}
            >
              <Input
                id="settings-name"
                className={INPUT}
                autoComplete="name"
                aria-invalid={errors.fullName ? true : undefined}
                aria-describedby={described("settings-name", false, !!errors.fullName)}
                {...form.register("fullName")}
              />
            </Field>
            <Field
              id="settings-display"
              label={t("settings.displayName")}
              hint={t("settings.displayNameHint")}
              error={message(errors.displayName?.message)}
            >
              <Input
                id="settings-display"
                className={INPUT}
                autoComplete="nickname"
                aria-invalid={errors.displayName ? true : undefined}
                aria-describedby={described(
                  "settings-display",
                  true,
                  !!errors.displayName,
                )}
                {...form.register("displayName")}
              />
            </Field>
            <Field
              id="settings-email"
              label={t("settings.email")}
              hint={t("settings.emailHint")}
            >
              <Input
                id="settings-email"
                className={INPUT}
                value={user.email}
                readOnly
                aria-describedby="settings-email-hint"
              />
            </Field>
            <Field
              id="settings-phone"
              label={t("settings.phone")}
              error={message(errors.phone?.message)}
            >
              <Input
                id="settings-phone"
                className={INPUT}
                type="tel"
                autoComplete="tel"
                inputMode="tel"
                aria-invalid={errors.phone ? true : undefined}
                aria-describedby={described("settings-phone", false, !!errors.phone)}
                {...form.register("phone")}
              />
            </Field>
          </div>
          <div className={GRID}>
            <Field id="settings-language" label={t("common.language")}>
              <Controller
                control={form.control}
                name="locale"
                render={({ field }) => (
                  <SelectField
                    id="settings-language"
                    label={t("common.language")}
                    value={field.value}
                    options={languages}
                    onChange={(next) => field.onChange(next)}
                  />
                )}
              />
            </Field>
            <Field
              id="settings-zone"
              label={t("settings.timeZone")}
              hint={
                unsupportedZone === null
                  ? undefined
                  : t("settings.accountZoneUnsupported")
              }
            >
              <Controller
                control={form.control}
                name="timeZone"
                render={({ field }) => (
                  <SelectField
                    id="settings-zone"
                    label={t("settings.timeZone")}
                    value={field.value}
                    options={zones}
                    describedBy={
                      unsupportedZone === null ? undefined : "settings-zone-hint"
                    }
                    onChange={(next) => field.onChange(next)}
                  />
                )}
              />
            </Field>
          </div>
        </div>
      </SettingsCard>
      <DirtyBar
        dirty={dirty}
        saving={formState.isSubmitting}
        error={error}
        onDiscard={() => {
          setError(null);
          reset(formValues(saved));
        }}
      />
    </form>
  );
}
