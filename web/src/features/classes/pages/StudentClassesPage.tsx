import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
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
        <p role="status" aria-live="polite" className="text-muted-foreground text-sm">
          {t("common.loading")}
        </p>
      ) : classes.isError ? (
        <div className="space-y-3">
          <p role="alert" className="text-sm">
            {t("student.loadFailed")}
          </p>
          <Button variant="outline" size="sm" onClick={() => void classes.refetch()}>
            {t("common.retry")}
          </Button>
        </div>
      ) : classes.data.items.length === 0 ? (
        <div className="rounded-lg border border-dashed p-8 text-center">
          <p className="text-sm">{t("student.noClasses")}</p>
          <Button asChild size="sm" className="mt-3">
            <Link to="/join">{t("student.joinClass")}</Link>
          </Button>
        </div>
      ) : (
        classes.data.items.map((c) => (
          <Card key={c.id} className="gap-0 p-4">
            <p className="text-sm font-medium">{c.name}</p>
            {c.description && (
              <p className="text-muted-foreground mt-1 text-xs">{c.description}</p>
            )}
          </Card>
        ))
      )}
    </div>
  );
}
