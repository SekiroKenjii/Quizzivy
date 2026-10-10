import { useId } from "react";
import { useTranslation } from "react-i18next";
import { SelectField } from "@/components/shared/form/fields/SelectField";
import { Input } from "@/components/ui/input";
import type { QuestionValues } from "@/features/question-bank/questionSchema";
import {
  BANK_LEVELS,
  BANK_SKILLS,
} from "@/features/question-bank/pages/teacher/bankFilters";
import { TagsField } from "./TagsField";

const LABEL = "text-[13px] leading-4 font-medium";

/**
 * QuestionSettingsFields is the Question editor's SETTINGS group: Points,
 * with its error under the input; Level and Skill, each optional with
 * "Not set"; and Tags.
 */
export function QuestionSettingsFields({
  value,
  onChange,
}: Readonly<{ value: QuestionValues; onChange: (value: QuestionValues) => void }>) {
  const { t } = useTranslation();
  const ids = { points: useId(), error: useId(), level: useId(), skill: useId() };
  const notSet = { value: "", label: t("questionEditor.notSet") };
  const invalid = !Number.isFinite(value.points) || value.points <= 0;
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-1.5">
        <label className={LABEL} htmlFor={ids.points}>
          {t("questionEditor.points")}
        </label>
        <Input
          id={ids.points}
          type="number"
          min={0.01}
          step={0.5}
          value={value.points}
          aria-invalid={invalid || undefined}
          aria-describedby={invalid ? ids.error : undefined}
          className="bg-background"
          onChange={(event) =>
            onChange({ ...value, points: Number(event.target.value) })
          }
        />
        {invalid ? (
          <p id={ids.error} className="text-danger-ink text-xs">
            {t("questionEditor.pointsError")}
          </p>
        ) : null}
      </div>
      <div className="flex flex-col gap-1.5">
        <label className={LABEL} htmlFor={ids.level}>
          {t("questionEditor.level")}
        </label>
        <SelectField
          id={ids.level}
          label={t("questionEditor.level")}
          value={value.level ?? ""}
          options={[
            notSet,
            ...BANK_LEVELS.map((level) => ({
              value: level,
              label: t(`bank.level.${level}`),
            })),
          ]}
          onChange={(next) =>
            onChange({
              ...value,
              level: BANK_LEVELS.find((level) => level === next) ?? null,
            })
          }
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <label className={LABEL} htmlFor={ids.skill}>
          {t("questionEditor.skill")}
        </label>
        <SelectField
          id={ids.skill}
          label={t("questionEditor.skill")}
          value={value.skill ?? ""}
          options={[
            notSet,
            ...BANK_SKILLS.map((skill) => ({
              value: skill,
              label: t(`tests.skill.${skill}`),
            })),
          ]}
          onChange={(next) =>
            onChange({
              ...value,
              skill: BANK_SKILLS.find((skill) => skill === next) ?? null,
            })
          }
        />
      </div>
      <TagsField tags={value.tags} onChange={(tags) => onChange({ ...value, tags })} />
    </div>
  );
}
