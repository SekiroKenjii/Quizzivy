import { useId, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  CalendarClock,
  Check,
  Eye,
  FileText,
  Lock,
  MessageSquareText,
  Minus,
  Pencil,
  ShieldCheck,
  type LucideIcon,
} from "lucide-react";
import { Callout } from "@/components/shared/Callout";
import {
  DialogShell,
  DialogShellBody,
  DialogShellFooter,
  DialogShellHeader,
} from "@/components/shared/form/DialogShell";
import { ChipsField } from "@/components/shared/form/fields/ChipsField";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/sonner";
import { useCan } from "@/features/auth/permissions";
import { fetchClasses } from "@/features/classes/api";
import type { TestVersion } from "@/features/tests/api";
import { ApiError } from "@/lib/api/errors";
import { formatMoment } from "@/lib/i18n/datetime";
import { useLocale } from "@/lib/i18n/useLocale";
import { cn } from "@/lib/utils";
import { updateAssignment, type Assignment } from "../api";
import { draftOf, type AssignmentDraft } from "../draft";
import { settingsBody, type SettingsGroup } from "../settingsBody";
import type { statusAt } from "../status";
import { windowIsValid, type RulesPatch } from "../wizardValues";
import {
  IntegrityRules,
  OrderRules,
  ResultRules,
  StudentNoteField,
} from "./wizard/RulesStep";
import { TimingRules, WindowFields, type SchedulePatch } from "./wizard/ScheduleStep";

type Status = ReturnType<typeof statusAt>;
type Pill = { on: boolean; text: string };
type Row = { label: string; value: ReactNode; sub?: string | null; pill?: Pill };

const GROUPS: readonly { group: SettingsGroup; icon: LucideIcon }[] = [
  { group: "timing", icon: FileText },
  { group: "window", icon: CalendarClock },
  { group: "integrity", icon: ShieldCheck },
  { group: "results", icon: Eye },
  { group: "note", icon: MessageSquareText },
];

/**
 * SettingsGroups is the assignment's Settings tab: Test & timing, Window,
 * Integrity, Results and the note to students (DG-71), each a card whose Edit
 * opens the wizard's own controls for that group (DG-162). Test & timing is
 * locked while the assignment is open (DG-65): its card shows a Lock pill in
 * place of Edit. Editing needs `teaching.assignments.write`.
 */
export function SettingsGroups({
  a,
  version,
  status,
}: Readonly<{ a: Assignment; version: TestVersion | undefined; status: Status }>) {
  const { t } = useTranslation();
  const write = useCan("teaching.assignments.write");
  const [editing, setEditing] = useState<SettingsGroup | null>(null);
  const rows = useRows(a, version);
  return (
    <>
      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,23.75rem),1fr))] items-start gap-3">
        {GROUPS.map(({ group, icon }) => (
          <SettingsCard
            key={group}
            icon={icon}
            title={t(`assignmentDetail.settings.${group}.title`)}
            hint={t(`assignmentDetail.settings.${group}.hint`)}
            locked={group === "timing" && status === "open"}
            onEdit={write ? () => setEditing(group) : null}
            rows={rows[group]}
          />
        ))}
      </div>
      {editing !== null && (
        <SettingsEditDialog
          key={editing}
          a={a}
          group={editing}
          status={status}
          onClose={() => setEditing(null)}
        />
      )}
    </>
  );
}

