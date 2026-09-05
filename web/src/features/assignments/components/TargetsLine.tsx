import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { GraduationCap } from "lucide-react";
import { Badge, badgeVariants } from "@/components/ui/badge";
import type { Assignment } from "@/features/assignments/api";
import { cn } from "@/lib/utils";

/** G-02's line under the bar: who the work went to, with each class one click away. */
export function TargetsLine({ assignment }: { assignment: Assignment }) {
  const { t } = useTranslation();
  const { classes, students } = assignment.targets;
  return (
    <div className="text-muted-foreground flex flex-wrap items-center gap-2 text-xs">
      <span>{t("assignments.detail.targets")}</span>
      {classes.map((c) => (
        <Link
          key={c.id}
          to={`/admin/classes/${c.id}`}
          className={cn(badgeVariants({ variant: "secondary" }), "hover:bg-accent")}
        >
          <GraduationCap aria-hidden="true" />
          {t("assignments.detail.classChip", { name: c.name, count: c.studentCount })}
        </Link>
      ))}
      {students.length > 0 ? (
        <Badge variant="secondary">
          {t("assignments.detail.extraStudents", { count: students.length })}
        </Badge>
      ) : null}
      <span>
        · {t("assignments.detail.targetCount", { count: assignment.targetCount ?? 0 })}
      </span>
    </div>
  );
}
