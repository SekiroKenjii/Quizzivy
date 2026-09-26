import { useTranslation } from "react-i18next";
import { ArrowRight, Check, CircleAlert } from "lucide-react";
import { Link } from "react-router";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import type { JoinOutcome } from "../context";

/**
 * JoinedState is the end of a join: the class the student is now in, or, when
 * the enrolment failed, the server's reason. Either way the way on is the
 * student's classes.
 */
export function JoinedState({ outcome }: Readonly<{ outcome: JoinOutcome }>) {
  const { t } = useTranslation();
  const joined = outcome.kind === "joined";
  const Icon = joined ? Check : CircleAlert;
  return (
    <div className="flex flex-col items-center gap-4 text-center">
      <span
        className={cn(
          "grid size-14 place-items-center rounded-3xl",
          joined ? "bg-success-soft text-success-ink" : "bg-muted text-fg",
        )}
      >
        <Icon aria-hidden="true" className="size-7" />
      </span>
      <div>
        <h1 className="text-h1 text-balance break-words">
          {t(joined ? "join.joined.title" : "join.joined.failedTitle", {
            className: outcome.className,
          })}
        </h1>
        {outcome.kind === "joined" ? (
          <p className="text-muted-fg text-body mt-1.5">
            {t("join.joined.body", { teacherName: outcome.teacherName })}
          </p>
        ) : (
          <p role="alert" className="text-muted-fg text-body mt-1.5">
            {outcome.message}
          </p>
        )}
      </div>
      <Button asChild size="xl" className="h-12 self-stretch">
        <Link to="/app/classes">
          {t("join.goToClasses")}
          <ArrowRight aria-hidden="true" className="size-[17px]" />
        </Link>
      </Button>
    </div>
  );
}
