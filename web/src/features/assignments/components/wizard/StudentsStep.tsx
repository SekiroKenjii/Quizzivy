import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { useQueryClient } from "@tanstack/react-query";
import { Check, Info, UserPlus, UserRound, Users, X } from "lucide-react";
import { ListSkeleton, LoadError } from "@/components/shared/ListState";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { Avatar } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import type { Token } from "@/features/assignments/components/TokenField";
import { AddStudentsDialog } from "@/features/assignments/components/wizard/AddStudentsDialog";
import type { TargetRoster } from "@/features/assignments/useTargetRoster";
import { fetchClasses } from "@/features/classes/api";
import { useLazyList } from "@/hooks/useLazyList";
import { cn } from "@/lib/utils";

const PAGE = 100;

/** RosterState is the part of the roster query the step reads. */
export interface RosterState {
  data: TargetRoster | undefined;
  isPending: boolean;
  isError: boolean;
  refetch: () => unknown;
}

/**
 * StudentsStep is the wizard's "Who takes it?": the teacher's active classes
 * as cards to tick, the students added one by one with the class that
 * already includes any of them, the total the assignment reaches, and the
 * dialog that adds students one by one.
 */
export function StudentsStep({
  classes,
  students,
  roster,
  onToggleClass,
  onStudentsChange,
}: Readonly<{
  classes: readonly Token[];
  students: readonly Token[];
  roster: RosterState;
  onToggleClass: (token: Token) => void;
  onStudentsChange: (students: Token[]) => void;
}>) {
  const { t } = useTranslation();
  const titleId = useId();
  const [adding, setAdding] = useState(false);
  const classIds = classes.map((klass) => klass.id);

  return (
    <div className="flex flex-col gap-3">
      <h2 id={titleId} className="text-md font-semibold">
        {t("assignments.wizard.studentsTitle")}
      </h2>
      <ClassCards labelledBy={titleId} picked={classes} onToggle={onToggleClass} />
      {students.length > 0 && (
        <IndividualsPanel
          classes={classes}
          students={students}
          classesOf={roster.data?.classesOf ?? {}}
          onChange={onStudentsChange}
        />
      )}
      <div className="bg-muted flex items-center gap-2 rounded-[10px] px-3.5 py-2.5 text-sm">
        <Users aria-hidden="true" className="text-muted-fg size-3.75 flex-none" />
        <TotalLine
          roster={roster}
          empty={classes.length === 0 && students.length === 0}
        />
      </div>
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="text-muted-fg hover:text-fg self-start"
        onClick={() => setAdding(true)}
      >
        <UserPlus aria-hidden="true" className="size-3.5" />
        {t("assignments.wizard.addIndividuals")}
      </Button>
      <AddStudentsDialog
        open={adding}
        onOpenChange={setAdding}
        selected={students}
        classIds={classIds}
        onDone={onStudentsChange}
      />
    </div>
  );
}

function ClassCards({
  labelledBy,
  picked,
  onToggle,
}: Readonly<{
  labelledBy: string;
  picked: readonly Token[];
  onToggle: (token: Token) => void;
}>) {
  const { t } = useTranslation();
  const client = useQueryClient();
  const queryKey = ["admin-classes", "wizard"];
  const list = useLazyList({
    queryKey,
    fetchPage: (page, signal) => fetchClasses({ page, limit: PAGE }, signal),
  });
  const on = new Set(picked.map((klass) => klass.id));
  const listed = new Set(list.items.map((klass) => klass.id));
  const cards: Token[] = [
    ...picked.filter((klass) => !listed.has(klass.id)),
    ...list.items.map((klass) => ({
      id: klass.id,
      label: klass.name,
      hint: String(klass.studentCount),
    })),
  ];

  if (list.isPending) return <ListSkeleton rows={3} />;
  if (list.isError) {
    return (
      <LoadError error={null} onRetry={() => void client.refetchQueries({ queryKey })}>
        {t("assignments.wizard.classesFailed")}
      </LoadError>
    );
  }
  if (cards.length === 0) {
    return (
      <p className="text-muted-fg rounded-[10px] border border-dashed px-3.5 py-6 text-center text-sm">
        {t("assignments.wizard.classesEmpty")}
      </p>
    );
  }
  return (
    <div role="group" aria-labelledby={labelledBy} className="flex flex-col gap-3">
      {cards.map((klass) => {
        const checked = on.has(klass.id);
        return (
          <button
            key={klass.id}
            type="button"
            role="checkbox"
            aria-checked={checked}
            onClick={() => onToggle(klass)}
            className={cn(
              "flex items-center gap-3 rounded-[10px] border px-3.5 py-3 text-left",
              checked ? "border-primary bg-muted" : "border-border bg-card",
            )}
          >
            <span
              aria-hidden="true"
              className={cn(
                "text-primary-fg grid size-4.5 flex-none place-items-center rounded-[5px] border",
                checked ? "border-primary bg-primary" : "border-ring bg-card",
              )}
            >
              {checked && <Check className="size-3" strokeWidth={2.5} />}
            </span>
            <span className="min-w-0 flex-1">
              <span className="block text-base leading-[1.3] font-medium [overflow-wrap:anywhere]">
                {klass.label}
              </span>
              {klass.hint !== undefined && (
                <span className="text-muted-fg text-meta block leading-[1.3]">
                  {t("assignments.wizard.classStudents", { count: Number(klass.hint) })}
                </span>
              )}
            </span>
          </button>
        );
      })}
      <LoadMoreSentinel
        active={list.hasMore}
        loading={list.loadingMore}
        onVisible={list.loadMore}
      />
    </div>
  );
}

