import { useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { Link, useParams } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { ClipboardList, UserPlus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DeckScale } from "@/components/ui/deck-scale";
import { Skeleton } from "@/components/ui/skeleton";
import { LoadError } from "@/components/shared/ListState";
import { StickyAside } from "@/components/shared/StickyAside";
import { useCan } from "@/features/auth/permissions";
import { fetchClass } from "@/features/classes/api";
import { AddMemberDialog } from "@/features/classes/components/AddMemberDialog";
import { ClassAssignmentsCard } from "@/features/classes/components/ClassAssignmentsCard";
import { ClassMembersCard } from "@/features/classes/components/ClassMembersCard";
import { ClassSettingsCard } from "@/features/classes/components/ClassSettingsCard";
import { JoinCodePanel } from "@/features/classes/components/JoinCodePanel";
import { JoinCodesOffCallout } from "@/features/classes/components/JoinCodesOffCallout";
import { PageHead } from "@/layouts/shell/PageHead";
import { useContentWidthAtLeast } from "@/layouts/shell/contentWidth";
import { ApiError } from "@/lib/api/errors";
import { useDisplayTimeZone } from "@/lib/i18n/datetime";
import { cn } from "@/lib/utils";

const TWO_COLUMNS = 1000;

/**
 * ClassDetailPage is one class, as the deck's Class detail draws it: back to
 * Classes, the name with its student count, Add student and Assign to class;
 * then, side by side from 1000px of content (two thirds and one third), the
 * members and the class's assignments, and the join code and the class's own
 * words, which stick as the page scrolls. Writing anything needs `teaching.classes.write`; without it the
 * page reads. A class that is missing, or another teacher's, is not found;
 * any other failure keeps the plain "Class" heading above a load error.
 */
export default function ClassDetailPage() {
  useDisplayTimeZone();
  const { t } = useTranslation();
  const { id = "" } = useParams();
  const canWrite = useCan("teaching.classes.write");
  const canAssign = useCan("teaching.assignments.write");
  const two = useContentWidthAtLeast(TWO_COLUMNS);
  const addButton = useRef<HTMLButtonElement>(null);
  const [adding, setAdding] = useState(false);
  const klass = useQuery({
    queryKey: ["teacher-class", id],
    queryFn: ({ signal }) => fetchClass(id, signal),
  });

  if (klass.isPending) return <DetailSkeleton />;
  if (klass.isError) {
    const missing = klass.error instanceof ApiError && klass.error.status === 404;
    return (
      <DeckScale className="flex w-full min-w-0 flex-col gap-4">
        <PageHead
          title={t(missing ? "classDetail.notFoundTitle" : "classDetail.crumb")}
          back={{ to: "/teacher/classes", label: t("teacherShell.nav.classes") }}
        />
        {missing ? (
          <p className="text-muted-fg text-sm">{t("classDetail.notFound")}</p>
        ) : (
          <LoadError error={klass.error} onRetry={() => void klass.refetch()}>
            {t("classDetail.loadFailed")}
          </LoadError>
        )}
      </DeckScale>
    );
  }

  const data = klass.data;
  return (
    <DeckScale className="flex w-full min-w-0 flex-col gap-4.5">
      <PageHead
        title={data.name}
        crumb
        back={{ to: "/teacher/classes", label: t("teacherShell.nav.classes") }}
        status={
          <span className="bg-muted inline-flex h-5.5 items-center rounded-full px-2.25 text-xs font-medium whitespace-nowrap">
            {t("classDetail.studentCount", { count: data.studentCount })}
          </span>
        }
        actions={
          <>
            {canWrite ? (
              <Button ref={addButton} variant="outline" onClick={() => setAdding(true)}>
                <UserPlus aria-hidden="true" />
                {t("classDetail.addStudent")}
              </Button>
            ) : null}
            {canAssign && data.archivedAt === null ? (
              <Button asChild>
                <Link to={`/teacher/assignments/new?class=${data.id}`}>
                  <ClipboardList aria-hidden="true" />
                  {t("classDetail.assignToClass")}
                </Link>
              </Button>
            ) : null}
          </>
        }
      />

      <div
        className={cn(
          "grid items-start gap-4.5",
          two && "grid-cols-[minmax(0,2fr)_minmax(0,1fr)]",
        )}
      >
        <div className="flex min-w-0 flex-col gap-4.5">
          <ClassMembersCard klass={data} canWrite={canWrite} />
          <ClassAssignmentsCard classId={data.id} />
        </div>
        <StickyAside
          as="div"
          docked={two}
          className="grid min-w-0 grid-cols-[repeat(auto-fit,minmax(min(100%,300px),1fr))] items-start gap-4.5"
        >
          <JoinCodesOffCallout />
          <JoinCodePanel klass={data} canWrite={canWrite} />
          <ClassSettingsCard klass={data} canWrite={canWrite} />
        </StickyAside>
      </div>

      <AddMemberDialog
        klass={data}
        open={adding}
        onOpenChange={setAdding}
        returnFocus={addButton}
      />
    </DeckScale>
  );
}

function DetailSkeleton() {
  const { t } = useTranslation();
  return (
    <div
      role="status"
      aria-live="polite"
      aria-label={t("common.loading")}
      className="flex w-full flex-col gap-4.5"
    >
      <Skeleton className="h-14 w-72 rounded-lg" />
      <div className="grid gap-4.5 min-[1280px]:grid-cols-[2fr_1fr]">
        <Skeleton className="h-80 rounded-xl" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
    </div>
  );
}
