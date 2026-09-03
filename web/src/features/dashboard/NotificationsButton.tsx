import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { useQuery } from "@tanstack/react-query";
import { Bell } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { getDashboard } from "@/features/dashboard/api";

/** The bell A-00 draws in the topbar. */
export function NotificationsButton() {
  const { t } = useTranslation();
  const summary = useQuery({
    queryKey: ["admin-dashboard"],
    queryFn: ({ signal }) => getDashboard(signal),
    staleTime: 60_000,
  });

  const grading = summary.data?.awaitingGrading ?? 0;
  const flagged = summary.data?.flaggedAttempts ?? 0;
  const total = grading + flagged;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="icon-sm"
          className="relative"
          aria-label={
            total > 0
              ? t("dashboard.notificationsWith", { count: total })
              : t("dashboard.notifications")
          }
        >
          <Bell aria-hidden="true" />
          {total > 0 ? (
            <span
              className="bg-destructive absolute top-1 right-1 size-1.5 rounded-full"
              aria-hidden="true"
            />
          ) : null}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-64">
        {total === 0 ? (
          <p className="text-muted-foreground px-2 py-3 text-sm">
            {t("dashboard.nothingOutstanding")}
          </p>
        ) : (
          <>
            {grading > 0 ? (
              <DropdownMenuItem asChild>
                <Link to="/admin">
                  <span>{t("dashboard.awaitingGrading")}</span>
                  <span className="ml-auto tabular-nums">{grading}</span>
                </Link>
              </DropdownMenuItem>
            ) : null}
            {flagged > 0 ? (
              <DropdownMenuItem asChild>
                <Link to="/admin">
                  <span>{t("dashboard.flagged")}</span>
                  <span className="ml-auto tabular-nums">{flagged}</span>
                </Link>
              </DropdownMenuItem>
            ) : null}
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
