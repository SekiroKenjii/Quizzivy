import { useId, useRef, useState, type SetStateAction } from "react";
import { useTranslation } from "react-i18next";
import {
  useBeforeUnload,
  useBlocker,
  useNavigate,
  useSearchParams,
} from "react-router";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { TFunction } from "i18next";
import { ArrowRight } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { Button } from "@/components/ui/button";
import { createAssignment } from "@/features/assignments/api";
import {
  WizardStepper,
  type WizardStep,
} from "@/features/assignments/components/WizardStepper";
import { StudentsStep } from "@/features/assignments/components/wizard/StudentsStep";
import { TestStep } from "@/features/assignments/components/wizard/TestStep";
import type { Token } from "@/features/assignments/components/TokenField";
import {
  draftBody,
  emptyDraft,
  type AssignmentDraft,
} from "@/features/assignments/draft";
import {
  useClassFromQuery,
  usePickFromQuery,
} from "@/features/assignments/usePreselect";
import { useTargetRoster } from "@/features/assignments/useTargetRoster";
import { PageHead } from "@/layouts/shell/PageHead";
import type { Locale } from "@/lib/i18n";
import { dayDate, fromDateTimeInput } from "@/lib/i18n/datetime";
import { failureMessage } from "@/lib/api/errors";
import { notify } from "@/lib/toast";

const STEP_KEYS = ["test", "students", "schedule", "rules"] as const;
const LAST_OPEN_STEP = 1;

/**
 * AssignmentWizardPage is the deck's New assignment: a stepper over Test,
 * Students, Schedule and Rules, the open step in `?step=`, a test preselected
 * by `?test=` and a class by `?class=`. Save draft sends every field of the
 * draft, the defaults included, and opens the Drafts tab. Leaving with
 * unsaved choices asks first.
 */
