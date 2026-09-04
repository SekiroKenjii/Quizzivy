import { useId, useMemo, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { FileText, Info, SquarePen, Users } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { PageHeader } from "@/components/shared/PageHeader";
import { PageAside } from "@/components/shared/PageAside";
import {
  TestVersionPicker,
  type PickedVersion,
} from "@/features/assignments/components/TestVersionPicker";
import {
  ClassTargetPicker,
  StudentTargetPicker,
} from "@/features/assignments/components/TargetPickers";
import type { Token } from "@/features/assignments/components/TokenField";
import { StudentRulesPreview } from "@/features/assignments/components/StudentRulesPreview";
import {
  createAssignment,
  getAssignment,
  updateAssignment,
  type Assignment,
  type AssignmentInput,
} from "@/features/assignments/api";
import { fetchClass } from "@/features/classes/api";
import { listVersions, type TestVersion } from "@/features/tests/api";
import { fromDateTimeInput, toDateTimeInput } from "@/lib/i18n/datetime";
import { ApiError, fieldMessages } from "@/lib/api/errors";

const DURATIONS = [15, 30, 45, 60, 90, 120, 180];
const ATTEMPTS = [1, 2, 3];
const FOCUS_LIMITS = [0, 1, 2, 3, 5];

interface Draft {
  picked: PickedVersion | null;
  classes: Token[];
  students: Token[];
  opensAt: string;
  closesAt: string;
  durationMinutes: number;
  maxAttempts: number;
  shuffleQuestions: boolean;
  shuffleOptions: boolean;
  review: {
    showScore: boolean;
    showCorrectAnswers: boolean;
    showExplanations: boolean;
  };
  integrity: {
    requireFullscreen: boolean;
    blockCopyPaste: boolean;
    maxFocusLoss: number;
    onLimitExceeded: "warn" | "flag";
    minAwayMs: number;
  };
}

// §10.3's defaults, restated here so a teacher who changes nothing has chosen
// the conservative option rather than missed a step.
function emptyDraft(): Draft {
  const opens = new Date();
  opens.setMinutes(0, 0, 0);
  const closes = new Date(opens.getTime() + 3 * 24 * 60 * 60 * 1000);

  return {
    picked: null,
    classes: [],
    students: [],
    opensAt: toDateTimeInput(opens),
    closesAt: toDateTimeInput(closes),
    durationMinutes: 45,
    maxAttempts: 1,
    shuffleQuestions: false,
    shuffleOptions: false,
    review: { showScore: true, showCorrectAnswers: false, showExplanations: false },
    integrity: {
      requireFullscreen: false,
      blockCopyPaste: true,
      maxFocusLoss: 0,
      onLimitExceeded: "flag",
      minAwayMs: 3000,
    },
  };
}

/** The deck's G-01. One scrolling form with a rail that shows the consequences. */
export default function AssignmentFormPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [params] = useSearchParams();
  const { id } = useParams<{ id: string }>();
  const editing = id !== undefined;
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [picking, setPicking] = useState(false);
  const [error, setError] = useState<{ summary: string; fields: string[] } | null>(
    null,
  );

  const fromClassId = params.get("classId");
  const fromClassQuery = useQuery({
    queryKey: ["admin-class", fromClassId],
    queryFn: ({ signal }) => fetchClass(fromClassId ?? "", signal),
    enabled: fromClassId !== null,
  });

  const preselected = useMemo(() => {
    const found = fromClassQuery.data;
    return found
      ? { id: found.id, label: found.name, hint: String(found.studentCount) }
      : null;
  }, [fromClassQuery.data]);

  const [appliedFor, setAppliedFor] = useState<string | null>(null);
  if (preselected && appliedFor !== preselected.id) {
    setAppliedFor(preselected.id);
    setDraft((current) =>
      current.classes.some((c) => c.id === preselected.id)
        ? current
        : { ...current, classes: [...current.classes, preselected] },
    );
  }

  const existing = useQuery({
    queryKey: ["admin-assignment", id],
    queryFn: ({ signal }) => getAssignment(id ?? "", signal),
    enabled: editing,
  });
  const testId = existing.data?.testId;
  const versions = useQuery({
    queryKey: ["admin-test-versions", testId],
    queryFn: ({ signal }) => listVersions(testId ?? "", signal),
    enabled: testId !== undefined,
  });
  const [hydratedFor, setHydratedFor] = useState<string | null>(null);
  if (existing.data && versions.data && hydratedFor !== existing.data.id) {
    setHydratedFor(existing.data.id);
    setDraft(fromAssignment(existing.data, versions.data.items));
  }
  const published = existing.data?.publishedAt != null;

  const save = useMutation({
    mutationFn: (asDraft: boolean) => {
      const body = { draft: asDraft, ...toBody(draft) };
      return editing ? updateAssignment(id, body) : createAssignment(body);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["admin-assignments"] });
      await queryClient.invalidateQueries({ queryKey: ["admin-assignment", id] });
      await queryClient.invalidateQueries({ queryKey: ["admin-dashboard"] });
      void navigate(editing ? `/admin/assignments/${id}` : "/admin/assignments");
    },
    onError: (cause) =>
      setError({
        summary:
          cause instanceof ApiError
            ? cause.message
            : t(editing ? "assignments.updateFailed" : "assignments.createFailed"),
        fields: fieldMessages(cause),
      }),
  });

  const hasTargets = draft.classes.length > 0 || draft.students.length > 0;
  const ready = draft.picked !== null && hasTargets;
  const savable = draft.picked !== null;

  if (editing && hydratedFor === null) {
    return existing.isError || versions.isError ? (
      <p role="alert" className="text-sm">
        {t("assignments.detail.loadFailed")}
      </p>
    ) : (
      <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
        {t("common.loading")}
      </p>
    );
  }

  return (
    <>
      <PageHeader
        title={t(editing ? "assignments.edit" : "assignments.new")}
        backTo={editing ? `/admin/assignments/${id}` : "/admin/assignments"}
      />

      <div className="space-y-6">
        <Card>
          <CardHeader>
            <CardTitle>{t("assignments.step1")}</CardTitle>
          </CardHeader>
          <CardContent className="pt-1">
            {draft.picked === null ? (
              <Button variant="outline" onClick={() => setPicking(true)}>
                <FileText aria-hidden="true" />
                {t("assignments.chooseTest")}
              </Button>
            ) : (
              <>
                <div className="flex items-center gap-3 rounded-md border p-3">
                  <FileText
                    className="text-muted-foreground size-5 shrink-0"
                    aria-hidden="true"
                  />
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">
                      {draft.picked.testTitle}
                    </p>
                    <p className="text-muted-foreground text-xs">
                      {t("assignments.versionMetaFull", {
                        questions: draft.picked.version.questionCount,
                        points: draft.picked.version.totalPoints,
                        audio: draft.picked.version.audioCount,
                        version: draft.picked.version.version,
                      })}
                    </p>
                  </div>
                  <Button variant="outline" size="sm" onClick={() => setPicking(true)}>
                    {t("assignments.changeTest")}
                  </Button>
                </div>
                <p className="text-muted-foreground mt-2 text-xs leading-relaxed">
                  {t("assignments.versionPinned", {
                    version: draft.picked.version.version,
                  })}
                </p>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("assignments.step2")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 pt-1">
            <ClassTargetPicker
              selected={draft.classes}
              onAdd={(token) =>
                setDraft((d) => ({ ...d, classes: [...d.classes, token] }))
              }
              onRemove={(id) =>
                setDraft((d) => ({
                  ...d,
                  classes: d.classes.filter((c) => c.id !== id),
                }))
              }
            />
            <StudentTargetPicker
              selected={draft.students}
              onAdd={(token) =>
                setDraft((d) => ({ ...d, students: [...d.students, token] }))
              }
              onRemove={(id) =>
                setDraft((d) => ({
                  ...d,
                  students: d.students.filter((s) => s.id !== id),
                }))
              }
            />
            <div className="bg-muted/40 flex items-start gap-2 rounded-md p-2.5">
              <Users
                className="text-muted-foreground mt-0.5 size-4 shrink-0"
                aria-hidden="true"
              />
              <p className="text-xs leading-relaxed">{t("assignments.rosterNote")}</p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("assignments.step3")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 pt-1">
            <div className="grid grid-cols-2 gap-3">
              <Field label={t("assignments.opensAt")}>
                {(id) => (
                  <Input
                    id={id}
                    type="datetime-local"
                    value={draft.opensAt}
                    onChange={(e) =>
                      setDraft((d) => ({ ...d, opensAt: e.target.value }))
                    }
                  />
                )}
              </Field>
              <Field label={t("assignments.closesAt")}>
                {(id) => (
                  <Input
                    id={id}
                    type="datetime-local"
                    value={draft.closesAt}
                    onChange={(e) =>
                      setDraft((d) => ({ ...d, closesAt: e.target.value }))
                    }
                  />
                )}
              </Field>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <Field
                label={t("assignments.duration")}
                hint={t("assignments.durationHint")}
              >
                {(id) => (
                  <Select
                    id={id}
                    value={draft.durationMinutes}
                    onChange={(e) =>
                      setDraft((d) => ({
                        ...d,
                        durationMinutes: Number(e.target.value),
                      }))
                    }
                  >
                    {DURATIONS.map((minutes) => (
                      <option key={minutes} value={minutes}>
                        {t("assignments.minutes", { count: minutes })}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
              <Field
                label={t("assignments.attempts")}
                {...(draft.maxAttempts > 1
                  ? { hint: t("assignments.attemptsHint") }
                  : {})}
              >
                {(id) => (
                  <Select
                    id={id}
                    value={draft.maxAttempts}
                    onChange={(e) =>
                      setDraft((d) => ({
                        ...d,
                        maxAttempts: Number(e.target.value),
                      }))
                    }
                  >
                    {ATTEMPTS.map((count) => (
                      <option key={count} value={count}>
                        {t("assignments.attemptCount", { count })}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("assignments.step4")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2.5 pt-1">
            <Toggle
              label={t("assignments.shuffleQuestions")}
              checked={draft.shuffleQuestions}
              onChange={(v) => setDraft((d) => ({ ...d, shuffleQuestions: v }))}
            />
            <Toggle
              label={t("assignments.shuffleOptions")}
              checked={draft.shuffleOptions}
              onChange={(v) => setDraft((d) => ({ ...d, shuffleOptions: v }))}
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("assignments.step5")}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-2.5 pt-1">
            <Toggle
              label={t("assignments.showScore")}
              checked={draft.review.showScore}
              onChange={(v) =>
                setDraft((d) => ({ ...d, review: { ...d.review, showScore: v } }))
              }
            />
            <Toggle
              label={t("assignments.showCorrectAnswers")}
              checked={draft.review.showCorrectAnswers}
              onChange={(v) =>
                setDraft((d) => ({
                  ...d,
                  review: { ...d.review, showCorrectAnswers: v },
                }))
              }
            />
            <Toggle
              label={t("assignments.showExplanations")}
              checked={draft.review.showExplanations}
              onChange={(v) =>
                setDraft((d) => ({
                  ...d,
                  review: { ...d.review, showExplanations: v },
                }))
              }
            />
            <p className="text-muted-foreground text-xs leading-relaxed">
              {t("assignments.reuseHint")}
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t("assignments.step6")}</CardTitle>
            <CardDescription>{t("assignments.integrityDefaults")}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 pt-1">
            <Toggle
              label={t("assignments.requireFullscreen")}
              checked={draft.integrity.requireFullscreen}
              onChange={(v) =>
                setDraft((d) => ({
                  ...d,
                  integrity: { ...d.integrity, requireFullscreen: v },
                }))
              }
            />
            <Toggle
              label={t("assignments.blockCopyPaste")}
              checked={draft.integrity.blockCopyPaste}
              onChange={(v) =>
                setDraft((d) => ({
                  ...d,
                  integrity: { ...d.integrity, blockCopyPaste: v },
                }))
              }
            />
            <div className="grid grid-cols-2 gap-3 pt-1">
              <Field label={t("assignments.maxFocusLoss")}>
                {(id) => (
                  <Select
                    id={id}
                    value={draft.integrity.maxFocusLoss}
                    onChange={(e) =>
                      setDraft((d) => ({
                        ...d,
                        integrity: {
                          ...d.integrity,
                          maxFocusLoss: Number(e.target.value),
                        },
                      }))
                    }
                  >
                    {FOCUS_LIMITS.map((count) => (
                      <option key={count} value={count}>
                        {count === 0
                          ? t("assignments.unlimited")
                          : t("assignments.timesAway", { count })}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
              <Field label={t("assignments.onLimitExceeded")}>
                {(id) => (
                  <Select
                    id={id}
                    disabled={draft.integrity.maxFocusLoss === 0}
                    value={draft.integrity.onLimitExceeded}
                    onChange={(e) =>
                      setDraft((d) => ({
                        ...d,
                        integrity: {
                          ...d.integrity,
                          onLimitExceeded: e.target.value as "warn" | "flag",
                        },
                      }))
                    }
                  >
                    <option value="warn">{t("assignments.actionWarn")}</option>
                    <option value="flag">{t("assignments.actionFlag")}</option>
                  </Select>
                )}
              </Field>
            </div>
            <div className="bg-muted/40 flex items-start gap-2 rounded-md p-2.5">
              <Info
                className="text-muted-foreground mt-0.5 size-4 shrink-0"
                aria-hidden="true"
              />
              <p className="text-xs leading-relaxed">
                {t("assignments.integrityHonesty")}
              </p>
            </div>
          </CardContent>
        </Card>
      </div>

      <PageAside label={t("assignments.summaryTitle")}>
        <Summary draft={draft} />
        <Separator />
        <StudentRulesPreview draft={draft} />

        {error === null ? null : (
          <div role="alert" className="text-destructive space-y-1 text-sm">
            <p>{error.summary}</p>
            {error.fields.map((message) => (
              <p key={message} className="text-xs">
                · {message}
              </p>
            ))}
          </div>
        )}

        <div className="space-y-2">
          <Button
            className="w-full"
            disabled={!ready || save.isPending}
            onClick={() => {
              setError(null);
              save.mutate(false);
            }}
          >
            {save.isPending
              ? t("common.loading")
              : t(published ? "assignments.saveChanges" : "assignments.assign")}
          </Button>
          {/* A draft needs only the test. */}
          {published ? null : (
            <Button
              variant="outline"
              className="w-full"
              disabled={!savable || save.isPending}
              onClick={() => {
                setError(null);
                save.mutate(true);
              }}
            >
              {t("assignments.saveDraft")}
            </Button>
          )}
        </div>
        {ready ? null : (
          <p className="text-muted-foreground text-xs">
            {savable ? t("assignments.needTargets") : t("assignments.readyHint")}
          </p>
        )}
      </PageAside>

      <TestVersionPicker
        open={picking}
        onOpenChange={setPicking}
        onPick={(picked) => setDraft((d) => ({ ...d, picked }))}
      />
    </>
  );
}

function toBody(draft: Draft): Omit<AssignmentInput, "draft"> {
  const picked = draft.picked;
  if (!picked) throw new Error("no version picked");
  return {
    testVersionId: picked.version.id,
    targets: {
      classIds: draft.classes.map((c) => c.id),
      studentIds: draft.students.map((s) => s.id),
    },
    window: {
      opensAt: fromDateTimeInput(draft.opensAt).toISOString(),
      closesAt: fromDateTimeInput(draft.closesAt).toISOString(),
    },
    durationMinutes: draft.durationMinutes,
    maxAttempts: draft.maxAttempts,
    shuffleQuestions: draft.shuffleQuestions,
    shuffleOptions: draft.shuffleOptions,
    review: draft.review,
    integrity: draft.integrity,
  };
}

function fromAssignment(a: Assignment, versions: TestVersion[]): Draft {
  const version = versions.find((v) => v.id === a.testVersionId);
  return {
    picked: version ? { testId: a.testId, testTitle: a.testTitle, version } : null,
    classes: a.targets.classes.map((c) => ({
      id: c.id,
      label: c.name,
      hint: String(c.studentCount),
    })),
    students: a.targets.students.map((s) => ({ id: s.id, label: s.name })),
    opensAt: toDateTimeInput(a.window.opensAt),
    closesAt: toDateTimeInput(a.window.closesAt),
    durationMinutes: a.durationMinutes,
    maxAttempts: a.maxAttempts,
    shuffleQuestions: a.shuffleQuestions,
    shuffleOptions: a.shuffleOptions,
    review: a.review,
    integrity: {
      ...a.integrity,
      onLimitExceeded: a.integrity.onLimitExceeded === "warn" ? "warn" : "flag",
    },
  };
}

function Summary({ draft }: { draft: Draft }) {
  const { t } = useTranslation();

  const upperBound =
    draft.classes.reduce((sum, c) => sum + Number(c.hint ?? 0), 0) +
    draft.students.length;
  const manual = draft.picked?.version.manualCount ?? 0;
  const days = windowDays(draft.opensAt, draft.closesAt);

  return (
    <div>
      <p className="text-muted-foreground mb-3 text-xs font-medium tracking-wide uppercase">
        {t("assignments.summaryTitle")}
      </p>
      <dl className="space-y-2 text-sm">
        <Line
          label={t("assignments.studentsLabel")}
          value={t("assignments.upTo", { count: upperBound })}
        />
        <Line
          label={t("assignments.windowLabel")}
          value={t("assignments.days", { count: days })}
        />
        <Line
          label={t("assignments.durationLabel")}
          value={t("assignments.minutes", { count: draft.durationMinutes })}
        />
        <Line
          label={t("assignments.attemptsLabel")}
          value={String(draft.maxAttempts)}
        />
        <Line
          label={t("assignments.manualLabel")}
          value={t("assignments.perAttempt", { count: manual })}
        />
      </dl>

      {manual > 0 && upperBound > 0 ? (
        <div className="bg-warning/15 mt-3 flex items-start gap-2 rounded-md p-2.5">
          <SquarePen
            className="text-warning-ink mt-0.5 size-4 shrink-0"
            aria-hidden="true"
          />
          <p className="text-xs leading-relaxed">
            {t("assignments.gradingCost", {
              students: upperBound,
              each: manual,
              total: upperBound * manual,
            })}
          </p>
        </div>
      ) : null}
    </div>
  );
}

function Line({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between gap-3">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="font-medium tabular-nums">{value}</dd>
    </div>
  );
}

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: (id: string) => ReactNode;
}) {
  const id = useId();
  return (
    <div>
      <Label htmlFor={id}>{label}</Label>
      <div className="mt-1.5">{children(id)}</div>
      {hint === undefined ? null : (
        <p className="text-muted-foreground mt-1 text-xs">{hint}</p>
      )}
    </div>
  );
}

function Toggle({
  label,
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <label className="flex items-center justify-between gap-3 text-sm">
      {label}
      <Switch checked={checked} onCheckedChange={onChange} />
    </label>
  );
}

function windowDays(opensAt: string, closesAt: string): number {
  const ms =
    fromDateTimeInput(closesAt).getTime() - fromDateTimeInput(opensAt).getTime();
  return Math.max(0, Math.round(ms / 86_400_000));
}
