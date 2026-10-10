import { useId } from "react";
import { useTranslation } from "react-i18next";
import { Eye, Info, ShieldCheck, Shuffle } from "lucide-react";
import { NumberStepper } from "@/components/shared/form/NumberStepper";
import { Segmented } from "@/components/ui/segmented";
import { Textarea } from "@/components/ui/textarea";
import {
  RuleGroup,
  RuleRow,
  SwitchRow,
} from "@/features/assignments/components/wizard/RuleGroup";
import type { AssignmentDraft } from "@/features/assignments/draft";
import {
  NOTE_LIMIT,
  leavingMode,
  leavingValue,
  type RulesPatch,
} from "@/features/assignments/wizardValues";

/**
 * RulesStep is the wizard's Rules: question order, what students see after
 * submitting and when, the class average, the integrity policy (D8) and the
 * note to students (DG-71). Explanations wait for correct answers, and the
 * action on passing the limit waits for a limit.
 */
export function RulesStep({
  draft,
  onChange,
}: Readonly<{ draft: AssignmentDraft; onChange: (patch: RulesPatch) => void }>) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-col gap-3.5">
      <h2 className="text-md font-semibold">{t("assignments.wizard.rulesTitle")}</h2>
      <OrderRules draft={draft} onChange={onChange} />
      <ResultRules draft={draft} onChange={onChange} />
      <IntegrityRules draft={draft} onChange={onChange} />
      <StudentNoteField draft={draft} onChange={onChange} />
    </div>
  );
}

/** OrderRules is the Rules' question order: shuffle the questions within each section and the options. */
export function OrderRules({
  draft,
  onChange,
}: Readonly<{ draft: AssignmentDraft; onChange: (patch: RulesPatch) => void }>) {
  const { t } = useTranslation();
  return (
    <RuleGroup icon={Shuffle} title={t("assignments.wizard.order")}>
      <SwitchRow
        label={t("assignments.wizard.shuffleQuestions")}
        hint={t("assignments.wizard.shuffleQuestionsHint")}
        checked={draft.shuffleQuestions}
        onChange={(shuffleQuestions) => onChange({ shuffleQuestions })}
      />
      <SwitchRow
        label={t("assignments.wizard.shuffleOptions")}
        hint={t("assignments.wizard.shuffleOptionsHint")}
        checked={draft.shuffleOptions}
        onChange={(shuffleOptions) => onChange({ shuffleOptions })}
      />
    </RuleGroup>
  );
}

/** ResultRules is what students see after submitting and when, and the class average; explanations wait for correct answers. */
export function ResultRules({
  draft,
  onChange,
}: Readonly<{ draft: AssignmentDraft; onChange: (patch: RulesPatch) => void }>) {
  const { t } = useTranslation();
  const { review } = draft;
  return (
    <RuleGroup icon={Eye} title={t("assignments.wizard.afterSubmit")}>
      <SwitchRow
        label={t("assignments.wizard.score")}
        hint={t("assignments.wizard.scoreHint")}
        checked={review.showScore}
        onChange={(showScore) => onChange({ review: { showScore } })}
      />
      <SwitchRow
        label={t("assignments.wizard.correct")}
        hint={t("assignments.wizard.correctHint")}
        checked={review.showCorrectAnswers}
        onChange={(showCorrectAnswers) =>
          onChange({
            review: showCorrectAnswers
              ? { showCorrectAnswers }
              : { showCorrectAnswers, showExplanations: false },
          })
        }
      />
      <SwitchRow
        label={t("assignments.wizard.explanations")}
        hint={t(
          review.showCorrectAnswers
            ? "assignments.wizard.explanationsHint"
            : "assignments.wizard.explanationsLocked",
        )}
        checked={review.showExplanations}
        disabled={!review.showCorrectAnswers}
        onChange={(showExplanations) => onChange({ review: { showExplanations } })}
      />
      <RuleRow
        label={t("assignments.wizard.release")}
        hint={t(
          review.release === "after_close"
            ? "assignments.wizard.releaseAfterCloseHint"
            : "assignments.wizard.releaseOnSubmitHint",
        )}
      >
        <Segmented
          fill
          className="[&>button]:justify-center"
          label={t("assignments.wizard.release")}
          value={review.release ?? "on_submit"}
          options={[
            { value: "on_submit", label: t("assignments.wizard.releaseOnSubmit") },
            {
              value: "after_close",
              label: t("assignments.wizard.releaseAfterClose"),
            },
          ]}
          onChange={(release) =>
            onChange({
              review: {
                release: release === "after_close" ? "after_close" : "on_submit",
              },
            })
          }
        />
      </RuleRow>
      <SwitchRow
        label={t("assignments.wizard.classAverage")}
        hint={t("assignments.wizard.classAverageHint")}
        checked={review.showClassAverage ?? false}
        onChange={(showClassAverage) => onChange({ review: { showClassAverage } })}
      />
    </RuleGroup>
  );
}