export default function AssignmentWizardPage() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [draft, setDraft] = useState<AssignmentDraft>(() => emptyDraft());
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const leaving = useRef(false);
  const laterId = useId();

  usePickFromQuery(params.get("test") ?? params.get("testId"), setDraft);
  useClassFromQuery(params.get("class") ?? params.get("classId"), setDraft);
  const roster = useTargetRoster(
    draft.classes.map((klass) => klass.id),
    draft.students.map((student) => student.id),
  );

  const step = stepIndex(params.get("step"));
  const open = (index: number) => {
    setError(null);
    setParams((current) => {
      const next = new URLSearchParams(current);
      next.set("step", String(index + 1));
      return next;
    });
  };
  const edit = (change: SetStateAction<AssignmentDraft>) => {
    setDraft(change);
    setDirty(true);
    setError(null);
  };

  const save = useMutation({
    mutationFn: (body: AssignmentDraft) =>
      createAssignment({ draft: true, ...draftBody(body) }),
    onSuccess: async () => {
      leaving.current = true;
      await queryClient.invalidateQueries({ queryKey: ["admin-assignments"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-dashboard"] });
      notify.success(t("assignments.wizard.savedDraft"));
      void navigate("/teacher/assignments?status=draft");
    },
    onError: (cause) =>
      setError(failureMessage(cause, t("assignments.wizard.saveFailed"))),
  });
  const saveDraft = () => {
    if (draft.picked === null) {
      setError(t("assignments.wizard.needTest"));
      return;
    }
    save.mutate(draft);
  };

  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      dirty &&
      !leaving.current &&
      currentLocation.pathname !== nextLocation.pathname &&
      nextLocation.pathname !== "/login",
  );
  useBeforeUnload((event) => {
    if (dirty && !leaving.current) event.preventDefault();
  });

  const values = stepValues(draft, t, i18n.language as Locale);
  const steps: WizardStep[] = STEP_KEYS.map((key, index) => ({
    label: t(`assignments.wizard.steps.${key}`),
    value: values[index] ?? "",
    disabled: index > LAST_OPEN_STEP,
  }));

  return (
    <div data-scale="deck" className="flex min-w-0 flex-col gap-4.5">
      <PageHead
        title={t("assignments.new")}
        description={t("assignments.wizard.description")}
      />
      <WizardStepper steps={steps} current={step} onOpen={open} />
      <div className="flex flex-wrap items-start gap-3.5">
        <section className="bg-card shadow-card border-border min-w-0 flex-[3_1_460px] rounded-xl border">
          <div className="p-4.5">
            {step === 0 && (
              <TestStep
                picked={draft.picked}
                onPick={(picked) => edit((current) => ({ ...current, picked }))}
              />
            )}
            {step === 1 && (
              <StudentsStep
                classes={draft.classes}
                students={draft.students}
                roster={roster}
                onToggleClass={(token: Token) =>
                  edit((current) => ({
                    ...current,
                    classes: current.classes.some((klass) => klass.id === token.id)
                      ? current.classes.filter((klass) => klass.id !== token.id)
                      : [...current.classes, token],
                  }))
                }
                onStudentsChange={(students) =>
                  edit((current) => ({ ...current, students }))
                }
              />
            )}
          </div>
          <div className="border-border flex flex-col gap-2 border-t px-4.5 py-3">
            {error !== null && (
              <p role="alert" className="text-danger text-sm">
                {error}
              </p>
            )}
            <div className="flex flex-wrap items-center justify-between gap-2">
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => {
                  if (step === 0) void navigate("/teacher/assignments");
                  else open(step - 1);
                }}
              >
                {step === 0 ? t("common.cancel") : t("common.back")}
              </Button>
              <span className="ml-auto flex flex-wrap items-center justify-end gap-2">
                <Button
                  type="button"
                  variant="ghost"
                  size="md"
                  disabled={save.isPending}
                  onClick={saveDraft}
                >
                  {t("assignments.saveDraft")}
                </Button>
                <Button
                  type="button"
                  size="md"
                  className="px-4"
                  disabled={step >= LAST_OPEN_STEP}
                  aria-describedby={step >= LAST_OPEN_STEP ? laterId : undefined}
                  onClick={() => open(step + 1)}
                >
                  {t("assignments.wizard.continue")}
                  <ArrowRight aria-hidden="true" />
                </Button>
              </span>
            </div>
            {step >= LAST_OPEN_STEP && (
              <p id={laterId} className="text-muted-fg text-meta text-right">
                {t("assignments.wizard.laterSteps")}
              </p>
            )}
          </div>
        </section>
      </div>
      <ConfirmDialog
        open={blocker.state === "blocked"}
        onOpenChange={(next) => {
          if (!next && blocker.state === "blocked") blocker.reset();
        }}
        title={t("assignments.wizard.leave.title")}
        description={t("assignments.wizard.leave.body")}
        confirmLabel={t("assignments.wizard.leave.confirm")}
        cancelLabel={t("assignments.wizard.leave.stay")}
        destructive
        onConfirm={() => {
          if (blocker.state === "blocked") blocker.proceed();
        }}
      />
    </div>
  );
}

function stepIndex(raw: string | null): number {
  const n = Number(raw);
  if (!Number.isInteger(n) || n < 1) return 0;
  return Math.min(n - 1, LAST_OPEN_STEP);
}

function stepValues(draft: AssignmentDraft, t: TFunction, locale: Locale): string[] {
  const targets = [
    ...draft.classes.map((klass) => klass.label),
    ...(draft.students.length > 0
      ? [t("assignments.wizard.value.individuals", { count: draft.students.length })]
      : []),
  ];
  const leaving = draft.integrity.maxFocusLoss;
  const rules = [
    draft.integrity.requireFullscreen && t("assignments.wizard.value.fullscreen"),
    draft.integrity.blockCopyPaste && t("assignments.wizard.value.noCopy"),
    leaving === -1 && t("assignments.wizard.value.noLeaving"),
    leaving > 0 && t("assignments.wizard.value.leavingAllowed", { count: leaving }),
    draft.review.showScore && t("assignments.wizard.value.scoreShown"),
  ].filter((part): part is string => typeof part === "string");
  return [
    draft.picked?.testTitle ?? t("assignments.wizard.value.noTest"),
    targets.length > 0 ? targets.join(", ") : t("assignments.wizard.value.noOne"),
    t("assignments.wizard.value.schedule", {
      opens: dayDate(fromDateTimeInput(draft.opensAt), locale),
      closes: dayDate(fromDateTimeInput(draft.closesAt), locale),
      minutes: draft.durationMinutes,
    }),
    rules.length > 0 ? rules.join(" · ") : t("assignments.wizard.value.defaultRules"),
  ];
}
