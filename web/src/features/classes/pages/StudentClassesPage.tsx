import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { EmptyState, ListSkeleton, LoadError } from "@/components/shared/ListState";
import { shortDate } from "@/lib/i18n/datetime";
import { fetchMyClasses } from "../api";

/** S-10's classes list: what the student is in, and the way into another. */
export default function StudentClassesPage() {
  const { t } = useTranslation();
  const classes = useQuery({
    queryKey: ["my-classes"],
    queryFn: ({ signal }) => fetchMyClasses(signal),
  });

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">
          {t("student.myClasses")}
        </h1>
        <Button asChild variant="outline" size="sm">
          <Link to="/join">
            <Plus aria-hidden="true" />
            {t("student.joinClass")}
          </Link>
        </Button>
      </div>

      {classes.isPending ? (
        <ListSkeleton rows={3} />
      ) : classes.isError ? (
        <LoadError error={classes.error} onRetry={() => void classes.refetch()}>
          {t("student.loadFailed")}
        </LoadError>
      ) : classes.data.items.length === 0 ? (
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
        classes.data.items.map((c) => (
          <Card key={c.id} className="gap-0 p-4">
            <p className="text-sm font-medium">{c.name}</p>
            {/* S-10's second line: who teaches it, and since when. */}
            <p className="text-muted-foreground mt-1 text-xs">
              {c.teacherName === null
                ? t("student.joinedOn", { date: shortDate(c.joinedAt) })
                : t("student.taughtBySince", {
                    teacher: c.teacherName,
                    date: shortDate(c.joinedAt),
                  })}
            </p>
            {c.description && (
              <p className="text-muted-foreground mt-1 text-xs">{c.description}</p>
            )}
          </Card>
        ))
      )}
    </div>
  );
}
