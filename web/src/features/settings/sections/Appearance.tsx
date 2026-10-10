import { useId } from "react";
import { useTranslation } from "react-i18next";
import { SettingsCard } from "@/components/shared/SettingsCard";
import { Switch } from "@/components/ui/switch";
import {
  chooseAccountPreference,
  useAccountPreferenceStatus,
} from "@/features/auth/accountPreferences";
import { useCompactTables } from "@/lib/compactTables";
import { useThemePreference, type ThemePreference } from "@/lib/theme";
import { cn } from "@/lib/utils";

const THEMES: readonly ThemePreference[] = ["light", "dark", "system"];

const SWATCH: Record<ThemePreference, { side: string; line: string }> = {
  light: { side: "bg-swatch-light-side", line: "bg-swatch-light-line" },
  dark: { side: "bg-swatch-dark-side", line: "bg-swatch-dark-line" },
  system: { side: "bg-swatch-device", line: "bg-swatch-device" },
};

function Swatch({ theme }: Readonly<{ theme: ThemePreference }>) {
  const { side, line } = SWATCH[theme];
  return (
    <span
      data-swatch={theme}
      aria-hidden="true"
      className="rounded-seg relative flex h-16 gap-1.5 overflow-hidden border p-2"
    >
      <span className="absolute inset-0 flex">
        {theme === "dark" ? null : <span className="bg-swatch-light flex-1" />}
        {theme === "light" ? null : <span className="bg-swatch-dark flex-1" />}
      </span>
      <span className={cn("relative w-[24%] rounded-sm", side)} />
      <span className="relative flex flex-1 flex-col gap-1.25">
        <span className={cn("h-2 w-[60%] rounded-[3px]", line)} />
        <span className={cn("h-2 w-[85%] rounded-[3px]", line)} />
      </span>
    </span>
  );
}

/**
 * AppearanceSection is the Appearance card of the teacher's settings: Light,
 * Dark or Match device, whose swatch is split rather than a gradient (DG-34),
 * and Compact tables (DG-37). Each choice applies at once and is saved to the
 * account's preferences; neither raises the DirtyBar or is undone by Discard.
 */
export function AppearanceSection() {
  const { t } = useTranslation();
  const themeLabel = useId();
  const compactLabel = useId();
  const preference = useThemePreference();
  const compact = useCompactTables();
  const saving = useAccountPreferenceStatus().phase === "saving";
  return (
    <SettingsCard
      title={t("settings.appearance.title")}
      description={t("settings.appearance.hint")}
    >
      <div className="flex flex-col gap-4.5 p-4.5">
        <div className="flex flex-col gap-2">
          <span id={themeLabel} className="text-ui leading-normal font-medium">
            {t("settings.appearance.theme")}
          </span>
          <div
            role="group"
            aria-labelledby={themeLabel}
            className="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-2.5"
          >
            {THEMES.map((theme) => (
              <button
                key={theme}
                type="button"
                aria-pressed={preference === theme}
                disabled={saving}
                className={cn(
                  "bg-card flex flex-col gap-2 rounded-[10px] border p-2 text-left",
                  preference === theme && "border-primary ring-primary ring-1",
                )}
                onClick={() => void chooseAccountPreference({ theme })}
              >
                <Swatch theme={theme} />
                <span className="px-0.5 text-sm leading-[normal] font-medium">
                  {t(`settings.appearance.themes.${theme}`)}
                </span>
              </button>
            ))}
          </div>
        </div>
        <div className="flex items-center gap-3.5">
          <span className="min-w-0 flex-1">
            <span
              id={compactLabel}
              className="text-ui block leading-normal font-medium"
            >
              {t("settings.appearance.compact")}
            </span>
            <span
              id={`${compactLabel}-hint`}
              className="text-muted-fg text-meta block leading-normal"
            >
              {t("settings.appearance.compactHint")}
            </span>
          </span>
          <Switch
            checked={compact}
            aria-labelledby={compactLabel}
            aria-describedby={`${compactLabel}-hint`}
            disabled={saving}
            onCheckedChange={(compactTables) =>
              void chooseAccountPreference({ compactTables })
            }
          />
        </div>
      </div>
    </SettingsCard>
  );
}
