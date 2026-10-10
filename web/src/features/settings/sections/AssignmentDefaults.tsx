import { useState } from "react";
import { useTranslation } from "react-i18next";
import { DirtyBar } from "@/components/shared/DirtyBar";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Segmented } from "@/components/ui/segmented";
import { Switch } from "@/components/ui/switch";
import { toast } from "@/components/ui/sonner";
import type { AssignmentDefaults } from "@/features/assignments/draft";
import { savePreferences } from "@/features/auth/accountPreferences";
import { failureMessage } from "@/lib/api/errors";
import { authStore, useAuthStore } from "@/stores/auth";

const DURATIONS = [30, 45, 60, 90] as const;

const RULES = [
  { field: "shuffleQuestions", key: "shuffle" },
  { field: "showScore", key: "score" },
  { field: "blockCopyPaste", key: "copy" },
  { field: "requireFullscreen", key: "fullscreen" },
] as const;

type Defaults = Required<AssignmentDefaults>;

/**
 * FALLBACK is what a new assignment starts with when nothing is stored, the
 * same as `emptyDraft`'s: 45 minutes, not shuffled, the score shown, copy
 * and paste blocked, fullscreen off.
 */
const FALLBACK: Defaults = {
  durationMinutes: 45,
  shuffleQuestions: false,
  showScore: true,
  blockCopyPaste: true,
  requireFullscreen: false,
};

function withFallback(stored: AssignmentDefaults | undefined): Defaults {
  return { ...FALLBACK, ...stored };
}

function same(a: Defaults, b: Defaults): boolean {
  return (
    a.durationMinutes === b.durationMinutes &&
    a.shuffleQuestions === b.shuffleQuestions &&
    a.showScore === b.showScore &&
    a.blockCopyPaste === b.blockCopyPaste &&
    a.requireFullscreen === b.requireFullscreen
  );
}

/**
 * AssignmentDefaultsSection is the Assignment defaults card of the teacher's
 * settings: the time limit (30, 45, 60 or 90 minutes) and four rules that
 * every new assignment starts with, and that each assignment can still
 * change. A change raises the DirtyBar; Save changes stores all five in
 * `preferences.assignmentDefaults`, and Discard restores the saved ones.
 */
export function AssignmentDefaultsSection() {
  const { t } = useTranslation();
  const stored = useAuthStore((s) => s.user?.preferences?.assignmentDefaults);
  const saved = withFallback(stored);
  const [draft, setDraft] = useState<Defaults | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const current = draft ?? saved;
  const dirty = draft !== null && !same(draft, saved);

  function change(patch: Partial<Defaults>) {
    setDraft({ ...current, ...patch });
  }

  async function submit() {
    const lease = authStore.captureActor();
    setSaving(true);
    setError(null);
    try {
      await savePreferences({ assignmentDefaults: current });
      if (!authStore.isCurrent(lease)) return;
      setDraft(null);
      toast(t("settings.saved"));
    } catch (cause) {
      if (authStore.isCurrent(lease)) setError(failureMessage(cause, t("api.failed")));
    } finally {
      if (authStore.isCurrent(lease)) setSaving(false);
    }
  }

  return (
    <>
      <SettingsCard
        title={t("settings.defaults.title")}
        description={t("settings.defaults.hint")}
        className="overflow-hidden"
      >
        <div className="flex flex-col gap-2 px-4.5 py-3.5">
          <span id="defaults-duration" className="text-ui leading-normal font-medium">
            {t("settings.defaults.duration")}
          </span>
          <Segmented
            label={t("settings.defaults.duration")}
            value={String(current.durationMinutes)}
            fill
            options={DURATIONS.map((minutes) => ({
              value: String(minutes),
              label: t("settings.defaults.minutes", { count: minutes }),
            }))}
            onChange={(value) => change({ durationMinutes: Number(value) })}
          />
        </div>
        {RULES.map(({ field, key }) => (
          <div key={field} className="flex items-center gap-3.5 border-t px-4.5 py-3">
            <span className="min-w-0 flex-1">
              <span
                id={`defaults-${key}`}
                className="text-ui block leading-normal font-medium"
              >
                {t(`settings.defaults.rules.${key}`)}
              </span>
              <span
                id={`defaults-${key}-hint`}
                className="text-muted-fg text-meta block leading-normal"
              >
                {t(`settings.defaults.rules.${key}Hint`)}
              </span>
            </span>
            <Switch
              checked={current[field]}
              aria-labelledby={`defaults-${key}`}
              aria-describedby={`defaults-${key}-hint`}
              disabled={saving}
              onCheckedChange={(on) => change({ [field]: on })}
            />
          </div>
        ))}
      </SettingsCard>
      <DirtyBar
        dirty={dirty}
        saving={saving}
        error={error}
        onDiscard={() => {
          setError(null);
          setDraft(null);
        }}
        onSave={() => void submit()}
      />
    </>
  );
}