/** IntegrityRules is the integrity policy (D8) with the honest limits of spec §10.5; the action on passing the limit waits for a limit. */
export function IntegrityRules({
  draft,
  onChange,
}: Readonly<{ draft: AssignmentDraft; onChange: (patch: RulesPatch) => void }>) {
  const { t } = useTranslation();
  const { integrity } = draft;
  const mode = leavingMode(integrity.maxFocusLoss);
  const unlimited = mode === "unlimited";
  return (
    <RuleGroup icon={ShieldCheck} title={t("assignments.wizard.integrity")}>
      <SwitchRow
        label={t("assignments.wizard.fullscreen")}
        hint={t("assignments.wizard.fullscreenHint")}
        checked={integrity.requireFullscreen}
        onChange={(requireFullscreen) => onChange({ integrity: { requireFullscreen } })}
      />
      <SwitchRow
        label={t("assignments.wizard.copy")}
        hint={t("assignments.wizard.copyHint")}
        checked={integrity.blockCopyPaste}
        onChange={(blockCopyPaste) => onChange({ integrity: { blockCopyPaste } })}
      />
      <RuleRow
        label={t("assignments.wizard.leaving")}
        hint={t("assignments.wizard.leavingHint")}
      >
        <span className="flex flex-col gap-2">
          <Segmented
            fill
            className="[&>button]:justify-center"
            label={t("assignments.wizard.leaving")}
            value={mode}
            options={[
              { value: "unlimited", label: t("assignments.wizard.leavingNoLimit") },
              { value: "none", label: t("assignments.wizard.leavingNone") },
              { value: "limit", label: t("assignments.wizard.leavingLimit") },
            ]}
            onChange={(next) =>
              onChange({
                integrity: {
                  maxFocusLoss: leavingValue(
                    next === "none" || next === "limit" ? next : "unlimited",
                    integrity.maxFocusLoss,
                  ),
                },
              })
            }
          />
          {mode === "limit" && (
            <span className="flex flex-wrap items-center gap-2.5">
              <NumberStepper
                label={t("assignments.wizard.timesAllowed")}
                value={integrity.maxFocusLoss}
                min={1}
                max={99}
                format={(count) => t("assignments.wizard.leavingTimes", { count })}
                onChange={(maxFocusLoss) => onChange({ integrity: { maxFocusLoss } })}
              />
              <span className="text-muted-fg text-meta">
                {t("assignments.wizard.timesAllowed")}
              </span>
            </span>
          )}
        </span>
      </RuleRow>
      <RuleRow
        dimmed={unlimited}
        label={t("assignments.wizard.action")}
        hint={
          unlimited
            ? t("assignments.wizard.actionHintNoLimit")
            : t(`assignments.wizard.actionHint.${integrity.onLimitExceeded}`)
        }
      >
        <Segmented
          fill
          className="[&>button]:justify-center"
          label={t("assignments.wizard.action")}
          value={integrity.onLimitExceeded}
          options={[
            {
              value: "warn",
              label: t("assignments.wizard.actionWarn"),
              disabled: unlimited,
            },
            {
              value: "flag",
              label: t("assignments.wizard.actionFlag"),
              disabled: unlimited,
            },
            {
              value: "auto_submit",
              label: t("assignments.wizard.actionAutoSubmit"),
              disabled: unlimited,
            },
          ]}
          onChange={(next) =>
            onChange({
              integrity: {
                onLimitExceeded:
                  next === "warn" || next === "auto_submit" ? next : "flag",
              },
            })
          }
        />
      </RuleRow>
      <RuleRow
        inline
        label={t("assignments.wizard.away")}
        hint={t("assignments.wizard.awayHint")}
      >
        <NumberStepper
          label={t("assignments.wizard.away")}
          value={Math.round(integrity.minAwayMs / 1000)}
          min={0}
          max={30}
          format={(count) => t("assignments.wizard.seconds", { count })}
          onChange={(seconds) => onChange({ integrity: { minAwayMs: seconds * 1000 } })}
        />
      </RuleRow>
      <div className="bg-sidebar text-muted-fg text-meta flex gap-2 px-3.5 py-2.5 leading-normal">
        <Info aria-hidden="true" className="mt-0.5 size-3.5 flex-none" />
        <div className="min-w-0 flex-1">
          <p>{t("assignments.wizard.honesty")}</p>
          <details className="mt-1.5">
            <summary className="text-fg w-fit cursor-pointer rounded-sm font-medium">
              {t("assignments.wizard.limits.title")}
            </summary>
            <p className="mt-1.5">{t("assignments.wizard.limits.detects")}</p>
            <p className="mt-1.5">{t("assignments.wizard.limits.cannotSee")}</p>
            <p className="mt-1.5">{t("assignments.wizard.limits.conversation")}</p>
          </details>
        </div>
      </div>
    </RuleGroup>
  );
}

/** StudentNoteField is the optional note to students (DG-71), up to NOTE_LIMIT characters. */
export function StudentNoteField({
  draft,
  onChange,
}: Readonly<{ draft: AssignmentDraft; onChange: (patch: RulesPatch) => void }>) {
  const { t } = useTranslation();
  const noteId = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={noteId} className="text-ui font-medium">
        {t("assignments.wizard.note")}
      </label>
      <Textarea
        id={noteId}
        value={draft.studentNote}
        maxLength={NOTE_LIMIT}
        rows={3}
        aria-describedby={`${noteId}-hint`}
        onChange={(event) => onChange({ studentNote: event.target.value })}
      />
      <span
        id={`${noteId}-hint`}
        className="text-muted-fg text-meta flex justify-between gap-3"
      >
        <span>{t("assignments.wizard.noteHint")}</span>
        <span className="tabular-nums">
          {t("assignments.wizard.noteCount", { count: draft.studentNote.length })}
        </span>
      </span>
    </div>
  );
}
