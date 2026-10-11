import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { Timer } from "lucide-react";
import { DateTimeField } from "@/components/shared/DateTimeField";
import { NumberStepper } from "@/components/shared/form/NumberStepper";
import { Segmented } from "@/components/ui/segmented";
import { RuleGroup, RuleRow } from "@/features/assignments/components/wizard/RuleGroup";
import type { AssignmentDraft } from "@/features/assignments/draft";
import { windowIsValid } from "@/features/assignments/wizardValues";

const PRESETS = [30, 45, 60];
const CUSTOM = "custom";

/** SchedulePatch is the part of a draft the Schedule step changes. */
export type SchedulePatch = Partial<
  Pick<AssignmentDraft, "opensAt" | "closesAt" | "durationMinutes" | "maxAttempts">
>;

type Props = Readonly<{
  draft: AssignmentDraft;
  onChange: (patch: SchedulePatch) => void;
}>;

/**
 * ScheduleStep is the wizard's "When can they take it?": the window in the
 * teacher's zone, the time limit as 30, 45 or 60 minutes or a custom value
 * from 1 to 600 in steps of 5, and the attempts per student from 1 to 99.
 */
export function ScheduleStep({ draft, onChange }: Props) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-3.5">
      <h2 className="text-md font-semibold">{t("assignments.wizard.scheduleTitle")}</h2>
      <WindowFields draft={draft} onChange={onChange} />
      <TimingRules draft={draft} onChange={onChange} />
    </div>
  );
}

/** WindowFields is the window's opening and closing in WINDOW_ZONE, with the error a close before the opening raises. */
export function WindowFields({ draft, onChange }: Props) {
  const { t } = useTranslation();
  const opensId = useId();
  const closesId = useId();
  const errorId = useId();
  const invalid = !windowIsValid(draft);
  return (
    <>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-3">
        <div className="flex flex-col gap-1.5">
          <label htmlFor={opensId} className="text-meta font-medium">
            {t("assignments.wizard.opens")}
          </label>
          <DateTimeField
            id={opensId}
            label={t("assignments.wizard.opens")}
            value={draft.opensAt}
            onChange={(opensAt) => onChange({ opensAt })}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor={closesId} className="text-meta font-medium">
            {t("assignments.wizard.closes")}
          </label>
          <DateTimeField
            id={closesId}
            label={t("assignments.wizard.closes")}
            value={draft.closesAt}
            onChange={(closesAt) => onChange({ closesAt })}
            aria-invalid={invalid || undefined}
            aria-describedby={invalid ? errorId : undefined}
          />
        </div>
      </div>
      {invalid && (
        <p id={errorId} role="alert" className="text-danger text-sm">
          {t("assignments.wizard.windowInvalid")}
        </p>
      )}
    </>
  );
}

/** TimingRules is the time limit, as a preset or a custom value, and the attempts per student. */
export function TimingRules({ draft, onChange }: Props) {
  const { t } = useTranslation();
  const [custom, setCustom] = useState(!PRESETS.includes(draft.durationMinutes));
  const minutes = (count: number) => t("assignments.wizard.minutes", { count });
  return (
    <RuleGroup icon={Timer} title={t("assignments.wizard.timeAndAttempts")}>
      <RuleRow
        label={t("assignments.wizard.timeLimit")}
        hint={t("assignments.wizard.timeLimitHint")}
      >
        <span className="flex flex-col gap-2">
          <Segmented
            fill
            className="[&>button]:justify-center"
            label={t("assignments.wizard.timeLimit")}
            value={custom ? CUSTOM : String(draft.durationMinutes)}
            options={[
              ...PRESETS.map((value) => ({
                value: String(value),
                label: minutes(value),
              })),
              { value: CUSTOM, label: t("assignments.wizard.custom") },
            ]}
            onChange={(value) => {
              if (value === CUSTOM) {
                setCustom(true);
                return;
              }
              setCustom(false);
              onChange({ durationMinutes: Number(value) });
            }}
          />
          {custom && (
            <span className="flex flex-wrap items-center gap-2.5">
              <NumberStepper
                label={t("assignments.wizard.timeLimit")}
                value={draft.durationMinutes}
                min={1}
                max={600}
                step={5}
                format={minutes}
                onChange={(durationMinutes) => onChange({ durationMinutes })}
              />
              <span className="text-muted-fg text-meta">
                {t("assignments.wizard.customRange")}
              </span>
            </span>
          )}
        </span>
      </RuleRow>
      <RuleRow
        inline
        label={t("assignments.wizard.attempts")}
        hint={t(
          draft.maxAttempts > 1
            ? "assignments.wizard.attemptsMany"
            : "assignments.wizard.attemptsOne",
        )}
      >
        <NumberStepper
          label={t("assignments.wizard.attempts")}
          value={draft.maxAttempts}
          min={1}
          max={99}
          onChange={(maxAttempts) => onChange({ maxAttempts })}
        />
      </RuleRow>
    </RuleGroup>
  );
}
