import { useRef, useState, type SetStateAction } from "react";
import { useTranslation } from "react-i18next";
import {
  useBeforeUnload,
  useBlocker,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowRight } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { Button } from "@/components/ui/button";
import {
  createAssignment,
  getAssignment,
  updateAssignment,
  type Assignment,
} from "@/features/assignments/api";
import {
  WizardStepper,
  type WizardStep,
} from "@/features/assignments/components/WizardStepper";
import { RulesStep } from "@/features/assignments/components/wizard/RulesStep";
import { ScheduleStep } from "@/features/assignments/components/wizard/ScheduleStep";
import { StudentsStep } from "@/features/assignments/components/wizard/StudentsStep";
import { TestStep } from "@/features/assignments/components/wizard/TestStep";
import { WizardAside } from "@/features/assignments/components/wizard/WizardAside";
import type { Token } from "@/features/assignments/components/TokenField";
import {
  draftBody,
  draftOf,
  emptyDraft,
  windowInstant,
  type AssignmentDraft,
} from "@/features/assignments/draft";
import {
  useClassFromQuery,
  usePickFromQuery,
} from "@/features/assignments/usePreselect";
import { useTargetRoster } from "@/features/assignments/useTargetRoster";
import {
  firstGap,
  stepValues,
  type RulesPatch,
} from "@/features/assignments/wizardValues";
import { listVersions } from "@/features/tests/api";
import { PageHead } from "@/layouts/shell/PageHead";
import type { Locale } from "@/lib/i18n";
import { failureMessage } from "@/lib/api/errors";
import { notify } from "@/lib/toast";
import { useAuthStore } from "@/stores/auth";

const STEP_KEYS = ["test", "students", "schedule", "rules"] as const;
const LAST_STEP = STEP_KEYS.length - 1;
const WINDOW_GAP = "assignments.wizard.windowInvalid";

/**
 * AssignmentWizardPage is the deck's New assignment, and the same wizard
 * for `/teacher/assignments/:id/edit`: a stepper over Test, Students,
 * Schedule and Rules with the open step in `?step=`, and the summary aside.
 * A new assignment starts from the teacher's stored defaults, with a test
 * preselected by `?test=` and a class by `?class=`.
 */
export default function AssignmentWizardPage() {
  const { id } = useParams<{ id: string }>();
  return id === undefined ? <NewAssignment /> : <EditAssignment id={id} />;
}

function NewAssignment() {
  const defaults = useAuthStore((s) => s.user?.preferences?.assignmentDefaults);
  const [params] = useSearchParams();
  return (
    <Wizard
      makeInitial={() => emptyDraft(new Date(), defaults ?? {})}
      testId={params.get("test") ?? params.get("testId")}
      classId={params.get("class") ?? params.get("classId")}
    />
  );
}

function EditAssignment({ id }: Readonly<{ id: string }>) {
  const { t } = useTranslation();
  const existing = useQuery({
    queryKey: ["admin-assignment", id],
    queryFn: ({ signal }) => getAssignment(id, signal),
  });
  const testId = existing.data?.testId;
  const versions = useQuery({
    queryKey: ["admin-test-versions", testId],
    queryFn: ({ signal }) => listVersions(testId ?? "", signal),
    enabled: testId !== undefined,
  });
  if (existing.data && versions.data) {
    const stored = existing.data;
    const items = versions.data.items;
    return (
      <Wizard
        key={stored.id}
        existing={stored}
        makeInitial={() => draftOf(stored, items)}
        testId={null}
        classId={null}
      />
    );
  }
  if (!existing.isError && !versions.isError)
    return (
      <div data-scale="deck">
        <ListSkeleton rows={8} />
      </div>
    );
  return (
    <div data-scale="deck">
      <LoadError
        error={existing.error ?? versions.error}
        onRetry={() => {
          void existing.refetch();
          void versions.refetch();
        }}
      >
        {t("assignments.detail.loadFailed")}
      </LoadError>
    </div>
  );
}

