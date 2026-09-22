import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { CircleCheck, ClipboardList, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  EmptyState,
  ListSkeleton,
  QueryStates,
  LoadError,
} from "@/components/shared/ListState";
import { listMyAssignments } from "@/features/assignments/api";
import { shortDate } from "@/lib/i18n/datetime";
import { fetchMyClasses, type MyClass } from "../api";

/** StudentClassesPage links each class to its assignments and keeps enrolment in one entry point. */
export default function StudentClassesPage() {
  const { t } = useTranslation();
  const classes = useQuery({
    queryKey: ["my-classes"],
    queryFn: ({ signal }) => fetchMyClasses(signal),
  });
  const assignments = useQuery({
    queryKey: ["my-assignments"],
    queryFn: ({ signal }) => listMyAssignments(signal),
  });
  const counts = (id: string) => ({
    open: assignments.data?.dueNow.filter((c) => c.classId === id).length ?? null,
    submitted:
      assignments.data?.completed.filter((c) => c.classId === id).length ?? null,
  });
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">
            {t("student.myClasses")}
          </h1>
          {classes.data && classes.data.items.length > 0 && (
            <p className="text-muted-foreground mt-1 text-sm">
              {t("student.inClasses", { count: classes.data.items.length })}
            </p>
          )}
        </div>
        <Button asChild variant="outline">
          <Link to="/join">
            <Plus aria-hidden="true" />
            {t("student.joinClass")}
          </Link>
        </Button>
      </div>
      {assignments.isError && (
        <LoadError error={assignments.error} onRetry={() => void assignments.refetch()}>
          {t("student.assignmentCountsFailed")}
        </LoadError>
      )}
      <QueryStates
        query={classes}
        skeleton={<ListSkeleton rows={3} />}
        failed={t("student.loadFailed")}
      >
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState>{t("student.noClasses")}</EmptyState>
          ) : (
            <div className="grid items-start gap-4 md:grid-cols-2 2xl:grid-cols-3">
              {data.items.map((c) => (
                <ClassCard key={c.id} klass={c} {...counts(c.id)} />
              ))}
            </div>
          )
        }
      </QueryStates>
    </div>
  );
}

function ClassCard({
  klass: c,
  open,
  submitted,
}: Readonly<{ klass: MyClass; open: number | null; submitted: number | null }>) {
  const { t } = useTranslation();
  return (
    <Card className="min-w-0 gap-0 p-5">
      <p className="text-base leading-snug font-semibold">{c.name}</p>
      <p className="text-muted-foreground mt-1 text-xs">
        <Since klass={c} />
      </p>
      {c.description && (
        <p className="text-muted-foreground mt-3 text-sm leading-relaxed">
          {c.description}
        </p>
      )}
      {open !== null && submitted !== null && (
        <div className="text-muted-foreground mt-3 flex flex-wrap items-center gap-4 text-xs">
          <span className="flex items-center gap-1.5">
            <ClipboardList className="size-3.5" aria-hidden="true" />
            {open === 0
              ? t("student.classOpenNone")
              : t("student.classOpen", { count: open })}
          </span>
          <span className="flex items-center gap-1.5">
            <CircleCheck className="size-3.5" aria-hidden="true" />
            {t("student.classSubmitted", { count: submitted })}
          </span>
        </div>
      )}
      <Button asChild variant="outline" className="mt-5 w-full">
        <Link to={`/app?classId=${encodeURIComponent(c.id)}`}>
          {t("student.viewClassAssignments")}
        </Link>
      </Button>
    </Card>
  );
}

function Since({ klass: c }: Readonly<{ klass: MyClass }>) {
  const { t } = useTranslation();
  const date = shortDate(c.joinedAt);
  return c.teacherName === null
    ? t("student.joinedOn", { date })
    : t("student.taughtBySince", { teacher: c.teacherName, date });
}
