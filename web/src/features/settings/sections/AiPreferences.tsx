import { useState } from "react";
import { useTranslation } from "react-i18next";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Segmented } from "@/components/ui/segmented";
import { toast } from "@/components/ui/sonner";
import { Switch } from "@/components/ui/switch";
import { savePreferences } from "@/features/auth/accountPreferences";
import { aiPreferencesOf, type AiPreferences } from "@/features/auth/preferences";
import { failureMessage } from "@/lib/api/errors";
import { useLocale } from "@/lib/i18n/useLocale";
import { authStore, useAuthStore } from "@/stores/auth";

const LANGUAGES = ["en", "vi"] as const;

const SWITCHES = ["tagDrafts", "readStudentAnswers", "readImports"] as const;

/**
 * AiPreferencesSection is the Preferences card of Settings › AI assistant:
 * the language the assistant writes explanations and feedback in, and the
 * three switches of `UserPreferences.ai`. Each choice applies at once and is
 * saved as the whole `ai` object through `PATCH /me/preferences`, as
 * Appearance saves its choices; it raises no DirtyBar. While a choice is
 * saving the controls stay focusable, answer `aria-disabled` and ignore a
 * press, and a failed save restores the stored value.
 */
export function AiPreferencesSection() {
  const { t } = useTranslation();
  const locale = useLocale();
  const stored = useAuthStore((s) => s.user?.preferences?.ai);
  const [pending, setPending] = useState<AiPreferences | null>(null);
  const saving = pending !== null;
  const shown = pending ?? aiPreferencesOf(stored, locale);

  async function choose(patch: Partial<AiPreferences>) {
    if (saving) return;
    const next = { ...aiPreferencesOf(stored, locale), ...patch };
    const lease = authStore.captureActor();
    setPending(next);
    try {
      await savePreferences({ ai: next });
      if (authStore.isCurrent(lease)) toast(t("settings.preferenceSaved"));
    } catch (cause) {
      if (authStore.isCurrent(lease))
        toast(failureMessage(cause, t("settings.preferenceFailed")));
    } finally {
      if (authStore.isCurrent(lease)) setPending(null);
    }
  }

  return (
    <SettingsCard
      title={t("settings.ai.prefs.title")}
      description={t("settings.ai.prefs.hint")}
    >
      <div className="flex flex-col px-4.5 pt-1 pb-2">
        <div className="flex flex-wrap items-center gap-3 py-3">
          <span className="min-w-0 flex-[1_1_240px]">
            <span id="ai-language" className="text-ui block leading-normal font-medium">
              {t("settings.ai.prefs.language")}
            </span>
            <span className="text-muted-fg text-meta block leading-normal">
              {t("settings.ai.prefs.languageHint")}
            </span>
          </span>
          <Segmented
            label={t("settings.ai.prefs.language")}
            value={shown.language}
            pending={saving}
            options={LANGUAGES.map((language) => ({
              value: language,
              label: t(`settings.ai.prefs.languages.${language}`),
            }))}
            onChange={(value) => {
              if (value === "en" || value === "vi") void choose({ language: value });
            }}
          />
        </div>
        {SWITCHES.map((key) => (
          <div key={key} className="flex items-center gap-3 border-t py-3">
            <span className="min-w-0 flex-1">
              <span
                id={`ai-${key}`}
                className="text-ui block leading-normal font-medium"
              >
                {t(`settings.ai.prefs.${key}`)}
              </span>
              <span
                id={`ai-${key}-hint`}
                className="text-muted-fg text-meta block leading-normal text-pretty"
              >
                {t(`settings.ai.prefs.${key}Hint`)}
              </span>
            </span>
            <Switch
              checked={shown[key]}
              aria-labelledby={`ai-${key}`}
              aria-describedby={`ai-${key}-hint`}
              aria-disabled={saving || undefined}
              onCheckedChange={(on) => void choose({ [key]: on })}
            />
          </div>
        ))}
      </div>
    </SettingsCard>
  );
}