function IndividualsPanel({
  classes,
  students,
  classesOf,
  onChange,
}: Readonly<{
  classes: readonly Token[];
  students: readonly Token[];
  classesOf: Readonly<Record<string, string[]>>;
  onChange: (students: Token[]) => void;
}>) {
  const { t } = useTranslation();
  const headingId = useId();
  const viaClass = (id: string) => {
    const through = new Set(classesOf[id] ?? []);
    return classes.find((klass) => through.has(klass.id))?.label;
  };
  return (
    <section
      aria-labelledby={headingId}
      className="border-border flex flex-col overflow-hidden rounded-[10px] border"
    >
      <div className="bg-sidebar flex flex-wrap items-center justify-between gap-2.5 px-3.5 py-2.5">
        <h3 id={headingId} className="flex items-center gap-2 text-sm font-semibold">
          <UserRound aria-hidden="true" className="text-muted-fg size-3.75" />
          {t("assignments.wizard.individuals")}
          <span className="bg-primary text-primary-fg text-2xs inline-flex h-5 min-w-5 items-center justify-center rounded-full px-1.5 font-semibold">
            {students.length}
          </span>
        </h3>
        <button
          type="button"
          onClick={() => onChange([])}
          className="text-muted-fg hover:text-fg text-meta rounded-sm"
        >
          {t("assignments.wizard.removeAll")}
        </button>
      </div>
      <ul className="m-0 list-none p-0">
        {students.map((student) => {
          const via = viaClass(student.id);
          return (
            <li
              key={student.id}
              className={cn(
                "border-border flex items-center gap-3 border-t px-3.5 py-2.25",
                via !== undefined && "opacity-75",
              )}
            >
              <Avatar name={student.label} size="30" />
              <span className="min-w-0 flex-1">
                <span className="text-ui block truncate font-medium">
                  {student.label}
                </span>
                <span
                  className={cn(
                    "flex items-center gap-1.25 text-xs",
                    via === undefined ? "text-muted-fg" : "text-warning-ink",
                  )}
                >
                  {via !== undefined && <Info aria-hidden="true" className="size-3" />}
                  {via !== undefined
                    ? t("assignments.wizard.viaClass", { class: via })
                    : (student.hint ?? t("assignments.wizard.notInClass"))}
                </span>
              </span>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t("assignments.wizard.removeStudent", {
                  name: student.label,
                })}
                className="text-muted-fg hover:text-fg"
                onClick={() => onChange(students.filter((s) => s.id !== student.id))}
              >
                <X aria-hidden="true" className="size-3.75" />
              </Button>
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function TotalLine({
  roster,
  empty,
}: Readonly<{ roster: RosterState; empty: boolean }>) {
  const { t } = useTranslation();
  if (empty)
    return <span className="flex-1">{t("assignments.wizard.total.none")}</span>;
  if (roster.isError) {
    return (
      <button
        type="button"
        onClick={() => void roster.refetch()}
        className="text-danger flex-1 rounded-sm text-left"
      >
        {t("assignments.rosterFailed")}
      </button>
    );
  }
  if (roster.isPending || roster.data === undefined) {
    return (
      <span role="status" className="text-muted-fg flex-1">
        {t("assignments.rosterLoading")}
      </span>
    );
  }
  return (
    <span role="status" className="flex-1">
      {totalText(roster.data, t)}
    </span>
  );
}

function totalText(
  roster: TargetRoster,
  t: ReturnType<typeof useTranslation>["t"],
): string {
  const extra = roster.total - roster.fromClasses;
  if (roster.total === 0) return t("assignments.wizard.total.none");
  if (roster.fromClasses > 0 && extra > 0) {
    return t("assignments.wizard.total.mixed", {
      total: roster.total,
      classes: roster.fromClasses,
      extra,
    });
  }
  if (extra > 0) return t("assignments.wizard.total.extra", { count: roster.total });
  return t("assignments.wizard.total.classes", { count: roster.total });
}
