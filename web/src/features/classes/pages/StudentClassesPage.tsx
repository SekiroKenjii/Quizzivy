import { useTranslation } from "react-i18next";
import { Link, useNavigate } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { CircleCheck, ClipboardList, Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { EmptyState, ListSkeleton, QueryStates } from "@/components/shared/ListState";
import { PageAside } from "@/components/shared/PageAside";
import { PanelLabel } from "@/components/shared/PanelLabel";
import { listMyAssignments } from "@/features/assignments/api";
import { JoinCodeForm } from "@/features/join/components/JoinCodeForm";
import { useMediaQuery } from "@/hooks/useMediaQuery";
import { formatDate, shortDate } from "@/lib/i18n/datetime";
import { fetchMyClasses, type MyClass } from "../api";

/**
 * S-10's classes list: what the student is in, and the way into another. From
 * 1024px it is S-17: a grid of class cards that also say what each class is
 * doing, and S-01's code field in F-11's panel.
 */
export default function StudentClassesPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const wide = useMediaQuery("(min-width: 1024px)");
  const classes = useQuery({
    queryKey: ["my-classes"],
    queryFn: ({ signal }) => fetchMyClasses(signal),
  });
  // The counts are S-03's lists, filtered to one class: nothing new is fetched.
  const assignments = useQuery({
    queryKey: ["my-assignments"],
    queryFn: ({ signal }) => listMyAssignments(signal),
    enabled: wide,
  });

  const counts = (id: string) => ({
    open: assignments.data?.dueNow.filter((c) => c.classId === id).length ?? null,
    submitted:
      assignments.data?.completed.filter((c) => c.classId === id).length ?? null,
  });

  return (
    <div className="space-y-3 lg:space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-semibold tracking-tight lg:text-xl">
            {t("student.myClasses")}
          </h1>
          {wide && classes.data !== undefined && classes.data.items.length > 0 && (
            <p className="text-muted-foreground mt-1 text-sm">
              {t("student.inClasses", { count: classes.data.items.length })}
            </p>
          )}
        </div>
        {!wide && (
          <Button asChild variant="outline" size="sm">
            <Link to="/join">
              <Plus aria-hidden="true" />
              {t("student.joinClass")}
            </Link>
          </Button>
        )}
      </div>

      <QueryStates
        query={classes}
        skeleton={<ListSkeleton rows={3} />}
        failed={t("student.loadFailed")}
      >
        {(data) =>
          data.items.length === 0 ? (
            <EmptyState
              action={
                <Button asChild size="sm">
                  <Link to="/join">{t("student.joinClass")}</Link>
                </Button>
              }
            >
              {t("student.noClasses")}
            </EmptyState>
          ) : (
            <div className="space-y-3 lg:grid lg:grid-cols-2 lg:items-start lg:gap-4 lg:space-y-0">
              {data.items.map((c) =>
                wide ? (
                  <ClassCard key={c.id} klass={c} {...counts(c.id)} />
                ) : (
                  <Card key={c.id} className="gap-0 p-4">
                    <p className="text-sm font-medium">{c.name}</p>
                    {/* S-10's second line: who teaches it, and since when. */}
                    <p className="text-muted-foreground mt-1 text-xs">
                      <Since klass={c} />
                    </p>
                    {c.description && (
                      <p className="text-muted-foreground mt-1 text-xs">
                        {c.description}
                      </p>
                    )}
                  </Card>
                ),
              )}
            </div>
          )
        }
      </QueryStates>

      {wide && (
        <PageAside label={t("student.joinClass")}>
          <div>
            <PanelLabel>{t("student.joinClass")}</PanelLabel>
            <p className="text-muted-foreground text-sm leading-relaxed">
              {t("join.subtitle")}
            </p>
            <div className="mt-3">
              {/* The confirm step is mandatory (§6.2): the panel only hands the code on. */}
              <JoinCodeForm
                id="panel-join-code"
                size="default"
                onContinue={(code) => void navigate(`/join/${code}/confirm`)}
              />
            </div>
          </div>
          <Separator />
          <p className="text-muted-foreground text-xs leading-relaxed">
            {t("join.panelNote")}
          </p>
        </PageAside>
      )}
    </div>
  );
}

/** S-17's class card: S-10's two lines, the description, and what the class is doing. */
function ClassCard({
  klass: c,
  open,
  submitted,
}: Readonly<{ klass: MyClass; open: number | null; submitted: number | null }>) {
  const { t } = useTranslation();
  return (
    <Card className="gap-0 p-5">
      <p className="text-base leading-snug font-semibold">{c.name}</p>
      <p className="text-muted-foreground mt-1 text-xs">
        <Since klass={c} withYear />
      </p>
      {c.description && (
        <p className="text-muted-foreground mt-3 text-sm leading-relaxed">
          {c.description}
        </p>
      )}
      {open !== null && submitted !== null && (
        <div className="text-muted-foreground mt-3 flex items-center gap-4 text-xs">
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
    </Card>
  );
}

function Since({
  klass: c,
  withYear = false,
}: Readonly<{ klass: MyClass; withYear?: boolean }>) {
  const { t } = useTranslation();
  const date = withYear ? formatDate(c.joinedAt) : shortDate(c.joinedAt);
  return c.teacherName === null
    ? t("student.joinedOn", { date })
    : t("student.taughtBySince", { teacher: c.teacherName, date });
}
