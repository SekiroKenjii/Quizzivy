import { useState } from "react";
import { useTranslation } from "react-i18next";
import { CircleAlert, CircleCheck, Info, Send } from "lucide-react";
import { FormDialog, type FormInfoRow } from "@/components/shared/form/FormDialog";
import type { PublishViolation } from "@/features/tests/api";

/** PublishProblem is one question the builder already knows publishing would refuse. */
export type PublishProblem = Readonly<{
  questionId: string;
  message: string;
  location: string | null;
}>;

interface PublishDialogProps {
  open: boolean;
  pending: boolean;
  error: string | null;
  problems: readonly PublishProblem[];
  violations: PublishViolation[] | null;
  warnings?: readonly { questionId: string; message: string }[];
  location?: (violation: PublishViolation) => string | null;
  onOpenChange: (open: boolean) => void;
  onGoTo: (questionId: string) => void;
  onGoToSection?: (sectionId: string) => void;
  onPublish: (changeNote: string) => void;
}

/**
 * PublishDialog is "Publish test?": the checks, an optional change note, and
 * Publish. The checks are the builder's own `publishProblem` for each loaded
 * question until the server answers, and the server's violations after a
 * refused publish; each row that names a question or a section has "Fix it",
 * which closes the dialog and goes there. While any check fails, Publish
 * refuses with an alert instead of sending. Missing explanations are listed
 * and never block.
 */
export function PublishDialog({
  open,
  pending,
  error,
  problems,
  violations,
  warnings = [],
  location,
  onOpenChange,
  onGoTo,
  onGoToSection,
  onPublish,
}: Readonly<PublishDialogProps>) {
  const { t } = useTranslation();
  const fix = t("builder.fixIt");
  const blocking = violations !== null ? violations.length > 0 : problems.length > 0;
  const [attempt, setAttempt] = useState({ open, refused: false });
  if (attempt.open !== open) setAttempt({ open, refused: false });

  function rows(): FormInfoRow[] {
    const checks: FormInfoRow[] =
      violations !== null
        ? violations.map((violation) => {
            const where = location?.(violation);
            const go = violationTarget(violation, onGoTo, onGoToSection);
            return {
              icon: CircleAlert,
              tone: "danger",
              text: where ? `${where} · ${violation.message}` : violation.message,
              ...(go ? { action: { label: fix, onAction: go } } : {}),
            };
          })
        : problems.map((problem) => ({
            icon: CircleAlert,
            tone: "warning",
            text: problem.location
              ? `${problem.location} · ${problem.message}`
              : problem.message,
            action: { label: fix, onAction: () => onGoTo(problem.questionId) },
          }));
    const passed: FormInfoRow[] = blocking
      ? []
      : [
          { icon: CircleCheck, tone: "success", text: t("builder.checkAnswers") },
          { icon: CircleCheck, tone: "success", text: t("builder.checkPoints") },
        ];
    const notes: FormInfoRow[] = warnings.map((warning) => ({
      icon: Info,
      tone: "info",
      text: warning.message,
      action: { label: fix, onAction: () => onGoTo(warning.questionId) },
    }));
    return [...checks, ...passed, ...notes];
  }

  return (
    <FormDialog
      open={open}
      onOpenChange={onOpenChange}
      title={t("builder.publishTitle")}
      description={t("builder.publishBody")}
      icon={Send}
      initial={{ checks: "", note: "" }}
      fields={[
        { kind: "info", name: "checks", label: t("builder.checks"), rows: rows() },
        {
          kind: "area",
          name: "note",
          label: t("builder.changeNote"),
          placeholder: t("builder.changeNotePlaceholder"),
          maxLength: 200,
        },
      ]}
      submitLabel={pending ? t("builder.publishing") : t("builder.publish")}
      pending={pending}
      error={attempt.refused && blocking ? t("builder.fixBeforePublishing") : error}
      onSubmit={({ note }) => {
        if (blocking) setAttempt({ open, refused: true });
        else onPublish(note);
      }}
    />
  );
}

function violationTarget(
  violation: PublishViolation,
  onGoTo: (questionId: string) => void,
  onGoToSection: ((sectionId: string) => void) | undefined,
): (() => void) | null {
  const { questionId, sectionId } = violation;
  if (questionId) return () => onGoTo(questionId);
  if (sectionId && onGoToSection) return () => onGoToSection(sectionId);
  return null;
}