function SettingsCard({
  icon: Icon,
  title,
  hint,
  locked,
  onEdit,
  rows,
}: Readonly<{
  icon: LucideIcon;
  title: string;
  hint: string;
  locked: boolean;
  onEdit: (() => void) | null;
  rows: readonly Row[];
}>) {
  const { t } = useTranslation();
  return (
    <section
      aria-label={title}
      className="bg-card shadow-card min-w-0 overflow-hidden rounded-xl border"
    >
      <div className="flex items-center gap-2.5 border-b py-3 pr-3 pl-4">
        <span className="bg-muted grid size-7 flex-none place-items-center rounded-[0.4375rem]">
          <Icon aria-hidden="true" className="size-3.75" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="block text-base font-semibold">{title}</span>
          <span className="text-muted-fg text-meta block">{hint}</span>
        </span>
        {locked ? (
          <span
            title={t("assignmentDetail.settings.locked")}
            className="text-muted-fg inline-flex flex-none items-center gap-1.25 text-xs whitespace-nowrap"
          >
            <Lock aria-hidden="true" className="size-3.25" />
            {t("assignmentDetail.settings.lockedPill")}
            <span className="sr-only">: {t("assignmentDetail.settings.locked")}</span>
          </span>
        ) : (
          onEdit !== null && (
            <Button
              variant="outline"
              size="sm"
              className="h-7.5 flex-none px-2.5"
              aria-label={t("assignmentDetail.settings.editLabel", { group: title })}
              onClick={onEdit}
            >
              <Pencil aria-hidden="true" />
              {t("assignmentDetail.settings.edit")}
            </Button>
          )
        )}
      </div>
      <dl>
        {rows.map((row, index) => (
          <div
            key={row.label}
            className={cn(
              "grid grid-cols-[minmax(6.875rem,9.375rem)_minmax(0,1fr)] items-baseline gap-x-4 gap-y-1 px-4 py-3",
              index > 0 && "border-t",
            )}
          >
            <dt className="text-muted-fg text-sm">{row.label}</dt>
            <dd className="flex min-w-0 flex-col gap-0.5">
              <span className="text-ui flex flex-wrap items-center gap-2 font-medium [overflow-wrap:anywhere]">
                {row.pill && <SettingPill pill={row.pill} />}
                {row.value}
              </span>
              {row.sub && <span className="text-muted-fg text-meta">{row.sub}</span>}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

function AssignedTo({ a }: Readonly<{ a: Assignment }>) {
  const { t } = useTranslation();
  const { classes, students } = a.targets;
  if (classes.length === 0 && students.length === 0)
    return <>{t("assignments.detail.noTargets")}</>;
  return (
    <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
      {classes.map((c) => (
        <Link
          key={c.id}
          to={`/teacher/classes/${c.id}`}
          className="underline-offset-2 hover:underline"
        >
          {c.name}
        </Link>
      ))}
      {students.length > 0 && (
        <span className="text-muted-fg font-normal">
          {t("assignments.detail.extraStudents", { count: students.length })}
        </span>
      )}
    </span>
  );
}

function SettingPill({ pill }: Readonly<{ pill: Pill }>) {
  const Icon = pill.on ? Check : Minus;
  return (
    <span
      className={cn(
        "inline-flex h-5.5 items-center gap-1.25 rounded-full px-2 text-xs font-medium",
        pill.on ? "bg-success-soft text-success-ink" : "bg-muted text-muted-fg",
      )}
    >
      <Icon aria-hidden="true" className="size-3" />
      {pill.text}
    </span>
  );
}

function useRows(
  a: Assignment,
  version: TestVersion | undefined,
): Record<SettingsGroup, Row[]> {
  const { t } = useTranslation();
  const locale = useLocale();
  const on = (value: boolean, yes: string, no: string): Pill => ({
    on: value,
    text: value ? yes : no,
  });
  const shown = t("assignmentDetail.settings.shown");
  const hidden = t("assignmentDetail.settings.hidden");
  const release = t(
    a.review.release === "after_close"
      ? "assignments.wizard.releaseAfterClose"
      : "assignments.wizard.releaseOnSubmit",
  );
  const closedEarly =
    a.window.closedAt != null &&
    new Date(a.window.closedAt).getTime() < new Date(a.window.closesAt).getTime();
  const { integrity, review } = a;
  return {
    timing: [
      {
        label: t("assignments.detail.test"),
        value: a.testTitle,
        sub:
          version === undefined
            ? t("assignments.detail.versionOnly", { version: a.testVersion })
            : t("assignments.detail.versionMeta", {
                questions: version.questionCount,
                points: version.totalPoints,
                audio: version.audioCount,
                manual: version.manualCount,
                version: version.version,
              }),
      },
      {
        label: t("assignments.wizard.timeLimit"),
        value: t("assignments.minutes", { count: a.durationMinutes }),
        sub: t("assignmentDetail.settings.timing.timerSub"),
      },
      {
        label: t("assignments.detail.attempts"),
        value:
          a.maxAttempts === 1
            ? t("assignments.detail.attemptsOne")
            : t("assignments.detail.attemptsMany", { count: a.maxAttempts }),
      },
      {
        label: t("assignments.wizard.order"),
        value: [
          a.shuffleQuestions
            ? t("assignmentDetail.settings.timing.questionsShuffled")
            : t("assignmentDetail.settings.timing.questionsInOrder"),
          a.shuffleOptions
            ? t("assignmentDetail.settings.timing.optionsShuffled")
            : t("assignmentDetail.settings.timing.optionsInOrder"),
        ].join(" · "),
      },
    ],
    window: [
      {
        label: t("assignments.wizard.opens"),
        value: formatMoment(a.window.opensAt, locale),
      },
      closedEarly
        ? {
            label: t("assignments.wizard.closes"),
            value: formatMoment(a.window.closedAt ?? a.window.closesAt, locale),
            sub: t("assignmentDetail.settings.window.closedEarly", {
              planned: formatMoment(a.window.closesAt, locale),
            }),
          }
        : {
            label: t("assignments.wizard.closes"),
            value: formatMoment(a.window.closesAt, locale),
            sub: t("assignmentDetail.settings.window.closesSub"),
          },
      {
        label: t("assignmentDetail.settings.window.assignedTo"),
        value: <AssignedTo a={a} />,
        sub: t("assignmentDetail.settings.window.students", {
          count: a.targetCount ?? 0,
        }),
      },
    ],
    integrity: [
      {
        label: t("assignments.detail.fullscreen"),
        value: "",
        pill: on(
          integrity.requireFullscreen,
          t("assignments.detail.required"),
          t("assignments.detail.notRequired"),
        ),
      },
      {
        label: t("assignments.wizard.leaving"),
        value:
          integrity.maxFocusLoss === 0
            ? t("assignments.detail.focusUnlimited")
            : t(`assignments.detail.focusLimit.${integrity.onLimitExceeded}`, {
                count: Math.max(0, integrity.maxFocusLoss),
              }),
      },
      {
        label: t("assignments.detail.copyPaste"),
        value: "",
        pill: on(
          integrity.blockCopyPaste,
          t("assignments.detail.blocked"),
          t("assignments.detail.allowed"),
        ),
      },
      {
        label: t("assignments.wizard.away"),
        value: t("assignments.wizard.seconds", {
          count: Math.round(integrity.minAwayMs / 1000),
        }),
      },
    ],
    results: [
      {
        label: t("assignments.wizard.score"),
        value: review.showScore ? release : "",
        pill: on(review.showScore, shown, hidden),
      },
      {
        label: t("assignments.wizard.correct"),
        value: review.showCorrectAnswers ? release : "",
        pill: on(review.showCorrectAnswers, shown, hidden),
      },
      {
        label: t("assignments.wizard.explanations"),
        value: "",
        pill: on(review.showExplanations, shown, hidden),
      },
      {
        label: t("assignments.wizard.classAverage"),
        value: "",
        pill: on(review.showClassAverage ?? false, shown, hidden),
      },
    ],
    note: [
      {
        label: t("assignmentDetail.settings.note.label"),
        value: a.studentNote ?? t("assignmentDetail.settings.note.none"),
      },
    ],
  };
}

function SettingsEditDialog({
  a,
  group,
  status,
  onClose,
}: Readonly<{
  a: Assignment;
  group: SettingsGroup;
  status: Status;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const [draft, setDraft] = useState<AssignmentDraft>(() => draftOf(a, []));
  const [error, setError] = useState<string | null>(null);
  const title = t(`assignmentDetail.settings.${group}.title`);
  const save = useMutation({
    mutationFn: () => updateAssignment(a.id, settingsBody(group, a, draft, status)),
    onSuccess: async (saved) => {
      client.setQueryData(["admin-assignment", a.id], saved);
      toast(t("assignmentDetail.settings.saved", { group: title }));
      onClose();
      await client.invalidateQueries({ queryKey: ["admin-assignments"] });
    },
    onError: (cause) =>
      setError(cause instanceof ApiError ? cause.message : t("common.actionFailed")),
  });
  const patch = (change: RulesPatch | SchedulePatch) =>
    setDraft((current) => merge(current, change));
  const invalid = group === "window" && !windowIsValid(draft);
  return (
    <DialogShell
      open
      width={560}
      onOpenChange={(open) => {
        if (!open && !save.isPending) onClose();
      }}
    >
      <DialogShellHeader
        title={t("assignmentDetail.settings.editTitle", { group: title })}
        description={t(`assignmentDetail.settings.${group}.hint`)}
        icon={Pencil}
      />
      <DialogShellBody>
        <GroupControls
          group={group}
          a={a}
          draft={draft}
          onChange={patch}
          onDraft={setDraft}
        />
        {error !== null && (
          <Callout tone="danger" announce>
            {error}
          </Callout>
        )}
      </DialogShellBody>
      <DialogShellFooter>
        <Button variant="outline" disabled={save.isPending} onClick={onClose}>
          {t("common.cancel")}
        </Button>
        <Button
          disabled={save.isPending || invalid}
          onClick={() => {
            setError(null);
            save.mutate();
          }}
        >
          {t("assignmentDetail.settings.save")}
        </Button>
      </DialogShellFooter>
    </DialogShell>
  );
}

function merge(
  current: AssignmentDraft,
  change: RulesPatch | SchedulePatch,
): AssignmentDraft {
  const { review, integrity, ...rest } = change as RulesPatch;
  return {
    ...current,
    ...rest,
    review: review ? { ...current.review, ...review } : current.review,
    integrity: integrity ? { ...current.integrity, ...integrity } : current.integrity,
  };
}

function GroupControls({
  group,
  a,
  draft,
  onChange,
  onDraft,
}: Readonly<{
  group: SettingsGroup;
  a: Assignment;
  draft: AssignmentDraft;
  onChange: (change: RulesPatch | SchedulePatch) => void;
  onDraft: (draft: AssignmentDraft) => void;
}>): ReactNode {
  switch (group) {
    case "timing":
      return (
        <>
          <TimingRules draft={draft} onChange={onChange} />
          <OrderRules draft={draft} onChange={onChange} />
        </>
      );
    case "window":
      return (
        <>
          <WindowFields draft={draft} onChange={onChange} />
          <ClassChips a={a} draft={draft} onDraft={onDraft} />
        </>
      );
    case "integrity":
      return <IntegrityRules draft={draft} onChange={onChange} />;
    case "results":
      return <ResultRules draft={draft} onChange={onChange} />;
    case "note":
      return <StudentNoteField draft={draft} onChange={onChange} />;
  }
}

function ClassChips({
  a,
  draft,
  onDraft,
}: Readonly<{
  a: Assignment;
  draft: AssignmentDraft;
  onDraft: (draft: AssignmentDraft) => void;
}>) {
  const { t } = useTranslation();
  const classes = useQuery({
    queryKey: ["admin-classes", "picker", { limit: 100 }],
    queryFn: ({ signal }) => fetchClasses({ limit: 100 }, signal),
    staleTime: 60_000,
  });
  const known = new Map(a.targets.classes.map((c) => [c.id, c.name]));
  for (const klass of classes.data?.items ?? []) known.set(klass.id, klass.name);
  const label = t("assignmentDetail.settings.window.assignedTo");
  const chipsId = useId();
  return (
    <div className="flex flex-col gap-1.5">
      <span className="text-meta font-medium">{label}</span>
      <ChipsField
        id={chipsId}
        label={label}
        value={draft.classes.map((c) => c.id)}
        options={[...known].map(([value, name]) => ({ value, label: name }))}
        onChange={(ids) =>
          onDraft({
            ...draft,
            classes: ids.map((id) => ({ id, label: known.get(id) ?? id })),
          })
        }
      />
      {a.targets.students.length > 0 && (
        <span className="text-muted-fg text-meta">
          {t("assignmentDetail.settings.window.namedStudents", {
            count: a.targets.students.length,
          })}
        </span>
      )}
    </div>
  );
}