function Wizard({
  existing,
  makeInitial,
  testId,
  classId,
}: Readonly<{
  existing?: Assignment;
  makeInitial: () => AssignmentDraft;
  testId: string | null;
  classId: string | null;
}>) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [params, setParams] = useSearchParams();
  const [draft, setDraft] = useState<AssignmentDraft>(makeInitial);
  const [dirty, setDirty] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const leaving = useRef(false);
  const saving = useRef(false);
  const published = existing !== undefined && existing.publishedAt !== null;

  usePickFromQuery(testId, setDraft);
  useClassFromQuery(classId, setDraft);
  const roster = useTargetRoster(
    draft.classes.map((klass) => klass.id),
    draft.students.map((student) => student.id),
  );
  const students = roster.data?.total ?? null;

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
  const editRules = (patch: RulesPatch) =>
    edit((current) => ({
      ...current,
      ...patch,
      review: { ...current.review, ...patch.review },
      integrity: { ...current.integrity, ...patch.integrity },
    }));

  const save = useMutation({
    mutationFn: ({ asDraft, body }: { asDraft: boolean; body: AssignmentDraft }) => {
      const input = { draft: asDraft, ...draftBody(body) };
      return existing === undefined
        ? createAssignment(input)
        : updateAssignment(existing.id, input);
    },
    onSuccess: async (saved, { asDraft, body }) => {
      leaving.current = true;
      await queryClient.invalidateQueries({ queryKey: ["admin-assignments"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-assignment", saved.id] });
      await queryClient.invalidateQueries({ queryKey: ["admin-dashboard"] });
      if (asDraft) {
        notify.success(t("assignments.wizard.savedDraft"));
        void navigate("/teacher/assignments?status=draft");
      } else if (published) {
        notify.success(t("assignments.wizard.saved"));
        void navigate(`/teacher/assignments/${saved.id}`);
      } else if (windowInstant(body.opensAt).getTime() > Date.now()) {
        notify.success(t("assignments.wizard.assigned"));
        void navigate("/teacher/assignments?status=scheduled");
      } else {
        notify.success(t("assignments.wizard.assignedOpen"));
        void navigate("/teacher/assignments?status=open");
      }
    },
    onError: (cause, { asDraft }) => {
      saving.current = false;
      setError(failureMessage(cause, t(failedKey(asDraft, existing !== undefined))));
    },
  });
  const saveDraft = () => {
    if (saving.current) return;
    if (draft.picked === null) {
      setError(t("assignments.wizard.needTest"));
      return;
    }
    saving.current = true;
    save.mutate({ asDraft: true, body: draft });
  };
  const assign = () => {
    if (saving.current) return;
    const gap = firstGap(draft);
    if (gap !== null) {
      open(gap.step);
      setError(gap.key === WINDOW_GAP ? null : t(gap.key));
      return;
    }
    saving.current = true;
    save.mutate({ asDraft: false, body: draft });
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
  }));

  return (
    <div data-scale="deck" className="flex min-w-0 flex-col gap-4.5">
      <PageHead
        title={existing === undefined ? t("assignments.new") : t("assignments.edit")}
        description={
          published
            ? t("assignments.wizard.editDescription")
            : t("assignments.wizard.description")
        }
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
                onStudentsChange={(picked) =>
                  edit((current) => ({ ...current, students: picked }))
                }
              />
            )}
            {step === 2 && (
              <ScheduleStep
                draft={draft}
                onChange={(patch) => edit((current) => ({ ...current, ...patch }))}
              />
            )}
            {step === 3 && <RulesStep draft={draft} onChange={editRules} />}
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
                  if (step === 0) void navigate(cancelTo(existing));
                  else open(step - 1);
                }}
              >
                {step === 0 ? t("common.cancel") : t("common.back")}
              </Button>
              <span className="ml-auto flex flex-wrap items-center justify-end gap-2">
                {!published && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="md"
                    disabled={save.isPending}
                    onClick={saveDraft}
                  >
                    {t("assignments.saveDraft")}
                  </Button>
                )}
                {step < LAST_STEP ? (
                  <Button
                    type="button"
                    size="md"
                    className="px-4"
                    onClick={() => open(step + 1)}
                  >
                    {t("assignments.wizard.continue")}
                    <ArrowRight aria-hidden="true" />
                  </Button>
                ) : (
                  <Button
                    type="button"
                    size="md"
                    className="px-4"
                    disabled={save.isPending}
                    onClick={assign}
                  >
                    {finalLabel(t, published, students)}
                    <ArrowRight aria-hidden="true" />
                  </Button>
                )}
              </span>
            </div>
          </div>
        </section>
        <WizardAside draft={draft} students={students} />
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
  return Math.min(n - 1, LAST_STEP);
}

function cancelTo(existing: Assignment | undefined): string {
  return existing === undefined
    ? "/teacher/assignments"
    : `/teacher/assignments/${existing.id}`;
}

function failedKey(asDraft: boolean, editing: boolean): string {
  if (asDraft) return "assignments.wizard.saveFailed";
  return editing ? "assignments.updateFailed" : "assignments.createFailed";
}

function finalLabel(
  t: ReturnType<typeof useTranslation>["t"],
  published: boolean,
  students: number | null,
): string {
  if (published) return t("assignments.saveChanges");
  if (students === null || students === 0) return t("assignments.assign");
  return t("assignments.wizard.assignTo", { count: students });
}
